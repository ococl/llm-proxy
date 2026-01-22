package usecase

import (
	"context"
	"time"

	"llm-proxy/domain/entity"
	domainerror "llm-proxy/domain/error"
	"llm-proxy/domain/port"
)

// ProxyRequestUseCase handles the core proxy request flow.
type ProxyRequestUseCase struct {
	logger          port.Logger
	config          port.ConfigProvider
	routeResolver   port.RouteResolver
	protocolConv    port.ProtocolConverter
	backendClient   port.BackendClient
	retryStrategy   port.RetryStrategy
	fallbackStrategy *FallbackStrategy
	loadBalancer    port.LoadBalancer
	metrics         port.MetricsProvider
	requestLogger   port.RequestLogger
}

// NewProxyRequestUseCase creates a new proxy request use case.
func NewProxyRequestUseCase(
	logger port.Logger,
	config port.ConfigProvider,
	routeResolver port.RouteResolver,
	protocolConv port.ProtocolConverter,
	backendClient port.BackendClient,
	retryStrategy port.RetryStrategy,
	fallbackStrategy *FallbackStrategy,
	loadBalancer port.LoadBalancer,
	metrics port.MetricsProvider,
	requestLogger port.RequestLogger,
) *ProxyRequestUseCase {
	return &ProxyRequestUseCase{
		logger:          logger,
		config:          config,
		routeResolver:   routeResolver,
		protocolConv:    protocolConv,
		backendClient:   backendClient,
		retryStrategy:   retryStrategy,
		fallbackStrategy: fallbackStrategy,
		loadBalancer:    loadBalancer,
		metrics:         metrics,
		requestLogger:   requestLogger,
	}
}

// Execute processes a proxy request and returns a response.
func (uc *ProxyRequestUseCase) Execute(ctx context.Context, req *entity.Request) (*entity.Response, error) {
	// 1. Validate request
	if err := uc.validateRequest(req); err != nil {
		return nil, err
	}

	// 2. Resolve routes
	routes, err := uc.routeResolver.Resolve(req.Model().String())
	if err != nil {
		return nil, err
	}

	// 3. Filter available routes (not in cooldown)
	availableRoutes := uc.fallbackStrategy.FilterAvailableRoutes(routes)
	if len(availableRoutes) == 0 {
		// Try fallback aliases
		fallbackRoutes, err := uc.fallbackStrategy.GetFallbackRoutes(req.Model().String(), uc.routeResolver)
		if err != nil || len(fallbackRoutes) == 0 {
			return nil, domainerror.NewNoBackend()
		}
		availableRoutes = fallbackRoutes
	}

	// 4. Select backend using load balancer
	backend := uc.loadBalancer.Select(availableRoutes)
	if backend == nil {
		return nil, domainerror.NewNoBackend()
	}

	// 5. Convert request to backend format
	backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
	if err != nil {
		return nil, domainerror.NewProtocolError("failed to convert request", err)
	}

	// 6. Execute with retry
	resp, err := uc.executeWithRetry(ctx, backendReq, availableRoutes)

	// 7. Convert response to client format
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
		// Select backend
		backend := uc.loadBalancer.Select(routes)
		if backend == nil {
			return nil, domainerror.NewNoBackend()
		}

		// Convert request
		backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
		if err != nil {
			return nil, domainerror.NewProtocolError("request conversion failed", err)
		}

		// Send request
		resp, err := uc.backendClient.Send(ctx, backendReq)
		if err == nil {
			// Success
			return resp, nil
		}

		lastErr = err

		// Record error
		uc.metrics.IncBackendErrors(backend.Name())

		// Check if should retry
		if !uc.retryStrategy.ShouldRetry(attempt, err) {
			return nil, err
		}

		// Get backoff delay
		delay := uc.retryStrategy.GetDelay(attempt)
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	return nil, lastErr
}

// ExecuteStreaming processes a streaming proxy request.
func (uc *ProxyRequestUseCase) ExecuteStreaming(
	ctx context.Context,
	req *entity.Request,
	handler func(*entity.Response) error,
) error {
	// Implementation for streaming
	return nil
}