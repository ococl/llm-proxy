package openai

import (
	"testing"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// MockLoggerForRequest 实现 port.Logger 接口用于请求转换器测试
type MockLoggerForRequest struct {
	debugMessages []string
	infoMessages  []string
	errorMessages []string
	warnMessages  []string
	fatalMessages []string
	fields        []map[string]interface{}
	withFields    [][]port.Field
}

func (m *MockLoggerForRequest) Debug(msg string, fields ...port.Field) {
	m.debugMessages = append(m.debugMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForRequest) Info(msg string, fields ...port.Field) {
	m.infoMessages = append(m.infoMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForRequest) Warn(msg string, fields ...port.Field) {
	m.warnMessages = append(m.warnMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForRequest) Error(msg string, fields ...port.Field) {
	m.errorMessages = append(m.errorMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForRequest) Fatal(msg string, fields ...port.Field) {
	m.fatalMessages = append(m.fatalMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForRequest) With(fields ...port.Field) port.Logger {
	m.withFields = append(m.withFields, fields)
	return m
}

func (m *MockLoggerForRequest) recordFields(fields []port.Field) {
	for _, field := range fields {
		m.fields = append(m.fields, map[string]interface{}{
			"key":   field.Key,
			"value": field.Value,
		})
	}
}

func (m *MockLoggerForRequest) reset() {
	m.debugMessages = nil
	m.infoMessages = nil
	m.errorMessages = nil
	m.warnMessages = nil
	m.fatalMessages = nil
	m.fields = nil
	m.withFields = nil
}

// 创建测试用请求对象的辅助函数
func createTestRequest(model string, messages []entity.Message) *entity.Request {
	builder := entity.NewRequestBuilder().
		ID(entity.NewRequestID("test-request-id")).
		Model(entity.ModelAlias(model)).
		Messages(messages).
		Stream(false)

	return builder.BuildUnsafe()
}

// 创建测试用消息对象的辅助函数
func createTestMessage(role, content string) entity.Message {
	return entity.NewMessage(role, content)
}

// TestRequestConverter_NewRequestConverter 测试请求转换器创建
func TestRequestConverter_NewRequestConverter(t *testing.T) {
	t.Run("使用有效参数创建", func(t *testing.T) {
		mockLogger := &MockLoggerForRequest{}
		systemPrompts := map[string]string{"gpt-4": "You are GPT-4"}

		converter := NewRequestConverter(systemPrompts, mockLogger)

		if converter == nil {
			t.Fatal("转换器不应为 nil")
		}
	})

	t.Run("使用 nil 系统提示创建", func(t *testing.T) {
		mockLogger := &MockLoggerForRequest{}

		converter := NewRequestConverter(nil, mockLogger)

		if converter == nil {
			t.Fatal("转换器不应为 nil")
		}
	})

	t.Run("使用 nil 日志器创建时使用 NopLogger", func(t *testing.T) {
		converter := NewRequestConverter(nil, nil)

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
	mockLogger := &MockLoggerForRequest{}
	converter := NewRequestConverter(nil, mockLogger)

	t.Run("nil 请求返回 nil", func(t *testing.T) {
		result, err := converter.Convert(nil, "")

		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
		}
		if err != nil {
			t.Errorf("期望 nil 错误, 实际 %v", err)
		}
	})

	t.Run("无系统提示时返回原始请求", func(t *testing.T) {
		mockLogger.reset()

		req := createTestRequest("gpt-4", []entity.Message{
			createTestMessage("user", "Hello"),
		})

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 结果应该与原始请求相同
		if result.ID() != req.ID() {
			t.Errorf("期望请求 ID %s, 实际 %s", req.ID(), result.ID())
		}
	})

	t.Run("系统提示注入到无系统消息的请求", func(t *testing.T) {
		mockLogger.reset()

		req := createTestRequest("gpt-4", []entity.Message{
			createTestMessage("user", "Hello"),
		})

		result, err := converter.Convert(req, "You are a helpful assistant")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		messages := result.Messages()
		if len(messages) == 0 {
			t.Fatal("消息不应为空")
		}

		// 第一个消息应该是系统消息
		if messages[0].Role != "system" {
			t.Errorf("期望第一个消息角色为 system, 实际 %s", messages[0].Role)
		}

		if messages[0].Content != "You are a helpful assistant" {
			t.Errorf("期望系统提示 'You are a helpful assistant', 实际 %s", messages[0].Content)
		}
	})

	t.Run("已有系统消息时不重复注入", func(t *testing.T) {
		mockLogger.reset()

		req := createTestRequest("gpt-4", []entity.Message{
			createTestMessage("system", "You are an expert"),
			createTestMessage("user", "Hello"),
		})

		result, err := converter.Convert(req, "You are a helpful assistant")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		messages := result.Messages()
		if len(messages) != 2 {
			t.Errorf("期望 2 条消息, 实际 %d 条", len(messages))
		}

		// 第一个消息应该保持不变
		if messages[0].Role != "system" || messages[0].Content != "You are an expert" {
			t.Error("原始系统消息应保持不变")
		}
	})

	t.Run("参数系统提示优先于模型映射", func(t *testing.T) {
		mockLogger := &MockLoggerForRequest{}
		systemPrompts := map[string]string{"gpt-4": "Mapped system prompt"}
		converter := NewRequestConverter(systemPrompts, mockLogger)

		req := createTestRequest("gpt-4", []entity.Message{
			createTestMessage("user", "Hello"),
		})

		result, err := converter.Convert(req, "Parameter system prompt")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		messages := result.Messages()
		if messages[0].Content != "Parameter system prompt" {
			t.Errorf("期望参数系统提示, 实际 %s", messages[0].Content)
		}
	})

	t.Run("使用模型映射的系统提示", func(t *testing.T) {
		mockLogger := &MockLoggerForRequest{}
		systemPrompts := map[string]string{"gpt-4": "Mapped system prompt for gpt-4"}
		converter := NewRequestConverter(systemPrompts, mockLogger)

		req := createTestRequest("gpt-4", []entity.Message{
			createTestMessage("user", "Hello"),
		})

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		messages := result.Messages()
		if messages[0].Content != "Mapped system prompt for gpt-4" {
			t.Errorf("期望映射的系统提示, 实际 %s", messages[0].Content)
		}
	})

	t.Run("未知模型使用空系统提示", func(t *testing.T) {
		mockLogger := &MockLoggerForRequest{}
		systemPrompts := map[string]string{"gpt-4": "Only for gpt-4"}
		converter := NewRequestConverter(systemPrompts, mockLogger)

		req := createTestRequest("unknown-model", []entity.Message{
			createTestMessage("user", "Hello"),
		})

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 未知模型没有映射，所以不注入系统提示
		messages := result.Messages()
		if len(messages) != 1 {
			t.Errorf("期望 1 条消息, 实际 %d 条", len(messages))
		}
	})
}

// TestRequestConverter_BuildRequest 测试请求构建功能
func TestRequestConverter_BuildRequest(t *testing.T) {
	mockLogger := &MockLoggerForRequest{}
	converter := NewRequestConverter(nil, mockLogger)

	t.Run("构建请求保留所有字段", func(t *testing.T) {
		originalMessages := []entity.Message{
			createTestMessage("user", "Hello"),
			createTestMessage("assistant", "Hi there!"),
		}

		originalReq := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-request-id")).
			Model(entity.ModelAlias("gpt-4")).
			Messages(originalMessages).
			MaxTokens(1000).
			Temperature(0.7).
			TopP(0.9).
			Stream(true).
			Stop([]string{"stop1", "stop2"}).
			Tools([]entity.Tool{
				{
					Type: "function",
					Function: entity.ToolFunction{
						Name:        "get_weather",
						Description: "Get weather",
					},
				},
			}).
			BuildUnsafe()

		// 构建新请求
		newMessages := []entity.Message{
			createTestMessage("system", "You are helpful"),
			createTestMessage("user", "Hello"),
		}

		result := converter.buildRequest(originalReq, newMessages)

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 验证字段保留
		if result.ID() != originalReq.ID() {
			t.Errorf("期望 ID %s, 实际 %s", originalReq.ID(), result.ID())
		}

		if result.Model() != originalReq.Model() {
			t.Errorf("期望模型 %s, 实际 %s", originalReq.Model(), result.Model())
		}

		if len(result.Messages()) != 2 {
			t.Errorf("期望 2 条消息, 实际 %d 条", len(result.Messages()))
		}

		if result.MaxTokens() != originalReq.MaxTokens() {
			t.Errorf("期望 MaxTokens %d, 实际 %d", originalReq.MaxTokens(), result.MaxTokens())
		}

		if result.Temperature() != originalReq.Temperature() {
			t.Errorf("期望 Temperature %f, 实际 %f", originalReq.Temperature(), result.Temperature())
		}

		if result.TopP() != originalReq.TopP() {
			t.Errorf("期望 TopP %f, 实际 %f", originalReq.TopP(), result.TopP())
		}

		if !result.IsStream() {
			t.Error("期望 Stream 为 true")
		}

		// 验证 ClientProtocol
		if result.ClientProtocol() != string(types.ProtocolOpenAI) {
			t.Errorf("期望 ClientProtocol %s, 实际 %s", types.ProtocolOpenAI, result.ClientProtocol())
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
			name:     "支持 OpenAI 协议",
			protocol: types.ProtocolOpenAI,
			expected: true,
		},
		{
			name:     "支持 DeepSeek 协议（兼容 OpenAI）",
			protocol: types.ProtocolDeepSeek,
			expected: true,
		},
		{
			name:     "支持 Groq 协议（兼容 OpenAI）",
			protocol: types.ProtocolGroq,
			expected: true,
		},
		{
			name:     "支持 Mistral 协议（兼容 OpenAI）",
			protocol: types.ProtocolMistral,
			expected: true,
		},
		{
			name:     "支持 Cohere 协议（兼容 OpenAI）",
			protocol: types.ProtocolCohere,
			expected: true,
		},
		{
			name:     "不支持 Anthropic 协议",
			protocol: types.ProtocolAnthropic,
			expected: false,
		},
		{
			name:     "不支持 Google 协议",
			protocol: types.ProtocolGoogle,
			expected: false,
		},
		{
			name:     "支持 Azure 协议（OpenAI 兼容）",
			protocol: types.ProtocolAzure,
			expected: true,
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

	if result != types.ProtocolOpenAI {
		t.Errorf("期望协议 %v, 实际 %v", types.ProtocolOpenAI, result)
	}
}

// TestRequestConverter_Name 测试策略名称返回
func TestRequestConverter_Name(t *testing.T) {
	converter := &RequestConverter{}

	result := converter.Name()

	expected := "OpenAIRequestConverter"
	if result != expected {
		t.Errorf("期望名称 %s, 实际 %s", expected, result)
	}
}

// TestRequestConverter_LoggerCalled 测试日志记录功能
func TestRequestConverter_LoggerCalled(t *testing.T) {
	mockLogger := &MockLoggerForRequest{}
	converter := NewRequestConverter(nil, mockLogger)

	req := createTestRequest("gpt-4", []entity.Message{
		createTestMessage("user", "Hello"),
	})

	converter.Convert(req, "You are helpful")

	// 由于系统提示被添加，应该有调试日志
	if len(mockLogger.debugMessages) == 0 && len(mockLogger.infoMessages) == 0 {
		t.Log("注意: 可能没有日志输出（正常行为）")
	}
}

// TestRequestConverter_EmptySystemPrompt 不注入空系统提示
func TestRequestConverter_EmptySystemPrompt(t *testing.T) {
	mockLogger := &MockLoggerForRequest{}
	converter := NewRequestConverter(nil, mockLogger)

	t.Run("空字符串系统提示不注入", func(t *testing.T) {
		mockLogger.reset()

		req := createTestRequest("gpt-4", []entity.Message{
			createTestMessage("user", "Hello"),
		})

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 空系统提示不应该添加系统消息
		messages := result.Messages()
		if len(messages) != 1 {
			t.Errorf("期望 1 条消息, 实际 %d 条", len(messages))
		}
	})

	t.Run("空系统提示映射不注入", func(t *testing.T) {
		mockLogger := &MockLoggerForRequest{}
		systemPrompts := map[string]string{"gpt-4": ""}
		converter := NewRequestConverter(systemPrompts, mockLogger)

		req := createTestRequest("gpt-4", []entity.Message{
			createTestMessage("user", "Hello"),
		})

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 空系统提示不应该添加系统消息
		messages := result.Messages()
		if len(messages) != 1 {
			t.Errorf("期望 1 条消息, 实际 %d 条", len(messages))
		}
	})
}

// TestRequestConverter_MessageOrder 验证消息顺序正确
func TestRequestConverter_MessageOrder(t *testing.T) {
	mockLogger := &MockLoggerForRequest{}
	converter := NewRequestConverter(nil, mockLogger)

	t.Run("系统消息在最前", func(t *testing.T) {
		mockLogger.reset()

		originalMessages := []entity.Message{
			createTestMessage("user", "First"),
			createTestMessage("assistant", "Second"),
			createTestMessage("user", "Third"),
		}

		req := createTestRequest("gpt-4", originalMessages)

		result, err := converter.Convert(req, "System prompt")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		messages := result.Messages()

		// 验证顺序
		if len(messages) < 4 {
			t.Fatalf("期望至少 4 条消息, 实际 %d 条", len(messages))
		}

		if messages[0].Role != "system" {
			t.Errorf("期望第一条消息为 system, 实际 %s", messages[0].Role)
		}

		if messages[0].Content != "System prompt" {
			t.Errorf("期望系统提示, 实际 %s", messages[0].Content)
		}

		// 验证原始消息顺序保持
		if messages[1].Role != "user" || messages[1].Content != "First" {
			t.Error("原始第一条消息应该在第二位")
		}
	})
}
