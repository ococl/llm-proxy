package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"llm-proxy/domain/port"
	"llm-proxy/infrastructure/config"

	"gopkg.in/natefinch/lumberjack.v2"
)

// sortJSONKeys 递归排序 JSON 对象的所有 key（升序）
// 输入可以是 map[string]interface{}, []interface{}, 或其他类型
// 返回排序后的对象
func sortJSONKeys(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		// 对于 map，创建一个新的有序 map
		sorted := make(map[string]interface{}, len(v))
		for key, value := range v {
			// 递归处理值
			sorted[key] = sortJSONKeys(value)
		}
		return sorted
	case []interface{}:
		// 对于数组，递归处理每个元素
		sorted := make([]interface{}, len(v))
		for i, item := range v {
			sorted[i] = sortJSONKeys(item)
		}
		return sorted
	default:
		// 基本类型直接返回
		return v
	}
}

// formatJSONWithSortedKeys 格式化 JSON，所有 key 按升序排列
// 输入可以是 map[string]interface{}, []byte, string, 或其他类型
// 返回格式化后的 JSON 字节数组
func formatJSONWithSortedKeys(data interface{}) ([]byte, error) {
	var jsonData interface{}

	switch v := data.(type) {
	case []byte:
		// 如果是字节数组，先尝试解析为 JSON
		if len(v) == 0 {
			return v, nil
		}
		// 检查是否是 JSON
		if err := json.Unmarshal(v, &jsonData); err != nil {
			// 不是有效的 JSON，直接返回原始数据
			return v, nil
		}
	case string:
		// 空字符串返回带引号的空字符串
		if v == "" {
			return []byte(`""`), nil
		}
		// 非 JSON 字符串作为 JSON 字符串值返回
		if err := json.Unmarshal([]byte(v), &jsonData); err != nil {
			return json.Marshal(v)
		}
	case map[string]interface{}:
		jsonData = v
	default:
		// 其他类型，尝试序列化后再解析
		tempJSON, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("序列化数据失败: %w", err)
		}
		if err := json.Unmarshal(tempJSON, &jsonData); err != nil {
			return tempJSON, nil
		}
	}

	// 递归排序所有 key
	sorted := sortJSONKeys(jsonData)

	// 使用自定义编码器确保 key 有序
	buf := &bytes.Buffer{}
	encoder := json.NewEncoder(buf)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(sorted); err != nil {
		return nil, fmt.Errorf("编码 JSON 失败: %w", err)
	}

	// 移除末尾的换行符（Encode 会自动添加）
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return result, nil
}

// marshalJSONSorted 将 map 序列化为 JSON，key 按升序排列
func marshalJSONSorted(data map[string]interface{}) ([]byte, error) {
	// 获取所有 key 并排序
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 手动构建 JSON
	buf := &bytes.Buffer{}
	buf.WriteString("{\n")

	for i, key := range keys {
		if i > 0 {
			buf.WriteString(",\n")
		}

		// 写入 key
		keyJSON, _ := json.Marshal(key)
		buf.WriteString("  ")
		buf.Write(keyJSON)
		buf.WriteString(": ")

		// 递归处理 value
		value := data[key]
		valueJSON, err := formatJSONWithSortedKeys(value)
		if err != nil {
			// 如果格式化失败，使用标准序列化
			valueJSON, _ = json.Marshal(value)
		}

		// 如果 value 是多行的，需要缩进
		valueStr := string(valueJSON)
		if strings.Contains(valueStr, "\n") {
			lines := strings.Split(valueStr, "\n")
			for j, line := range lines {
				if j > 0 {
					buf.WriteString("\n  ")
				}
				buf.WriteString(line)
			}
		} else {
			buf.WriteString(valueStr)
		}
	}

	buf.WriteString("\n}")
	return buf.Bytes(), nil
}

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
	config   *config.RequestBodyConfig
	mu       sync.Mutex
	rootDir  string
	baseDir  string
	disabled bool
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

	if !cfg.Logging.RequestBody.IsEnabled() {
		bodyLogger = &RequestBodyLogger{
			config:   &cfg.Logging.RequestBody,
			disabled: true,
			testMode: false,
		}
		return nil
	}

	dateDir := time.Now().Format("2006-01-02")
	rootDir := cfg.Logging.RequestBody.GetBaseDir()
	baseDir := filepath.Join(rootDir, dateDir)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("创建请求体日志目录失败: %w", err)
	}

	bodyLogger = &RequestBodyLogger{
		config:  &cfg.Logging.RequestBody,
		rootDir: rootDir,
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

	if bodyLogger != nil {
		bodyLogger = nil
		bodyLoggerInit = false
	}
	return nil
}

// GetRequestBodyLogger 获取请求体日志器实例
func GetRequestBodyLogger() *RequestBodyLogger {
	bodyLoggerMu.RLock()
	defer bodyLoggerMu.RUnlock()

	if !bodyLoggerInit || bodyLogger == nil {
		return nil
	}

	return bodyLogger
}

// Write 写入请求体日志
func (l *RequestBodyLogger) Write(reqID string, logType BodyLogType, httpReq *http.Request, body []byte) error {
	if l.testMode || l.disabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	timePrefix := now.Format("150405")

	// baseDir 已经包含了日期目录和 request_body 子目录
	filename := fmt.Sprintf("%s_%s_%s.httpdump", timePrefix, reqID, string(logType))
	filePath := filepath.Join(l.baseDir, filename)

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
	if len(body) > 0 && l.config.ShouldIncludeBody() {
		// 尝试格式化 JSON
		if formatted, err := formatJSONWithSortedKeys(body); err == nil {
			sb.Write(formatted)
		} else {
			sb.Write(body)
		}
		if !bytes.HasSuffix(body, []byte("\n")) {
			sb.WriteString("\n")
		}
	}

	// 写入文件
	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

// WriteResponse 写入响应体日志
func (l *RequestBodyLogger) WriteResponse(reqID string, logType BodyLogType, statusCode int, header http.Header, body []byte) error {
	if l.testMode || l.disabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	timePrefix := now.Format("150405")

	// baseDir 已经包含了日期目录和 request_body 子目录
	filename := fmt.Sprintf("%s_%s_%s.httpdump", timePrefix, reqID, string(logType))
	filePath := filepath.Join(l.baseDir, filename)

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
	if len(body) > 0 && l.config.ShouldIncludeBody() {
		// 尝试格式化 JSON
		if formatted, err := formatJSONWithSortedKeys(body); err == nil {
			sb.Write(formatted)
		} else {
			sb.Write(body)
		}
		if !bytes.HasSuffix(body, []byte("\n")) {
			sb.WriteString("\n")
		}
	}

	// 写入文件
	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

// WriteFromMap 从 map 数据写入请求体日志（用于客户端请求）
func (l *RequestBodyLogger) WriteFromMap(reqID string, logType BodyLogType, method, path, protocol string, headers map[string][]string, body map[string]interface{}) error {
	if l.testMode || l.disabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	timePrefix := now.Format("150405")

	// baseDir 已经包含了日期目录和 request_body 子目录
	filename := fmt.Sprintf("%s_%s_%s.httpdump", timePrefix, reqID, string(logType))
	filePath := filepath.Join(l.baseDir, filename)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s %s %s\r\n", method, path, protocol))

	// 写入请求头
	for key, values := range headers {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			sb.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
		}
	}

	sb.WriteString("\r\n")

	// 写入请求体
	if body != nil && l.config.ShouldIncludeBody() {
		if bodyJSON, err := marshalJSONSorted(body); err == nil {
			sb.Write(bodyJSON)
			sb.WriteString("\n")
		}
	}

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

// WriteResponseFromMap 从 map 数据写入响应体日志
func (l *RequestBodyLogger) WriteResponseFromMap(reqID string, logType BodyLogType, statusCode int, headers map[string][]string, body interface{}) error {
	if l.testMode || l.disabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	timePrefix := now.Format("150405")

	// baseDir 已经包含了日期目录和 request_body 子目录
	filename := fmt.Sprintf("%s_%s_%s.httpdump", timePrefix, reqID, string(logType))
	filePath := filepath.Join(l.baseDir, filename)

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
		switch v := body.(type) {
		case string:
			// 尝试格式化字符串形式的 JSON
			if formatted, err := formatJSONWithSortedKeys(v); err == nil {
				sb.Write(formatted)
			} else {
				sb.WriteString(v)
			}
			if !strings.HasSuffix(v, "\n") {
				sb.WriteString("\n")
			}
		case map[string]interface{}:
			// 对于 map 类型，使用排序后的 JSON
			if bodyJSON, err := marshalJSONSorted(v); err == nil {
				sb.Write(bodyJSON)
				sb.WriteString("\n")
			}
		default:
			// 其他类型，尝试格式化
			if bodyJSON, err := formatJSONWithSortedKeys(body); err == nil {
				sb.Write(bodyJSON)
				sb.WriteString("\n")
			}
		}
	}

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

// WriteUpstreamResponse 写入上游响应体日志（处理 io.Reader）
func (l *RequestBodyLogger) WriteUpstreamResponse(reqID string, statusCode int, header http.Header, body io.Reader) error {
	if l.testMode || l.disabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	timePrefix := now.Format("150405")

	// baseDir 已经包含了日期目录和 request_body 子目录
	filename := fmt.Sprintf("%s_%s_%s.httpdump", timePrefix, reqID, string(BodyLogTypeUpstreamResponse))
	filePath := filepath.Join(l.baseDir, filename)

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
			// 尝试格式化 JSON
			if formatted, err := formatJSONWithSortedKeys(bodyBytes); err == nil {
				sb.Write(formatted)
			} else {
				sb.Write(bodyBytes)
			}
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
	if err := logger.WriteFromMap(reqID, logType, method, path, protocol, headers, body); err != nil {
		// 记录日志写入失败但不中断请求处理
		GeneralSugar.Errorw("写入请求体日志失败",
			port.ReqID(reqID),
			port.Error(err),
		)
	}
}

// LogResponseBody 便捷函数：记录响应体
func LogResponseBody(reqID string, logType BodyLogType, statusCode int, headers map[string][]string, body interface{}) {
	logger := GetRequestBodyLogger()
	if logger == nil {
		return
	}
	if err := logger.WriteResponseFromMap(reqID, logType, statusCode, headers, body); err != nil {
		// 记录日志写入失败但不中断请求处理
		GeneralSugar.Errorw("写入响应体日志失败",
			port.ReqID(reqID),
			port.Error(err),
		)
	}
}

// CleanupOldLogs 清理超过保留天数的日志目录
// 由 cron 任务每小时调用一次
func CleanupOldLogs() error {
	logger := GetRequestBodyLogger()
	if logger == nil || logger.config == nil {
		return nil
	}

	if logger.testMode || logger.disabled {
		return nil
	}

	maxAgeDays := logger.config.GetMaxAgeDays()
	if maxAgeDays <= 0 {
		maxAgeDays = 14 // 默认 14 天
	}

	cutoffTime := time.Now().AddDate(0, 0, -maxAgeDays)

	return filepath.Walk(logger.rootDir, func(path string, info os.FileInfo, err error) error {
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
				GeneralSugar.Errorw("删除过期日志目录失败",
					port.ReqID(path),
					port.Error(err),
				)
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
		"disabled":    logger.disabled,
		"rootDir":     logger.rootDir,
		"baseDir":     logger.baseDir,
	}
}

// WriteDiff 写入请求体差异日志
func (l *RequestBodyLogger) WriteDiff(reqID string, original, modified map[string]interface{}) error {
	if l.testMode || l.disabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	timePrefix := now.Format("150405")

	filename := fmt.Sprintf("%s_%s_request_diff.json", timePrefix, reqID)
	filePath := filepath.Join(l.baseDir, filename)

	diffResult := CompareJSON(original, modified, DefaultDiffOptions())

	diffJSON, err := diffResult.ToJSON()
	if err != nil {
		return fmt.Errorf("生成 diff JSON 失败: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# Request Diff: client_request -> upstream_request\n")
	sb.WriteString("# 此文件记录客户端请求转换为上游请求时的字段差异\n")
	sb.WriteString("# 忽略预期差异字段: model\n")
	sb.WriteString("\n")
	sb.WriteString(diffJSON)
	sb.WriteString("\n")

	if !diffResult.IsEmpty() {
		sb.WriteString("\n# 差异摘要:\n")
		summary := diffResult.ToSummary()
		for _, line := range strings.Split(summary, "\n") {
			if line != "" {
				sb.WriteString("# ")
				sb.WriteString(line)
				sb.WriteString("\n")
			}
		}
	}

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

// LogRequestDiff 便捷函数：记录请求体差异
func LogRequestDiff(reqID string, original, modified map[string]interface{}) {
	logger := GetRequestBodyLogger()
	if logger == nil {
		return
	}
	if err := logger.WriteDiff(reqID, original, modified); err != nil {
		GeneralSugar.Errorw("写入请求体差异日志失败",
			port.ReqID(reqID),
			port.Error(err),
		)
	}
}
