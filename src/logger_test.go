package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoggerInitialization(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "logger_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &Config{
		Logging: Logging{
			Level:         "debug",
			ConsoleLevel:  "info",
			BaseDir:       tempDir,
			SeparateFiles: true,
			MaxFileSizeMB: 1,
			MaxAgeDays:    7,
			MaxBackups:    3,
			Compress:      true,
			Colorize:      boolPtr(false),
			DebugMode:     true,
		},
	}

	err = InitLoggers(cfg)
	if err != nil {
		t.Fatalf("初始化Loggers失败: %v", err)
	}

	expectedDirs := []string{
		tempDir,
		filepath.Join(tempDir, "system"),
		filepath.Join(tempDir, "network"),
		filepath.Join(tempDir, "proxy"),
		filepath.Join(tempDir, "llm_debug"),
	}

	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("期望的日志目录不存在: %s", dir)
		}
	}

	if GeneralLogger == nil || GeneralSugar == nil {
		t.Error("GeneralLogger或GeneralSugar为nil")
	}
	if SystemLogger == nil || SystemSugar == nil {
		t.Error("SystemLogger或SystemSugar为nil")
	}
	if NetworkLogger == nil || NetworkSugar == nil {
		t.Error("NetworkLogger或NetworkSugar为nil")
	}
	if ProxyLogger == nil || ProxySugar == nil {
		t.Error("ProxyLogger或ProxySugar为nil")
	}
	if DebugLogger == nil || DebugSugar == nil {
		t.Error("DebugLogger或DebugSugar为nil")
	}

	ShutdownLoggers()
}

func TestLoggerRotation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "logger_rotation_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &Config{
		Logging: Logging{
			Level:         "debug",
			BaseDir:       tempDir,
			MaxFileSizeMB: 1,
			MaxAgeDays:    7,
			MaxBackups:    3,
			Colorize:      boolPtr(false),
		},
	}

	err = InitLoggers(cfg)
	if err != nil {
		t.Fatalf("初始化Loggers失败: %v", err)
	}
	defer ShutdownLoggers()

	longMessage := strings.Repeat("这是一条用于测试日志轮转的长消息，会增加日志文件大小。", 100)

	for i := 0; i < 50; i++ {
		GeneralSugar.Infof("轮转测试日志 %d: %s", i, longMessage)
	}

	time.Sleep(100 * time.Millisecond)

	logFile := filepath.Join(tempDir, "general.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("日志文件未创建")
	}

	for i := 50; i < 100; i++ {
		GeneralSugar.Infof("轮转测试日志 %d: %s", i, longMessage)
	}

	time.Sleep(100 * time.Millisecond)
}

func TestSensitiveDataMasking(t *testing.T) {
	masker := NewSensitiveDataMasker()

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "API Key脱敏",
			input:    "Authorization: Bearer sk-abc123def4567890ghijklmnopqrstuv",
			expected: "Authorization: Bearer sk-a****stuv",
		},
		{
			name:     "API Key脱敏短",
			input:    "key sk-abcdef",
			expected: "key sk-abcdef",
		},
		{
			name:     "API Key in JSON",
			input:    `{"api_key": "sk-zyxwvuts9876543210abcdef"}`,
			expected: `{"api_key": "sk-z****cdef"}`,
		},
		{
			name:     "普通文本不脱敏",
			input:    "这是一条普通日志消息",
			expected: "这是一条普通日志消息",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := masker.Mask(tc.input)
			if result != tc.expected {
				t.Errorf("脱敏结果不匹配。期望: %s, 实际: %s", tc.expected, result)
			}
		})
	}
}

func TestDebugModeSwitch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "debug_mode_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &Config{
		Logging: Logging{
			Level:     "info",
			BaseDir:   tempDir,
			DebugMode: false,
			Colorize:  boolPtr(false),
		},
	}

	err = InitLoggers(cfg)
	if err != nil {
		t.Fatalf("初始化Loggers失败: %v", err)
	}
	defer ShutdownLoggers()

	DebugSugar.Debug("这条debug消息在debug模式关闭时可能不会显示")
	DebugSugar.Info("这条info消息应该总是显示")

	debugLogFile := filepath.Join(tempDir, "llm_debug", "debug.log")
	if _, err := os.Stat(debugLogFile); err == nil {
		content, err := os.ReadFile(debugLogFile)
		if err != nil {
			t.Errorf("读取debug日志文件失败: %v", err)
		}
		contentStr := string(content)
		if len(contentStr) == 0 {
			t.Log("Debug模式关闭，debug日志文件为空（符合预期）")
		}
	}
}

func TestColorizeOption(t *testing.T) {
	testCases := []struct {
		name     string
		colorize *bool
		expected bool
	}{
		{"默认着色", nil, true},
		{"启用着色", boolPtr(true), true},
		{"禁用着色", boolPtr(false), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Logging: Logging{
					Level:    "info",
					BaseDir:  "./logs",
					Colorize: tc.colorize,
				},
			}

			result := cfg.Logging.GetColorize()
			if result != tc.expected {
				t.Errorf("Colorize配置不匹配。期望: %v, 实际: %v", tc.expected, result)
			}
		})
	}
}

func TestLogLevelParsing(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "log_level_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testLevels := []string{
		"debug", "DEBUG", "Debug",
		"info", "INFO", "Info",
		"warn", "WARN", "warning",
		"error", "ERROR", "Error",
	}

	for _, level := range testLevels {
		cfg := &Config{
			Logging: Logging{
				Level:    level,
				BaseDir:  tempDir,
				Colorize: boolPtr(false),
			},
		}

		err = InitLoggers(cfg)
		if err != nil {
			t.Errorf("初始化Loggers失败，级别 %s: %v", level, err)
			continue
		}

		GeneralSugar.Debug("Debug级别日志")
		GeneralSugar.Info("Info级别日志")
		GeneralSugar.Warn("Warn级别日志")
		GeneralSugar.Error("Error级别日志")

		ShutdownLoggers()
	}
}

func TestLoggersShutdown(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "logger_shutdown_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &Config{
		Logging: Logging{
			Level:         "info",
			BaseDir:       tempDir,
			Colorize:      boolPtr(false),
			SeparateFiles: true,
		},
	}

	err = InitLoggers(cfg)
	if err != nil {
		t.Fatalf("初始化Loggers失败: %v", err)
	}

	GeneralSugar.Info("关闭前的测试日志")
	ProxySugar.Info("Proxy关闭前的测试日志")

	err = ShutdownLoggers()
	if err != nil {
		t.Errorf("关闭Loggers失败: %v", err)
	}

	generalLogFile := filepath.Join(tempDir, "general.log")
	proxyLogFile := filepath.Join(tempDir, "proxy", "proxy.log")

	for _, logFile := range []string{generalLogFile, proxyLogFile} {
		if _, err := os.Stat(logFile); err != nil {
			t.Errorf("日志文件不存在: %s", logFile)
			continue
		}

		content, err := os.ReadFile(logFile)
		if err != nil {
			t.Errorf("读取日志文件失败 %s: %v", logFile, err)
			continue
		}

		if len(content) == 0 {
			t.Errorf("日志文件为空: %s", logFile)
		}
	}
}
