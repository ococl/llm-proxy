package backend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/types"
)

// TestHTTPClient_NewHTTPClient 测试 HTTPClient 构造函数。
func TestHTTPClient_NewHTTPClient(t *testing.T) {
	tests := []struct {
		name        string
		inputClient *http.Client
		expectNil   bool
	}{
		{
			name:        "nil client 使用默认客户端",
			inputClient: nil,
			expectNil:   false,
		},
		{
			name:        "自定义客户端",
			inputClient: &http.Client{},
			expectNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewHTTPClient(tt.inputClient)

			if tt.expectNil {
				if client != nil {
					t.Errorf("期望 nil，但得到 %v", client)
				}
				return
			}

			if client == nil {
				t.Fatal("期望非 nil，但得到 nil")
			}

			if client.GetHTTPClient() == nil {
				t.Error("GetHTTPClient() 不应返回 nil")
			}
		})
	}
}

// TestHTTPClient_GetHTTPClient 测试获取底层 HTTP 客户端。
func TestHTTPClient_GetHTTPClient(t *testing.T) {
	customClient := &http.Client{}
	httpClient := NewHTTPClient(customClient)

	retrieved := httpClient.GetHTTPClient()
	if retrieved != customClient {
		t.Errorf("期望返回相同的客户端实例")
	}
}

// TestHTTPClient_Send_NilRequest 测试发送 nil 请求。
func TestHTTPClient_Send_NilRequest(t *testing.T) {
	httpClient := NewHTTPClient(nil)

	_, err := httpClient.Send(context.Background(), nil)
	if err == nil {
		t.Error("期望错误，但得到 nil")
	}
}

// TestHTTPClient_Send_NilBackend 测试发送 BackendRequest 但 Backend 为 nil。
func TestHTTPClient_Send_NilBackend(t *testing.T) {
	httpClient := NewHTTPClient(nil)

	req := &BackendRequest{
		Backend: nil,
		Body:    map[string]interface{}{"test": "value"},
	}

	_, err := httpClient.Send(context.Background(), req)
	if err == nil {
		t.Error("期望错误，但得到 nil")
	}
}

// TestHTTPClient_Send_Success 测试成功发送请求。
func TestHTTPClient_Send_Success(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求
		if r.Method != "POST" {
			t.Errorf("期望 POST 方法，得到 %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("期望 Content-Type: application/json，得到 %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices": [{"message": {"content": "test"}}]}`))
	}))
	defer server.Close()

	// 创建后端
	backend, err := entity.NewBackend("test", server.URL, "test-key", true, types.ProtocolOpenAI)
	if err != nil {
		t.Fatalf("创建后端失败: %v", err)
	}

	httpClient := NewHTTPClient(nil)

	req := &BackendRequest{
		Backend: backend,
		Body:    map[string]interface{}{"model": "gpt-3.5-turbo"},
		Headers: map[string][]string{},
		Path:    "/v1/chat/completions",
		Stream:  false,
	}

	resp, err := httpClient.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 200，得到 %d", resp.StatusCode)
	}
}

// TestHTTPClient_Send_WithAPIKey 测试带 API Key 的请求（OpenAI 协议）。
func TestHTTPClient_Send_WithAPIKey_OpenAI(t *testing.T) {
	var capturedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	backend, _ := entity.NewBackend("test", server.URL, "sk-test-api-key-12345", true, types.ProtocolOpenAI)

	httpClient := NewHTTPClient(nil)

	req := &BackendRequest{
		Backend: backend,
		Body:    map[string]interface{}{},
	}

	_, err := httpClient.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}

	if capturedAuth != "Bearer sk-test-api-key-12345" {
		t.Errorf("期望 Authorization header，得到 %s", capturedAuth)
	}
}

// TestHTTPClient_Send_WithAPIKey_Anthropic 测试带 API Key 的请求（Anthropic 协议）。
func TestHTTPClient_Send_WithAPIKey_Anthropic(t *testing.T) {
	var capturedAPIKey string
	var capturedVersion string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAPIKey = r.Header.Get("x-api-key")
		capturedVersion = r.Header.Get("anthropic-version")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	backend, _ := entity.NewBackend("test", server.URL, "sk-ant-api-key-12345", true, types.ProtocolAnthropic)

	httpClient := NewHTTPClient(nil)

	req := &BackendRequest{
		Backend: backend,
		Body:    map[string]interface{}{},
	}

	_, err := httpClient.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}

	if capturedAPIKey != "sk-ant-api-key-12345" {
		t.Errorf("期望 x-api-key header，得到 %s", capturedAPIKey)
	}
	if capturedVersion != "2023-06-01" {
		t.Errorf("期望 anthropic-version header，得到 %s", capturedVersion)
	}
}

// TestHTTPClient_Send_WithLocale 测试带 Locale 的请求。
func TestHTTPClient_Send_WithLocale(t *testing.T) {
	var capturedLocale string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedLocale = r.Header.Get("Accept-Language")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	backend, _ := entity.NewBackendWithLocale("test", server.URL, "key", true, types.ProtocolOpenAI, "zh-CN")

	httpClient := NewHTTPClient(nil)

	req := &BackendRequest{
		Backend: backend,
		Body:    map[string]interface{}{},
	}

	_, err := httpClient.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}

	if capturedLocale != "zh-CN" {
		t.Errorf("期望 Accept-Language: zh-CN，得到 %s", capturedLocale)
	}
}

// TestHTTPClient_Send_WithEmptyAPIKey 测试空 API Key 不设置 Authorization header。
func TestHTTPClient_Send_WithEmptyAPIKey(t *testing.T) {
	var capturedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	backend, _ := entity.NewBackend("test", server.URL, "", true, types.ProtocolOpenAI)

	httpClient := NewHTTPClient(nil)

	req := &BackendRequest{
		Backend: backend,
		Body:    map[string]interface{}{},
	}

	_, err := httpClient.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}

	if capturedAuth != "" {
		t.Errorf("期望空的 Authorization header，得到 %s", capturedAuth)
	}
}

// TestHTTPClient_Send_WithCustomHeaders 测试自定义 headers。
func TestHTTPClient_Send_WithCustomHeaders(t *testing.T) {
	var capturedCustom string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCustom = r.Header.Get("X-Custom-Header")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	backend, _ := entity.NewBackend("test", server.URL, "key", true, types.ProtocolOpenAI)

	httpClient := NewHTTPClient(nil)

	req := &BackendRequest{
		Backend: backend,
		Body:    map[string]interface{}{},
		Headers: map[string][]string{
			"X-Custom-Header": {"custom-value"},
		},
	}

	_, err := httpClient.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}

	if capturedCustom != "custom-value" {
		t.Errorf("期望 X-Custom-Header: custom-value，得到 %s", capturedCustom)
	}
}

// TestHTTPClient_Send_HopByHopHeaders 测试 hop-by-hop headers 被过滤。
func TestHTTPClient_Send_HopByHopHeaders(t *testing.T) {
	var capturedConnection string
	var capturedTransferEncoding string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedConnection = r.Header.Get("Connection")
		capturedTransferEncoding = r.Header.Get("Transfer-Encoding")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	backend, _ := entity.NewBackend("test", server.URL, "key", true, types.ProtocolOpenAI)

	httpClient := NewHTTPClient(nil)

	req := &BackendRequest{
		Backend: backend,
		Body:    map[string]interface{}{},
		Headers: map[string][]string{
			"Connection":        {"keep-alive"},
			"Transfer-Encoding": {"chunked"},
			"X-Custom-Header":   {"custom-value"},
		},
	}

	_, err := httpClient.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}

	// hop-by-hop headers 应该被过滤
	if capturedConnection != "" {
		t.Errorf("期望 Connection header 被过滤，得到 %s", capturedConnection)
	}
	if capturedTransferEncoding != "" {
		t.Errorf("期望 Transfer-Encoding header 被过滤，得到 %s", capturedTransferEncoding)
	}
}

// TestHTTPClient_Send_InvalidJSON 测试无效 JSON body。
func TestHTTPClient_Send_InvalidJSON(t *testing.T) {
	backend, _ := entity.NewBackend("test", "https://example.com", "key", true, types.ProtocolOpenAI)

	httpClient := NewHTTPClient(nil)

	// 使用无法序列化的值
	req := &BackendRequest{
		Backend: backend,
		Body: map[string]interface{}{
			"channel": make(chan int), // channel 无法 JSON 序列化
		},
	}

	_, err := httpClient.Send(context.Background(), req)
	if err == nil {
		t.Error("期望错误（无效 JSON），但得到 nil")
	}
}

// TestHTTPClient_Send_DefaultProtocol 测试默认协议（OpenAI）。
func TestHTTPClient_Send_DefaultProtocol(t *testing.T) {
	var capturedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	// 创建后端时不指定协议
	backend, _ := entity.NewBackend("test", server.URL, "test-key", true, "")

	httpClient := NewHTTPClient(nil)

	req := &BackendRequest{
		Backend: backend,
		Body:    map[string]interface{}{},
	}

	_, err := httpClient.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}

	// 默认协议应该使用 Bearer token
	if capturedAuth != "Bearer test-key" {
		t.Errorf("期望 Authorization: Bearer test-key，得到 %s", capturedAuth)
	}
}

// TestHTTPClient_Send_PathConstruction 测试路径构造。
func TestHTTPClient_Send_PathConstruction(t *testing.T) {
	var capturedURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	backend, _ := entity.NewBackend("test", server.URL, "key", true, types.ProtocolOpenAI)

	httpClient := NewHTTPClient(nil)

	req := &BackendRequest{
		Backend: backend,
		Body:    map[string]interface{}{},
		Path:    "v1/chat/completions", // 不带前导斜杠
	}

	_, err := httpClient.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}

	// 应该正确拼接 URL
	if capturedURL != "/v1/chat/completions" {
		t.Errorf("期望路径 /v1/chat/completions，得到 %s", capturedURL)
	}
}

// TestHTTPClient_Send_PathWithLeadingSlash 测试带前导斜杠的路径。
func TestHTTPClient_Send_PathWithLeadingSlash(t *testing.T) {
	var capturedURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	backend, _ := entity.NewBackend("test", server.URL, "key", true, types.ProtocolOpenAI)

	httpClient := NewHTTPClient(nil)

	req := &BackendRequest{
		Backend: backend,
		Body:    map[string]interface{}{},
		Path:    "/v1/chat/completions", // 带前导斜杠
	}

	_, err := httpClient.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}

	// 应该正确处理前导斜杠
	if capturedURL != "/v1/chat/completions" {
		t.Errorf("期望路径 /v1/chat/completions，得到 %s", capturedURL)
	}
}

// TestHTTPClient_Send_BackendWithTrailingSlash 测试后端 URL 带尾斜杠。
func TestHTTPClient_Send_BackendWithTrailingSlash(t *testing.T) {
	var capturedURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	backend, _ := entity.NewBackend("test", server.URL+"/", "key", true, types.ProtocolOpenAI)

	httpClient := NewHTTPClient(nil)

	req := &BackendRequest{
		Backend: backend,
		Body:    map[string]interface{}{},
		Path:    "v1/chat/completions",
	}

	_, err := httpClient.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}

	// 应该正确处理尾斜杠
	if capturedURL != "/v1/chat/completions" {
		t.Errorf("期望路径 /v1/chat/completions，得到 %s", capturedURL)
	}
}

// TestIsHopByHopHeader 测试 hop-by-hop header 判断函数。
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
		{"Content-Type", false},
		{"Authorization", false},
		{"Accept-Language", false},
		{"X-Custom-Header", false},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			result := isHopByHopHeader(tt.header)
			if result != tt.expected {
				t.Errorf("isHopByHopHeader(%q) = %v, 期望 %v", tt.header, result, tt.expected)
			}
		})
	}
}
