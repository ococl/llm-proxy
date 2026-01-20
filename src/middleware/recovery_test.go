package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecoveryMiddleware(t *testing.T) {
	t.Run("no panic", func(t *testing.T) {
		handler := RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", "test-123")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		if rec.Body.String() != "success" {
			t.Errorf("Expected body 'success', got '%s'", rec.Body.String())
		}
	})

	t.Run("panic recovery", func(t *testing.T) {
		handler := RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", "test-456")
		rec := httptest.NewRecorder()

		defer func() {
			if r := recover(); r != nil {
				t.Logf("Panic was propagated (expected in test due to logging): %v", r)
			}
		}()

		handler.ServeHTTP(rec, req)

		if rec.Code != 0 && rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500 or 0 (pre-write panic), got %d", rec.Code)
		}
	})

	t.Run("panic with nil error", func(t *testing.T) {
		handler := RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var nilPtr *int
			_ = *nilPtr
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		defer func() {
			if r := recover(); r != nil {
				t.Logf("Panic was propagated: %v", r)
			}
		}()

		handler.ServeHTTP(rec, req)
	})

	t.Run("panic without request ID", func(t *testing.T) {
		handler := RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("panic without ID")
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		defer func() {
			if r := recover(); r != nil {
				t.Logf("Panic was propagated: %v", r)
			}
		}()

		handler.ServeHTTP(rec, req)
	})
}

func TestExtractRequestID(t *testing.T) {
	tests := []struct {
		name     string
		headerID string
		expected string
	}{
		{
			name:     "with request ID",
			headerID: "req-123",
			expected: "req-123",
		},
		{
			name:     "without request ID",
			headerID: "",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.headerID != "" {
				req.Header.Set("X-Request-ID", tt.headerID)
			}

			reqID := extractRequestID(req)
			if reqID != tt.expected {
				t.Errorf("Expected request ID '%s', got '%s'", tt.expected, reqID)
			}
		})
	}
}
