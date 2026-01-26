package logging

import (
	"llm-proxy/infrastructure/config"
)

// LogCategory 定义日志分类
type LogCategory string

const (
	// LogCategoryGeneral 普通日志（系统运行、状态变更等）
	LogCategoryGeneral LogCategory = "general"
	// LogCategoryRequest 请求日志（完整请求/响应数据）
	LogCategoryRequest LogCategory = "request"
	// LogCategoryError 错误日志（错误详情、堆栈等）
	LogCategoryError LogCategory = "error"
	// LogCategoryDebug 调试日志（详细调试信息）
	LogCategoryDebug LogCategory = "debug"
	// LogCategoryNetwork 网络日志（网络请求/响应）
	LogCategoryNetwork LogCategory = "network"
	// LogCategoryRequestBody 请求体日志（完整的HTTP请求/响应体）
	LogCategoryRequestBody LogCategory = "request_body"
)

// LogTarget 定义日志输出目标
type LogTarget string

const (
	// LogTargetConsole 控制台输出
	LogTargetConsole LogTarget = "console"
	// LogTargetFile 文件输出
	LogTargetFile LogTarget = "file"
	// LogTargetBoth 同时输出到控制台和文件
	LogTargetBoth LogTarget = "both"
	// LogTargetNone 不输出
	LogTargetNone LogTarget = "none"
)

// LogLevelConfig 定义每个输出目标的日志级别
type LogLevelConfig struct {
	Console string `yaml:"console"` // 控制台日志级别
	File    string `yaml:"file"`    // 文件日志级别
}

// TargetConfig 定义每个日志分类的目标配置
type TargetConfig struct {
	Target   string         `yaml:"target"`   // 输出目标: console/file/both/none
	Levels   LogLevelConfig `yaml:"levels"`   // 日志级别配置
	Path     string         `yaml:"path"`     // 文件路径（仅文件目标）
	MaxSize  int            `yaml:"max_size"` // 最大文件大小(MB)
	MaxAge   int            `yaml:"max_age"`  // 最大保留天数
	Compress bool           `yaml:"compress"` // 是否压缩
}

// MultiTargetConfig 多目标日志配置
type MultiTargetConfig struct {
	// 全局控制台配置
	Console ConsoleConfig `yaml:"console"`

	// 全局文件配置
	File FileConfig `yaml:"file"`

	// 各分类的独立配置
	Categories map[string]TargetConfig `yaml:"categories"`
}

// ConsoleConfig 控制台输出配置
type ConsoleConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Level    string `yaml:"level"`
	Style    string `yaml:"style"`  // compact/verbose
	Format   string `yaml:"format"` // markdown/plain
	Colorize bool   `yaml:"colorize"`
}

// FileConfig 文件输出配置
type FileConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Level      string `yaml:"level"`
	BaseDir    string `yaml:"base_dir"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxAgeDays int    `yaml:"max_age_days"`
	MaxBackups int    `yaml:"max_backups"`
	Compress   bool   `yaml:"compress"`
	Format     string `yaml:"format"` // json/text
}

// DefaultMultiTargetConfig 返回默认的多目标配置
func DefaultMultiTargetConfig() *MultiTargetConfig {
	return &MultiTargetConfig{
		Console: ConsoleConfig{
			Enabled:  true,
			Level:    "info",
			Style:    "compact",
			Format:   "markdown",
			Colorize: true,
		},
		File: FileConfig{
			Enabled:    true,
			Level:      "debug",
			BaseDir:    "./logs",
			MaxSizeMB:  100,
			MaxAgeDays: 7,
			MaxBackups: 10,
			Compress:   true,
			Format:     "json",
		},
		Categories: map[string]TargetConfig{
			string(LogCategoryGeneral): {
				Target: string(LogTargetBoth),
				Levels: LogLevelConfig{
					Console: "info",
					File:    "debug",
				},
			},
			string(LogCategoryRequest): {
				Target: string(LogTargetFile),
				Levels: LogLevelConfig{
					Console: "warn", // 请求日志不在控制台显示，除非是警告
					File:    "debug",
				},
				Path:     "requests/requests.log",
				MaxSize:  200,
				MaxAge:   14,
				Compress: true,
			},
			string(LogCategoryError): {
				Target: string(LogTargetBoth),
				Levels: LogLevelConfig{
					Console: "error",
					File:    "error",
				},
				Path:     "errors/errors.log",
				MaxSize:  100,
				MaxAge:   30,
				Compress: true,
			},
			string(LogCategoryDebug): {
				Target: string(LogTargetFile),
				Levels: LogLevelConfig{
					Console: "debug",
					File:    "debug",
				},
				Path: "debug/debug.log",
			},
			string(LogCategoryNetwork): {
				Target: string(LogTargetFile),
				Levels: LogLevelConfig{
					Console: "info",
					File:    "debug",
				},
				Path: "network/network.log",
			},
			string(LogCategoryRequestBody): {
				// 请求体日志由 body.go 的 InitRequestBodyLogger 处理
				// 此处配置为仅文件输出，控制台级别为 none
				Target: string(LogTargetFile),
				Levels: LogLevelConfig{
					Console: "none", // 请求体不输出到控制台
					File:    "debug",
				},
				Path: "request_body/{date}/{time}_{req_id}_{type}.log",
			},
		},
	}
}

// ConvertFromLegacyConfig 从旧配置转换到新配置
func ConvertFromLegacyConfig(cfg *config.Config) *MultiTargetConfig {
	newCfg := DefaultMultiTargetConfig()

	// 转换控制台配置
	newCfg.Console.Enabled = cfg.Logging.GetColorize()
	newCfg.Console.Level = cfg.Logging.GetConsoleLevel()
	newCfg.Console.Style = cfg.Logging.GetConsoleStyle()
	newCfg.Console.Format = cfg.Logging.GetConsoleFormat()
	newCfg.Console.Colorize = cfg.Logging.GetColorize()

	// 转换文件配置
	newCfg.File.Enabled = true
	newCfg.File.Level = cfg.Logging.GetLevel()
	newCfg.File.BaseDir = cfg.Logging.GetBaseDir()
	newCfg.File.MaxSizeMB = cfg.Logging.GetMaxFileSizeMB()
	newCfg.File.MaxAgeDays = cfg.Logging.GetMaxAgeDays()
	newCfg.File.MaxBackups = cfg.Logging.GetMaxBackups()
	newCfg.File.Compress = cfg.Logging.Compress
	newCfg.File.Format = cfg.Logging.GetFormat()

	// 如果使用旧的分文件模式，转换为新配置
	if cfg.Logging.SeparateFiles {
		// 必须先获取map元素的副本，修改后再存回去
		generalCfg := newCfg.Categories[string(LogCategoryGeneral)]
		generalCfg.Path = "general.log"
		newCfg.Categories[string(LogCategoryGeneral)] = generalCfg

		requestCfg := newCfg.Categories[string(LogCategoryRequest)]
		requestCfg.Path = "requests/requests.log"
		requestCfg.Target = string(LogTargetFile)
		newCfg.Categories[string(LogCategoryRequest)] = requestCfg

		errorCfg := newCfg.Categories[string(LogCategoryError)]
		errorCfg.Path = "errors/errors.log"
		errorCfg.Target = string(LogTargetFile)
		newCfg.Categories[string(LogCategoryError)] = errorCfg

		networkCfg := newCfg.Categories[string(LogCategoryNetwork)]
		networkCfg.Path = "network/network.log"
		newCfg.Categories[string(LogCategoryNetwork)] = networkCfg

		debugCfg := newCfg.Categories[string(LogCategoryDebug)]
		debugCfg.Path = "debug/debug.log"
		newCfg.Categories[string(LogCategoryDebug)] = debugCfg
	}

	return newCfg
}

// IsValidTarget 检查目标配置是否有效
func (t *TargetConfig) IsValidTarget() bool {
	switch LogTarget(t.Target) {
	case LogTargetConsole, LogTargetFile, LogTargetBoth, LogTargetNone:
		return true
	default:
		return false
	}
}

// ShouldLogToConsole 判断是否应该输出到控制台
func (t *TargetConfig) ShouldLogToConsole() bool {
	switch LogTarget(t.Target) {
	case LogTargetConsole, LogTargetBoth:
		return true
	case LogTargetFile, LogTargetNone:
		return false
	default:
		return false
	}
}

// ShouldLogToFile 判断是否应该输出到文件
func (t *TargetConfig) ShouldLogToFile() bool {
	switch LogTarget(t.Target) {
	case LogTargetFile, LogTargetBoth:
		return true
	case LogTargetConsole, LogTargetNone:
		return false
	default:
		return true // 默认输出到文件
	}
}
