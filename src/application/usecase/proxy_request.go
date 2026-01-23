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

	uc.logger.Info("proxy request started",
		port.String("req_id", reqID),
		port.String("model", modelName),
	)

	if err := uc.validateRequest(req); err != nil {
		uc.logger.Warn("request validation failed",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.Error(err),
		)
		return nil, err
	}

	routes, err := uc.routeResolver.Resolve(req.Model().String())
	if err != nil {
		uc.logger.Error("route resolution failed",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.Error(err),
		)
		return nil, err
	}

	uc.logger.Debug("routes resolved",
		port.String("req_id", reqID),
		port.String("model", modelName),
		port.Int("total_routes", len(routes)),
	)

	availableRoutes := uc.fallbackStrategy.FilterAvailableRoutes(routes)
	if len(availableRoutes) == 0 {
		uc.logger.Warn("all backends in cooldown, attempting fallback",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.Int("cooldown_count", len(routes)),
		)

		fallbackRoutes, err := uc.fallbackStrategy.GetFallbackRoutes(req.Model().String(), uc.routeResolver)
		if err != nil || len(fallbackRoutes) == 0 {
			uc.logger.Error("no backends available",
				port.String("req_id", reqID),
				port.String("model", modelName),
			)
			return nil, domainerror.NewNoBackend()
		}

		uc.logger.Info("fallback triggered",
			port.String("req_id", reqID),
			port.String("original_model", modelName),
			port.Int("fallback_routes", len(fallbackRoutes)),
		)
		availableRoutes = fallbackRoutes
	}

	uc.logger.Debug("available routes filtered",
		port.String("req_id", reqID),
		port.String("model", modelName),
		port.Int("available_count", len(availableRoutes)),
	)

	backend := uc.loadBalancer.Select(availableRoutes)
	if backend == nil {
		uc.logger.Error("backend selection failed",
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

	uc.logger.Debug("backend selected",
		port.String("req_id", reqID),
		port.String("model", modelName),
		port.String("backend", backend.Name()),
		port.String("backend_model", backendModelName),
	)

	backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
	if err != nil {
		uc.logger.Error("protocol conversion failed",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.String("backend", backend.Name()),
			port.Error(err),
		)
		return nil, domainerror.NewProtocolError("failed to convert request", err)
	}

	resp, err := uc.executeWithRetry(ctx, backendReq, availableRoutes, backendModelName)
	if err != nil {
		uc.logger.Error("proxy request failed",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.Int64("duration_ms", time.Since(startTime).Milliseconds()),
			port.Error(err),
		)
		return nil, err
	}

	clientResp, err := uc.protocolConv.FromBackend(resp, backend.Protocol())
	if err != nil {
		uc.logger.Error("response conversion failed",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.String("backend", backend.Name()),
			port.Error(err),
		)
		return nil, domainerror.NewProtocolError("failed to convert response", err)
	}

	uc.logger.Info("proxy request completed",
		port.String("req_id", reqID),
		port.String("model", modelName),
		port.String("backend", backend.Name()),
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
			uc.logger.Error("no backend available for retry",
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
			uc.logger.Debug("retry attempt",
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.String("backend", backend.Name()),
				port.Int("attempt", attempt),
				port.Int("max_retries", maxRetries),
			)
		}

		backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
		if err != nil {
			uc.logger.Error("request conversion failed in retry",
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.String("backend", backend.Name()),
				port.Int("attempt", attempt),
				port.Error(err),
			)
			return nil, domainerror.NewProtocolError("request conversion failed", err)
		}

		resp, err := uc.backendClient.Send(ctx, backendReq, backend, currentBackendModel)
		if err == nil {
			if attempt > 0 {
				uc.logger.Info("retry succeeded",
					port.String("req_id", reqID),
					port.String("model", modelName),
					port.String("backend", backend.Name()),
					port.Int("attempt", attempt),
				)
			}
			return resp, nil
		}

		lastErr = err
		uc.metrics.IncBackendErrors(backend.Name())

		uc.logger.Warn("backend error, checking retry",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.String("backend", backend.Name()),
			port.Int("attempt", attempt),
			port.Error(err),
		)

		if !uc.retryStrategy.ShouldRetry(attempt, err) {
			uc.logger.Error("max retries exceeded",
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.String("backend", backend.Name()),
				port.Int("attempts", attempt+1),
				port.Error(lastErr),
			)
			return nil, domainerror.NewBackendError(backend.Name(), err)
		}

		delay := uc.retryStrategy.GetDelay(attempt)
		if delay > 0 {
			uc.logger.Debug("waiting before retry",
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.Int64("delay_ms", delay.Milliseconds()),
				port.Int("next_attempt", attempt+1),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				uc.logger.Warn("request cancelled during retry",
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

	uc.logger.Info("streaming request started",
		port.String("req_id", reqID),
		port.String("model", modelName),
	)

	if err := uc.validateRequest(req); err != nil {
		uc.logger.Warn("streaming request validation failed",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.Error(err),
		)
		return err
	}

	routes, err := uc.routeResolver.Resolve(req.Model().String())
	if err != nil {
		uc.logger.Error("streaming route resolution failed",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.Error(err),
		)
		return err
	}

	availableRoutes := uc.fallbackStrategy.FilterAvailableRoutes(routes)
	if len(availableRoutes) == 0 {
		uc.logger.Warn("streaming: all backends in cooldown, attempting fallback",
			port.String("req_id", reqID),
			port.String("model", modelName),
		)

		fallbackRoutes, err := uc.fallbackStrategy.GetFallbackRoutes(req.Model().String(), uc.routeResolver)
		if err != nil || len(fallbackRoutes) == 0 {
			uc.logger.Error("streaming: no backends available",
				port.String("req_id", reqID),
				port.String("model", modelName),
			)
			return domainerror.NewNoBackend()
		}

		uc.logger.Info("streaming: fallback triggered",
			port.String("req_id", reqID),
			port.String("original_model", modelName),
			port.Int("fallback_routes", len(fallbackRoutes)),
		)
		availableRoutes = fallbackRoutes
	}

	backend := uc.loadBalancer.Select(availableRoutes)
	if backend == nil {
		uc.logger.Error("streaming: backend selection failed",
			port.String("req_id", reqID),
			port.String("model", modelName),
		)
		return domainerror.NewNoBackend()
	}

	uc.logger.Debug("streaming: backend selected",
		port.String("req_id", reqID),
		port.String("model", modelName),
		port.String("backend", backend.Name()),
	)

	backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
	if err != nil {
		uc.logger.Error("streaming: protocol conversion failed",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.String("backend", backend.Name()),
			port.Error(err),
		)
		return domainerror.NewProtocolError("failed to convert request", err)
	}

	selectedRoute := findRouteForBackend(availableRoutes, backend)
	backendModelName := modelName
	if selectedRoute != nil {
		backendModelName = selectedRoute.Model
	}

	err = uc.executeStreamingWithRetry(ctx, backendReq, availableRoutes, backendModelName, handler)
	if err != nil {
		uc.logger.Error("streaming request failed",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.Int64("duration_ms", time.Since(startTime).Milliseconds()),
			port.Error(err),
		)
		return err
	}

	uc.logger.Info("streaming request completed",
		port.String("req_id", reqID),
		port.String("model", modelName),
		port.String("backend", backend.Name()),
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
			uc.logger.Error("streaming: no backend available for retry",
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
			uc.logger.Debug("streaming: retry attempt",
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.String("backend", backend.Name()),
				port.Int("attempt", attempt),
				port.Int("max_retries", maxRetries),
			)
		}

		backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
		if err != nil {
			uc.logger.Error("streaming: request conversion failed in retry",
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.String("backend", backend.Name()),
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
				uc.logger.Warn("streaming: upstream returned null choices, using empty array",
					port.String("req_id", reqID),
					port.String("backend", backend.Name()),
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
				uc.logger.Info("streaming: retry succeeded",
					port.String("req_id", reqID),
					port.String("model", modelName),
					port.String("backend", backend.Name()),
					port.Int("attempt", attempt),
				)
			}
			return nil
		}

		lastErr = err
		uc.metrics.IncBackendErrors(backend.Name())

		uc.logger.Warn("streaming: backend error, checking retry",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.String("backend", backend.Name()),
			port.Int("attempt", attempt),
			port.Error(err),
		)

		if !uc.retryStrategy.ShouldRetry(attempt, err) {
			uc.logger.Error("streaming: max retries exceeded",
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.String("backend", backend.Name()),
				port.Int("attempts", attempt+1),
				port.Error(lastErr),
			)
			return domainerror.NewBackendError(backend.Name(), err)
		}

		delay := uc.retryStrategy.GetDelay(attempt)
		if delay > 0 {
			uc.logger.Debug("streaming: waiting before retry",
				port.String("req_id", reqID),
				port.String("model", modelName),
				port.Int64("delay_ms", delay.Milliseconds()),
				port.Int("next_attempt", attempt+1),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				uc.logger.Warn("streaming: request cancelled during retry",
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
