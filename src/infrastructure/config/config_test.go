package config

import (
	"testing"
)

func boolPtr(b bool) *bool {
	return &b
}

func TestBackend_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  *bool
		expected bool
	}{
		{"nil (default true)", nil, true},
		{"explicit true", boolPtr(true), true},
		{"explicit false", boolPtr(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Backend{Enabled: tt.enabled}
			if got := b.IsEnabled(); got != tt.expected {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestModelRoute_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  *bool
		expected bool
	}{
		{"nil (default true)", nil, true},
		{"explicit true", boolPtr(true), true},
		{"explicit false", boolPtr(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ModelRoute{Enabled: tt.enabled}
			if got := r.IsEnabled(); got != tt.expected {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestModelAlias_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  *bool
		expected bool
	}{
		{"nil (default true)", nil, true},
		{"explicit true", boolPtr(true), true},
		{"explicit false", boolPtr(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ModelAlias{Enabled: tt.enabled}
			if got := m.IsEnabled(); got != tt.expected {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestModelRoute_IsReasoningEnabled(t *testing.T) {
	tests := []struct {
		name      string
		reasoning *bool
		expected  bool
	}{
		{"nil (default false)", nil, false},
		{"explicit true", boolPtr(true), true},
		{"explicit false", boolPtr(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ModelRoute{Reasoning: tt.reasoning}
			if got := r.IsReasoningEnabled(); got != tt.expected {
				t.Errorf("IsReasoningEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func newTestManager(cfg *Config) *Manager {
	return &Manager{config: cfg}
}

func TestManager_GetBackend(t *testing.T) {
	cfg := &Config{
		Backends: []Backend{
			{Name: "backend1", URL: "http://b1.com"},
			{Name: "backend2", URL: "http://b2.com"},
		},
	}
	cm := newTestManager(cfg)

	tests := []struct {
		name     string
		expected *Backend
	}{
		{"backend1", &cfg.Backends[0]},
		{"backend2", &cfg.Backends[1]},
		{"nonexistent", nil},
	}

	for _, tt := range tests {
		got := cm.GetBackend(tt.name)
		if tt.expected == nil {
			if got != nil {
				t.Errorf("GetBackend(%q) = %v, want nil", tt.name, got)
			}
		} else {
			if got == nil || got.Name != tt.expected.Name {
				t.Errorf("GetBackend(%q) = %v, want %v", tt.name, got, tt.expected)
			}
		}
	}
}

func TestBackend_GetProtocol(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		expected string
	}{
		{"empty defaults to openai", "", "openai"},
		{"openai", "openai", "openai"},
		{"anthropic", "anthropic", "anthropic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Backend{Protocol: tt.protocol}
			if got := b.GetProtocol(); got != tt.expected {
				t.Errorf("Backend.GetProtocol() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestModelRoute_GetProtocol(t *testing.T) {
	tests := []struct {
		name            string
		routeProtocol   string
		backendProtocol string
		expected        string
	}{
		{"empty route uses backend", "", "anthropic", "anthropic"},
		{"route overrides backend", "openai", "anthropic", "openai"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ModelRoute{Protocol: tt.routeProtocol}
			if got := r.GetProtocol(tt.backendProtocol); got != tt.expected {
				t.Errorf("ModelRoute.GetProtocol() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFallback_IsBackoffEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  *bool
		expected bool
	}{
		{"nil defaults to false", nil, false},
		{"explicit true", boolPtr(true), true},
		{"explicit false", boolPtr(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Fallback{EnableBackoff: tt.enabled}
			if got := f.IsBackoffEnabled(); got != tt.expected {
				t.Errorf("Fallback.IsBackoffEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFallback_GetBackoffInitialDelay(t *testing.T) {
	tests := []struct {
		name     string
		delay    int
		expected int
	}{
		{"zero defaults to 100", 0, 100},
		{"negative defaults to 100", -10, 100},
		{"positive value", 200, 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Fallback{BackoffInitialDelay: tt.delay}
			if got := f.GetBackoffInitialDelay(); got != tt.expected {
				t.Errorf("Fallback.GetBackoffInitialDelay() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFallback_GetBackoffMaxDelay(t *testing.T) {
	tests := []struct {
		name     string
		delay    int
		expected int
	}{
		{"zero defaults to 5000", 0, 5000},
		{"negative defaults to 5000", -10, 5000},
		{"positive value", 10000, 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Fallback{BackoffMaxDelay: tt.delay}
			if got := f.GetBackoffMaxDelay(); got != tt.expected {
				t.Errorf("Fallback.GetBackoffMaxDelay() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFallback_GetBackoffMultiplier(t *testing.T) {
	tests := []struct {
		name       string
		multiplier float64
		expected   float64
	}{
		{"zero defaults to 2.0", 0, 2.0},
		{"negative defaults to 2.0", -1.0, 2.0},
		{"positive value", 3.0, 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Fallback{BackoffMultiplier: tt.multiplier}
			if got := f.GetBackoffMultiplier(); got != tt.expected {
				t.Errorf("Fallback.GetBackoffMultiplier() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFallback_GetBackoffJitter(t *testing.T) {
	tests := []struct {
		name     string
		jitter   float64
		expected float64
	}{
		{"zero is valid", 0, 0.0},
		{"within range", 0.5, 0.5},
		{"negative defaults to 0.1", -0.1, 0.1},
		{"above 1 defaults to 0.1", 1.5, 0.1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Fallback{BackoffJitter: tt.jitter}
			if got := f.GetBackoffJitter(); got != tt.expected {
				t.Errorf("Fallback.GetBackoffJitter() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestManager_Get(t *testing.T) {
	cfg := &Config{Listen: ":8080"}
	cm := newTestManager(cfg)

	got := cm.Get()
	if got.Listen != ":8080" {
		t.Errorf("Manager.Get() = %q, want %q", got.Listen, ":8080")
	}
}

func TestLogging_GetLevel(t *testing.T) {
	l := Logging{Level: "debug"}
	if got := l.GetLevel(); got != "debug" {
		t.Errorf("Logging.GetLevel() = %q, want %q", got, "debug")
	}
}

func TestLogging_GetConsoleLevel(t *testing.T) {
	l := Logging{ConsoleLevel: "warn"}
	if got := l.GetConsoleLevel(); got != "warn" {
		t.Errorf("Logging.GetConsoleLevel() = %q, want %q", got, "warn")
	}
}

func TestLogging_GetBaseDir(t *testing.T) {
	l := Logging{BaseDir: "/var/log/app"}
	if got := l.GetBaseDir(); got != "/var/log/app" {
		t.Errorf("Logging.GetBaseDir() = %q, want %q", got, "/var/log/app")
	}
}
