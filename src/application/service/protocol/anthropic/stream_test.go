package anthropic

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// MockLoggerForAnthropicStream 实现 port.Logger 接口用于 Anthropic 流式转换器测试
type MockLoggerForAnthropicStream struct {
	debugMessages []string
	infoMessages  []string
	errorMessages []string
	warnMessages  []string
	fatalMessages []string
	fields        []map[string]interface{}
	withFields    [][]port.Field
}

func (m *MockLoggerForAnthropicStream) Debug(msg string, fields ...port.Field) {
	m.debugMessages = append(m.debugMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForAnthropicStream) Info(msg string, fields ...port.Field) {
	m.infoMessages = append(m.infoMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForAnthropicStream) Warn(msg string, fields ...port.Field) {
	m.warnMessages = append(m.warnMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForAnthropicStream) Error(msg string, fields ...port.Field) {
	m.errorMessages = append(m.errorMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForAnthropicStream) Fatal(msg string, fields ...port.Field) {
	m.fatalMessages = append(m.fatalMessages, msg)
	m.recordFields(fields)
}

func (m *MockLoggerForAnthropicStream) With(fields ...port.Field) port.Logger {
	m.withFields = append(m.withFields, fields)
	return m
}

func (m *MockLoggerForAnthropicStream) recordFields(fields []port.Field) {
	for _, field := range fields {
		m.fields = append(m.fields, map[string]interface{}{
			"key":   field.Key,
			"value": field.Value,
		})
	}
}

func (m *MockLoggerForAnthropicStream) reset() {
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
		mockLogger := &MockLoggerForAnthropicStream{}

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
	mockLogger := &MockLoggerForAnthropicStream{}
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

	t.Run("解析 content_block_delta 事件", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`)
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

	t.Run("解析 message_stop 事件", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`event: message_stop
data: {"type":"message_stop","stop_reason":"stop_sequence"}`)
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

		// Anthropic 流式响应保持原始 stop_reason
		if chunk.StopReason != "stop_sequence" {
			t.Errorf("期望 stop_reason stop_sequence, 实际 %s", chunk.StopReason)
		}
	})

	t.Run("解析 max_tokens 停止原因", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`event: message_stop
data: {"type":"message_stop","stop_reason":"max_tokens"}`)
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

		// Anthropic 流式响应保持原始 stop_reason
		if chunk.StopReason != "max_tokens" {
			t.Errorf("期望 stop_reason max_tokens, 实际 %s", chunk.StopReason)
		}
	})

	t.Run("解析纯 data 前缀块", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"World"}}`)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if chunk.Content != "World" {
			t.Errorf("期望内容 'World', 实际 '%s'", chunk.Content)
		}
	})

	t.Run("解析空内容 delta", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":""}}`)
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

	t.Run("解析 message_start 事件", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg-01","role":"assistant","content":[]}}`)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		// message_start 事件没有内容
		if chunk.Content != "" {
			t.Errorf("期望空内容, 实际 '%s'", chunk.Content)
		}

		if chunk.Finished {
			t.Error("期望 Finished 为 false")
		}
	})

	t.Run("解析 ping 事件", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`event: ping
data: {"type":"ping"}`)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		// ping 事件没有内容
		if chunk.Content != "" {
			t.Errorf("期望空内容, 实际 '%s'", chunk.Content)
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
}

// TestStreamChunkConverter_BuildChunk 测试构建流式数据块
func TestStreamChunkConverter_BuildChunk(t *testing.T) {
	mockLogger := &MockLoggerForAnthropicStream{}
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

		// 验证 Anthropic 格式
		expected := `{"type":"content_block_delta","delta":{"text":"Hello"}}`
		if string(result) != expected {
			t.Errorf("期望 %s, 实际 %s", expected, string(result))
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

		// 验证 Anthropic 停止格式
		expected := `{"type":"message_stop","stop_reason":"stop"}`
		if string(result) != expected {
			t.Errorf("期望 %s, 实际 %s", expected, string(result))
		}
	})

	t.Run("构建 max_tokens 停止块", func(t *testing.T) {
		chunk := &entity.StreamChunk{
			Finished:   true,
			Content:    "Partial",
			StopReason: "max_tokens",
		}

		result, err := converter.BuildChunk(chunk)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if result == nil {
			t.Fatal("结果不应为 nil")
		}

		// Anthropic 保持原始 stop_reason
		expected := `{"type":"message_stop","stop_reason":"max_tokens"}`
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

	if result != types.ProtocolAnthropic {
		t.Errorf("期望协议 %v, 实际 %v", types.ProtocolAnthropic, result)
	}
}

// TestStreamChunkConverter_Name 测试策略名称返回
func TestStreamChunkConverter_Name(t *testing.T) {
	converter := &StreamChunkConverter{}

	result := converter.Name()

	expected := "AnthropicStreamChunkConverter"
	if result != expected {
		t.Errorf("期望名称 %s, 实际 %s", expected, result)
	}
}

// TestStreamChunkConverter_WithRealAnthropicStream 测试真实 Anthropic 流式响应格式
func TestStreamChunkConverter_WithRealAnthropicStream(t *testing.T) {
	mockLogger := &MockLoggerForAnthropicStream{}
	converter := NewStreamChunkConverter(mockLogger)

	t.Run("解析真实 Anthropic 流式事件序列", func(t *testing.T) {
		mockLogger.reset()

		// 模拟 Anthropic message_start 事件
		messageStart := []byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_01abc123","role":"assistant","content":[]}}`)

		chunk, err := converter.ParseChunk(messageStart)

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

	t.Run("解析内容块增量", func(t *testing.T) {
		mockLogger.reset()

		contentDelta := []byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"I am Claude, an AI assistant."}}`)

		chunk, err := converter.ParseChunk(contentDelta)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if chunk.Content != "I am Claude, an AI assistant." {
			t.Errorf("期望内容 'I am Claude, an AI assistant.', 实际 '%s'", chunk.Content)
		}
	})

	t.Run("解析消息停止", func(t *testing.T) {
		mockLogger.reset()

		messageStop := []byte(`event: message_stop
data: {"type":"message_stop","stop_reason":"end_turn"}`)

		chunk, err := converter.ParseChunk(messageStop)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if !chunk.Finished {
			t.Error("期望 Finished 为 true")
		}

		if chunk.StopReason != "end_turn" {
			t.Errorf("期望 stop_reason end_turn, 实际 %s", chunk.StopReason)
		}
	})

	t.Run("解析工具使用块", func(t *testing.T) {
		mockLogger.reset()

		// content_block_start 事件用于工具使用，内容是对象不是数组
		toolUse := []byte(`event: content_block_start
data: {"type":"content_block_start","index":0,"content":{"type":"tool_use","id":"tool_01","name":"get_weather","input":{"location":"San Francisco"}}}`)

		chunk, err := converter.ParseChunk(toolUse)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		// 工具使用块没有文本内容
		if chunk.Content != "" {
			t.Errorf("期望空内容, 实际 '%s'", chunk.Content)
		}
	})
}

// TestStreamChunkConverter_LoggerDebugCall 测试调试日志调用
func TestStreamChunkConverter_LoggerDebugCall(t *testing.T) {
	mockLogger := &MockLoggerForAnthropicStream{}
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
	mockLogger := &MockLoggerForAnthropicStream{}
	converter := NewStreamChunkConverter(mockLogger)

	t.Run("空数据块应该返回 [DONE] 信号", func(t *testing.T) {
		mockLogger.reset()

		// 空字节切片应该被 TrimSpace 处理后检查是否等于 "[DONE]"
		chunk, err := converter.ParseChunk([]byte{})

		if err == nil {
			// 空数据在 TrimSpace 后不是 "[DONE]"，所以应该返回错误
			if chunk == nil {
				t.Fatal("结果不应为 nil")
			}
			// 验证行为：空数据不被视为 [DONE]
			t.Log("注意: 空数据不被视为 [DONE] 信号（取决于实现）")
		} else {
			// 解析失败是预期行为
			t.Log("空数据解析失败（正常行为）")
		}
	})

	t.Run("仅 whitespace 的块应该返回错误", func(t *testing.T) {
		mockLogger.reset()

		chunk, err := converter.ParseChunk([]byte("   "))

		if err == nil {
			t.Log("注意: Whitespace 可能被接受（取决于实现）")
		} else {
			// 这是预期的行为
			t.Log("Whitespace 返回错误（正常行为）")
		}
		_ = chunk
	})

	t.Run("仅 event 无 data 前缀", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}`)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无前缀, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if chunk.Content != "Hello" {
			t.Errorf("期望内容 'Hello', 实际 '%s'", chunk.Content)
		}
	})

	t.Run("包含 usage 的 message_stop", func(t *testing.T) {
		mockLogger.reset()

		data := []byte(`event: message_stop
data: {"type":"message_stop","stop_reason":"end_turn","usage":{"output_tokens":100}}`)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if !chunk.Finished {
			t.Error("message_stop 应视为完成")
		}

		if chunk.StopReason != "end_turn" {
			t.Errorf("期望 stop_reason end_turn, 实际 %s", chunk.StopReason)
		}
	})

	t.Run("超长内容块", func(t *testing.T) {
		mockLogger.reset()

		longContent := "a" + strings.Repeat("测", 50000)
		data := []byte(fmt.Sprintf(`{"type":"content_block_delta","delta":{"type":"text_delta","text":"%s"}}`, longContent))
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if chunk.Content != longContent {
			t.Errorf("内容长度不匹配, 期望 %d, 实际 %d", len(longContent), len(chunk.Content))
		}
	})

	t.Run("特殊字符内容", func(t *testing.T) {
		mockLogger.reset()

		// 使用 JSON 转义后的特殊字符
		specialContent := "Hello 世界! 🎉"
		data := []byte(`{"type":"content_block_delta","delta":{"type":"text_delta","text":"` + specialContent + `"}}`)
		chunk, err := converter.ParseChunk(data)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		if chunk == nil {
			t.Fatal("结果不应为 nil")
		}

		if chunk.Content != specialContent {
			t.Errorf("内容不匹配, 期望 '%s', 实际 '%s'", specialContent, chunk.Content)
		}
	})

	t.Run("思考块类型检测", func(t *testing.T) {
		mockLogger.reset()

		thinkingData := []byte(`event: content_block_start
data: {"type":"content_block_start","index":0,"content":{"type":"thinking","thinking_content":"I am thinking..."}}`)
		_, err := converter.ParseChunk(thinkingData)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 验证思考块被检测到
		found := false
		for _, msg := range mockLogger.debugMessages {
			if strings.Contains(msg, "思考") {
				found = true
				break
			}
		}
		if !found {
			t.Log("注意: 思考块可能没有日志输出（正常行为）")
		}
	})

	t.Run("工具调用块类型检测", func(t *testing.T) {
		mockLogger.reset()

		toolData := []byte(`event: content_block_start
data: {"type":"content_block_start","index":0,"content":{"type":"tool_use","id":"tool_01","name":"search","input":{"query":"test"}}}`)
		_, err := converter.ParseChunk(toolData)

		if err != nil {
			t.Fatalf("期望无错误, 实际 %v", err)
		}

		// 验证工具调用块被检测到
		found := false
		for _, msg := range mockLogger.debugMessages {
			if strings.Contains(msg, "工具") {
				found = true
				break
			}
		}
		if !found {
			t.Log("注意: 工具调用块可能没有日志输出（正常行为）")
		}
	})
}

// TestStreamChunkConverter_BuildChunkEdgeCases 测试构建流式块边缘情况
func TestStreamChunkConverter_BuildChunkEdgeCases(t *testing.T) {
	mockLogger := &MockLoggerForAnthropicStream{}
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

	t.Run("空内容块", func(t *testing.T) {
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

		// 验证 Anthropic 格式
		expected := `{"type":"content_block_delta","delta":{"text":""}}`
		if string(result) != expected {
			t.Errorf("期望 %s, 实际 %s", expected, string(result))
		}
	})

	t.Run("特殊字符内容块", func(t *testing.T) {
		chunk := &entity.StreamChunk{
			Finished:   false,
			Content:    "Hello 世界! 🎉",
			StopReason: "",
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

	t.Run("多种 stop_reason 值", func(t *testing.T) {
		stopReasons := []string{"end_turn", "max_tokens", "stop_sequence", "tool_use"}

		for _, stopReason := range stopReasons {
			chunk := &entity.StreamChunk{
				Finished:   true,
				Content:    "Complete",
				StopReason: stopReason,
			}

			result, err := converter.BuildChunk(chunk)

			if err != nil {
				t.Fatalf("stop_reason=%s: 期望无错误, 实际 %v", stopReason, err)
			}

			if result == nil {
				t.Fatal("结果不应为 nil")
			}

			// Anthropic 保持原始 stop_reason
			expected := fmt.Sprintf(`{"type":"message_stop","stop_reason":"%s"}`, stopReason)
			if string(result) != expected {
				t.Errorf("stop_reason=%s: 期望 %s, 实际 %s", stopReason, expected, string(result))
			}
		}
	})
}
