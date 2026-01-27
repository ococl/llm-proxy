package http

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SystemPromptConfig 系统提示词配置结构
type SystemPromptConfig struct {
	Position        string   `yaml:"position"`         // 注入位置: "before" 或 "after"
	Separator       string   `yaml:"separator"`        // 分隔符: "newline", "double-newline", "none", "custom"
	CustomSeparator string   `yaml:"custom_separator"` // 自定义分隔符
	Models          []string `yaml:"models"`           // 模型匹配规则，支持通配符
	Enabled         bool     `yaml:"enabled"`          // 是否启用
	Priority        int      `yaml:"priority"`         // 加载优先级（数值小的优先）
	Content         string   // 提示词内容（不包含在 YAML 中）
	FilePath        string   // 文件路径（用于调试）
}

// IsMatch 判断模型名称是否匹配配置
func (c *SystemPromptConfig) IsMatch(model string) bool {
	if !c.Enabled {
		return false
	}
	if len(c.Models) == 0 {
		return true
	}
	for _, pattern := range c.Models {
		if matchWildcard(pattern, model) {
			return true
		}
	}
	return false
}

// GetSeparator 获取实际使用的分隔符
func (c *SystemPromptConfig) GetSeparator() string {
	switch c.Separator {
	case "newline":
		return "\n"
	case "double-newline":
		return "\n\n"
	case "none":
		return ""
	case "custom":
		return c.CustomSeparator
	default:
		return "\n\n"
	}
}

// SystemPromptManager 系统提示词管理器
type SystemPromptManager struct {
	prompts          []*SystemPromptConfig
	customVariables  map[string]string
	lastLoadTime     time.Time
	fileModTimes     map[string]time.Time // 文件修改时间跟踪（用于缓存）
	directoryModTime time.Time            // 目录修改时间跟踪
}

// NewSystemPromptManager 创建系统提示词管理器
func NewSystemPromptManager() *SystemPromptManager {
	return &SystemPromptManager{
		prompts:          make([]*SystemPromptConfig, 0),
		customVariables:  make(map[string]string),
		fileModTimes:     make(map[string]time.Time),
		directoryModTime: time.Time{},
	}
}

// SetCustomVariables 设置自定义变量
func (m *SystemPromptManager) SetCustomVariables(vars map[string]string) {
	m.customVariables = vars
	m.clearCache()
}

// clearCache 清除缓存
func (m *SystemPromptManager) clearCache() {
	m.lastLoadTime = time.Time{}
	m.fileModTimes = make(map[string]time.Time)
	m.directoryModTime = time.Time{}
}

// LoadSystemPrompts 从文件加载系统提示词配置（带缓存）
func (m *SystemPromptManager) LoadSystemPrompts() error {
	// 检查单文件
	singleFileStat, singleFileErr := os.Stat("system_prompt.md")
	if singleFileErr == nil && !singleFileStat.ModTime().After(m.lastLoadTime) {
		modTime, exists := m.fileModTimes["system_prompt.md"]
		if exists && modTime.Equal(singleFileStat.ModTime()) {
			return nil
		}
		m.fileModTimes["system_prompt.md"] = singleFileStat.ModTime()
	}

	// 检查目录
	dirStat, dirErr := os.Stat("system_prompts")
	if dirErr == nil && dirStat.IsDir() {
		if !m.directoryModTime.IsZero() && m.directoryModTime.Equal(dirStat.ModTime()) {
			return nil
		}
		m.directoryModTime = dirStat.ModTime()
	}

	m.prompts = make([]*SystemPromptConfig, 0)

	// 1. 尝试加载单个 system_prompt.md 文件
	if err := m.loadSingleFile("system_prompt.md"); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("加载 system_prompt.md 失败: %w", err)
		}
	}

	// 2. 尝试加载 system_prompts/ 目录
	if err := m.loadDirectory("system_prompts"); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("加载 system_prompts/ 目录失败: %w", err)
		}
	}

	m.lastLoadTime = time.Now()

	// 3. 按 priority 排序，然后按文件名排序
	sort.Slice(m.prompts, func(i, j int) bool {
		if m.prompts[i].Priority != m.prompts[j].Priority {
			return m.prompts[i].Priority < m.prompts[j].Priority
		}
		return m.prompts[i].FilePath < m.prompts[j].FilePath
	})

	return nil
}

// loadSingleFile 加载单个文件
func (m *SystemPromptManager) loadSingleFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	config, err := parseSystemPromptFile(path, data, m.customVariables)
	if err != nil {
		return err
	}

	m.prompts = append(m.prompts, config)
	return nil
}

// loadDirectory 加载目录中的所有 .md 文件
func (m *SystemPromptManager) loadDirectory(dirPath string) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(dirPath, file.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		config, err := parseSystemPromptFile(filePath, data, m.customVariables)
		if err != nil {
			continue
		}

		m.prompts = append(m.prompts, config)
	}

	return nil
}

// parseSystemPromptFile 解析系统提示词文件
func parseSystemPromptFile(filePath string, data []byte, customVars map[string]string) (*SystemPromptConfig, error) {
	content := string(data)

	var yamlPart, contentPart string
	if idx := strings.Index(content, "---"); idx >= 0 {
		yamlPart = strings.TrimSpace(content[:idx])
		contentPart = strings.TrimSpace(content[idx+3:])
	} else {
		yamlPart = ""
		contentPart = content
	}

	config := &SystemPromptConfig{
		FilePath:        filePath,
		Position:        "before",
		Separator:       "double-newline",
		Models:          []string{"*"},
		Enabled:         true,
		Priority:        100,
		Content:         contentPart,
		CustomSeparator: "",
	}

	if yamlPart != "" {
		if err := yaml.Unmarshal([]byte(yamlPart), config); err != nil {
			return nil, fmt.Errorf("解析 YAML 配置失败: %w", err)
		}
	}

	config.Content = expandVariablesWithCustomVars(config.Content, customVars)

	return config, nil
}

// expandVariablesWithCustomVars 扩展变量（内置变量、自定义变量和环境变量）
// 自定义变量会覆盖内置变量
func expandVariablesWithCustomVars(content string, customVars map[string]string) string {
	now := time.Now()

	builtinVars := map[string]string{
		"_OS":       runtime.GOOS,
		"_ARCH":     runtime.GOARCH,
		"_HOSTNAME": getHostname(),
		"_USER":     getUsername(),
		"_HOME":     getHomeDir(),
		"_SHELL":    getShell(),
		"_LANG":     getLang(),
		"_DATE":     now.Format("2006-01-02"),
		"_TIME":     now.Format("15:04:05"),
		"_DATETIME": now.Format("2006-01-02T15:04:05"),
	}

	// 合并自定义变量（自定义变量优先级高于内置变量）
	for name, value := range customVars {
		builtinVars[name] = value
	}

	for name, value := range builtinVars {
		placeholder := fmt.Sprintf("${%s}", name)
		content = strings.ReplaceAll(content, placeholder, value)
	}

	content = expandEnvVars(content)

	return content
}

// expandEnvVars 扩展环境变量
func expandEnvVars(content string) string {
	var result strings.Builder
	i := 0

	for i < len(content) {
		if i+1 < len(content) && content[i] == '$' && content[i+1] == '{' {
			endIdx := strings.Index(content[i:], "}")
			if endIdx == -1 {
				result.WriteByte(content[i])
				i++
				continue
			}

			endIdx += i
			varStr := content[i+2 : endIdx]

			varName := varStr
			defaultValue := ""
			if idx := strings.Index(varStr, ":-"); idx >= 0 {
				varName = varStr[:idx]
				defaultValue = varStr[idx+2:]
			}

			value := os.Getenv(varName)
			if value == "" && defaultValue != "" {
				value = defaultValue
			}

			result.WriteString(value)
			i = endIdx + 1
		} else {
			result.WriteByte(content[i])
			i++
		}
	}

	return result.String()
}

// matchWildcard 通配符匹配
func matchWildcard(pattern, s string) bool {
	if pattern == s || pattern == "*" {
		return true
	}

	patternParts := strings.Split(pattern, "*")
	if len(patternParts) == 1 {
		return pattern == s
	}

	if !strings.HasPrefix(s, patternParts[0]) {
		return false
	}

	if !strings.HasSuffix(s, patternParts[len(patternParts)-1]) {
		return false
	}

	remaining := s[len(patternParts[0]):]
	for _, part := range patternParts[1 : len(patternParts)-1] {
		idx := strings.Index(remaining, part)
		if idx == -1 {
			return false
		}
		remaining = remaining[idx+len(part):]
	}

	return true
}

// GetPrompts 获取所有已加载的提示词配置
func (m *SystemPromptManager) GetPrompts() []*SystemPromptConfig {
	return m.prompts
}

// GetPromptsForModel 获取匹配指定模型的提示词配置
func (m *SystemPromptManager) GetPromptsForModel(model string) []*SystemPromptConfig {
	result := make([]*SystemPromptConfig, 0)
	for _, prompt := range m.prompts {
		if prompt.IsMatch(model) {
			result = append(result, prompt)
		}
	}
	return result
}

// 获取主机名
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// 获取当前用户名
func getUsername() string {
	// Unix-like 系统
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	// Windows 系统
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}
	return "unknown"
}

// 获取用户主目录
func getHomeDir() string {
	if dir := os.Getenv("HOME"); dir != "" {
		return dir
	}
	if dir := os.Getenv("USERPROFILE"); dir != "" {
		return dir
	}
	return "unknown"
}

// 获取当前 Shell
func getShell() string {
	// Unix-like 系统
	if shell := os.Getenv("SHELL"); shell != "" {
		// 只返回 shell 名称
		parts := strings.Split(shell, "/")
		return parts[len(parts)-1]
	}
	// Windows 系统
	if shell := os.Getenv("COMSPEC"); shell != "" {
		parts := strings.Split(shell, "\\")
		return parts[len(parts)-1]
	}
	return "unknown"
}

// 获取系统语言
func getLang() string {
	if lang := os.Getenv("LANG"); lang != "" {
		return lang
	}
	// Windows 系统
	if lang := os.Getenv("LANGUAGE"); lang != "" {
		return lang
	}
	return "unknown"
}
