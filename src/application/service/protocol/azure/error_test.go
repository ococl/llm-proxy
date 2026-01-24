package azure

import (
	"net/http"
	"testing"

	"llm-proxy/domain/port"
	"llm-proxy/domain/types"

	domainerror "llm-proxy/domain/error"
)

// MockLogger 用于测试的 Mock 日志记录器。
type MockLogger struct {
	debugMessages []string
}

func (m *MockLogger) Debug(msg string, fields ...port.Field) {
	m.debugMessages = append(m.debugMessages, msg)
}

func (m *MockLogger) Error(msg string, fields ...port.Field) {}

func (m *MockLogger) Fatal(msg string, fields ...port.Field) {}

func (m *MockLogger) Info(msg string, fields ...port.Field) {}

func (m *MockLogger) Warn(msg string, fields ...port.Field) {}

func (m *MockLogger) With(fields ...port.Field) port.Logger {
	return m
}

// TestErrorConverter_Convert 测试 Azure 错误转换器的 Convert 方法。
func TestErrorConverter_Convert(t *testing.T) {
	logger := &MockLogger{}
	converter := NewErrorConverter(logger)

	t.Run("解析 Azure 标准错误格式", func(t *testing.T) {
		body := []byte(`{"error":{"code":"invalid_request_error","message":"Invalid request parameters"}}`)
		result := converter.Convert(http.StatusBadRequest, body)

		if result == nil {
			t.Fatal("期望非 nil 结果")
		}

		if result.Code != domainerror.CodeInvalidRequest {
			t.Errorf("期望错误码 %s, 实际 %s", domainerror.CodeInvalidRequest, result.Code)
		}

		if result.HTTPStatus != http.StatusBadRequest {
			t.Errorf("期望状态码 %d, 实际 %d", http.StatusBadRequest, result.HTTPStatus)
		}

		if result.Message != "Invalid request parameters" {
			t.Errorf("期望消息 'Invalid request parameters', 实际 '%s'", result.Message)
		}

		if result.Provider != "azure" {
			t.Errorf("期望 provider 'azure', 实际 '%s'", result.Provider)
		}

		if result.Retryable {
			t.Error("400 错误不应重试")
		}
	})

	t.Run("处理 content_filter 错误", func(t *testing.T) {
		body := []byte(`{"error":{"code":"content_filter","message":"Content filtered due to safety policy"}}`)
		result := converter.Convert(http.StatusBadRequest, body)

		if result.Code != domainerror.CodeInvalidRequest {
			t.Errorf("content_filter 应映射到 %s, 实际 %s", domainerror.CodeInvalidRequest, result.Code)
		}

		if result.Retryable {
			t.Error("content_filter 错误不应重试")
		}
	})

	t.Run("处理 rate_limit 错误", func(t *testing.T) {
		body := []byte(`{"error":{"code":"rate_limit_exceeded","message":"Rate limit exceeded"}}`)
		result := converter.Convert(http.StatusTooManyRequests, body)

		if result.Code != domainerror.CodeRateLimited {
			t.Errorf("rate_limit 应映射到 %s, 实际 %s", domainerror.CodeRateLimited, result.Code)
		}

		if !result.Retryable {
			t.Error("429 错误应可重试")
		}
	})

	t.Run("处理认证错误", func(t *testing.T) {
		body := []byte(`{"error":{"code":"invalid_api_key","message":"Invalid API key"}}`)
		result := converter.Convert(http.StatusUnauthorized, body)

		if result.Code != domainerror.CodeUnauthorized {
			t.Errorf("invalid_api_key 应映射到 %s, 实际 %s", domainerror.CodeUnauthorized, result.Code)
		}

		if result.Retryable {
			t.Error("401 错误不应重试")
		}
	})

	t.Run("处理数字状态码错误", func(t *testing.T) {
		body := []byte(`{"error":{"code":"404","message":"Resource not found"}}`)
		result := converter.Convert(http.StatusNotFound, body)

		if result.Code != domainerror.CodeBadRequest {
			t.Errorf("404 应映射到 %s, 实际 %s", domainerror.CodeBadRequest, result.Code)
		}
	})

	t.Run("处理服务端错误", func(t *testing.T) {
		body := []byte(`{"error":{"code":"internal_server_error","message":"Internal server error"}}`)
		result := converter.Convert(http.StatusInternalServerError, body)

		if result.Code != domainerror.CodeBackendError {
			t.Errorf("500 应映射到 %s, 实际 %s", domainerror.CodeBackendError, result.Code)
		}

		if !result.Retryable {
			t.Error("500 错误应可重试")
		}
	})

	t.Run("处理空响应体", func(t *testing.T) {
		result := converter.Convert(http.StatusBadRequest, nil)

		if result == nil {
			t.Fatal("期望非 nil 结果")
		}

		if result.Code != domainerror.CodeInvalidRequest {
			t.Errorf("空响应应使用状态码映射, 期望 %s, 实际 %s", domainerror.CodeInvalidRequest, result.Code)
		}

		if result.Message != "Azure OpenAI 请求参数无效" {
			t.Errorf("期望默认中文消息, 实际 '%s'", result.Message)
		}
	})

	t.Run("处理无效 JSON", func(t *testing.T) {
		body := []byte("invalid json response")
		result := converter.Convert(http.StatusInternalServerError, body)

		if result.Code != domainerror.CodeBackendError {
			t.Errorf("无效 JSON 应使用状态码映射, 期望 %s, 实际 %s", domainerror.CodeBackendError, result.Code)
		}
	})

	t.Run("记录调试日志", func(t *testing.T) {
		logger := &MockLogger{}
		converter := NewErrorConverter(logger)

		body := []byte(`{"error":{"code":"429","message":"Rate limited"}}`)
		converter.Convert(http.StatusTooManyRequests, body)

		if len(logger.debugMessages) != 1 {
			t.Errorf("期望 1 条调试日志, 实际 %d 条", len(logger.debugMessages))
		}
	})
}

// TestErrorConverter_Supports 测试 Supports 方法。
func TestErrorConverter_Supports(t *testing.T) {
	converter := NewErrorConverter(nil)

	tests := []struct {
		protocol types.Protocol
		expected bool
	}{
		{types.ProtocolAzure, true},
		{types.ProtocolOpenAI, false},
		{types.ProtocolAnthropic, false},
		{types.ProtocolGoogle, false},
		{types.ProtocolDeepSeek, false},
		{types.ProtocolGroq, false},
		{types.ProtocolMistral, false},
		{types.ProtocolCohere, false},
		{types.ProtocolCustom, false},
	}

	for _, tt := range tests {
		result := converter.Supports(tt.protocol)
		if result != tt.expected {
			t.Errorf("Supports(%s): 期望 %v, 实际 %v", tt.protocol, tt.expected, result)
		}
	}
}

// TestErrorConverter_Protocol 测试 Protocol 方法。
func TestErrorConverter_Protocol(t *testing.T) {
	converter := NewErrorConverter(nil)

	if converter.Protocol() != types.ProtocolAzure {
		t.Errorf("期望 Protocol Azure, 实际 %s", converter.Protocol())
	}
}

// TestErrorConverter_Name 测试 Name 方法。
func TestErrorConverter_Name(t *testing.T) {
	converter := NewErrorConverter(nil)

	if converter.Name() != "AzureErrorConverter" {
		t.Errorf("期望名称 'AzureErrorConverter', 实际 '%s'", converter.Name())
	}
}

// TestGetErrorCodeFromCode 测试错误码转换逻辑。
func TestGetErrorCodeFromCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"content_filter", string(domainerror.CodeInvalidRequest)},
		{"429", string(domainerror.CodeRateLimited)},
		{"rate_limit_exceeded", string(domainerror.CodeRateLimited)},
		{"invalid_api_key", string(domainerror.CodeUnauthorized)},
		{"authentication_error", string(domainerror.CodeUnauthorized)},
		{"invalid_request_error", string(domainerror.CodeInvalidRequest)},
		{"invalid_parameter", string(domainerror.CodeInvalidRequest)},
		{"not_found_error", string(domainerror.CodeBadRequest)},
		{"permission_error", string(domainerror.CodeBadRequest)},
		{"400", string(domainerror.CodeInvalidRequest)},
		{"401", string(domainerror.CodeUnauthorized)},
		{"403", string(domainerror.CodeBadRequest)},
		{"404", string(domainerror.CodeBadRequest)},
		{"500", string(domainerror.CodeBackendError)},
		{"unknown_code", string(domainerror.CodeUnknown)},
	}

	for _, tt := range tests {
		result := getErrorCodeFromCode(tt.input)
		if result != tt.expected {
			t.Errorf("getErrorCodeFromCode(%s): 期望 %s, 实际 %s", tt.input, tt.expected, result)
		}
	}
}

// TestIsRetryableStatus 测试状态码可重试性判断。
func TestIsRetryableStatus(t *testing.T) {
	tests := []struct {
		statusCode int
		retryable  bool
	}{
		{200, false},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
	}

	for _, tt := range tests {
		result := isRetryableStatus(tt.statusCode)
		if result != tt.retryable {
			t.Errorf("isRetryableStatus(%d): 期望 %v, 实际 %v", tt.statusCode, tt.retryable, result)
		}
	}
}

// TestGetDefaultMessage 测试默认错误消息生成。
func TestGetDefaultMessage(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   string
	}{
		{400, "Azure OpenAI 请求参数无效"},
		{401, "Azure OpenAI 认证失败"},
		{403, "Azure OpenAI 权限不足"},
		{404, "Azure OpenAI 资源未找到"},
		{429, "Azure OpenAI 请求频率超限"},
		{500, "Azure OpenAI 内部服务器错误"},
		{502, "Azure OpenAI 网关错误"},
		{503, "Azure OpenAI 服务不可用"},
		{504, "Azure OpenAI 网关超时"},
		{418, "Azure OpenAI 请求失败"}, // 未知状态码
	}

	for _, tt := range tests {
		result := getDefaultMessage(tt.statusCode)
		if result != tt.expected {
			t.Errorf("getDefaultMessage(%d): 期望 '%s', 实际 '%s'", tt.statusCode, tt.expected, result)
		}
	}
}

// TestGetErrorCodeFromStatus 测试状态码到错误码的映射。
func TestGetErrorCodeFromStatus(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   string
	}{
		{400, string(domainerror.CodeInvalidRequest)},
		{401, string(domainerror.CodeUnauthorized)},
		{403, string(domainerror.CodeBadRequest)},
		{404, string(domainerror.CodeBadRequest)},
		{429, string(domainerror.CodeRateLimited)},
		{500, string(domainerror.CodeBackendError)},
		{502, string(domainerror.CodeBackendError)},
		{503, string(domainerror.CodeBackendError)},
		{504, string(domainerror.CodeBackendError)},
		{418, string(domainerror.CodeUnknown)},
	}

	for _, tt := range tests {
		result := getErrorCodeFromStatus(tt.statusCode)
		if result != tt.expected {
			t.Errorf("getErrorCodeFromStatus(%d): 期望 %s, 实际 %s", tt.statusCode, tt.expected, result)
		}
	}
}

// TestErrorConverter_NilLogger 测试 nil 日志记录器处理。
func TestErrorConverter_NilLogger(t *testing.T) {
	// 不应 panic
	converter := NewErrorConverter(nil)

	body := []byte(`{"error":{"code":"400","message":"test"}}`)
	result := converter.Convert(http.StatusBadRequest, body)

	if result == nil {
		t.Error("nil logger 不应影响转换结果")
	}
}
