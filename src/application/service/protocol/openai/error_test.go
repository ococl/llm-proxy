package openai

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

// TestErrorConverter_Convert 测试 OpenAI 错误转换器的 Convert 方法。
func TestErrorConverter_Convert(t *testing.T) {
	logger := &MockLogger{}
	converter := NewErrorConverter(logger)

	t.Run("解析 OpenAI 标准错误格式", func(t *testing.T) {
		body := []byte(`{"error":{"message":"Invalid request parameters","type":"invalid_request_error"}}`)
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

		if result.Provider != "openai" {
			t.Errorf("期望 provider 'openai', 实际 '%s'", result.Provider)
		}

		if result.Retryable {
			t.Error("400 错误不应重试")
		}
	})

	t.Run("处理 authentication_error 错误", func(t *testing.T) {
		body := []byte(`{"error":{"message":"Invalid API key","type":"authentication_error"}}`)
		result := converter.Convert(http.StatusUnauthorized, body)

		if result.Code != domainerror.CodeUnauthorized {
			t.Errorf("authentication_error 应映射到 %s, 实际 %s", domainerror.CodeUnauthorized, result.Code)
		}

		if result.Retryable {
			t.Error("401 错误不应重试")
		}
	})

	t.Run("处理 permission_error 错误", func(t *testing.T) {
		body := []byte(`{"error":{"message":"Permission denied","type":"permission_error"}}`)
		result := converter.Convert(http.StatusForbidden, body)

		if result.Code != domainerror.CodeBadRequest {
			t.Errorf("permission_error 应映射到 %s, 实际 %s", domainerror.CodeBadRequest, result.Code)
		}

		if result.Retryable {
			t.Error("403 错误不应重试")
		}
	})

	t.Run("处理 not_found_error 错误", func(t *testing.T) {
		body := []byte(`{"error":{"message":"Resource not found","type":"not_found_error"}}`)
		result := converter.Convert(http.StatusNotFound, body)

		if result.Code != domainerror.CodeBadRequest {
			t.Errorf("not_found_error 应映射到 %s, 实际 %s", domainerror.CodeBadRequest, result.Code)
		}
	})

	t.Run("处理 rate_limit_error 错误", func(t *testing.T) {
		body := []byte(`{"error":{"message":"Rate limit exceeded","type":"rate_limit_error"}}`)
		result := converter.Convert(http.StatusTooManyRequests, body)

		if result.Code != domainerror.CodeRateLimited {
			t.Errorf("rate_limit_error 应映射到 %s, 实际 %s", domainerror.CodeRateLimited, result.Code)
		}

		if !result.Retryable {
			t.Error("429 错误应可重试")
		}
	})

	t.Run("处理 internal_server_error 错误", func(t *testing.T) {
		body := []byte(`{"error":{"message":"Internal server error","type":"internal_server_error"}}`)
		result := converter.Convert(http.StatusInternalServerError, body)

		if result.Code != domainerror.CodeBackendError {
			t.Errorf("500 应映射到 %s, 实际 %s", domainerror.CodeBackendError, result.Code)
		}

		if !result.Retryable {
			t.Error("500 错误应可重试")
		}
	})

	t.Run("处理 invalid_api_key 别名字段", func(t *testing.T) {
		body := []byte(`{"error":{"message":"Invalid API key","type":"invalid_api_key"}}`)
		result := converter.Convert(http.StatusUnauthorized, body)

		if result.Code != domainerror.CodeUnauthorized {
			t.Errorf("invalid_api_key 应映射到 %s, 实际 %s", domainerror.CodeUnauthorized, result.Code)
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

		if result.Message != "OpenAI 请求参数无效" {
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

		body := []byte(`{"error":{"message":"Rate limited","type":"rate_limit_error"}}`)
		converter.Convert(http.StatusTooManyRequests, body)

		if len(logger.debugMessages) != 1 {
			t.Errorf("期望 1 条调试日志, 实际 %d 条", len(logger.debugMessages))
		}
	})

	t.Run("处理 502 网关错误", func(t *testing.T) {
		body := []byte(`{"error":{"message":"Bad Gateway","type":"internal_server_error"}}`)
		result := converter.Convert(http.StatusBadGateway, body)

		if result.Code != domainerror.CodeBackendError {
			t.Errorf("502 应映射到 %s, 实际 %s", domainerror.CodeBackendError, result.Code)
		}

		if !result.Retryable {
			t.Error("502 错误应可重试")
		}
	})

	t.Run("处理 503 服务不可用", func(t *testing.T) {
		body := []byte(`{"error":{"message":"Service unavailable","type":"internal_server_error"}}`)
		result := converter.Convert(http.StatusServiceUnavailable, body)

		if result.Code != domainerror.CodeBackendError {
			t.Errorf("503 应映射到 %s, 实际 %s", domainerror.CodeBackendError, result.Code)
		}

		if !result.Retryable {
			t.Error("503 错误应可重试")
		}
	})

	t.Run("处理 504 网关超时", func(t *testing.T) {
		body := []byte(`{"error":{"message":"Gateway timeout","type":"internal_server_error"}}`)
		result := converter.Convert(http.StatusGatewayTimeout, body)

		if result.Code != domainerror.CodeBackendError {
			t.Errorf("504 应映射到 %s, 实际 %s", domainerror.CodeBackendError, result.Code)
		}

		if !result.Retryable {
			t.Error("504 错误应可重试")
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
		{types.ProtocolOpenAI, true},
		{types.ProtocolAnthropic, false},
		{types.ProtocolGoogle, false},
		{types.ProtocolAzure, true},    // Azure OpenAI 兼容 OpenAI 格式
		{types.ProtocolDeepSeek, true}, // OpenAI 兼容
		{types.ProtocolGroq, true},     // OpenAI 兼容
		{types.ProtocolMistral, true},  // OpenAI 兼容
		{types.ProtocolCohere, true},   // OpenAI 兼容
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

	if converter.Protocol() != types.ProtocolOpenAI {
		t.Errorf("期望 Protocol OpenAI, 实际 %s", converter.Protocol())
	}
}

// TestErrorConverter_Name 测试 Name 方法。
func TestErrorConverter_Name(t *testing.T) {
	converter := NewErrorConverter(nil)

	if converter.Name() != "OpenAIErrorConverter" {
		t.Errorf("期望名称 'OpenAIErrorConverter', 实际 '%s'", converter.Name())
	}
}

// TestGetErrorCodeFromType 测试错误类型转换逻辑。
func TestGetErrorCodeFromType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"invalid_request_error", string(domainerror.CodeInvalidRequest)},
		{"authentication_error", string(domainerror.CodeUnauthorized)},
		{"invalid_api_key", string(domainerror.CodeUnauthorized)},
		{"permission_error", string(domainerror.CodeBadRequest)},
		{"not_found_error", string(domainerror.CodeBadRequest)},
		{"rate_limit_error", string(domainerror.CodeRateLimited)},
		{"internal_server_error", string(domainerror.CodeBackendError)},
		{"unknown_type", string(domainerror.CodeUnknown)},
	}

	for _, tt := range tests {
		result := getErrorCodeFromType(tt.input)
		if result != tt.expected {
			t.Errorf("getErrorCodeFromType(%s): 期望 %s, 实际 %s", tt.input, tt.expected, result)
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
		{400, "OpenAI 请求参数无效"},
		{401, "OpenAI 认证失败"},
		{403, "OpenAI 权限不足"},
		{404, "OpenAI 资源未找到"},
		{429, "OpenAI 请求频率超限"},
		{500, "OpenAI 内部服务器错误"},
		{502, "OpenAI 网关错误"},
		{503, "OpenAI 服务不可用"},
		{504, "OpenAI 网关超时"},
		{418, "OpenAI 请求失败"}, // 未知状态码
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

// TestGetErrorTypeCodeFromStatus 测试状态码到错误类型代码的映射。
func TestGetErrorTypeCodeFromStatus(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   string
	}{
		{400, "invalid_request_error"},
		{401, "authentication_error"},
		{403, "permission_error"},
		{404, "not_found_error"},
		{429, "rate_limit_error"},
		{500, "internal_server_error"},
		{502, "internal_server_error"},
		{503, "internal_server_error"},
		{504, "internal_server_error"},
		{418, "unknown_error"},
	}

	for _, tt := range tests {
		result := getErrorTypeCodeFromStatus(tt.statusCode)
		if result != tt.expected {
			t.Errorf("getErrorTypeCodeFromStatus(%d): 期望 %s, 实际 %s", tt.statusCode, tt.expected, result)
		}
	}
}

// TestErrorConverter_NilLogger 测试 nil 日志记录器处理。
func TestErrorConverter_NilLogger(t *testing.T) {
	// 不应 panic
	converter := NewErrorConverter(nil)

	body := []byte(`{"error":{"message":"test","type":"invalid_request_error"}}`)
	result := converter.Convert(http.StatusBadRequest, body)

	if result == nil {
		t.Error("nil logger 不应影响转换结果")
	}
}

// TestErrorConverter_OpenAICompatibleProtocols 测试 OpenAI 兼容协议的识别。
func TestErrorConverter_OpenAICompatibleProtocols(t *testing.T) {
	converter := NewErrorConverter(nil)

	compatibleProtocols := []types.Protocol{
		types.ProtocolOpenAI,
		types.ProtocolAzure, // Azure OpenAI 兼容 OpenAI 格式
		types.ProtocolDeepSeek,
		types.ProtocolGroq,
		types.ProtocolMistral,
		types.ProtocolCohere,
	}

	for _, proto := range compatibleProtocols {
		if !converter.Supports(proto) {
			t.Errorf("协议 %s 应被识别为 OpenAI 兼容", proto)
		}
	}

	incompatibleProtocols := []types.Protocol{
		types.ProtocolAnthropic,
		types.ProtocolGoogle,
		types.ProtocolCustom,
	}

	for _, proto := range incompatibleProtocols {
		if converter.Supports(proto) {
			t.Errorf("协议 %s 不应被识别为 OpenAI 兼容", proto)
		}
	}
}
