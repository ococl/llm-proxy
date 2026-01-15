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
