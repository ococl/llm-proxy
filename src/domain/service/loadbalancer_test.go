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
			{Backend: backend, Model: "gpt-4", Priority: 1, Enabled: true},
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
			{Backend: backend1, Model: "gpt-4", Priority: 1, Enabled: true},
			{Backend: backend2, Model: "claude", Priority: 1, Enabled: true},
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
			{Backend: backend1, Model: "gpt-4", Priority: 1, Enabled: true},
			{Backend: backend2, Model: "claude", Priority: 1, Enabled: true},
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
			{Backend: disabledBackend, Model: "model", Priority: 1, Enabled: false},
			{Backend: enabledBackend, Model: "model", Priority: 1, Enabled: true},
		}

		result := lb.Select(routes)
		if result == nil {
			t.Error("Expected non-nil result with enabled backend")
		}
		if result.Name() != "enabled" {
			t.Errorf("Expected 'enabled', got '%s'", result.Name())
		}
	})

	// ============ Weighted Strategy Tests ============

	t.Run("Weighted selects highest priority", func(t *testing.T) {
		lb := NewLoadBalancer(StrategyWeighted)
		backend1, _ := entity.NewBackend("high-priority", "https://high.com", "key1", true, types.ProtocolOpenAI)
		backend2, _ := entity.NewBackend("low-priority", "https://low.com", "key2", true, types.ProtocolOpenAI)

		routes := []*port.Route{
			{Backend: backend1, Model: "model", Priority: 1, Enabled: true},
			{Backend: backend2, Model: "model", Priority: 10, Enabled: true},
		}

		// 多次选择，应该总是返回高优先级
		for i := 0; i < 20; i++ {
			result := lb.Select(routes)
			if result == nil {
				t.Error("Expected non-nil result")
			}
			if result.Name() != "high-priority" {
				t.Errorf("Expected 'high-priority', got '%s'", result.Name())
			}
		}
	})

	t.Run("Weighted falls back to next priority when highest unavailable", func(t *testing.T) {
		lb := NewLoadBalancer(StrategyWeighted)
		backend1, _ := entity.NewBackend("high-priority", "https://high.com", "key1", true, types.ProtocolOpenAI)
		backend2, _ := entity.NewBackend("medium-priority", "https://medium.com", "key2", true, types.ProtocolOpenAI)
		backend3, _ := entity.NewBackend("low-priority", "https://low.com", "key3", true, types.ProtocolOpenAI)

		routes := []*port.Route{
			{Backend: backend1, Model: "model", Priority: 1, Enabled: false}, // 禁用
			{Backend: backend2, Model: "model", Priority: 5, Enabled: true},
			{Backend: backend3, Model: "model", Priority: 10, Enabled: true},
		}

		// 应该回退到 priority 5
		result := lb.Select(routes)
		if result == nil {
			t.Error("Expected non-nil result with fallback")
		}
		if result.Name() != "medium-priority" {
			t.Errorf("Expected 'medium-priority', got '%s'", result.Name())
		}
	})

	t.Run("Weighted falls back through all priorities", func(t *testing.T) {
		lb := NewLoadBalancer(StrategyWeighted)
		backend1, _ := entity.NewBackend("p1", "https://p1.com", "key1", true, types.ProtocolOpenAI)
		backend2, _ := entity.NewBackend("p2", "https://p2.com", "key2", true, types.ProtocolOpenAI)
		backend3, _ := entity.NewBackend("p3", "https://p3.com", "key3", true, types.ProtocolOpenAI)

		routes := []*port.Route{
			{Backend: backend1, Model: "model", Priority: 1, Enabled: false},
			{Backend: backend2, Model: "model", Priority: 2, Enabled: false},
			{Backend: backend3, Model: "model", Priority: 3, Enabled: true},
		}

		result := lb.Select(routes)
		if result == nil {
			t.Error("Expected non-nil result with fallback")
		}
		if result.Name() != "p3" {
			t.Errorf("Expected 'p3', got '%s'", result.Name())
		}
	})

	t.Run("Weighted selects first from same priority group", func(t *testing.T) {
		lb := NewLoadBalancer(StrategyWeighted)
		backend1, _ := entity.NewBackend("backend1", "https://b1.com", "key1", true, types.ProtocolOpenAI)
		backend2, _ := entity.NewBackend("backend2", "https://b2.com", "key2", true, types.ProtocolOpenAI)
		backend3, _ := entity.NewBackend("backend3", "https://b3.com", "key3", true, types.ProtocolOpenAI)

		routes := []*port.Route{
			{Backend: backend1, Model: "model", Priority: 1, Enabled: true},
			{Backend: backend2, Model: "model", Priority: 1, Enabled: true},
			{Backend: backend3, Model: "model", Priority: 2, Enabled: true},
		}

		// 多次选择，应该总是选择第一个后端（在 priority 1 组内按配置顺序选择第一个）
		for i := 0; i < 100; i++ {
			result := lb.Select(routes)
			if result == nil {
				t.Error("Expected non-nil result")
			}
			// 应该总是选择第一个（backend1）
			if result.Name() != "backend1" {
				t.Errorf("Expected 'backend1', got '%s' at iteration %d", result.Name(), i)
			}
		}
		// backend3 不应该被选择（priority 2 更低）
	})

	t.Run("Weighted handles negative priorities", func(t *testing.T) {
		lb := NewLoadBalancer(StrategyWeighted)
		backend1, _ := entity.NewBackend("negative", "https://neg.com", "key1", true, types.ProtocolOpenAI)
		backend2, _ := entity.NewBackend("positive", "https://pos.com", "key2", true, types.ProtocolOpenAI)

		routes := []*port.Route{
			{Backend: backend1, Model: "model", Priority: -5, Enabled: true}, // 更高优先级（更小的数字）
			{Backend: backend2, Model: "model", Priority: 10, Enabled: true},
		}

		// 应该总是选择负数优先级（更高优先级）
		result := lb.Select(routes)
		if result == nil {
			t.Error("Expected non-nil result")
		}
		if result.Name() != "negative" {
			t.Errorf("Expected 'negative' (priority -5), got '%s'", result.Name())
		}
	})

	t.Run("Weighted returns nil when all routes disabled", func(t *testing.T) {
		lb := NewLoadBalancer(StrategyWeighted)
		backend1, _ := entity.NewBackend("backend1", "https://b1.com", "key1", true, types.ProtocolOpenAI)
		backend2, _ := entity.NewBackend("backend2", "https://b2.com", "key2", true, types.ProtocolOpenAI)

		routes := []*port.Route{
			{Backend: backend1, Model: "model", Priority: 1, Enabled: false},
			{Backend: backend2, Model: "model", Priority: 2, Enabled: false},
		}

		result := lb.Select(routes)
		if result != nil {
			t.Error("Expected nil when all routes disabled")
		}
	})
}
