package backend

import (
	"net/http"
	"testing"

	"llm-proxy/domain/port"
)

func TestBackendClientAdapter_Send_Success(t *testing.T) {
	mockClient := &http.Client{}
	httpClient := NewHTTPClient(mockClient)
	adapter := NewBackendClientAdapter(httpClient, &port.NopLogger{}, &port.NopBodyLogger{})

	if adapter == nil {
		t.Error("BackendClientAdapter should not be nil")
	}

	if adapter.GetHTTPClient() != mockClient {
		t.Error("GetHTTPClient should return the underlying HTTP client")
	}
}

func TestBackendClientAdapter_Send_WithAllOptions(t *testing.T) {
	mockClient := &http.Client{}
	httpClient := NewHTTPClient(mockClient)
	adapter := NewBackendClientAdapter(httpClient, &port.NopLogger{}, &port.NopBodyLogger{})

	if adapter == nil {
		t.Fatal("BackendClientAdapter should not be nil")
	}

	httpClientFromAdapter := adapter.GetHTTPClient()
	if httpClientFromAdapter == nil {
		t.Error("GetHTTPClient should not return nil")
	}
}

func TestHTTPClient_NewHTTPClient_NilClient(t *testing.T) {
	client := NewHTTPClient(nil)
	if client == nil {
		t.Error("NewHTTPClient should not return nil even with nil input")
	}
	if client.GetHTTPClient() == nil {
		t.Error("HTTP client should use http.DefaultClient when nil is passed")
	}
}

func TestHTTPClient_NewHTTPClient_WithClient(t *testing.T) {
	mockClient := &http.Client{}
	client := NewHTTPClient(mockClient)

	if client.GetHTTPClient() != mockClient {
		t.Error("NewHTTPClient should use the provided client")
	}
}

// Test isThinkingModeEnabled 函数
func TestIsThinkingModeEnabled(t *testing.T) {
	tests := []struct {
		name     string
		body     map[string]interface{}
		expected bool
	}{
		{
			name:     "extra_body 中 thinking 启用",
			body:     map[string]interface{}{"extra_body": map[string]interface{}{"thinking": map[string]interface{}{"type": "enabled"}}},
			expected: true,
		},
		{
			name:     "顶层 thinking 启用",
			body:     map[string]interface{}{"thinking": map[string]interface{}{"type": "enabled"}},
			expected: true,
		},
		{
			name:     "thinking 类型为 non_thinking",
			body:     map[string]interface{}{"thinking": map[string]interface{}{"type": "non_thinking"}},
			expected: false,
		},
		{
			name:     "thinking 不存在",
			body:     map[string]interface{}{"model": "deepseek-chat"},
			expected: false,
		},
		{
			name:     "extra_body 不存在",
			body:     map[string]interface{}{"messages": []interface{}{}},
			expected: false,
		},
		{
			name:     "thinking 字段为空 map",
			body:     map[string]interface{}{"thinking": map[string]interface{}{}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isThinkingModeEnabled(tt.body)
			if result != tt.expected {
				t.Errorf("isThinkingModeEnabled() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// Test fixReasoningContentInMessages 函数
func TestFixReasoningContentInMessages(t *testing.T) {
	t.Run("无 tool_calls 的 assistant 消息不修改", func(t *testing.T) {
		messages := []interface{}{
			map[string]interface{}{
				"role":    "assistant",
				"content": "你好",
			},
		}
		body := map[string]interface{}{
			"messages": messages,
		}

		fixReasoningContentInMessages(body)

		// 确保没有添加 reasoning_content
		msg := messages[0].(map[string]interface{})
		if _, exists := msg["reasoning_content"]; exists {
			t.Error("无 tool_calls 的 assistant 消息不应添加 reasoning_content")
		}
	})

	t.Run("包含 tool_calls 时始终添加 reasoning_content", func(t *testing.T) {
		messages := []interface{}{
			map[string]interface{}{
				"role":       "assistant",
				"content":    nil,
				"tool_calls": []interface{}{},
			},
		}
		body := map[string]interface{}{
			"messages": messages,
		}

		fixReasoningContentInMessages(body)

		// 即使没有启用 thinking 模式，包含 tool_calls 的消息也应该有 reasoning_content
		msg := messages[0].(map[string]interface{})
		if _, exists := msg["reasoning_content"]; !exists {
			t.Error("包含 tool_calls 的 assistant 消息应该有 reasoning_content")
		}
	})

	t.Run("已有顶层 reasoning_content 不修改", func(t *testing.T) {
		messages := []interface{}{
			map[string]interface{}{
				"role":              "assistant",
				"content":           nil,
				"tool_calls":        []interface{}{},
				"reasoning_content": "已存在的 reasoning",
			},
		}
		body := map[string]interface{}{
			"extra_body": map[string]interface{}{"thinking": map[string]interface{}{"type": "enabled"}},
			"messages":   messages,
		}

		fixReasoningContentInMessages(body)

		msg := messages[0].(map[string]interface{})
		if msg["reasoning_content"] != "已存在的 reasoning" {
			t.Error("已有顶层 reasoning_content 不应被修改")
		}
	})

	t.Run("从 additional_kwargs 提升 reasoning_content", func(t *testing.T) {
		messages := []interface{}{
			map[string]interface{}{
				"role":       "assistant",
				"content":    nil,
				"tool_calls": []interface{}{map[string]interface{}{"id": "call_1"}},
				"additional_kwargs": map[string]interface{}{
					"reasoning_content": "需要提升的 reasoning",
				},
			},
		}
		body := map[string]interface{}{
			"extra_body": map[string]interface{}{"thinking": map[string]interface{}{"type": "enabled"}},
			"messages":   messages,
		}

		fixReasoningContentInMessages(body)

		msg := messages[0].(map[string]interface{})
		rc, exists := msg["reasoning_content"]
		if !exists {
			t.Fatal("reasoning_content 应该被提升到顶层")
		}
		if rc != "需要提升的 reasoning" {
			t.Errorf("reasoning_content = %v, 期望 '需要提升的 reasoning'", rc)
		}
		// additional_kwargs 中的应该被移除
		if ak, ok := msg["additional_kwargs"].(map[string]interface{}); ok {
			if _, exists := ak["reasoning_content"]; exists {
				t.Error("additional_kwargs 中的 reasoning_content 应该被移除")
			}
		}
	})

	t.Run("非 assistant 角色不修改", func(t *testing.T) {
		messages := []interface{}{
			map[string]interface{}{
				"role":    "user",
				"content": "你好",
				"additional_kwargs": map[string]interface{}{
					"reasoning_content": "这不应该被提升",
				},
			},
		}
		body := map[string]interface{}{
			"extra_body": map[string]interface{}{"thinking": map[string]interface{}{"type": "enabled"}},
			"messages":   messages,
		}

		fixReasoningContentInMessages(body)

		msg := messages[0].(map[string]interface{})
		if _, exists := msg["reasoning_content"]; exists {
			t.Error("非 assistant 角色不应添加 reasoning_content")
		}
	})

	t.Run("无 tool_calls 的 assistant 消息不修改", func(t *testing.T) {
		messages := []interface{}{
			map[string]interface{}{
				"role":    "assistant",
				"content": "你好",
				"additional_kwargs": map[string]interface{}{
					"reasoning_content": "这不应该被提升",
				},
			},
		}
		body := map[string]interface{}{
			"extra_body": map[string]interface{}{"thinking": map[string]interface{}{"type": "enabled"}},
			"messages":   messages,
		}

		fixReasoningContentInMessages(body)

		msg := messages[0].(map[string]interface{})
		if _, exists := msg["reasoning_content"]; exists {
			t.Error("无 tool_calls 的 assistant 消息不应添加 reasoning_content")
		}
	})

	t.Run("多个 messages 只处理需要的", func(t *testing.T) {
		messages := []interface{}{
			map[string]interface{}{
				"role":    "user",
				"content": "你好",
			},
			map[string]interface{}{
				"role":       "assistant",
				"content":    nil,
				"tool_calls": []interface{}{},
				"additional_kwargs": map[string]interface{}{
					"reasoning_content": "需要提升",
				},
			},
			map[string]interface{}{
				"role":    "assistant",
				"content": "不需要 reasoning",
			},
		}
		body := map[string]interface{}{
			"extra_body": map[string]interface{}{"thinking": map[string]interface{}{"type": "enabled"}},
			"messages":   messages,
		}

		fixReasoningContentInMessages(body)

		// 只处理第二个消息
		msg2 := messages[1].(map[string]interface{})
		if _, exists := msg2["reasoning_content"]; !exists {
			t.Error("第二个消息的 reasoning_content 应该被提升")
		}

		// 第三个消息不应该被修改
		msg3 := messages[2].(map[string]interface{})
		if _, exists := msg3["reasoning_content"]; exists {
			t.Error("第三个消息不应有 reasoning_content")
		}
	})

	t.Run("无 reasoning_content 时添加空字符串", func(t *testing.T) {
		messages := []interface{}{
			map[string]interface{}{
				"role":       "assistant",
				"content":    nil,
				"tool_calls": []interface{}{map[string]interface{}{"id": "call_1"}},
				// 既没有顶层 reasoning_content，也没有 additional_kwargs.reasoning_content
			},
		}
		body := map[string]interface{}{
			"extra_body": map[string]interface{}{"thinking": map[string]interface{}{"type": "enabled"}},
			"messages":   messages,
		}

		fixReasoningContentInMessages(body)

		msg := messages[0].(map[string]interface{})
		rc, exists := msg["reasoning_content"]
		if !exists {
			t.Fatal("应该添加 reasoning_content 字段")
		}
		if rc != "" {
			t.Errorf("reasoning_content 应该是空字符串，实际为 %v", rc)
		}
	})
}
