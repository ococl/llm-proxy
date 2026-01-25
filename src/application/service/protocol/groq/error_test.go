package groq

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

// TestErrorConverter_Convert 测试 Groq 错误转换器的 Convert 方法。
func TestErrorConverter_Convert(t *testing.T) {
	logger := &MockLogger{}
	converter := NewErrorConverter(logger)

	t.Run("处理原始响应体作为消息", func(t *testing.T) {
		body := []byte("Groq API error: model overloaded")
		result := converter.Convert(http.StatusTooManyRequests, body)

		if result == nil {
			t.Fatal("期望非 nil 结果")
		}

		if result.HTTPStatus != http.StatusTooManyRequests {
			t.Errorf("期望状态码 %d, 实际 %d", http.StatusTooManyRequests, result.HTTPStatus)
		}

		if result.Message != "Groq API error: model overloaded" {
			t.Errorf("期望原始消息, 实际 '%s'", result.Message)
		}

		if result.Provider != "groq" {
			t.Errorf("期望 provider 'groq', 实际 '%s'", result.Provider)
		}

		if !result.Retryable {
			t.Error("429 错误应可重试")
		}
	})

	t.Run("处理空响应体使用默认消息", func(t *testing.T) {
		result := converter.Convert(http.StatusBadRequest, nil)

		if result == nil {
			t.Fatal("期望非 nil 结果")
		}

		if result.Code != domainerror.CodeInvalidRequest {
			t.Errorf("空响应应使用状态码映射, 期望 %s, 实际 %s", domainerror.CodeInvalidRequest, result.Code)
		}

		if result.Message != "Groq 请求参数无效" {
			t.Errorf("期望默认中文消息, 实际 '%s'", result.Message)
		}
	})

	t.Run("处理认证错误", func(t *testing.T) {
		body := []byte("Invalid API key")
		result := converter.Convert(http.StatusUnauthorized, body)

		if result.Code != domainerror.CodeUnauthorized {
			t.Errorf("401 应映射到 %s, 实际 %s", domainerror.CodeUnauthorized, result.Code)
		}

		if result.Retryable {
			t.Error("401 错误不应重试")
		}
	})

	t.Run("处理 Flex Tier 容量超出 (498)", func(t *testing.T) {
		body := []byte("Flex tier capacity exceeded")
		result := converter.Convert(498, body)

		if result.Code != domainerror.CodeBackendError {
			t.Errorf("498 应映射到 %s, 实际 %s", domainerror.CodeBackendError, result.Code)
		}

		if !result.Retryable {
			t.Error("498 错误应可重试")
		}

		if result.Message != "Flex tier capacity exceeded" {
			t.Errorf("期望原始消息, 实际 '%s'", result.Message)
		}
	})

	t.Run("处理请求已取消 (499)", func(t *testing.T) {
		body := []byte("Request cancelled by client")
		result := converter.Convert(499, body)

		if result.Code != domainerror.CodeInvalidRequest {
			t.Errorf("499 应映射到 %s, 实际 %s", domainerror.CodeInvalidRequest, result.Code)
		}

		if result.Retryable {
			t.Error("499 错误不应重试")
		}

		if result.Message != "Request cancelled by client" {
			t.Errorf("期望原始消息, 实际 '%s'", result.Message)
		}
	})

	t.Run("处理 500 服务器错误", func(t *testing.T) {
		body := []byte("Internal server error")
		result := converter.Convert(http.StatusInternalServerError, body)

		if result.Code != domainerror.CodeBackendError {
			t.Errorf("500 应映射到 %s, 实际 %s", domainerror.CodeBackendError, result.Code)
		}

		if !result.Retryable {
			t.Error("500 错误应可重试")
		}
	})

	t.Run("处理 502 网关错误", func(t *testing.T) {
		body := []byte("Bad Gateway")
		result := converter.Convert(http.StatusBadGateway, body)

		if result.Code != domainerror.CodeBackendError {
			t.Errorf("502 应映射到 %s, 实际 %s", domainerror.CodeBackendError, result.Code)
		}

		if !result.Retryable {
			t.Error("502 错误应可重试")
		}
	})

	t.Run("处理 503 服务不可用", func(t *testing.T) {
		body := []byte("Service unavailable")
		result := converter.Convert(http.StatusServiceUnavailable, body)

		if result.Code != domainerror.CodeBackendError {
			t.Errorf("503 应映射到 %s, 实际 %s", domainerror.CodeBackendError, result.Code)
		}

		if !result.Retryable {
			t.Error("503 错误应可重试")
		}
	})

	t.Run("处理 504 网关超时", func(t *testing.T) {
		body := []byte("Gateway timeout")
		result := converter.Convert(http.StatusGatewayTimeout, body)

		if result.Code != domainerror.CodeBackendError {
			t.Errorf("504 应映射到 %s, 实际 %s", domainerror.CodeBackendError, result.Code)
		}

		if !result.Retryable {
			t.Error("504 错误应可重试")
		}
	})

	t.Run("记录调试日志", func(t *testing.T) {
		logger := &MockLogger{}
		converter := NewErrorConverter(logger)

		body := []byte("Rate limited")
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
		{types.ProtocolGroq, true},
		{types.ProtocolOpenAI, false},
		{types.ProtocolAnthropic, false},
		{types.ProtocolGoogle, false},
		{types.ProtocolAzure, false},
		{types.ProtocolDeepSeek, false},
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

	if converter.Protocol() != types.ProtocolGroq {
		t.Errorf("期望 Protocol Groq, 实际 %s", converter.Protocol())
	}
}

// TestErrorConverter_Name 测试 Name 方法。
func TestErrorConverter_Name(t *testing.T) {
	converter := NewErrorConverter(nil)

	if converter.Name() != "GroqErrorConverter" {
		t.Errorf("期望名称 'GroqErrorConverter', 实际 '%s'", converter.Name())
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
		{498, string(domainerror.CodeBackendError)},   // Flex Tier Capacity Exceeded
		{499, string(domainerror.CodeInvalidRequest)}, // Request Cancelled
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
		{498, true},  // Flex Tier Capacity Exceeded 可重试
		{499, false}, // Request Cancelled 不可重试
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
		{400, "Groq 请求参数无效"},
		{401, "Groq 认证失败"},
		{403, "Groq 权限不足"},
		{404, "Groq 资源未找到"},
		{429, "Groq 请求频率超限"},
		{498, "Groq Flex Tier 容量已超出"},
		{499, "Groq 请求已取消"},
		{500, "Groq 内部服务器错误"},
		{502, "Groq 网关错误"},
		{503, "Groq 服务不可用"},
		{504, "Groq 网关超时"},
		{418, "Groq 请求失败"},
	}

	for _, tt := range tests {
		result := getDefaultMessage(tt.statusCode)
		if result != tt.expected {
			t.Errorf("getDefaultMessage(%d): 期望 '%s', 实际 '%s'", tt.statusCode, tt.expected, result)
		}
	}
}

// TestErrorConverter_NilLogger 测试 nil 日志记录器处理。
func TestErrorConverter_NilLogger(t *testing.T) {
	// 不应 panic
	converter := NewErrorConverter(nil)

	body := []byte("test error")
	result := converter.Convert(http.StatusBadRequest, body)

	if result == nil {
		t.Error("nil logger 不应影响转换结果")
	}
}

// TestErrorConverter_GroqOnlyProtocol 测试 Groq 协议的独占性。
func TestErrorConverter_GroqOnlyProtocol(t *testing.T) {
	converter := NewErrorConverter(nil)

	// Groq 应该是唯一支持 Groq 协议的转换器
	if !converter.Supports(types.ProtocolGroq) {
		t.Error("应支持 Groq 协议")
	}

	// 其他所有协议都不应被支持
	otherProtocols := []types.Protocol{
		types.ProtocolOpenAI,
		types.ProtocolAnthropic,
		types.ProtocolGoogle,
		types.ProtocolAzure,
		types.ProtocolDeepSeek,
		types.ProtocolMistral,
		types.ProtocolCohere,
		types.ProtocolCustom,
	}

	for _, proto := range otherProtocols {
		if converter.Supports(proto) {
			t.Errorf("Groq 错误转换器不应支持 %s 协议", proto)
		}
	}
}
