package service

import (
	"testing"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/types"
)

func TestProtocolConverter_ToBackend_PassThrough(t *testing.T) {
	converter := NewProtocolConverter(nil)

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
	converter := NewProtocolConverter(systemPrompts)

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
	converter := NewProtocolConverter(nil)

	req := entity.NewRequest(
		entity.NewRequestID("test-456"),
		entity.NewModelAlias("claude-3"),
		[]entity.Message{
			entity.NewMessage("user", "Test message"),
		},
	)

	result, err := converter.ToBackend(req, types.ProtocolAnthropic)
	if err != nil {
		t.Errorf("ToBackend returned unexpected error: %v", err)
	}
	if result != req {
		t.Error("ToBackend should pass through for Anthropic protocol")
	}
}

func TestProtocolConverter_FromBackend_PassThrough(t *testing.T) {
	converter := NewProtocolConverter(nil)

	resp := entity.NewResponse(
		"resp-123",
		"gpt-4",
		[]entity.Choice{
			entity.NewChoice(0, entity.NewMessage("assistant", "Hello!"), "stop"),
		},
		entity.NewUsage(10, 5),
	)

	result, err := converter.FromBackend(resp, types.ProtocolOpenAI)
	if err != nil {
		t.Errorf("FromBackend returned unexpected error: %v", err)
	}
	if result != resp {
		t.Error("FromBackend should return the same response when no transformation is needed")
	}
}

func TestProtocolConverter_FromBackend_Anthropic(t *testing.T) {
	converter := NewProtocolConverter(nil)

	resp := entity.NewResponse(
		"resp-456",
		"claude-3",
		[]entity.Choice{
			entity.NewChoice(0, entity.NewMessage("assistant", "Hi!"), "stop"),
		},
		entity.NewUsage(8, 4),
	)

	result, err := converter.FromBackend(resp, types.ProtocolAnthropic)
	if err != nil {
		t.Errorf("FromBackend returned unexpected error: %v", err)
	}
	if result != resp {
		t.Error("FromBackend should pass through for Anthropic protocol")
	}
}

func TestProtocolConverter_ToBackend_NilRequest(t *testing.T) {
	converter := NewProtocolConverter(nil)

	_, err := converter.ToBackend(nil, types.ProtocolOpenAI)
	if err == nil {
		t.Error("ToBackend should return error for nil request")
	}
}

func TestProtocolConverter_FromBackend_NilResponse(t *testing.T) {
	converter := NewProtocolConverter(nil)

	_, err := converter.FromBackend(nil, types.ProtocolOpenAI)
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
