package entity

import (
	"context"
	"testing"
)

func TestRequestID(t *testing.T) {
	t.Run("NewRequestID creates valid ID", func(t *testing.T) {
		id := NewRequestID("req-123")
		if id.String() != "req-123" {
			t.Errorf("Expected 'req-123', got '%s'", id.String())
		}
	})

	t.Run("IsEmpty returns true for empty ID", func(t *testing.T) {
		id := NewRequestID("")
		if !id.IsEmpty() {
			t.Error("Expected IsEmpty to return true")
		}
	})

	t.Run("IsEmpty returns false for non-empty ID", func(t *testing.T) {
		id := NewRequestID("req-123")
		if id.IsEmpty() {
			t.Error("Expected IsEmpty to return false")
		}
	})
}

func TestMessage(t *testing.T) {
	t.Run("NewMessage creates valid message", func(t *testing.T) {
		msg := NewMessage("user", "Hello")
		if msg.Role != "user" {
			t.Errorf("Expected role 'user', got '%s'", msg.Role)
		}
		if msg.Content != "Hello" {
			t.Errorf("Expected content 'Hello', got '%s'", msg.Content)
		}
	})

	t.Run("IsEmpty returns true for empty message", func(t *testing.T) {
		msg := Message{}
		if !msg.IsEmpty() {
			t.Error("Expected IsEmpty to return true")
		}
	})

	t.Run("IsEmpty returns false for message with content", func(t *testing.T) {
		msg := NewMessage("user", "Hello")
		if msg.IsEmpty() {
			t.Error("Expected IsEmpty to return false")
		}
	})

	t.Run("IsToolCall returns true for message with tool calls", func(t *testing.T) {
		msg := Message{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{ID: "call-1", Type: "function"},
			},
		}
		if !msg.IsToolCall() {
			t.Error("Expected IsToolCall to return true")
		}
	})

	t.Run("IsToolResult returns true for tool result message", func(t *testing.T) {
		msg := Message{
			Role:       "tool",
			ToolCallID: "call-1",
		}
		if !msg.IsToolResult() {
			t.Error("Expected IsToolResult to return true")
		}
	})
}

func TestRequest_New(t *testing.T) {
	t.Run("NewRequest creates valid request", func(t *testing.T) {
		id := NewRequestID("req-123")
		model := NewModelAlias("gpt-4")
		messages := []Message{NewMessage("user", "Hello")}

		req := NewRequest(id, model, messages)
		if req.ID() != id {
			t.Errorf("Expected ID '%s', got '%s'", id, req.ID())
		}
		if req.Model() != model {
			t.Errorf("Expected model '%s', got '%s'", model, req.Model())
		}
		if len(req.Messages()) != 1 {
			t.Errorf("Expected 1 message, got %d", len(req.Messages()))
		}
	})

	t.Run("NewRequest sets default values", func(t *testing.T) {
		req := NewRequest(NewRequestID("req-1"), NewModelAlias("gpt-4"), []Message{})
		if req.Temperature() != 1.0 {
			t.Errorf("Expected default temperature 1.0, got %f", req.Temperature())
		}
		if req.TopP() != 1.0 {
			t.Errorf("Expected default topP 1.0, got %f", req.TopP())
		}
		if req.Context() == nil {
			t.Error("Expected non-nil context")
		}
	})

	t.Run("WithModel creates new request with different model", func(t *testing.T) {
		req := NewRequest(NewRequestID("req-1"), NewModelAlias("gpt-4"), []Message{})
		newReq := req.WithModel(NewModelAlias("claude"))

		if newReq.Model().String() != "claude" {
			t.Errorf("Expected model 'claude', got '%s'", newReq.Model())
		}
		if req.Model().String() != "gpt-4" {
			t.Error("Original request should not be modified")
		}
	})

	t.Run("WithContext creates new request with different context", func(t *testing.T) {
		req := NewRequest(NewRequestID("req-1"), NewModelAlias("gpt-4"), []Message{})
		ctx := context.WithValue(context.Background(), "key", "value")
		newReq := req.WithContext(ctx)

		if newReq.Context() != ctx {
			t.Error("Expected new context")
		}
	})

	t.Run("WithStreamHandler creates new request with handler", func(t *testing.T) {
		req := NewRequest(NewRequestID("req-1"), NewModelAlias("gpt-4"), []Message{})
		handler := func(chunk []byte) error { return nil }
		newReq := req.WithStreamHandler(handler)

		if newReq.StreamHandler() == nil {
			t.Error("Expected non-nil stream handler")
		}
	})
}

func TestRequestBuilder(t *testing.T) {
	t.Run("Build creates valid request", func(t *testing.T) {
		builder := NewRequestBuilder().
			ID(NewRequestID("req-1")).
			Model(NewModelAlias("gpt-4")).
			Messages([]Message{NewMessage("user", "Hello")}).
			MaxTokens(100).
			Temperature(0.7).
			Stream(true)

		req, err := builder.Build()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if req.MaxTokens() != 100 {
			t.Errorf("Expected max tokens 100, got %d", req.MaxTokens())
		}
		if req.Temperature() != 0.7 {
			t.Errorf("Expected temperature 0.7, got %f", req.Temperature())
		}
		if !req.IsStream() {
			t.Error("Expected stream to be true")
		}
	})

	t.Run("Build fails without ID", func(t *testing.T) {
		builder := NewRequestBuilder().
			Model(NewModelAlias("gpt-4")).
			Messages([]Message{NewMessage("user", "Hello")})

		_, err := builder.Build()
		if err == nil {
			t.Error("Expected error for missing ID")
		}
	})

	t.Run("Build fails without model", func(t *testing.T) {
		builder := NewRequestBuilder().
			ID(NewRequestID("req-1")).
			Messages([]Message{NewMessage("user", "Hello")})

		_, err := builder.Build()
		if err == nil {
			t.Error("Expected error for missing model")
		}
	})

	t.Run("Build fails without messages", func(t *testing.T) {
		builder := NewRequestBuilder().
			ID(NewRequestID("req-1")).
			Model(NewModelAlias("gpt-4"))

		_, err := builder.Build()
		if err == nil {
			t.Error("Expected error for missing messages")
		}
	})

	t.Run("BuildUnsafe creates request without validation", func(t *testing.T) {
		builder := NewRequestBuilder().
			ID(NewRequestID("req-1")).
			Model(NewModelAlias("gpt-4")).
			Messages([]Message{NewMessage("user", "Hello")})
		req := builder.BuildUnsafe()
		if req == nil {
			t.Error("Expected non-nil request from BuildUnsafe")
		}
	})
}

func TestUsage(t *testing.T) {
	t.Run("NewUsage calculates total", func(t *testing.T) {
		usage := NewUsage(100, 50)
		if usage.PromptTokens != 100 {
			t.Errorf("Expected prompt tokens 100, got %d", usage.PromptTokens)
		}
		if usage.CompletionTokens != 50 {
			t.Errorf("Expected completion tokens 50, got %d", usage.CompletionTokens)
		}
		if usage.TotalTokens != 150 {
			t.Errorf("Expected total tokens 150, got %d", usage.TotalTokens)
		}
	})

	t.Run("IsEmpty returns true for zero usage", func(t *testing.T) {
		usage := Usage{}
		if !usage.IsEmpty() {
			t.Error("Expected IsEmpty to return true")
		}
	})

	t.Run("IsEmpty returns false for non-zero usage", func(t *testing.T) {
		usage := NewUsage(10, 5)
		if usage.IsEmpty() {
			t.Error("Expected IsEmpty to return false")
		}
	})
}

func TestChoice(t *testing.T) {
	t.Run("NewChoice creates valid choice", func(t *testing.T) {
		msg := NewMessage("assistant", "Hello")
		choice := NewChoice(0, msg, "stop")

		if choice.Index != 0 {
			t.Errorf("Expected index 0, got %d", choice.Index)
		}
		if choice.Message.Content != "Hello" {
			t.Errorf("Expected content 'Hello', got '%s'", choice.Message.Content)
		}
		if choice.FinishReason != "stop" {
			t.Errorf("Expected finish reason 'stop', got '%s'", choice.FinishReason)
		}
	})

	t.Run("IsComplete returns true for complete choice", func(t *testing.T) {
		choice := NewChoice(0, Message{}, "stop")
		if !choice.IsComplete() {
			t.Error("Expected IsComplete to return true")
		}
	})

	t.Run("IsComplete returns false for incomplete choice", func(t *testing.T) {
		choice := Choice{Index: 0}
		if choice.IsComplete() {
			t.Error("Expected IsComplete to return false")
		}
	})
}

func TestResponse_New(t *testing.T) {
	t.Run("NewResponse creates valid response", func(t *testing.T) {
		choices := []Choice{NewChoice(0, NewMessage("assistant", "Hello"), "stop")}
		usage := NewUsage(10, 5)
		resp := NewResponse("resp-1", "gpt-4", choices, usage)

		if resp.ID != "resp-1" {
			t.Errorf("Expected ID 'resp-1', got '%s'", resp.ID)
		}
		if resp.Model != "gpt-4" {
			t.Errorf("Expected model 'gpt-4', got '%s'", resp.Model)
		}
		if len(resp.Choices) != 1 {
			t.Errorf("Expected 1 choice, got %d", len(resp.Choices))
		}
		if resp.Usage.TotalTokens != 15 {
			t.Errorf("Expected total tokens 15, got %d", resp.Usage.TotalTokens)
		}
	})

	t.Run("FirstChoice returns first choice", func(t *testing.T) {
		choices := []Choice{
			NewChoice(0, NewMessage("assistant", "First"), "stop"),
			NewChoice(1, NewMessage("assistant", "Second"), "stop"),
		}
		resp := NewResponse("resp-1", "gpt-4", choices, Usage{})

		first := resp.FirstChoice()
		if first == nil {
			t.Error("Expected non-nil first choice")
		}
		if first.Message.Content != "First" {
			t.Errorf("Expected 'First', got '%s'", first.Message.Content)
		}
	})

	t.Run("FirstChoice returns nil for empty choices", func(t *testing.T) {
		resp := NewResponse("resp-1", "gpt-4", []Choice{}, Usage{})
		if resp.FirstChoice() != nil {
			t.Error("Expected nil for empty choices")
		}
	})
}

func TestResponseBuilder(t *testing.T) {
	t.Run("Build creates valid response", func(t *testing.T) {
		builder := NewResponseBuilder().
			ID("resp-1").
			Model("gpt-4").
			Choices([]Choice{NewChoice(0, Message{}, "stop")}).
			Usage(NewUsage(10, 5))

		resp, err := builder.Build()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if resp.ID != "resp-1" {
			t.Errorf("Expected ID 'resp-1', got '%s'", resp.ID)
		}
	})

	t.Run("Build fails without ID", func(t *testing.T) {
		builder := NewResponseBuilder().Model("gpt-4")
		_, err := builder.Build()
		if err == nil {
			t.Error("Expected error for missing ID")
		}
	})

	t.Run("Build fails without model", func(t *testing.T) {
		builder := NewResponseBuilder().ID("resp-1")
		_, err := builder.Build()
		if err == nil {
			t.Error("Expected error for missing model")
		}
	})

	t.Run("BuildUnsafe creates response without validation", func(t *testing.T) {
		builder := NewResponseBuilder().
			ID("resp-1").
			Model("gpt-4")
		resp := builder.BuildUnsafe()
		if resp == nil {
			t.Error("Expected non-nil response from BuildUnsafe")
		}
	})
}
