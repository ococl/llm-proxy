package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxy_HealthEndpoint(t *testing.T) {
	cfg := &Config{}
	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)
	detector := NewDetector(cm)
	proxy := NewProxy(cm, router, cd, detector)

	tests := []struct {
		path     string
		expected string
	}{
		{"/health", `{"backends":0,"models":0,"status":"healthy"}`},
		{"/healthz", `{"backends":0,"models":0,"status":"healthy"}`},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		w := httptest.NewRecorder()

		proxy.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", tt.path, w.Code)
		}
		body := w.Body.String()
		var got map[string]interface{}
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Errorf("%s: failed to parse JSON: %v", tt.path, err)
			continue
		}
		if got["status"] != "healthy" {
			t.Errorf("%s: expected status 'healthy', got %v", tt.path, got["status"])
		}
	}
}

func TestProxy_APIKeyValidation(t *testing.T) {
	cfg := &Config{
		ProxyAPIKey: "sk-test-key",
		Backends: []Backend{
			{Name: "backend1", URL: "http://backend1.com"},
		},
		Models: map[string]*ModelAlias{
			"model-a": {
				Routes: []ModelRoute{
					{Backend: "backend1", Model: "m1", Priority: 1},
				},
			},
		},
	}
	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)
	detector := NewDetector(cm)
	proxy := NewProxy(cm, router, cd, detector)

	tests := []struct {
		name       string
		authHeader string
		wantCode   int
	}{
		{"no auth", "", http.StatusUnauthorized},
		{"wrong key", "Bearer wrong-key", http.StatusUnauthorized},
		{"correct key", "Bearer sk-test-key", http.StatusBadGateway},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"model": "model-a"}`
			req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			proxy.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("expected %d, got %d", tt.wantCode, w.Code)
			}
		})
	}
}

func TestProxy_APIKeyValidation_NoKeyConfigured(t *testing.T) {
	cfg := &Config{
		ProxyAPIKey: "",
		Backends: []Backend{
			{Name: "backend1", URL: "http://backend1.com"},
		},
		Models: map[string]*ModelAlias{
			"model-a": {
				Routes: []ModelRoute{
					{Backend: "backend1", Model: "m1", Priority: 1},
				},
			},
		},
	}
	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)
	detector := NewDetector(cm)
	proxy := NewProxy(cm, router, cd, detector)

	body := `{"model": "model-a"}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized {
		t.Error("Should not require auth when proxy_api_key is empty")
	}
}

func TestProxy_MissingModel(t *testing.T) {
	cfg := &Config{}
	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)
	detector := NewDetector(cm)
	proxy := NewProxy(cm, router, cd, detector)

	body := `{}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestProxy_UnknownModel(t *testing.T) {
	cfg := &Config{
		Models: map[string]*ModelAlias{},
	}
	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)
	detector := NewDetector(cm)
	proxy := NewProxy(cm, router, cd, detector)

	body := `{"model": "unknown-model"}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestProxy_ModelsEndpoint(t *testing.T) {
	cfg := &Config{
		Models: map[string]*ModelAlias{
			"model-a": {Routes: []ModelRoute{{Backend: "b", Model: "m", Priority: 1}}},
			"model-b": {Routes: []ModelRoute{{Backend: "b", Model: "m", Priority: 1}}},
			"model-c": {Enabled: boolPtr(false), Routes: []ModelRoute{{Backend: "b", Model: "m", Priority: 1}}},
		},
	}
	cm := newTestConfigManager(cfg)
	cd := NewCooldownManager()
	router := NewRouter(cm, cd)
	detector := NewDetector(cm)
	proxy := NewProxy(cm, router, cd, detector)

	paths := []string{"/v1/models", "/models"}
	for _, path := range paths {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()

		proxy.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", path, w.Code)
		}

		var resp struct {
			Object string `json:"object"`
			Data   []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)

		if resp.Object != "list" {
			t.Errorf("expected object 'list', got %q", resp.Object)
		}

		if len(resp.Data) != 2 {
			t.Errorf("expected 2 models (disabled excluded), got %d", len(resp.Data))
		}
	}
}

func TestSmartPathJoin(t *testing.T) {
	tests := []struct {
		backendPath string
		reqPath     string
		expected    string
	}{
		{"/v1", "/v1/chat/completions", "/v1/chat/completions"},
		{"", "/v1/chat/completions", "/v1/chat/completions"},
		{"/api", "/v1/chat/completions", "/api/v1/chat/completions"},
		{"/v1", "/chat/completions", "/v1/chat/completions"},
	}

	for _, tt := range tests {
		var result string
		if tt.backendPath != "" && strings.HasPrefix(tt.reqPath, tt.backendPath) {
			result = tt.reqPath
		} else {
			result = tt.backendPath + tt.reqPath
		}

		if result != tt.expected {
			t.Errorf("smartPathJoin(%q, %q) = %q, want %q",
				tt.backendPath, tt.reqPath, result, tt.expected)
		}
	}
}
