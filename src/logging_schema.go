package main

import (
	"regexp"

	"go.uber.org/zap"
)

// LoggerType 日志器类型枚举
type LoggerType string

const (
	GeneralLoggerType LoggerType = "general"
	SystemLoggerType  LoggerType = "system"
	NetworkLoggerType LoggerType = "network"
	ProxyLoggerType   LoggerType = "proxy"
	DebugLoggerType   LoggerType = "debug"
)

// 全局Logger实例
var (
	// GeneralLogger - 通用日志：启动、关闭、性能指标
	GeneralLogger *zap.Logger

	// SystemLogger - 系统日志：配置加载、验证、panic
	SystemLogger *zap.Logger

	// NetworkLogger - 网络日志：HTTP异常、连接错误、验证失败
	NetworkLogger *zap.Logger

	// ProxyLogger - 代理日志：请求、路由、后端、回退
	ProxyLogger *zap.Logger

	// DebugLogger - 调试日志：system_prompt注入详情（debug_mode控制）
	DebugLogger *zap.Logger

	// Sugar接口（便捷的printf风格）
	GeneralSugar *zap.SugaredLogger
	SystemSugar  *zap.SugaredLogger
	NetworkSugar *zap.SugaredLogger
	ProxySugar   *zap.SugaredLogger
	DebugSugar   *zap.SugaredLogger
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
	regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{20,})`),
	regexp.MustCompile(`(?i)(bearer\s+)([a-zA-Z0-9\-_]{20,})`),
	regexp.MustCompile(`(?i)(api[_-]?key["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
	regexp.MustCompile(`(?i)(authorization["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
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
