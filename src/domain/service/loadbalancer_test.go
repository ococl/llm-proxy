package service

import (
	"testing"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

func TestLoadBalancer_New(t *testing.T) {
	t.Run("NewLoadBalancer with Random strategy", func(t *testing.T) {
		lb := NewLoadBalancer(StrategyRandom)
		if lb == nil {
			t.Error("Expected non-nil LoadBalancer")
		}
	})

	t.Run("NewLoadBalancer with RoundRobin strategy", func(t *testing.T) {
		lb := NewLoadBalancer(StrategyRoundRobin)
		if lb == nil {
			t.Error("Expected non-nil LoadBalancer")
		}
	})
}

func TestLoadBalancer_Select(t *testing.T) {
	t.Run("Returns nil for empty routes", func(t *testing.T) {
		lb := NewLoadBalancer(StrategyRandom)
		result := lb.Select(nil)
		if result != nil {
			t.Error("Expected nil for nil routes")
		}

		result = lb.Select([]*port.Route{})
		if result != nil {
			t.Error("Expected nil for empty routes")
		}
	})

	t.Run("Returns single route", func(t *testing.T) {
		lb := NewLoadBalancer(StrategyRandom)
		backend, _ := entity.NewBackend("openai", "https://api.openai.com", "key", true, types.ProtocolOpenAI)

		routes := []*port.Route{
			{Backend: backend, Model: "gpt-4", Priority: 1},
		}

		result := lb.Select(routes)
		if result == nil {
			t.Error("Expected non-nil result")
		}
		if result.Name() != "openai" {
			t.Errorf("Expected 'openai', got '%s'", result.Name())
		}
	})

	t.Run("Random strategy returns random backend", func(t *testing.T) {
		lb := NewLoadBalancer(StrategyRandom)
		backend1, _ := entity.NewBackend("openai", "https://api.openai.com", "key1", true, types.ProtocolOpenAI)
		backend2, _ := entity.NewBackend("anthropic", "https://api.anthropic.com", "key2", true, types.ProtocolOpenAI)

		routes := []*port.Route{
			{Backend: backend1, Model: "gpt-4", Priority: 1},
			{Backend: backend2, Model: "claude", Priority: 1},
		}

		selectedBackends := make(map[string]int)
		for i := 0; i < 100; i++ {
			result := lb.Select(routes)
			if result != nil {
				selectedBackends[result.Name()]++
			}
		}

		if len(selectedBackends) < 2 {
			t.Errorf("Expected multiple backends to be selected, got %v", selectedBackends)
		}
	})

	t.Run("RoundRobin strategy may return different backends", func(t *testing.T) {
		lb := NewLoadBalancer(StrategyRoundRobin)
		backend1, _ := entity.NewBackend("openai", "https://api.openai.com", "key1", true, types.ProtocolOpenAI)
		backend2, _ := entity.NewBackend("anthropic", "https://api.anthropic.com", "key2", true, types.ProtocolOpenAI)

		routes := []*port.Route{
			{Backend: backend1, Model: "gpt-4", Priority: 1},
			{Backend: backend2, Model: "claude", Priority: 1},
		}

		result1 := lb.Select(routes)
		result2 := lb.Select(routes)

		// Round-robin implementation detail
		if result1 == nil || result2 == nil {
			t.Error("Expected non-nil results")
		}
	})

	t.Run("Selects only enabled backends", func(t *testing.T) {
		lb := NewLoadBalancer(StrategyRandom)
		enabledBackend, _ := entity.NewBackend("enabled", "https://enabled.com", "key", true, types.ProtocolOpenAI)
		disabledBackend, _ := entity.NewBackend("disabled", "https://disabled.com", "key", false, types.ProtocolOpenAI)

		routes := []*port.Route{
			{Backend: disabledBackend, Model: "model", Priority: 1},
			{Backend: enabledBackend, Model: "model", Priority: 1},
		}

		result := lb.Select(routes)
		if result == nil {
			t.Error("Expected non-nil result with enabled backend")
		}
		if result.Name() != "enabled" {
			t.Errorf("Expected 'enabled', got '%s'", result.Name())
		}
	})
}
