package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"llm-proxy/config"
)

var (
	configMu      sync.RWMutex
	testMode      = false
	maskSensitive = true
)

func SetTestMode(enabled bool) {
	testMode = enabled
}

func InitTestLoggers() {
	GeneralLogger, GeneralSugar = createNoOpLogger()
	SystemLogger, SystemSugar = createNoOpLogger()
	NetworkLogger, NetworkSugar = createNoOpLogger()
	ProxyLogger, ProxySugar = createNoOpLogger()
	DebugLogger, DebugSugar = createNoOpLogger()
	FileOnlyLogger, FileOnlySugar = createNoOpLogger()
}

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{20,})`),
	regexp.MustCompile(`(?i)(pk-[a-zA-Z0-9]{20,})`),
	regexp.MustCompile(`(?i)(bearer\s+)([a-zA-Z0-9\-_]{20,})`),
	regexp.MustCompile(`(?i)(api[_-]?key["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
	regexp.MustCompile(`(?i)(authorization["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
	regexp.MustCompile(`(?i)(password["\s:=]+)([a-zA-Z0-9\-_!@#$%^&*()]{8,})`),
	regexp.MustCompile(`(?i)(token["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
	regexp.MustCompile(`(?i)(secret["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
}

func InitLogger(cfg *config.Config) error {
	configMu.Lock()
	maskSensitive = cfg.Logging.ShouldMaskSensitive()
	configMu.Unlock()
	return Init(cfg)
}

func ShutdownLogger() {
	Shutdown()
}

func MaskSensitiveData(s string) string {
	if !maskSensitive {
		return s
	}
	result := s
	cfg := GetLoggingConfig()
	patterns := sensitivePatterns
	if cfg != nil && cfg.ShouldUseDetailedMasking() {
		patterns = append(patterns, getExtendedSensitivePatterns()...)
	}
	for _, pattern := range patterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			if len(match) > 8 {
				return match[:4] + "****" + match[len(match)-4:]
			}
			return "****"
		})
	}
	return result
}

func getExtendedSensitivePatterns() []*regexp.Regexp {
	return []*regexp.Regexp{
		regexp.MustCompile(`(?i)([a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,})`),
		regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`),
		regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		regexp.MustCompile(`\b(\+?\d[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`),
	}
}

func WriteRequestLogFile(cfg *config.Config, reqID string, content string) error {
	if testMode {
		return nil
	}

	maskedContent := content
	if maskSensitive {
		maskedContent = MaskSensitiveData(content)
	}

	maxSize := cfg.Logging.GetMaxLogContentSize()
	if maxSize > 0 && len(maskedContent) > maxSize {
		truncatedContent := maskedContent[:maxSize]
		truncatedContent += fmt.Sprintf("\n\n[日志内容过大,已截断。原始大小: %d 字节, 截断后: %d 字节]\n", len(maskedContent), maxSize)
		maskedContent = truncatedContent
	}

	if cfg.Logging.SeparateFiles {
		filename := filepath.Join(cfg.Logging.RequestDir, reqID+".log")
		return os.WriteFile(filename, []byte(maskedContent), 0644)
	}
	WriteRequestLog(reqID, maskedContent)
	return nil
}

func WriteErrorLogFile(cfg *config.Config, reqID string, content string) error {
	if testMode {
		return nil
	}

	maskedContent := content
	if maskSensitive {
		maskedContent = MaskSensitiveData(content)
	}

	maxSize := cfg.Logging.GetMaxLogContentSize()
	if maxSize > 0 && len(maskedContent) > maxSize {
		truncatedContent := maskedContent[:maxSize]
		truncatedContent += fmt.Sprintf("\n\n[日志内容过大,已截断。原始大小: %d 字节, 截断后: %d 字节]\n", len(maskedContent), maxSize)
		maskedContent = truncatedContent
	}

	if cfg.Logging.SeparateFiles {
		filename := filepath.Join(cfg.Logging.ErrorDir, reqID+".log")
		return os.WriteFile(filename, []byte(maskedContent), 0644)
	}
	WriteErrorLog(reqID, maskedContent)
	return nil
}

type RequestMetrics struct {
	StartTime    time.Time
	RequestID    string
	ModelAlias   string
	Attempts     int
	TotalLatency time.Duration
	BackendTimes map[string]time.Duration
}

func NewRequestMetrics(reqID, modelAlias string) *RequestMetrics {
	return &RequestMetrics{
		StartTime:    time.Now(),
		RequestID:    reqID,
		ModelAlias:   modelAlias,
		BackendTimes: make(map[string]time.Duration),
	}
}

func (m *RequestMetrics) RecordBackendTime(backend string, duration time.Duration) {
	m.BackendTimes[backend] = duration
	m.Attempts++
}

func (m *RequestMetrics) Finish(success bool, finalBackend string) {
	if !IsMetricsEnabled() || testMode {
		return
	}
	m.TotalLatency = time.Since(m.StartTime)
	status := "成功"
	if !success {
		status = "失败"
	}
	var backendDetails []string
	for backend, duration := range m.BackendTimes {
		backendDetails = append(backendDetails, fmt.Sprintf("%s=%dms", backend, duration.Milliseconds()))
	}
	LogMetrics(m.RequestID, m.ModelAlias, status, finalBackend, m.Attempts, m.TotalLatency.Milliseconds(), strings.Join(backendDetails, ", "))
}
