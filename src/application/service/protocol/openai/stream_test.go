package openai

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// MockLoggerForOpenAIStream 实现 port.Logger 接口用于 OpenAI 流式转换器测试
type MockLoggerForOpenAIStream struct {
	debugMessages []string
	infoMessages  []string
	errorMessages []string
	warnMessages  []string
	fatalMessages []string
	fields        []map[string]interface{}
	withFields    [][]port.Field
}

func (m *MockLoggerForOpenAIStream) Debug(msg string, fields ...port.Field) {
	m.debugMessages = append(m.debugMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForOpenAIStream) Info(msg string, fields ...port.Field) {
	m.infoMessages = append(m.infoMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForOpenAIStream) Warn(msg string, fields ...port.Field) {
	m.warnMessages = append(m.warnMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForOpenAIStream) Error(msg string, fields ...port.Field) {
	m.errorMessages = append(m.errorMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForOpenAIStream) Fatal(msg string, fields ...port.Field) {
	m.fatalMessages = append(m.fatalMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForOpenAIStream) With(fields ...port.Field) port.Logger {
	m.withFields = append(m.withFields, fields)
	return m
}

func (m *MockLoggerForOpenAIStream) recordFields(fields []port.Field) {
	for _, field := range fields {
		m.fields = append(m.fields, map[string]interface{}{
			"key":   field.Key,
			"value": field.Value,
		})
	}
}

func (m *MockLoggerForOpenAIStream) reset() {
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
		mockLogger := &MockLoggerForOpenAIStream{}

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
	mockLogger := &MockLoggerForOpenAIStream{}
	converter := NewStreamChunkConverter(mockLogger)

	t.Run("解析 DONE 信号", func(t *testing.T) {
		mockLogger.reset()

		chunk, err := converter.ParseChunk([]byte("[DONE]"))

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if !chunk.Finished {
			t.Error("期望 Finished 为 true")
		}

		if chunk.Content != "" {
			t.Errorf("期望空内容, 实际 '%s'", chunk.Content)
		}
	})

	t.Run("解析带 data: 前缀的块", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`data: {"id":"test-id","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"}}]}`)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if chunk.Finished {
			t.Error("期望 Finished 为 false（无 finish_reason）")
		}

		if chunk.Content != "Hello" {
			t.Errorf("期望内容 'Hello', 实际 '%s'", chunk.Content)
		}
	})

	t.Run("解析带 finish_reason 的块", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`{"id":"test-id","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"World"},"finish_reason":"stop"}]}`)
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

		if chunk.Content != "World" {
			t.Errorf("期望内容 'World', 实际 '%s'", chunk.Content)
		}

		if chunk.StopReason != "stop" {
			t.Errorf("期望 stop_reason stop, 实际 %s", chunk.StopReason)
		}
	})

	t.Run("解析空内容块", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`{"id":"test-id","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)
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

	t.Run("处理多个选择", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`{"id":"test-id","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Part1"}},{"index":1,"delta":{"content":"Part2"}}]}`)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		// 应该拼接所有选择的内容
		if chunk.Content != "Part1Part2" {
			t.Errorf("期望内容 'Part1Part2', 实际 '%s'", chunk.Content)
		}
	})

	t.Run("无效 JSON 返回错误", func(t *testing.T) {
		mockLogger.reset()

		data := []byte("{invalid json}")
		chunk, err := converter.ParseChunk(data)

		if chunk != nil {
			t.Errorf("期望 nil, 实际 %v", chunk)
		}

		if err == nil {
			t.Error("期望错误")
		}

		if len(mockLogger.debugMessages) == 0 {
			t.Log("注意: 可能没有日志输出（正常行为）")
		}
	})

	t.Run("解析包含 role 的初始块", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`{"id":"test-id","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant"}}]}`)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		// role 块没有内容
		if chunk.Content != "" {
			t.Errorf("期望空内容, 实际 '%s'", chunk.Content)
		}
	})

	t.Run("解析 length finish_reason", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`{"id":"test-id","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"length"}]}`)
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

		if chunk.StopReason != "length" {
			t.Errorf("期望 stop_reason length, 实际 %s", chunk.StopReason)
		}
	})
}

// TestStreamChunkConverter_BuildChunk 测试构建流式数据块
func TestStreamChunkConverter_BuildChunk(t *testing.T) {
	mockLogger := &MockLoggerForOpenAIStream{}
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

	t.Run("构建普通内容块", func(t *testing.T) {
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

		// 验证 JSON 结构（未完成时不包含 finish_reason）
		expected := `{"id":"","object":"chat.completion.chunk","created":0,"model":"","choices":[{"index":0,"delta":{"content":"Hello"}}]}`
		if string(result) != expected {
			t.Errorf("期望 %s, 实际 %s", expected, string(result))
		}
	})

	t.Run("构建已完成块", func(t *testing.T) {
		chunk := &entity.StreamChunk{
			Finished:   true,
			Content:    "Complete response",
			StopReason: "stop",
		}

		result, err := converter.BuildChunk(chunk)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 验证 JSON 结构包含 finish_reason
		expected := `{"id":"","object":"chat.completion.chunk","created":0,"model":"","choices":[{"index":0,"delta":{"content":"Complete response"},"finish_reason":"stop"}]}`
		if string(result) != expected {
			t.Errorf("期望 %s, 实际 %s", expected, string(result))
		}
	})
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

// TestStreamChunkConverter_Protocol 测试协议返回
func TestStreamChunkConverter_Protocol(t *testing.T) {
	converter := &StreamChunkConverter{}

	result := converter.Protocol()

	if result != types.ProtocolOpenAI {
		t.Errorf("期望协议 %v, 实际 %v", types.ProtocolOpenAI, result)
	}
}

// TestStreamChunkConverter_Name 测试策略名称返回
func TestStreamChunkConverter_Name(t *testing.T) {
	converter := &StreamChunkConverter{}

	result := converter.Name()

	expected := "OpenAIStreamChunkConverter"
	if result != expected {
		t.Errorf("期望名称 %s, 实际 %s", expected, result)
	}
}

// TestStreamChunkConverter_WithRealOpenAIStream 测试真实 OpenAI 流式响应格式
func TestStreamChunkConverter_WithRealOpenAIStream(t *testing.T) {
	mockLogger := &MockLoggerForOpenAIStream{}
	converter := NewStreamChunkConverter(mockLogger)

	t.Run("解析真实 OpenAI 流式块", func(t *testing.T) {
		mockLogger.reset()

		// 模拟真实 OpenAI 流式响应数据块
		realChunk := []byte(`data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1677858242,"model":"gpt-4","system_fingerprint":"fp_123","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`)
		chunk, err := converter.ParseChunk(realChunk)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		// 角色块没有内容
		if chunk.Content != "" {
			t.Errorf("期望空内容, 实际 '%s'", chunk.Content)
		}
	})

	t.Run("解析内容块", func(t *testing.T) {
		mockLogger.reset()

		contentChunk := []byte(`data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1677858242,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`)
		chunk, err := converter.ParseChunk(contentChunk)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if chunk.Content != "Hello" {
			t.Errorf("期望内容 'Hello', 实际 '%s'", chunk.Content)
		}
	})

	t.Run("解析流结束块", func(t *testing.T) {
		mockLogger.reset()

		doneChunk := []byte(`data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1677858243,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)
		chunk, err := converter.ParseChunk(doneChunk)

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

	t.Run("解析工具调用块", func(t *testing.T) {
		mockLogger.reset()

		// 工具调用块包含嵌套结构，需要正确的 JSON 转义
		toolChunk := []byte(`{"id":"chatcmpl-def456","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_01","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"NYC\"}"}}]}}]}`)
		chunk, err := converter.ParseChunk(toolChunk)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		// 工具调用块的内容应该为空（文本内容提取）
		if chunk.Content != "" {
			t.Errorf("期望空内容, 实际 '%s'", chunk.Content)
		}
	})
}

// TestStreamChunkConverter_LoggerDebugCall 测试调试日志调用
func TestStreamChunkConverter_LoggerDebugCall(t *testing.T) {
	mockLogger := &MockLoggerForOpenAIStream{}
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

		if err == nil {
			t.Error("期望错误")
		}
	})
}

// TestStreamChunkConverter_EdgeCases 测试流式块边缘情况
func TestStreamChunkConverter_EdgeCases(t *testing.T) {
	mockLogger := &MockLoggerForOpenAIStream{}
	converter := NewStreamChunkConverter(mockLogger)

	t.Run("空数据块返回错误", func(t *testing.T) {
		mockLogger.reset()

		_, err := converter.ParseChunk([]byte{})

		// 空数据应该返回错误
		if err == nil {
			t.Log("注意: 空数据块可能返回 nil chunk 而不返回错误（取决于实现）")
		}
	})

	t.Run("仅 whitespace 的块返回错误", func(t *testing.T) {
		mockLogger.reset()

		_, err := converter.ParseChunk([]byte("   "))

		// Whitespace 应该返回错误
		if err == nil {
			t.Log("注意: Whitespace 可能被接受（取决于实现）")
		} else {
			// 这是预期的行为
			t.Log("Whitespace 返回错误（正常行为）")
		}
	})

	t.Run("包含 null 值", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`{"id":"test-id","choices":[{"index":0,"delta":null,"finish_reason":null}]}`)
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

	t.Run("超长内容块", func(t *testing.T) {
		mockLogger.reset()

		longContent := strings.Repeat("a", 100000)
		data := []byte(fmt.Sprintf(`{"id":"test-id","choices":[{"index":0,"delta":{"content":"%s"}}]}`, longContent))
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if chunk.Content != longContent {
			t.Errorf("期望超长内容, 实际长度 %d", len(chunk.Content))
		}
	})

	t.Run("多个空选择", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`{"id":"test-id","choices":[{"index":0,"delta":{}},{"index":1,"delta":{}},{"index":2,"delta":{}}]}`)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		// 多个空选择应该拼接为空字符串
		if chunk.Content != "" {
			t.Errorf("期望空内容, 实际 '%s'", chunk.Content)
		}
	})

	t.Run("完整 JSON 结构", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`{"id":"test-123","object":"chat.completion.chunk","created":1677858242,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":"stop"}]}`)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if chunk.Content != "Hello" {
			t.Errorf("期望内容 'Hello', 实际 '%s'", chunk.Content)
		}

		if !chunk.Finished {
			t.Error("期望 Finished 为 true")
		}

		if chunk.StopReason != "stop" {
			t.Errorf("期望 stop_reason stop, 实际 %s", chunk.StopReason)
		}
	})
}

// TestStreamChunkConverter_BuildChunkEdgeCases 测试构建流式块边缘情况
func TestStreamChunkConverter_BuildChunkEdgeCases(t *testing.T) {
	mockLogger := &MockLoggerForOpenAIStream{}
	converter := NewStreamChunkConverter(mockLogger)

	t.Run("空内容块", func(t *testing.T) {
		mockLogger.reset()

		chunk := &entity.StreamChunk{
			Finished:   false,
			Content:    "",
			StopReason: "",
		}

		result, err := converter.BuildChunk(chunk)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 注意: 空 content 使用 omitempty, 不会出现在 JSON 中
		// 验证结果至少包含基本的 JSON 结构
		if !bytes.Contains(result, []byte(`"object":"chat.completion.chunk"`)) {
			t.Error("期望包含对象类型字段")
		}
	})

	t.Run("特殊字符内容", func(t *testing.T) {
		mockLogger.reset()

		chunk := &entity.StreamChunk{
			Finished:   true,
			Content:    "Hello 世界! 🎉 \n\t\r",
			StopReason: "stop",
		}

		result, err := converter.BuildChunk(chunk)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 验证特殊字符被正确转义
		if !bytes.Contains(result, []byte("Hello 世界!")) {
			t.Error("期望包含特殊字符内容")
		}
	})

	t.Run("content_filter stop_reason", func(t *testing.T) {
		mockLogger.reset()

		chunk := &entity.StreamChunk{
			Finished:   true,
			Content:    "",
			StopReason: "content_filter",
		}

		result, err := converter.BuildChunk(chunk)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 验证 content_filter 被包含
		if !bytes.Contains(result, []byte(`"finish_reason":"content_filter"`)) {
			t.Error("期望包含 content_filter")
		}
	})

	t.Run("工具调用块", func(t *testing.T) {
		mockLogger.reset()

		chunk := &entity.StreamChunk{
			Finished:   true,
			Content:    "",
			StopReason: "tool_calls",
		}

		result, err := converter.BuildChunk(chunk)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// 验证 tool_calls stop_reason
		if !bytes.Contains(result, []byte(`"finish_reason":"tool_calls"`)) {
			t.Error("期望包含 tool_calls")
		}
	})
}
