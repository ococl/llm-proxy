package http

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemPromptConfig_IsMatch(t *testing.T) {
	tests := []struct {
		name   string
		config *SystemPromptConfig
		model  string
		want   bool
	}{
		{
			name: "匹配所有模型",
			config: &SystemPromptConfig{
				Enabled: true,
				Models:  []string{"*"},
			},
			model: "gpt-4",
			want:  true,
		},
		{
			name: "匹配通配符前缀",
			config: &SystemPromptConfig{
				Enabled: true,
				Models:  []string{"gpt-*"},
			},
			model: "gpt-4",
			want:  true,
		},
		{
			name: "匹配通配符后缀",
			config: &SystemPromptConfig{
				Enabled: true,
				Models:  []string{"*-turbo"},
			},
			model: "gpt-3.5-turbo",
			want:  true,
		},
		{
			name: "不匹配",
			config: &SystemPromptConfig{
				Enabled: true,
				Models:  []string{"claude-*"},
			},
			model: "gpt-4",
			want:  false,
		},
		{
			name: "已禁用",
			config: &SystemPromptConfig{
				Enabled: false,
				Models:  []string{"*"},
			},
			model: "gpt-4",
			want:  false,
		},
		{
			name: "多个模型模式匹配其中一个",
			config: &SystemPromptConfig{
				Enabled: true,
				Models:  []string{"gpt-*", "claude-*"},
			},
			model: "claude-3",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.IsMatch(tt.model); got != tt.want {
				t.Errorf("IsMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSystemPromptConfig_GetSeparator(t *testing.T) {
	tests := []struct {
		name   string
		config *SystemPromptConfig
		want   string
	}{
		{
			name: "换行符分隔",
			config: &SystemPromptConfig{
				Separator: "newline",
			},
			want: "\n",
		},
		{
			name: "双换行符分隔",
			config: &SystemPromptConfig{
				Separator: "double-newline",
			},
			want: "\n\n",
		},
		{
			name: "无分隔符",
			config: &SystemPromptConfig{
				Separator: "none",
			},
			want: "",
		},
		{
			name: "自定义分隔符",
			config: &SystemPromptConfig{
				Separator:       "custom",
				CustomSeparator: "---",
			},
			want: "---",
		},
		{
			name: "默认双换行符",
			config: &SystemPromptConfig{
				Separator: "",
			},
			want: "\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.GetSeparator(); got != tt.want {
				t.Errorf("GetSeparator() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMatchWildcard(t *testing.T) {
	tests := []struct {
		pattern string
		s       string
		want    bool
	}{
		{"*", "anything", true},
		{"*", "", true},
		{"gpt-*", "gpt-4", true},
		{"gpt-*", "gpt-3.5-turbo", true},
		{"gpt-*", "claude-3", false},
		{"*-turbo", "gpt-3.5-turbo", true},
		{"*-turbo", "gpt-4", false},
		{"exact", "exact", true},
		{"exact", "exactly", false},
		{"claude-3-*", "claude-3-opus", true},
		{"claude-3-*", "claude-3", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"/"+tt.s, func(t *testing.T) {
			if got := matchWildcard(tt.pattern, tt.s); got != tt.want {
				t.Errorf("matchWildcard(%q, %q) = %v, want %v", tt.pattern, tt.s, got, tt.want)
			}
		})
	}
}

func TestSystemPromptManager_LoadSystemPrompts(t *testing.T) {
	tempDir := t.TempDir()

	singleFile := filepath.Join(tempDir, "system_prompt.md")
	singleContent := `position: before
models: ["*"]
---

这是一个系统提示词。`
	if err := os.WriteFile(singleFile, []byte(singleContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	originalDir, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("切换目录失败: %v", err)
	}
	defer os.Chdir(originalDir)

	manager := NewSystemPromptManager()
	if err := manager.LoadSystemPrompts(); err != nil {
		t.Errorf("LoadSystemPrompts() error = %v", err)
	}

	prompts := manager.GetPrompts()
	if len(prompts) == 0 {
		t.Error("应该加载到提示词")
	}
}

func TestSystemPromptManager_LoadSystemPrompts_Directory(t *testing.T) {
	tempDir := t.TempDir()

	systemPromptsDir := filepath.Join(tempDir, "system_prompts")
	if err := os.MkdirAll(systemPromptsDir, 0755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}

	files := map[string]string{
		"01_base.md": `position: before
priority: 10
models: ["*"]
---

基础提示词。`,
		"02_security.md": `position: after
priority: 20
models: ["*"]
---

安全提示词。`,
		"ignored.txt": "应该被忽略",
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(systemPromptsDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("写入测试文件 %s 失败: %v", name, err)
		}
	}

	originalDir, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("切换目录失败: %v", err)
	}
	defer os.Chdir(originalDir)

	manager := NewSystemPromptManager()
	if err := manager.LoadSystemPrompts(); err != nil {
		t.Errorf("LoadSystemPrompts() error = %v", err)
	}

	prompts := manager.GetPrompts()
	if len(prompts) != 2 {
		t.Errorf("应该加载 2 个提示词，实际加载 %d 个", len(prompts))
	}

	if len(prompts) >= 2 {
		if prompts[0].Priority != 10 {
			t.Errorf("第一个提示词的 priority 应该是 10，实际是 %d", prompts[0].Priority)
		}
		if prompts[1].Priority != 20 {
			t.Errorf("第二个提示词的 priority 应该是 20，实际是 %d", prompts[1].Priority)
		}
	}
}

func TestParseSystemPromptFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    *SystemPromptConfig
	}{
		{
			name: "包含 YAML 配置",
			content: `position: before
separator: newline
models: ["gpt-*"]
enabled: true
priority: 50
---

提示词内容`,
			want: &SystemPromptConfig{
				Position:  "before",
				Separator: "newline",
				Models:    []string{"gpt-*"},
				Enabled:   true,
				Priority:  50,
				Content:   "提示词内容",
			},
		},
		{
			name:    "无 YAML 配置",
			content: `纯文本提示词`,
			want: &SystemPromptConfig{
				Position:  "before",
				Separator: "double-newline",
				Models:    []string{"*"},
				Enabled:   true,
				Priority:  100,
				Content:   "纯文本提示词",
			},
		},
		{
			name: "包含内置变量",
			content: `---
当前操作系统: ${_OS}
架构: ${_ARCH}`,
			want: &SystemPromptConfig{
				Position:  "before",
				Separator: "double-newline",
				Models:    []string{"*"},
				Enabled:   true,
				Priority:  100,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSystemPromptFile("test.md", []byte(tt.content), map[string]string{})
			if err != nil {
				t.Errorf("parseSystemPromptFile() error = %v", err)
				return
			}

			if got.Position != tt.want.Position {
				t.Errorf("Position = %v, want %v", got.Position, tt.want.Position)
			}
			if got.Separator != tt.want.Separator {
				t.Errorf("Separator = %v, want %v", got.Separator, tt.want.Separator)
			}
			if got.Enabled != tt.want.Enabled {
				t.Errorf("Enabled = %v, want %v", got.Enabled, tt.want.Enabled)
			}
			if got.Priority != tt.want.Priority {
				t.Errorf("Priority = %v, want %v", got.Priority, tt.want.Priority)
			}
			if tt.name == "包含内置变量" {
				if !strings.Contains(got.Content, "当前操作系统:") {
					t.Errorf("Content 应该包含扩展后的操作系统信息")
				}
			} else if got.Content != tt.want.Content {
				t.Errorf("Content = %v, want %v", got.Content, tt.want.Content)
			}
		})
	}
}

func TestExpandEnvVars(t *testing.T) {
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "简单环境变量",
			content: "值: ${TEST_VAR}",
			want:    "值: test_value",
		},
		{
			name:    "带默认值的环境变量",
			content: "值: ${NONEXISTENT_VAR:-default}",
			want:    "值: default",
		},
		{
			name:    "存在的环境变量也使用默认值",
			content: "值: ${TEST_VAR:-default}",
			want:    "值: test_value",
		},
		{
			name:    "无大括号的环境变量不处理",
			content: "值: $TEST_VAR",
			want:    "值: $TEST_VAR",
		},
		{
			name:    "未闭合的大括号不处理",
			content: "值: ${TEST_VAR",
			want:    "值: ${TEST_VAR",
		},
		{
			name:    "混合文本",
			content: "前缀 ${TEST_VAR} 后缀",
			want:    "前缀 test_value 后缀",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expandEnvVars(tt.content); got != tt.want {
				t.Errorf("expandEnvVars() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSystemPromptManager_GetPromptsForModel(t *testing.T) {
	manager := NewSystemPromptManager()
	manager.prompts = []*SystemPromptConfig{
		{Enabled: true, Models: []string{"*"}, Priority: 100, Content: "all"},
		{Enabled: true, Models: []string{"gpt-*"}, Priority: 50, Content: "gpt"},
		{Enabled: true, Models: []string{"claude-*"}, Priority: 50, Content: "claude"},
		{Enabled: false, Models: []string{"*"}, Priority: 10, Content: "disabled"},
	}

	tests := []struct {
		name     string
		model    string
		wantLen  int
		wantCont []string
	}{
		{
			name:     "GPT 模型",
			model:    "gpt-4",
			wantLen:  2,
			wantCont: []string{"all", "gpt"},
		},
		{
			name:     "Claude 模型",
			model:    "claude-3",
			wantLen:  2,
			wantCont: []string{"all", "claude"},
		},
		{
			name:     "其他模型",
			model:    "other-model",
			wantLen:  1,
			wantCont: []string{"all"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompts := manager.GetPromptsForModel(tt.model)
			if len(prompts) != tt.wantLen {
				t.Errorf("GetPromptsForModel() 返回数量 = %d, want %d", len(prompts), tt.wantLen)
			}
			for i, want := range tt.wantCont {
				if i < len(prompts) && prompts[i].Content != want {
					t.Errorf("prompts[%d].Content = %q, want %q", i, prompts[i].Content, want)
				}
			}
		})
	}
}
