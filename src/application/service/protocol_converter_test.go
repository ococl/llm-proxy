package service

import (
	"encoding/json"
	"testing"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

func TestProtocolConverter_ToBackend_PassThrough(t *testing.T) {
	converter := NewProtocolConverter(nil, &port.NopLogger{})

	req := entity.NewRequest(
		entity.NewRequestID("test-123"),
		entity.NewModelAlias("gpt-4"),
		[]entity.Message{
			entity.NewMessage("user", "Hello, world!"),
		},
	)

	result, err := converter.ToBackend(req, types.ProtocolOpenAI)
	if err != nil {
		t.Errorf("ToBackend returned unexpected error: %v", err)
	}
	if result != req {
		t.Error("ToBackend should return the same request when no transformation is needed")
	}
}

func TestProtocolConverter_ToBackend_WithSystemPrompt(t *testing.T) {
	systemPrompts := map[string]string{
		"gpt-4": "You are a helpful assistant.",
	}
	converter := NewProtocolConverter(systemPrompts, &port.NopLogger{})

	req := entity.NewRequest(
		entity.NewRequestID("test-123"),
		entity.NewModelAlias("gpt-4"),
		[]entity.Message{
			entity.NewMessage("user", "Hello!"),
		},
	)

	result, err := converter.ToBackend(req, types.ProtocolOpenAI)
	if err != nil {
		t.Errorf("ToBackend returned unexpected error: %v", err)
	}

	messages := result.Messages()
	if len(messages) < 2 {
		t.Errorf("Expected at least 2 messages (system + user), got %d", len(messages))
	}

	if len(messages) >= 2 && messages[0].Role != "system" {
		t.Errorf("First message should be system, got %s", messages[0].Role)
	}

	if len(messages) >= 2 && messages[0].Content != "You are a helpful assistant." {
		t.Errorf("System message content = %v, want %v", messages[0].Content, "You are a helpful assistant.")
	}
}

func TestProtocolConverter_ToBackend_Anthropic(t *testing.T) {
	converter := NewProtocolConverter(nil, &port.NopLogger{})

	t.Run("filters out system messages", func(t *testing.T) {
		req := entity.NewRequest(
			entity.NewRequestID("test-456"),
			entity.NewModelAlias("claude-3"),
			[]entity.Message{
				entity.NewMessage("system", "You are a helpful assistant."),
				entity.NewMessage("user", "Test message"),
			},
		)

		result, err := converter.ToBackend(req, types.ProtocolAnthropic)
		if err != nil {
			t.Errorf("ToBackend returned unexpected error: %v", err)
		}

		messages := result.Messages()
		if len(messages) != 1 {
			t.Errorf("Expected 1 message after filtering system, got %d", len(messages))
		}

		if messages[0].Role != "user" {
			t.Errorf("Expected first message to be user, got %s", messages[0].Role)
		}
	})

	t.Run("ensures max_tokens is set", func(t *testing.T) {
		req := entity.NewRequest(
			entity.NewRequestID("test-457"),
			entity.NewModelAlias("claude-3"),
			[]entity.Message{
				entity.NewMessage("system", "System prompt"),
				entity.NewMessage("user", "Hello"),
			},
		)

		result, err := converter.ToBackend(req, types.ProtocolAnthropic)
		if err != nil {
			t.Errorf("ToBackend returned unexpected error: %v", err)
		}

		if result.MaxTokens() == 0 {
			t.Error("Expected max_tokens to be set for Anthropic protocol")
		}

		if result.MaxTokens() != 1024 {
			t.Errorf("Expected default max_tokens=1024, got %d", result.MaxTokens())
		}
	})

	t.Run("preserves existing max_tokens if set", func(t *testing.T) {
		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-458")).
			Model(entity.NewModelAlias("claude-3")).
			Messages([]entity.Message{
				entity.NewMessage("user", "Test"),
			}).
			MaxTokens(2048).
			BuildUnsafe()

		result, err := converter.ToBackend(req, types.ProtocolAnthropic)
		if err != nil {
			t.Errorf("ToBackend returned unexpected error: %v", err)
		}

		if result.MaxTokens() != 2048 {
			t.Errorf("Expected max_tokens=2048, got %d", result.MaxTokens())
		}
	})

	t.Run("passes through when no system messages", func(t *testing.T) {
		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-459")).
			Model(entity.NewModelAlias("claude-3")).
			Messages([]entity.Message{
				entity.NewMessage("user", "Test"),
			}).
			MaxTokens(1000).
			BuildUnsafe()

		result, err := converter.ToBackend(req, types.ProtocolAnthropic)
		if err != nil {
			t.Errorf("ToBackend returned unexpected error: %v", err)
		}

		if result != req {
			t.Error("Expected pass-through when no system messages and max_tokens is set")
		}
	})
}

func TestProtocolConverter_FromBackend_PassThrough(t *testing.T) {
	converter := NewProtocolConverter(nil, &port.NopLogger{})

	resp := entity.NewResponse(
		"resp-123",
		"gpt-4",
		[]entity.Choice{
			entity.NewChoice(0, entity.NewMessage("assistant", "Hello!"), "stop"),
		},
		entity.NewUsage(10, 5),
	)

	// 序列化响应为字节
	respBody, _ := json.Marshal(resp)
	result, err := converter.FromBackend(respBody, "gpt-4", types.ProtocolOpenAI)
	if err != nil {
		t.Errorf("FromBackend returned unexpected error: %v", err)
	}
	if result == nil || result.ID != resp.ID {
		t.Error("FromBackend should return a valid response")
	}
}

func TestProtocolConverter_FromBackend_Anthropic(t *testing.T) {
	converter := NewProtocolConverter(nil, &port.NopLogger{})

	resp := entity.NewResponse(
		"resp-456",
		"claude-3",
		[]entity.Choice{
			entity.NewChoice(0, entity.NewMessage("assistant", "Hi!"), "stop"),
		},
		entity.NewUsage(8, 4),
	)

	respBody, _ := json.Marshal(resp)
	result, err := converter.FromBackend(respBody, "claude-3", types.ProtocolAnthropic)
	if err != nil {
		t.Errorf("FromBackend returned unexpected error: %v", err)
	}
	if result == nil {
		t.Error("FromBackend should return a valid response for Anthropic")
	}
}

func TestProtocolConverter_ToBackend_NilRequest(t *testing.T) {
	converter := NewProtocolConverter(nil, &port.NopLogger{})

	_, err := converter.ToBackend(nil, types.ProtocolOpenAI)
	if err == nil {
		t.Error("ToBackend should return error for nil request")
	}
}

func TestProtocolConverter_FromBackend_NilResponse(t *testing.T) {
	converter := NewProtocolConverter(nil, &port.NopLogger{})

	_, err := converter.FromBackend(nil, "gpt-4", types.ProtocolOpenAI)
	if err == nil {
		t.Error("FromBackend should return error for nil response")
	}
}

func TestDefaultProtocolConverter(t *testing.T) {
	converter := DefaultProtocolConverter()
	if converter == nil {
		t.Error("DefaultProtocolConverter should not return nil")
	}
}

func TestResponseConverter_MergeStreamChunks_AnyContent(t *testing.T) {
	converter := NewResponseConverter()

	t.Run("merges string content correctly", func(t *testing.T) {
		// 构造带有 Delta 的流式响应块
		chunks := []*entity.Response{
			{
				ID:      "resp-1",
				Model:   "gpt-4",
				Choices: []entity.Choice{{Index: 0, Delta: &entity.Message{Role: "assistant", Content: "Hello"}, FinishReason: ""}},
				Usage:   entity.NewUsage(5, 0),
			},
			{
				ID:      "resp-1",
				Model:   "gpt-4",
				Choices: []entity.Choice{{Index: 0, Delta: &entity.Message{Role: "assistant", Content: " world"}, FinishReason: ""}},
				Usage:   entity.NewUsage(5, 5),
			},
		}

		result := converter.MergeStreamChunks(chunks)

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		// 检查合并后的内容
		if len(result.Choices) != 1 {
			t.Errorf("Expected 1 choice, got %d", len(result.Choices))
		}

		choice := result.Choices[0]
		content, ok := choice.Message.Content.(string)
		if !ok {
			t.Fatal("Expected content to be string")
		}
		if content != "Hello world" {
			t.Errorf("Expected merged content 'Hello world', got '%s'", content)
		}

		// 检查 Usage - 应该累加
		if result.Usage.PromptTokens != 10 {
			t.Errorf("Expected 10 prompt tokens, got %d", result.Usage.PromptTokens)
		}
		if result.Usage.CompletionTokens != 5 {
			t.Errorf("Expected 5 completion tokens, got %d", result.Usage.CompletionTokens)
		}
	})

	t.Run("handles empty chunks", func(t *testing.T) {
		result := converter.MergeStreamChunks([]*entity.Response{})
		if result != nil {
			t.Error("Expected nil for empty chunks")
		}
	})

	t.Run("merges non-string content by converting to string", func(t *testing.T) {
		multimodalContent := []interface{}{
			map[string]interface{}{"type": "text", "text": "Image: "},
		}
		chunks := []*entity.Response{
			{
				ID:      "resp-1",
				Model:   "gpt-4",
				Choices: []entity.Choice{{Index: 0, Delta: &entity.Message{Role: "assistant", Content: multimodalContent}, FinishReason: ""}},
				Usage:   entity.NewUsage(10, 0),
			},
			{
				ID:      "resp-1",
				Model:   "gpt-4",
				Choices: []entity.Choice{{Index: 0, Delta: &entity.Message{Role: "assistant", Content: "an image"}, FinishReason: ""}},
				Usage:   entity.NewUsage(10, 5),
			},
		}

		result := converter.MergeStreamChunks(chunks)

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		// 验证非字符串内容被正确处理
		content, ok := result.Choices[0].Message.Content.(string)
		if !ok {
			t.Fatalf("Expected content to be string after merging, got %T", result.Choices[0].Message.Content)
		}

		// 验证内容包含多模态标记
		if !contains(content, "Image:") || !contains(content, "an image") {
			t.Errorf("Expected merged content to contain image markers, got '%s'", content)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
