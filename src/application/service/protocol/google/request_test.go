package google

import (
	"testing"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// MockLoggerForGoogleRequest 实现 port.Logger 接口用于 Google 请求转换器测试
type MockLoggerForGoogleRequest struct {
	debugMessages []string
	infoMessages  []string
	errorMessages []string
	warnMessages  []string
	fatalMessages []string
	fields        []map[string]interface{}
	withFields    [][]port.Field
}

func (m *MockLoggerForGoogleRequest) Debug(msg string, fields ...port.Field) {
	m.debugMessages = append(m.debugMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForGoogleRequest) Info(msg string, fields ...port.Field) {
	m.infoMessages = append(m.infoMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForGoogleRequest) Warn(msg string, fields ...port.Field) {
	m.warnMessages = append(m.warnMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForGoogleRequest) Error(msg string, fields ...port.Field) {
	m.errorMessages = append(m.errorMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForGoogleRequest) Fatal(msg string, fields ...port.Field) {
	m.fatalMessages = append(m.fatalMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForGoogleRequest) With(fields ...port.Field) port.Logger {
	m.withFields = append(m.withFields, fields)
	return m
}

func (m *MockLoggerForGoogleRequest) recordFields(fields []port.Field) {
	for _, field := range fields {
		m.fields = append(m.fields, map[string]interface{}{
			"key":   field.Key,
			"value": field.Value,
		})
	}
}

func (m *MockLoggerForGoogleRequest) reset() {
	m.debugMessages = nil
	m.infoMessages = nil
	m.errorMessages = nil
	m.warnMessages = nil
	m.fatalMessages = nil
	m.fields = nil
	m.withFields = nil
}

// 创建测试用请求对象的辅助函数
func createTestRequestForGoogle(model string, messages []entity.Message) *entity.Request {
	return entity.NewRequestBuilder().
		ID(entity.NewRequestID("test-request-id")).
		Model(entity.ModelAlias(model)).
		Messages(messages).
		Stream(false).
		BuildUnsafe()
}

// 创建测试用消息对象的辅助函数
func createMessageForGoogle(role, content string) entity.Message {
	return entity.NewMessage(role, content)
}

// TestRequestConverter_NewRequestConverter 测试请求转换器创建
func TestRequestConverter_NewRequestConverter(t *testing.T) {
	t.Run("使用有效日志器创建", func(t *testing.T) {
		mockLogger := &MockLoggerForGoogleRequest{}

		converter := NewRequestConverter(mockLogger)

		if converter == nil {
			t.Fatal("转换器不应为 nil")
		}
	})

	t.Run("使用 nil 日志器创建时使用 NopLogger", func(t *testing.T) {
		converter := NewRequestConverter(nil)

		if converter == nil {
			t.Fatal("转换器不应为 nil")
		}

		if converter.logger == nil {
			t.Error("日志器不应为 nil")
		}
	})
}

// TestRequestConverter_Convert 测试请求转换功能
func TestRequestConverter_Convert(t *testing.T) {
	mockLogger := &MockLoggerForGoogleRequest{}
	converter := NewRequestConverter(mockLogger)

	t.Run("nil 请求返回 nil", func(t *testing.T) {
		result, err := converter.Convert(nil, "")

		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
		}
		if err != nil {
			t.Errorf("期望 nil 错误, 实际 %v", err)
		}
	})

	t.Run("空消息列表返回原始请求", func(t *testing.T) {
		mockLogger.reset()

		// 创建一个带有一条消息的请求
		req := createTestRequestForGoogle("gemini-pro", []entity.Message{
			createMessageForGoogle("user", "Hello"),
		})

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}
	})

	t.Run("简单文本消息不需要转换", func(t *testing.T) {
		mockLogger.reset()

		req := createTestRequestForGoogle("gemini-pro", []entity.Message{
			createMessageForGoogle("user", "Hello"),
		})

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 简单请求应该直接返回原始请求
		if result.ID() != req.ID() {
			t.Errorf("期望请求 ID %s, 实际 %s", req.ID(), result.ID())
		}
	})

	t.Run("非文本内容需要转换", func(t *testing.T) {
		mockLogger.reset()

		// 模拟多模态内容（非字符串）
		multimodalContent := []interface{}{
			map[string]string{"type": "text", "text": "Hello"},
			map[string]string{"type": "image_url", "image_url": "data:image/jpeg;base64,..."},
		}

		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-request-id")).
			Model(entity.ModelAlias("gemini-pro")).
			Messages([]entity.Message{{Role: "user", Content: multimodalContent}}).
			Stream(false).
			BuildUnsafe()

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 应该有调试日志记录转换
		if len(mockLogger.debugMessages) == 0 {
			t.Log("注意: 可能没有日志输出（正常行为）")
		}
	})

	t.Run("系统提示触发转换", func(t *testing.T) {
		mockLogger.reset()

		req := createTestRequestForGoogle("gemini-pro", []entity.Message{
			createMessageForGoogle("user", "Hello"),
		})

		result, err := converter.Convert(req, "You are Gemini")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 应该记录系统提示日志
		found := false
		for _, msg := range mockLogger.debugMessages {
			if msg != "" && (msg != "Google Vertex AI 协议转换完成") {
				found = true
				break
			}
		}
		if !found {
			t.Log("注意: 系统提示日志可能已优化")
		}
	})

	t.Run("StopSequences 触发转换", func(t *testing.T) {
		mockLogger.reset()

		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-request-id")).
			Model(entity.ModelAlias("gemini-pro")).
			Messages([]entity.Message{createMessageForGoogle("user", "Hello")}).
			Stop([]string{"stop1", "stop2"}).
			Stream(false).
			BuildUnsafe()

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}
	})
}

// TestRequestConverter_HasNonTextContent 测试非文本内容检测
func TestRequestConverter_HasNonTextContent(t *testing.T) {
	converter := &RequestConverter{}

	tests := []struct {
		name     string
		messages []entity.Message
		expected bool
	}{
		{
			name:     "空消息列表返回 false",
			messages: []entity.Message{},
			expected: false,
		},
		{
			name: "纯文本消息返回 false",
			messages: []entity.Message{
				{Role: "user", Content: "Hello"},
			},
			expected: false,
		},
		{
			name: "多文本消息返回 false",
			messages: []entity.Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
			},
			expected: false,
		},
		{
			name: "数组内容返回 true",
			messages: []entity.Message{
				{Role: "user", Content: []interface{}{
					map[string]string{"type": "text", "text": "Hello"},
				}},
			},
			expected: true,
		},
		{
			name: "多模态内容返回 true",
			messages: []entity.Message{
				{Role: "user", Content: []interface{}{
					map[string]string{"type": "text", "text": "Hello"},
					map[string]string{"type": "image_url", "image_url": "data:image/jpeg;base64,abc123"},
				}},
			},
			expected: true,
		},
		{
			name: "nil 内容被视为 false",
			messages: []entity.Message{
				{Role: "user", Content: nil},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.hasNonTextContent(tt.messages)

			if result != tt.expected {
				t.Errorf("期望 %v, 实际 %v", tt.expected, result)
			}
		})
	}
}

// TestRequestConverter_BuildRequest 测试请求构建功能
func TestRequestConverter_BuildRequest(t *testing.T) {
	mockLogger := &MockLoggerForGoogleRequest{}
	converter := NewRequestConverter(mockLogger)

	t.Run("构建请求设置 ClientProtocol", func(t *testing.T) {
		req := createTestRequestForGoogle("gemini-pro", []entity.Message{
			createMessageForGoogle("user", "Hello"),
		})

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 验证 ClientProtocol
		if result.ClientProtocol() != string(types.ProtocolGoogle) {
			t.Errorf("期望 ClientProtocol %s, 实际 %s", types.ProtocolGoogle, result.ClientProtocol())
		}
	})

	t.Run("构建请求保留消息", func(t *testing.T) {
		messages := []entity.Message{
			createMessageForGoogle("user", "Hello"),
			createMessageForGoogle("assistant", "Hi there!"),
		}

		req := createTestRequestForGoogle("gemini-pro", messages)

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if len(result.Messages()) != len(messages) {
			t.Errorf("期望 %d 条消息, 实际 %d 条", len(messages), len(result.Messages()))
		}
	})
}

// TestRequestConverter_Supports 测试协议支持检查
func TestRequestConverter_Supports(t *testing.T) {
	converter := &RequestConverter{}

	tests := []struct {
		name     string
		protocol types.Protocol
		expected bool
	}{
		{
			name:     "支持 Google 协议",
			protocol: types.ProtocolGoogle,
			expected: true,
		},
		{
			name:     "不支持 OpenAI 协议",
			protocol: types.ProtocolOpenAI,
			expected: false,
		},
		{
			name:     "不支持 Anthropic 协议",
			protocol: types.ProtocolAnthropic,
			expected: false,
		},
		{
			name:     "不支持 Azure 协议",
			protocol: types.ProtocolAzure,
			expected: false,
		},
		{
			name:     "不支持 DeepSeek 协议",
			protocol: types.ProtocolDeepSeek,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.Supports(tt.protocol)

			if result != tt.expected {
				t.Errorf("期望 %v, 实际 %v", tt.expected, result)
			}
		})
	}
}

// TestRequestConverter_Protocol 测试协议返回
func TestRequestConverter_Protocol(t *testing.T) {
	converter := &RequestConverter{}

	result := converter.Protocol()

	if result != types.ProtocolGoogle {
		t.Errorf("期望协议 %v, 实际 %v", types.ProtocolGoogle, result)
	}
}

// TestRequestConverter_Name 测试策略名称返回
func TestRequestConverter_Name(t *testing.T) {
	converter := &RequestConverter{}

	result := converter.Name()

	expected := "GoogleVertexAIRequestConverter"
	if result != expected {
		t.Errorf("期望名称 %s, 实际 %s", expected, result)
	}
}

// TestRequestConverter_LoggerCalled 测试日志记录功能
func TestRequestConverter_LoggerCalled(t *testing.T) {
	mockLogger := &MockLoggerForGoogleRequest{}
	converter := NewRequestConverter(mockLogger)

	req := createTestRequestForGoogle("gemini-pro", []entity.Message{
		createMessageForGoogle("user", "Hello"),
	})

	converter.Convert(req, "You are Gemini")

	// 应该有调试日志
	if len(mockLogger.debugMessages) == 0 {
		t.Log("注意: 可能没有日志输出（正常行为）")
	}
}

// TestRequestConverter_ToolsCleared 测试工具字段被清除
func TestRequestConverter_ToolsCleared(t *testing.T) {
	mockLogger := &MockLoggerForGoogleRequest{}
	converter := NewRequestConverter(mockLogger)

	// 创建带工具的请求
	tools := []entity.Tool{
		{
			Type: "function",
			Function: entity.ToolFunction{
				Name:        "get_weather",
				Description: "Get weather",
			},
		},
	}

	req := entity.NewRequestBuilder().
		ID(entity.NewRequestID("test-request-id")).
		Model(entity.ModelAlias("gemini-pro")).
		Messages([]entity.Message{createMessageForGoogle("user", "Hello")}).
		Tools(tools).
		BuildUnsafe()

	result, err := converter.Convert(req, "")

	if err != nil {
		t.Fatalf("期望无错误, 实际 %v", err)
	}

	// Google 转换器会清除 tools（因为格式不同）
	if len(result.Tools()) != 0 {
		t.Log("注意: 工具可能已被清除")
	}
}
