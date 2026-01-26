package usecase

import (
	"testing"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

type mockConfigProvider struct {
	config *port.Config
}

func (m *mockConfigProvider) Get() *port.Config {
	return m.config
}

func (m *mockConfigProvider) GetBackend(name string) *entity.Backend {
	return nil
}

func (m *mockConfigProvider) GetBackends() []*entity.Backend {
	return nil
}

func (m *mockConfigProvider) GetModelAlias(alias string) *port.ModelAlias {
	return nil
}

func (m *mockConfigProvider) Watch() <-chan struct{} {
	return nil
}

func (m *mockConfigProvider) GetRateLimitConfig() *port.RateLimitConfig {
	return nil
}

func (m *mockConfigProvider) GetConcurrencyConfig() *port.ConcurrencyConfig {
	return nil
}

func (m *mockConfigProvider) GetFallbackConfig() *port.FallbackConfig {
	return nil
}

func (m *mockConfigProvider) GetDetectionConfig() *port.DetectionConfig {
	return nil
}

func (m *mockConfigProvider) GetProxyConfig() *port.ProxyConfig {
	return nil
}

func (m *mockConfigProvider) GetTimeoutConfig() *port.TimeoutConfig {
	return nil
}

func (m *mockConfigProvider) GetListen() string {
	return ""
}

func (m *mockConfigProvider) GetProxyAPIKey() string {
	return ""
}

type mockBackendRepository struct {
	backends map[string]*entity.Backend
}

func (m *mockBackendRepository) GetAll() []*entity.Backend {
	var result []*entity.Backend
	for _, b := range m.backends {
		result = append(result, b)
	}
	return result
}

func (m *mockBackendRepository) GetByName(name string) *entity.Backend {
	return m.backends[name]
}

func (m *mockBackendRepository) GetEnabled() []*entity.Backend {
	var result []*entity.Backend
	for _, b := range m.backends {
		if b.IsEnabled() {
			result = append(result, b)
		}
	}
	return result
}

func (m *mockBackendRepository) GetByNames(names []string) []*entity.Backend {
	var result []*entity.Backend
	for _, name := range names {
		if b, ok := m.backends[name]; ok {
			result = append(result, b)
		}
	}
	return result
}

func TestRouteResolveUseCase_Resolve_EnabledField(t *testing.T) {
	backend1, _ := entity.NewBackend("backend1", "http://b1.com", "key1", true, types.ProtocolOpenAI)
	backend2, _ := entity.NewBackend("backend2", "http://b2.com", "key2", true, types.ProtocolOpenAI)

	backendRepo := &mockBackendRepository{
		backends: map[string]*entity.Backend{
			"backend1": backend1,
			"backend2": backend2,
		},
	}

	configProvider := &mockConfigProvider{
		config: &port.Config{
			Models: map[string]*port.ModelAlias{
				"test/model": {
					Enabled: true,
					Routes: []port.ModelRoute{
						{
							Backend:  "backend1",
							Model:    "model1",
							Priority: 1,
							Enabled:  true,
						},
						{
							Backend:  "backend2",
							Model:    "model2",
							Priority: 2,
							Enabled:  false,
						},
					},
				},
			},
		},
	}

	uc := NewRouteResolveUseCase(configProvider, backendRepo, nil)

	routes, err := uc.Resolve("test/model")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(routes) != 1 {
		t.Fatalf("Expected 1 route, got %d", len(routes))
	}

	if !routes[0].Enabled {
		t.Error("Route.Enabled should be true for enabled route")
	}

	if !routes[0].IsEnabled() {
		t.Error("Route.IsEnabled() should return true for enabled route")
	}

	if routes[0].Backend.Name() != "backend1" {
		t.Errorf("Expected backend1, got %s", routes[0].Backend.Name())
	}
}

func TestRouteResolveUseCase_Resolve_DisabledRoute(t *testing.T) {
	backend1, _ := entity.NewBackend("backend1", "http://b1.com", "key1", true, types.ProtocolOpenAI)

	backendRepo := &mockBackendRepository{
		backends: map[string]*entity.Backend{
			"backend1": backend1,
		},
	}

	configProvider := &mockConfigProvider{
		config: &port.Config{
			Models: map[string]*port.ModelAlias{
				"test/model": {
					Enabled: true,
					Routes: []port.ModelRoute{
						{
							Backend:  "backend1",
							Model:    "model1",
							Priority: 1,
							Enabled:  false,
						},
					},
				},
			},
		},
	}

	uc := NewRouteResolveUseCase(configProvider, backendRepo, nil)

	routes, err := uc.Resolve("test/model")
	if err == nil {
		t.Error("Expected error for disabled route, got nil")
	}

	if routes != nil {
		t.Errorf("Expected nil routes, got %v", routes)
	}
}

func TestRouteResolveUseCase_Resolve_MultipleRoutes(t *testing.T) {
	backend1, _ := entity.NewBackend("backend1", "http://b1.com", "key1", true, types.ProtocolOpenAI)
	backend2, _ := entity.NewBackend("backend2", "http://b2.com", "key2", true, types.ProtocolOpenAI)
	backend3, _ := entity.NewBackend("backend3", "http://b3.com", "key3", true, types.ProtocolAnthropic)

	backendRepo := &mockBackendRepository{
		backends: map[string]*entity.Backend{
			"backend1": backend1,
			"backend2": backend2,
			"backend3": backend3,
		},
	}

	configProvider := &mockConfigProvider{
		config: &port.Config{
			Models: map[string]*port.ModelAlias{
				"test/model": {
					Enabled: true,
					Routes: []port.ModelRoute{
						{
							Backend:  "backend1",
							Model:    "model1",
							Priority: 1,
							Enabled:  true,
						},
						{
							Backend:  "backend2",
							Model:    "model2",
							Priority: 2,
							Enabled:  true,
						},
						{
							Backend:  "backend3",
							Model:    "model3",
							Priority: 3,
							Enabled:  true,
						},
					},
				},
			},
		},
	}

	uc := NewRouteResolveUseCase(configProvider, backendRepo, nil)

	routes, err := uc.Resolve("test/model")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(routes) != 3 {
		t.Fatalf("Expected 3 routes, got %d", len(routes))
	}

	for i, route := range routes {
		if !route.Enabled {
			t.Errorf("Route %d: Enabled should be true", i)
		}
		if !route.IsEnabled() {
			t.Errorf("Route %d: IsEnabled() should return true", i)
		}
	}

	if routes[0].Priority != 1 || routes[1].Priority != 2 || routes[2].Priority != 3 {
		t.Error("Route priorities not preserved correctly")
	}
}

func TestRouteResolveUseCase_Resolve_DisabledBackend(t *testing.T) {
	backend1, _ := entity.NewBackend("backend1", "http://b1.com", "key1", false, types.ProtocolOpenAI)

	backendRepo := &mockBackendRepository{
		backends: map[string]*entity.Backend{
			"backend1": backend1,
		},
	}

	configProvider := &mockConfigProvider{
		config: &port.Config{
			Models: map[string]*port.ModelAlias{
				"test/model": {
					Enabled: true,
					Routes: []port.ModelRoute{
						{
							Backend:  "backend1",
							Model:    "model1",
							Priority: 1,
							Enabled:  true,
						},
					},
				},
			},
		},
	}

	uc := NewRouteResolveUseCase(configProvider, backendRepo, nil)

	routes, err := uc.Resolve("test/model")
	if err == nil {
		t.Error("Expected error for disabled backend, got nil")
	}

	if routes != nil {
		t.Errorf("Expected nil routes, got %v", routes)
	}
}

func TestRouteResolveUseCase_Resolve_MixedEnabledDisabled(t *testing.T) {
	backend1, _ := entity.NewBackend("backend1", "http://b1.com", "key1", true, types.ProtocolOpenAI)
	backend2, _ := entity.NewBackend("backend2", "http://b2.com", "key2", true, types.ProtocolOpenAI)
	backend3, _ := entity.NewBackend("backend3", "http://b3.com", "key3", false, types.ProtocolOpenAI)

	backendRepo := &mockBackendRepository{
		backends: map[string]*entity.Backend{
			"backend1": backend1,
			"backend2": backend2,
			"backend3": backend3,
		},
	}

	configProvider := &mockConfigProvider{
		config: &port.Config{
			Models: map[string]*port.ModelAlias{
				"test/model": {
					Enabled: true,
					Routes: []port.ModelRoute{
						{
							Backend:  "backend1",
							Model:    "model1",
							Priority: 1,
							Enabled:  true,
						},
						{
							Backend:  "backend2",
							Model:    "model2",
							Priority: 2,
							Enabled:  false,
						},
						{
							Backend:  "backend3",
							Model:    "model3",
							Priority: 3,
							Enabled:  true,
						},
					},
				},
			},
		},
	}

	uc := NewRouteResolveUseCase(configProvider, backendRepo, nil)

	routes, err := uc.Resolve("test/model")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(routes) != 1 {
		t.Fatalf("Expected 1 enabled route, got %d", len(routes))
	}

	if routes[0].Backend.Name() != "backend1" {
		t.Errorf("Expected backend1, got %s", routes[0].Backend.Name())
	}

	if !routes[0].Enabled {
		t.Error("Route should be enabled")
	}
}
