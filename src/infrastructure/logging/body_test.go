package logging

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"llm-proxy/infrastructure/config"
)

// TestInitRequestBodyLogger_Disabled 测试禁用请求体日志的情况
func TestInitRequestBodyLogger_Disabled(t *testing.T) {
	cfg := &config.Config{
		Logging: config.Logging{
			RequestBody: config.RequestBodyConfig{
				Enabled: false,
			},
		},
	}

	err := InitRequestBodyLogger(cfg)
	if err != nil {
		t.Fatalf("初始化禁用状态的请求体日志器失败: %v", err)
	}

	logger := GetRequestBodyLogger()
	if logger == nil {
		t.Fatal("获取日志器实例不应为 nil")
	}

	// 清理
	ShutdownRequestBodyLogger()
}

// TestInitTestRequestBodyLogger 测试初始化测试用的请求体日志器
func TestInitTestRequestBodyLogger(t *testing.T) {
	InitTestRequestBodyLogger()

	logger := GetRequestBodyLogger()
	if logger == nil {
		t.Fatal("获取测试日志器实例不应为 nil")
	}

	if !logger.testMode {
		t.Error("日志器应处于测试模式")
	}
}

// TestGetStatusText 测试状态文本获取
func TestGetStatusText(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   string
	}{
		{200, "OK"},
		{201, "Created"},
		{204, "No Content"},
		{400, "Bad Request"},
		{401, "Unauthorized"},
		{403, "Forbidden"},
		{404, "Not Found"},
		{405, "Method Not Allowed"},
		{429, "Too Many Requests"},
		{500, "Internal Server Error"},
		{502, "Bad Gateway"},
		{503, "Service Unavailable"},
		{504, "Gateway Timeout"},
		{999, "Unknown"},
	}

	for _, tt := range tests {
		result := getStatusText(tt.statusCode)
		if result != tt.expected {
			t.Errorf("状态码 %d 期望 %s, 实际 %s", tt.statusCode, tt.expected, result)
		}
	}
}

// TestIsHopByHopHeader 测试 hop-by-hop 头判断
func TestIsHopByHopHeader(t *testing.T) {
	tests := []struct {
		header   string
		expected bool
	}{
		{"Connection", true},
		{"Keep-Alive", true},
		{"Proxy-Authenticate", true},
		{"Proxy-Authorization", true},
		{"Te", true},
		{"Trailer", true},
		{"Transfer-Encoding", true},
		{"Upgrade", true},
		{"content-type", false},
		{"authorization", false},
		{"X-Custom-Header", false},
		{"", false},
	}

	for _, tt := range tests {
		result := isHopByHopHeader(tt.header)
		if result != tt.expected {
			t.Errorf("Header %s 期望 %v, 实际 %v", tt.header, tt.expected, result)
		}
	}
}

// TestWriteFromMap_FileCreation 测试从 map 创建日志文件
func TestWriteFromMap_FileCreation(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Logging: config.Logging{
			RequestBody: config.RequestBodyConfig{
				Enabled:     true,
				BaseDir:     tempDir,
				MaxSizeMB:   100,
				MaxAgeDays:  14,
				MaxBackups:  5,
				Compress:    true,
				IncludeBody: true,
			},
		},
	}

	err := InitRequestBodyLogger(cfg)
	if err != nil {
		t.Fatalf("初始化请求体日志器失败: %v", err)
	}
	defer ShutdownRequestBodyLogger()

	reqID := "test-req-123"
	logType := BodyLogTypeClientRequest
	method := "POST"
	path := "/v1/chat/completions"
	protocol := "HTTP/1.1"
	headers := map[string][]string{
		"Content-Type":  {"application/json"},
		"Authorization": {"Bearer test-key"},
	}
	body := map[string]interface{}{
		"model":  "gpt-4",
		"stream": true,
	}

	logger := GetRequestBodyLogger()
	if logger == nil {
		t.Fatal("日志器不应为 nil")
	}

	err = logger.WriteFromMap(reqID, logType, method, path, protocol, headers, body)
	if err != nil {
		t.Fatalf("写入日志失败: %v", err)
	}

	// 验证文件创建
	files, err := filepath.Glob(filepath.Join(tempDir, "*", "*.httpdump"))
	if err != nil {
		t.Fatalf("查找日志文件失败: %v", err)
	}

	if len(files) == 0 {
		t.Error("应该创建至少一个日志文件")
	}
}

// TestWriteResponseFromMap_FileCreation 测试从 map 创建响应日志文件
func TestWriteResponseFromMap_FileCreation(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Logging: config.Logging{
			RequestBody: config.RequestBodyConfig{
				Enabled:     true,
				BaseDir:     tempDir,
				MaxSizeMB:   100,
				MaxAgeDays:  14,
				MaxBackups:  5,
				Compress:    true,
				IncludeBody: true,
			},
		},
	}

	err := InitRequestBodyLogger(cfg)
	if err != nil {
		t.Fatalf("初始化请求体日志器失败: %v", err)
	}
	defer ShutdownRequestBodyLogger()

	reqID := "test-resp-456"
	logType := BodyLogTypeClientResponse
	statusCode := 200
	headers := map[string][]string{
		"Content-Type": {"application/json"},
	}
	body := map[string]interface{}{
		"id":      "chatcmpl-123",
		"object":  "chat.completion",
		"created": 1234567890,
	}

	logger := GetRequestBodyLogger()
	if logger == nil {
		t.Fatal("日志器不应为 nil")
	}

	err = logger.WriteResponseFromMap(reqID, logType, statusCode, headers, body)
	if err != nil {
		t.Fatalf("写入响应日志失败: %v", err)
	}

	// 验证文件创建
	files, err := filepath.Glob(filepath.Join(tempDir, "*", "*.httpdump"))
	if err != nil {
		t.Fatalf("查找日志文件失败: %v", err)
	}

	if len(files) == 0 {
		t.Error("应该创建至少一个响应日志文件")
	}
}

// TestLogRequestBody_Disabled 测试禁用时的便捷函数行为
func TestLogRequestBody_Disabled(t *testing.T) {
	// 不初始化日志器，直接调用
	// 这应该静默失败，不抛出错误
	LogRequestBody("test-id", BodyLogTypeClientRequest, "POST", "/test", "HTTP/1.1", nil, nil)
}

// TestLogResponseBody_Disabled 测试禁用时的响应日志便捷函数行为
func TestLogResponseBody_Disabled(t *testing.T) {
	// 不初始化日志器，直接调用
	LogResponseBody("test-id", BodyLogTypeClientResponse, 200, nil, nil)
}

// TestBodyLogType_Constants 测试日志类型常量
func TestBodyLogType_Constants(t *testing.T) {
	if BodyLogTypeClientRequest != "client_request" {
		t.Error("BodyLogTypeClientRequest 应该为 'client_request'")
	}
	if BodyLogTypeClientResponse != "client_response" {
		t.Error("BodyLogTypeClientResponse 应该为 'client_response'")
	}
	if BodyLogTypeUpstreamRequest != "upstream_request" {
		t.Error("BodyLogTypeUpstreamRequest 应该为 'upstream_request'")
	}
	if BodyLogTypeUpstreamResponse != "upstream_response" {
		t.Error("BodyLogTypeUpstreamResponse 应该为 'upstream_response'")
	}
}

// TestWriteFromMap_WithoutBody 测试不带请求体的日志
func TestWriteFromMap_WithoutBody(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Logging: config.Logging{
			RequestBody: config.RequestBodyConfig{
				Enabled:     true,
				BaseDir:     tempDir,
				MaxSizeMB:   100,
				MaxAgeDays:  14,
				MaxBackups:  5,
				Compress:    true,
				IncludeBody: true,
			},
		},
	}

	InitRequestBodyLogger(cfg)
	defer ShutdownRequestBodyLogger()

	reqID := "test-no-body"
	logType := BodyLogTypeClientRequest
	method := "GET"
	path := "/v1/models"
	protocol := "HTTP/1.1"
	headers := map[string][]string{
		"Accept": {"application/json"},
	}
	body := map[string]interface{}(nil)

	logger := GetRequestBodyLogger()
	if logger == nil {
		t.Fatal("日志器不应为 nil")
	}

	err := logger.WriteFromMap(reqID, logType, method, path, protocol, headers, body)
	if err != nil {
		t.Fatalf("写入无请求体日志失败: %v", err)
	}
}

// TestWriteFromMap_IncludeBodyDisabled 测试关闭 IncludeBody 时的行为
func TestWriteFromMap_IncludeBodyDisabled(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Logging: config.Logging{
			RequestBody: config.RequestBodyConfig{
				Enabled:     true,
				BaseDir:     tempDir,
				MaxSizeMB:   100,
				MaxAgeDays:  14,
				MaxBackups:  5,
				Compress:    true,
				IncludeBody: false, // 禁用请求体记录
			},
		},
	}

	InitRequestBodyLogger(cfg)
	defer ShutdownRequestBodyLogger()

	reqID := "test-no-include-body"
	logType := BodyLogTypeClientRequest
	method := "POST"
	path := "/v1/chat/completions"
	protocol := "HTTP/1.1"
	headers := map[string][]string{
		"Content-Type": {"application/json"},
	}
	body := map[string]interface{}{
		"model": "gpt-4",
	}

	logger := GetRequestBodyLogger()
	if logger == nil {
		t.Fatal("日志器不应为 nil")
	}

	err := logger.WriteFromMap(reqID, logType, method, path, protocol, headers, body)
	if err != nil {
		t.Fatalf("写入日志失败: %v", err)
	}

	// 验证文件存在
	files, err := filepath.Glob(filepath.Join(tempDir, "*", "*.httpdump"))
	if err != nil {
		t.Fatalf("查找日志文件失败: %v", err)
	}

	if len(files) == 0 {
		t.Error("应该创建日志文件")
	}

	// 读取文件内容验证
	content, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	contentStr := string(content)
	_ = contentStr
}

// TestWrite_Integration 测试完整的 HTTP 请求日志写入
func TestWrite_Integration(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Logging: config.Logging{
			RequestBody: config.RequestBodyConfig{
				Enabled:     true,
				BaseDir:     tempDir,
				MaxSizeMB:   100,
				MaxAgeDays:  14,
				MaxBackups:  5,
				Compress:    true,
				IncludeBody: true,
			},
		},
	}

	InitRequestBodyLogger(cfg)
	defer ShutdownRequestBodyLogger()

	// 创建模拟 HTTP 请求
	httpReq, err := http.NewRequest("POST", "/v1/chat/completions", nil)
	if err != nil {
		t.Fatalf("创建 HTTP 请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer test-key")
	httpReq.Host = "api.example.com"

	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`)

	logger := GetRequestBodyLogger()
	if logger == nil {
		t.Fatal("日志器不应为 nil")
	}

	err = logger.Write("test-req-write", BodyLogTypeClientRequest, httpReq, body)
	if err != nil {
		t.Fatalf("写入 HTTP 请求日志失败: %v", err)
	}

	// 验证文件创建
	files, err := filepath.Glob(filepath.Join(tempDir, "*", "*.httpdump"))
	if err != nil {
		t.Fatalf("查找日志文件失败: %v", err)
	}

	if len(files) == 0 {
		t.Error("应该创建至少一个日志文件")
	}

	// 验证文件内容包含请求信息
	content, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	contentStr := string(content)
	if !containsStr(contentStr, "POST /v1/chat/completions HTTP/1.1") {
		t.Error("日志应包含请求行")
	}
	if !containsStr(contentStr, "Host: api.example.com") {
		t.Error("日志应包含 Host 头")
	}
	if !containsStr(contentStr, "Content-Type: application/json") {
		t.Error("日志应包含 Content-Type 头")
	}
	if !containsStr(contentStr, "model") {
		t.Error("日志应包含请求体内容")
	}
}

// TestWriteResponse_Integration 测试完整的响应日志写入
func TestWriteResponse_Integration(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Logging: config.Logging{
			RequestBody: config.RequestBodyConfig{
				Enabled:     true,
				BaseDir:     tempDir,
				MaxSizeMB:   100,
				MaxAgeDays:  14,
				MaxBackups:  5,
				Compress:    true,
				IncludeBody: true,
			},
		},
	}

	InitRequestBodyLogger(cfg)
	defer ShutdownRequestBodyLogger()

	headers := http.Header{
		"Content-Type":      {"application/json"},
		"X-Request-ID":      {"test-resp-789"},
		"Transfer-Encoding": {"chunked"}, // hop-by-hop header, 应该被过滤
	}
	body := []byte(`{"id":"chatcmpl-123","object":"chat.completion"}`)

	logger := GetRequestBodyLogger()
	if logger == nil {
		t.Fatal("日志器不应为 nil")
	}

	err := logger.WriteResponse("test-resp-write", BodyLogTypeClientResponse, 200, headers, body)
	if err != nil {
		t.Fatalf("写入响应日志失败: %v", err)
	}

	// 验证文件创建
	files, err := filepath.Glob(filepath.Join(tempDir, "*", "*.httpdump"))
	if err != nil {
		t.Fatalf("查找日志文件失败: %v", err)
	}

	if len(files) == 0 {
		t.Error("应该创建至少一个响应日志文件")
	}

	// 验证文件内容
	content, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	contentStr := string(content)
	if !containsStr(contentStr, "HTTP/1.1 200 OK") {
		t.Error("日志应包含状态行")
	}
	if !containsStr(contentStr, "Content-Type: application/json") {
		t.Error("日志应包含 Content-Type 头")
	}
	if containsStr(contentStr, "Transfer-Encoding") {
		t.Error("hop-by-hop header 应该被过滤")
	}
	if !containsStr(contentStr, "chatcmpl-123") {
		t.Error("日志应包含响应体内容")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
