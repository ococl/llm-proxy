package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"llm-proxy/domain/entity"
	domainerror "llm-proxy/domain/error"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// MockLogger for testing
type MockLogger struct {
	lastError error
	lastWarn  error
	lastMsg   string
}

func (m *MockLogger) Debug(msg string, fields ...port.Field) {}
func (m *MockLogger) Info(msg string, fields ...port.Field)  {}
func (m *MockLogger) Warn(msg string, fields ...port.Field) {
	m.lastWarn = extractError(fields)
}
func (m *MockLogger) Error(msg string, fields ...port.Field) {
	m.lastError = extractError(fields)
	m.lastMsg = msg
}
func (m *MockLogger) Fatal(msg string, fields ...port.Field)  {}
func (m *MockLogger) With(fields ...port.Field) port.Logger   { return m }
func (m *MockLogger) LogRequest(reqID string, content string) {}
func (m *MockLogger) LogError(reqID string, content string)   {}

func extractError(fields []port.Field) error {
	for _, f := range fields {
		if e, ok := f.Value.(error); ok {
			return e
		}
	}
	return nil
}

// MockConfigProvider for testing
type MockConfigProvider struct {
	backends []*entity.Backend
	models   map[string]*port.ModelAlias
}

func (m *MockConfigProvider) Get() *port.Config {
	return &port.Config{
		Backends: m.backends,
		Models:   m.models,
	}
}
func (m *MockConfigProvider) Watch() <-chan struct{}                      { return nil }
func (m *MockConfigProvider) GetBackend(name string) *entity.Backend      { return nil }
func (m *MockConfigProvider) GetModelAlias(alias string) *port.ModelAlias { return nil }

func TestHealthHandler_ServeHTTP(t *testing.T) {
	t.Run("Returns healthy status", func(t *testing.T) {
		logger := &MockLogger{}
		backend, _ := entity.NewBackend("test", "https://test.com", "test-key", true, "")
		configProvider := &MockConfigProvider{
			backends: []*entity.Backend{backend},
			models: map[string]*port.ModelAlias{
				"gpt-4": {Enabled: true},
			},
		}

		handler := NewHealthHandler(configProvider, logger)
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		var status HealthStatus
		if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
			t.Errorf("Failed to unmarshal response: %v", err)
		}

		if status.Status != "healthy" {
			t.Errorf("Expected status 'healthy', got '%s'", status.Status)
		}
		if status.Backends != 1 {
			t.Errorf("Expected 1 backend, got %d", status.Backends)
		}
		if status.Models != 1 {
			t.Errorf("Expected 1 model, got %d", status.Models)
		}
	})

	t.Run("Returns correct content type", func(t *testing.T) {
		logger := &MockLogger{}
		backend, _ := entity.NewBackend("test", "https://test.com", "test-key", true, "")
		configProvider := &MockConfigProvider{
			backends: []*entity.Backend{backend},
			models:   map[string]*port.ModelAlias{},
		}

		handler := NewHealthHandler(configProvider, logger)
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Header().Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got '%s'", rec.Header().Get("Content-Type"))
		}
	})
}

func TestHealthHandler_IsHealthy(t *testing.T) {
	t.Run("Returns true when enabled backends exist", func(t *testing.T) {
		logger := &MockLogger{}
		backend, _ := entity.NewBackend("test", "https://test.com", "test-key", true, "")
		configProvider := &MockConfigProvider{
			backends: []*entity.Backend{backend},
		}

		handler := NewHealthHandler(configProvider, logger)
		if !handler.IsHealthy() {
			t.Error("Expected IsHealthy to return true")
		}
	})

	t.Run("Returns false when no backends", func(t *testing.T) {
		logger := &MockLogger{}
		configProvider := &MockConfigProvider{
			backends: []*entity.Backend{},
		}

		handler := NewHealthHandler(configProvider, logger)
		if handler.IsHealthy() {
			t.Error("Expected IsHealthy to return false")
		}
	})

	t.Run("Returns false when all backends disabled", func(t *testing.T) {
		logger := &MockLogger{}
		backend, _ := entity.NewBackend("test", "https://test.com", "test-key", false, "")
		configProvider := &MockConfigProvider{
			backends: []*entity.Backend{backend},
		}

		handler := NewHealthHandler(configProvider, logger)
		if handler.IsHealthy() {
			t.Error("Expected IsHealthy to return false")
		}
	})
}

func TestErrorPresenter_WriteJSONError(t *testing.T) {
	t.Run("Writes JSON error response", func(t *testing.T) {
		logger := &MockLogger{}
		presenter := NewErrorPresenter(logger)

		rec := httptest.NewRecorder()

		presenter.WriteJSONError(rec, "test error", 400, "")

		if rec.Code != 400 {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}

		if rec.Header().Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got '%s'", rec.Header().Get("Content-Type"))
		}
	})

	t.Run("Includes trace ID in response", func(t *testing.T) {
		logger := &MockLogger{}
		presenter := NewErrorPresenter(logger)

		rec := httptest.NewRecorder()

		presenter.WriteJSONError(rec, "test error", 400, "trace-123")

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Errorf("Failed to unmarshal response: %v", err)
		}

		errorObj, ok := response["error"].(map[string]interface{})
		if !ok {
			t.Error("Expected 'error' field in response")
		}

		if errorObj["req_id"] != "trace-123" {
			t.Errorf("Expected req_id 'trace-123', got '%v'", errorObj["req_id"])
		}
	})
}

func TestErrorPresenter_WriteError(t *testing.T) {
	t.Run("Handles LLMProxyError", func(t *testing.T) {
		logger := &MockLogger{}
		presenter := NewErrorPresenter(logger)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		err := domainerror.NewBadRequest("test validation failed")
		presenter.WriteError(rec, req, err)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("Handles generic error", func(t *testing.T) {
		logger := &MockLogger{}
		presenter := NewErrorPresenter(logger)

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		presenter.WriteError(rec, req, nil)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rec.Code)
		}
	})
}

func TestRecoveryMiddleware_Middleware(t *testing.T) {
	t.Run("Passes through normal request", func(t *testing.T) {
		logger := &MockLogger{}
		middleware := NewRecoveryMiddleware(logger)

		var called bool
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.Write([]byte("OK"))
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		middleware.Middleware(handler).ServeHTTP(rec, req)

		if !called {
			t.Error("Expected handler to be called")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("Recovers from panic", func(t *testing.T) {
		logger := &MockLogger{}
		middleware := NewRecoveryMiddleware(logger)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		middleware.Middleware(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rec.Code)
		}

		if logger.lastMsg == "" {
			t.Error("Expected error to be logged")
		}
	})

	t.Run("Extracts request ID from header", func(t *testing.T) {
		logger := &MockLogger{}
		middleware := NewRecoveryMiddleware(logger)

		var capturedReqID string
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedReqID = extractRequestID(r)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", "test-req-123")
		rec := httptest.NewRecorder()

		middleware.Middleware(handler).ServeHTTP(rec, req)

		if capturedReqID != "test-req-123" {
			t.Errorf("Expected request ID 'test-req-123', got '%s'", capturedReqID)
		}
	})
}

func TestExtractReqID(t *testing.T) {
	t.Run("Extracts from X-Trace-ID header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Trace-ID", "trace-123")

		reqID := extractReqID(req)
		if reqID != "trace-123" {
			t.Errorf("Expected 'trace-123', got '%s'", reqID)
		}
	})

	t.Run("Falls back to X-Request-ID header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", "req-456")

		reqID := extractReqID(req)
		if reqID != "req-456" {
			t.Errorf("Expected 'req-456', got '%s'", reqID)
		}
	})

	t.Run("Returns empty string when no headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)

		reqID := extractReqID(req)
		if reqID != "" {
			t.Errorf("Expected empty string, got '%s'", reqID)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestExtractMessages(t *testing.T) {
	handler := &ProxyHandler{
		logger: &MockLogger{},
	}

	t.Run("拒绝空 messages 数组", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"messages": []interface{}{},
		}

		_, err := handler.extractMessages(reqBody)
		if err == nil {
			t.Error("期望返回错误，但得到 nil")
		}

		if err != nil {
			expectedSubstring := "messages 数组不能为空"
			if !contains(err.Error(), expectedSubstring) {
				t.Errorf("期望错误信息包含 '%s'，得到 '%s'", expectedSubstring, err.Error())
			}
		}
	})

	t.Run("拒绝缺少 role 的消息", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"messages": []interface{}{
				map[string]interface{}{
					"content": "hello",
				},
			},
		}

		_, err := handler.extractMessages(reqBody)
		if err == nil {
			t.Error("期望返回错误，但得到 nil")
		}
	})

	t.Run("拒绝 role 为空字符串的消息", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"messages": []interface{}{
				map[string]interface{}{
					"role":    "",
					"content": "hello",
				},
			},
		}

		_, err := handler.extractMessages(reqBody)
		if err == nil {
			t.Error("期望返回错误，但得到 nil")
		}
	})

	t.Run("接受有效的消息数组", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"messages": []interface{}{
				map[string]interface{}{
					"role":    "user",
					"content": "hello",
				},
			},
		}

		messages, err := handler.extractMessages(reqBody)
		if err != nil {
			t.Errorf("期望成功，但得到错误: %v", err)
		}

		if len(messages) != 1 {
			t.Errorf("期望 1 条消息，得到 %d", len(messages))
		}

		if messages[0].Role != "user" {
			t.Errorf("期望 role='user'，得到 '%s'", messages[0].Role)
		}
	})
}

func TestIsStreamRequest(t *testing.T) {
	handler := &ProxyHandler{}

	t.Run("stream 为 true 返回 true", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"stream": true,
		}
		if !handler.isStreamRequest(reqBody) {
			t.Error("期望 stream=true 时返回 true")
		}
	})

	t.Run("stream 为 false 返回 false", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"stream": false,
		}
		if handler.isStreamRequest(reqBody) {
			t.Error("期望 stream=false 时返回 false")
		}
	})

	t.Run("stream 不存在返回 false", func(t *testing.T) {
		reqBody := map[string]interface{}{}
		if handler.isStreamRequest(reqBody) {
			t.Error("期望 stream 不存在时返回 false")
		}
	})
}

func TestDetectProtocol(t *testing.T) {
	handler := &ProxyHandler{}

	t.Run("从请求头检测 Anthropic", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/complete", nil)
		req.Header.Set("x-api-key", "sk-ant-xxx")

		protocol := handler.detectProtocol(req)
		if protocol != types.ProtocolAnthropic {
			t.Errorf("期望 'anthropic'，得到 '%s'", protocol)
		}
	})

	t.Run("默认检测为 OpenAI", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
		req.Header.Set("Authorization", "Bearer sk-xxx")

		protocol := handler.detectProtocol(req)
		if protocol != types.ProtocolOpenAI {
			t.Errorf("期望 'openai'，得到 '%s'", protocol)
		}
	})
}

func TestValidateAPIKey(t *testing.T) {
	handler := &ProxyHandler{}

	t.Run("正确的 API Key 通过验证", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
		req.Header.Set("Authorization", "Bearer test-api-key")

		if !handler.validateAPIKey(req, "test-api-key", types.ProtocolOpenAI) {
			t.Error("期望正确的 API Key 通过验证")
		}
	})

	t.Run("错误的 API Key 拒绝", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
		req.Header.Set("Authorization", "Bearer wrong-api-key")

		if handler.validateAPIKey(req, "test-api-key", types.ProtocolOpenAI) {
			t.Error("期望错误的 API Key 被拒绝")
		}
	})

	t.Run("缺少 API Key 拒绝", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/chat/completions", nil)

		if handler.validateAPIKey(req, "test-api-key", types.ProtocolOpenAI) {
			t.Error("期望缺少 API Key 被拒绝")
		}
	})
}

func TestNewProxyHandler(t *testing.T) {
	t.Run("创建非空处理器", func(t *testing.T) {
		logger := &MockLogger{}
		errorPresenter := NewErrorPresenter(logger)

		handler := NewProxyHandler(nil, nil, logger, errorPresenter)

		if handler == nil {
			t.Error("期望创建非空处理器")
		}
	})
}
