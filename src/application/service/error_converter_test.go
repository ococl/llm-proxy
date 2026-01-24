package service

import (
	"testing"

	domainerror "llm-proxy/domain/error"
	"llm-proxy/domain/types"
)

func TestErrorResponseConverter_ConvertError(t *testing.T) {
	converter := NewErrorResponseConverter()

	t.Run("nil error returns nil", func(t *testing.T) {
		result := converter.ConvertError(nil, "backend-1", "req-123")
		if result != nil {
			t.Error("Expected nil for nil error")
		}
	})

	t.Run("converts proxy error", func(t *testing.T) {
		proxyErr := domainerror.New(domainerror.ErrorTypeClient, domainerror.CodeBadRequest, "测试错误")
		result := converter.ConvertError(proxyErr, "backend-1", "req-123")

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if result.Error.Code != "BAD_REQUEST" {
			t.Errorf("Expected code BAD_REQUEST, got %s", result.Error.Code)
		}

		if result.Error.Message != "测试错误" {
			t.Errorf("Expected message '测试错误', got %s", result.Error.Message)
		}

		if result.Error.Type != "client" {
			t.Errorf("Expected type 'client', got %s", result.Error.Type)
		}

		if result.ReqID != "req-123" {
			t.Errorf("Expected reqID 'req-123', got %s", result.ReqID)
		}

		if result.Backend != "backend-1" {
			t.Errorf("Expected backend 'backend-1', got %s", result.Backend)
		}
	})

	t.Run("converts generic error", func(t *testing.T) {
		genericErr := &customError{msg: "generic error"}
		result := converter.ConvertError(genericErr, "backend-1", "req-123")

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if result.Error.Code != "INTERNAL_ERROR" {
			t.Errorf("Expected code INTERNAL_ERROR, got %s", result.Error.Code)
		}

		if result.Error.Message != "generic error" {
			t.Errorf("Expected message 'generic error', got %s", result.Error.Message)
		}
	})
}

type customError struct {
	msg string
}

func (e *customError) Error() string {
	return e.msg
}

func TestErrorResponseConverter_ConvertAPIResponse(t *testing.T) {
	converter := NewErrorResponseConverter()

	t.Run("parses OpenAI error", func(t *testing.T) {
		body := []byte(`{"error":{"code":"invalid_api_key","message":"Invalid API key","type":"authentication_error"}}`)
		result := converter.ConvertAPIResponse(401, body, "backend-1", "req-123")

		if result.Error.Code != "invalid_api_key" {
			t.Errorf("Expected code 'invalid_api_key', got %s", result.Error.Code)
		}

		if result.Error.Message != "Invalid API key" {
			t.Errorf("Expected message 'Invalid API key', got %s", result.Error.Message)
		}

		if result.Error.Type != "authentication_error" {
			t.Errorf("Expected type 'authentication_error', got %s", result.Error.Type)
		}
	})

	t.Run("parses Anthropic error", func(t *testing.T) {
		body := []byte(`{"error":{"type":"invalid_request","message":"Missing required parameter"}}`)
		result := converter.ConvertAPIResponse(400, body, "backend-2", "req-456")

		if result.Error.Code != "INVALID REQUEST" { // Anthropic 类型没有 code，使用 normalize 后结果
			t.Errorf("Expected code 'INVALID REQUEST', got %s", result.Error.Code)
		}

		if result.Error.Message != "Missing required parameter" {
			t.Errorf("Expected message 'Missing required parameter', got %s", result.Error.Message)
		}

		if result.Error.Type != "invalid_request" {
			t.Errorf("Expected type 'invalid_request', got %s", result.Error.Type)
		}
	})

	t.Run("falls back to status code error", func(t *testing.T) {
		body := []byte(`invalid json`)
		result := converter.ConvertAPIResponse(429, body, "backend-3", "req-789")

		if result.Error.Code != "RATE_LIMIT_ERROR" {
			t.Errorf("Expected code 'RATE_LIMIT_ERROR', got %s", result.Error.Code)
		}

		if result.Error.Message != "请求过于频繁，请稍后重试" {
			t.Errorf("Expected Chinese message, got %s", result.Error.Message)
		}
	})
}

func TestErrorResponseConverter_errorFromStatusCode(t *testing.T) {
	converter := NewErrorResponseConverter()

	tests := []struct {
		statusCode   int
		expectedCode string
		expectedType string
	}{
		{400, "INVALID_REQUEST", "invalid_request_error"},
		{401, "AUTHENTICATION_ERROR", "authentication_error"},
		{403, "PERMISSION_DENIED", "permission_denied_error"},
		{404, "NOT_FOUND", "not_found_error"},
		{429, "RATE_LIMIT_ERROR", "rate_limit_error"},
		{503, "SERVICE_UNAVAILABLE", "service_unavailable_error"},
		{500, "INTERNAL_ERROR", "internal_server_error"},
		{502, "INTERNAL_ERROR", "internal_server_error"},
	}

	for _, tt := range tests {
		result := converter.errorFromStatusCode(tt.statusCode)

		if result.Code != tt.expectedCode {
			t.Errorf("Status %d: expected code %s, got %s", tt.statusCode, tt.expectedCode, result.Code)
		}

		if result.Type != tt.expectedType {
			t.Errorf("Status %d: expected type %s, got %s", tt.statusCode, tt.expectedType, result.Type)
		}
	}
}

func TestErrorResponseConverter_normalizeErrorCode(t *testing.T) {
	converter := NewErrorResponseConverter()

	tests := []struct {
		code      string
		errorType string
		expected  string
	}{
		{"custom_code", "some_type", "custom_code"},
		{"", "invalid_request_error", "INVALID_REQUEST"},
		{"", "authentication_error", "AUTHENTICATION_ERROR"},
		{"", "rate_limit_error", "RATE_LIMIT_ERROR"},
		{"", "context_length_exceeded", "CONTEXT_LENGTH_EXCEEDED"}, // 保持下划线
		{"", "unknown_type", "UNKNOWN TYPE"},
	}

	for _, tt := range tests {
		result := converter.normalizeErrorCode(tt.code, tt.errorType)
		if result != tt.expected {
			t.Errorf("normalizeErrorCode(%s, %s): expected %s, got %s", tt.code, tt.errorType, tt.expected, result)
		}
	}
}

func TestErrorResponseConverter_IsRetryableError(t *testing.T) {
	converter := NewErrorResponseConverter()

	tests := []struct {
		statusCode int
		errType    string
		retryable  bool
	}{
		{500, "internal_server_error", true},
		{503, "service_unavailable_error", true},
		{429, "rate_limit_error", true},
		{401, "authentication_error", false},
		{403, "permission_denied_error", false},
		{400, "invalid_request_error", false},
	}

	for _, tt := range tests {
		apiErr := &APIError{Type: tt.errType}
		result := converter.IsRetryableError(tt.statusCode, apiErr)
		if result != tt.retryable {
			t.Errorf("IsRetryableError(%d, %s): expected %v, got %v", tt.statusCode, tt.errType, tt.retryable, result)
		}
	}
}

func TestProtocol_IsValid(t *testing.T) {
	tests := []struct {
		protocol types.Protocol
		valid    bool
	}{
		{types.ProtocolOpenAI, true},
		{types.ProtocolAnthropic, true},
		{types.ProtocolGoogle, true},
		{types.ProtocolAzure, true},
		{types.ProtocolDeepSeek, true},
		{types.ProtocolGroq, true},
		{types.ProtocolMistral, true},
		{types.ProtocolCohere, true},
		{types.ProtocolCustom, true},
		{types.Protocol("unknown"), false},
		{types.Protocol(""), false},
	}

	for _, tt := range tests {
		result := tt.protocol.IsValid()
		if result != tt.valid {
			t.Errorf("Protocol(%s).IsValid(): expected %v, got %v", tt.protocol, tt.valid, result)
		}
	}
}

func TestProtocol_IsOpenAICompatible(t *testing.T) {
	tests := []struct {
		protocol   types.Protocol
		compatible bool
	}{
		{types.ProtocolOpenAI, true},
		{types.ProtocolAzure, true},
		{types.ProtocolDeepSeek, true},
		{types.ProtocolGroq, true},
		{types.ProtocolAnthropic, false},
		{types.ProtocolGoogle, false},
		{types.Protocol("custom"), false},
	}

	for _, tt := range tests {
		result := tt.protocol.IsOpenAICompatible()
		if result != tt.compatible {
			t.Errorf("Protocol(%s).IsOpenAICompatible(): expected %v, got %v", tt.protocol, tt.compatible, result)
		}
	}
}

func TestProtocol_IsAnthropicCompatible(t *testing.T) {
	tests := []struct {
		protocol   types.Protocol
		compatible bool
	}{
		{types.ProtocolAnthropic, true},
		{types.ProtocolOpenAI, false},
		{types.ProtocolGoogle, false},
		{types.Protocol("custom"), false},
	}

	for _, tt := range tests {
		result := tt.protocol.IsAnthropicCompatible()
		if result != tt.compatible {
			t.Errorf("Protocol(%s).IsAnthropicCompatible(): expected %v, got %v", tt.protocol, tt.compatible, result)
		}
	}
}

func TestProtocol_RequiresSystemPromptField(t *testing.T) {
	if !types.ProtocolAnthropic.RequiresSystemPromptField() {
		t.Error("Anthropic should require system prompt field")
	}

	if types.ProtocolOpenAI.RequiresSystemPromptField() {
		t.Error("OpenAI should not require system prompt field")
	}
}

func TestProtocol_SupportsTools(t *testing.T) {
	tests := []struct {
		protocol types.Protocol
		supports bool
	}{
		{types.ProtocolOpenAI, true},
		{types.ProtocolAnthropic, true},
		{types.ProtocolAzure, true},
		{types.ProtocolGoogle, true},    // Google Vertex AI 支持 Function Calling
		{types.ProtocolMistral, true},   // Mistral 支持工具调用
		{types.ProtocolCohere, true},    // Cohere 支持工具调用
		{types.ProtocolDeepSeek, false}, // DeepSeek 暂不支持工具调用
		{types.ProtocolGroq, false},     // Groq 暂不支持工具调用
	}

	for _, tt := range tests {
		result := tt.protocol.SupportsTools()
		if result != tt.supports {
			t.Errorf("Protocol(%s).SupportsTools(): expected %v, got %v", tt.protocol, tt.supports, result)
		}
	}
}

func TestProtocol_GetStreamingFormat(t *testing.T) {
	if types.ProtocolOpenAI.GetStreamingFormat() != types.StreamingFormatSSE {
		t.Error("OpenAI should use SSE streaming format")
	}

	if types.ProtocolAnthropic.GetStreamingFormat() != types.StreamingFormatSSE {
		t.Error("Anthropic should use SSE streaming format")
	}
}
