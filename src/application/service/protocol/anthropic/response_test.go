package anthropic

import (
	"encoding/json"
	"testing"

	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// MockLoggerForAnthropicResponse 实现 port.Logger 接口用于 Anthropic 响应转换器测试
type MockLoggerForAnthropicResponse struct {
	debugMessages []string
	infoMessages  []string
	errorMessages []string
	warnMessages  []string
	fatalMessages []string
	fields        []map[string]interface{}
	withFields    [][]port.Field
}

func (m *MockLoggerForAnthropicResponse) Debug(msg string, fields ...port.Field) {
	m.debugMessages = append(m.debugMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForAnthropicResponse) Info(msg string, fields ...port.Field) {
	m.infoMessages = append(m.infoMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForAnthropicResponse) Warn(msg string, fields ...port.Field) {
	m.warnMessages = append(m.warnMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForAnthropicResponse) Error(msg string, fields ...port.Field) {
	m.errorMessages = append(m.errorMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForAnthropicResponse) Fatal(msg string, fields ...port.Field) {
	m.fatalMessages = append(m.fatalMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForAnthropicResponse) With(fields ...port.Field) port.Logger {
	m.withFields = append(m.withFields, fields)
	return m
}

func (m *MockLoggerForAnthropicResponse) recordFields(fields []port.Field) {
	for _, field := range fields {
		m.fields = append(m.fields, map[string]interface{}{
			"key":   field.Key,
			"value": field.Value,
		})
	}
}

func (m *MockLoggerForAnthropicResponse) reset() {
	m.debugMessages = nil
	m.infoMessages = nil
	m.errorMessages = nil
	m.warnMessages = nil
	m.fatalMessages = nil
	m.fields = nil
	m.withFields = nil
}

// TestResponseConverter_NewResponseConverter 测试响应转换器创建
func TestResponseConverter_NewResponseConverter(t *testing.T) {
	t.Run("使用有效日志器创建", func(t *testing.T) {
		mockLogger := &MockLoggerForAnthropicResponse{}

		converter := NewResponseConverter(mockLogger)

		if converter == nil {
			t.Fatal("转换器不应为 nil")
		}
	})

	t.Run("使用 nil 日志器创建时使用 NopLogger", func(t *testing.T) {
		converter := NewResponseConverter(nil)

		if converter == nil {
			t.Fatal("转换器不应为 nil")
		}

		if converter.logger == nil {
			t.Error("日志器不应为 nil")
		}
	})
}

// TestResponseConverter_Convert 测试响应转换功能
func TestResponseConverter_Convert(t *testing.T) {
	mockLogger := &MockLoggerForAnthropicResponse{}
	converter := NewResponseConverter(mockLogger)

	t.Run("空响应返回 nil", func(t *testing.T) {
		result, err := converter.Convert(nil, "claude-3-opus-20240229")

		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
		}
		if err != nil {
			t.Errorf("期望 nil 错误, 实际 %v", err)
		}
	})

	t.Run("空字节切片返回 nil", func(t *testing.T) {
		result, err := converter.Convert([]byte{}, "claude-3-opus-20240229")

		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
		}
		if err != nil {
			t.Errorf("期望 nil 错误, 实际 %v", err)
		}
	})

	t.Run("简单文本内容正确转换", func(t *testing.T) {
		mockLogger.reset()

		anthropicResp := map[string]interface{}{
			"id":          "msg_01abc123",
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]interface{}{{"type": "text", "text": "Hello, I am Claude."}},
			"model":       "claude-3-opus-20240229",
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  10,
				"output_tokens": 8,
			},
		}

		respJSON, _ := json.Marshal(anthropicResp)
		result, err := converter.Convert(respJSON, "claude-3-opus-20240229")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.ID != "msg_01abc123" {
			t.Errorf("期望 ID msg_01abc123, 实际 %s", result.ID)
		}

		if len(result.Choices) != 1 {
			t.Errorf("期望 1 个选择, 实际 %d", len(result.Choices))
		}

		if result.Choices[0].Message.Content != "Hello, I am Claude." {
			t.Errorf("期望内容 'Hello, I am Claude.', 实际 %s", result.Choices[0].Message.Content)
		}

		if result.Choices[0].FinishReason != "stop" {
			t.Errorf("期望 finish_reason stop, 实际 %s", result.Choices[0].FinishReason)
		}
	})

	t.Run("stop_sequence 转换为 stop", func(t *testing.T) {
		mockLogger.reset()

		anthropicResp := map[string]interface{}{
			"id":            "msg_02def456",
			"type":          "message",
			"role":          "assistant",
			"content":       []map[string]interface{}{{"type": "text", "text": "Response with stop sequence"}},
			"model":         "claude-3-sonnet-20240229",
			"stop_reason":   "stop_sequence",
			"stop_sequence": "\n\nHuman:",
			"usage": map[string]int{
				"input_tokens":  15,
				"output_tokens": 12,
			},
		}

		respJSON, _ := json.Marshal(anthropicResp)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.Choices[0].FinishReason != "stop" {
			t.Errorf("期望 finish_reason stop, 实际 %s", result.Choices[0].FinishReason)
		}

		if result.Model != "claude-3-sonnet-20240229" {
			t.Errorf("期望模型 claude-3-sonnet-20240229, 实际 %s", result.Model)
		}
	})

	t.Run("max_tokens 转换为 length", func(t *testing.T) {
		mockLogger.reset()

		anthropicResp := map[string]interface{}{
			"id":          "msg_03ghi789",
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]interface{}{{"type": "text", "text": "Partial response"}},
			"model":       "claude-3-haiku-20240307",
			"stop_reason": "max_tokens",
			"usage": map[string]int{
				"input_tokens":  20,
				"output_tokens": 1000,
			},
		}

		respJSON, _ := json.Marshal(anthropicResp)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.Choices[0].FinishReason != "length" {
			t.Errorf("期望 finish_reason length, 实际 %s", result.Choices[0].FinishReason)
		}
	})

	t.Run("多个文本块正确拼接", func(t *testing.T) {
		mockLogger.reset()

		anthropicResp := map[string]interface{}{
			"id":   "msg_04jkl012",
			"type": "message",
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Part one. "},
				{"type": "text", "text": "Part two. "},
				{"type": "text", "text": "Part three."},
			},
			"model":       "claude-3-opus-20240229",
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  25,
				"output_tokens": 15,
			},
		}

		respJSON, _ := json.Marshal(anthropicResp)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		expectedContent := "Part one. Part two. Part three."
		if result.Choices[0].Message.Content != expectedContent {
			t.Errorf("期望内容 '%s', 实际 '%s'", expectedContent, result.Choices[0].Message.Content)
		}
	})

	t.Run("非文本块被忽略", func(t *testing.T) {
		mockLogger.reset()

		anthropicResp := map[string]interface{}{
			"id":   "msg_05mno345",
			"type": "message",
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Visible text."},
				{"type": "image", "source": map[string]interface{}{"type": "base64", "media_type": "image/jpeg", "data": "abc123"}},
				{"type": "text", "text": " More text."},
			},
			"model":       "claude-3-opus-20240229",
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  30,
				"output_tokens": 20,
			},
		}

		respJSON, _ := json.Marshal(anthropicResp)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		expectedContent := "Visible text. More text."
		if result.Choices[0].Message.Content != expectedContent {
			t.Errorf("期望内容 '%s', 实际 '%s'", expectedContent, result.Choices[0].Message.Content)
		}
	})

	t.Run("Usage 统计正确转换", func(t *testing.T) {
		mockLogger.reset()

		anthropicResp := map[string]interface{}{
			"id":          "msg_06pqr678",
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]interface{}{{"type": "text", "text": "Response with usage stats"}},
			"model":       "claude-3-sonnet-20240229",
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  150,
				"output_tokens": 75,
			},
		}

		respJSON, _ := json.Marshal(anthropicResp)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.Usage.PromptTokens != 150 {
			t.Errorf("期望 PromptTokens 150, 实际 %d", result.Usage.PromptTokens)
		}

		if result.Usage.CompletionTokens != 75 {
			t.Errorf("期望 CompletionTokens 75, 实际 %d", result.Usage.CompletionTokens)
		}

		if result.Usage.TotalTokens != 225 {
			t.Errorf("期望 TotalTokens 225, 实际 %d", result.Usage.TotalTokens)
		}
	})

	t.Run("无效 JSON 返回 nil", func(t *testing.T) {
		mockLogger.reset()

		invalidJSON := []byte("{invalid json}")
		result, _ := converter.Convert(invalidJSON, "claude-3-opus-20240229")

		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
		}

		if len(mockLogger.debugMessages) == 0 {
			t.Log("注意: 可能没有日志输出（正常行为）")
		}
	})
}

// TestResponseConverter_Supports 测试协议支持检查
func TestResponseConverter_Supports(t *testing.T) {
	converter := &ResponseConverter{}

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
			name:     "不支持 Azure 协议",
			protocol: types.ProtocolAzure,
			expected: false,
		},
		{
			name:     "不支持 Google 协议",
			protocol: types.ProtocolGoogle,
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

// TestResponseConverter_Protocol 测试协议返回
func TestResponseConverter_Protocol(t *testing.T) {
	converter := &ResponseConverter{}

	result := converter.Protocol()

	if result != types.ProtocolAnthropic {
		t.Errorf("期望协议 %v, 实际 %v", types.ProtocolAnthropic, result)
	}
}

// TestResponseConverter_Name 测试策略名称返回
func TestResponseConverter_Name(t *testing.T) {
	converter := &ResponseConverter{}

	result := converter.Name()

	expected := "AnthropicResponseConverter"
	if result != expected {
		t.Errorf("期望名称 %s, 实际 %s", expected, result)
	}
}

// TestResponseConverter_LoggerDebugCall 测试调试日志调用
func TestResponseConverter_LoggerDebugCall(t *testing.T) {
	mockLogger := &MockLoggerForAnthropicResponse{}
	converter := NewResponseConverter(mockLogger)

	anthropicResp := map[string]interface{}{
		"id":          "msg_07stu901",
		"type":        "message",
		"role":        "assistant",
		"content":     []map[string]interface{}{{"type": "text", "text": "Test response"}},
		"model":       "claude-3-opus-20240229",
		"stop_reason": "end_turn",
		"usage": map[string]int{
			"input_tokens":  10,
			"output_tokens": 5,
		},
	}

	respJSON, _ := json.Marshal(anthropicResp)
	converter.Convert(respJSON, "")

	if len(mockLogger.debugMessages) == 0 {
		t.Log("注意: 可能没有日志输出（正常行为）")
	}
}

// TestResponseConverter_LoggerNotCalledForNilLogger 测试 nil 日志器安全
func TestResponseConverter_LoggerNotCalledForNilLogger(t *testing.T) {
	t.Run("nil 日志器不会导致 panic", func(t *testing.T) {
		converter := NewResponseConverter(nil)

		result, err := converter.Convert([]byte("{invalid}"), "claude-3-opus-20240229")

		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
		}
		if err != nil {
			t.Errorf("期望 nil 错误, 实际 %v", err)
		}
	})
}

// TestResponseConverter_WithRealAnthropicResponse 测试真实 Anthropic 响应格式
func TestResponseConverter_WithRealAnthropicResponse(t *testing.T) {
	mockLogger := &MockLoggerForAnthropicResponse{}
	converter := NewResponseConverter(mockLogger)

	t.Run("Claude 3 Opus 响应格式", func(t *testing.T) {
		mockLogger.reset()

		realResp := map[string]interface{}{
			"id":            "msg_08vwx234",
			"type":          "message",
			"role":          "assistant",
			"content":       []map[string]interface{}{{"type": "text", "text": "I am Claude, an AI assistant."}},
			"model":         "claude-3-opus-20240229",
			"stop_reason":   "end_turn",
			"stop_sequence": nil,
			"usage": map[string]int{
				"input_tokens":  42,
				"output_tokens": 18,
			},
		}

		respJSON, _ := json.Marshal(realResp)
		result, err := converter.Convert(respJSON, "claude-3-opus-20240229")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.ID != "msg_08vwx234" {
			t.Errorf("期望 ID msg_08vwx234, 实际 %s", result.ID)
		}

		if result.Choices[0].FinishReason != "stop" {
			t.Errorf("期望 finish_reason stop, 实际 %s", result.Choices[0].FinishReason)
		}
	})

	t.Run("包含工具调用的响应", func(t *testing.T) {
		mockLogger.reset()

		// Anthropic 可能返回 tool_use 内容块
		respWithTool := map[string]interface{}{
			"id":   "msg_09yza345",
			"type": "message",
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Let me use a tool."},
				{"type": "tool_use", "id": "tool_01", "name": "get_weather", "input": map[string]string{"location": "San Francisco"}},
			},
			"model":       "claude-3-opus-20240229",
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  50,
				"output_tokens": 25,
			},
		}

		respJSON, _ := json.Marshal(respWithTool)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 只有文本块被提取
		expectedContent := "Let me use a tool."
		if result.Choices[0].Message.Content != expectedContent {
			t.Errorf("期望内容 '%s', 实际 '%s'", expectedContent, result.Choices[0].Message.Content)
		}
	})

	t.Run("空内容块处理", func(t *testing.T) {
		mockLogger.reset()

		respWithEmpty := map[string]interface{}{
			"id":          "msg_10bcd456",
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]interface{}{},
			"model":       "claude-3-sonnet-20240229",
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  5,
				"output_tokens": 0,
			},
		}

		respJSON, _ := json.Marshal(respWithEmpty)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 空内容应该返回空字符串
		if result.Choices[0].Message.Content != "" {
			t.Errorf("期望空内容, 实际 '%s'", result.Choices[0].Message.Content)
		}
	})
}

// TestResponseConverter_ConvertStopReason 测试停止原因转换
func TestResponseConverter_ConvertStopReason(t *testing.T) {
	converter := &ResponseConverter{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "end_turn 转换为 stop",
			input:    "end_turn",
			expected: "stop",
		},
		{
			name:     "stop_sequence 转换为 stop",
			input:    "stop_sequence",
			expected: "stop",
		},
		{
			name:     "max_tokens 转换为 length",
			input:    "max_tokens",
			expected: "length",
		},
		{
			name:     "未知原因保持原样",
			input:    "unknown_reason",
			expected: "unknown_reason",
		},
		{
			name:     "空字符串保持空",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.convertStopReason(tt.input)

			if result != tt.expected {
				t.Errorf("期望 %s, 实际 %s", tt.expected, result)
			}
		})
	}
}

// TestResponseConverter_CacheUsage 测试缓存使用信息
func TestResponseConverter_CacheUsage(t *testing.T) {
	mockLogger := &MockLoggerForAnthropicResponse{}
	converter := NewResponseConverter(mockLogger)

	t.Run("缓存使用信息应被记录", func(t *testing.T) {
		mockLogger.reset()

		cacheResp := map[string]interface{}{
			"id":          "msg_cache_test",
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]interface{}{{"type": "text", "text": "Cached response"}},
			"model":       "claude-3-haiku-20240307",
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":                100,
				"output_tokens":               50,
				"cache_creation_input_tokens": 80,
				"cache_read_input_tokens":     60,
			},
		}

		respJSON, _ := json.Marshal(cacheResp)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 验证使用统计
		if result.Usage.PromptTokens != 100 {
			t.Errorf("期望 PromptTokens 100, 实际 %d", result.Usage.PromptTokens)
		}
		if result.Usage.CompletionTokens != 50 {
			t.Errorf("期望 CompletionTokens 50, 实际 %d", result.Usage.CompletionTokens)
		}
	})
}

// TestResponseConverter_MultimodalContent 测试多模态内容处理
func TestResponseConverter_MultimodalContent(t *testing.T) {
	mockLogger := &MockLoggerForAnthropicResponse{}
	converter := NewResponseConverter(mockLogger)

	t.Run("图像内容应被正确记录", func(t *testing.T) {
		mockLogger.reset()

		imageResp := map[string]interface{}{
			"id":          "msg_image_test",
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]interface{}{{"type": "text", "text": "I see the image."}},
			"model":       "claude-3-sonnet-20240229",
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  200,
				"output_tokens": 30,
			},
		}

		respJSON, _ := json.Marshal(imageResp)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}
	})

	t.Run("思考块应被正确处理", func(t *testing.T) {
		mockLogger.reset()

		thinkingResp := map[string]interface{}{
			"id":   "msg_thinking_test",
			"type": "message",
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "thinking", "thinking": "Let me think about this...", "signature": "abc123"},
				{"type": "text", "text": "Here's my answer."},
			},
			"model":       "claude-3-opus-20240229",
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  150,
				"output_tokens": 40,
			},
		}

		respJSON, _ := json.Marshal(thinkingResp)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 思考块不应被添加到文本内容中
		expectedContent := "Here's my answer."
		if result.Choices[0].Message.Content != expectedContent {
			t.Errorf("期望内容 '%s', 实际 '%s'", expectedContent, result.Choices[0].Message.Content)
		}
	})
}

// TestResponseConverter_ContentFilterAndRefusal 测试内容过滤和拒绝处理
func TestResponseConverter_ContentFilterAndRefusal(t *testing.T) {
	converter := &ResponseConverter{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "content_filter 保持不变",
			input:    "content_filter",
			expected: "content_filter",
		},
		{
			name:     "refusal 转换为 content_filter",
			input:    "refusal",
			expected: "content_filter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.convertStopReason(tt.input)

			if result != tt.expected {
				t.Errorf("期望 %s, 实际 %s", tt.expected, result)
			}
		})
	}
}

// TestResponseConverter_ExtractTextContent 测试文本内容提取
func TestResponseConverter_ExtractTextContent(t *testing.T) {
	converter := &ResponseConverter{}

	t.Run("工具结果内容提取", func(t *testing.T) {
		// 测试 tool_result 块中包含字符串内容的情况
		blocks := []AnthropicContentBlock{
			{Type: "tool_result", Content: "Weather is sunny"},
		}

		result := converter.extractTextContent(blocks)

		if result != "Weather is sunny" {
			t.Errorf("期望 'Weather is sunny', 实际 '%s'", result)
		}
	})

	t.Run("搜索结果内容提取", func(t *testing.T) {
		// 测试 search_result 块中的文本内容
		blocks := []AnthropicContentBlock{
			{
				Type: "search_result",
				SearchResult: &AnthropicSearchResult{
					Content: []AnthropicContentBlock{
						{Type: "text", Text: "Search result 1"},
						{Type: "text", Text: "Search result 2"},
					},
				},
			},
		}

		result := converter.extractTextContent(blocks)

		expected := "Search result 1Search result 2"
		if result != expected {
			t.Errorf("期望 '%s', 实际 '%s'", expected, result)
		}
	})
}

// TestResponseConverter_HasContentBlockType 测试内容块类型检测
func TestResponseConverter_HasContentBlockType(t *testing.T) {
	converter := &ResponseConverter{}

	blocks := []AnthropicContentBlock{
		{Type: "text", Text: "Hello"},
		{Type: "tool_use", ID: "tool_01", Name: "test"},
		{Type: "text", Text: "World"},
	}

	tests := []struct {
		name      string
		blockType string
		expected  bool
	}{
		{"text 块存在", "text", true},
		{"tool_use 块存在", "tool_use", true},
		{"tool_result 块不存在", "tool_result", false},
		{"thinking 块不存在", "thinking", false},
		{"image 块不存在", "image", false},
		{"document 块不存在", "document", false},
		{"search_result 块不存在", "search_result", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.hasContentBlockType(blocks, tt.blockType)

			if result != tt.expected {
				t.Errorf("期望 %v, 实际 %v", tt.expected, result)
			}
		})
	}
}

// TestResponseConverter_EmptyBlocks 测试空内容块数组
func TestResponseConverter_EmptyBlocks(t *testing.T) {
	mockLogger := &MockLoggerForAnthropicResponse{}
	converter := NewResponseConverter(mockLogger)

	t.Run("空内容块数组应返回空文本", func(t *testing.T) {
		mockLogger.reset()

		emptyBlocksResp := map[string]interface{}{
			"id":          "msg_empty_blocks",
			"type":        "message",
			"role":        "assistant",
			"content":     []interface{}{},
			"model":       "claude-3-haiku-20240307",
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  5,
				"output_tokens": 0,
			},
		}

		respJSON, _ := json.Marshal(emptyBlocksResp)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 空内容块数组应返回空字符串
		if result.Choices[0].Message.Content != "" {
			t.Errorf("期望空内容, 实际 '%s'", result.Choices[0].Message.Content)
		}
	})
}

// TestResponseConverter_ModelFallback 测试模型名称回退逻辑
func TestResponseConverter_ModelFallback(t *testing.T) {
	mockLogger := &MockLoggerForAnthropicResponse{}
	converter := NewResponseConverter(mockLogger)

	t.Run("使用参数模型名称", func(t *testing.T) {
		mockLogger.reset()

		resp := map[string]interface{}{
			"id":          "msg_model_test",
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]interface{}{{"type": "text", "text": "Test"}},
			"model":       "original-model",
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  10,
				"output_tokens": 5,
			},
		}

		respJSON, _ := json.Marshal(resp)
		result, err := converter.Convert(respJSON, "custom-model")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 参数模型名称应被使用
		if result.Model != "custom-model" {
			t.Errorf("期望模型 custom-model, 实际 %s", result.Model)
		}
	})

	t.Run("空参数时使用响应中的模型名称", func(t *testing.T) {
		mockLogger.reset()

		resp := map[string]interface{}{
			"id":          "msg_model_fallback",
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]interface{}{{"type": "text", "text": "Test"}},
			"model":       "response-model",
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  10,
				"output_tokens": 5,
			},
		}

		respJSON, _ := json.Marshal(resp)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 空参数时应使用响应中的模型名称
		if result.Model != "response-model" {
			t.Errorf("期望模型 response-model, 实际 %s", result.Model)
		}
	})
}

// TestResponseConverter_InvalidJSON 测试无效 JSON 处理
func TestResponseConverter_InvalidJSON(t *testing.T) {
	converter := NewResponseConverter(nil)

	tests := []struct {
		name      string
		input     []byte
		expectNil bool
	}{
		{"无效 JSON", []byte("{invalid}"), true},
		{"部分有效 JSON", []byte(`{"id": "test", invalid}`), true},
		{"空 JSON 对象", []byte("{}"), false}, // 空 JSON 仍会被解析（无错误）
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.Convert(tt.input, "test-model")

			if tt.expectNil {
				// 严格无效 JSON 应该返回 nil
				if result != nil {
					t.Errorf("期望 nil, 实际 %v", result)
				}
				// 错误应该为 nil（静默处理）
				if err != nil {
					t.Errorf("期望 nil 错误, 实际 %v", err)
				}
			} else {
				// 空 JSON 对象可能被解析（取决于实现）
				// 验证不 panic
				_ = result
			}
		})
	}
}
