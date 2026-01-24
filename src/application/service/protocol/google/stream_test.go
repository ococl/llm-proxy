package google

import (
	"encoding/json"
	"testing"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// MockLoggerForGoogleStream 实现 port.Logger 接口用于 Google 流式转换器测试
type MockLoggerForGoogleStream struct {
	debugMessages []string
	infoMessages  []string
	errorMessages []string
	warnMessages  []string
	fatalMessages []string
	fields        []map[string]interface{}
	withFields    [][]port.Field
}

func (m *MockLoggerForGoogleStream) Debug(msg string, fields ...port.Field) {
	m.debugMessages = append(m.debugMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForGoogleStream) Info(msg string, fields ...port.Field) {
	m.infoMessages = append(m.infoMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForGoogleStream) Warn(msg string, fields ...port.Field) {
	m.warnMessages = append(m.warnMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForGoogleStream) Error(msg string, fields ...port.Field) {
	m.errorMessages = append(m.errorMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForGoogleStream) Fatal(msg string, fields ...port.Field) {
	m.fatalMessages = append(m.fatalMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForGoogleStream) With(fields ...port.Field) port.Logger {
	m.withFields = append(m.withFields, fields)
	return m
}

func (m *MockLoggerForGoogleStream) recordFields(fields []port.Field) {
	for _, field := range fields {
		m.fields = append(m.fields, map[string]interface{}{
			"key":   field.Key,
			"value": field.Value,
		})
	}
}

func (m *MockLoggerForGoogleStream) reset() {
	m.debugMessages = nil
	m.infoMessages = nil
	m.errorMessages = nil
	m.warnMessages = nil
	m.fatalMessages = nil
	m.fields = nil
	m.withFields = nil
}

// TestStreamChunkConverter_NewStreamChunkConverter 测试流式块转换器创建
func TestStreamChunkConverter_NewStreamChunkConverter(t *testing.T) {
	t.Run("使用有效日志器创建", func(t *testing.T) {
		mockLogger := &MockLoggerForGoogleStream{}

		converter := NewStreamChunkConverter(mockLogger)

		if converter == nil {
			t.Fatal("转换器不应为 nil")
		}
	})

	t.Run("使用 nil 日志器创建时使用 NopLogger", func(t *testing.T) {
		converter := NewStreamChunkConverter(nil)

		if converter == nil {
			t.Fatal("转换器不应为 nil")
		}

		if converter.logger == nil {
			t.Error("日志器不应为 nil")
		}
	})
}

// TestStreamChunkConverter_ParseChunk 测试解析流式数据块
func TestStreamChunkConverter_ParseChunk(t *testing.T) {
	mockLogger := &MockLoggerForGoogleStream{}
	converter := NewStreamChunkConverter(mockLogger)

	t.Run("空数据返回 nil", func(t *testing.T) {
		mockLogger.reset()

		chunk, err := converter.ParseChunk([]byte{})

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk != nil {
			t.Errorf("期望 nil, 实际 %v", chunk)
		}
	})

	t.Run("nil 数据返回 nil", func(t *testing.T) {
		mockLogger.reset()

		chunk, err := converter.ParseChunk(nil)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk != nil {
			t.Errorf("期望 nil, 实际 %v", chunk)
		}
	})

	t.Run("解析 chunk 格式数据", func(t *testing.T) {
		mockLogger.reset()

		googleChunk := map[string]interface{}{
			"id":      "google-stream-001",
			"object":  "chat.completion.chunk",
			"created": 1677858242,
			"model":   "gemini-pro",
			"chunk":   "Hello",
		}

		data, _ := json.Marshal(googleChunk)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if chunk.Finished {
			t.Error("期望 Finished 为 false")
		}

		if chunk.Content != "Hello" {
			t.Errorf("期望内容 'Hello', 实际 '%s'", chunk.Content)
		}
	})

	t.Run("解析 usageMetadata 结束标记", func(t *testing.T) {
		mockLogger.reset()

		googleChunk := map[string]interface{}{
			"id":      "google-stream-002",
			"object":  "chat.completion.chunk",
			"created": 1677858243,
			"model":   "gemini-pro",
			"usageMetadata": map[string]int{
				"promptTokenCount":     10,
				"candidatesTokenCount": 20,
				"totalTokenCount":      30,
			},
		}

		data, _ := json.Marshal(googleChunk)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if !chunk.Finished {
			t.Error("期望 Finished 为 true")
		}

		if chunk.StopReason != "stop" {
			t.Errorf("期望 stop_reason stop, 实际 %s", chunk.StopReason)
		}
	})

	t.Run("解析 candidates 内容", func(t *testing.T) {
		mockLogger.reset()

		googleChunk := map[string]interface{}{
			"id":      "google-stream-003",
			"object":  "chat.completion.chunk",
			"created": 1677858244,
			"model":   "gemini-pro",
			"candidates": []map[string]interface{}{
				{
					"index": 0,
					"content": map[string]interface{}{
						"role": "model",
						"parts": []map[string]interface{}{
							{"text": "Part 1"},
							{"text": "Part 2"},
						},
					},
				},
			},
		}

		data, _ := json.Marshal(googleChunk)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if chunk.Finished {
			t.Error("期望 Finished 为 false")
		}

		expectedContent := "Part 1Part 2"
		if chunk.Content != expectedContent {
			t.Errorf("期望内容 '%s', 实际 '%s'", expectedContent, chunk.Content)
		}
	})

	t.Run("请求被阻止时返回 content_filter", func(t *testing.T) {
		mockLogger.reset()

		googleChunk := map[string]interface{}{
			"id":      "google-stream-004",
			"object":  "chat.completion.chunk",
			"created": 1677858245,
			"model":   "gemini-pro",
			"promptFeedback": map[string]interface{}{
				"blockReason":        "SAFETY",
				"blockReasonMessage": "Content filtered for safety",
			},
		}

		data, _ := json.Marshal(googleChunk)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if !chunk.Finished {
			t.Error("期望 Finished 为 true")
		}

		if chunk.StopReason != "content_filter" {
			t.Errorf("期望 stop_reason content_filter, 实际 %s", chunk.StopReason)
		}

		if chunk.Error != "Content filtered for safety" {
			t.Errorf("期望错误消息, 实际 '%s'", chunk.Error)
		}
	})

	t.Run("无效 JSON 返回 nil", func(t *testing.T) {
		mockLogger.reset()

		chunk, err := converter.ParseChunk([]byte("{invalid json}"))

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk != nil {
			t.Errorf("期望 nil, 实际 %v", chunk)
		}
	})
}

// TestStreamChunkConverter_BuildChunk 测试构建流式数据块
func TestStreamChunkConverter_BuildChunk(t *testing.T) {
	mockLogger := &MockLoggerForGoogleStream{}
	converter := NewStreamChunkConverter(mockLogger)

	t.Run("nil 输入返回 nil", func(t *testing.T) {
		result, err := converter.BuildChunk(nil)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result != nil {
			t.Errorf("期望 nil, 实际 %v", result)
		}
	})

	t.Run("构建内容块", func(t *testing.T) {
		chunk := &entity.StreamChunk{
			Finished:   false,
			Content:    "Hello",
			StopReason: "",
		}

		result, err := converter.BuildChunk(chunk)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 验证 Google 格式
		var resultMap map[string]interface{}
		if err := json.Unmarshal(result, &resultMap); err != nil {
			t.Fatalf("JSON 解析失败: %v", err)
		}

		if resultMap["chunk"] != "Hello" {
			t.Errorf("期望 chunk 字段为 'Hello'")
		}
	})

	t.Run("构建已完成块", func(t *testing.T) {
		chunk := &entity.StreamChunk{
			Finished:   true,
			Content:    "Complete",
			StopReason: "stop",
		}

		result, err := converter.BuildChunk(chunk)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 验证 Google 停止格式
		var resultMap map[string]interface{}
		if err := json.Unmarshal(result, &resultMap); err != nil {
			t.Fatalf("JSON 解析失败: %v", err)
		}

		if resultMap["chunk"] != "Complete" {
			t.Errorf("期望 chunk 字段")
		}

		if resultMap["usageMetadata"] == nil {
			t.Error("期望 usageMetadata 字段")
		}

		candidates := resultMap["candidates"].([]interface{})
		if len(candidates) != 1 {
			t.Errorf("期望 1 个 candidate, 实际 %d", len(candidates))
		}

		candidate := candidates[0].(map[string]interface{})
		if candidate["finishReason"] != "STOP" {
			t.Errorf("期望 finishReason STOP, 实际 %v", candidate["finishReason"])
		}
	})

	t.Run("构建 max_tokens 停止块", func(t *testing.T) {
		chunk := &entity.StreamChunk{
			Finished:   true,
			Content:    "Partial",
			StopReason: "length",
		}

		result, err := converter.BuildChunk(chunk)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		var resultMap map[string]interface{}
		if err := json.Unmarshal(result, &resultMap); err != nil {
			t.Fatalf("JSON 解析失败: %v", err)
		}

		candidates := resultMap["candidates"].([]interface{})
		candidate := candidates[0].(map[string]interface{})
		if candidate["finishReason"] != "MAX_TOKENS" {
			t.Errorf("期望 finishReason MAX_TOKENS, 实际 %v", candidate["finishReason"])
		}
	})
}

// TestStreamChunkConverter_GoogleStopReason 测试停止原因转换
func TestStreamChunkConverter_GoogleStopReason(t *testing.T) {
	converter := &StreamChunkConverter{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "stop 转换为 STOP",
			input:    "stop",
			expected: "STOP",
		},
		{
			name:     "length 转换为 MAX_TOKENS",
			input:    "length",
			expected: "MAX_TOKENS",
		},
		{
			name:     "content_filter 转换为 SAFETY",
			input:    "content_filter",
			expected: "SAFETY",
		},
		{
			name:     "未知原因保持原样",
			input:    "UNKNOWN",
			expected: "UNKNOWN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.googleStopReason(tt.input)

			if result != tt.expected {
				t.Errorf("期望 %s, 实际 %s", tt.expected, result)
			}
		})
	}
}

// TestStreamChunkConverter_Supports 测试协议支持检查
func TestStreamChunkConverter_Supports(t *testing.T) {
	converter := &StreamChunkConverter{}

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

// TestStreamChunkConverter_Protocol 测试协议返回
func TestStreamChunkConverter_Protocol(t *testing.T) {
	converter := &StreamChunkConverter{}

	result := converter.Protocol()

	if result != types.ProtocolGoogle {
		t.Errorf("期望协议 %v, 实际 %v", types.ProtocolGoogle, result)
	}
}

// TestStreamChunkConverter_Name 测试策略名称返回
func TestStreamChunkConverter_Name(t *testing.T) {
	converter := &StreamChunkConverter{}

	result := converter.Name()

	expected := "GoogleVertexAIStreamChunkConverter"
	if result != expected {
		t.Errorf("期望名称 %s, 实际 %s", expected, result)
	}
}

// TestStreamChunkConverter_WithRealGoogleStream 测试真实 Google 流式响应格式
func TestStreamChunkConverter_WithRealGoogleStream(t *testing.T) {
	mockLogger := &MockLoggerForGoogleStream{}
	converter := NewStreamChunkConverter(mockLogger)

	t.Run("Gemini Pro 流式块格式", func(t *testing.T) {
		mockLogger.reset()

		realChunk := map[string]interface{}{
			"id":      "google-stream-005",
			"object":  "chat.completion.chunk",
			"created": 1677858246,
			"model":   "gemini-pro",
			"chunk":   "I am Gemini, a large language model built by Google.",
		}

		data, _ := json.Marshal(realChunk)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if chunk.Finished {
			t.Error("期望 Finished 为 false")
		}

		expectedContent := "I am Gemini, a large language model built by Google."
		if chunk.Content != expectedContent {
			t.Errorf("期望内容 '%s', 实际 '%s'", expectedContent, chunk.Content)
		}
	})

	t.Run("Gemini 1.5 Flash 流式结束格式", func(t *testing.T) {
		mockLogger.reset()

		realChunk := map[string]interface{}{
			"id":      "google-stream-006",
			"object":  "chat.completion.chunk",
			"created": 1677858247,
			"model":   "gemini-1.5-flash",
			"usageMetadata": map[string]int{
				"promptTokenCount":     50,
				"candidatesTokenCount": 100,
				"totalTokenCount":      150,
			},
		}

		data, _ := json.Marshal(realChunk)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if !chunk.Finished {
			t.Error("期望 Finished 为 true")
		}

		if chunk.StopReason != "stop" {
			t.Errorf("期望 stop_reason stop, 实际 %s", chunk.StopReason)
		}
	})

	t.Run("空内容块处理", func(t *testing.T) {
		mockLogger.reset()

		emptyChunk := map[string]interface{}{
			"id":         "google-stream-007",
			"object":     "chat.completion.chunk",
			"created":    1677858248,
			"model":      "gemini-pro",
			"chunk":      "",
			"candidates": []map[string]interface{}{},
		}

		data, _ := json.Marshal(emptyChunk)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if chunk.Content != "" {
			t.Errorf("期望空内容, 实际 '%s'", chunk.Content)
		}
	})
}

// TestStreamChunkConverter_ParseStream 测试完整流解析
func TestStreamChunkConverter_ParseStream(t *testing.T) {
	mockLogger := &MockLoggerForGoogleStream{}
	converter := NewStreamChunkConverter(mockLogger)

	t.Run("解析多行流式数据", func(t *testing.T) {
		mockLogger.reset()

		// 模拟多行 JSON Lines 格式
		streamData := []byte(`{"id":"1","chunk":"Hello"}
{"id":"2","chunk":" World"}
{"id":"3","usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":20,"totalTokenCount":30}}`)

		chunks, errs := converter.ParseStream(streamData)

		var results []*entity.StreamChunk
		for chunk := range chunks {
			results = append(results, chunk)
		}

		// 检查错误（errs 通道在没有错误时是 nil）
		select {
		case err := <-errs:
			if err != nil {
				t.Fatalf("期望无错误, 实际 %v", err)
			}
		default:
			// 没有错误，这是正常的
		}

		if len(results) != 3 {
			t.Errorf("期望 3 个块, 实际 %d", len(results))
		}

		// 验证第一个块
		if results[0].Content != "Hello" {
			t.Errorf("期望 'Hello', 实际 '%s'", results[0].Content)
		}

		// 验证第二个块
		if results[1].Content != " World" {
			t.Errorf("期望 ' World', 实际 '%s'", results[1].Content)
		}

		// 验证最后一个块（结束标记）
		if !results[2].Finished {
			t.Error("期望最后一个块已结束")
		}
	})
}

// TestStreamChunkConverter_LoggerDebugCall 测试调试日志调用
func TestStreamChunkConverter_LoggerDebugCall(t *testing.T) {
	mockLogger := &MockLoggerForGoogleStream{}
	converter := NewStreamChunkConverter(mockLogger)

	// 无效 JSON 应该触发调试日志
	converter.ParseChunk([]byte("{invalid}"))

	if len(mockLogger.debugMessages) == 0 {
		t.Log("注意: 可能没有日志输出（正常行为）")
	}
}

// TestStreamChunkConverter_LoggerNotCalledForNilLogger 测试 nil 日志器安全
func TestStreamChunkConverter_LoggerNotCalledForNilLogger(t *testing.T) {
	t.Run("nil 日志器不会导致 panic", func(t *testing.T) {
		converter := NewStreamChunkConverter(nil)

		chunk, err := converter.ParseChunk([]byte("{invalid}"))

		if chunk != nil {
			t.Errorf("期望 nil, 实际 %v", chunk)
		}

		if err != nil {
			t.Errorf("期望 nil 错误, 实际 %v", err)
		}
	})
}
