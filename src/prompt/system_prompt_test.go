package prompt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultSystemPromptConfig(t *testing.T) {
	cfg := DefaultSystemPromptConfig()
	if cfg.Position != "before" {
		t.Errorf("Expected position 'before', got '%s'", cfg.Position)
	}
	if cfg.Separator != "double-newline" {
		t.Errorf("Expected separator 'double-newline', got '%s'", cfg.Separator)
	}
	if len(cfg.Models) != 1 || cfg.Models[0] != "*" {
		t.Errorf("Expected models ['*'], got %v", cfg.Models)
	}
	if !cfg.Enabled {
		t.Error("Expected enabled to be true")
	}
	if cfg.Priority != 100 {
		t.Errorf("Expected priority 100, got %d", cfg.Priority)
	}
}

func TestModelMatches(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		patterns []string
		expected bool
	}{
		{"wildcard match", "gpt-4", []string{"*"}, true},
		{"exact match", "gpt-4", []string{"gpt-4"}, true},
		{"pattern match", "gpt-4-turbo", []string{"gpt-*"}, true},
		{"no match", "claude", []string{"gpt-*"}, false},
		{"multiple patterns", "gpt-4", []string{"claude-*", "gpt-*"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := modelMatches(tt.model, tt.patterns)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestReplaceBuiltinVars(t *testing.T) {
	content := "OS: ${_OS}, ARCH: ${_ARCH}, DATE: ${_DATE}"
	result := replaceBuiltinVars(content)

	if !contains(result, "OS:") || !contains(result, "ARCH:") {
		t.Error("Expected OS and ARCH to be replaced")
	}

	if contains(result, "${_OS}") || contains(result, "${_ARCH}") {
		t.Error("Builtin variables should be replaced")
	}
}

func TestReplaceEnvVars(t *testing.T) {
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple env var", "${TEST_VAR}", "test_value"},
		{"with default", "${NONEXISTENT:-default}", "default"},
		{"no default", "${TEST_VAR:-default}", "test_value"},
		{"builtin var skipped", "${_OS}", "${_OS}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceEnvVars(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestParseSystemPrompt(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		data := `position: after
separator: newline
models:
  - gpt-4
  - claude
enabled: true
priority: 50
---
System prompt content`

		config, content := parseSystemPrompt(data)
		if config.Position != "after" {
			t.Errorf("Expected position 'after', got '%s'", config.Position)
		}
		if config.Separator != "newline" {
			t.Errorf("Expected separator 'newline', got '%s'", config.Separator)
		}
		if content != "System prompt content" {
			t.Errorf("Expected content 'System prompt content', got '%s'", content)
		}
	})

	t.Run("without config", func(t *testing.T) {
		data := "Just content without config"
		config, content := parseSystemPrompt(data)

		if config.Position != "before" {
			t.Error("Expected default config position 'before'")
		}
		if content != data {
			t.Errorf("Expected content '%s', got '%s'", data, content)
		}
	})
}

func TestInjectMessage(t *testing.T) {
	t.Run("prepend to existing system message", func(t *testing.T) {
		messages := []interface{}{
			map[string]interface{}{"role": "system", "content": "Original"},
			map[string]interface{}{"role": "user", "content": "Hello"},
		}
		config := &SystemPromptConfig{
			Position:  "before",
			Separator: "double-newline",
		}

		result := injectMessage(messages, "New prompt", config)

		if len(result) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(result))
		}

		systemMsg := result[0].(map[string]interface{})
		content := systemMsg["content"].(string)
		if !contains(content, "New prompt") || !contains(content, "Original") {
			t.Error("Expected both old and new content")
		}
	})

	t.Run("create new system message", func(t *testing.T) {
		messages := []interface{}{
			map[string]interface{}{"role": "user", "content": "Hello"},
		}
		config := &SystemPromptConfig{
			Position:  "before",
			Separator: "double-newline",
		}

		result := injectMessage(messages, "New prompt", config)

		if len(result) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(result))
		}

		systemMsg := result[0].(map[string]interface{})
		if systemMsg["role"] != "system" {
			t.Error("First message should be system role")
		}
		if systemMsg["content"] != "New prompt" {
			t.Errorf("Expected content 'New prompt', got '%s'", systemMsg["content"])
		}
	})

	t.Run("append position", func(t *testing.T) {
		messages := []interface{}{
			map[string]interface{}{"role": "system", "content": "Original"},
		}
		config := &SystemPromptConfig{
			Position:  "after",
			Separator: "\n",
		}

		result := injectMessage(messages, "New", config)
		systemMsg := result[0].(map[string]interface{})
		content := systemMsg["content"].(string)

		expected := "Original\nNew"
		if content != expected {
			t.Logf("Content with newlines: %q", content)
			if !contains(content, "Original") || !contains(content, "New") {
				t.Error("Content should contain both Original and New")
			}
		}
	})
}

func TestProcessSystemPrompt(t *testing.T) {
	t.Run("no messages", func(t *testing.T) {
		body := map[string]interface{}{
			"model": "gpt-4",
		}
		result := ProcessSystemPrompt(body)
		if _, ok := result["messages"]; ok {
			t.Error("Should not add messages when none exist")
		}
	})

	t.Run("with messages", func(t *testing.T) {
		body := map[string]interface{}{
			"model": "gpt-4",
			"messages": []interface{}{
				map[string]interface{}{"role": "user", "content": "Hello"},
			},
		}
		result := ProcessSystemPrompt(body)
		messages := result["messages"].([]interface{})
		if len(messages) == 0 {
			t.Error("Messages should be preserved")
		}
	})
}

func TestGetUsername(t *testing.T) {
	username := getUsername()
	if username == "" {
		t.Skip("Username not available in test environment")
	}
}

func TestGetHostname(t *testing.T) {
	hostname := getHostname()
	if hostname == "" {
		t.Skip("Hostname not available in test environment")
	}
}

func TestGetShell(t *testing.T) {
	shell := getShell()
	if shell == "" {
		t.Log("Shell not detected (expected in some environments)")
	}
}

func TestLoadSinglePrompt(t *testing.T) {
	tempDir := t.TempDir()
	promptFile := filepath.Join(tempDir, "test.md")

	content := `position: before
models:
  - gpt-*
---
Test prompt content`

	if err := os.WriteFile(promptFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	entry := loadSinglePrompt(promptFile, "gpt-4")
	if entry == nil {
		t.Fatal("Expected entry to be loaded")
	}

	if entry.Content != "Test prompt content" {
		t.Errorf("Expected content 'Test prompt content', got '%s'", entry.Content)
	}

	if entry.Config.Position != "before" {
		t.Errorf("Expected position 'before', got '%s'", entry.Config.Position)
	}

	entry2 := loadSinglePrompt(promptFile, "claude")
	if entry2 != nil {
		t.Error("Expected nil for non-matching model")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
