package usecase

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	domain_service "llm-proxy/domain/service"
	"llm-proxy/domain/types"
)

// MockBackendClient is a mock implementation of port.BackendClient
type MockBackendClient struct {
	sendFunc func(ctx context.Context, req *entity.Request, backend *entity.Backend) (*entity.Response, error)
}

func (m *MockBackendClient) Send(ctx context.Context, req *entity.Request, backend *entity.Backend) (*entity.Response, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, req, backend)
	}
	return nil, nil
}

func (m *MockBackendClient) GetHTTPClient() *http.Client {
	return nil
}

// MockRouteResolver is a mock implementation of port.RouteResolver
type MockRouteResolver struct {
	resolveFunc func(alias string) ([]*port.Route, error)
}

func (m *MockRouteResolver) Resolve(alias string) ([]*port.Route, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(alias)
	}
	return nil, nil
}

// MockLogger is a mock implementation of port.Logger
type MockLogger struct{}

func (m *MockLogger) Debug(msg string, fields ...port.Field)  {}
func (m *MockLogger) Info(msg string, fields ...port.Field)   {}
func (m *MockLogger) Warn(msg string, fields ...port.Field)   {}
func (m *MockLogger) Error(msg string, fields ...port.Field)  {}
func (m *MockLogger) Fatal(msg string, fields ...port.Field)  {}
func (m *MockLogger) With(fields ...port.Field) port.Logger   { return m }
func (m *MockLogger) LogRequest(reqID string, content string) {}
func (m *MockLogger) LogError(reqID string, content string)   {}

// MockConfigProvider is a mock implementation of port.ConfigProvider
type MockConfigProvider struct{}

func (m *MockConfigProvider) Get() *port.Config                           { return nil }
func (m *MockConfigProvider) Watch() <-chan struct{}                      { return nil }
func (m *MockConfigProvider) GetBackend(name string) *entity.Backend      { return nil }
func (m *MockConfigProvider) GetModelAlias(alias string) *port.ModelAlias { return nil }

// MockMetricsProvider is a mock implementation of port.MetricsProvider
type MockMetricsProvider struct{}

func (m *MockMetricsProvider) IncRequestsTotal(backend string)                       {}
func (m *MockMetricsProvider) RecordDuration(backend string, duration time.Duration) {}
func (m *MockMetricsProvider) IncBackendErrors(backend string)                       {}
func (m *MockMetricsProvider) SetCircuitBreakerState(backend string, state int)      {}
func (m *MockMetricsProvider) IncActiveRequests()                                    {}
func (m *MockMetricsProvider) DecActiveRequests()                                    {}
func (m *MockMetricsProvider) GetSnapshot() map[string]interface{}                   { return nil }

// MockRequestLogger is a mock implementation of port.RequestLogger
type MockRequestLogger struct{}

func (m *MockRequestLogger) LogRequest(reqID string, content string) {}
func (m *MockRequestLogger) LogError(reqID string, content string)   {}

func TestNewProxyRequestUseCase(t *testing.T) {
	uc := NewProxyRequestUseCase(
		&MockLogger{},
		&MockConfigProvider{},
		&MockRouteResolver{},
		nil, // protocolConverter
		&MockBackendClient{},
		nil, // retryStrategy
		nil, // fallbackStrategy
		nil, // loadBalancer
		&MockMetricsProvider{},
		&MockRequestLogger{},
	)

	if uc == nil {
		t.Error("NewProxyRequestUseCase should not return nil")
	}
}

func TestProxyRequestUseCase_ValidateRequest_EmptyModel(t *testing.T) {
	uc := NewProxyRequestUseCase(
		&MockLogger{},
		&MockConfigProvider{},
		&MockRouteResolver{},
		nil,
		&MockBackendClient{},
		nil,
		nil,
		nil,
		&MockMetricsProvider{},
		&MockRequestLogger{},
	)

	req := entity.NewRequest(
		entity.NewRequestID("test-123"),
		entity.NewModelAlias(""), // Empty model
		[]entity.Message{
			entity.NewMessage("user", "Hello"),
		},
	)

	_, err := uc.Execute(context.Background(), req)
	if err == nil {
		t.Error("Execute should return error for empty model")
	}
}

func TestRetryStrategy_GetMaxRetries(t *testing.T) {
	strategy := NewRetryStrategy(
		3, // maxRetries
		true,
		100*time.Millisecond,
		1*time.Second,
		2.0,
		0.1,
	)

	if strategy.GetMaxRetries() != 3 {
		t.Errorf("GetMaxRetries() = %v, want %v", strategy.GetMaxRetries(), 3)
	}
}

func TestRetryStrategy_ShouldRetry(t *testing.T) {
	strategy := NewRetryStrategy(
		3,
		true,
		100*time.Millisecond,
		1*time.Second,
		2.0,
		0.1,
	)

	testErr := errors.New("test error")

	// Should retry for first 3 attempts (0, 1, 2)
	if !strategy.ShouldRetry(0, testErr) {
		t.Error("ShouldRetry(0) should return true")
	}
	if !strategy.ShouldRetry(1, testErr) {
		t.Error("ShouldRetry(1) should return true")
	}
	if !strategy.ShouldRetry(2, testErr) {
		t.Error("ShouldRetry(2) should return true")
	}

	// Should not retry for attempt 3 (exceeds maxRetries)
	if strategy.ShouldRetry(3, testErr) {
		t.Error("ShouldRetry(3) should return false")
	}
}

func TestRetryStrategy_ShouldRetry_NoBackoff(t *testing.T) {
	strategy := NewRetryStrategy(
		3,
		false, // No backoff
		100*time.Millisecond,
		1*time.Second,
		2.0,
		0.1,
	)

	testErr := errors.New("test error")

	// Should still retry
	if !strategy.ShouldRetry(0, testErr) {
		t.Error("ShouldRetry should work even without backoff")
	}
}

func TestRetryStrategy_GetDelay(t *testing.T) {
	strategy := NewRetryStrategy(
		3,
		true,
		100*time.Millisecond,
		1*time.Second,
		2.0,
		0.0, // No jitter for predictable test
	)

	// Delay should increase with attempts
	delay0 := strategy.GetDelay(0)
	delay1 := strategy.GetDelay(1)
	delay2 := strategy.GetDelay(2)

	if delay1 <= delay0 {
		t.Error("Delay should increase with attempts")
	}
	if delay2 <= delay1 {
		t.Error("Delay should increase with attempts")
	}
}

func TestLoadBalancer_Select(t *testing.T) {
	lb := domain_service.NewLoadBalancer(domain_service.StrategyRandom)

	backend1, _ := entity.NewBackend("b1", "http://b1", "key1", true, types.ProtocolOpenAI)
	backend2, _ := entity.NewBackend("b2", "http://b2", "key2", true, types.ProtocolOpenAI)

	routes := []*port.Route{
		{Backend: backend1, Model: "m1", Priority: 1},
		{Backend: backend2, Model: "m1", Priority: 1},
	}

	// Select should return one of the backends
	result := lb.Select(routes)
	if result == nil {
		t.Error("LoadBalancer.Select should return a backend")
	}
	if result.Name() != "b1" && result.Name() != "b2" {
		t.Errorf("LoadBalancer.Select returned unexpected backend: %v", result.Name())
	}
}

func TestLoadBalancer_EmptyRoutes(t *testing.T) {
	lb := domain_service.NewLoadBalancer(domain_service.StrategyRandom)

	result := lb.Select(nil)
	if result != nil {
		t.Error("LoadBalancer.Select should return nil for nil routes")
	}

	result = lb.Select([]*port.Route{})
	if result != nil {
		t.Error("LoadBalancer.Select should return nil for empty routes")
	}
}
