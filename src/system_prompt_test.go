package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseSystemPrompt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantPos  string
		wantPri  int
		wantCont string
	}{
		{
			"simple text without config",
			"hello world",
			"before",
			100,
			"hello world",
		},
		{
			"with config at bottom delimiter only",
			"position: after\npriority: 10\nenabled: true\n---\ncontent here",
			"after",
			10,
			"content here",
		},
		{
			"disabled config",
			"enabled: false\n---\nshould not load",
			"before",
			100,
			"should not load",
		},
		{
			"no delimiter means all content",
			"some prompt without config",
			"before",
			100,
			"some prompt without config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotConf, gotCont := parseSystemPrompt(tt.input)
			if gotConf.Position != tt.wantPos {
				t.Errorf("position = %v, want %v", gotConf.Position, tt.wantPos)
			}
			if gotConf.Priority != tt.wantPri {
				t.Errorf("priority = %v, want %v", gotConf.Priority, tt.wantPri)
			}
			if gotCont != tt.wantCont {
				t.Errorf("content = %v, want %v", gotCont, tt.wantCont)
			}
		})
	}
}

func TestReplaceEnvVars(t *testing.T) {
	t.Setenv("TEST_VAR", "foo")
	tests := []struct {
		input string
		want  string
	}{
		{"hello ${TEST_VAR}", "hello foo"},
		{"hello ${NON_EXISTENT:-bar}", "hello bar"},
		{"hello ${NON_EXISTENT}", "hello "},
	}

	for _, tt := range tests {
		if got := replaceEnvVars(tt.input); got != tt.want {
			t.Errorf("replaceEnvVars(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestReplaceBuiltinVars(t *testing.T) {
	content := "OS: ${_OS}, ARCH: ${_ARCH}"
	got := replaceBuiltinVars(content)

	if !contains(got, runtime.GOOS) {
		t.Errorf("expected OS %s in %q", runtime.GOOS, got)
	}
	if !contains(got, runtime.GOARCH) {
		t.Errorf("expected ARCH %s in %q", runtime.GOARCH, got)
	}
}

func TestInjectMessage(t *testing.T) {
	t.Run("before position", func(t *testing.T) {
		messages := []interface{}{
			map[string]interface{}{"role": "system", "content": "original"},
			map[string]interface{}{"role": "user", "content": "hi"},
		}
		conf := &SystemPromptConfig{Position: "before", Separator: "newline", Enabled: true}
		got := injectMessage(messages, "injected", conf)

		content := got[0].(map[string]interface{})["content"].(string)
		if content != "injected\noriginal" {
			t.Errorf("before failed: got %q", content)
		}
	})

	t.Run("after position", func(t *testing.T) {
		messages := []interface{}{
			map[string]interface{}{"role": "system", "content": "original"},
			map[string]interface{}{"role": "user", "content": "hi"},
		}
		conf := &SystemPromptConfig{Position: "after", Separator: "newline", Enabled: true}
		got := injectMessage(messages, "injected", conf)

		content := got[0].(map[string]interface{})["content"].(string)
		if content != "original\ninjected" {
			t.Errorf("after failed: got %q", content)
		}
	})

	t.Run("no existing system message", func(t *testing.T) {
		messages := []interface{}{
			map[string]interface{}{"role": "user", "content": "hi"},
		}
		conf := &SystemPromptConfig{Position: "before", Enabled: true}
		got := injectMessage(messages, "new system", conf)

		if len(got) != 2 {
			t.Errorf("expected 2 messages, got %d", len(got))
		}
		first := got[0].(map[string]interface{})
		if first["role"] != "system" || first["content"] != "new system" {
			t.Errorf("first message should be new system: %v", first)
		}
	})
}

func TestModelMatches(t *testing.T) {
	tests := []struct {
		model    string
		patterns []string
		want     bool
	}{
		{"gpt-4", []string{"*"}, true},
		{"claude-3-opus", []string{"claude-*"}, true},
		{"gpt-4-turbo", []string{"gpt-4*"}, true},
		{"gemini-pro", []string{"claude-*", "gpt-*"}, false},
	}

	for _, tt := range tests {
		if got := modelMatches(tt.model, tt.patterns); got != tt.want {
			t.Errorf("modelMatches(%q, %v) = %v, want %v", tt.model, tt.patterns, got, tt.want)
		}
	}
}

func TestLoadAllSystemPrompts(t *testing.T) {
	tmpDir := t.TempDir()

	singleFile := filepath.Join(tmpDir, "system_prompt.md")
	os.WriteFile(singleFile, []byte("priority: 50\n---\nsingle file"), 0644)

	promptsDir := filepath.Join(tmpDir, "system_prompts")
	os.MkdirAll(promptsDir, 0755)
	os.WriteFile(filepath.Join(promptsDir, "01_first.md"), []byte("priority: 10\n---\nfirst"), 0644)
	os.WriteFile(filepath.Join(promptsDir, "02_second.md"), []byte("priority: 20\n---\nsecond"), 0644)

	entries := loadAllSystemPrompts(tmpDir, "gpt-4")

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if entries[0].Config.Priority != 10 {
		t.Errorf("first entry priority should be 10, got %d", entries[0].Config.Priority)
	}
	if entries[1].Config.Priority != 20 {
		t.Errorf("second entry priority should be 20, got %d", entries[1].Config.Priority)
	}
	if entries[2].Config.Priority != 50 {
		t.Errorf("third entry priority should be 50, got %d", entries[2].Config.Priority)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
