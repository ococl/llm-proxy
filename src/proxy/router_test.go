package proxy

import (
	"testing"
	"time"

	"llm-proxy/backend"
	"llm-proxy/config"
)

func boolPtr(b bool) *bool {
	return &b
}

func newTestManager(cfg *config.Config) *config.Manager {
	cm := &config.Manager{}
	cm.SetConfigForTest(cfg)
	return cm
}

func TestRouter_Resolve_Basic(t *testing.T) {
	cfg := &config.Config{
		Backends: []config.Backend{
			{Name: "backend1", URL: "http://backend1.com"},
			{Name: "backend2", URL: "http://backend2.com"},
		},
		Models: map[string]*config.ModelAlias{
			"model-a": {
				Routes: []config.ModelRoute{
					{Backend: "backend1", Model: "real-model-1", Priority: 1},
					{Backend: "backend2", Model: "real-model-2", Priority: 2},
				},
			},
		},
	}

	cm := newTestManager(cfg)
	cd := backend.NewCooldownManager()
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

func TestRouter_Resolve_DisabledBackend(t *testing.T) {
	cfg := &config.Config{
		Backends: []config.Backend{
			{Name: "backend1", URL: "http://backend1.com", Enabled: boolPtr(false)},
			{Name: "backend2", URL: "http://backend2.com"},
		},
		Models: map[string]*config.ModelAlias{
			"model-a": {
				Routes: []config.ModelRoute{
					{Backend: "backend1", Model: "m1", Priority: 1},
					{Backend: "backend2", Model: "m2", Priority: 2},
				},
			},
		},
	}

	cm := newTestManager(cfg)
	cd := backend.NewCooldownManager()
	router := NewRouter(cm, cd)

	routes, _ := router.Resolve("model-a")
	if len(routes) != 1 {
		t.Fatalf("Expected 1 route (disabled backend skipped), got %d", len(routes))
	}
	if routes[0].BackendName != "backend2" {
		t.Errorf("Expected backend2, got %s", routes[0].BackendName)
	}
}

func TestRouter_Resolve_CoolingDown(t *testing.T) {
	cfg := &config.Config{
		Backends: []config.Backend{
			{Name: "backend1", URL: "http://backend1.com"},
			{Name: "backend2", URL: "http://backend2.com"},
		},
		Models: map[string]*config.ModelAlias{
			"model-a": {
				Routes: []config.ModelRoute{
					{Backend: "backend1", Model: "m1", Priority: 1},
					{Backend: "backend2", Model: "m2", Priority: 2},
				},
			},
		},
	}

	cm := newTestManager(cfg)
	cd := backend.NewCooldownManager()
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
	cfg := &config.Config{
		Backends: []config.Backend{
			{Name: "backend1", URL: "http://backend1.com"},
			{Name: "backend2", URL: "http://backend2.com"},
		},
		Models: map[string]*config.ModelAlias{
			"primary": {
				Routes: []config.ModelRoute{
					{Backend: "backend1", Model: "m1", Priority: 1},
				},
			},
			"fallback": {
				Routes: []config.ModelRoute{
					{Backend: "backend2", Model: "m2", Priority: 1},
				},
			},
		},
		Fallback: config.Fallback{
			AliasFallback: map[string][]string{
				"primary": {"fallback"},
			},
		},
	}

	cm := newTestManager(cfg)
	cd := backend.NewCooldownManager()
	router := NewRouter(cm, cd)

	routes, _ := router.Resolve("primary")
	if len(routes) != 2 {
		t.Fatalf("Expected 2 routes (primary + fallback), got %d", len(routes))
	}
}
