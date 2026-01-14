package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	generalLogger   *os.File
	logMu           sync.Mutex
	configMu        sync.RWMutex
	logLevel        = "info"
	consoleLogLevel = "info"
	testMode        = false
	currentLogDate  string
	currentLogSize  int64
	maxFileSizeMB   = 100
	maskSensitive   = true
	enableMetrics   = false
	separateFiles   = false
	loggingConfig   *Logging

	asyncLogger *AsyncLogger
)

type LogTarget int

const (
	LogTargetBoth LogTarget = iota
	LogTargetFile
	LogTargetConsole
)

type LogEntry struct {
	Level    string
	Target   LogTarget
	Message  string
	IsMetric bool
}

type AsyncLogger struct {
	entries chan LogEntry
	done    chan struct{}
	wg      sync.WaitGroup
	enabled bool
}

func NewAsyncLogger(bufferSize int) *AsyncLogger {
	if bufferSize <= 0 {
		bufferSize = 10000
	}
	return &AsyncLogger{
		entries: make(chan LogEntry, bufferSize),
		done:    make(chan struct{}),
		enabled: true,
	}
}

func (al *AsyncLogger) Start() {
	al.wg.Add(1)
	go al.worker()
}

func (al *AsyncLogger) worker() {
	for {
		select {
		case entry := <-al.entries:
			if al.enabled {
				logMessage(entry.Level, entry.Target, entry.Message)
			}
		case <-al.done:
			return
		}
	}
}

func (al *AsyncLogger) Stop() {
	al.enabled = false
	close(al.done)
	al.wg.Wait()

	for {
		select {
		case entry := <-al.entries:
			logMessage(entry.Level, entry.Target, entry.Message)
		default:
			return
		}
	}
}

func (al *AsyncLogger) Log(level string, target LogTarget, format string, args ...interface{}) {
	if !al.enabled {
		return
	}
	entry := LogEntry{
		Level:   level,
		Target:  target,
		Message: fmt.Sprintf(format, args...),
	}
	select {
	case al.entries <- entry:
	default:
		if loggingConfig != nil && loggingConfig.ShouldDropOnFull() {
			return
		}
		al.entries <- entry
	}
}

func SetTestMode(enabled bool) {
	testMode = enabled
}

var levelPriority = map[string]int{
	"debug": 0,
	"info":  1,
	"warn":  2,
	"error": 3,
}

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{20,})`),
	regexp.MustCompile(`(?i)(bearer\s+)([a-zA-Z0-9\-_]{20,})`),
	regexp.MustCompile(`(?i)(api[_-]?key["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
	regexp.MustCompile(`(?i)(authorization["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
}

func InitLogger(cfg *Config) error {
	configMu.Lock()
	loggingConfig = &cfg.Logging
	logLevel = strings.ToLower(cfg.Logging.Level)
	if logLevel == "" {
		logLevel = "info"
	}
	consoleLogLevel = strings.ToLower(cfg.Logging.ConsoleLevel)
	if consoleLogLevel == "" {
		consoleLogLevel = "info"
	}
	maskSensitive = cfg.Logging.ShouldMaskSensitive()
	enableMetrics = cfg.Logging.EnableMetrics
	separateFiles = cfg.Logging.SeparateFiles
	if cfg.Logging.MaxFileSizeMB > 0 {
		maxFileSizeMB = cfg.Logging.MaxFileSizeMB
	}

	if cfg.Logging.ShouldAsync() {
		asyncLogger = NewAsyncLogger(cfg.Logging.GetBufferSize())
		asyncLogger.Start()
	}
	configMu.Unlock()

	if separateFiles {
		if err := os.MkdirAll(cfg.Logging.RequestDir, 0755); err != nil {
			return err
		}
		if err := os.MkdirAll(cfg.Logging.ErrorDir, 0755); err != nil {
			return err
		}
	}

	dir := filepath.Dir(cfg.Logging.GeneralFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return rotateLogIfNeeded(cfg.Logging.GeneralFile)
}

func ShutdownLogger() {
	if asyncLogger != nil {
		asyncLogger.Stop()
	}
}

func rotateLogIfNeeded(basePath string) error {
	today := time.Now().Format("2006-01-02")

	if generalLogger != nil {
		if currentLogDate == today && currentLogSize < int64(maxFileSizeMB)*1024*1024 {
			return nil
		}
		generalLogger.Close()
	}

	logPath := getRotatedLogPath(basePath, today)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	stat, _ := f.Stat()
	currentLogSize = stat.Size()
	currentLogDate = today
	generalLogger = f
	return nil
}

func getRotatedLogPath(basePath, date string) string {
	ext := filepath.Ext(basePath)
	base := strings.TrimSuffix(basePath, ext)
	return fmt.Sprintf("%s_%s%s", base, date, ext)
}

func MaskSensitiveData(s string) string {
	if !maskSensitive {
		return s
	}
	result := s
	for _, pattern := range sensitivePatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			if len(match) > 8 {
				return match[:4] + "****" + match[len(match)-4:]
			}
			return "****"
		})
	}
	return result
}

func logMessage(level string, target LogTarget, message string) {
	if testMode {
		return
	}

	configMu.RLock()
	currentLogLevel := logLevel
	currentConsoleLevel := consoleLogLevel
	currentMaskSensitive := maskSensitive
	currentLoggingConfig := loggingConfig
	configMu.RUnlock()

	levelLower := strings.ToLower(level)
	filePriority := levelPriority[levelLower] >= levelPriority[currentLogLevel]
	consolePriority := levelPriority[levelLower] >= levelPriority[currentConsoleLevel]

	shouldLogFile := (target == LogTargetBoth || target == LogTargetFile) && filePriority
	shouldLogConsole := (target == LogTargetBoth || target == LogTargetConsole) && consolePriority

	if !shouldLogFile && !shouldLogConsole {
		return
	}

	logMu.Lock()
	defer logMu.Unlock()

	msg := message

	if currentMaskSensitive {
		msg = MaskSensitiveData(msg)
	}

	now := time.Now()
	fileTime := now.Format("2006-01-02 15:04:05")
	consoleTime := now.Format("15:04:05")
	fileLine := fmt.Sprintf("[%s] [%s] %s\n", fileTime, strings.ToUpper(level), msg)

	if shouldLogConsole {
		if shouldUseColor() {
			consoleLine := fmt.Sprintf("%s  %s  %s\n", colorTimeStr(consoleTime), colorLevel(level), highlightRequestID(msg))
			fmt.Print(consoleLine)
		} else {
			fmt.Print(fileLine)
		}
	}

	if shouldLogFile && generalLogger != nil {
		if currentLoggingConfig != nil {
			rotateLogIfNeeded(currentLoggingConfig.GeneralFile)
		}
		generalLogger.WriteString(fileLine)
		currentLogSize += int64(len(fileLine))
	}
}

func logInternal(level string, target LogTarget, format string, args ...interface{}) {
	if asyncLogger != nil && asyncLogger.enabled {
		asyncLogger.Log(level, target, format, args...)
		return
	}
	logMessage(level, target, fmt.Sprintf(format, args...))
}

func LogGeneral(level, format string, args ...interface{}) {
	logInternal(level, LogTargetBoth, format, args...)
}

func LogFile(level, format string, args ...interface{}) {
	logInternal(level, LogTargetFile, format, args...)
}

func LogConsole(level, format string, args ...interface{}) {
	logInternal(level, LogTargetConsole, format, args...)
}

func LogRequest(cfg *Config, reqID string, content string) error {
	if testMode {
		return nil
	}

	maskedContent := content
	if maskSensitive {
		maskedContent = MaskSensitiveData(content)
	}

	if separateFiles {
		filename := filepath.Join(cfg.Logging.RequestDir, reqID+".log")
		return os.WriteFile(filename, []byte(maskedContent), 0644)
	}

	LogFile("INFO", "[请求 %s]\n%s", reqID, maskedContent)
	return nil
}

func LogError(cfg *Config, reqID string, content string) error {
	if testMode {
		return nil
	}

	maskedContent := content
	if maskSensitive {
		maskedContent = MaskSensitiveData(content)
	}

	if separateFiles {
		filename := filepath.Join(cfg.Logging.ErrorDir, reqID+".log")
		return os.WriteFile(filename, []byte(maskedContent), 0644)
	}

	LogFile("ERROR", "[错误 %s]\n%s", reqID, maskedContent)
	return nil
}

func WriteRequestLog(cfg *Config, reqID string, content string) error {
	return LogRequest(cfg, reqID, content)
}

func WriteErrorLog(cfg *Config, reqID string, content string) error {
	return LogError(cfg, reqID, content)
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
	if !enableMetrics || testMode {
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

	LogGeneral("INFO", "[性能指标] 请求=%s 模型=%s 状态=%s 后端=%s 尝试次数=%d 总耗时=%dms 后端耗时=[%s]",
		m.RequestID, m.ModelAlias, status, finalBackend, m.Attempts, m.TotalLatency.Milliseconds(),
		strings.Join(backendDetails, ", "))
}
