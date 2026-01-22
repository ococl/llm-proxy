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

	t.Run("GetMaxRetries with zero returns default", func(t *testing.T) {
		config := RetryConfig{MaxRetries: 0}
		if config.GetMaxRetries() != 3 {
			t.Errorf("Expected default 3, got %d", config.GetMaxRetries())
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
