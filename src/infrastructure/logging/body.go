package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"llm-proxy/infrastructure/config"

	"gopkg.in/natefinch/lumberjack.v2"
)

// BodyLogType 请求体日志类型
type BodyLogType string

const (
	// BodyLogTypeClientRequest 客户端请求
	BodyLogTypeClientRequest BodyLogType = "client_request"
	// BodyLogTypeClientResponse 客户端响应
	BodyLogTypeClientResponse BodyLogType = "client_response"
	// BodyLogTypeUpstreamRequest 上游请求
	BodyLogTypeUpstreamRequest BodyLogType = "upstream_request"
	// BodyLogTypeUpstreamResponse 上游响应
	BodyLogTypeUpstreamResponse BodyLogType = "upstream_response"
)

// RequestBodyLogger 请求体日志写入器
type RequestBodyLogger struct {
	writer   *lumberjack.Logger
	config   *config.RequestBodyConfig
	mu       sync.Mutex
	baseDir  string
	testMode bool
}

// 全局实例
var (
	bodyLogger     *RequestBodyLogger
	bodyLoggerMu   sync.RWMutex
	bodyLoggerInit bool
)

// InitRequestBodyLogger 初始化请求体日志写入器
func InitRequestBodyLogger(cfg *config.Config) error {
	bodyLoggerMu.Lock()
	defer bodyLoggerMu.Unlock()

	bodyLoggerInit = true

	if !cfg.Logging.RequestBody.Enabled {
		bodyLogger = &RequestBodyLogger{
			config:   &cfg.Logging.RequestBody,
			testMode: false,
		}
		return nil
	}

	baseDir := cfg.Logging.RequestBody.GetBaseDir()
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("创建请求体日志目录失败: %w", err)
	}

	writer := &lumberjack.Logger{
		Filename:   filepath.Join(baseDir, "request_body.log"),
		MaxSize:    cfg.Logging.RequestBody.GetMaxSizeMB(),
		MaxAge:     cfg.Logging.RequestBody.GetMaxAgeDays(),
		MaxBackups: cfg.Logging.RequestBody.GetMaxBackups(),
		Compress:   cfg.Logging.RequestBody.ShouldCompress(),
	}

	bodyLogger = &RequestBodyLogger{
		writer:  writer,
		config:  &cfg.Logging.RequestBody,
		baseDir: baseDir,
	}

	return nil
}

// InitTestRequestBodyLogger 初始化测试用的请求体日志器
func InitTestRequestBodyLogger() {
	bodyLoggerMu.Lock()
	defer bodyLoggerMu.Unlock()

	bodyLogger = &RequestBodyLogger{
		config:   &config.RequestBodyConfig{},
		testMode: true,
	}
	bodyLoggerInit = true
}

// ShutdownRequestBodyLogger 关闭请求体日志器
func ShutdownRequestBodyLogger() error {
	bodyLoggerMu.Lock()
	defer bodyLoggerMu.Unlock()

	if bodyLogger != nil && bodyLogger.writer != nil {
		err := bodyLogger.writer.Close()
		bodyLogger = nil
		bodyLoggerInit = false
		return err
	}
	return nil
}

// GetRequestBodyLogger 获取请求体日志器实例
func GetRequestBodyLogger() *RequestBodyLogger {
	bodyLoggerMu.RLock()
	defer bodyLoggerMu.RUnlock()

	if !bodyLoggerInit {
		return nil
	}
	return bodyLogger
}

// Write 写入请求体日志
func (l *RequestBodyLogger) Write(reqID string, logType BodyLogType, httpReq *http.Request, body []byte) error {
	if l.testMode {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 生成文件名: {date}/{time}_{req_id}_{type}.httpdump
	now := time.Now()
	dateDir := now.Format("2006-01-02")
	timePrefix := now.Format("150405")

	dir := filepath.Join(l.baseDir, dateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建日期目录失败: %w", err)
	}

	filename := fmt.Sprintf("%s_%s_%s.httpdump", timePrefix, reqID, string(logType))
	filePath := filepath.Join(dir, filename)

	// 构建 HTTP Dump 内容
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s %s %s\r\n", httpReq.Method, httpReq.URL.Path, httpReq.Proto))
	sb.WriteString(fmt.Sprintf("Host: %s\r\n", httpReq.Host))

	// 写入请求头
	for key, values := range httpReq.Header {
		for _, value := range values {
			sb.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
		}
	}

	// 空行分隔符
	sb.WriteString("\r\n")

	// 写入请求体
	if len(body) > 0 {
		sb.Write(body)
		if !bytes.HasSuffix(body, []byte("\n")) {
			sb.WriteString("\n")
		}
	}

	// 写入文件
	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

// WriteResponse 写入响应体日志
func (l *RequestBodyLogger) WriteResponse(reqID string, logType BodyLogType, statusCode int, header http.Header, body []byte) error {
	if l.testMode {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 生成文件名: {date}/{time}_{req_id}_{type}.httpdump
	now := time.Now()
	dateDir := now.Format("2006-01-02")
	timePrefix := now.Format("150405")

	dir := filepath.Join(l.baseDir, dateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建日期目录失败: %w", err)
	}

	filename := fmt.Sprintf("%s_%s_%s.httpdump", timePrefix, reqID, string(logType))
	filePath := filepath.Join(dir, filename)

	// 构建 HTTP Dump 内容
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, getStatusText(statusCode)))

	// 写入响应头
	for key, values := range header {
		for _, value := range values {
			if isHopByHopHeader(key) {
				continue
			}
			sb.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
		}
	}

	// 空行分隔符
	sb.WriteString("\r\n")

	// 写入响应体
	if len(body) > 0 {
		sb.Write(body)
		if !bytes.HasSuffix(body, []byte("\n")) {
			sb.WriteString("\n")
		}
	}

	// 写入文件
	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

// WriteFromMap 从 map 数据写入请求体日志（用于客户端请求）
func (l *RequestBodyLogger) WriteFromMap(reqID string, logType BodyLogType, method, path, protocol string, headers map[string][]string, body map[string]interface{}) error {
	if l.testMode {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	dateDir := now.Format("2006-01-02")
	timePrefix := now.Format("150405")

	dir := filepath.Join(l.baseDir, dateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建日期目录失败: %w", err)
	}

	filename := fmt.Sprintf("%s_%s_%s.httpdump", timePrefix, reqID, string(logType))
	filePath := filepath.Join(dir, filename)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s %s %s\r\n", method, path, protocol))

	// 写入请求头
	for key, values := range headers {
		for _, value := range values {
			sb.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
		}
	}

	sb.WriteString("\r\n")

	// 写入请求体
	if body != nil && l.config.ShouldIncludeBody() {
		if bodyJSON, err := json.MarshalIndent(body, "", "  "); err == nil {
			sb.Write(bodyJSON)
			sb.WriteString("\n")
		}
	}

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

// WriteResponseFromMap 从 map 数据写入响应体日志
func (l *RequestBodyLogger) WriteResponseFromMap(reqID string, logType BodyLogType, statusCode int, headers map[string][]string, body interface{}) error {
	if l.testMode {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	dateDir := now.Format("2006-01-02")
	timePrefix := now.Format("150405")

	dir := filepath.Join(l.baseDir, dateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建日期目录失败: %w", err)
	}

	filename := fmt.Sprintf("%s_%s_%s.httpdump", timePrefix, reqID, string(logType))
	filePath := filepath.Join(dir, filename)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, getStatusText(statusCode)))

	// 写入响应头
	for key, values := range headers {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			sb.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
		}
	}

	sb.WriteString("\r\n")

	// 写入响应体
	if body != nil && l.config.ShouldIncludeBody() {
		if bodyJSON, err := json.MarshalIndent(body, "", "  "); err == nil {
			sb.Write(bodyJSON)
			sb.WriteString("\n")
		}
	}

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

// WriteUpstreamResponse 写入上游响应体日志（处理 io.Reader）
func (l *RequestBodyLogger) WriteUpstreamResponse(reqID string, statusCode int, header http.Header, body io.Reader) error {
	if l.testMode {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	dateDir := now.Format("2006-01-02")
	timePrefix := now.Format("150405")

	dir := filepath.Join(l.baseDir, dateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建日期目录失败: %w", err)
	}

	filename := fmt.Sprintf("%s_%s_%s.httpdump", timePrefix, reqID, string(BodyLogTypeUpstreamResponse))
	filePath := filepath.Join(dir, filename)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, getStatusText(statusCode)))

	// 写入响应头
	for key, values := range header {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			sb.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
		}
	}

	sb.WriteString("\r\n")

	// 读取并写入响应体
	if body != nil && l.config.ShouldIncludeBody() {
		bodyBytes, err := io.ReadAll(body)
		if err != nil {
			sb.WriteString(fmt.Sprintf("[读取响应体失败: %v]\n", err))
		} else {
			sb.Write(bodyBytes)
			if !bytes.HasSuffix(bodyBytes, []byte("\n")) {
				sb.WriteString("\n")
			}
		}
	}

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

// 辅助函数：获取 HTTP 状态文本
func getStatusText(statusCode int) string {
	switch statusCode {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 204:
		return "No Content"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 405:
		return "Method Not Allowed"
	case 429:
		return "Too Many Requests"
	case 500:
		return "Internal Server Error"
	case 502:
		return "Bad Gateway"
	case 503:
		return "Service Unavailable"
	case 504:
		return "Gateway Timeout"
	default:
		return "Unknown"
	}
}

// 辅助函数：判断是否为 hop-by-hop 头
func isHopByHopHeader(key string) bool {
	hopByHopHeaders := map[string]bool{
		"connection":          true,
		"keep-alive":          true,
		"proxy-authenticate":  true,
		"proxy-authorization": true,
		"te":                  true,
		"trailer":             true,
		"transfer-encoding":   true,
		"upgrade":             true,
	}
	return hopByHopHeaders[strings.ToLower(key)]
}

// LogRequestBody 便捷函数：记录请求体
func LogRequestBody(reqID string, logType BodyLogType, method, path, protocol string, headers map[string][]string, body map[string]interface{}) {
	logger := GetRequestBodyLogger()
	if logger == nil {
		return
	}
	_ = logger.WriteFromMap(reqID, logType, method, path, protocol, headers, body)
}

// LogResponseBody 便捷函数：记录响应体
func LogResponseBody(reqID string, logType BodyLogType, statusCode int, headers map[string][]string, body interface{}) {
	logger := GetRequestBodyLogger()
	if logger == nil {
		return
	}
	_ = logger.WriteResponseFromMap(reqID, logType, statusCode, headers, body)
}

// CleanupOldLogs 清理超过保留天数的日志目录
// 由 cron 任务每小时调用一次
func CleanupOldLogs() error {
	logger := GetRequestBodyLogger()
	if logger == nil || logger.config == nil {
		return nil
	}

	if logger.testMode {
		return nil
	}

	maxAgeDays := logger.config.GetMaxAgeDays()
	if maxAgeDays <= 0 {
		maxAgeDays = 14 // 默认 14 天
	}

	cutoffTime := time.Now().AddDate(0, 0, -maxAgeDays)

	return filepath.Walk(logger.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// 忽略不存在的路径
			return nil
		}

		if !info.IsDir() {
			return nil
		}

		// 尝试解析目录名为日期 (YYYY-MM-DD)
		dirName := filepath.Base(path)
		dirDate, err := time.Parse("2006-01-02", dirName)
		if err != nil {
			// 不是日期格式目录，跳过
			return nil
		}

		// 如果目录过期，删除它
		if dirDate.Before(cutoffTime) {
			if err := os.RemoveAll(path); err != nil {
				// 记录错误但不停止遍历
				loggerWriteError := fmt.Errorf("删除过期日志目录失败: %w", err)
				_ = loggerWriteError
			}
		}

		return nil
	})
}

// GetRequestBodyLoggerInfo 获取请求体日志器状态信息（用于监控）
func GetRequestBodyLoggerInfo() map[string]interface{} {
	logger := GetRequestBodyLogger()
	if logger == nil {
		return map[string]interface{}{
			"initialized": false,
		}
	}

	return map[string]interface{}{
		"initialized": true,
		"testMode":    logger.testMode,
		"baseDir":     logger.baseDir,
	}
}
