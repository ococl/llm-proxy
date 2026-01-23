package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"llm-proxy/infrastructure/config"
)

func TestNewConcurrencyLimiter(t *testing.T) {
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
				Concurrency: config.ConcurrencyConfig{
					Enabled:     tt.enabled,
					MaxRequests: 10,
				},
			}
			mgr := &config.Manager{}
			mgr.SetConfigForTest(cfg)

			cl := NewConcurrencyLimiter(mgr)
			if cl == nil {
				t.Fatal("NewConcurrencyLimiter returned nil")
			}
			if tt.enabled && cl.global == nil {
				t.Error("Expected global channel to be initialized when enabled")
			}
			if !tt.enabled && cl.global != nil {
				t.Error("Expected global channel to be nil when disabled")
			}
		})
	}
}

func TestConcurrencyLimiter_Acquire(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		cfg := &config.Config{
			Concurrency: config.ConcurrencyConfig{
				Enabled: false,
			},
		}
		mgr := &config.Manager{}
		mgr.SetConfigForTest(cfg)

		cl := NewConcurrencyLimiter(mgr)
		ctx := context.Background()
		if err := cl.Acquire(ctx); err != nil {
			t.Errorf("Expected Acquire to succeed when disabled, got error: %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		cfg := &config.Config{
			Concurrency: config.ConcurrencyConfig{
				Enabled:     true,
				MaxRequests: 2,
			},
		}
		mgr := &config.Manager{}
		mgr.SetConfigForTest(cfg)

		cl := NewConcurrencyLimiter(mgr)
		ctx := context.Background()

		if err := cl.Acquire(ctx); err != nil {
			t.Errorf("First Acquire failed: %v", err)
		}
		if err := cl.Acquire(ctx); err != nil {
			t.Errorf("Second Acquire failed: %v", err)
		}

		cl.Release()
		cl.Release()
	})

	t.Run("timeout", func(t *testing.T) {
		cfg := &config.Config{
			Concurrency: config.ConcurrencyConfig{
				Enabled:     true,
				MaxRequests: 1,
			},
		}
		mgr := &config.Manager{}
		mgr.SetConfigForTest(cfg)

		cl := NewConcurrencyLimiter(mgr)

		ctx1 := context.Background()
		if err := cl.Acquire(ctx1); err != nil {
			t.Fatalf("First Acquire failed: %v", err)
		}

		ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		if err := cl.Acquire(ctx2); err == nil {
			t.Error("Expected Acquire to timeout")
		}

		cl.Release()
	})

	t.Run("queue overflow", func(t *testing.T) {
		cfg := &config.Config{
			Concurrency: config.ConcurrencyConfig{
				Enabled:      true,
				MaxRequests:  1,
				MaxQueueSize: 0,
			},
		}
		mgr := &config.Manager{}
		mgr.SetConfigForTest(cfg)

		cl := NewConcurrencyLimiter(mgr)
		ctx1 := context.Background()
		if err := cl.Acquire(ctx1); err != nil {
			t.Fatalf("First Acquire failed: %v", err)
		}

		ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		err := cl.Acquire(ctx2)
		if err == nil {
			t.Error("Expected queue overflow error, got nil")
			cl.Release()
		} else if err != context.DeadlineExceeded {
			t.Logf("Got error (expected): %v", err)
		}

		cl.Release()
	})
}

func TestConcurrencyLimiter_Release(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		cfg := &config.Config{
			Concurrency: config.ConcurrencyConfig{
				Enabled: false,
			},
		}
		mgr := &config.Manager{}
		mgr.SetConfigForTest(cfg)

		cl := NewConcurrencyLimiter(mgr)
		cl.Release()
	})

	t.Run("enabled", func(t *testing.T) {
		cfg := &config.Config{
			Concurrency: config.ConcurrencyConfig{
				Enabled:     true,
				MaxRequests: 1,
			},
		}
		mgr := &config.Manager{}
		mgr.SetConfigForTest(cfg)

		cl := NewConcurrencyLimiter(mgr)
		ctx := context.Background()
		if err := cl.Acquire(ctx); err != nil {
			t.Fatalf("Acquire failed: %v", err)
		}

		cl.Release()

		if err := cl.Acquire(ctx); err != nil {
			t.Error("Expected Acquire to succeed after Release")
		}
		cl.Release()
	})
}

func TestConcurrencyLimiter_Middleware(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		cfg := &config.Config{
			Concurrency: config.ConcurrencyConfig{
				Enabled: false,
			},
		}
		mgr := &config.Manager{}
		mgr.SetConfigForTest(cfg)

		cl := NewConcurrencyLimiter(mgr)
		handler := cl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("concurrency limit", func(t *testing.T) {
		cfg := &config.Config{
			Concurrency: config.ConcurrencyConfig{
				Enabled:      true,
				MaxRequests:  1,
				QueueTimeout: 50 * time.Millisecond,
			},
		}
		mgr := &config.Manager{}
		mgr.SetConfigForTest(cfg)

		cl := NewConcurrencyLimiter(mgr)

		slowHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		})
		handler := cl.Middleware(slowHandler)

		var wg sync.WaitGroup
		results := make([]int, 2)

		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				results[idx] = rec.Code
			}(i)
			time.Sleep(10 * time.Millisecond)
		}

		wg.Wait()

		successCount := 0
		for _, code := range results {
			if code == http.StatusOK {
				successCount++
			}
		}

		if successCount != 1 {
			t.Errorf("Expected 1 successful request, got %d (results: %v)", successCount, results)
		}
	})
}

func TestConcurrencyLimiter_Concurrent(t *testing.T) {
	cfg := &config.Config{
		Concurrency: config.ConcurrencyConfig{
			Enabled:     true,
			MaxRequests: 50,
		},
	}
	mgr := &config.Manager{}
	mgr.SetConfigForTest(cfg)

	cl := NewConcurrencyLimiter(mgr)
	ctx := context.Background()

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := cl.Acquire(ctx); err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
				time.Sleep(1 * time.Millisecond)
				cl.Release()
			}
		}()
	}

	wg.Wait()

	if successCount == 0 {
		t.Error("Expected some requests to succeed")
	}
}
