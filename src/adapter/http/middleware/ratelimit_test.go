package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"llm-proxy/infrastructure/config"
)

func TestNewRateLimiter(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{"enabled", true},
		{"disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				RateLimit: config.RateLimitConfig{
					Enabled:   tt.enabled,
					GlobalRPS: 100.0,
				},
			}
			mgr := &config.Manager{}
			mgr.SetConfigForTest(cfg)

			rl := NewRateLimiter(mgr)
			if rl == nil {
				t.Fatal("NewRateLimiter returned nil")
			}
			if tt.enabled && rl.global == nil {
				t.Error("Expected global limiter to be initialized when enabled")
			}
			if !tt.enabled && rl.global != nil {
				t.Error("Expected global limiter to be nil when disabled")
			}
		})
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		cfg := &config.Config{
			RateLimit: config.RateLimitConfig{
				Enabled: false,
			},
		}
		mgr := &config.Manager{}
		mgr.SetConfigForTest(cfg)

		rl := NewRateLimiter(mgr)
		if !rl.Allow("192.168.1.1", "gpt-4") {
			t.Error("Expected Allow to return true when disabled")
		}
	})

	t.Run("global limit", func(t *testing.T) {
		cfg := &config.Config{
			RateLimit: config.RateLimitConfig{
				Enabled:     true,
				GlobalRPS:   2.0,
				BurstFactor: 1.0,
			},
		}
		mgr := &config.Manager{}
		mgr.SetConfigForTest(cfg)

		rl := NewRateLimiter(mgr)
		if !rl.Allow("", "") {
			t.Error("First request should be allowed")
		}
		if !rl.Allow("", "") {
			t.Error("Second request should be allowed")
		}
		if rl.Allow("", "") {
			t.Error("Third request should be rate limited")
		}
	})

	t.Run("per IP limit", func(t *testing.T) {
		cfg := &config.Config{
			RateLimit: config.RateLimitConfig{
				Enabled:  true,
				PerIPRPS: 1.0,
			},
		}
		mgr := &config.Manager{}
		mgr.SetConfigForTest(cfg)

		rl := NewRateLimiter(mgr)
		ip := "192.168.1.1"
		if !rl.Allow(ip, "") {
			t.Error("First request should be allowed")
		}
		if rl.Allow(ip, "") {
			t.Error("Second request should be rate limited")
		}
	})

	t.Run("per model limit", func(t *testing.T) {
		cfg := &config.Config{
			RateLimit: config.RateLimitConfig{
				Enabled: true,
				PerModelRPS: map[string]float64{
					"gpt-4": 1.0,
				},
			},
		}
		mgr := &config.Manager{}
		mgr.SetConfigForTest(cfg)

		rl := NewRateLimiter(mgr)
		if !rl.Allow("", "gpt-4") {
			t.Error("First request should be allowed")
		}
		if rl.Allow("", "gpt-4") {
			t.Error("Second request should be rate limited")
		}
	})
}

func TestRateLimiter_getIPLimiter(t *testing.T) {
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:  true,
			PerIPRPS: 100.0,
		},
	}
	mgr := &config.Manager{}
	mgr.SetConfigForTest(cfg)

	rl := NewRateLimiter(mgr)
	ip := "192.168.1.1"

	limiter1 := rl.getIPLimiter(ip)
	if limiter1 == nil {
		t.Fatal("getIPLimiter returned nil")
	}

	limiter2 := rl.getIPLimiter(ip)
	if limiter1 != limiter2 {
		t.Error("getIPLimiter should return the same instance for same IP")
	}

	limiter3 := rl.getIPLimiter("192.168.1.2")
	if limiter1 == limiter3 {
		t.Error("getIPLimiter should return different instances for different IPs")
	}
}

func TestRateLimiter_getModelLimiter(t *testing.T) {
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:   true,
			GlobalRPS: 100.0,
			PerModelRPS: map[string]float64{
				"gpt-4": 50.0,
			},
		},
	}
	mgr := &config.Manager{}
	mgr.SetConfigForTest(cfg)

	rl := NewRateLimiter(mgr)

	limiter1 := rl.getModelLimiter("gpt-4")
	if limiter1 == nil {
		t.Fatal("getModelLimiter returned nil")
	}

	limiter2 := rl.getModelLimiter("claude")
	if limiter2 == nil {
		t.Fatal("getModelLimiter returned nil for unconfigured model")
	}

	limiter3 := rl.getModelLimiter("gpt-4")
	if limiter1 != limiter3 {
		t.Error("getModelLimiter should return the same instance for same model")
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		cfg := &config.Config{
			RateLimit: config.RateLimitConfig{
				Enabled: false,
			},
		}
		mgr := &config.Manager{}
		mgr.SetConfigForTest(cfg)

		rl := NewRateLimiter(mgr)
		handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("rate limited", func(t *testing.T) {
		cfg := &config.Config{
			RateLimit: config.RateLimitConfig{
				Enabled:   true,
				GlobalRPS: 1.0,
			},
		}
		mgr := &config.Manager{}
		mgr.SetConfigForTest(cfg)

		rl := NewRateLimiter(mgr)
		handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req1 := httptest.NewRequest("POST", "/v1/chat/completions", nil)
		rec1 := httptest.NewRecorder()
		handler.ServeHTTP(rec1, req1)
		if rec1.Code != http.StatusOK {
			t.Errorf("First request: expected status 200, got %d", rec1.Code)
		}

		req2 := httptest.NewRequest("POST", "/v1/chat/completions", nil)
		rec2 := httptest.NewRecorder()
		handler.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusTooManyRequests {
			t.Errorf("Second request: expected status 429, got %d", rec2.Code)
		}
	})

	t.Run("parse model from body", func(t *testing.T) {
		cfg := &config.Config{
			RateLimit: config.RateLimitConfig{
				Enabled: true,
				PerModelRPS: map[string]float64{
					"gpt-4": 1.0,
				},
			},
		}
		mgr := &config.Manager{}
		mgr.SetConfigForTest(cfg)

		rl := NewRateLimiter(mgr)
		handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		body := map[string]interface{}{
			"model": "gpt-4",
			"messages": []interface{}{
				map[string]string{"role": "user", "content": "test"},
			},
		}
		bodyBytes, _ := json.Marshal(body)

		req1 := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyBytes))
		rec1 := httptest.NewRecorder()
		handler.ServeHTTP(rec1, req1)
		if rec1.Code != http.StatusOK {
			t.Errorf("First request: expected status 200, got %d", rec1.Code)
		}

		req2 := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyBytes))
		rec2 := httptest.NewRecorder()
		handler.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusTooManyRequests {
			t.Errorf("Second request: expected status 429, got %d", rec2.Code)
		}
	})
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expected   string
	}{
		{
			name:     "X-Forwarded-For",
			headers:  map[string]string{"X-Forwarded-For": "192.168.1.1"},
			expected: "192.168.1.1",
		},
		{
			name:     "X-Real-IP",
			headers:  map[string]string{"X-Real-IP": "192.168.1.2"},
			expected: "192.168.1.2",
		},
		{
			name:       "RemoteAddr",
			remoteAddr: "192.168.1.3:12345",
			expected:   "192.168.1.3",
		},
		{
			name:     "X-Forwarded-For priority",
			headers:  map[string]string{"X-Forwarded-For": "192.168.1.1", "X-Real-IP": "192.168.1.2"},
			expected: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if tt.remoteAddr != "" {
				req.RemoteAddr = tt.remoteAddr
			}

			ip := ExtractIP(req)
			if ip != tt.expected {
				t.Errorf("Expected IP %s, got %s", tt.expected, ip)
			}
		})
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:   true,
			GlobalRPS: 1000.0,
			PerIPRPS:  100.0,
		},
	}
	mgr := &config.Manager{}
	mgr.SetConfigForTest(cfg)

	rl := NewRateLimiter(mgr)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ip := "192.168.1." + string(rune(idx%10+'0'))
			model := "gpt-4"
			rl.Allow(ip, model)
			rl.getIPLimiter(ip)
			rl.getModelLimiter(model)
		}(i)
	}
	wg.Wait()
}

func TestRateLimiter_BurstFactor(t *testing.T) {
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:     true,
			GlobalRPS:   10.0,
			BurstFactor: 2.0,
		},
	}
	mgr := &config.Manager{}
	mgr.SetConfigForTest(cfg)

	rl := NewRateLimiter(mgr)
	if rl.global == nil {
		t.Fatal("global limiter not initialized")
	}

	allowedCount := 0
	for i := 0; i < 25; i++ {
		if rl.Allow("", "") {
			allowedCount++
		}
	}

	if allowedCount < 20 {
		t.Errorf("Expected at least 20 allowed requests with burst factor 2.0, got %d", allowedCount)
	}
}
