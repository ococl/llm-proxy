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
		port.String("req_id", reqID),
		port.String("model", modelName),
		port.String("client_protocol", req.ClientProtocol()),
		port.Bool("stream", req.IsStream()),
	)

	if err := uc.validateRequest(req); err != nil {
		uc.logger.Warn("非流式验证失败",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.Error(err),
		)
		return nil, err
	}

	routes, err := uc.routeResolver.Resolve(req.Model().String())
	if err != nil {
		uc.logger.Error("非流式解析路由失败",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.Error(err),
		)
		return nil, err
	}

	uc.logger.Debug("路由解析完成",
		port.String("req_id", reqID),
		port.String("model", modelName),
		port.Int("total_routes", len(routes)),
	)

	availableRoutes := uc.fallbackStrategy.FilterAvailableRoutes(routes)
	if len(availableRoutes) == 0 {
		uc.logger.Warn("非流式后端全部冷却，尝试降级",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.Int("cooldown_count", len(routes)),
		)

		fallbackRoutes, err := uc.fallbackStrategy.GetFallbackRoutes(req.Model().String(), uc.routeResolver)
		if err != nil || len(fallbackRoutes) == 0 {
			uc.logger.Error("非流式无可用后端",
				port.String("req_id", reqID),
				port.String("model", modelName),
			)
			return nil, domainerror.NewNoBackend()
		}

		uc.logger.Info("非流式触发降级",
			port.String("req_id", reqID),
			port.String("original_model", modelName),
			port.Int("fallback_routes", len(fallbackRoutes)),
		)
		availableRoutes = fallbackRoutes
	}

	uc.logger.Debug("路由过滤完成",
		port.String("req_id", reqID),
		port.String("model", modelName),
		port.Int("available_count", len(availableRoutes)),
	)

	backend := uc.loadBalancer.Select(availableRoutes)
	if backend == nil {
		uc.logger.Error("非流式选择后端失败",
			port.String("req_id", reqID),
			port.String("model", modelName),
		)
		return nil, domainerror.NewNoBackend()
	}

	selectedRoute := findRouteForBackend(availableRoutes, backend)
	backendModelName := modelName
	if selectedRoute != nil {
		backendModelName = selectedRoute.Model
	}

	uc.logger.Debug("后端选择",
		port.String("req_id", reqID),
		port.String("model", modelName),
		port.String("backend", backend.Name()),
		port.String("backend_model", backendModelName),
	)

	backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
	if err != nil {
		uc.logger.Error("非流式转换协议失败",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.String("backend", backend.Name()),
			port.String("backend_model", backendModelName),
			port.Error(err),
		)
		return nil, domainerror.NewProtocolError("failed to convert request", err)
	}

	resp, err := uc.executeWithRetry(ctx, backendReq, availableRoutes, backendModelName)
	if err != nil {
		uc.logger.Error("非流式请求失败",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.Int64("duration_ms", time.Since(startTime).Milliseconds()),
			port.Error(err),
		)
		return nil, err
	}

	clientResp, err := uc.protocolConv.FromBackend(resp, backend.Protocol())
	if err != nil {
		uc.logger.Error("非流式响应转换失败",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.String("backend", backend.Name()),
			port.String("backend_model", backendModelName),
			port.Error(err),
		)
		return nil, domainerror.NewProtocolError("failed to convert response", err)
	}

	uc.logger.Info("非流式请求完成",
		port.String("req_id", reqID),
		port.Int64("duration_ms", time.Since(startTime).Milliseconds()),
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
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.Int("attempt", attempt),
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
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.String("backend", backend.Name()),
				port.String("backend_model", currentBackendModel),
				port.Int("attempt", attempt),
				port.Int("max_retries", maxRetries),
			)
		}

		backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
		if err != nil {
			uc.logger.Error("重试时转换失败",
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.String("backend", backend.Name()),
				port.String("backend_model", currentBackendModel),
				port.Int("attempt", attempt),
				port.Error(err),
			)
			return nil, domainerror.NewProtocolError("request conversion failed", err)
		}

		resp, err := uc.backendClient.Send(ctx, backendReq, backend, currentBackendModel)
		if err == nil {
			if attempt > 0 {
				uc.logger.Info("重试成功",
					port.String("req_id", reqID),
					port.String("model", modelName),
					port.String("backend", backend.Name()),
					port.String("backend_model", currentBackendModel),
					port.Int("attempt", attempt),
				)
			}
			return resp, nil
		}

		lastErr = err
		uc.metrics.IncBackendErrors(backend.Name())

		uc.logger.Warn("后端错误，尝试重试",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.String("backend", backend.Name()),
			port.String("backend_model", currentBackendModel),
			port.Int("attempt", attempt),
			port.Error(err),
		)

		if !uc.retryStrategy.ShouldRetry(attempt, err) {
			uc.logger.Error("重试次数耗尽",
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.String("backend", backend.Name()),
				port.String("backend_model", currentBackendModel),
				port.Int("attempts", attempt+1),
				port.Error(lastErr),
			)
			return nil, domainerror.NewBackendError(backend.Name(), err)
		}

		delay := uc.retryStrategy.GetDelay(attempt)
		if delay > 0 {
			uc.logger.Debug("重试等待",
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.Int64("delay_ms", delay.Milliseconds()),
				port.Int("next_attempt", attempt+1),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				uc.logger.Warn("重试被取消",
					port.String("req_id", reqID),
					port.String("model", modelName),
					port.Int("attempt", attempt),
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

// ExecuteStreaming processes a streaming proxy request.
func (uc *ProxyRequestUseCase) ExecuteStreaming(
	ctx context.Context,
	req *entity.Request,
	handler func(*entity.Response) error,
) error {
	startTime := time.Now()
	reqID := req.ID().String()
	modelName := req.Model().String()

	uc.logger.Info("请求开始",
		port.String("req_id", reqID),
		port.String("model", modelName),
		port.String("client_protocol", req.ClientProtocol()),
	)

	if err := uc.validateRequest(req); err != nil {
		uc.logger.Warn("验证失败",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.Error(err),
		)
		return err
	}

	routes, err := uc.routeResolver.Resolve(req.Model().String())
	if err != nil {
		uc.logger.Error("解析路由失败",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.Error(err),
		)
		return err
	}

	availableRoutes := uc.fallbackStrategy.FilterAvailableRoutes(routes)
	if len(availableRoutes) == 0 {
		uc.logger.Warn("后端全部冷却，尝试降级",
			port.String("req_id", reqID),
			port.String("model", modelName),
		)

		fallbackRoutes, err := uc.fallbackStrategy.GetFallbackRoutes(req.Model().String(), uc.routeResolver)
		if err != nil || len(fallbackRoutes) == 0 {
			uc.logger.Error("无可用后端",
				port.String("req_id", reqID),
				port.String("model", modelName),
			)
			return domainerror.NewNoBackend()
		}

		uc.logger.Info("触发降级",
			port.String("req_id", reqID),
			port.String("original_model", modelName),
			port.Int("fallback_routes", len(fallbackRoutes)),
		)
		availableRoutes = fallbackRoutes
	}

	backend := uc.loadBalancer.Select(availableRoutes)
	if backend == nil {
		uc.logger.Error("选择后端失败",
			port.String("req_id", reqID),
			port.String("model", modelName),
		)
		return domainerror.NewNoBackend()
	}

	selectedRoute := findRouteForBackend(availableRoutes, backend)
	backendModelName := modelName
	if selectedRoute != nil {
		backendModelName = selectedRoute.Model
	}

	uc.logger.Debug("后端选择",
		port.String("req_id", reqID),
		port.String("model", modelName),
		port.String("backend", backend.Name()),
		port.String("backend_model", backendModelName),
	)

	backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
	if err != nil {
		uc.logger.Error("转换协议失败",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.String("backend", backend.Name()),
			port.String("backend_model", backendModelName),
			port.Error(err),
		)
		return domainerror.NewProtocolError("failed to convert request", err)
	}

	err = uc.executeStreamingWithRetry(ctx, backendReq, availableRoutes, backendModelName, handler)
	if err != nil {
		uc.logger.Error("请求失败",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.String("backend", backend.Name()),
			port.String("backend_model", backendModelName),
			port.Int64("duration_ms", time.Since(startTime).Milliseconds()),
			port.Error(err),
		)
		return err
	}

	uc.logger.Info("请求完成",
		port.String("req_id", reqID),
		port.Int64("duration_ms", time.Since(startTime).Milliseconds()),
	)

	return nil
}

func (uc *ProxyRequestUseCase) executeStreamingWithRetry(
	ctx context.Context,
	req *entity.Request,
	routes []*port.Route,
	backendModelName string,
	handler func(*entity.Response) error,
) error {
	var lastErr error
	maxRetries := uc.retryStrategy.GetMaxRetries()
	reqID := req.ID().String()
	modelName := req.Model().String()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		backend := uc.loadBalancer.Select(routes)
		if backend == nil {
			uc.logger.Error("重试时无可用后端",
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.Int("attempt", attempt),
			)
			return domainerror.NewNoBackend()
		}

		selectedRoute := findRouteForBackend(routes, backend)
		currentBackendModel := backendModelName
		if selectedRoute != nil {
			currentBackendModel = selectedRoute.Model
		}

		if attempt > 0 {
			uc.logger.Debug("重试请求",
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.String("backend", backend.Name()),
				port.String("backend_model", currentBackendModel),
				port.Int("attempt", attempt),
				port.Int("max_retries", maxRetries),
			)
		}

		backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
		if err != nil {
			uc.logger.Error("重试时转换失败",
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.String("backend", backend.Name()),
				port.String("backend_model", currentBackendModel),
				port.Int("attempt", attempt),
				port.Error(err),
			)
			return domainerror.NewProtocolError("request conversion failed", err)
		}

		streamHandler := func(chunk []byte) error {
			var chunkData map[string]interface{}
			if err := json.Unmarshal(chunk, &chunkData); err != nil {
				return err
			}

			responseID, _ := chunkData["id"].(string)
			if responseID == "" {
				responseID = "resp-" + req.ID().String()
			}

			model, _ := chunkData["model"].(string)
			if model == "" {
				model = req.Model().String()
			}

			builder := entity.NewResponseBuilder().
				ID(responseID).
				Model(model).
				Object("chat.completion.chunk")

			choicesArray := []entity.Choice{}

			choicesRaw, choicesExists := chunkData["choices"]
			if choicesExists && choicesRaw == nil {
				uc.logger.Warn("上游返回空choices",
					port.String("req_id", reqID),
					port.String("model", modelName),
					port.String("backend", backend.Name()),
					port.String("backend_model", currentBackendModel),
					port.String("response_id", responseID),
				)
			}

			if choices, ok := chunkData["choices"].([]interface{}); ok && len(choices) > 0 {
				if choiceMap, ok := choices[0].(map[string]interface{}); ok {
					index, _ := choiceMap["index"].(float64)
					finishReason, _ := choiceMap["finish_reason"].(string)

					choice := entity.Choice{
						Index:        int(index),
						FinishReason: finishReason,
					}

					if deltaMap, ok := choiceMap["delta"].(map[string]interface{}); ok {
						content, _ := deltaMap["content"].(string)
						role, _ := deltaMap["role"].(string)

						if role != "" || content != "" {
							choice.Delta = &entity.Message{
								Role:    role,
								Content: content,
							}
						}
					}

					choicesArray = append(choicesArray, choice)
				}
			}

			builder.Choices(choicesArray)
			resp := builder.BuildUnsafe()
			return handler(resp)
		}

		clientAdapter := uc.backendClient
		err = clientAdapter.SendStreaming(ctx, backendReq, backend, currentBackendModel, streamHandler)
		if err == nil {
			if attempt > 0 {
				uc.logger.Info("重试成功",
					port.String("req_id", reqID),
					port.String("model", modelName),
					port.String("backend", backend.Name()),
					port.String("backend_model", currentBackendModel),
					port.Int("attempt", attempt),
				)
			}
			return nil
		}

		lastErr = err
		uc.metrics.IncBackendErrors(backend.Name())

		uc.logger.Warn("后端错误，尝试重试",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.String("backend", backend.Name()),
			port.String("backend_model", currentBackendModel),
			port.Int("attempt", attempt),
			port.Error(err),
		)

		if !uc.retryStrategy.ShouldRetry(attempt, err) {
			uc.logger.Error("重试次数耗尽",
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.String("backend", backend.Name()),
				port.String("backend_model", currentBackendModel),
				port.Int("attempts", attempt+1),
				port.Error(lastErr),
			)
			return domainerror.NewBackendError(backend.Name(), err)
		}

		delay := uc.retryStrategy.GetDelay(attempt)
		if delay > 0 {
			uc.logger.Debug("重试等待",
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.Int64("delay_ms", delay.Milliseconds()),
				port.Int("next_attempt", attempt+1),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				uc.logger.Warn("重试被取消",
					port.String("req_id", reqID),
					port.String("model", modelName),
					port.Int("attempt", attempt),
				)
				return ctx.Err()
			}
		}
	}

	if lastErr != nil {
		return domainerror.NewNoBackend().WithCause(lastErr)
	}
	return domainerror.NewNoBackend()
}

// findRouteForBackend finds the route that corresponds to the given backend.
func findRouteForBackend(routes []*port.Route, backend *entity.Backend) *port.Route {
	for _, route := range routes {
		if route.Backend.Name() == backend.Name() {
			return route
		}
	}
	return nil
}
