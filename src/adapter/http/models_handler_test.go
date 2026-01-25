package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
)

// MockConfigProviderForModels 用于测试的 Mock 配置提供器。
type MockConfigProviderForModels struct {
	config *port.Config
}

func (m *MockConfigProviderForModels) Get() *port.Config {
	return m.config
}

func (m *MockConfigProviderForModels) Watch() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (m *MockConfigProviderForModels) GetBackend(name string) *entity.Backend {
	return nil
}

func (m *MockConfigProviderForModels) GetModelAlias(alias string) *port.ModelAlias {
	return m.config.Models[alias]
}

// MockLoggerForModels 用于测试的 Mock 日志记录器。
type MockLoggerForModels struct {
	messages []string
}

func (m *MockLoggerForModels) Debug(msg string, fields ...port.Field) {
	m.messages = append(m.messages, msg)
}

func (m *MockLoggerForModels) Error(msg string, fields ...port.Field) {}

func (m *MockLoggerForModels) Fatal(msg string, fields ...port.Field) {}

func (m *MockLoggerForModels) Info(msg string, fields ...port.Field) {}

func (m *MockLoggerForModels) Warn(msg string, fields ...port.Field) {}

func (m *MockLoggerForModels) With(fields ...port.Field) port.Logger {
	return m
}

// TestModelsHandler_ServeHTTP 测试 ModelsHandler 的 ServeHTTP 方法。
func TestModelsHandler_ServeHTTP(t *testing.T) {
	t.Run("返回模型列表", func(t *testing.T) {
		config := &port.Config{
			Models: map[string]*port.ModelAlias{
				"gpt-4":         {},
				"claude-3-opus": {},
			},
		}
		provider := &MockConfigProviderForModels{config: config}
		logger := &MockLoggerForModels{}
		handler := NewModelsHandler(provider, logger)

		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var resp ModelsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析响应 JSON 失败: %v", err)
		}

		if resp.Object != "list" {
			t.Errorf("期望 Object 'list', 实际 '%s'", resp.Object)
		}

		if len(resp.Data) != 2 {
			t.Errorf("期望 2 个模型, 实际 %d 个", len(resp.Data))
		}

		// 验证模型数据
		modelIDs := make(map[string]bool)
		for _, model := range resp.Data {
			if model.Object != "model" {
				t.Errorf("模型 %s 的 Object 期望 'model', 实际 '%s'", model.ID, model.Object)
			}
			if model.OwnedBy != "llm-proxy" {
				t.Errorf("模型 %s 的 OwnedBy 期望 'llm-proxy', 实际 '%s'", model.ID, model.OwnedBy)
			}
			if model.Created == 0 {
				t.Errorf("模型 %s 的 Created 不应为 0", model.ID)
			}
			modelIDs[model.ID] = true
		}

		if !modelIDs["gpt-4"] {
			t.Error("响应中缺少 gpt-4 模型")
		}
		if !modelIDs["claude-3-opus"] {
			t.Error("响应中缺少 claude-3-opus 模型")
		}
	})

	t.Run("空模型列表", func(t *testing.T) {
		config := &port.Config{
			Models: map[string]*port.ModelAlias{},
		}
		provider := &MockConfigProviderForModels{config: config}
		logger := &MockLoggerForModels{}
		handler := NewModelsHandler(provider, logger)

		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("期望状态码 %d, 实际 %d", http.StatusOK, rec.Code)
		}

		var resp ModelsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析响应 JSON 失败: %v", err)
		}

		if len(resp.Data) != 0 {
			t.Errorf("期望 0 个模型, 实际 %d 个", len(resp.Data))
		}
	})

	t.Run("拒绝非 GET 请求", func(t *testing.T) {
		config := &port.Config{
			Models: map[string]*port.ModelAlias{
				"gpt-4": {},
			},
		}
		provider := &MockConfigProviderForModels{config: config}
		logger := &MockLoggerForModels{}
		handler := NewModelsHandler(provider, logger)

		req := httptest.NewRequest(http.MethodPost, "/v1/models", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("期望状态码 %d, 实际 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("正确设置响应头", func(t *testing.T) {
		config := &port.Config{
			Models: map[string]*port.ModelAlias{
				"test-model": {},
			},
		}
		provider := &MockConfigProviderForModels{config: config}
		logger := &MockLoggerForModels{}
		handler := NewModelsHandler(provider, logger)

		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Header().Get("Content-Type") != "application/json" {
			t.Errorf("期望 Content-Type 'application/json', 实际 '%s'", rec.Header().Get("Content-Type"))
		}

		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("期望 X-Content-Type-Options 'nosniff', 实际 '%s'", rec.Header().Get("X-Content-Type-Options"))
		}
	})

	t.Run("模型按配置别名返回", func(t *testing.T) {
		config := &port.Config{
			Models: map[string]*port.ModelAlias{
				"my-model-alias": {
					Routes: []port.ModelRoute{
						{Model: "actual-model-name"},
					},
				},
			},
		}
		provider := &MockConfigProviderForModels{config: config}
		logger := &MockLoggerForModels{}
		handler := NewModelsHandler(provider, logger)

		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		var resp ModelsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析响应 JSON 失败: %v", err)
		}

		if len(resp.Data) != 1 {
			t.Errorf("期望 1 个模型, 实际 %d 个", len(resp.Data))
		}

		if resp.Data[0].ID != "my-model-alias" {
			t.Errorf("期望模型 ID 'my-model-alias', 实际 '%s'", resp.Data[0].ID)
		}
	})

	t.Run("Created 时间戳有效", func(t *testing.T) {
		config := &port.Config{
			Models: map[string]*port.ModelAlias{
				"test": {},
			},
		}
		provider := &MockConfigProviderForModels{config: config}
		logger := &MockLoggerForModels{}
		handler := NewModelsHandler(provider, logger)

		before := time.Now().Unix()
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		after := time.Now().Unix()

		var resp ModelsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析响应 JSON 失败: %v", err)
		}

		if resp.Data[0].Created < before || resp.Data[0].Created > after {
			t.Errorf("Created 时间戳 %d 不在有效范围内 [%d, %d]", resp.Data[0].Created, before, after)
		}
	})
}

// TestModelsHandler_NewModelsHandler 测试 NewModelsHandler 函数。
func TestModelsHandler_NewModelsHandler(t *testing.T) {
	config := &port.Config{
		Models: map[string]*port.ModelAlias{},
	}
	provider := &MockConfigProviderForModels{config: config}
	logger := &MockLoggerForModels{}

	handler := NewModelsHandler(provider, logger)

	if handler == nil {
		t.Fatal("NewModelsHandler 返回 nil")
	}

	if handler.config == nil {
		t.Error("handler.config 为 nil")
	}

	if handler.logger == nil {
		t.Error("handler.logger 为 nil")
	}
}

// TestModelsResponse 验证 ModelsResponse 结构正确性。
func TestModelsResponse(t *testing.T) {
	resp := ModelsResponse{
		Object: "list",
		Data: []Model{
			{
				ID:      "test-model",
				Object:  "model",
				Created: time.Now().Unix(),
				OwnedBy: "test-owner",
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("JSON 序列化失败: %v", err)
	}

	var unmarshaled ModelsResponse
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("JSON 反序列化失败: %v", err)
	}

	if unmarshaled.Object != resp.Object {
		t.Errorf("Object 不匹配: 期望 '%s', 实际 '%s'", resp.Object, unmarshaled.Object)
	}

	if len(unmarshaled.Data) != len(resp.Data) {
		t.Errorf("Data 长度不匹配: 期望 %d, 实际 %d", len(resp.Data), len(unmarshaled.Data))
	}
}

// TestModel 验证 Model 结构正确性。
func TestModel(t *testing.T) {
	model := Model{
		ID:      "test-model",
		Object:  "model",
		Created: 1234567890,
		OwnedBy: "test-owner",
	}

	data, err := json.Marshal(model)
	if err != nil {
		t.Fatalf("JSON 序列化失败: %v", err)
	}

	var unmarshaled Model
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("JSON 反序列化失败: %v", err)
	}

	if unmarshaled.ID != model.ID {
		t.Errorf("ID 不匹配: 期望 '%s', 实际 '%s'", model.ID, unmarshaled.ID)
	}

	if unmarshaled.Object != model.Object {
		t.Errorf("Object 不匹配: 期望 '%s', 实际 '%s'", model.Object, unmarshaled.Object)
	}

	if unmarshaled.Created != model.Created {
		t.Errorf("Created 不匹配: 期望 %d, 实际 %d", model.Created, unmarshaled.Created)
	}

	if unmarshaled.OwnedBy != model.OwnedBy {
		t.Errorf("OwnedBy 不匹配: 期望 '%s', 实际 '%s'", model.OwnedBy, unmarshaled.OwnedBy)
	}
}
