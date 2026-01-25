package service

import (
	"context"
	"testing"

	"llm-proxy/domain/entity"
)

// TestNewRequestValidator 测试 NewRequestValidator 构造函数。
func TestNewRequestValidator(t *testing.T) {
	validator := NewRequestValidator(50, 2048)

	if validator == nil {
		t.Fatal("NewRequestValidator 返回 nil")
	}

	// 验证默认允许的角色
	if !validator.allowedRoles["system"] {
		t.Error("应允许 system 角色")
	}
	if !validator.allowedRoles["user"] {
		t.Error("应允许 user 角色")
	}
	if !validator.allowedRoles["assistant"] {
		t.Error("应允许 assistant 角色")
	}
	if !validator.allowedRoles["tool"] {
		t.Error("应允许 tool 角色")
	}

	// 验证不允许的角色
	if validator.allowedRoles["invalid"] {
		t.Error("不应允许 invalid 角色")
	}

	if validator.maxMessagesPerRequest != 50 {
		t.Errorf("maxMessagesPerRequest 期望 50, 实际 %d", validator.maxMessagesPerRequest)
	}

	if validator.maxTokensPerRequest != 2048 {
		t.Errorf("maxTokensPerRequest 期望 2048, 实际 %d", validator.maxTokensPerRequest)
	}
}

// TestDefaultRequestValidator 测试 DefaultRequestValidator 函数。
func TestDefaultRequestValidator(t *testing.T) {
	validator := DefaultRequestValidator()

	if validator == nil {
		t.Fatal("DefaultRequestValidator 返回 nil")
	}

	if validator.maxMessagesPerRequest != 100 {
		t.Errorf("maxMessagesPerRequest 期望 100, 实际 %d", validator.maxMessagesPerRequest)
	}

	if validator.maxTokensPerRequest != 4096 {
		t.Errorf("maxTokensPerRequest 期望 4096, 实际 %d", validator.maxTokensPerRequest)
	}
}

// buildTestRequest 是辅助函数，用于构建有效的测试请求
func buildTestRequest(model string, messages []entity.Message) *entity.Request {
	return entity.NewRequestBuilder().
		ID(entity.NewRequestID("test-req-id")).
		Model(entity.ModelAlias(model)).
		Messages(messages).
		Context(context.Background()).
		BuildUnsafe()
}

// TestRequestValidator_Validate 测试 Validate 方法的各种场景。
func TestRequestValidator_Validate(t *testing.T) {
	t.Run("消息数量超限应返回错误", func(t *testing.T) {
		validator := NewRequestValidator(2, 4096)

		messages := make([]entity.Message, 3)
		for i := range messages {
			messages[i] = entity.Message{Role: "user", Content: "test"}
		}

		req := buildTestRequest("gpt-4", messages)
		err := validator.Validate(req)

		if err == nil {
			t.Error("消息数量超限应返回错误")
		}
	})

	t.Run("max_tokens 超限应返回错误", func(t *testing.T) {
		validator := NewRequestValidator(100, 10)

		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("gpt-4")).
			Messages([]entity.Message{{Role: "user", Content: "hello"}}).
			MaxTokens(100).
			Context(context.Background()).
			BuildUnsafe()

		err := validator.Validate(req)

		if err == nil {
			t.Error("max_tokens 超限应返回错误")
		}
	})

	t.Run("空消息应返回错误", func(t *testing.T) {
		validator := NewRequestValidator(100, 4096)

		messages := []entity.Message{
			{Role: "user", Content: "valid"},
			{Role: "user"},
		}

		req := buildTestRequest("gpt-4", messages)
		err := validator.Validate(req)

		if err == nil {
			t.Error("空消息应返回错误")
		}
	})

	t.Run("无效角色应返回错误", func(t *testing.T) {
		validator := NewRequestValidator(100, 4096)

		messages := []entity.Message{
			{Role: "invalid_role", Content: "test"},
		}

		req := buildTestRequest("gpt-4", messages)
		err := validator.Validate(req)

		if err == nil {
			t.Error("无效角色应返回错误")
		}
	})

	t.Run("tool_call 缺少 ID 应返回错误", func(t *testing.T) {
		validator := NewRequestValidator(100, 4096)

		messages := []entity.Message{
			{
				Role: "assistant",
				ToolCalls: []entity.ToolCall{
					{Function: entity.ToolCallFunction{Name: "test_function"}},
				},
			},
		}

		req := buildTestRequest("gpt-4", messages)
		err := validator.Validate(req)

		if err == nil {
			t.Error("tool_call 缺少 ID 应返回错误")
		}
	})

	t.Run("tool_call 缺少函数名应返回错误", func(t *testing.T) {
		validator := NewRequestValidator(100, 4096)

		messages := []entity.Message{
			{
				Role: "assistant",
				ToolCalls: []entity.ToolCall{
					{ID: "call_123", Function: entity.ToolCallFunction{Name: ""}},
				},
			},
		}

		req := buildTestRequest("gpt-4", messages)
		err := validator.Validate(req)

		if err == nil {
			t.Error("tool_call 缺少函数名应返回错误")
		}
	})

	t.Run("temperature 小于 0 应返回错误", func(t *testing.T) {
		validator := NewRequestValidator(100, 4096)

		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("gpt-4")).
			Messages([]entity.Message{{Role: "user", Content: "hello"}}).
			Temperature(-0.1).
			Context(context.Background()).
			BuildUnsafe()

		err := validator.Validate(req)

		if err == nil {
			t.Error("temperature 小于 0 应返回错误")
		}
	})

	t.Run("temperature 大于 2 应返回错误", func(t *testing.T) {
		validator := NewRequestValidator(100, 4096)

		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("gpt-4")).
			Messages([]entity.Message{{Role: "user", Content: "hello"}}).
			Temperature(2.1).
			Context(context.Background()).
			BuildUnsafe()

		err := validator.Validate(req)

		if err == nil {
			t.Error("temperature 大于 2 应返回错误")
		}
	})

	t.Run("top_p 小于 0 应返回错误", func(t *testing.T) {
		validator := NewRequestValidator(100, 4096)

		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("gpt-4")).
			Messages([]entity.Message{{Role: "user", Content: "hello"}}).
			TopP(-0.1).
			Context(context.Background()).
			BuildUnsafe()

		err := validator.Validate(req)

		if err == nil {
			t.Error("top_p 小于 0 应返回错误")
		}
	})

	t.Run("top_p 大于 1 应返回错误", func(t *testing.T) {
		validator := NewRequestValidator(100, 4096)

		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("gpt-4")).
			Messages([]entity.Message{{Role: "user", Content: "hello"}}).
			TopP(1.1).
			Context(context.Background()).
			BuildUnsafe()

		err := validator.Validate(req)

		if err == nil {
			t.Error("top_p 大于 1 应返回错误")
		}
	})

	t.Run("有效请求应返回 nil", func(t *testing.T) {
		validator := NewRequestValidator(100, 4096)

		req := buildTestRequest("gpt-4", []entity.Message{{Role: "user", Content: "hello"}})
		err := validator.Validate(req)

		if err != nil {
			t.Errorf("有效请求应返回 nil, 实际错误: %v", err)
		}
	})

	t.Run("边界值 temperature 0 应有效", func(t *testing.T) {
		validator := NewRequestValidator(100, 4096)

		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("gpt-4")).
			Messages([]entity.Message{{Role: "user", Content: "hello"}}).
			Temperature(0).
			Context(context.Background()).
			BuildUnsafe()

		err := validator.Validate(req)

		if err != nil {
			t.Errorf("temperature 0 应有效, 实际错误: %v", err)
		}
	})

	t.Run("边界值 temperature 2 应有效", func(t *testing.T) {
		validator := NewRequestValidator(100, 4096)

		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("gpt-4")).
			Messages([]entity.Message{{Role: "user", Content: "hello"}}).
			Temperature(2).
			Context(context.Background()).
			BuildUnsafe()

		err := validator.Validate(req)

		if err != nil {
			t.Errorf("temperature 2 应有效, 实际错误: %v", err)
		}
	})

	t.Run("边界值 top_p 0 应有效", func(t *testing.T) {
		validator := NewRequestValidator(100, 4096)

		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("gpt-4")).
			Messages([]entity.Message{{Role: "user", Content: "hello"}}).
			TopP(0).
			Context(context.Background()).
			BuildUnsafe()

		err := validator.Validate(req)

		if err != nil {
			t.Errorf("top_p 0 应有效, 实际错误: %v", err)
		}
	})

	t.Run("边界值 top_p 1 应有效", func(t *testing.T) {
		validator := NewRequestValidator(100, 4096)

		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("gpt-4")).
			Messages([]entity.Message{{Role: "user", Content: "hello"}}).
			TopP(1).
			Context(context.Background()).
			BuildUnsafe()

		err := validator.Validate(req)

		if err != nil {
			t.Errorf("top_p 1 应有效, 实际错误: %v", err)
		}
	})

	t.Run("tool 角色应有效", func(t *testing.T) {
		validator := NewRequestValidator(100, 4096)

		messages := []entity.Message{
			{Role: "tool", Content: "tool result"},
		}

		req := buildTestRequest("gpt-4", messages)
		err := validator.Validate(req)

		if err != nil {
			t.Errorf("tool 角色应有效, 实际错误: %v", err)
		}
	})

	t.Run("多个有效消息应通过验证", func(t *testing.T) {
		validator := NewRequestValidator(100, 4096)

		messages := []entity.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello!"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		}

		req := entity.NewRequestBuilder().
			ID(entity.NewRequestID("test-req-id")).
			Model(entity.ModelAlias("gpt-4")).
			Messages(messages).
			Temperature(0.7).
			TopP(0.95).
			Context(context.Background()).
			BuildUnsafe()

		err := validator.Validate(req)

		if err != nil {
			t.Errorf("多个有效消息应通过验证, 实际错误: %v", err)
		}
	})

	t.Run("有效 tool_call 应通过验证", func(t *testing.T) {
		validator := NewRequestValidator(100, 4096)

		messages := []entity.Message{
			{
				Role: "assistant",
				ToolCalls: []entity.ToolCall{
					{ID: "call_123", Function: entity.ToolCallFunction{Name: "get_weather"}},
				},
			},
		}

		req := buildTestRequest("gpt-4", messages)
		err := validator.Validate(req)

		if err != nil {
			t.Errorf("有效 tool_call 应通过验证, 实际错误: %v", err)
		}
	})
}

// TestRequestValidator_Unlimited 测试无限限制的场景。
func TestRequestValidator_Unlimited(t *testing.T) {
	t.Run("maxMessages 为 0 应允许任意数量消息", func(t *testing.T) {
		validator := NewRequestValidator(0, 4096)

		messages := make([]entity.Message, 1000)
		for i := range messages {
			messages[i] = entity.Message{Role: "user", Content: "test"}
		}

		req, _ := entity.NewRequestBuilder().
			Model(entity.ModelAlias("gpt-4")).
			Messages(messages).
			Build()

		if req == nil {
			t.Skip("Build() 返回 nil")
			return
		}

		err := validator.Validate(req)

		if err != nil {
			t.Errorf("maxMessages 为 0 应允许任意数量消息, 实际错误: %v", err)
		}
	})

	t.Run("maxTokens 为 0 应允许任意数量 tokens", func(t *testing.T) {
		validator := NewRequestValidator(100, 0)

		req, _ := entity.NewRequestBuilder().
			Model(entity.ModelAlias("gpt-4")).
			Messages([]entity.Message{{Role: "user", Content: "hello"}}).
			MaxTokens(100000).
			Build()

		if req == nil {
			t.Skip("Build() 返回 nil")
			return
		}

		err := validator.Validate(req)

		if err != nil {
			t.Errorf("maxTokens 为 0 应允许任意数量 tokens, 实际错误: %v", err)
		}
	})
}
