package http

import (
	"encoding/json"
	"testing"

	"llm-proxy/domain/port"
)

func TestProxyHandler_InjectSystemPrompt(t *testing.T) {
	tests := []struct {
		name           string
		prompts        []*SystemPromptConfig
		reqBody        map[string]interface{}
		wantSysContent string
	}{
		{
			name: "注入到没有 system 消息的请求",
			prompts: []*SystemPromptConfig{
				{
					Enabled:  true,
					Position: "before",
					Models:   []string{"*"},
					Content:  "系统提示词",
				},
			},
			reqBody: map[string]interface{}{
				"model": "gpt-4",
				"messages": []interface{}{
					map[string]interface{}{"role": "user", "content": "你好"},
				},
			},
			wantSysContent: "系统提示词",
		},
		{
			name: "position: before 在原有 system 消息前注入",
			prompts: []*SystemPromptConfig{
				{
					Enabled:   true,
					Position:  "before",
					Models:    []string{"*"},
					Separator: "double-newline",
					Content:   "前置内容",
				},
			},
			reqBody: map[string]interface{}{
				"model": "gpt-4",
				"messages": []interface{}{
					map[string]interface{}{"role": "system", "content": "原有系统提示词"},
					map[string]interface{}{"role": "user", "content": "你好"},
				},
			},
			wantSysContent: "前置内容\n\n原有系统提示词",
		},
		{
			name: "position: after 在原有 system 消息后注入",
			prompts: []*SystemPromptConfig{
				{
					Enabled:         true,
					Position:        "after",
					Models:          []string{"*"},
					Separator:       "custom",
					CustomSeparator: " --- ",
					Content:         "后置内容",
				},
			},
			reqBody: map[string]interface{}{
				"model": "gpt-4",
				"messages": []interface{}{
					map[string]interface{}{"role": "system", "content": "原有系统提示词"},
					map[string]interface{}{"role": "user", "content": "你好"},
				},
			},
			wantSysContent: "原有系统提示词 --- 后置内容",
		},
		{
			name: "多个提示词按添加顺序注入",
			prompts: []*SystemPromptConfig{
				{
					Enabled:   true,
					Position:  "before",
					Priority:  20,
					Models:    []string{"*"},
					Separator: "double-newline",
					Content:   "第一个提示词",
				},
				{
					Enabled:   true,
					Position:  "before",
					Priority:  10,
					Models:    []string{"*"},
					Separator: "double-newline",
					Content:   "第二个提示词",
				},
			},
			reqBody: map[string]interface{}{
				"model": "gpt-4",
				"messages": []interface{}{
					map[string]interface{}{"role": "system", "content": "原有"},
				},
			},
			wantSysContent: "第二个提示词\n\n第一个提示词\n\n原有",
		},
		{
			name: "模型不匹配时不注入",
			prompts: []*SystemPromptConfig{
				{
					Enabled:  true,
					Position: "before",
					Models:   []string{"claude-*"},
					Content:  "Claude 提示词",
				},
			},
			reqBody: map[string]interface{}{
				"model": "gpt-4",
				"messages": []interface{}{
					map[string]interface{}{"role": "system", "content": "原有"},
				},
			},
			wantSysContent: "原有",
		},
		{
			name: "已禁用的提示词不注入",
			prompts: []*SystemPromptConfig{
				{
					Enabled:  false,
					Position: "before",
					Models:   []string{"*"},
					Content:  "禁用的提示词",
				},
			},
			reqBody: map[string]interface{}{
				"model": "gpt-4",
				"messages": []interface{}{
					map[string]interface{}{"role": "system", "content": "原有"},
				},
			},
			wantSysContent: "原有",
		},
		{
			name: "没有 model 字段时不注入",
			prompts: []*SystemPromptConfig{
				{
					Enabled:  true,
					Position: "before",
					Models:   []string{"*"},
					Content:  "提示词",
				},
			},
			reqBody: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{"role": "user", "content": "你好"},
				},
			},
			wantSysContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &ProxyHandler{
				logger:              &port.NopLogger{},
				systemPromptManager: NewSystemPromptManager(),
			}
			handler.systemPromptManager.prompts = tt.prompts

			result := handler.injectSystemPrompt(tt.reqBody)

			messages, ok := result["messages"].([]interface{})
			if !ok {
				if tt.wantSysContent != "" {
					t.Errorf("期望有 messages 字段")
				}
				return
			}

			if len(messages) == 0 && tt.wantSysContent != "" {
				t.Errorf("期望至少有一条消息")
				return
			}

			if tt.wantSysContent != "" {
				firstMsg, ok := messages[0].(map[string]interface{})
				if !ok {
					t.Errorf("第一条消息格式不正确")
					return
				}
				if role, ok := firstMsg["role"].(string); !ok || role != "system" {
					t.Errorf("第一条消息不是 system 类型")
				}
				content, ok := firstMsg["content"].(string)
				if !ok {
					t.Errorf("content 字段类型不正确")
				}
				if content != tt.wantSysContent {
					t.Errorf("system content = %q, want %q", content, tt.wantSysContent)
				}
			}
		})
	}
}

func TestProxyHandler_InjectSystemPrompt_RealScenario(t *testing.T) {
	handler := &ProxyHandler{
		logger:              &port.NopLogger{},
		systemPromptManager: NewSystemPromptManager(),
	}

	handler.systemPromptManager.prompts = []*SystemPromptConfig{
		{
			Enabled:   true,
			Position:  "before",
			Priority:  10,
			Models:    []string{"*"},
			Separator: "double-newline",
			Content:   "你是一个专业的助手。当前操作系统是 ${_OS}。",
		},
		{
			Enabled:   true,
			Position:  "after",
			Priority:  20,
			Models:    []string{"*"},
			Separator: "double-newline",
			Content:   "请确保回答安全可靠。",
		},
	}

	reqBodyJSON := `{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": "原有系统提示词"},
			{"role": "user", "content": "你好"}
		]
	}`

	var reqBody map[string]interface{}
	if err := json.Unmarshal([]byte(reqBodyJSON), &reqBody); err != nil {
		t.Fatalf("解析请求体失败: %v", err)
	}

	result := handler.injectSystemPrompt(reqBody)

	messages, ok := result["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		t.Fatal("期望有 messages 数组")
	}

	firstMsg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatal("第一条消息格式不正确")
	}

	content, ok := firstMsg["content"].(string)
	if !ok {
		t.Fatal("content 字段类型不正确")
	}

	expectedPrefix := "你是一个专业的助手。当前操作系统是 "
	if content[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("内容前缀不正确，期望包含 %q", expectedPrefix)
	}

	if !contains(content, "原有系统提示词") {
		t.Errorf("内容应包含原有系统提示词")
	}

	if !contains(content, "请确保回答安全可靠") {
		t.Errorf("内容应包含后置提示词")
	}

	if len(messages) != 2 {
		t.Errorf("消息数量应为 2，实际为 %d", len(messages))
	}

	secondMsg, ok := messages[1].(map[string]interface{})
	if !ok {
		t.Fatal("第二条消息格式不正确")
	}

	if secondMsg["role"] != "user" {
		t.Errorf("第二条消息角色应为 user")
	}
}
