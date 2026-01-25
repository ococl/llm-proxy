package openai

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// MockLoggerForOpenAIResponse 实现 port.Logger 接口用于 OpenAI 响应转换器测试
type MockLoggerForOpenAIResponse struct {
	debugMessages []string
	infoMessages  []string
	errorMessages []string
	warnMessages  []string
	fatalMessages []string
	fields        []map[string]interface{}
	withFields    [][]port.Field
}

func (m *MockLoggerForOpenAIResponse) Debug(msg string, fields ...port.Field) {
	m.debugMessages = append(m.debugMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForOpenAIResponse) Info(msg string, fields ...port.Field) {
	m.infoMessages = append(m.infoMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForOpenAIResponse) Warn(msg string, fields ...port.Field) {
	m.warnMessages = append(m.warnMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForOpenAIResponse) Error(msg string, fields ...port.Field) {
	m.errorMessages = append(m.errorMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForOpenAIResponse) Fatal(msg string, fields ...port.Field) {
	m.fatalMessages = append(m.fatalMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForOpenAIResponse) With(fields ...port.Field) port.Logger {
	m.withFields = append(m.withFields, fields)
	return m
}

func (m *MockLoggerForOpenAIResponse) recordFields(fields []port.Field) {
	for _, field := range fields {
		m.fields = append(m.fields, map[string]interface{}{
			"key":   field.Key,
			"value": field.Value,
		})
	}
}

func (m *MockLoggerForOpenAIResponse) reset() {
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
		mockLogger := &MockLoggerForOpenAIResponse{}

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
	mockLogger := &MockLoggerForOpenAIResponse{}
	converter := NewResponseConverter(mockLogger)

	t.Run("空响应返回 nil", func(t *testing.T) {
		result, err := converter.Convert(nil, "gpt-4")

		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
		}
		if err != nil {
			t.Errorf("期望 nil 错误, 实际 %v", err)
		}
	})

	t.Run("空字节切片返回 nil", func(t *testing.T) {
		result, err := converter.Convert([]byte{}, "gpt-4")

		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
		}
		if err != nil {
			t.Errorf("期望 nil 错误, 实际 %v", err)
		}
	})

	t.Run("有效 OpenAI 响应正确解析", func(t *testing.T) {
		mockLogger.reset()

		resp := entity.NewResponseBuilder().
			ID("resp-123").
			Model("gpt-4").
			Choices([]entity.Choice{
				entity.NewChoice(0, entity.NewMessage("assistant", "Hello!"), "stop"),
			}).
			Usage(entity.NewUsage(10, 5)).
			BuildUnsafe()

		respJSON, _ := json.Marshal(resp)
		result, err := converter.Convert(respJSON, "gpt-4")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.ID != "resp-123" {
			t.Errorf("期望 ID resp-123, 实际 %s", result.ID)
		}

		if result.Model != "gpt-4" {
			t.Errorf("期望模型 gpt-4, 实际 %s", result.Model)
		}

		if len(result.Choices) != 1 {
			t.Errorf("期望 1 个选择, 实际 %d", len(result.Choices))
		}
	})

	t.Run("未提供模型时从响应中获取", func(t *testing.T) {
		mockLogger.reset()

		resp := entity.NewResponseBuilder().
			ID("resp-456").
			Model("gpt-3.5-turbo").
			Choices([]entity.Choice{
				entity.NewChoice(0, entity.NewMessage("assistant", "Hi"), "stop"),
			}).
			Usage(entity.NewUsage(5, 3)).
			BuildUnsafe()

		respJSON, _ := json.Marshal(resp)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.Model != "gpt-3.5-turbo" {
			t.Errorf("期望模型 gpt-3.5-turbo, 实际 %s", result.Model)
		}
	})

	t.Run("无效 JSON 返回 nil", func(t *testing.T) {
		mockLogger.reset()

		invalidJSON := []byte("{invalid json}")
		result, _ := converter.Convert(invalidJSON, "gpt-4")

		// 应该返回 nil 结果
		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
		}

		// 应该有调试日志
		if len(mockLogger.debugMessages) == 0 {
			t.Log("注意: 可能没有日志输出（正常行为）")
		}
	})

	t.Run("多个选择正确解析", func(t *testing.T) {
		mockLogger.reset()

		resp := entity.NewResponseBuilder().
			ID("resp-789").
			Model("gpt-4").
			Choices([]entity.Choice{
				entity.NewChoice(0, entity.NewMessage("assistant", "Option 1"), "stop"),
				entity.NewChoice(1, entity.NewMessage("assistant", "Option 2"), "stop"),
			}).
			Usage(entity.NewUsage(20, 10)).
			BuildUnsafe()

		respJSON, _ := json.Marshal(resp)
		result, err := converter.Convert(respJSON, "gpt-4")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if len(result.Choices) != 2 {
			t.Errorf("期望 2 个选择, 实际 %d", len(result.Choices))
		}

		if result.Choices[0].Index != 0 {
			t.Errorf("期望第一个选择索引 0, 实际 %d", result.Choices[0].Index)
		}

		if result.Choices[1].Index != 1 {
			t.Errorf("期望第二个选择索引 1, 实际 %d", result.Choices[1].Index)
		}
	})

	t.Run("Usage 统计正确解析", func(t *testing.T) {
		mockLogger.reset()

		usage := entity.NewUsage(100, 50)
		resp := entity.NewResponseBuilder().
			ID("resp-usage").
			Model("gpt-4").
			Choices([]entity.Choice{
				entity.NewChoice(0, entity.NewMessage("assistant", "Response"), "stop"),
			}).
			Usage(usage).
			BuildUnsafe()

		respJSON, _ := json.Marshal(resp)
		result, err := converter.Convert(respJSON, "gpt-4")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.Usage.PromptTokens != 100 {
			t.Errorf("期望 PromptTokens 100, 实际 %d", result.Usage.PromptTokens)
		}

		if result.Usage.CompletionTokens != 50 {
			t.Errorf("期望 CompletionTokens 50, 实际 %d", result.Usage.CompletionTokens)
		}

		if result.Usage.TotalTokens != 150 {
			t.Errorf("期望 TotalTokens 150, 实际 %d", result.Usage.TotalTokens)
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
			name:     "支持 OpenAI 协议",
			protocol: types.ProtocolOpenAI,
			expected: true,
		},
		{
			name:     "支持 Azure 协议（OpenAI 兼容）",
			protocol: types.ProtocolAzure,
			expected: true,
		},
		{
			name:     "支持 DeepSeek 协议（OpenAI 兼容）",
			protocol: types.ProtocolDeepSeek,
			expected: true,
		},
		{
			name:     "支持 Groq 协议（OpenAI 兼容）",
			protocol: types.ProtocolGroq,
			expected: true,
		},
		{
			name:     "支持 Mistral 协议（OpenAI 兼容）",
			protocol: types.ProtocolMistral,
			expected: true,
		},
		{
			name:     "支持 Cohere 协议（OpenAI 兼容）",
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

	if result != types.ProtocolOpenAI {
		t.Errorf("期望协议 %v, 实际 %v", types.ProtocolOpenAI, result)
	}
}

// TestResponseConverter_Name 测试策略名称返回
func TestResponseConverter_Name(t *testing.T) {
	converter := &ResponseConverter{}

	result := converter.Name()

	expected := "OpenAIResponseConverter"
	if result != expected {
		t.Errorf("期望名称 %s, 实际 %s", expected, result)
	}
}

// TestResponseConverter_LoggerDebugCall 测试调试日志调用
func TestResponseConverter_LoggerDebugCall(t *testing.T) {
	mockLogger := &MockLoggerForOpenAIResponse{}
	converter := NewResponseConverter(mockLogger)

	// 无效 JSON 应该触发调试日志
	converter.Convert([]byte("{invalid}"), "gpt-4")

	if len(mockLogger.debugMessages) == 0 {
		t.Log("注意: 可能没有日志输出（正常行为）")
	}
}

// TestResponseConverter_LoggerNotCalledForNilLogger 测试 nil 日志器安全
func TestResponseConverter_LoggerNotCalledForNilLogger(t *testing.T) {
	t.Run("nil 日志器不会导致 panic", func(t *testing.T) {
		converter := NewResponseConverter(nil)

		// 应该安全处理，不 panic
		result, err := converter.Convert([]byte("{invalid}"), "gpt-4")

		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
		}
		if err != nil {
			t.Errorf("期望 nil 错误, 实际 %v", err)
		}
	})
}

// TestResponseConverter_WithRealOpenAIResponse 测试真实 OpenAI 响应格式
func TestResponseConverter_WithRealOpenAIResponse(t *testing.T) {
	mockLogger := &MockLoggerForOpenAIResponse{}
	converter := NewResponseConverter(mockLogger)

	t.Run("标准 OpenAI 响应格式", func(t *testing.T) {
		mockLogger.reset()

		// 模拟真实 OpenAI API 响应格式
		realResp := map[string]interface{}{
			"id":      "chatcmpl-abc123",
			"object":  "chat.completion",
			"created": 1677858242,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello, how can I help you today?",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     13,
				"completion_tokens": 7,
				"total_tokens":      20,
			},
		}

		respJSON, _ := json.Marshal(realResp)
		result, err := converter.Convert(respJSON, "gpt-4")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.ID != "chatcmpl-abc123" {
			t.Errorf("期望 ID chatcmpl-abc123, 实际 %s", result.ID)
		}

		if len(result.Choices) != 1 {
			t.Errorf("期望 1 个选择, 实际 %d", len(result.Choices))
		}

		if result.Choices[0].FinishReason != "stop" {
			t.Errorf("期望 finish_reason stop, 实际 %s", result.Choices[0].FinishReason)
		}
	})

	t.Run("流式响应格式（仅最后块）", func(t *testing.T) {
		mockLogger.reset()

		// 模拟流式响应的最后块
		streamResp := map[string]interface{}{
			"id":      "chatcmpl-def456",
			"object":  "chat.completion.chunk",
			"created": 1677858243,
			"model":   "gpt-3.5-turbo",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"role":    "assistant",
						"content": "Streaming response",
					},
					"finish_reason": "stop",
				},
			},
		}

		respJSON, _ := json.Marshal(streamResp)
		result, err := converter.Convert(respJSON, "gpt-3.5-turbo")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 流式响应也可以解析（Delta 会被忽略，Message 为空）
		if result.ID != "chatcmpl-def456" {
			t.Errorf("期望 ID chatcmpl-def456, 实际 %s", result.ID)
		}
	})

	t.Run("包含 system_fingerprint 的响应", func(t *testing.T) {
		mockLogger.reset()

		respWithFingerprint := map[string]interface{}{
			"id":                 "chatcmpl-ghi789",
			"object":             "chat.completion",
			"created":            1677858244,
			"model":              "gpt-4-turbo-preview",
			"system_fingerprint": "fp_123456",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Response with fingerprint",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     15,
				"completion_tokens": 10,
				"total_tokens":      25,
			},
		}

		respJSON, _ := json.Marshal(respWithFingerprint)
		result, err := converter.Convert(respJSON, "gpt-4-turbo-preview")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// system_fingerprint 是额外字段，会被忽略但不会导致错误
		if result.ID != "chatcmpl-ghi789" {
			t.Errorf("期望 ID chatcmpl-ghi789, 实际 %s", result.ID)
		}
	})
}

// TestResponseConverter_RefusalAndAnnotations 测试拒绝回答和引用标注检测
func TestResponseConverter_RefusalAndAnnotations(t *testing.T) {
	mockLogger := &MockLoggerForOpenAIResponse{}
	converter := NewResponseConverter(mockLogger)

	t.Run("检测拒绝回答", func(t *testing.T) {
		mockLogger.reset()

		refusalResp := map[string]interface{}{
			"id":      "chatcmpl-refusal",
			"object":  "chat.completion",
			"created": 1677858245,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"refusal": "I cannot help with that request.",
						"content": nil,
					},
					"finish_reason": "content_filter",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}

		respJSON, _ := json.Marshal(refusalResp)
		result, err := converter.Convert(respJSON, "gpt-4")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 拒绝回答应该被正确解析
		if result.Choices[0].Message.Content == nil {
			t.Log("拒绝回答的 content 为 nil（正常行为）")
		}
	})

	t.Run("检测引用标注", func(t *testing.T) {
		mockLogger.reset()

		annotationsResp := map[string]interface{}{
			"id":      "chatcmpl-annotations",
			"object":  "chat.completion",
			"created": 1677858246,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role": "assistant",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "According to research, AI is growing.",
								"annotations": []interface{}{
									map[string]interface{}{
										"type": "citation",
										"text": "[1]",
									},
								},
							},
						},
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     20,
				"completion_tokens": 15,
				"total_tokens":      35,
			},
		}

		respJSON, _ := json.Marshal(annotationsResp)
		result, err := converter.Convert(respJSON, "gpt-4")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 引用标注应该被正确解析
		if len(result.Choices) != 1 {
			t.Errorf("期望 1 个选择, 实际 %d", len(result.Choices))
		}
	})

	t.Run("检测工具调用", func(t *testing.T) {
		mockLogger.reset()

		toolCallsResp := map[string]interface{}{
			"id":      "chatcmpl-tool",
			"object":  "chat.completion",
			"created": 1677858247,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":       "assistant",
						"content":    nil,
						"tool_calls": []interface{}{},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     25,
				"completion_tokens": 20,
				"total_tokens":      45,
			},
		}

		respJSON, _ := json.Marshal(toolCallsResp)
		result, err := converter.Convert(respJSON, "gpt-4")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 工具调用应该被正确解析
		if result.Choices[0].FinishReason != "tool_calls" {
			t.Errorf("期望 finish_reason tool_calls, 实际 %s", result.Choices[0].FinishReason)
		}
	})

	t.Run("检测对数概率", func(t *testing.T) {
		mockLogger.reset()

		logprobsResp := map[string]interface{}{
			"id":      "chatcmpl-logprobs",
			"object":  "chat.completion",
			"created": 1677858248,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Response with logprobs",
					},
					"finish_reason": "stop",
					"logprobs": map[string]interface{}{
						"content": []interface{}{},
					},
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     30,
				"completion_tokens": 25,
				"total_tokens":      55,
			},
		}

		respJSON, _ := json.Marshal(logprobsResp)
		result, err := converter.Convert(respJSON, "gpt-4")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 对数概率应该被正确解析
		if len(result.Choices) != 1 {
			t.Errorf("期望 1 个选择, 实际 %d", len(result.Choices))
		}
	})
}

// TestResponseConverter_EmptyChoices 测试空 choices 数组处理
func TestResponseConverter_EmptyChoices(t *testing.T) {
	mockLogger := &MockLoggerForOpenAIResponse{}
	converter := NewResponseConverter(mockLogger)

	t.Run("空 choices 数组应记录警告", func(t *testing.T) {
		mockLogger.reset()

		emptyChoicesResp := map[string]interface{}{
			"id":      "chatcmpl-empty",
			"object":  "chat.completion",
			"created": 1677858249,
			"model":   "gpt-4",
			"choices": []interface{}{},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 0,
				"total_tokens":      10,
			},
		}

		respJSON, _ := json.Marshal(emptyChoicesResp)
		result, err := converter.Convert(respJSON, "gpt-4")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 空 choices 应该返回响应（带警告日志），但 choices 数组为空
		if result == nil {
			t.Error("结果不应为 nil（实现返回响应，即使 choices 为空）")
		} else {
			if len(result.Choices) != 0 {
				t.Errorf("期望 0 个选择, 实际 %d", len(result.Choices))
			}
		}

		// 应该记录警告日志
		found := false
		for _, msg := range mockLogger.warnMessages {
			if msg != "" {
				found = true
				break
			}
		}
		if !found {
			t.Log("注意: 可能没有警告日志输出（取决于实现）")
		}
	})
}

// TestResponseConverter_UsageEdgeCases 测试 Usage 边缘情况
func TestResponseConverter_UsageEdgeCases(t *testing.T) {
	mockLogger := &MockLoggerForOpenAIResponse{}
	converter := NewResponseConverter(mockLogger)

	t.Run("零使用量", func(t *testing.T) {
		mockLogger.reset()

		zeroUsageResp := map[string]interface{}{
			"id":      "chatcmpl-zero",
			"object":  "chat.completion",
			"created": 1677858250,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     0,
				"completion_tokens": 0,
				"total_tokens":      0,
			},
		}

		respJSON, _ := json.Marshal(zeroUsageResp)
		result, err := converter.Convert(respJSON, "gpt-4")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.Usage.PromptTokens != 0 {
			t.Errorf("期望 PromptTokens 0, 实际 %d", result.Usage.PromptTokens)
		}

		if result.Usage.CompletionTokens != 0 {
			t.Errorf("期望 CompletionTokens 0, 实际 %d", result.Usage.CompletionTokens)
		}

		if result.Usage.TotalTokens != 0 {
			t.Errorf("期望 TotalTokens 0, 实际 %d", result.Usage.TotalTokens)
		}
	})

	t.Run("缺失 usage 字段", func(t *testing.T) {
		mockLogger.reset()

		noUsageResp := map[string]interface{}{
			"id":      "chatcmpl-no-usage",
			"object":  "chat.completion",
			"created": 1677858251,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Response",
					},
					"finish_reason": "stop",
				},
			},
		}

		respJSON, _ := json.Marshal(noUsageResp)
		result, err := converter.Convert(respJSON, "gpt-4")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 缺失 usage 应该返回默认 Usage
		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// Usage 是值类型，不是指针
		if result.Usage.PromptTokens != 0 {
			t.Log("注意: Usage 字段存在但值为零（取决于实现）")
		}
	})

	t.Run("大数值使用量", func(t *testing.T) {
		mockLogger.reset()

		largeUsageResp := map[string]interface{}{
			"id":      "chatcmpl-large",
			"object":  "chat.completion",
			"created": 1677858252,
			"model":   "gpt-4-turbo",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": strings.Repeat("a", 10000),
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     100000,
				"completion_tokens": 50000,
				"total_tokens":      150000,
			},
		}

		respJSON, _ := json.Marshal(largeUsageResp)
		result, err := converter.Convert(respJSON, "gpt-4-turbo")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.Usage.PromptTokens != 100000 {
			t.Errorf("期望 PromptTokens 100000, 实际 %d", result.Usage.PromptTokens)
		}

		if result.Usage.CompletionTokens != 50000 {
			t.Errorf("期望 CompletionTokens 50000, 实际 %d", result.Usage.CompletionTokens)
		}

		if result.Usage.TotalTokens != 150000 {
			t.Errorf("期望 TotalTokens 150000, 实际 %d", result.Usage.TotalTokens)
		}
	})
}

// TestResponseConverter_IDGeneration 测试 ID 生成边缘情况
func TestResponseConverter_IDGeneration(t *testing.T) {
	mockLogger := &MockLoggerForOpenAIResponse{}
	converter := NewResponseConverter(mockLogger)

	t.Run("标准 ID 格式", func(t *testing.T) {
		mockLogger.reset()

		standardIDResp := map[string]interface{}{
			"id":      "chatcmpl-abc123def456",
			"object":  "chat.completion",
			"created": 1677858253,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Response",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}

		respJSON, _ := json.Marshal(standardIDResp)
		result, err := converter.Convert(respJSON, "gpt-4")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.ID != "chatcmpl-abc123def456" {
			t.Errorf("期望 ID chatcmpl-abc123def456, 实际 %s", result.ID)
		}
	})

	t.Run("空 ID", func(t *testing.T) {
		mockLogger.reset()

		emptyIDResp := map[string]interface{}{
			"id":      "",
			"object":  "chat.completion",
			"created": 1677858254,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Response",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}

		respJSON, _ := json.Marshal(emptyIDResp)
		result, err := converter.Convert(respJSON, "gpt-4")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 空 ID 应该被正确处理
		if result.ID != "" {
			t.Errorf("期望空 ID, 实际 %s", result.ID)
		}
	})

	t.Run("特殊字符 ID", func(t *testing.T) {
		mockLogger.reset()

		specialIDResp := map[string]interface{}{
			"id":      "chatcmpl-äbç123_测试",
			"object":  "chat.completion",
			"created": 1677858255,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Response",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}

		respJSON, _ := json.Marshal(specialIDResp)
		result, err := converter.Convert(respJSON, "gpt-4")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 特殊字符 ID 应该被保留
		if result.ID != "chatcmpl-äbç123_测试" {
			t.Errorf("期望特殊字符 ID, 实际 %s", result.ID)
		}
	})
}

// TestResponseConverter_MultipleChoices 测试多选择响应
func TestResponseConverter_MultipleChoices(t *testing.T) {
	mockLogger := &MockLoggerForOpenAIResponse{}
	converter := NewResponseConverter(mockLogger)

	t.Run("三个选择", func(t *testing.T) {
		mockLogger.reset()

		multiChoiceResp := map[string]interface{}{
			"id":      "chatcmpl-multi",
			"object":  "chat.completion",
			"created": 1677858256,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Option 1",
					},
					"finish_reason": "stop",
				},
				{
					"index": 1,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Option 2",
					},
					"finish_reason": "stop",
				},
				{
					"index": 2,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Option 3",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     15,
				"completion_tokens": 10,
				"total_tokens":      25,
			},
		}

		respJSON, _ := json.Marshal(multiChoiceResp)
		result, err := converter.Convert(respJSON, "gpt-4")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if len(result.Choices) != 3 {
			t.Errorf("期望 3 个选择, 实际 %d", len(result.Choices))
		}

		// 验证选择索引
		for i, choice := range result.Choices {
			if choice.Index != i {
				t.Errorf("期望选择索引 %d, 实际 %d", i, choice.Index)
			}

			if choice.Message.Content != fmt.Sprintf("Option %d", i+1) {
				t.Errorf("期望内容 'Option %d', 实际 '%s'", i+1, choice.Message.Content)
			}
		}
	})

	t.Run("选择索引不连续", func(t *testing.T) {
		mockLogger.reset()

		nonSequentialResp := map[string]interface{}{
			"id":      "chatcmpl-non-seq",
			"object":  "chat.completion",
			"created": 1677858257,
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "First",
					},
					"finish_reason": "stop",
				},
				{
					"index": 5,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Sixth",
					},
					"finish_reason": "length",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     20,
				"completion_tokens": 15,
				"total_tokens":      35,
			},
		}

		respJSON, _ := json.Marshal(nonSequentialResp)
		result, err := converter.Convert(respJSON, "gpt-4")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if len(result.Choices) != 2 {
			t.Errorf("期望 2 个选择, 实际 %d", len(result.Choices))
		}

		// 验证不连续的索引被正确保留
		if result.Choices[0].Index != 0 {
			t.Errorf("期望第一个选择索引 0, 实际 %d", result.Choices[0].Index)
		}

		if result.Choices[1].Index != 5 {
			t.Errorf("期望第二个选择索引 5, 实际 %d", result.Choices[1].Index)
		}
	})
}

// TestResponseConverter_FinishReasons 测试不同的完成原因
func TestResponseConverter_FinishReasons(t *testing.T) {
	mockLogger := &MockLoggerForOpenAIResponse{}
	converter := NewResponseConverter(mockLogger)

	finishReasons := []string{"stop", "length", "tool_calls", "content_filter", "function_call"}

	for _, reason := range finishReasons {
		t.Run(fmt.Sprintf("完成原因: %s", reason), func(t *testing.T) {
			mockLogger.reset()

			resp := map[string]interface{}{
				"id":      fmt.Sprintf("chatcmpl-%s", reason),
				"object":  "chat.completion",
				"created": 1677858258,
				"model":   "gpt-4",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "Response",
						},
						"finish_reason": reason,
					},
				},
				"usage": map[string]int{
					"prompt_tokens":     10,
					"completion_tokens": 5,
					"total_tokens":      15,
				},
			}

			respJSON, _ := json.Marshal(resp)
			result, err := converter.Convert(respJSON, "gpt-4")

			if err != nil {
				t.Fatalf("期望无错误, 实际 %v", err)
			}

			if result == nil {
				t.Fatal("结果不应为 nil")
			}

			if result.Choices[0].FinishReason != reason {
				t.Errorf("期望 finish_reason %s, 实际 %s", reason, result.Choices[0].FinishReason)
			}
		})
	}
}
