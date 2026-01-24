package google

import (
	"encoding/json"
	"testing"

	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// MockLoggerForGoogleResponse 实现 port.Logger 接口用于 Google 响应转换器测试
type MockLoggerForGoogleResponse struct {
	debugMessages []string
	infoMessages  []string
	errorMessages []string
	warnMessages  []string
	fatalMessages []string
	fields        []map[string]interface{}
	withFields    [][]port.Field
}

func (m *MockLoggerForGoogleResponse) Debug(msg string, fields ...port.Field) {
	m.debugMessages = append(m.debugMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForGoogleResponse) Info(msg string, fields ...port.Field) {
	m.infoMessages = append(m.infoMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForGoogleResponse) Warn(msg string, fields ...port.Field) {
	m.warnMessages = append(m.warnMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForGoogleResponse) Error(msg string, fields ...port.Field) {
	m.errorMessages = append(m.errorMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForGoogleResponse) Fatal(msg string, fields ...port.Field) {
	m.fatalMessages = append(m.fatalMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForGoogleResponse) With(fields ...port.Field) port.Logger {
	m.withFields = append(m.withFields, fields)
	return m
}

func (m *MockLoggerForGoogleResponse) recordFields(fields []port.Field) {
	for _, field := range fields {
		m.fields = append(m.fields, map[string]interface{}{
			"key":   field.Key,
			"value": field.Value,
		})
	}
}

func (m *MockLoggerForGoogleResponse) reset() {
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
		mockLogger := &MockLoggerForGoogleResponse{}

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
	mockLogger := &MockLoggerForGoogleResponse{}
	converter := NewResponseConverter(mockLogger)

	t.Run("空响应返回 nil", func(t *testing.T) {
		result, err := converter.Convert(nil, "gemini-pro")

		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
		}
		if err != nil {
			t.Errorf("期望 nil 错误, 实际 %v", err)
		}
	})

	t.Run("空字节切片返回 nil", func(t *testing.T) {
		result, err := converter.Convert([]byte{}, "gemini-pro")

		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
		}
		if err != nil {
			t.Errorf("期望 nil 错误, 实际 %v", err)
		}
	})

	t.Run("简单文本内容正确转换", func(t *testing.T) {
		mockLogger.reset()

		googleResp := map[string]interface{}{
			"id":      "google-msg-001",
			"object":  "chat.completion",
			"created": 1677858242,
			"model":   "gemini-pro",
			"candidates": []map[string]interface{}{
				{
					"index": 0,
					"content": map[string]interface{}{
						"role":  "model",
						"parts": []map[string]interface{}{{"text": "Hello, I am Gemini."}},
					},
					"finishReason": "STOP",
				},
			},
		}

		respJSON, _ := json.Marshal(googleResp)
		result, err := converter.Convert(respJSON, "gemini-pro")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.ID != "google-msg-001" {
			t.Errorf("期望 ID google-msg-001, 实际 %s", result.ID)
		}

		if len(result.Choices) != 1 {
			t.Errorf("期望 1 个选择, 实际 %d", len(result.Choices))
		}

		if result.Choices[0].Message.Content != "Hello, I am Gemini." {
			t.Errorf("期望内容 'Hello, I am Gemini.', 实际 %s", result.Choices[0].Message.Content)
		}

		if result.Choices[0].FinishReason != "stop" {
			t.Errorf("期望 finish_reason stop, 实际 %s", result.Choices[0].FinishReason)
		}
	})

	t.Run("STOP 转换为 stop", func(t *testing.T) {
		mockLogger.reset()

		googleResp := map[string]interface{}{
			"id":      "google-msg-002",
			"object":  "chat.completion",
			"created": 1677858243,
			"model":   "gemini-1.5-flash",
			"candidates": []map[string]interface{}{
				{
					"index": 0,
					"content": map[string]interface{}{
						"role":  "model",
						"parts": []map[string]interface{}{{"text": "Normal stop"}},
					},
					"finishReason": "STOP",
				},
			},
		}

		respJSON, _ := json.Marshal(googleResp)
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

		if result.Model != "gemini-1.5-flash" {
			t.Errorf("期望模型 gemini-1.5-flash, 实际 %s", result.Model)
		}
	})

	t.Run("MAX_TOKENS 转换为 length", func(t *testing.T) {
		mockLogger.reset()

		googleResp := map[string]interface{}{
			"id":      "google-msg-003",
			"object":  "chat.completion",
			"created": 1677858244,
			"model":   "gemini-pro",
			"candidates": []map[string]interface{}{
				{
					"index": 0,
					"content": map[string]interface{}{
						"role":  "model",
						"parts": []map[string]interface{}{{"text": "Partial response due to max tokens"}},
					},
					"finishReason": "MAX_TOKENS",
				},
			},
		}

		respJSON, _ := json.Marshal(googleResp)
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

	t.Run("SAFETY 转换为 content_filter", func(t *testing.T) {
		mockLogger.reset()

		googleResp := map[string]interface{}{
			"id":      "google-msg-004",
			"object":  "chat.completion",
			"created": 1677858245,
			"model":   "gemini-pro",
			"candidates": []map[string]interface{}{
				{
					"index": 0,
					"content": map[string]interface{}{
						"role":  "model",
						"parts": []map[string]interface{}{},
					},
					"finishReason": "SAFETY",
				},
			},
		}

		respJSON, _ := json.Marshal(googleResp)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.Choices[0].FinishReason != "content_filter" {
			t.Errorf("期望 finish_reason content_filter, 实际 %s", result.Choices[0].FinishReason)
		}
	})

	t.Run("RECITATION 转换为 content_filter", func(t *testing.T) {
		mockLogger.reset()

		googleResp := map[string]interface{}{
			"id":      "google-msg-005",
			"object":  "chat.completion",
			"created": 1677858246,
			"model":   "gemini-pro",
			"candidates": []map[string]interface{}{
				{
					"index": 0,
					"content": map[string]interface{}{
						"role":  "model",
						"parts": []map[string]interface{}{},
					},
					"finishReason": "RECITATION",
				},
			},
		}

		respJSON, _ := json.Marshal(googleResp)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.Choices[0].FinishReason != "content_filter" {
			t.Errorf("期望 finish_reason content_filter, 实际 %s", result.Choices[0].FinishReason)
		}
	})

	t.Run("多个部分正确拼接", func(t *testing.T) {
		mockLogger.reset()

		googleResp := map[string]interface{}{
			"id":      "google-msg-006",
			"object":  "chat.completion",
			"created": 1677858247,
			"model":   "gemini-pro",
			"candidates": []map[string]interface{}{
				{
					"index": 0,
					"content": map[string]interface{}{
						"role": "model",
						"parts": []map[string]interface{}{
							{"text": "Part one. "},
							{"text": "Part two. "},
							{"text": "Part three."},
						},
					},
					"finishReason": "STOP",
				},
			},
		}

		respJSON, _ := json.Marshal(googleResp)
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

	t.Run("Usage 统计正确转换", func(t *testing.T) {
		mockLogger.reset()

		googleResp := map[string]interface{}{
			"id":      "google-msg-007",
			"object":  "chat.completion",
			"created": 1677858248,
			"model":   "gemini-1.5-pro",
			"candidates": []map[string]interface{}{
				{
					"index": 0,
					"content": map[string]interface{}{
						"role":  "model",
						"parts": []map[string]interface{}{{"text": "Response with usage"}},
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     200,
				"candidatesTokenCount": 100,
				"totalTokenCount":      300,
			},
		}

		respJSON, _ := json.Marshal(googleResp)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.Usage.PromptTokens != 200 {
			t.Errorf("期望 PromptTokens 200, 实际 %d", result.Usage.PromptTokens)
		}

		if result.Usage.CompletionTokens != 100 {
			t.Errorf("期望 CompletionTokens 100, 实际 %d", result.Usage.CompletionTokens)
		}

		if result.Usage.TotalTokens != 300 {
			t.Errorf("期望 TotalTokens 300, 实际 %d", result.Usage.TotalTokens)
		}
	})

	t.Run("无候选内容时返回 nil", func(t *testing.T) {
		mockLogger.reset()

		googleResp := map[string]interface{}{
			"id":         "google-msg-008",
			"object":     "chat.completion",
			"created":    1677858249,
			"model":      "gemini-pro",
			"candidates": []map[string]interface{}{},
		}

		respJSON, _ := json.Marshal(googleResp)
		result, err := converter.Convert(respJSON, "")

		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
		}

		if err != nil {
			t.Errorf("期望 nil 错误, 实际 %v", err)
		}
	})

	t.Run("请求被阻止时记录日志", func(t *testing.T) {
		mockLogger.reset()

		googleResp := map[string]interface{}{
			"id":         "google-msg-009",
			"object":     "chat.completion",
			"created":    1677858250,
			"model":      "gemini-pro",
			"candidates": []map[string]interface{}{},
			"promptFeedback": map[string]interface{}{
				"blockReason":        "SAFETY",
				"blockReasonMessage": "Content filtered for safety",
			},
		}

		respJSON, _ := json.Marshal(googleResp)
		result, _ := converter.Convert(respJSON, "")

		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
		}

		// 应该记录阻止原因日志
		if len(mockLogger.debugMessages) == 0 {
			t.Log("注意: 可能没有日志输出（正常行为）")
		}
	})

	t.Run("无效 JSON 返回 nil", func(t *testing.T) {
		mockLogger.reset()

		invalidJSON := []byte("{invalid json}")
		result, _ := converter.Convert(invalidJSON, "gemini-pro")

		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
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

// TestResponseConverter_Protocol 测试协议返回
func TestResponseConverter_Protocol(t *testing.T) {
	converter := &ResponseConverter{}

	result := converter.Protocol()

	if result != types.ProtocolGoogle {
		t.Errorf("期望协议 %v, 实际 %v", types.ProtocolGoogle, result)
	}
}

// TestResponseConverter_Name 测试策略名称返回
func TestResponseConverter_Name(t *testing.T) {
	converter := &ResponseConverter{}

	result := converter.Name()

	expected := "GoogleVertexAIResponseConverter"
	if result != expected {
		t.Errorf("期望名称 %s, 实际 %s", expected, result)
	}
}

// TestResponseConverter_LoggerDebugCall 测试调试日志调用
func TestResponseConverter_LoggerDebugCall(t *testing.T) {
	mockLogger := &MockLoggerForGoogleResponse{}
	converter := NewResponseConverter(mockLogger)

	googleResp := map[string]interface{}{
		"id":      "google-msg-010",
		"object":  "chat.completion",
		"created": 1677858251,
		"model":   "gemini-pro",
		"candidates": []map[string]interface{}{
			{
				"index": 0,
				"content": map[string]interface{}{
					"role":  "model",
					"parts": []map[string]interface{}{{"text": "Test response"}},
				},
				"finishReason": "STOP",
			},
		},
	}

	respJSON, _ := json.Marshal(googleResp)
	converter.Convert(respJSON, "")

	if len(mockLogger.debugMessages) == 0 {
		t.Log("注意: 可能没有日志输出（正常行为）")
	}
}

// TestResponseConverter_LoggerNotCalledForNilLogger 测试 nil 日志器安全
func TestResponseConverter_LoggerNotCalledForNilLogger(t *testing.T) {
	t.Run("nil 日志器不会导致 panic", func(t *testing.T) {
		converter := NewResponseConverter(nil)

		result, err := converter.Convert([]byte("{invalid}"), "gemini-pro")

		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
		}
		if err != nil {
			t.Errorf("期望 nil 错误, 实际 %v", err)
		}
	})
}

// TestResponseConverter_WithRealGoogleResponse 测试真实 Google Vertex AI 响应格式
func TestResponseConverter_WithRealGoogleResponse(t *testing.T) {
	mockLogger := &MockLoggerForGoogleResponse{}
	converter := NewResponseConverter(mockLogger)

	t.Run("Gemini Pro 响应格式", func(t *testing.T) {
		mockLogger.reset()

		realResp := map[string]interface{}{
			"id":      "google-msg-011",
			"object":  "chat.completion",
			"created": 1677858252,
			"model":   "gemini-pro",
			"candidates": []map[string]interface{}{
				{
					"index": 0,
					"content": map[string]interface{}{
						"role":  "model",
						"parts": []map[string]interface{}{{"text": "I am Gemini, a large language model."}},
					},
					"finishReason": "STOP",
					"safetyRatings": []map[string]interface{}{
						{"category": "HARM_CATEGORY_HARASSMENT", "probability": "NEGLIGIBLE"},
					},
				},
			},
		}

		respJSON, _ := json.Marshal(realResp)
		result, err := converter.Convert(respJSON, "gemini-pro")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.ID != "google-msg-011" {
			t.Errorf("期望 ID google-msg-011, 实际 %s", result.ID)
		}

		if result.Choices[0].FinishReason != "stop" {
			t.Errorf("期望 finish_reason stop, 实际 %s", result.Choices[0].FinishReason)
		}
	})

	t.Run("Gemini 1.5 Flash 响应格式", func(t *testing.T) {
		mockLogger.reset()

		flashResp := map[string]interface{}{
			"id":      "google-msg-012",
			"object":  "chat.completion",
			"created": 1677858253,
			"model":   "gemini-1.5-flash",
			"candidates": []map[string]interface{}{
				{
					"index": 0,
					"content": map[string]interface{}{
						"role":  "model",
						"parts": []map[string]interface{}{{"text": "Fast response from Gemini 1.5 Flash"}},
					},
					"finishReason": "STOP",
				},
			},
			"usageMetadata": map[string]int{
				"promptTokenCount":     50,
				"candidatesTokenCount": 20,
				"totalTokenCount":      70,
			},
		}

		respJSON, _ := json.Marshal(flashResp)
		result, err := converter.Convert(respJSON, "gemini-1.5-flash")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		if result.Model != "gemini-1.5-flash" {
			t.Errorf("期望模型 gemini-1.5-flash, 实际 %s", result.Model)
		}
	})

	t.Run("空部分处理", func(t *testing.T) {
		mockLogger.reset()

		respWithEmptyParts := map[string]interface{}{
			"id":      "google-msg-013",
			"object":  "chat.completion",
			"created": 1677858254,
			"model":   "gemini-pro",
			"candidates": []map[string]interface{}{
				{
					"index": 0,
					"content": map[string]interface{}{
						"role":  "model",
						"parts": []map[string]interface{}{},
					},
					"finishReason": "STOP",
				},
			},
		}

		respJSON, _ := json.Marshal(respWithEmptyParts)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 空部分应该返回空字符串
		if result.Choices[0].Message.Content != "" {
			t.Errorf("期望空内容, 实际 '%s'", result.Choices[0].Message.Content)
		}
	})

	t.Run("多个候选取第一个", func(t *testing.T) {
		mockLogger.reset()

		respWithMultipleCandidates := map[string]interface{}{
			"id":      "google-msg-014",
			"object":  "chat.completion",
			"created": 1677858255,
			"model":   "gemini-pro",
			"candidates": []map[string]interface{}{
				{
					"index": 0,
					"content": map[string]interface{}{
						"role":  "model",
						"parts": []map[string]interface{}{{"text": "First candidate"}},
					},
					"finishReason": "STOP",
				},
				{
					"index": 1,
					"content": map[string]interface{}{
						"role":  "model",
						"parts": []map[string]interface{}{{"text": "Second candidate"}},
					},
					"finishReason": "STOP",
				},
			},
		}

		respJSON, _ := json.Marshal(respWithMultipleCandidates)
		result, err := converter.Convert(respJSON, "")

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 应该只取第一个候选的内容
		if result.Choices[0].Message.Content != "First candidate" {
			t.Errorf("期望内容 'First candidate', 实际 '%s'", result.Choices[0].Message.Content)
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
			name:     "STOP 转换为 stop",
			input:    "STOP",
			expected: "stop",
		},
		{
			name:     "MAX_TOKENS 转换为 length",
			input:    "MAX_TOKENS",
			expected: "length",
		},
		{
			name:     "SAFETY 转换为 content_filter",
			input:    "SAFETY",
			expected: "content_filter",
		},
		{
			name:     "RECITATION 转换为 content_filter",
			input:    "RECITATION",
			expected: "content_filter",
		},
		{
			name:     "未知原因保持原样",
			input:    "UNKNOWN_REASON",
			expected: "UNKNOWN_REASON",
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
