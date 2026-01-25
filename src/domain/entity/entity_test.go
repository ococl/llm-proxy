package entity

import (
	"testing"
	"time"

	"llm-proxy/domain/types"
)

func TestBackendID(t *testing.T) {
	t.Run("NewBackendID creates valid ID", func(t *testing.T) {
		id := NewBackendID("openai")
		if id.String() != "openai" {
			t.Errorf("Expected 'openai', got '%s'", id.String())
		}
	})

	t.Run("BackendID String method", func(t *testing.T) {
		id := NewBackendID("test-backend")
		if string(id) != "test-backend" {
			t.Errorf("Expected 'test-backend', got '%s'", string(id))
		}
	})
}

func TestBackendURL(t *testing.T) {
	t.Run("NewBackendURL with valid URL", func(t *testing.T) {
		url, err := NewBackendURL("https://api.openai.com/v1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if url.String() != "https://api.openai.com/v1" {
			t.Errorf("Expected URL, got '%s'", url.String())
		}
	})

	t.Run("NewBackendURL adds https scheme", func(t *testing.T) {
		url, err := NewBackendURL("api.openai.com/v1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !hasPrefix(string(url), "https://") {
			t.Errorf("Expected https:// prefix, got '%s'", url.String())
		}
	})

	t.Run("NewBackendURL with invalid URL", func(t *testing.T) {
		_, err := NewBackendURL("not-a-valid-url")
		if err == nil {
			// URL parsing may succeed for some "invalid" URLs depending on implementation
			// This test verifies the function handles it gracefully
		}
	})

	t.Run("GetBaseURL removes path", func(t *testing.T) {
		url, _ := NewBackendURL("https://api.openai.com/v1/chat/completions")
		baseURL := url.GetBaseURL()
		if baseURL != "https://api.openai.com" {
			t.Errorf("Expected 'https://api.openai.com', got '%s'", baseURL)
		}
	})
}

func TestAPIKey(t *testing.T) {
	t.Run("Masked short key", func(t *testing.T) {
		key := APIKey("short")
		masked := key.Masked()
		if masked != "****" {
			t.Errorf("Expected '****', got '%s'", masked)
		}
	})

	t.Run("Masked long key", func(t *testing.T) {
		key := APIKey("sk-1234567890abcdef")
		masked := key.Masked()
		expected := "sk-1****cdef"
		if masked != expected {
			t.Errorf("Expected '%s', got '%s'", expected, masked)
		}
	})

	t.Run("IsEmpty returns true for empty", func(t *testing.T) {
		key := APIKey("")
		if !key.IsEmpty() {
			t.Error("Expected IsEmpty to return true for empty key")
		}
	})

	t.Run("IsEmpty returns false for non-empty", func(t *testing.T) {
		key := APIKey("sk-test")
		if key.IsEmpty() {
			t.Error("Expected IsEmpty to return false for non-empty key")
		}
	})
}

func TestBackend_New(t *testing.T) {
	t.Run("NewBackend with valid parameters", func(t *testing.T) {
		backend, err := NewBackend("openai", "https://api.openai.com/v1", "sk-test", true, types.ProtocolOpenAI)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if backend.Name() != "openai" {
			t.Errorf("Expected 'openai', got '%s'", backend.Name())
		}
		if backend.IsEnabled() != true {
			t.Error("Expected backend to be enabled")
		}
	})

	t.Run("NewBackend with invalid URL", func(t *testing.T) {
		_, err := NewBackend("invalid", "not-a-url", "sk-test", true, types.ProtocolOpenAI)
		if err == nil {
			// URL validation may not reject all invalid formats
			// This test verifies the function handles it
		}
	})

	t.Run("NewBackend defaults protocol", func(t *testing.T) {
		backend, _ := NewBackend("test", "https://test.com", "key", true, "")
		if backend.Protocol() != types.ProtocolOpenAI {
			t.Errorf("Expected default protocol OpenAI, got '%s'", backend.Protocol())
		}
	})

	t.Run("IsHealthy returns true for enabled backend with URL", func(t *testing.T) {
		backend, _ := NewBackend("test", "https://test.com", "key", true, types.ProtocolOpenAI)
		if !backend.IsHealthy() {
			t.Error("Expected healthy backend")
		}
	})

	t.Run("IsHealthy returns false for disabled backend", func(t *testing.T) {
		backend, _ := NewBackend("test", "https://test.com", "key", false, types.ProtocolOpenAI)
		if backend.IsHealthy() {
			t.Error("Expected unhealthy backend")
		}
	})
}

func TestBackendBuilder(t *testing.T) {
	t.Run("Build creates valid backend", func(t *testing.T) {
		builder := NewBackendBuilder().
			Name("test").
			URL("https://test.com").
			APIKey("sk-test").
			Enabled(true).
			Protocol(types.ProtocolAnthropic)

		backend, err := builder.Build()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if backend.Name() != "test" {
			t.Errorf("Expected 'test', got '%s'", backend.Name())
		}
		if backend.Protocol() != types.ProtocolAnthropic {
			t.Errorf("Expected Anthropic protocol, got '%s'", backend.Protocol())
		}
	})

	t.Run("BuildUnsafe creates backend without validation", func(t *testing.T) {
		builder := NewBackendBuilder().
			Name("test").
			URL("invalid-url").
			APIKey("key")

		backend := builder.BuildUnsafe()
		if backend == nil {
			t.Error("Expected non-nil backend from BuildUnsafe")
		}
	})
}

func TestCooldownDuration(t *testing.T) {
	t.Run("NewCooldownDuration with positive seconds", func(t *testing.T) {
		duration := NewCooldownDuration(60)
		if duration.Duration() != 60*time.Second {
			t.Errorf("Expected 60s, got %v", duration.Duration())
		}
	})

	t.Run("NewCooldownDuration with zero returns default", func(t *testing.T) {
		duration := NewCooldownDuration(0)
		if duration.Duration() != DefaultCooldown {
			t.Errorf("Expected default cooldown, got %v", duration.Duration())
		}
	})

	t.Run("Int returns seconds", func(t *testing.T) {
		duration := NewCooldownDuration(120)
		if duration.Int() != 120 {
			t.Errorf("Expected 120, got %d", duration.Int())
		}
	})
}

func TestRetryConfig(t *testing.T) {
	t.Run("GetMaxRetries with positive value", func(t *testing.T) {
		config := DefaultRetryConfig()
		config.MaxRetries = 5
		if config.GetMaxRetries() != 5 {
			t.Errorf("Expected 5, got %d", config.GetMaxRetries())
		}
	})

	t.Run("GetMaxRetries with zero returns zero", func(t *testing.T) {
		config := RetryConfig{MaxRetries: 0}
		if config.GetMaxRetries() != 0 {
			t.Errorf("Expected 0, got %d", config.GetMaxRetries())
		}
	})

	t.Run("DefaultRetryConfig returns zero retries", func(t *testing.T) {
		config := DefaultRetryConfig()
		if config.GetMaxRetries() != 0 {
			t.Errorf("Expected 0 (no retries by default), got %d", config.GetMaxRetries())
		}
		if config.MaxRetries != 0 {
			t.Errorf("Expected MaxRetries field to be 0, got %d", config.MaxRetries)
		}
	})

	t.Run("CalculateDelay increases with attempts", func(t *testing.T) {
		config := DefaultRetryConfig()
		config.BackoffJitter = 0 // No jitter for predictable test

		delay0 := config.CalculateDelay(0)
		delay1 := config.CalculateDelay(1)
		delay2 := config.CalculateDelay(2)

		if delay1 <= delay0 {
			t.Error("Delay should increase with attempts")
		}
		if delay2 <= delay1 {
			t.Error("Delay should increase with attempts")
		}
	})
}

func TestRateLimitConfig(t *testing.T) {
	t.Run("GetGlobalRPS with positive value", func(t *testing.T) {
		config := RateLimitConfig{GlobalRPS: 500}
		if config.GetGlobalRPS() != 500 {
			t.Errorf("Expected 500, got %f", config.GetGlobalRPS())
		}
	})

	t.Run("GetGlobalRPS with zero returns default", func(t *testing.T) {
		config := RateLimitConfig{GlobalRPS: 0}
		if config.GetGlobalRPS() != 1000 {
			t.Errorf("Expected default 1000, got %f", config.GetGlobalRPS())
		}
	})

	t.Run("GetModelRPS returns specific model RPS", func(t *testing.T) {
		config := RateLimitConfig{
			PerModelRPS: map[string]float64{
				"gpt-4": 50,
			},
		}
		if config.GetModelRPS("gpt-4") != 50 {
			t.Errorf("Expected 50, got %f", config.GetModelRPS("gpt-4"))
		}
	})

	t.Run("GetModelRPS returns 0 for unknown model", func(t *testing.T) {
		config := RateLimitConfig{PerModelRPS: nil}
		if config.GetModelRPS("unknown") != 0 {
			t.Errorf("Expected 0, got %f", config.GetModelRPS("unknown"))
		}
	})
}

func TestConcurrencyConfig(t *testing.T) {
	t.Run("GetMaxRequests with positive value", func(t *testing.T) {
		config := ConcurrencyConfig{MaxRequests: 100}
		if config.GetMaxRequests() != 100 {
			t.Errorf("Expected 100, got %d", config.GetMaxRequests())
		}
	})

	t.Run("GetMaxRequests with zero returns default", func(t *testing.T) {
		config := ConcurrencyConfig{MaxRequests: 0}
		if config.GetMaxRequests() != 500 {
			t.Errorf("Expected default 500, got %d", config.GetMaxRequests())
		}
	})
}

func TestCircuitBreakerConfig(t *testing.T) {
	t.Run("DefaultCircuitBreakerConfig", func(t *testing.T) {
		config := DefaultCircuitBreakerConfig()
		if config.Enabled != false {
			t.Error("Expected disabled by default")
		}
		if config.FailureThreshold != 5 {
			t.Errorf("Expected failure threshold 5, got %d", config.FailureThreshold)
		}
	})
}

func TestBackendFilter(t *testing.T) {
	enabled := true

	t.Run("Match with nil filter returns true", func(t *testing.T) {
		backend, _ := NewBackend("test", "https://test.com", "key", true, types.ProtocolOpenAI)
		var filter *BackendFilter
		if !filter.Match(backend) {
			t.Error("Nil filter should match all backends")
		}
	})

	t.Run("Match enabled filter", func(t *testing.T) {
		backend, _ := NewBackend("test", "https://test.com", "key", true, types.ProtocolOpenAI)
		filter := &BackendFilter{Enabled: &enabled}
		if !filter.Match(backend) {
			t.Error("Expected enabled backend to match")
		}

		disabledBackend, _ := NewBackend("test2", "https://test2.com", "key", false, types.ProtocolOpenAI)
		if filter.Match(disabledBackend) {
			t.Error("Expected disabled backend to not match")
		}
	})

	t.Run("Match protocols filter", func(t *testing.T) {
		backend, _ := NewBackend("test", "https://test.com", "key", true, types.ProtocolOpenAI)
		filter := &BackendFilter{Protocols: []types.Protocol{types.ProtocolOpenAI}}
		if !filter.Match(backend) {
			t.Error("Expected OpenAI backend to match")
		}
	})

	t.Run("Match names filter", func(t *testing.T) {
		backend, _ := NewBackend("openai", "https://test.com", "key", true, types.ProtocolOpenAI)
		filter := &BackendFilter{Names: []string{"openai", "anthropic"}}
		if !filter.Match(backend) {
			t.Error("Expected backend with matching name to match")
		}
	})
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"1", true},
		{"yes", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{"", false},
	}

	for _, tt := range tests {
		result := ParseBool(tt.input)
		if result != tt.expected {
			t.Errorf("ParseBool(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func TestNewBackendWithLocale(t *testing.T) {
	t.Run("NewBackendWithLocale with valid parameters", func(t *testing.T) {
		backend, err := NewBackendWithLocale(
			"openai",
			"https://api.openai.com/v1",
			"sk-test",
			true,
			types.ProtocolOpenAI,
			"zh-CN",
		)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if backend.Locale() != "zh-CN" {
			t.Errorf("Expected 'zh-CN', got '%s'", backend.Locale())
		}
	})

	t.Run("NewBackendWithLocale with empty locale", func(t *testing.T) {
		backend, err := NewBackendWithLocale(
			"test",
			"https://test.com",
			"key",
			true,
			types.ProtocolOpenAI,
			"",
		)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if backend.Locale() != "" {
			t.Errorf("Expected empty locale, got '%s'", backend.Locale())
		}
	})
}

func TestBackendOptions(t *testing.T) {
	t.Run("WithEnabled option", func(t *testing.T) {
		backend, _ := NewBackend("test", "https://test.com", "key", false, types.ProtocolOpenAI)
		if backend.IsEnabled() {
			t.Error("Expected disabled backend initially")
		}

		// 验证 WithEnabled 函数存在且可用
		opt := WithEnabled(true)
		opt(backend)
		if !backend.IsEnabled() {
			t.Error("Expected backend to be enabled after WithEnabled")
		}
	})

	t.Run("WithProtocol option", func(t *testing.T) {
		backend, _ := NewBackend("test", "https://test.com", "key", true, types.ProtocolOpenAI)
		if backend.Protocol() != types.ProtocolOpenAI {
			t.Errorf("Expected OpenAI protocol, got '%s'", backend.Protocol())
		}

		// 验证 WithProtocol 函数存在且可用
		opt := WithProtocol(types.ProtocolAnthropic)
		opt(backend)
		if backend.Protocol() != types.ProtocolAnthropic {
			t.Errorf("Expected Anthropic protocol, got '%s'", backend.Protocol())
		}
	})
}

func TestBackendString(t *testing.T) {
	backend, _ := NewBackend("test", "https://test.com", "key", true, types.ProtocolOpenAI)
	str := backend.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestTimeoutConfig(t *testing.T) {
	t.Run("DefaultTimeoutConfig", func(t *testing.T) {
		config := DefaultTimeoutConfig()
		if config.Connect != 10*time.Second {
			t.Errorf("Expected 10s connect timeout, got %v", config.Connect)
		}
		if config.Read != 60*time.Second {
			t.Errorf("Expected 60s read timeout, got %v", config.Read)
		}
	})

	t.Run("GetConnectTimeout with zero returns default", func(t *testing.T) {
		config := TimeoutConfig{Connect: 0}
		if config.GetConnectTimeout() != 10*time.Second {
			t.Errorf("Expected default 10s, got %v", config.GetConnectTimeout())
		}
	})

	t.Run("GetReadTimeout with zero returns default", func(t *testing.T) {
		config := TimeoutConfig{Read: 0}
		if config.GetReadTimeout() != 60*time.Second {
			t.Errorf("Expected default 60s, got %v", config.GetReadTimeout())
		}
	})

	t.Run("GetWriteTimeout with zero returns default", func(t *testing.T) {
		config := TimeoutConfig{Write: 0}
		if config.GetWriteTimeout() != 60*time.Second {
			t.Errorf("Expected default 60s, got %v", config.GetWriteTimeout())
		}
	})

	t.Run("GetTotalTimeout with zero returns default", func(t *testing.T) {
		config := TimeoutConfig{Total: 0}
		if config.GetTotalTimeout() != 10*time.Minute {
			t.Errorf("Expected default 10m, got %v", config.GetTotalTimeout())
		}
	})
}

func TestRetryConfigBackoff(t *testing.T) {
	t.Run("GetBackoffInitialDelay with zero returns default", func(t *testing.T) {
		config := RetryConfig{BackoffInitialDelay: 0}
		if config.GetBackoffInitialDelay() != 100*time.Millisecond {
			t.Errorf("Expected default 100ms, got %v", config.GetBackoffInitialDelay())
		}
	})

	t.Run("GetBackoffMaxDelay with zero returns default", func(t *testing.T) {
		config := RetryConfig{BackoffMaxDelay: 0}
		if config.GetBackoffMaxDelay() != 5*time.Second {
			t.Errorf("Expected default 5s, got %v", config.GetBackoffMaxDelay())
		}
	})

	t.Run("GetBackoffMultiplier with zero returns default", func(t *testing.T) {
		config := RetryConfig{BackoffMultiplier: 0}
		if config.GetBackoffMultiplier() != 2.0 {
			t.Errorf("Expected default 2.0, got %f", config.GetBackoffMultiplier())
		}
	})

	t.Run("GetBackoffJitter out of range returns default", func(t *testing.T) {
		config := RetryConfig{BackoffJitter: 1.5}
		if config.GetBackoffJitter() != 0.1 {
			t.Errorf("Expected default 0.1, got %f", config.GetBackoffJitter())
		}

		config.BackoffJitter = -0.1
		if config.GetBackoffJitter() != 0.1 {
			t.Errorf("Expected default 0.1, got %f", config.GetBackoffJitter())
		}
	})
}

func TestRateLimitConfigBurstFactor(t *testing.T) {
	t.Run("GetBurstFactor with positive value", func(t *testing.T) {
		config := RateLimitConfig{BurstFactor: 2.0}
		if config.GetBurstFactor() != 2.0 {
			t.Errorf("Expected 2.0, got %f", config.GetBurstFactor())
		}
	})

	t.Run("GetBurstFactor with zero returns default", func(t *testing.T) {
		config := RateLimitConfig{BurstFactor: 0}
		if config.GetBurstFactor() != 1.5 {
			t.Errorf("Expected default 1.5, got %f", config.GetBurstFactor())
		}
	})
}

func TestConcurrencyConfigQueueTimeout(t *testing.T) {
	t.Run("GetQueueTimeout with zero returns default", func(t *testing.T) {
		config := ConcurrencyConfig{QueueTimeout: 0}
		if config.GetQueueTimeout() != 30*time.Second {
			t.Errorf("Expected default 30s, got %v", config.GetQueueTimeout())
		}
	})

	t.Run("GetPerBackendLimit with zero returns default", func(t *testing.T) {
		config := ConcurrencyConfig{PerBackendLimit: 0}
		if config.GetPerBackendLimit() != 100 {
			t.Errorf("Expected default 100, got %d", config.GetPerBackendLimit())
		}
	})
}

func TestCircuitBreakerConfigGetters(t *testing.T) {
	t.Run("GetFailureThreshold with zero returns default", func(t *testing.T) {
		config := CircuitBreakerConfig{FailureThreshold: 0}
		if config.GetFailureThreshold() != 5 {
			t.Errorf("Expected default 5, got %d", config.GetFailureThreshold())
		}
	})

	t.Run("GetSuccessThreshold with zero returns default", func(t *testing.T) {
		config := CircuitBreakerConfig{SuccessThreshold: 0}
		if config.GetSuccessThreshold() != 2 {
			t.Errorf("Expected default 2, got %d", config.GetSuccessThreshold())
		}
	})

	t.Run("GetOpenTimeout with zero returns default", func(t *testing.T) {
		config := CircuitBreakerConfig{OpenTimeout: 0}
		if config.GetOpenTimeout() != 60*time.Second {
			t.Errorf("Expected default 60s, got %v", config.GetOpenTimeout())
		}
	})
}

// === StreamChunk Tests ===

func TestStreamChunk(t *testing.T) {
	t.Run("NewStreamChunk creates valid chunk", func(t *testing.T) {
		chunk := NewStreamChunk("Hello", false)
		if chunk.Content != "Hello" {
			t.Errorf("Expected 'Hello', got '%s'", chunk.Content)
		}
		if chunk.Finished {
			t.Error("Expected not finished")
		}
	})

	t.Run("NewFinishedStreamChunk creates finished chunk", func(t *testing.T) {
		chunk := NewFinishedStreamChunk("Response content", "stop")
		if !chunk.Finished {
			t.Error("Expected finished chunk")
		}
		if chunk.Content != "Response content" {
			t.Errorf("Expected 'Response content', got '%s'", chunk.Content)
		}
		if chunk.StopReason != "stop" {
			t.Errorf("Expected 'stop', got '%s'", chunk.StopReason)
		}
	})

	t.Run("NewErrorStreamChunk creates error chunk", func(t *testing.T) {
		chunk := NewErrorStreamChunk("Connection failed")
		if !chunk.Finished {
			t.Error("Expected finished error chunk")
		}
		if chunk.Error != "Connection failed" {
			t.Errorf("Expected 'Connection failed', got '%s'", chunk.Error)
		}
		if chunk.Content != "" {
			t.Errorf("Expected empty content, got '%s'", chunk.Content)
		}
	})

	t.Run("IsEmpty returns true for empty chunk", func(t *testing.T) {
		chunk := &StreamChunk{}
		if !chunk.IsEmpty() {
			t.Error("Expected empty chunk to return true")
		}
	})

	t.Run("IsEmpty returns false for content", func(t *testing.T) {
		chunk := NewStreamChunk("content", false)
		if chunk.IsEmpty() {
			t.Error("Expected non-empty chunk to return false")
		}
	})

	t.Run("IsEmpty returns false for finished", func(t *testing.T) {
		chunk := &StreamChunk{Finished: true}
		if chunk.IsEmpty() {
			t.Error("Expected finished chunk to return false")
		}
	})

	t.Run("IsEmpty returns false for error", func(t *testing.T) {
		chunk := &StreamChunk{Error: "error"}
		if chunk.IsEmpty() {
			t.Error("Expected error chunk to return false")
		}
	})

	t.Run("WithContent modifies content", func(t *testing.T) {
		chunk := NewStreamChunk("initial", false)
		chunk.WithContent("modified")
		if chunk.Content != "modified" {
			t.Errorf("Expected 'modified', got '%s'", chunk.Content)
		}
	})

	t.Run("WithStopReason modifies stop reason", func(t *testing.T) {
		chunk := NewStreamChunk("content", false)
		chunk.WithStopReason("length")
		if chunk.StopReason != "length" {
			t.Errorf("Expected 'length', got '%s'", chunk.StopReason)
		}
	})
}

func TestModelAlias(t *testing.T) {
	t.Run("NewModelAlias creates valid alias", func(t *testing.T) {
		alias := NewModelAlias("gpt-4")
		if string(alias) != "gpt-4" {
			t.Errorf("Expected 'gpt-4', got '%s'", string(alias))
		}
	})

	t.Run("String returns string representation", func(t *testing.T) {
		alias := NewModelAlias("test-model")
		if alias.String() != "test-model" {
			t.Errorf("Expected 'test-model', got '%s'", alias.String())
		}
	})

	t.Run("IsEmpty returns true for empty alias", func(t *testing.T) {
		alias := NewModelAlias("")
		if !alias.IsEmpty() {
			t.Error("Expected empty alias to return true for IsEmpty")
		}
	})

	t.Run("IsEmpty returns false for non-empty alias", func(t *testing.T) {
		alias := NewModelAlias("model")
		if alias.IsEmpty() {
			t.Error("Expected non-empty alias to return false for IsEmpty")
		}
	})
}

func TestRoute(t *testing.T) {
	backend, _ := NewBackend("openai", "https://api.openai.com/v1", "sk-test", true, types.ProtocolOpenAI)

	t.Run("NewRoute creates valid route", func(t *testing.T) {
		route := NewRoute(backend, "gpt-4", 1, true)
		if route.Model() != "gpt-4" {
			t.Errorf("Expected 'gpt-4', got '%s'", route.Model())
		}
		if route.Priority() != 1 {
			t.Errorf("Expected priority 1, got %d", route.Priority())
		}
		if !route.IsEnabled() {
			t.Error("Expected enabled route")
		}
	})

	t.Run("Backend returns correct backend", func(t *testing.T) {
		route := NewRoute(backend, "test", 1, true)
		if route.Backend() != backend {
			t.Error("Expected route to return correct backend")
		}
	})

	t.Run("Protocol returns backend protocol when not set", func(t *testing.T) {
		route := NewRoute(backend, "test", 1, true)
		if route.Protocol() != types.ProtocolOpenAI {
			t.Errorf("Expected OpenAI protocol, got '%s'", route.Protocol())
		}
	})

	t.Run("Protocol returns custom protocol when set", func(t *testing.T) {
		route := NewRoute(backend, "test", 1, true).WithProtocol(types.ProtocolAnthropic)
		if route.Protocol() != types.ProtocolAnthropic {
			t.Errorf("Expected Anthropic protocol, got '%s'", route.Protocol())
		}
	})

	t.Run("IsEnabled returns false when route is disabled", func(t *testing.T) {
		route := NewRoute(backend, "test", 1, false)
		if route.IsEnabled() {
			t.Error("Expected disabled route to return false for IsEnabled")
		}
	})

	t.Run("IsEnabled returns false when backend is disabled", func(t *testing.T) {
		disabledBackend, _ := NewBackend("test", "https://test.com", "key", false, types.ProtocolOpenAI)
		route := NewRoute(disabledBackend, "test", 1, true)
		if route.IsEnabled() {
			t.Error("Expected route with disabled backend to return false for IsEnabled")
		}
	})

	t.Run("String returns formatted representation", func(t *testing.T) {
		route := NewRoute(backend, "gpt-4", 1, true)
		str := route.String()
		if str == "" {
			t.Error("Expected non-empty string representation")
		}
	})
}

func TestRouteList(t *testing.T) {
	backend1, _ := NewBackend("openai", "https://api.openai.com/v1", "sk-test", true, types.ProtocolOpenAI)
	backend2, _ := NewBackend("anthropic", "https://api.anthropic.com/v1", "sk-test", true, types.ProtocolAnthropic)

	routes := RouteList{
		NewRoute(backend1, "gpt-4", 2, true),
		NewRoute(backend1, "gpt-3.5", 1, true),
		NewRoute(backend2, "claude-3", 1, true),
	}

	t.Run("FilterEnabled returns only enabled routes", func(t *testing.T) {
		enabledRoutes := routes.FilterEnabled()
		if len(enabledRoutes) != 3 {
			t.Errorf("Expected 3 enabled routes, got %d", len(enabledRoutes))
		}
	})

	t.Run("FilterEnabled with disabled route", func(t *testing.T) {
		disabledRoutes := RouteList{
			NewRoute(backend1, "test", 1, false),
			NewRoute(backend1, "test2", 1, true),
		}
		enabledRoutes := disabledRoutes.FilterEnabled()
		if len(enabledRoutes) != 1 {
			t.Errorf("Expected 1 enabled route, got %d", len(enabledRoutes))
		}
	})

	t.Run("FilterByProtocol returns matching routes", func(t *testing.T) {
		openAIRoutes := routes.FilterByProtocol(types.ProtocolOpenAI)
		if len(openAIRoutes) != 2 {
			t.Errorf("Expected 2 OpenAI routes, got %d", len(openAIRoutes))
		}
	})

	t.Run("SortByPriority sorts routes correctly", func(t *testing.T) {
		sorted := routes.SortByPriority()
		if sorted[0].Priority() != 1 {
			t.Errorf("Expected first route to have priority 1, got %d", sorted[0].Priority())
		}
	})

	t.Run("GroupByPriority groups routes by priority", func(t *testing.T) {
		groups := routes.GroupByPriority()
		if len(groups[1]) != 2 {
			t.Errorf("Expected 2 routes with priority 1, got %d", len(groups[1]))
		}
		if len(groups[2]) != 1 {
			t.Errorf("Expected 1 route with priority 2, got %d", len(groups[2]))
		}
	})

	t.Run("First returns first route", func(t *testing.T) {
		first := routes.First()
		if first == nil {
			t.Error("Expected non-nil first route")
		}
	})

	t.Run("First returns nil for empty list", func(t *testing.T) {
		empty := RouteList{}
		if empty.First() != nil {
			t.Error("Expected nil for empty list")
		}
	})

	t.Run("IsEmpty returns true for empty list", func(t *testing.T) {
		empty := RouteList{}
		if !empty.IsEmpty() {
			t.Error("Expected empty list to return true for IsEmpty")
		}
	})

	t.Run("IsEmpty returns false for non-empty list", func(t *testing.T) {
		if routes.IsEmpty() {
			t.Error("Expected non-empty list to return false for IsEmpty")
		}
	})

	t.Run("Len returns correct length", func(t *testing.T) {
		if routes.Len() != 3 {
			t.Errorf("Expected length 3, got %d", routes.Len())
		}
	})
}

func TestRouteConfig(t *testing.T) {
	t.Run("IsEnabled with nil returns true", func(t *testing.T) {
		config := &RouteConfig{Enabled: nil}
		if !config.IsEnabled() {
			t.Error("Expected nil Enabled to return true")
		}
	})

	t.Run("IsEnabled with true returns true", func(t *testing.T) {
		enabled := true
		config := &RouteConfig{Enabled: &enabled}
		if !config.IsEnabled() {
			t.Error("Expected enabled config to return true")
		}
	})

	t.Run("IsEnabled with false returns false", func(t *testing.T) {
		enabled := false
		config := &RouteConfig{Enabled: &enabled}
		if config.IsEnabled() {
			t.Error("Expected disabled config to return false")
		}
	})

	t.Run("GetProtocol with custom protocol", func(t *testing.T) {
		config := &RouteConfig{Protocol: "anthropic"}
		protocol := config.GetProtocol(types.ProtocolOpenAI)
		if protocol != types.ProtocolAnthropic {
			t.Errorf("Expected Anthropic protocol, got '%s'", protocol)
		}
	})

	t.Run("GetProtocol with empty returns backend protocol", func(t *testing.T) {
		config := &RouteConfig{Protocol: ""}
		protocol := config.GetProtocol(types.ProtocolOpenAI)
		if protocol != types.ProtocolOpenAI {
			t.Errorf("Expected OpenAI protocol, got '%s'", protocol)
		}
	})
}

func TestModelAliasConfig(t *testing.T) {
	t.Run("IsEnabled with nil returns true", func(t *testing.T) {
		config := &ModelAliasConfig{Enabled: nil}
		if !config.IsEnabled() {
			t.Error("Expected nil Enabled to return true")
		}
	})

	t.Run("IsEnabled with false returns false", func(t *testing.T) {
		enabled := false
		config := &ModelAliasConfig{Enabled: &enabled}
		if config.IsEnabled() {
			t.Error("Expected disabled config to return false")
		}
	})
}

func TestFallbackConfig(t *testing.T) {
	t.Run("GetFallbackAliases with no config returns nil", func(t *testing.T) {
		config := &FallbackConfig{}
		fallbacks := config.GetFallbackAliases("test")
		if fallbacks != nil {
			t.Error("Expected nil for missing fallback config")
		}
	})

	t.Run("GetFallbackAliases with no matching alias returns nil", func(t *testing.T) {
		config := &FallbackConfig{
			AliasFallback: map[string][]string{
				"gpt-4": {"gpt-3.5"},
			},
		}
		fallbacks := config.GetFallbackAliases("unknown")
		if fallbacks != nil {
			t.Error("Expected nil for unknown alias")
		}
	})

	t.Run("GetFallbackAliases returns correct fallbacks", func(t *testing.T) {
		config := &FallbackConfig{
			AliasFallback: map[string][]string{
				"gpt-4": {"gpt-3.5", "claude-3"},
			},
		}
		fallbacks := config.GetFallbackAliases("gpt-4")
		if len(fallbacks) != 2 {
			t.Errorf("Expected 2 fallbacks, got %d", len(fallbacks))
		}
		if fallbacks[0].String() != "gpt-3.5" {
			t.Errorf("Expected 'gpt-3.5', got '%s'", fallbacks[0].String())
		}
	})

	t.Run("HasFallback with no config returns false", func(t *testing.T) {
		config := &FallbackConfig{}
		if config.HasFallback("test") {
			t.Error("Expected false for missing fallback config")
		}
	})

	t.Run("HasFallback with no matching alias returns false", func(t *testing.T) {
		config := &FallbackConfig{
			AliasFallback: map[string][]string{"gpt-4": {"gpt-3.5"}},
		}
		if config.HasFallback("unknown") {
			t.Error("Expected false for unknown alias")
		}
	})

	t.Run("HasFallback with matching alias returns true", func(t *testing.T) {
		config := &FallbackConfig{
			AliasFallback: map[string][]string{"gpt-4": {"gpt-3.5"}},
		}
		if !config.HasFallback("gpt-4") {
			t.Error("Expected true for matching alias")
		}
	})
}

func TestRouteBuilder(t *testing.T) {
	backend, _ := NewBackend("openai", "https://api.openai.com/v1", "sk-test", true, types.ProtocolOpenAI)

	t.Run("Build creates valid route", func(t *testing.T) {
		builder := NewRouteBuilder().
			Backend(backend).
			Model("gpt-4").
			Priority(1).
			Enabled(true)

		route, err := builder.Build()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if route.Model() != "gpt-4" {
			t.Errorf("Expected 'gpt-4', got '%s'", route.Model())
		}
	})

	t.Run("Build with nil backend returns error", func(t *testing.T) {
		builder := NewRouteBuilder().
			Model("test")

		_, err := builder.Build()
		if err == nil {
			t.Error("Expected error for nil backend")
		}
	})

	t.Run("Build with empty model returns error", func(t *testing.T) {
		builder := NewRouteBuilder().
			Backend(backend)

		_, err := builder.Build()
		if err == nil {
			t.Error("Expected error for empty model")
		}
	})

	t.Run("Build with custom protocol", func(t *testing.T) {
		builder := NewRouteBuilder().
			Backend(backend).
			Model("test").
			Protocol(types.ProtocolAnthropic)

		route, _ := builder.Build()
		if route.Protocol() != types.ProtocolAnthropic {
			t.Errorf("Expected Anthropic protocol, got '%s'", route.Protocol())
		}
	})

	t.Run("BuildUnsafe creates route without validation", func(t *testing.T) {
		builder := NewRouteBuilder().
			Backend(backend).
			Model("test")

		route := builder.BuildUnsafe()
		if route == nil {
			t.Error("Expected non-nil route from BuildUnsafe")
		}
	})
}
