package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"llm-proxy/backend"
	"llm-proxy/config"
)

// TestFallbackOn500Error verifies that when a backend returns 500, the proxy falls back to the next backend
func TestFallbackOn500Error(t *testing.T) {
	// Create two backends - first returns 500, second returns 200
	var backend1Called bool
	var backend2Called bool

	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backend1Called = true
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backend2Called = true
		w.WriteHeader(http.StatusOK)
		response := `{"id": "chatcmpl-abc123", "object": "chat.completion", "choices": [{"message": {"content": "Hello"}}]}`
		w.Write([]byte(response))
	}))
	defer backend2.Close()

	cfg := &config.Config{
		Backends: []config.Backend{
			{Name: "backend1", URL: backend1.URL},
			{Name: "backend2", URL: backend2.URL},
		},
		Models: map[string]*config.ModelAlias{
			"test-model": {
				Routes: []config.ModelRoute{
					{Backend: "backend1", Model: "real-model-1", Priority: 1},
					{Backend: "backend2", Model: "real-model-2", Priority: 2},
				},
			},
		},
		Fallback: config.Fallback{
			CooldownSeconds: 60,
			MaxRetries:      3,
		},
		Detection: config.Detection{
			ErrorCodes:    []string{"4xx", "5xx"},
			ErrorPatterns: []string{"insufficient_quota", "rate_limit"},
		},
	}

	cm := newTestManager(cfg)
	cd := backend.NewCooldownManager()
	router := NewRouter(cm, cd)
	detector := NewDetector(cm)
	p := NewProxy(cm, router, cd, detector)

	// Create a request
	reqBody := `{"model": "test-model", "messages": [{"role": "user", "content": "Hello"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	// Verify that backend1 was called (returned 500)
	if !backend1Called {
		t.Error("Backend1 should have been called")
	}

	// Verify that backend2 was called (fallback triggered)
	if !backend2Called {
		t.Error("Backend2 should have been called (fallback should have been triggered)")
	}

	// Verify the final response is from backend2 (200)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 from backend2, got %d", w.Code)
	}

	// Verify response contains the expected data from backend2
	response := w.Body.String()
	if !strings.Contains(response, "chatcmpl-abc123") {
		t.Errorf("Expected response from backend2, got: %s", response)
	}

	t.Logf("Test passed: Fallback from 500 error triggered correctly")
	t.Logf("Backend1 called: %v, Backend2 called: %v", backend1Called, backend2Called)
}

// TestFallbackOn429Error verifies that when a backend returns 429, the proxy falls back to the next backend
func TestFallbackOn429Error(t *testing.T) {
	var backend1Called bool
	var backend2Called bool

	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backend1Called = true
		w.WriteHeader(http.StatusTooManyRequests)
		w.Header().Set("Retry-After", "60")
		w.Write([]byte(`{"error": {"message": "Rate limit exceeded"}}`))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backend2Called = true
		w.WriteHeader(http.StatusOK)
		response := `{"id": "chatcmpl-def456", "object": "chat.completion", "choices": [{"message": {"content": "World"}}]}`
		w.Write([]byte(response))
	}))
	defer backend2.Close()

	cfg := &config.Config{
		Backends: []config.Backend{
			{Name: "backend1", URL: backend1.URL},
			{Name: "backend2", URL: backend2.URL},
		},
		Models: map[string]*config.ModelAlias{
			"test-model": {
				Routes: []config.ModelRoute{
					{Backend: "backend1", Model: "real-model-1", Priority: 1},
					{Backend: "backend2", Model: "real-model-2", Priority: 2},
				},
			},
		},
		Fallback: config.Fallback{
			CooldownSeconds: 60,
			MaxRetries:      3,
		},
		Detection: config.Detection{
			ErrorCodes:    []string{"4xx", "5xx"},
			ErrorPatterns: []string{"insufficient_quota", "rate_limit"},
		},
	}

	cm := newTestManager(cfg)
	cd := backend.NewCooldownManager()
	router := NewRouter(cm, cd)
	detector := NewDetector(cm)
	p := NewProxy(cm, router, cd, detector)

	reqBody := `{"model": "test-model", "messages": [{"role": "user", "content": "Hello"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	if !backend1Called {
		t.Error("Backend1 should have been called")
	}

	if !backend2Called {
		t.Error("Backend2 should have been called (fallback should have been triggered)")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 from backend2, got %d", w.Code)
	}

	response := w.Body.String()
	if !strings.Contains(response, "chatcmpl-def456") {
		t.Errorf("Expected response from backend2, got: %s", response)
	}

	t.Logf("Test passed: Fallback from 429 error triggered correctly")
	t.Logf("Backend1 called: %v, Backend2 called: %v", backend1Called, backend2Called)
}

// TestNoFallbackOnSuccess verifies that when a backend returns 200, no fallback occurs
func TestNoFallbackOnSuccess(t *testing.T) {
	var backend1Called bool
	var backend2Called bool

	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backend1Called = true
		w.WriteHeader(http.StatusOK)
		response := `{"id": "chatcmpl-success", "object": "chat.completion", "choices": [{"message": {"content": "Success"}}]}`
		w.Write([]byte(response))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backend2Called = true
		w.WriteHeader(http.StatusOK)
		response := `{"id": "chatcmpl-fallback", "object": "chat.completion", "choices": [{"message": {"content": "Fallback"}}]}`
		w.Write([]byte(response))
	}))
	defer backend2.Close()

	cfg := &config.Config{
		Backends: []config.Backend{
			{Name: "backend1", URL: backend1.URL},
			{Name: "backend2", URL: backend2.URL},
		},
		Models: map[string]*config.ModelAlias{
			"test-model": {
				Routes: []config.ModelRoute{
					{Backend: "backend1", Model: "real-model-1", Priority: 1},
					{Backend: "backend2", Model: "real-model-2", Priority: 2},
				},
			},
		},
		Fallback: config.Fallback{
			CooldownSeconds: 60,
			MaxRetries:      3,
		},
		Detection: config.Detection{
			ErrorCodes:    []string{"4xx", "5xx"},
			ErrorPatterns: []string{"insufficient_quota", "rate_limit"},
		},
	}

	cm := newTestManager(cfg)
	cd := backend.NewCooldownManager()
	router := NewRouter(cm, cd)
	detector := NewDetector(cm)
	p := NewProxy(cm, router, cd, detector)

	reqBody := `{"model": "test-model", "messages": [{"role": "user", "content": "Hello"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	if !backend1Called {
		t.Error("Backend1 should have been called")
	}

	// Backend2 should NOT be called when backend1 succeeds
	if backend2Called {
		t.Error("Backend2 should NOT have been called (no fallback needed)")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 from backend1, got %d", w.Code)
	}

	response := w.Body.String()
	if !strings.Contains(response, "chatcmpl-success") {
		t.Errorf("Expected response from backend1, got: %s", response)
	}

	t.Logf("Test passed: No fallback when backend succeeds")
	t.Logf("Backend1 called: %v, Backend2 called: %v", backend1Called, backend2Called)
}

// TestFallbackExhaustedAllBackends verifies behavior when all backends fail
func TestFallbackExhaustedAllBackends(t *testing.T) {
	var backend1Called bool
	var backend2Called bool

	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backend1Called = true
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backend2Called = true
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error": "service unavailable"}`))
	}))
	defer backend2.Close()

	cfg := &config.Config{
		Backends: []config.Backend{
			{Name: "backend1", URL: backend1.URL},
			{Name: "backend2", URL: backend2.URL},
		},
		Models: map[string]*config.ModelAlias{
			"test-model": {
				Routes: []config.ModelRoute{
					{Backend: "backend1", Model: "real-model-1", Priority: 1},
					{Backend: "backend2", Model: "real-model-2", Priority: 2},
				},
			},
		},
		Fallback: config.Fallback{
			CooldownSeconds: 60,
			MaxRetries:      3,
		},
		Detection: config.Detection{
			ErrorCodes:    []string{"4xx", "5xx"},
			ErrorPatterns: []string{"insufficient_quota", "rate_limit"},
		},
	}

	cm := newTestManager(cfg)
	cd := backend.NewCooldownManager()
	router := NewRouter(cm, cd)
	detector := NewDetector(cm)
	p := NewProxy(cm, router, cd, detector)

	reqBody := `{"model": "test-model", "messages": [{"role": "user", "content": "Hello"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	if !backend1Called {
		t.Error("Backend1 should have been called")
	}

	if !backend2Called {
		t.Error("Backend2 should have been called (all backends should be tried)")
	}

	// When all backends fail, the proxy should return the last error status
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 from last backend, got %d", w.Code)
	}

	t.Logf("Test passed: All backends exhausted correctly")
	t.Logf("Backend1 called: %v, Backend2 called: %v", backend1Called, backend2Called)
}

// Helper function to unmarshal JSON for verification
func unmarshalJSON(t *testing.T, data string) map[string]interface{} {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	return result
}
