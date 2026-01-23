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
	if err := uc.validateRequest(req); err != nil {
		return nil, err
	}

	routes, err := uc.routeResolver.Resolve(req.Model().String())
	if err != nil {
		return nil, err
	}

	availableRoutes := uc.fallbackStrategy.FilterAvailableRoutes(routes)
	if len(availableRoutes) == 0 {
		fallbackRoutes, err := uc.fallbackStrategy.GetFallbackRoutes(req.Model().String(), uc.routeResolver)
		if err != nil || len(fallbackRoutes) == 0 {
			return nil, domainerror.NewNoBackend()
		}
		availableRoutes = fallbackRoutes
	}

	backend := uc.loadBalancer.Select(availableRoutes)
	if backend == nil {
		return nil, domainerror.NewNoBackend()
	}

	backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
	if err != nil {
		return nil, domainerror.NewProtocolError("failed to convert request", err)
	}

	resp, err := uc.executeWithRetry(ctx, backendReq, availableRoutes)
	if err != nil {
		return nil, err
	}

	clientResp, err := uc.protocolConv.FromBackend(resp, backend.Protocol())
	if err != nil {
		return nil, domainerror.NewProtocolError("failed to convert response", err)
	}

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
) (*entity.Response, error) {
	var lastErr error
	maxRetries := uc.retryStrategy.GetMaxRetries()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		backend := uc.loadBalancer.Select(routes)
		if backend == nil {
			return nil, domainerror.NewNoBackend()
		}

		backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
		if err != nil {
			return nil, domainerror.NewProtocolError("request conversion failed", err)
		}

		resp, err := uc.backendClient.Send(ctx, backendReq, backend)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		uc.metrics.IncBackendErrors(backend.Name())

		if !uc.retryStrategy.ShouldRetry(attempt, err) {
			return nil, domainerror.NewBackendError(backend.Name(), err)
		}

		delay := uc.retryStrategy.GetDelay(attempt)
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
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
	if err := uc.validateRequest(req); err != nil {
		return err
	}

	routes, err := uc.routeResolver.Resolve(req.Model().String())
	if err != nil {
		return err
	}

	availableRoutes := uc.fallbackStrategy.FilterAvailableRoutes(routes)
	if len(availableRoutes) == 0 {
		fallbackRoutes, err := uc.fallbackStrategy.GetFallbackRoutes(req.Model().String(), uc.routeResolver)
		if err != nil || len(fallbackRoutes) == 0 {
			return domainerror.NewNoBackend()
		}
		availableRoutes = fallbackRoutes
	}

	backend := uc.loadBalancer.Select(availableRoutes)
	if backend == nil {
		return domainerror.NewNoBackend()
	}

	backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
	if err != nil {
		return domainerror.NewProtocolError("failed to convert request", err)
	}

	return uc.executeStreamingWithRetry(ctx, backendReq, availableRoutes, handler)
}

func (uc *ProxyRequestUseCase) executeStreamingWithRetry(
	ctx context.Context,
	req *entity.Request,
	routes []*port.Route,
	handler func(*entity.Response) error,
) error {
	var lastErr error
	maxRetries := uc.retryStrategy.GetMaxRetries()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		backend := uc.loadBalancer.Select(routes)
		if backend == nil {
			return domainerror.NewNoBackend()
		}

		backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
		if err != nil {
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

						delta := entity.Message{
							Role:    role,
							Content: content,
						}
						choice.Delta = &delta
					}

					builder.Choices([]entity.Choice{choice})
				}
			}

			resp := builder.BuildUnsafe()
			return handler(resp)
		}

		clientAdapter := uc.backendClient
		err = clientAdapter.SendStreaming(ctx, backendReq, backend, streamHandler)
		if err == nil {
			return nil
		}

		lastErr = err
		uc.metrics.IncBackendErrors(backend.Name())

		if !uc.retryStrategy.ShouldRetry(attempt, err) {
			return domainerror.NewBackendError(backend.Name(), err)
		}

		delay := uc.retryStrategy.GetDelay(attempt)
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	if lastErr != nil {
		return domainerror.NewNoBackend().WithCause(lastErr)
	}
	return domainerror.NewNoBackend()
}
