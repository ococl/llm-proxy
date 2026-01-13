package main

import (
	"testing"
	"time"
)

func boolPtr(b bool) *bool {
	return &b
}

func newTestConfigManager(cfg *Config) *ConfigManager {
	return &ConfigManager{config: cfg}
}

func TestRouter_Resolve_Basic(t *testing.T) {
	cfg := &Config{
		Backends: []Backend{
			{Name: "backend1", URL: "http://backend1.com"},
			{Name: "backend2", URL: "http://backend2.com"},
		},
		Models: map[string]*ModelAlias{
			"model-a": {
				Routes: []ModelRoute{
					{Backend: "backend1", Model: "real-model-1", Priority: 1},
					{Backend: "backend2", Model: "real-model-2", Priority: 2},
				},
			},
		},
	}

	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)

	routes, err := router.Resolve("model-a")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if len(routes) != 2 {
		t.Fatalf("Expected 2 routes, got %d", len(routes))
	}

	if routes[0].BackendName != "backend1" {
		t.Errorf("First route should be backend1, got %s", routes[0].BackendName)
	}
	if routes[1].BackendName != "backend2" {
		t.Errorf("Second route should be backend2, got %s", routes[1].BackendName)
	}
}

func TestRouter_Resolve_UnknownAlias(t *testing.T) {
	cfg := &Config{
		Backends: []Backend{},
		Models:   map[string]*ModelAlias{},
	}

	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)

	routes, _ := router.Resolve("unknown")
	if len(routes) != 0 {
		t.Errorf("Unknown alias should return empty routes, got %d", len(routes))
	}
}

func TestRouter_Resolve_DisabledBackend(t *testing.T) {
	cfg := &Config{
		Backends: []Backend{
			{Name: "backend1", URL: "http://backend1.com", Enabled: boolPtr(false)},
			{Name: "backend2", URL: "http://backend2.com"},
		},
		Models: map[string]*ModelAlias{
			"model-a": {
				Routes: []ModelRoute{
					{Backend: "backend1", Model: "m1", Priority: 1},
					{Backend: "backend2", Model: "m2", Priority: 2},
				},
			},
		},
	}

	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)

	routes, _ := router.Resolve("model-a")
	if len(routes) != 1 {
		t.Fatalf("Expected 1 route (disabled backend skipped), got %d", len(routes))
	}
	if routes[0].BackendName != "backend2" {
		t.Errorf("Expected backend2, got %s", routes[0].BackendName)
	}
}

func TestRouter_Resolve_DisabledRoute(t *testing.T) {
	cfg := &Config{
		Backends: []Backend{
			{Name: "backend1", URL: "http://backend1.com"},
			{Name: "backend2", URL: "http://backend2.com"},
		},
		Models: map[string]*ModelAlias{
			"model-a": {
				Routes: []ModelRoute{
					{Backend: "backend1", Model: "m1", Priority: 1, Enabled: boolPtr(false)},
					{Backend: "backend2", Model: "m2", Priority: 2},
				},
			},
		},
	}

	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)

	routes, _ := router.Resolve("model-a")
	if len(routes) != 1 {
		t.Fatalf("Expected 1 route (disabled route skipped), got %d", len(routes))
	}
	if routes[0].BackendName != "backend2" {
		t.Errorf("Expected backend2, got %s", routes[0].BackendName)
	}
}

func TestRouter_Resolve_DisabledAlias(t *testing.T) {
	cfg := &Config{
		Backends: []Backend{
			{Name: "backend1", URL: "http://backend1.com"},
		},
		Models: map[string]*ModelAlias{
			"model-a": {
				Enabled: boolPtr(false),
				Routes: []ModelRoute{
					{Backend: "backend1", Model: "m1", Priority: 1},
				},
			},
		},
	}

	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)

	routes, _ := router.Resolve("model-a")
	if len(routes) != 0 {
		t.Errorf("Disabled alias should return empty routes, got %d", len(routes))
	}
}

func TestRouter_Resolve_CoolingDown(t *testing.T) {
	cfg := &Config{
		Backends: []Backend{
			{Name: "backend1", URL: "http://backend1.com"},
			{Name: "backend2", URL: "http://backend2.com"},
		},
		Models: map[string]*ModelAlias{
			"model-a": {
				Routes: []ModelRoute{
					{Backend: "backend1", Model: "m1", Priority: 1},
					{Backend: "backend2", Model: "m2", Priority: 2},
				},
			},
		},
	}

	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)

	key := cd.Key("backend1", "m1")
	cd.SetCooldown(key, time.Hour)

	routes, _ := router.Resolve("model-a")
	if len(routes) != 1 {
		t.Fatalf("Expected 1 route (cooling down skipped), got %d", len(routes))
	}
	if routes[0].BackendName != "backend2" {
		t.Errorf("Expected backend2, got %s", routes[0].BackendName)
	}
}

func TestRouter_Resolve_AliasFallback(t *testing.T) {
	cfg := &Config{
		Backends: []Backend{
			{Name: "backend1", URL: "http://backend1.com"},
			{Name: "backend2", URL: "http://backend2.com"},
		},
		Models: map[string]*ModelAlias{
			"primary": {
				Routes: []ModelRoute{
					{Backend: "backend1", Model: "m1", Priority: 1},
				},
			},
			"fallback": {
				Routes: []ModelRoute{
					{Backend: "backend2", Model: "m2", Priority: 1},
				},
			},
		},
		Fallback: Fallback{
			AliasFallback: map[string][]string{
				"primary": {"fallback"},
			},
		},
	}

	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)

	routes, _ := router.Resolve("primary")
	if len(routes) != 2 {
		t.Fatalf("Expected 2 routes (primary + fallback), got %d", len(routes))
	}
	if routes[0].BackendName != "backend1" {
		t.Errorf("First route should be backend1, got %s", routes[0].BackendName)
	}
	if routes[1].BackendName != "backend2" {
		t.Errorf("Second route should be backend2 (fallback), got %s", routes[1].BackendName)
	}
}

func TestRouter_Resolve_CircularFallback(t *testing.T) {
	cfg := &Config{
		Backends: []Backend{
			{Name: "backend1", URL: "http://backend1.com"},
		},
		Models: map[string]*ModelAlias{
			"alias-a": {
				Routes: []ModelRoute{
					{Backend: "backend1", Model: "m1", Priority: 1},
				},
			},
			"alias-b": {
				Routes: []ModelRoute{
					{Backend: "backend1", Model: "m2", Priority: 1},
				},
			},
		},
		Fallback: Fallback{
			AliasFallback: map[string][]string{
				"alias-a": {"alias-b"},
				"alias-b": {"alias-a"},
			},
		},
	}

	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)

	routes, _ := router.Resolve("alias-a")
	if len(routes) != 2 {
		t.Fatalf("Expected 2 routes (circular should be detected), got %d", len(routes))
	}
}

func TestRouter_Resolve_PriorityOrder(t *testing.T) {
	cfg := &Config{
		Backends: []Backend{
			{Name: "backend1", URL: "http://backend1.com"},
			{Name: "backend2", URL: "http://backend2.com"},
			{Name: "backend3", URL: "http://backend3.com"},
		},
		Models: map[string]*ModelAlias{
			"model-a": {
				Routes: []ModelRoute{
					{Backend: "backend3", Model: "m3", Priority: 3},
					{Backend: "backend1", Model: "m1", Priority: 1},
					{Backend: "backend2", Model: "m2", Priority: 2},
				},
			},
		},
	}

	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)

	routes, _ := router.Resolve("model-a")
	if len(routes) != 3 {
		t.Fatalf("Expected 3 routes, got %d", len(routes))
	}

	if routes[0].BackendName != "backend1" {
		t.Errorf("First route should be backend1 (priority 1), got %s", routes[0].BackendName)
	}
	if routes[1].BackendName != "backend2" {
		t.Errorf("Second route should be backend2 (priority 2), got %s", routes[1].BackendName)
	}
	if routes[2].BackendName != "backend3" {
		t.Errorf("Third route should be backend3 (priority 3), got %s", routes[2].BackendName)
	}
}

func TestRouter_Resolve_LoadBalancing(t *testing.T) {
	cfg := &Config{
		Backends: []Backend{
			{Name: "backend1", URL: "http://backend1.com"},
			{Name: "backend2", URL: "http://backend2.com"},
			{Name: "backend3", URL: "http://backend3.com"},
		},
		Models: map[string]*ModelAlias{
			"model-a": {
				Routes: []ModelRoute{
					{Backend: "backend1", Model: "m1", Priority: 1},
					{Backend: "backend2", Model: "m2", Priority: 1},
					{Backend: "backend3", Model: "m3", Priority: 1},
				},
			},
		},
	}

	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)

	counts := make(map[string]int)
	for i := 0; i < 100; i++ {
		routes, _ := router.Resolve("model-a")
		if len(routes) > 0 {
			counts[routes[0].BackendName]++
		}
	}

	if len(counts) < 2 {
		t.Errorf("Load balancing should distribute across backends, only got %d unique first choices", len(counts))
	}
}

func TestRouter_Resolve_MissingBackend(t *testing.T) {
	cfg := &Config{
		Backends: []Backend{
			{Name: "backend1", URL: "http://backend1.com"},
		},
		Models: map[string]*ModelAlias{
			"model-a": {
				Routes: []ModelRoute{
					{Backend: "nonexistent", Model: "m1", Priority: 1},
					{Backend: "backend1", Model: "m2", Priority: 2},
				},
			},
		},
	}

	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)

	routes, _ := router.Resolve("model-a")
	if len(routes) != 1 {
		t.Fatalf("Expected 1 route (missing backend skipped), got %d", len(routes))
	}
	if routes[0].BackendName != "backend1" {
		t.Errorf("Expected backend1, got %s", routes[0].BackendName)
	}
}
