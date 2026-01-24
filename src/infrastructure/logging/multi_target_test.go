package logging

import (
	"testing"

	"llm-proxy/infrastructure/config"
)

func boolPtr(b bool) *bool {
	return &b
}

func TestDefaultMultiTargetConfig(t *testing.T) {
	cfg := DefaultMultiTargetConfig()

	// 验证控制台配置
	if !cfg.Console.Enabled {
		t.Error("控制台应该启用")
	}
	if cfg.Console.Level != "info" {
		t.Errorf("期望控制台级别为 info, 实际为 %s", cfg.Console.Level)
	}

	// 验证文件配置
	if !cfg.File.Enabled {
		t.Error("文件输出应该启用")
	}
	if cfg.File.Level != "debug" {
		t.Errorf("期望文件级别为 debug, 实际为 %s", cfg.File.Level)
	}

	// 验证分类配置
	if len(cfg.Categories) != 5 {
		t.Errorf("期望 5 个分类, 实际为 %d", len(cfg.Categories))
	}

	// 验证各分类配置
	categories := []string{string(LogCategoryGeneral), string(LogCategoryRequest), string(LogCategoryError), string(LogCategoryDebug), string(LogCategoryNetwork)}
	for _, cat := range categories {
		if _, ok := cfg.Categories[cat]; !ok {
			t.Errorf("缺少分类配置: %s", cat)
		}
	}

	// 验证请求日志只在文件输出
	if cfg.Categories[string(LogCategoryRequest)].Target != string(LogTargetFile) {
		t.Error("请求日志应该只在文件输出")
	}
}

func TestConvertFromLegacyConfig(t *testing.T) {
	// 测试从旧配置转换
	legacyCfg := &config.Config{
		Logging: config.Logging{
			Level:         "debug",
			ConsoleLevel:  "info",
			BaseDir:       "./logs",
			MaxFileSizeMB: 200,
			MaxAgeDays:    14,
			MaxBackups:    5,
			Compress:      true,
			Format:        "json",
			Colorize:      boolPtr(true),
			ConsoleStyle:  "compact",
			ConsoleFormat: "markdown",
			SeparateFiles: true,
			MaskSensitive: boolPtr(true),
		},
	}

	newCfg := ConvertFromLegacyConfig(legacyCfg)

	// 验证控制台配置
	if newCfg.Console.Level != "info" {
		t.Errorf("期望控制台级别为 info, 实际为 %s", newCfg.Console.Level)
	}
	if newCfg.Console.Colorize != true {
		t.Error("颜色应该启用")
	}

	// 验证文件配置
	if newCfg.File.Level != "debug" {
		t.Errorf("期望文件级别为 debug, 实际为 %s", newCfg.File.Level)
	}
	if newCfg.File.BaseDir != "./logs" {
		t.Errorf("期望 base_dir 为 ./logs, 实际为 %s", newCfg.File.BaseDir)
	}

	// 验证分文件模式
	if newCfg.Categories[string(LogCategoryGeneral)].Path != "general.log" {
		t.Error("通用日志路径配置错误")
	}
	if newCfg.Categories[string(LogCategoryRequest)].Target != string(LogTargetFile) {
		t.Error("请求日志应该只在文件输出")
	}
}

func TestTargetConfig_IsValidTarget(t *testing.T) {
	tests := []struct {
		target   string
		expected bool
	}{
		{"console", true},
		{"file", true},
		{"both", true},
		{"none", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		cfg := &TargetConfig{Target: tt.target}
		if cfg.IsValidTarget() != tt.expected {
			t.Errorf("Target %s 期望有效性为 %v, 实际为 %v", tt.target, tt.expected, cfg.IsValidTarget())
		}
	}
}

func TestTargetConfig_ShouldLogToConsole(t *testing.T) {
	tests := []struct {
		target   string
		expected bool
	}{
		{"console", true},
		{"both", true},
		{"file", false},
		{"none", false},
	}

	for _, tt := range tests {
		cfg := &TargetConfig{Target: tt.target}
		if cfg.ShouldLogToConsole() != tt.expected {
			t.Errorf("Target %s 期望控制台输出为 %v, 实际为 %v", tt.target, tt.expected, cfg.ShouldLogToConsole())
		}
	}
}

func TestTargetConfig_ShouldLogToFile(t *testing.T) {
	tests := []struct {
		target   string
		expected bool
	}{
		{"file", true},
		{"both", true},
		{"console", false},
		{"none", false},
	}

	for _, tt := range tests {
		cfg := &TargetConfig{Target: tt.target}
		if cfg.ShouldLogToFile() != tt.expected {
			t.Errorf("Target %s 期望文件输出为 %v, 实际为 %v", tt.target, tt.expected, cfg.ShouldLogToFile())
		}
	}
}

func TestLogCategory_Constants(t *testing.T) {
	// 验证日志分类常量
	if LogCategoryGeneral != "general" {
		t.Error("LogCategoryGeneral 应该为 'general'")
	}
	if LogCategoryRequest != "request" {
		t.Error("LogCategoryRequest 应该为 'request'")
	}
	if LogCategoryError != "error" {
		t.Error("LogCategoryError 应该为 'error'")
	}
	if LogCategoryDebug != "debug" {
		t.Error("LogCategoryDebug 应该为 'debug'")
	}
	if LogCategoryNetwork != "network" {
		t.Error("LogCategoryNetwork 应该为 'network'")
	}
}

func TestLogTarget_Constants(t *testing.T) {
	// 验证日志目标常量
	if LogTargetConsole != "console" {
		t.Error("LogTargetConsole 应该为 'console'")
	}
	if LogTargetFile != "file" {
		t.Error("LogTargetFile 应该为 'file'")
	}
	if LogTargetBoth != "both" {
		t.Error("LogTargetBoth 应该为 'both'")
	}
	if LogTargetNone != "none" {
		t.Error("LogTargetNone 应该为 'none'")
	}
}
