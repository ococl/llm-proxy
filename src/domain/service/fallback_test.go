package service

import (
	"errors"
	"testing"
	"time"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

type MockCooldownProvider struct {
	coolingDown map[string]bool
}

func NewMockCooldownProvider() *MockCooldownProvider {
	return &MockCooldownProvider{
		coolingDown: make(map[string]bool),
	}
}

func (m *MockCooldownProvider) IsCoolingDown(backend, model string) bool {
	key := backend + ":" + model
	return m.coolingDown[key]
}

func (m *MockCooldownProvider) SetCooldown(backend, model string, duration time.Duration) {
	key := backend + ":" + model
	m.coolingDown[key] = true
}

func (m *MockCooldownProvider) ClearExpired() {
}

type MockRouteResolver struct {
	routes map[string][]*port.Route
	errors map[string]error
}

func NewMockRouteResolver() *MockRouteResolver {
	return &MockRouteResolver{
		routes: make(map[string][]*port.Route),
		errors: make(map[string]error),
	}
}

func (m *MockRouteResolver) Resolve(alias string) ([]*port.Route, error) {
	if err, ok := m.errors[alias]; ok {
		return nil, err
	}
	return m.routes[alias], nil
}

func (m *MockRouteResolver) AddRoute(alias string, backend *entity.Backend, model string) {
	route := &port.Route{
		Backend:  backend,
		Model:    model,
		Priority: 1,
	}
	m.routes[alias] = append(m.routes[alias], route)
}

func (m *MockRouteResolver) SetError(alias string, err error) {
	m.errors[alias] = err
}

func createTestBackend(name string) *entity.Backend {
	backend, _ := entity.NewBackend(name, "https://api.test.com", "test-key", true, types.ProtocolOpenAI)
	return backend
}

func TestNewFallbackStrategy(t *testing.T) {
	cooldownMgr := NewMockCooldownProvider()
	fallbackAliases := map[string][]entity.ModelAlias{
		"gpt-4": {entity.NewModelAlias("claude")},
	}
	backoffConfig := entity.RetryConfig{
		EnableBackoff:       true,
		BackoffInitialDelay: 100 * time.Millisecond,
		BackoffMaxDelay:     5 * time.Second,
		BackoffMultiplier:   2.0,
		BackoffJitter:       0.1,
		MaxRetries:          3,
	}

	fs := NewFallbackStrategy(cooldownMgr, fallbackAliases, backoffConfig)

	if fs == nil {
		t.Fatal("NewFallbackStrategy returned nil")
	}
	if !fs.enableBackoff {
		t.Error("enableBackoff should be true")
	}
	if fs.maxRetries != 3 {
		t.Errorf("maxRetries = %d, want 3", fs.maxRetries)
	}
}

func TestFallbackStrategy_ShouldRetry(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
		attempt    int
		want       bool
	}{
		{"first attempt, should retry", 3, 0, true},
		{"within limit, should retry", 3, 2, true},
		{"at limit, should not retry", 3, 3, false},
		{"exceeds limit, should not retry", 3, 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &FallbackStrategy{maxRetries: tt.maxRetries}
			got := fs.ShouldRetry(tt.attempt, errors.New("test error"))
			if got != tt.want {
				t.Errorf("ShouldRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFallbackStrategy_GetBackoffDelay(t *testing.T) {
	tests := []struct {
		name              string
		enableBackoff     bool
		backoffInitial    time.Duration
		backoffMax        time.Duration
		backoffMultiplier float64
		backoffJitter     float64
		attempt           int
		wantMin           time.Duration
		wantMax           time.Duration
	}{
		{"backoff disabled, returns zero", false, 100 * time.Millisecond, 5 * time.Second, 2.0, 0.1, 1, 0, 0},
		{"attempt 0, returns zero", true, 100 * time.Millisecond, 5 * time.Second, 2.0, 0.1, 0, 0, 0},
		{"attempt 1, returns initial with jitter", true, 100 * time.Millisecond, 5 * time.Second, 2.0, 0.1, 1, 90 * time.Millisecond, 110 * time.Millisecond},
		{"attempt 2, exponential growth", true, 100 * time.Millisecond, 5 * time.Second, 2.0, 0.1, 2, 180 * time.Millisecond, 220 * time.Millisecond},
		{"attempt 3, exponential growth", true, 100 * time.Millisecond, 5 * time.Second, 2.0, 0.1, 3, 360 * time.Millisecond, 440 * time.Millisecond},
		{"exceeds max, capped at max", true, 1 * time.Second, 2 * time.Second, 3.0, 0.1, 5, 1800 * time.Millisecond, 2200 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &FallbackStrategy{
				enableBackoff:     tt.enableBackoff,
				backoffInitial:    tt.backoffInitial,
				backoffMax:        tt.backoffMax,
				backoffMultiplier: tt.backoffMultiplier,
				backoffJitter:     tt.backoffJitter,
			}

			for i := 0; i < 10; i++ {
				got := fs.GetBackoffDelay(tt.attempt)
				if got < tt.wantMin || got > tt.wantMax {
					t.Errorf("GetBackoffDelay() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
				}
			}
		})
	}
}

func TestFallbackStrategy_GetMaxRetries(t *testing.T) {
	fs := &FallbackStrategy{maxRetries: 5}
	got := fs.GetMaxRetries()
	if got != 5 {
		t.Errorf("GetMaxRetries() = %d, want 5", got)
	}
}

func TestFallbackStrategy_FilterAvailableRoutes(t *testing.T) {
	cooldownMgr := NewMockCooldownProvider()
	fs := &FallbackStrategy{cooldownMgr: cooldownMgr}

	backend1 := createTestBackend("backend1")
	backend2 := createTestBackend("backend2")
	backend3 := createTestBackend("backend3")

	routes := []*port.Route{
		{Backend: backend1, Model: "model1", Priority: 1},
		{Backend: backend2, Model: "model2", Priority: 1},
		{Backend: backend3, Model: "model3", Priority: 2},
	}

	t.Run("all available", func(t *testing.T) {
		available := fs.FilterAvailableRoutes(routes)
		if len(available) != 3 {
			t.Errorf("FilterAvailableRoutes() returned %d routes, want 3", len(available))
		}
	})

	t.Run("some in cooldown", func(t *testing.T) {
		cooldownMgr.SetCooldown("backend2", "model2", 5*time.Minute)
		available := fs.FilterAvailableRoutes(routes)
		if len(available) != 2 {
			t.Errorf("FilterAvailableRoutes() returned %d routes, want 2", len(available))
		}
		for _, route := range available {
			if route.Backend.Name() == "backend2" {
				t.Error("backend2 should be filtered out")
			}
		}
	})

	t.Run("all in cooldown", func(t *testing.T) {
		cooldownMgr.SetCooldown("backend1", "model1", 5*time.Minute)
		cooldownMgr.SetCooldown("backend2", "model2", 5*time.Minute)
		cooldownMgr.SetCooldown("backend3", "model3", 5*time.Minute)
		available := fs.FilterAvailableRoutes(routes)
		if len(available) != 0 {
			t.Errorf("FilterAvailableRoutes() returned %d routes, want 0", len(available))
		}
	})
}

func TestFallbackStrategy_GetFallbackRoutes(t *testing.T) {
	cooldownMgr := NewMockCooldownProvider()
	fallbackAliases := map[string][]entity.ModelAlias{
		"gpt-4":  {entity.NewModelAlias("claude"), entity.NewModelAlias("gemini")},
		"claude": {entity.NewModelAlias("gpt-4")},
	}
	fs := NewFallbackStrategy(cooldownMgr, fallbackAliases, entity.RetryConfig{})

	resolver := NewMockRouteResolver()
	backend1 := createTestBackend("anthropic")
	backend2 := createTestBackend("google")
	resolver.AddRoute("claude", backend1, "claude-3")
	resolver.AddRoute("gemini", backend2, "gemini-pro")

	t.Run("success with multiple fallbacks", func(t *testing.T) {
		routes, err := fs.GetFallbackRoutes("gpt-4", resolver)
		if err != nil {
			t.Fatalf("GetFallbackRoutes() error = %v", err)
		}
		if len(routes) != 2 {
			t.Errorf("GetFallbackRoutes() returned %d routes, want 2", len(routes))
		}
	})

	t.Run("no fallback configured", func(t *testing.T) {
		routes, err := fs.GetFallbackRoutes("unknown-model", resolver)
		if err != nil {
			t.Fatalf("GetFallbackRoutes() error = %v", err)
		}
		if routes != nil {
			t.Errorf("GetFallbackRoutes() returned %v, want nil", routes)
		}
	})

	t.Run("fallback resolution fails", func(t *testing.T) {
		resolver.SetError("claude", errors.New("resolution failed"))
		routes, err := fs.GetFallbackRoutes("gpt-4", resolver)
		if err != nil {
			t.Fatalf("GetFallbackRoutes() error = %v", err)
		}
		if len(routes) != 1 {
			t.Errorf("GetFallbackRoutes() returned %d routes, want 1", len(routes))
		}
	})
}

func TestFallbackStrategy_GetNextRetryDelay(t *testing.T) {
	t.Run("backoff enabled", func(t *testing.T) {
		fs := &FallbackStrategy{
			enableBackoff:     true,
			backoffInitial:    100 * time.Millisecond,
			backoffMax:        5 * time.Second,
			backoffMultiplier: 2.0,
			backoffJitter:     0.1,
		}

		delay := fs.GetNextRetryDelay(1)
		if delay < 90*time.Millisecond || delay > 110*time.Millisecond {
			t.Errorf("GetNextRetryDelay(1) = %v, want between 90ms and 110ms", delay)
		}
	})

	t.Run("backoff disabled", func(t *testing.T) {
		fs := &FallbackStrategy{enableBackoff: false}
		delay := fs.GetNextRetryDelay(1)
		if delay != 0 {
			t.Errorf("GetNextRetryDelay(1) = %v, want 0", delay)
		}
	})
}

func TestFallbackStrategy_BackoffJitterVariation(t *testing.T) {
	fs := &FallbackStrategy{
		enableBackoff:     true,
		backoffInitial:    1 * time.Second,
		backoffMax:        10 * time.Second,
		backoffMultiplier: 2.0,
		backoffJitter:     0.2,
	}

	samples := make([]time.Duration, 100)
	for i := 0; i < 100; i++ {
		samples[i] = fs.GetBackoffDelay(1)
	}

	allSame := true
	first := samples[0]
	for _, sample := range samples[1:] {
		if sample != first {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("GetBackoffDelay() returned identical values, jitter not working")
	}

	minExpected := 800 * time.Millisecond
	maxExpected := 1200 * time.Millisecond
	for i, sample := range samples {
		if sample < minExpected || sample > maxExpected {
			t.Errorf("sample[%d] = %v, want between %v and %v", i, sample, minExpected, maxExpected)
		}
	}
}
