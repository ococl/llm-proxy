package main

import (
	"regexp"
	"time"

	"go.uber.org/zap"
)

type LoggerType string

const (
	GeneralLoggerType LoggerType = "general"
	SystemLoggerType  LoggerType = "system"
	NetworkLoggerType LoggerType = "network"
	ProxyLoggerType   LoggerType = "proxy"
	DebugLoggerType   LoggerType = "debug"
)

var (
	GeneralLogger *zap.Logger
	SystemLogger  *zap.Logger
	NetworkLogger *zap.Logger
	ProxyLogger   *zap.Logger
	DebugLogger   *zap.Logger

	GeneralSugar *zap.SugaredLogger
	SystemSugar  *zap.SugaredLogger
	NetworkSugar *zap.SugaredLogger
	ProxySugar   *zap.SugaredLogger
	DebugSugar   *zap.SugaredLogger

	flushTicker *time.Ticker
	flushDone   chan struct{}
)

// LevelPriority 日志级别优先级映射
var LevelPriority = map[string]int{
	"debug": 0,
	"info":  1,
	"warn":  2,
	"error": 3,
}

// SensitivePatterns 敏感信息正则模式
var SensitivePatterns = []*regexp.Regexp{
	// API Keys
	regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{20,})`),
	regexp.MustCompile(`(?i)(pk-[a-zA-Z0-9]{20,})`),
	// Authorization headers
	regexp.MustCompile(`(?i)(bearer\s+)([a-zA-Z0-9\-_]{20,})`),
	// Generic API keys
	regexp.MustCompile(`(?i)(api[_-]?key["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
	regexp.MustCompile(`(?i)(authorization["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
	// Passwords in URLs or configs
	regexp.MustCompile(`(?i)(password["\s:=]+)([a-zA-Z0-9\-_!@#$%^&*()]{8,})`),
	// Tokens
	regexp.MustCompile(`(?i)(token["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
	// Secret keys
	regexp.MustCompile(`(?i)(secret["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
}

// SensitiveDataMasker 敏感信息脱敏处理器
type SensitiveDataMasker struct{}

// NewSensitiveDataMasker 创建脱敏处理器
func NewSensitiveDataMasker() *SensitiveDataMasker {
	return &SensitiveDataMasker{}
}

// Mask 对敏感信息进行脱敏
func (m *SensitiveDataMasker) Mask(data string) string {
	result := data
	for _, pattern := range SensitivePatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			if len(match) > 8 {
				return match[:4] + "****" + match[len(match)-4:]
			}
			return "****"
		})
	}
	return result
}
