package usecase

import (
	"context"
	"encoding/json"
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
) *ProxyRequestUseCase {
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
	}
}

// Execute processes a proxy request and returns a response.
func (uc *ProxyRequestUseCase) Execute(ctx context.Context, req *entity.Request) (*entity.Response, error) {
	startTime := time.Now()
	reqID := req.ID().String()
	modelName := req.Model().String()

	uc.logger.Info("非流式请求开始",
		port.String("请求ID", reqID),
		port.String("模型", modelName),
		port.String("客户端协议", req.ClientProtocol()),
		port.Bool("流式", req.IsStream()),
	)

	if err := uc.validateRequest(req); err != nil {
		uc.logger.Warn("非流式验证失败",
			port.String("请求ID", reqID),
			port.String("模型", modelName),
			port.Error(err),
		)
		return nil, err
	}

	routes, err := uc.routeResolver.Resolve(req.Model().String())
	if err != nil {
		uc.logger.Error("非流式解析路由失败",
			port.String("请求ID", reqID),
			port.String("模型", modelName),
			port.Error(err),
		)
		return nil, err
	}

	uc.logger.Debug("路由解析完成",
		port.String("请求ID", reqID),
		port.String("模型", modelName),
		port.Int("总路由数", len(routes)),
	)

	availableRoutes := uc.fallbackStrategy.FilterAvailableRoutes(routes)
	if len(availableRoutes) == 0 {
		uc.logger.Warn("非流式后端全部冷却，尝试降级",
			port.String("请求ID", reqID),
			port.String("模型", modelName),
			port.Int("冷却数量", len(routes)),
		)

		fallbackRoutes, err := uc.fallbackStrategy.GetFallbackRoutes(req.Model().String(), uc.routeResolver)
		if err != nil || len(fallbackRoutes) == 0 {
			uc.logger.Error("非流式无可用后端",
				port.String("请求ID", reqID),
				port.String("模型", modelName),
			)
			return nil, domainerror.NewNoBackend()
		}

		uc.logger.Info("非流式触发降级",
			port.String("请求ID", reqID),
			port.String("原始模型", modelName),
			port.Int("降级路由数", len(fallbackRoutes)),
		)
		availableRoutes = fallbackRoutes
	}

	uc.logger.Debug("路由过滤完成",
		port.String("请求ID", reqID),
		port.String("模型", modelName),
		port.Int("可用数量", len(availableRoutes)),
	)

	backend := uc.loadBalancer.Select(availableRoutes)
	if backend == nil {
		uc.logger.Error("非流式选择后端失败",
			port.String("请求ID", reqID),
			port.String("模型", modelName),
		)
		return nil, domainerror.NewNoBackend()
	}

	selectedRoute := findRouteForBackend(availableRoutes, backend)
	backendModelName := modelName
	if selectedRoute != nil {
		backendModelName = selectedRoute.Model
	}

	uc.logger.Debug("后端选择",
		port.String("请求ID", reqID),
		port.String("模型", modelName),
		port.String("后端", backend.Name()),
		port.String("后端模型", backendModelName),
	)

	backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
	if err != nil {
		uc.logger.Error("非流式转换协议失败",
			port.String("请求ID", reqID),
			port.String("模型", modelName),
			port.String("后端", backend.Name()),
			port.String("后端模型", backendModelName),
			port.Error(err),
		)
		return nil, domainerror.NewProtocolError("failed to convert request", err)
	}

	resp, err := uc.executeWithRetry(ctx, backendReq, availableRoutes, backendModelName)
	if err != nil {
		uc.logger.Error("非流式请求失败",
			port.String("请求ID", reqID),
			port.String("模型", modelName),
			port.Int64("持续时间毫秒", time.Since(startTime).Milliseconds()),
			port.Error(err),
		)
		return nil, err
	}

	// 将响应序列化为字节，然后进行协议转换
	respBody, _ := json.Marshal(resp)
	clientResp, err := uc.protocolConv.FromBackend(respBody, modelName, backend.Protocol())
	if err != nil {
		uc.logger.Error("非流式响应转换失败",
			port.String("请求ID", reqID),
			port.String("模型", modelName),
			port.String("后端", backend.Name()),
			port.String("后端模型", backendModelName),
			port.Error(err),
		)
		return nil, domainerror.NewProtocolError("failed to convert response", err)
	}

	uc.logger.Info("非流式请求完成",
		port.String("请求ID", reqID),
		port.Int64("持续时间毫秒", time.Since(startTime).Milliseconds()),
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
			uc.logger.Error("重试时无可用后端",
				port.String("请求ID", reqID),
				port.String("模型", modelName),
				port.Int("尝试次数", attempt),
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
				port.String("请求ID", reqID),
				port.String("模型", modelName),
				port.String("后端", backend.Name()),
				port.String("后端模型", currentBackendModel),
				port.Int("尝试次数", attempt),
				port.Int("最大重试次数", maxRetries),
			)
		}

		backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
		if err != nil {
			uc.logger.Error("重试时转换失败",
				port.String("请求ID", reqID),
				port.String("模型", modelName),
				port.String("后端", backend.Name()),
				port.String("后端模型", currentBackendModel),
				port.Int("尝试次数", attempt),
				port.Error(err),
			)
			return nil, domainerror.NewProtocolError("request conversion failed", err)
		}

		resp, err := uc.backendClient.Send(ctx, backendReq, backend, currentBackendModel)
		if err == nil {
			if attempt > 0 {
				uc.logger.Info("重试成功",
					port.String("请求ID", reqID),
					port.String("模型", modelName),
					port.String("后端", backend.Name()),
					port.String("后端模型", currentBackendModel),
					port.Int("尝试次数", attempt),
				)
			}
			return resp, nil
		}

		lastErr = err
		uc.metrics.IncBackendErrors(backend.Name())

		uc.logger.Warn("后端错误，尝试重试",
			port.String("请求ID", reqID),
			port.String("模型", modelName),
			port.String("后端", backend.Name()),
			port.String("后端模型", currentBackendModel),
			port.Int("尝试次数", attempt),
			port.Error(err),
		)

		if !uc.retryStrategy.ShouldRetry(attempt, err) {
			uc.logger.Error("重试次数耗尽",
				port.String("请求ID", reqID),
				port.String("模型", modelName),
				port.String("后端", backend.Name()),
				port.String("后端模型", currentBackendModel),
				port.Int("总尝试次数", attempt+1),
				port.Error(lastErr),
			)
			return nil, domainerror.NewBackendError(backend.Name(), err)
		}

		delay := uc.retryStrategy.GetDelay(attempt)
		if delay > 0 {
			uc.logger.Debug("重试等待",
				port.String("请求ID", reqID),
				port.String("模型", modelName),
				port.Int64("延迟毫秒", delay.Milliseconds()),
				port.Int("下次尝试", attempt+1),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				uc.logger.Warn("重试被取消",
					port.String("请求ID", reqID),
					port.String("模型", modelName),
					port.Int("尝试次数", attempt),
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

// findRouteForBackend 根据后端查找对应的路由
func findRouteForBackend(routes []*port.Route, backend *entity.Backend) *port.Route {
	for _, route := range routes {
		if route.Backend.Name() == backend.Name() {
			return route
		}
	}
	return nil
}
