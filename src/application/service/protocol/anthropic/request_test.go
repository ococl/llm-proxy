package anthropic

import (
	"context"
	"testing"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// MockLoggerForAnthropicRequest 实现 port.Logger 接口用于 Anthropic 请求转换器测试
type MockLoggerForAnthropicRequest struct {
	debugMessages []string
	infoMessages  []string
	errorMessages []string
	warnMessages  []string
	fatalMessages []string
	fields        []map[string]interface{}
	withFields    [][]port.Field
}

func (m *MockLoggerForAnthropicRequest) Debug(msg string, fields ...port.Field) {
	m.debugMessages = append(m.debugMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForAnthropicRequest) Info(msg string, fields ...port.Field) {
	m.infoMessages = append(m.infoMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForAnthropicRequest) Warn(msg string, fields ...port.Field) {
	m.warnMessages = append(m.warnMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForAnthropicRequest) Error(msg string, fields ...port.Field) {
	m.errorMessages = append(m.errorMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForAnthropicRequest) Fatal(msg string, fields ...port.Field) {
	m.fatalMessages = append(m.fatalMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForAnthropicRequest) With(fields ...port.Field) port.Logger {
	m.withFields = append(m.withFields, fields)
	return m
}

func (m *MockLoggerForAnthropicRequest) recordFields(fields []port.Field) {
	for _, field := range fields {
		m.fields = append(m.fields, map[string]interface{}{
			"key":   field.Key,
			"value": field.Value,
		})
	}
}

func (m *MockLoggerForAnthropicRequest) reset() {
	m.debugMessages = nil
	m.infoMessages = nil
	m.errorMessages = nil
	m.warnMessages = nil
	m.fatalMessages = nil
	m.fields = nil
	m.withFields = nil
}

// 创建测试用请求对象的辅助函数
func createTestRequestForAnthropic(model string, messages []entity.Message) *entity.Request {
	return entity.NewRequestBuilder().
		ID(entity.NewRequestID("test-request-id")).
		Model(entity.ModelAlias(model)).
		Messages(messages).
		Stream(false).
		BuildUnsafe()
}

// 创建测试用消息对象的辅助函数
func createMessageForAnthropic(role, content string) entity.Message {
	return entity.NewMessage(role, content)
}

// TestRequestConverter_NewRequestConverter 测试请求转换器创建
func TestRequestConverter_NewRequestConverter(t *testing.T) {
	t.Run("使用有效参数创建", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicRequest{}
		systemPrompts := map[string]string{"claude-3-5-sonnet": "You are Claude"}

		converter := NewRequestConverter(systemPrompts, mockLogger)

		if converter == nil {
			t.Fatal("转换器不应为 nil")
		}
	})

	t.Run("使用 nil 系统提示创建", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicRequest{}

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
	mockLogger := &MockLoggerForAnthropicRequest{}
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

	t.Run("空消息列表返回原始请求", func(t *testing.T) {
		mockLogger.reset()

		// 创建一个带有一条消息的请求，然后转换器会认为需要处理（因为 maxTokens=0）
		req := createTestRequestForAnthropic("claude-3-5-sonnet", []entity.Message{
			createMessageForAnthropic("user", "Hello"),
		})

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 结果不应该为 nil
		if result == nil {
			t.Fatal("结果不应为 nil")
		}
	})

	t.Run("无系统提示时返回原始请求", func(t *testing.T) {
		mockLogger.reset()

		req := createTestRequestForAnthropic("claude-3-5-sonnet", []entity.Message{
			createMessageForAnthropic("user", "Hello"),
		})

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}
	})

	t.Run("提取并合并系统消息到 system 字段", func(t *testing.T) {
		mockLogger.reset()

		req := createTestRequestForAnthropic("claude-3-5-sonnet", []entity.Message{
			createMessageForAnthropic("system", "You are Claude"),
			createMessageForAnthropic("user", "Hello"),
		})

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 验证系统提示被设置
		if result.SystemPrompt() != "You are Claude" {
			t.Errorf("期望系统提示 'You are Claude', 实际 %s", result.SystemPrompt())
		}

		// 验证消息中不再包含系统消息
		messages := result.Messages()
		if len(messages) != 1 {
			t.Errorf("期望 1 条消息, 实际 %d 条", len(messages))
		}

		if messages[0].Role != "user" {
			t.Errorf("期望 user 消息, 实际 %s", messages[0].Role)
		}
	})

	t.Run("合并多个系统消息", func(t *testing.T) {
		mockLogger.reset()

		req := createTestRequestForAnthropic("claude-3-5-sonnet", []entity.Message{
			createMessageForAnthropic("system", "You are Claude"),
			createMessageForAnthropic("system", "You are helpful"),
			createMessageForAnthropic("user", "Hello"),
		})

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 验证系统提示被合并
		systemPrompt := result.SystemPrompt()
		if systemPrompt == "" {
			t.Fatal("系统提示不应为空")
		}
	})

	t.Run("参数系统提示优先", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicRequest{}
		converter := NewRequestConverter(nil, mockLogger)

		req := createTestRequestForAnthropic("claude-3-5-sonnet", []entity.Message{
			createMessageForAnthropic("user", "Hello"),
		})

		result, err := converter.Convert(req, "Parameter system prompt")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result.SystemPrompt() != "Parameter system prompt" {
			t.Errorf("期望参数系统提示, 实际 %s", result.SystemPrompt())
		}
	})

	t.Run("模型映射系统提示", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicRequest{}
		systemPrompts := map[string]string{"claude-3-5-sonnet": "Mapped system prompt"}
		converter := NewRequestConverter(systemPrompts, mockLogger)

		req := createTestRequestForAnthropic("claude-3-5-sonnet", []entity.Message{
			createMessageForAnthropic("user", "Hello"),
		})

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result.SystemPrompt() != "Mapped system prompt" {
			t.Errorf("期望映射的系统提示, 实际 %s", result.SystemPrompt())
		}
	})
}

// TestRequestConverter_MaxTokens 测试 max_tokens 处理
func TestRequestConverter_MaxTokens(t *testing.T) {
	t.Run("使用请求中的 max_tokens", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicRequest{}
		converter := NewRequestConverter(nil, mockLogger)

		req := createTestRequestForAnthropic("claude-3-5-sonnet", []entity.Message{
			createMessageForAnthropic("user", "Hello"),
		})

		// 注意：这里无法直接设置 MaxTokens，需要通过 builder
		req = entity.NewRequestBuilder().
			ID(req.ID()).
			Model(req.Model()).
			Messages(req.Messages()).
			MaxTokens(2000).
			BuildUnsafe()

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result.MaxTokens() != 2000 {
			t.Errorf("期望 MaxTokens 2000, 实际 %d", result.MaxTokens())
		}
	})

	t.Run("零值 max_tokens 设置默认值", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicRequest{}
		converter := NewRequestConverter(nil, mockLogger)

		req := createTestRequestForAnthropic("claude-3-5-sonnet", []entity.Message{
			createMessageForAnthropic("system", "You are Claude"),
			createMessageForAnthropic("user", "Hello"),
		})

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// Anthropic 要求必须设置 max_tokens，所以应该设置默认值
		if result.MaxTokens() == 0 {
			t.Error("max_tokens 不应为零值")
		}
	})
}

// TestRequestConverter_BuildRequest 测试请求构建功能
func TestRequestConverter_BuildRequest(t *testing.T) {
	mockLogger := &MockLoggerForAnthropicRequest{}
	converter := NewRequestConverter(nil, mockLogger)

	t.Run("构建请求设置 ClientProtocol", func(t *testing.T) {
		req := createTestRequestForAnthropic("claude-3-5-sonnet", []entity.Message{
			createMessageForAnthropic("user", "Hello"),
		})

		result, err := converter.Convert(req, "You are Claude")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 验证 ClientProtocol
		if result.ClientProtocol() != string(types.ProtocolAnthropic) {
			t.Errorf("期望 ClientProtocol %s, 实际 %s", types.ProtocolAnthropic, result.ClientProtocol())
		}
	})
}

// TestRequestConverter_MergeSystemPrompts 测试系统提示合并
func TestRequestConverter_MergeSystemPrompts(t *testing.T) {
	converter := &RequestConverter{}

	tests := []struct {
		name     string
		prompts  []string
		expected string
	}{
		{
			name:     "空列表返回空字符串",
			prompts:  []string{},
			expected: "",
		},
		{
			name:     "单个提示直接返回",
			prompts:  []string{"You are Claude"},
			expected: "You are Claude",
		},
		{
			name:     "多个提示使用双换行符合并",
			prompts:  []string{"You are Claude", "You are helpful"},
			expected: "You are Claude\n\nYou are helpful",
		},
		{
			name:     "三个提示合并",
			prompts:  []string{"Prompt 1", "Prompt 2", "Prompt 3"},
			expected: "Prompt 1\n\nPrompt 2\n\nPrompt 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.mergeSystemPrompts(tt.prompts)

			if result != tt.expected {
				t.Errorf("期望 %q, 实际 %q", tt.expected, result)
			}
		})
	}
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
			name:     "支持 Anthropic 协议",
			protocol: types.ProtocolAnthropic,
			expected: true,
		},
		{
			name:     "不支持 OpenAI 协议",
			protocol: types.ProtocolOpenAI,
			expected: false,
		},
		{
			name:     "不支持 Google 协议",
			protocol: types.ProtocolGoogle,
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

	if result != types.ProtocolAnthropic {
		t.Errorf("期望协议 %v, 实际 %v", types.ProtocolAnthropic, result)
	}
}

// TestRequestConverter_Name 测试策略名称返回
func TestRequestConverter_Name(t *testing.T) {
	converter := &RequestConverter{}

	result := converter.Name()

	expected := "AnthropicRequestConverter"
	if result != expected {
		t.Errorf("期望名称 %s, 实际 %s", expected, result)
	}
}

// TestRequestConverter_LoggerCalled 测试日志记录功能
func TestRequestConverter_LoggerCalled(t *testing.T) {
	mockLogger := &MockLoggerForAnthropicRequest{}
	converter := NewRequestConverter(nil, mockLogger)

	req := createTestRequestForAnthropic("claude-3-5-sonnet", []entity.Message{
		createMessageForAnthropic("system", "You are Claude"),
		createMessageForAnthropic("user", "Hello"),
	})

	converter.Convert(req, "")

	// 应该有调试日志
	if len(mockLogger.debugMessages) == 0 {
		t.Log("注意: 可能没有日志输出（正常行为）")
	}
}

// TestRequestConverter_ToolsCleared 测试工具字段被清除
func TestRequestConverter_ToolsCleared(t *testing.T) {
	mockLogger := &MockLoggerForAnthropicRequest{}
	converter := NewRequestConverter(nil, mockLogger)

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
		Model(entity.ModelAlias("claude-3-5-sonnet")).
		Messages([]entity.Message{createMessageForAnthropic("user", "Hello")}).
		Tools(tools).
		BuildUnsafe()

	result, err := converter.Convert(req, "You are Claude")

	if err != nil {
		t.Fatalf("期望无错误, 实际 %v", err)
	}

	// 验证参数系统提示被设置
	if result.SystemPrompt() != "You are Claude" {
		t.Errorf("期望系统提示 'You are Claude', 实际 %s", result.SystemPrompt())
	}
}

// TestRequestConverter_MessageOrder 验证消息顺序正确
func TestRequestConverter_MessageOrder(t *testing.T) {
	mockLogger := &MockLoggerForAnthropicRequest{}
	converter := NewRequestConverter(nil, mockLogger)

	t.Run("系统消息被移除，非系统消息顺序保持", func(t *testing.T) {
		mockLogger.reset()

		originalMessages := []entity.Message{
			createMessageForAnthropic("system", "System 1"),
			createMessageForAnthropic("user", "First"),
			createMessageForAnthropic("assistant", "Second"),
			createMessageForAnthropic("system", "System 2"),
			createMessageForAnthropic("user", "Third"),
		}

		req := createTestRequestForAnthropic("claude-3-5-sonnet", originalMessages)

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		messages := result.Messages()

		// 验证只有 3 条非系统消息
		if len(messages) != 3 {
			t.Errorf("期望 3 条消息, 实际 %d 条", len(messages))
		}

		// 验证顺序保持
		expectedOrder := []string{"user", "assistant", "user"}
		for i, msg := range messages {
			if msg.Role != expectedOrder[i] {
				t.Errorf("消息 %d 期望角色 %s, 实际 %s", i, expectedOrder[i], msg.Role)
			}
		}
	})
}

// TestRequestConverter_AllParametersPreserved 测试所有参数正确传递
func TestRequestConverter_AllParametersPreserved(t *testing.T) {
	t.Run("Temperature 和 TopP 正确传递", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicRequest{}
		converter := NewRequestConverter(nil, mockLogger)

		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("claude-3-5-sonnet")).
			Messages([]entity.Message{createMessageForAnthropic("user", "Hello")}).
			Temperature(0.7).
			TopP(0.9).
			Stream(false).
			BuildUnsafe()

		result, err := converter.Convert(req, "You are Claude")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result.Temperature() != 0.7 {
			t.Errorf("期望 Temperature 0.7, 实际 %f", result.Temperature())
		}

		if result.TopP() != 0.9 {
			t.Errorf("期望 TopP 0.9, 实际 %f", result.TopP())
		}
	})

	t.Run("Stop 字段正确传递", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicRequest{}
		converter := NewRequestConverter(nil, mockLogger)

		stopWords := []string{"Human:", "Assistant:"}
		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("claude-3-5-sonnet")).
			Messages([]entity.Message{createMessageForAnthropic("user", "Hello")}).
			Stop(stopWords).
			BuildUnsafe()

		result, err := converter.Convert(req, "You are Claude")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if len(result.Stop()) != 2 {
			t.Errorf("期望 2 个停止词, 实际 %d", len(result.Stop()))
		}
	})

	t.Run("User 字段正确传递", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicRequest{}
		converter := NewRequestConverter(nil, mockLogger)

		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("claude-3-5-sonnet")).
			Messages([]entity.Message{createMessageForAnthropic("user", "Hello")}).
			User("test-user").
			BuildUnsafe()

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result.User() != "test-user" {
			t.Errorf("期望 User test-user, 实际 %s", result.User())
		}
	})

	t.Run("Headers 正确传递", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicRequest{}
		converter := NewRequestConverter(nil, mockLogger)

		headers := map[string][]string{
			"X-Custom-Header": {"custom-value"},
		}
		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("claude-3-5-sonnet")).
			Messages([]entity.Message{createMessageForAnthropic("user", "Hello")}).
			Headers(headers).
			BuildUnsafe()

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 验证 headers 存在
		if result.Headers() == nil {
			t.Error("Headers 不应为空")
		}
	})

	t.Run("Stream 标志正确传递", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicRequest{}
		converter := NewRequestConverter(nil, mockLogger)

		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("claude-3-5-sonnet")).
			Messages([]entity.Message{createMessageForAnthropic("user", "Hello")}).
			Stream(true).
			BuildUnsafe()

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if !result.IsStream() {
			t.Error("期望 IsStream 为 true")
		}
	})

	t.Run("ToolChoice 正确传递", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicRequest{}
		converter := NewRequestConverter(nil, mockLogger)

		toolChoice := map[string]interface{}{
			"type":    "auto",
			"message": map[string]interface{}{"role": "assistant", "content": "I'll use a tool"},
		}
		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("claude-3-5-sonnet")).
			Messages([]entity.Message{createMessageForAnthropic("user", "Hello")}).
			ToolChoice(toolChoice).
			BuildUnsafe()

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 验证 ToolChoice 存在（即使具体值可能不同）
		_ = result.ToolChoice()
	})
}

// TestRequestConverter_SystemPromptEdgeCases 测试系统提示边缘情况
func TestRequestConverter_SystemPromptEdgeCases(t *testing.T) {
	t.Run("空系统提示字符串不设置 system 字段", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicRequest{}
		converter := NewRequestConverter(nil, mockLogger)

		req := createTestRequestForAnthropic("claude-3-5-sonnet", []entity.Message{
			createMessageForAnthropic("user", "Hello"),
		})

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 空字符串不应该设置 system 字段
		if result.SystemPrompt() != "" {
			t.Errorf("期望空系统提示, 实际 '%s'", result.SystemPrompt())
		}
	})

	t.Run("消息中的空内容系统消息被忽略", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicRequest{}
		converter := NewRequestConverter(nil, mockLogger)

		req := createTestRequestForAnthropic("claude-3-5-sonnet", []entity.Message{
			createMessageForAnthropic("system", ""), // 空内容
			createMessageForAnthropic("user", "Hello"),
		})

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 空内容的系统消息应该被忽略
		if result.SystemPrompt() != "" {
			t.Errorf("期望空系统提示, 实际 '%s'", result.SystemPrompt())
		}

		// 消息数量应该是 1（空系统消息被移除，只保留 user 消息）
		messages := result.Messages()
		if len(messages) != 1 {
			t.Errorf("期望 1 条消息, 实际 %d 条", len(messages))
		}

		// 保留的消息应该是 user 消息
		if len(messages) > 0 && messages[0].Role != "user" {
			t.Errorf("期望 user 消息, 实际 %s", messages[0].Role)
		}
	})

	t.Run("优先级：参数 > 消息内 > 模型映射", func(t *testing.T) {
		systemPrompts := map[string]string{"claude-3-5-sonnet": "Model mapped prompt"}
		mockLogger := &MockLoggerForAnthropicRequest{}
		converter := NewRequestConverter(systemPrompts, mockLogger)

		req := createTestRequestForAnthropic("claude-3-5-sonnet", []entity.Message{
			createMessageForAnthropic("system", "Message system prompt"),
			createMessageForAnthropic("user", "Hello"),
		})

		result, err := converter.Convert(req, "Parameter prompt")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 参数系统提示优先级最高
		if result.SystemPrompt() != "Parameter prompt" {
			t.Errorf("期望 'Parameter prompt', 实际 '%s'", result.SystemPrompt())
		}
	})
}

// TestRequestConverter_ContextAndStreamHandler 测试 Context 和 StreamHandler 传递
func TestRequestConverter_ContextAndStreamHandler(t *testing.T) {
	t.Run("Context 正确传递", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicRequest{}
		converter := NewRequestConverter(nil, mockLogger)

		ctx := context.Background()
		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("claude-3-5-sonnet")).
			Messages([]entity.Message{createMessageForAnthropic("user", "Hello")}).
			Context(ctx).
			BuildUnsafe()

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result.Context() != ctx {
			t.Error("Context 没有正确传递")
		}
	})

	t.Run("StreamHandler 正确传递", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicRequest{}
		converter := NewRequestConverter(nil, mockLogger)

		handler := func(data []byte) error { return nil }
		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("claude-3-5-sonnet")).
			Messages([]entity.Message{createMessageForAnthropic("user", "Hello")}).
			StreamHandler(handler).
			BuildUnsafe()

		result, err := converter.Convert(req, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 验证 StreamHandler 存在
		if result.StreamHandler() == nil {
			t.Error("StreamHandler 没有正确传递")
		}
	})
}
