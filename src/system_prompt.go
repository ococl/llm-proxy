package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type SystemPromptConfig struct {
	Position        string   `yaml:"position"`
	Separator       string   `yaml:"separator"`
	CustomSeparator string   `yaml:"custom_separator"`
	Models          []string `yaml:"models"`
	Enabled         bool     `yaml:"enabled"`
	Priority        int      `yaml:"priority"`
}

type SystemPromptEntry struct {
	Config   *SystemPromptConfig
	Content  string
	FilePath string
}

func DefaultSystemPromptConfig() *SystemPromptConfig {
	return &SystemPromptConfig{
		Position:  "before",
		Separator: "double-newline",
		Models:    []string{"*"},
		Enabled:   true,
		Priority:  100,
	}
}

func ProcessSystemPrompt(body map[string]interface{}) map[string]interface{} {
	exePath, err := os.Executable()
	if err != nil {
		return body
	}
	exeDir := filepath.Dir(exePath)

	model, _ := body["model"].(string)
	entries := loadAllSystemPrompts(exeDir, model)
	if len(entries) == 0 {
		return body
	}

	messages, ok := body["messages"].([]interface{})
	if !ok {
		return body
	}

	for _, entry := range entries {
		content := replaceBuiltinVars(entry.Content)
		content = replaceEnvVars(content)
		messages = injectMessage(messages, content, entry.Config)
	}

	body["messages"] = messages
	return body
}

func loadAllSystemPrompts(exeDir string, model string) []*SystemPromptEntry {
	var entries []*SystemPromptEntry

	singleFile := filepath.Join(exeDir, "system_prompt.md")
	if entry := loadSinglePrompt(singleFile, model); entry != nil {
		entries = append(entries, entry)
	}

	promptsDir := filepath.Join(exeDir, "system_prompts")
	if files, err := filepath.Glob(filepath.Join(promptsDir, "*.md")); err == nil {
		for _, f := range files {
			if entry := loadSinglePrompt(f, model); entry != nil {
				entries = append(entries, entry)
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Config.Priority != entries[j].Config.Priority {
			return entries[i].Config.Priority < entries[j].Config.Priority
		}
		return entries[i].FilePath < entries[j].FilePath
	})

	return entries
}

func loadSinglePrompt(filePath string, model string) *SystemPromptEntry {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	config, content := parseSystemPrompt(string(data))
	if !config.Enabled {
		return nil
	}

	if !modelMatches(model, config.Models) {
		return nil
	}

	return &SystemPromptEntry{
		Config:   config,
		Content:  content,
		FilePath: filePath,
	}
}

func parseSystemPrompt(data string) (*SystemPromptConfig, string) {
	config := DefaultSystemPromptConfig()
	content := data

	idx := strings.Index(data, "\n---")
	if idx != -1 {
		yamlPart := data[:idx]
		content = strings.TrimSpace(data[idx+4:])
		if err := yaml.Unmarshal([]byte(yamlPart), config); err != nil {
			DebugSugar.Warnw("解析system prompt配置失败，使用默认配置", "error", err)
		}
	}

	return config, content
}

func modelMatches(model string, patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == "*" {
			return true
		}
		matched, _ := filepath.Match(pattern, model)
		if matched {
			return true
		}
	}
	return false
}

func replaceBuiltinVars(content string) string {
	now := time.Now()

	builtins := map[string]string{
		"_OS":       runtime.GOOS,
		"_ARCH":     runtime.GOARCH,
		"_HOSTNAME": getHostname(),
		"_USER":     getUsername(),
		"_HOME":     os.Getenv("HOME"),
		"_SHELL":    getShell(),
		"_LANG":     os.Getenv("LANG"),
		"_DATE":     now.Format("2006-01-02"),
		"_TIME":     now.Format("15:04:05"),
		"_DATETIME": now.Format("2006-01-02T15:04:05"),
	}

	if builtins["_HOME"] == "" {
		builtins["_HOME"] = os.Getenv("USERPROFILE")
	}

	for key, val := range builtins {
		content = strings.ReplaceAll(content, "${"+key+"}", val)
	}

	return content
}

func getHostname() string {
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return ""
}

func getUsername() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return os.Getenv("USERNAME")
}

func getShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return filepath.Base(shell)
	}
	if comspec := os.Getenv("COMSPEC"); comspec != "" {
		base := filepath.Base(comspec)
		if strings.EqualFold(base, "cmd.exe") {
			return "cmd"
		}
		return base
	}
	if psPath := os.Getenv("PSModulePath"); psPath != "" {
		return "powershell"
	}
	return ""
}

func replaceEnvVars(content string) string {
	re := regexp.MustCompile(`\$\{([^}_][^}:]*?)(?::-([^}]*))?\}`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		sub := re.FindStringSubmatch(match)
		envVar := sub[1]
		defaultVal := ""
		if len(sub) > 2 {
			defaultVal = sub[2]
		}

		val := os.Getenv(envVar)
		if val == "" {
			return defaultVal
		}
		return val
	})
}

func injectMessage(messages []interface{}, content string, config *SystemPromptConfig) []interface{} {
	separator := "\n\n"
	switch config.Separator {
	case "newline":
		separator = "\n"
	case "none":
		separator = ""
	case "custom":
		separator = config.CustomSeparator
	}

	systemIdx := -1
	for i, m := range messages {
		if msg, ok := m.(map[string]interface{}); ok {
			if role, _ := msg["role"].(string); role == "system" {
				systemIdx = i
				break
			}
		}
	}

	if systemIdx == -1 {
		newMsg := map[string]interface{}{
			"role":    "system",
			"content": content,
		}
		return append([]interface{}{newMsg}, messages...)
	}

	msg, ok := messages[systemIdx].(map[string]interface{})
	if !ok {
		return messages
	}
	oldContent, _ := msg["content"].(string)

	switch config.Position {
	case "before":
		msg["content"] = content + separator + oldContent
	case "after":
		msg["content"] = oldContent + separator + content
	default:
		msg["content"] = content + separator + oldContent
	}

	return messages
}
