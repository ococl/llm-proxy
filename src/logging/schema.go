package logging

import (
	"regexp"
	"time"

	"go.uber.org/zap"
)

var (
	GeneralLogger  *zap.Logger
	SystemLogger   *zap.Logger
	NetworkLogger  *zap.Logger
	ProxyLogger    *zap.Logger
	DebugLogger    *zap.Logger
	FileOnlyLogger *zap.Logger
	RequestLogger  *zap.Logger
	ErrorLogger    *zap.Logger

	GeneralSugar  *zap.SugaredLogger
	SystemSugar   *zap.SugaredLogger
	NetworkSugar  *zap.SugaredLogger
	ProxySugar    *zap.SugaredLogger
	DebugSugar    *zap.SugaredLogger
	FileOnlySugar *zap.SugaredLogger

	flushTicker *time.Ticker
	flushDone   chan struct{}
)

var LevelPriority = map[string]int{
	"debug": 0,
	"info":  1,
	"warn":  2,
	"error": 3,
}

var SensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{20,})`),
	regexp.MustCompile(`(?i)(pk-[a-zA-Z0-9]{20,})`),
	regexp.MustCompile(`(?i)(bearer\s+)([a-zA-Z0-9\-_]{20,})`),
	regexp.MustCompile(`(?i)(api[_-]?key["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
	regexp.MustCompile(`(?i)(authorization["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
	regexp.MustCompile(`(?i)(password["\s:=]+)([a-zA-Z0-9\-_!@#$%^&*()]{8,})`),
	regexp.MustCompile(`(?i)(token["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
	regexp.MustCompile(`(?i)(secret["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
}

type SensitiveDataMasker struct{}

func NewSensitiveDataMasker() *SensitiveDataMasker {
	return &SensitiveDataMasker{}
}

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
