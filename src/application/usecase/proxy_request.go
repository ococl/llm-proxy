package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"llm-proxy/domain/entity"
	domainerror "llm-proxy/domain/error"
	"llm-proxy/domain/port"
	"llm-proxy/domain/service"
)

// ProxyRequestUseCase handles the core proxy request flow.
type ProxyRequestUseCase struct {
	logger           port.Logger
	config           port.ConfigProvider
	routeResolver    port.RouteResolver
	protocolConv     port.ProtocolConverter
	backendClient    port.BackendClient
	retryStrategy    port.RetryStrategy
	fallbackStrategy *service.FallbackStrategy
	loadBalancer     port.LoadBalancer
	metrics          port.MetricsProvider
	requestLogger    port.RequestLogger
	bodyLogger       port.BodyLogger
	cooldownProvider port.CooldownProvider
}

// NewProxyRequestUseCase creates a new proxy request use case.
func NewProxyRequestUseCase(
	logger port.Logger,
	config port.ConfigProvider,
	routeResolver port.RouteResolver,
	protocolConv port.ProtocolConverter,
	backendClient port.BackendClient,
	retryStrategy port.RetryStrategy,
	fallbackStrategy *service.FallbackStrategy,
	loadBalancer port.LoadBalancer,
	metrics port.MetricsProvider,
	requestLogger port.RequestLogger,
	bodyLogger port.BodyLogger,
	cooldownProvider port.CooldownProvider,
) *ProxyRequestUseCase {
	if bodyLogger == nil {
		bodyLogger = &port.NopBodyLogger{}
	}
	if cooldownProvider == nil {
		cooldownProvider = &port.NopCooldownProvider{}
	}
	return &ProxyRequestUseCase{
		logger:           logger,
		config:           config,
		routeResolver:    routeResolver,
		protocolConv:     protocolConv,
		backendClient:    backendClient,
		retryStrategy:    retryStrategy,
		fallbackStrategy: fallbackStrategy,
		loadBalancer:     loadBalancer,
		metrics:          metrics,
		requestLogger:    requestLogger,
		bodyLogger:       bodyLogger,
		cooldownProvider: cooldownProvider,
	}
}

// Execute processes a proxy request and returns a response.
func (uc *ProxyRequestUseCase) Execute(ctx context.Context, req *entity.Request) (*entity.Response, error) {
	startTime := time.Now()
	reqID := req.ID().String()
	modelName := req.Model().String()

	uc.logger.Info("非流式请求开始",
		port.ReqID(reqID),
		port.Model(modelName),
		port.ClientProtocol(req.ClientProtocol()),
		port.Streaming(req.IsStream()),
	)

	if err := uc.validateRequest(req); err != nil {
		uc.logger.Warn("非流式验证失败",
			port.ReqID(reqID),
			port.Model(modelName),
			port.Error(err),
		)
		return nil, err
	}

	routes, err := uc.routeResolver.Resolve(req.Model().String())
	if err != nil {
		uc.logger.Error("非流式解析路由失败",
			port.ReqID(reqID),
			port.Model(modelName),
			port.Error(err),
		)
		return nil, err
	}

	uc.logger.Debug("路由解析完成",
		port.ReqID(reqID),
		port.Model(modelName),
		port.TotalRoutes(len(routes)),
	)

	availableRoutes := uc.fallbackStrategy.FilterAvailableRoutes(routes)
	if len(availableRoutes) == 0 {
		uc.logger.Warn("非流式后端全部冷却，尝试降级",
			port.ReqID(reqID),
			port.Model(modelName),
			port.CooldownCount(len(routes)),
		)

		fallbackRoutes, err := uc.fallbackStrategy.GetFallbackRoutes(req.Model().String(), uc.routeResolver)
		if err != nil || len(fallbackRoutes) == 0 {
			uc.logger.Error("非流式无可用后端",
				port.ReqID(reqID),
				port.Model(modelName),
			)
			return nil, domainerror.NewNoBackend()
		}

		uc.logger.Info("非流式触发降级",
			port.ReqID(reqID),
			port.OriginalModel(modelName),
			port.FallbackRoutes(len(fallbackRoutes)),
		)
		availableRoutes = fallbackRoutes
	}

	uc.logger.Debug("路由过滤完成",
		port.ReqID(reqID),
		port.Model(modelName),
		port.AvailableCount(len(availableRoutes)),
	)

	backend := uc.loadBalancer.Select(availableRoutes)
	if backend == nil {
		uc.logger.Error("非流式选择后端失败",
			port.ReqID(reqID),
			port.Model(modelName),
		)
		return nil, domainerror.NewNoBackend()
	}

	selectedRoute := findRouteForBackend(availableRoutes, backend)
	backendModelName := modelName
	if selectedRoute != nil {
		backendModelName = selectedRoute.Model
	}

	uc.logger.Debug("backend选择",
		port.ReqID(reqID),
		port.Model(modelName),
		port.Backend(backend.Name()),
		port.BackendModel(backendModelName),
	)

	backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
	if err != nil {
		uc.logger.Error("非流式转换协议失败",
			port.ReqID(reqID),
			port.Model(modelName),
			port.Backend(backend.Name()),
			port.BackendModel(backendModelName),
			port.Error(err),
		)
		return nil, domainerror.NewProtocolError("failed to convert request", err)
	}

	resp, err := uc.executeWithRetry(ctx, backendReq, availableRoutes, backendModelName)
	if err != nil {
		uc.logger.Error("非流式请求失败",
			port.ReqID(reqID),
			port.Model(modelName),
			port.DurationMSInt(time.Since(startTime).Milliseconds()),
			port.Error(err),
		)
		return nil, err
	}

	// 将响应序列化为字节，然后进行协议转换
	respBody, _ := json.Marshal(resp)
	clientResp, err := uc.protocolConv.FromBackend(respBody, modelName, backend.Protocol())
	if err != nil {
		uc.logger.Error("非流式响应转换失败",
			port.ReqID(reqID),
			port.Model(modelName),
			port.Backend(backend.Name()),
			port.BackendModel(backendModelName),
			port.Error(err),
		)
		return nil, domainerror.NewProtocolError("failed to convert response", err)
	}

	uc.logger.Info("非流式请求完成",
		port.ReqID(reqID),
		port.DurationMSInt(time.Since(startTime).Milliseconds()),
	)

	return clientResp, nil
}

// validateRequest validates the incoming request.
func (uc *ProxyRequestUseCase) validateRequest(req *entity.Request) error {
	if req.Model().IsEmpty() {
		return domainerror.NewMissingModel()
	}
	return nil
}

// executeWithRetry executes the request with retry logic.
func (uc *ProxyRequestUseCase) executeWithRetry(
	ctx context.Context,
	req *entity.Request,
	routes []*port.Route,
	backendModelName string,
) (*entity.Response, error) {
	var lastErr error
	maxRetries := uc.retryStrategy.GetMaxRetries()
	reqID := req.ID().String()
	modelName := req.Model().String()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		backend := uc.loadBalancer.Select(routes)
		if backend == nil {
			uc.logger.Error("重试时无可用backend",
				port.ReqID(reqID),
				port.Model(modelName),
				port.Attempt(attempt),
			)
			return nil, domainerror.NewNoBackend()
		}

		selectedRoute := findRouteForBackend(routes, backend)
		currentBackendModel := backendModelName
		if selectedRoute != nil {
			currentBackendModel = selectedRoute.Model
		}

		if attempt > 0 {
			uc.logger.Debug("重试请求",
				port.ReqID(reqID),
				port.Model(modelName),
				port.Backend(backend.Name()),
				port.BackendModel(currentBackendModel),
				port.Attempt(attempt),
				port.MaxRetries(maxRetries),
			)
		}

		backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
		if err != nil {
			uc.logger.Error("重试时转换失败",
				port.ReqID(reqID),
				port.Model(modelName),
				port.Backend(backend.Name()),
				port.BackendModel(currentBackendModel),
				port.Attempt(attempt),
				port.Error(err),
			)
			return nil, domainerror.NewProtocolError("request conversion failed", err)
		}

		resp, err := uc.backendClient.Send(ctx, backendReq, backend, currentBackendModel)
		if err == nil {
			if attempt > 0 {
				uc.logger.Info("重试成功",
					port.ReqID(reqID),
					port.Model(modelName),
					port.Backend(backend.Name()),
					port.BackendModel(currentBackendModel),
					port.Attempt(attempt),
				)
			}
			return resp, nil
		}

		lastErr = err
		uc.metrics.IncBackendErrors(backend.Name())

		uc.logger.Warn("backend错误，尝试重试",
			port.ReqID(reqID),
			port.Model(modelName),
			port.Backend(backend.Name()),
			port.BackendModel(currentBackendModel),
			port.Attempt(attempt),
			port.Error(err),
		)

		if !uc.retryStrategy.ShouldRetry(attempt, err) {
			// 遇到不可重试的错误（如4xx客户端错误），将后端加入冷却
			uc.cooldownBackendIfNeeded(backend.Name(), modelName, err)

			uc.logger.Error("后端不可重试错误，触发冷却",
				port.ReqID(reqID),
				port.Model(modelName),
				port.Backend(backend.Name()),
				port.BackendModel(currentBackendModel),
				port.TotalAttempts(attempt+1),
				port.Error(lastErr),
			)
			return nil, domainerror.NewBackendError(backend.Name(), err)
		}

		delay := uc.retryStrategy.GetDelay(attempt)
		if delay > 0 {
			uc.logger.Debug("重试等待",
				port.ReqID(reqID),
				port.Model(modelName),
				port.DelayMSInt(delay.Milliseconds()),
				port.NextAttempt(attempt+1),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				uc.logger.Warn("重试被取消",
					port.ReqID(reqID),
					port.Model(modelName),
					port.Attempt(attempt),
				)
				return nil, ctx.Err()
			}
		}
	}

	if lastErr != nil {
		return nil, domainerror.NewNoBackend().WithCause(lastErr)
	}
	return nil, domainerror.NewNoBackend()
}

// cooldownBackendIfNeeded 在遇到不可重试错误时将后端加入冷却。
// 对于4xx客户端错误（认证、权限、请求格式错误等），将后端短暂冷却，
// 避免立即重试同一后端。
func (uc *ProxyRequestUseCase) cooldownBackendIfNeeded(backendName, modelName string, err error) {
	if err == nil {
		return
	}

	errMsg := err.Error()
	// 只对客户端错误(4xx)触发冷却，服务器错误(5xx)不应该触发冷却
	// 因为5xx可能是暂时性问题，应该让负载均衡器选择其他后端重试
	if isClientErrorForCooldown(errMsg) {
		// 使用默认冷却时间 30 秒
		uc.cooldownProvider.SetCooldown(backendName, modelName, 30*time.Second)
		uc.logger.Info("后端触发冷却（客户端错误）",
			port.ReqID(""),
			port.Backend(backendName),
			port.Model(modelName),
			port.Int("冷却秒数", 30),
		)
	}
}

// isClientErrorForCooldown 判断是否应该触发冷却的客户端错误
func isClientErrorForCooldown(errMsg string) bool {
	clientErrorPatterns := []string{
		"401", "Unauthorized",
		"403", "Forbidden",
		"400", "Bad Request",
		"404", "Not Found",
		"422", "Unprocessable Entity",
	}

	for _, pattern := range clientErrorPatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}
	return false
}

// findRouteForBackend 根据backend查找对应的路由
func findRouteForBackend(routes []*port.Route, backend *entity.Backend) *port.Route {
	for _, route := range routes {
		if route.Backend.Name() == backend.Name() {
			return route
		}
	}
	return nil
}
