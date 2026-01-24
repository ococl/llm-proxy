package anthropic

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

// TestErrorConverter_Convert 测试 Anthropic 错误转换器的 Convert 方法。
func TestErrorConverter_Convert(t *testing.T) {
	logger := &MockLogger{}
	converter := NewErrorConverter(logger)

	t.Run("解析 Anthropic 标准错误格式", func(t *testing.T) {
		body := []byte(`{"type":"error","error":{"type":"invalid_request","message":"Invalid request parameters"}}`)
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

		if result.Provider != "anthropic" {
			t.Errorf("期望 provider 'anthropic', 实际 '%s'", result.Provider)
		}

		if result.Retryable {
			t.Error("400 错误不应重试")
		}
	})

	t.Run("处理 authentication 错误", func(t *testing.T) {
		body := []byte(`{"type":"error","error":{"type":"authentication","message":"Invalid API key"}}`)
		result := converter.Convert(http.StatusUnauthorized, body)

		if result.Code != domainerror.CodeUnauthorized {
			t.Errorf("authentication 应映射到 %s, 实际 %s", domainerror.CodeUnauthorized, result.Code)
		}

		if result.Retryable {
			t.Error("401 错误不应重试")
		}
	})

	t.Run("处理 permission 错误", func(t *testing.T) {
		body := []byte(`{"type":"error","error":{"type":"permission","message":"Permission denied"}}`)
		result := converter.Convert(http.StatusForbidden, body)

		if result.Code != domainerror.CodeBadRequest {
			t.Errorf("permission 应映射到 %s, 实际 %s", domainerror.CodeBadRequest, result.Code)
		}

		if result.Retryable {
			t.Error("403 错误不应重试")
		}
	})

	t.Run("处理 not_found 错误", func(t *testing.T) {
		body := []byte(`{"type":"error","error":{"type":"not_found","message":"Resource not found"}}`)
		result := converter.Convert(http.StatusNotFound, body)

		if result.Code != domainerror.CodeBadRequest {
			t.Errorf("not_found 应映射到 %s, 实际 %s", domainerror.CodeBadRequest, result.Code)
		}
	})

	t.Run("处理 rate_limit 错误", func(t *testing.T) {
		body := []byte(`{"type":"error","error":{"type":"rate_limit","message":"Rate limit exceeded"}}`)
		result := converter.Convert(http.StatusTooManyRequests, body)

		if result.Code != domainerror.CodeRateLimited {
			t.Errorf("rate_limit 应映射到 %s, 实际 %s", domainerror.CodeRateLimited, result.Code)
		}

		if !result.Retryable {
			t.Error("429 错误应可重试")
		}
	})

	t.Run("处理 overloaded_error 错误", func(t *testing.T) {
		body := []byte(`{"type":"error","error":{"type":"overloaded_error","message":"Service overloaded"}}`)
		result := converter.Convert(http.StatusServiceUnavailable, body)

		if result.Code != domainerror.CodeBackendError {
			t.Errorf("overloaded_error 应映射到 %s, 实际 %s", domainerror.CodeBackendError, result.Code)
		}

		if !result.Retryable {
			t.Error("overloaded_error 应可重试")
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

		if result.Message != "Anthropic 请求参数无效" {
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

		body := []byte(`{"type":"error","error":{"type":"rate_limit","message":"Rate limited"}}`)
		converter.Convert(http.StatusTooManyRequests, body)

		if len(logger.debugMessages) != 1 {
			t.Errorf("期望 1 条调试日志, 实际 %d 条", len(logger.debugMessages))
		}
	})

	t.Run("处理 500 服务器错误", func(t *testing.T) {
		// Anthropic 使用 overloaded_error 表示服务器过载/错误
		body := []byte(`{"type":"error","error":{"type":"overloaded_error","message":"Internal error"}}`)
		result := converter.Convert(http.StatusInternalServerError, body)

		if result.Code != domainerror.CodeBackendError {
			t.Errorf("500 应映射到 %s, 实际 %s", domainerror.CodeBackendError, result.Code)
		}

		if !result.Retryable {
			t.Error("500 错误应可重试")
		}
	})

	t.Run("处理 502 网关错误", func(t *testing.T) {
		body := []byte(`{"type":"error","error":{"type":"overloaded_error","message":"Bad Gateway"}}`)
		result := converter.Convert(http.StatusBadGateway, body)

		if result.Code != domainerror.CodeBackendError {
			t.Errorf("502 应映射到 %s, 实际 %s", domainerror.CodeBackendError, result.Code)
		}

		if !result.Retryable {
			t.Error("502 错误应可重试")
		}
	})

	t.Run("处理 503 服务不可用", func(t *testing.T) {
		body := []byte(`{"type":"error","error":{"type":"overloaded_error","message":"Service unavailable"}}`)
		result := converter.Convert(http.StatusServiceUnavailable, body)

		if result.Code != domainerror.CodeBackendError {
			t.Errorf("503 应映射到 %s, 实际 %s", domainerror.CodeBackendError, result.Code)
		}

		if !result.Retryable {
			t.Error("503 错误应可重试")
		}
	})

	t.Run("处理 504 网关超时", func(t *testing.T) {
		body := []byte(`{"type":"error","error":{"type":"overloaded_error","message":"Gateway timeout"}}`)
		result := converter.Convert(http.StatusGatewayTimeout, body)

		if result.Code != domainerror.CodeBackendError {
			t.Errorf("504 应映射到 %s, 实际 %s", domainerror.CodeBackendError, result.Code)
		}

		if !result.Retryable {
			t.Error("504 错误应可重试")
		}
	})

	t.Run("处理未知错误类型", func(t *testing.T) {
		body := []byte(`{"type":"error","error":{"type":"unknown_error","message":"Unknown error"}}`)
		result := converter.Convert(http.StatusBadRequest, body)

		if result.Code != domainerror.CodeUnknown {
			t.Errorf("unknown_error 应映射到 %s, 实际 %s", domainerror.CodeUnknown, result.Code)
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
		{types.ProtocolAnthropic, true},
		{types.ProtocolOpenAI, false},
		{types.ProtocolGoogle, false},
		{types.ProtocolAzure, false},
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

	if converter.Protocol() != types.ProtocolAnthropic {
		t.Errorf("期望 Protocol Anthropic, 实际 %s", converter.Protocol())
	}
}

// TestErrorConverter_Name 测试 Name 方法。
func TestErrorConverter_Name(t *testing.T) {
	converter := NewErrorConverter(nil)

	if converter.Name() != "AnthropicErrorConverter" {
		t.Errorf("期望名称 'AnthropicErrorConverter', 实际 '%s'", converter.Name())
	}
}

// TestGetErrorCodeFromType 测试 Anthropic 错误类型转换逻辑。
func TestGetErrorCodeFromType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"invalid_request", string(domainerror.CodeInvalidRequest)},
		{"authentication", string(domainerror.CodeUnauthorized)},
		{"permission", string(domainerror.CodeBadRequest)},
		{"not_found", string(domainerror.CodeBadRequest)},
		{"rate_limit", string(domainerror.CodeRateLimited)},
		{"overloaded_error", string(domainerror.CodeBackendError)},
		{"unknown_type", string(domainerror.CodeUnknown)},
	}

	for _, tt := range tests {
		result := getErrorCodeFromType(tt.input)
		if result != tt.expected {
			t.Errorf("getErrorCodeFromType(%s): 期望 %s, 实际 %s", tt.input, tt.expected, result)
		}
	}
}

// TestIsRetryableStatus 测试 Anthropic 状态码可重试性判断。
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

// TestGetDefaultMessage 测试 Anthropic 默认错误消息生成。
func TestGetDefaultMessage(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   string
	}{
		{400, "Anthropic 请求参数无效"},
		{401, "Anthropic 认证失败"},
		{403, "Anthropic 权限不足"},
		{404, "Anthropic 资源未找到"},
		{429, "Anthropic 请求频率超限"},
		{500, "Anthropic 内部服务器错误"},
		{502, "Anthropic 网关错误"},
		{503, "Anthropic 服务不可用"},
		{504, "Anthropic 网关超时"},
		{418, "Anthropic 请求失败"},
	}

	for _, tt := range tests {
		result := getDefaultMessage(tt.statusCode)
		if result != tt.expected {
			t.Errorf("getDefaultMessage(%d): 期望 '%s', 实际 '%s'", tt.statusCode, tt.expected, result)
		}
	}
}

// TestGetErrorCodeFromStatus 测试 Anthropic 状态码到错误码的映射。
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

// TestGetErrorTypeCodeFromStatus 测试 Anthropic 状态码到错误类型代码的映射。
func TestGetErrorTypeCodeFromStatus(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   string
	}{
		{400, "invalid_request"},
		{401, "authentication"},
		{403, "permission"},
		{404, "not_found"},
		{429, "rate_limit"},
		{500, "overloaded_error"},
		{502, "overloaded_error"},
		{503, "overloaded_error"},
		{504, "overloaded_error"},
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

	body := []byte(`{"type":"error","error":{"type":"invalid_request","message":"test"}}`)
	result := converter.Convert(http.StatusBadRequest, body)

	if result == nil {
		t.Error("nil logger 不应影响转换结果")
	}
}

// TestErrorConverter_AnthropicOnlyProtocol 测试 Anthropic 协议的独占性。
func TestErrorConverter_AnthropicOnlyProtocol(t *testing.T) {
	converter := NewErrorConverter(nil)

	// Anthropic 应该是唯一支持 Anthropic 协议的转换器
	if !converter.Supports(types.ProtocolAnthropic) {
		t.Error("应支持 Anthropic 协议")
	}

	// 其他所有协议都不应被支持
	otherProtocols := []types.Protocol{
		types.ProtocolOpenAI,
		types.ProtocolGoogle,
		types.ProtocolAzure,
		types.ProtocolDeepSeek,
		types.ProtocolGroq,
		types.ProtocolMistral,
		types.ProtocolCohere,
		types.ProtocolCustom,
	}

	for _, proto := range otherProtocols {
		if converter.Supports(proto) {
			t.Errorf("Anthropic 错误转换器不应支持 %s 协议", proto)
		}
	}
}
