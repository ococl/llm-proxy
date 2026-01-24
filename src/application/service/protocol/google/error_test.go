package google

import (
	"encoding/json"
	"net/http"
	"testing"

	"llm-proxy/domain/port"
	"llm-proxy/domain/types"

	domainerror "llm-proxy/domain/error"
)

// MockLogger 实现 port.Logger 接口用于测试
type MockLogger struct {
	debugMessages []string
	infoMessages  []string
	errorMessages []string
	warnMessages  []string
	fatalMessages []string
	fields        []map[string]interface{}
	withFields    [][]port.Field
}

func (m *MockLogger) Debug(msg string, fields ...port.Field) {
	m.debugMessages = append(m.debugMessages, msg)
	m.recordFields(fields)
}

func (m *MockLogger) Info(msg string, fields ...port.Field) {
	m.infoMessages = append(m.infoMessages, msg)
	m.recordFields(fields)
}

func (m *MockLogger) Warn(msg string, fields ...port.Field) {
	m.warnMessages = append(m.warnMessages, msg)
	m.recordFields(fields)
}

func (m *MockLogger) Error(msg string, fields ...port.Field) {
	m.errorMessages = append(m.errorMessages, msg)
	m.recordFields(fields)
}

func (m *MockLogger) Fatal(msg string, fields ...port.Field) {
	m.fatalMessages = append(m.fatalMessages, msg)
	m.recordFields(fields)
}

func (m *MockLogger) With(fields ...port.Field) port.Logger {
	m.withFields = append(m.withFields, fields)
	return m
}

func (m *MockLogger) recordFields(fields []port.Field) {
	for _, field := range fields {
		m.fields = append(m.fields, map[string]interface{}{
			"key":   field.Key,
			"value": field.Value,
		})
	}
}

func (m *MockLogger) reset() {
	m.debugMessages = nil
	m.infoMessages = nil
	m.errorMessages = nil
	m.warnMessages = nil
	m.fatalMessages = nil
	m.fields = nil
	m.withFields = nil
}

// TestErrorConverter_NewErrorConverter 测试错误转换器创建
func TestErrorConverter_NewErrorConverter(t *testing.T) {
	t.Run("使用有效日志器创建", func(t *testing.T) {
		mockLogger := &MockLogger{}
		converter := NewErrorConverter(mockLogger)

		if converter == nil {
			t.Fatal("转换器不应为 nil")
		}

		if converter.logger != mockLogger {
			t.Error("日志器未正确设置")
		}
	})

	t.Run("使用 nil 日志器创建时使用 NopLogger", func(t *testing.T) {
		converter := NewErrorConverter(nil)

		if converter == nil {
			t.Fatal("转换器不应为 nil")
		}

		if converter.logger == nil {
			t.Error("日志器不应为 nil")
		}

		// 验证 NopLogger 正常工作
		converter.logger.Debug("test")
		converter.logger.Info("test")
		converter.logger.Error("test")
	})

	t.Run("使用 NopLogger 初始化", func(t *testing.T) {
		converter := NewErrorConverter(&port.NopLogger{})

		if converter == nil {
			t.Fatal("转换器不应为 nil")
		}
	})
}

// TestErrorConverter_Convert 测试错误转换功能
func TestErrorConverter_Convert(t *testing.T) {
	mockLogger := &MockLogger{}
	converter := NewErrorConverter(mockLogger)

	tests := []struct {
		name            string
		statusCode      int
		respBody        []byte
		expectCode      domainerror.ErrorCode
		expectHTTP      int
		expectProvider  string
		expectRetryable bool
	}{
		{
			name:            "空响应返回默认错误",
			statusCode:      http.StatusInternalServerError,
			respBody:        nil,
			expectCode:      domainerror.CodeBackendError,
			expectHTTP:      http.StatusInternalServerError,
			expectProvider:  "google",
			expectRetryable: true,
		},
		{
			name:            "空字节切片返回默认错误",
			statusCode:      http.StatusBadRequest,
			respBody:        []byte{},
			expectCode:      domainerror.CodeInvalidRequest,
			expectHTTP:      http.StatusBadRequest,
			expectProvider:  "google",
			expectRetryable: false,
		},
		{
			name:            "无效 JSON 返回原始错误",
			statusCode:      http.StatusInternalServerError,
			respBody:        []byte("invalid json response"),
			expectCode:      domainerror.CodeBackendError,
			expectHTTP:      http.StatusInternalServerError,
			expectProvider:  "google",
			expectRetryable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger.reset()
			err := converter.Convert(tt.statusCode, tt.respBody)

			if err == nil {
				t.Fatal("错误不应为 nil")
			}

			if err.Code != tt.expectCode {
				t.Errorf("期望错误代码 %v, 实际 %v", tt.expectCode, err.Code)
			}

			if err.HTTPStatus != tt.expectHTTP {
				t.Errorf("期望 HTTP 状态码 %d, 实际 %d", tt.expectHTTP, err.HTTPStatus)
			}

			if err.Provider != tt.expectProvider {
				t.Errorf("期望提供商 %s, 实际 %s", tt.expectProvider, err.Provider)
			}

			if err.Retryable != tt.expectRetryable {
				t.Errorf("期望可重试 %v, 实际 %v", tt.expectRetryable, err.Retryable)
			}
		})
	}
}

// TestErrorConverter_ConvertErrorType 测试错误类型转换
func TestErrorConverter_ConvertErrorType(t *testing.T) {
	tests := []struct {
		name         string
		googleStatus string
		expectCode   domainerror.ErrorCode
	}{
		{
			name:         "INVALID_ARGUMENT 转换为无效请求错误",
			googleStatus: "INVALID_ARGUMENT",
			expectCode:   domainerror.CodeInvalidRequest,
		},
		{
			name:         "INVALID_ARGUMENT 小写也能正确转换",
			googleStatus: "invalid_argument",
			expectCode:   domainerror.CodeInvalidRequest,
		},
		{
			name:         "UNAUTHENTICATED 转换为未授权错误",
			googleStatus: "UNAUTHENTICATED",
			expectCode:   domainerror.CodeUnauthorized,
		},
		{
			name:         "PERMISSION_DENIED 转换为错误请求错误",
			googleStatus: "PERMISSION_DENIED",
			expectCode:   domainerror.CodeBadRequest,
		},
		{
			name:         "NOT_FOUND 转换为错误请求错误",
			googleStatus: "NOT_FOUND",
			expectCode:   domainerror.CodeBadRequest,
		},
		{
			name:         "ALREADY_EXISTS 转换为错误请求错误",
			googleStatus: "ALREADY_EXISTS",
			expectCode:   domainerror.CodeBadRequest,
		},
		{
			name:         "RESOURCE_EXHAUSTED 转换为速率限制错误",
			googleStatus: "RESOURCE_EXHAUSTED",
			expectCode:   domainerror.CodeRateLimited,
		},
		{
			name:         "FAILED_PRECONDITION 转换为错误请求错误",
			googleStatus: "FAILED_PRECONDITION",
			expectCode:   domainerror.CodeBadRequest,
		},
		{
			name:         "ABORTED 转换为后端错误",
			googleStatus: "ABORTED",
			expectCode:   domainerror.CodeBackendError,
		},
		{
			name:         "OUT_OF_RANGE 转换为错误请求错误",
			googleStatus: "OUT_OF_RANGE",
			expectCode:   domainerror.CodeBadRequest,
		},
		{
			name:         "UNIMPLEMENTED 转换为后端错误",
			googleStatus: "UNIMPLEMENTED",
			expectCode:   domainerror.CodeBackendError,
		},
		{
			name:         "INTERNAL 转换为后端错误",
			googleStatus: "INTERNAL",
			expectCode:   domainerror.CodeBackendError,
		},
		{
			name:         "UNAVAILABLE 转换为后端错误",
			googleStatus: "UNAVAILABLE",
			expectCode:   domainerror.CodeBackendError,
		},
		{
			name:         "DATA_LOSS 转换为后端错误",
			googleStatus: "DATA_LOSS",
			expectCode:   domainerror.CodeBackendError,
		},
		{
			name:         "未知状态转换为后端错误",
			googleStatus: "UNKNOWN_ERROR",
			expectCode:   domainerror.CodeBackendError,
		},
		{
			name:         "空状态字符串转换为后端错误",
			googleStatus: "",
			expectCode:   domainerror.CodeBackendError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := &ErrorConverter{}
			result := converter.convertErrorType(tt.googleStatus)

			if domainerror.ErrorCode(result) != tt.expectCode {
				t.Errorf("期望错误代码 %v, 实际 %v", tt.expectCode, result)
			}
		})
	}
}

// TestErrorConverter_ConvertStatusCode 测试状态码转换
func TestErrorConverter_ConvertStatusCode(t *testing.T) {
	converter := &ErrorConverter{}

	tests := []struct {
		name       string
		statusCode int
		expectCode domainerror.ErrorCode
	}{
		{
			name:       "400 转换为无效请求错误",
			statusCode: 400,
			expectCode: domainerror.CodeInvalidRequest,
		},
		{
			name:       "401 转换为未授权错误",
			statusCode: 401,
			expectCode: domainerror.CodeUnauthorized,
		},
		{
			name:       "403 转换为错误请求错误",
			statusCode: 403,
			expectCode: domainerror.CodeBadRequest,
		},
		{
			name:       "404 转换为错误请求错误",
			statusCode: 404,
			expectCode: domainerror.CodeBadRequest,
		},
		{
			name:       "409 转换为错误请求错误",
			statusCode: 409,
			expectCode: domainerror.CodeBadRequest,
		},
		{
			name:       "429 转换为速率限制错误",
			statusCode: 429,
			expectCode: domainerror.CodeRateLimited,
		},
		{
			name:       "499 转换为错误请求错误",
			statusCode: 499,
			expectCode: domainerror.CodeBadRequest,
		},
		{
			name:       "500 转换为后端错误",
			statusCode: 500,
			expectCode: domainerror.CodeBackendError,
		},
		{
			name:       "502 转换为后端错误",
			statusCode: 502,
			expectCode: domainerror.CodeBackendError,
		},
		{
			name:       "503 转换为后端错误",
			statusCode: 503,
			expectCode: domainerror.CodeBackendError,
		},
		{
			name:       "504 转换为后端错误",
			statusCode: 504,
			expectCode: domainerror.CodeBackendError,
		},
		{
			name:       "400-499 范围的其他状态码转换为无效请求错误",
			statusCode: 422,
			expectCode: domainerror.CodeInvalidRequest,
		},
		{
			name:       "其他 5xx 状态码转换为后端错误",
			statusCode: 599,
			expectCode: domainerror.CodeBackendError,
		},
		{
			name:       "低于 400 的状态码转换为后端错误",
			statusCode: 200,
			expectCode: domainerror.CodeBackendError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.convertStatusCode(tt.statusCode)

			if domainerror.ErrorCode(result) != tt.expectCode {
				t.Errorf("期望错误代码 %v, 实际 %v", tt.expectCode, result)
			}
		})
	}
}

// TestErrorConverter_IsRetryableStatus 测试可重试状态检查
func TestErrorConverter_IsRetryableStatus(t *testing.T) {
	converter := &ErrorConverter{}

	tests := []struct {
		name        string
		statusCode  int
		expectRetry bool
	}{
		{
			name:        "429 速率限制可重试",
			statusCode:  429,
			expectRetry: true,
		},
		{
			name:        "500 服务器错误可重试",
			statusCode:  500,
			expectRetry: true,
		},
		{
			name:        "501 服务器错误可重试",
			statusCode:  501,
			expectRetry: true,
		},
		{
			name:        "502 服务器错误可重试",
			statusCode:  502,
			expectRetry: true,
		},
		{
			name:        "503 服务器错误可重试",
			statusCode:  503,
			expectRetry: true,
		},
		{
			name:        "504 服务器错误可重试",
			statusCode:  504,
			expectRetry: true,
		},
		{
			name:        "599 服务器错误可重试",
			statusCode:  599,
			expectRetry: true,
		},
		{
			name:        "400 客户端错误不可重试",
			statusCode:  400,
			expectRetry: false,
		},
		{
			name:        "401 客户端错误不可重试",
			statusCode:  401,
			expectRetry: false,
		},
		{
			name:        "403 客户端错误不可重试",
			statusCode:  403,
			expectRetry: false,
		},
		{
			name:        "404 客户端错误不可重试",
			statusCode:  404,
			expectRetry: false,
		},
		{
			name:        "200 成功不可重试",
			statusCode:  200,
			expectRetry: false,
		},
		{
			name:        "300 重定向不可重试",
			statusCode:  301,
			expectRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.isRetryableStatus(tt.statusCode)

			if result != tt.expectRetry {
				t.Errorf("期望可重试 %v, 实际 %v", tt.expectRetry, result)
			}
		})
	}
}

// TestErrorConverter_HttpStatusMessage 测试 HTTP 状态码消息
func TestErrorConverter_HttpStatusMessage(t *testing.T) {
	converter := &ErrorConverter{}

	tests := []struct {
		name          string
		statusCode    int
		expectMessage string
	}{
		{
			name:          "400 返回参数无效消息",
			statusCode:    400,
			expectMessage: "Google Vertex AI 请求参数无效",
		},
		{
			name:          "401 返回认证失败消息",
			statusCode:    401,
			expectMessage: "Google Vertex AI 认证失败",
		},
		{
			name:          "403 返回权限不足消息",
			statusCode:    403,
			expectMessage: "Google Vertex AI 权限不足",
		},
		{
			name:          "404 返回资源未找到消息",
			statusCode:    404,
			expectMessage: "Google Vertex AI 资源未找到",
		},
		{
			name:          "429 返回频率超限消息",
			statusCode:    429,
			expectMessage: "Google Vertex AI 请求频率超限",
		},
		{
			name:          "500 返回内部服务器错误消息",
			statusCode:    500,
			expectMessage: "Google Vertex AI 内部服务器错误",
		},
		{
			name:          "502 返回网关错误消息",
			statusCode:    502,
			expectMessage: "Google Vertex AI 网关错误",
		},
		{
			name:          "503 返回服务不可用消息",
			statusCode:    503,
			expectMessage: "Google Vertex AI 服务不可用",
		},
		{
			name:          "504 返回网关超时消息",
			statusCode:    504,
			expectMessage: "Google Vertex AI 网关超时",
		},
		{
			name:          "其他状态码返回默认消息",
			statusCode:    418,
			expectMessage: "Google Vertex AI 请求失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.httpStatusMessage(tt.statusCode)

			if result != tt.expectMessage {
				t.Errorf("期望消息 %q, 实际 %q", tt.expectMessage, result)
			}
		})
	}
}

// TestErrorConverter_DefaultError 测试默认错误生成
func TestErrorConverter_DefaultError(t *testing.T) {
	mockLogger := &MockLogger{}
	converter := NewErrorConverter(mockLogger)

	tests := []struct {
		name           string
		statusCode     int
		expectCode     domainerror.ErrorCode
		expectHTTP     int
		expectProvider string
		expectMessage  string
	}{
		{
			name:           "400 生成无效请求默认错误",
			statusCode:     400,
			expectCode:     domainerror.CodeInvalidRequest,
			expectHTTP:     400,
			expectProvider: "google",
			expectMessage:  "Google Vertex AI 请求参数无效",
		},
		{
			name:           "401 生成未授权默认错误",
			statusCode:     401,
			expectCode:     domainerror.CodeUnauthorized,
			expectHTTP:     401,
			expectProvider: "google",
			expectMessage:  "Google Vertex AI 认证失败",
		},
		{
			name:           "500 生成后端错误默认错误",
			statusCode:     500,
			expectCode:     domainerror.CodeBackendError,
			expectHTTP:     500,
			expectProvider: "google",
			expectMessage:  "Google Vertex AI 内部服务器错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger.reset()
			err := converter.defaultError(tt.statusCode)

			if err == nil {
				t.Fatal("错误不应为 nil")
			}

			if err.Code != tt.expectCode {
				t.Errorf("期望错误代码 %v, 实际 %v", tt.expectCode, err.Code)
			}

			if err.HTTPStatus != tt.expectHTTP {
				t.Errorf("期望 HTTP 状态码 %d, 实际 %d", tt.expectHTTP, err.HTTPStatus)
			}

			if err.Provider != tt.expectProvider {
				t.Errorf("期望提供商 %s, 实际 %s", tt.expectProvider, err.Provider)
			}

			if err.Message != tt.expectMessage {
				t.Errorf("期望消息 %q, 实际 %q", tt.expectMessage, err.Message)
			}
		})
	}
}

// TestErrorConverter_Convert_WithRealGoogleResponse 测试真实 Google 错误响应转换
func TestErrorConverter_Convert_WithRealGoogleResponse(t *testing.T) {
	mockLogger := &MockLogger{}
	converter := NewErrorConverter(mockLogger)

	t.Run("转换带有详情的 Google 错误响应", func(t *testing.T) {
		mockLogger.reset()

		// 构造真实的 Google Vertex AI 错误响应
		googleResp := GoogleErrorResponse{}
		googleResp.Error.Code = 400
		googleResp.Error.Message = "Invalid argument: model not found"
		googleResp.Error.Status = "INVALID_ARGUMENT"
		googleResp.Error.Details = []GoogleErrorDetail{
			{
				Type:     "type.googleapis.com/google.rpc.ErrorInfo",
				Reason:   "MODEL_NOT_FOUND",
				Domain:   "googleapis.com",
				Metadata: map[string]string{"model": "gemini-pro"},
			},
		}

		respBody, _ := json.Marshal(googleResp)
		err := converter.Convert(400, respBody)

		if err == nil {
			t.Fatal("错误不应为 nil")
		}

		if err.Code != domainerror.CodeInvalidRequest {
			t.Errorf("期望错误代码 %v, 实际 %v", domainerror.CodeInvalidRequest, err.Code)
		}

		// 验证消息包含原始消息和详情原因
		if err.Message == "" {
			t.Error("消息不应为空")
		}
	})

	t.Run("转换 UNAUTHENTICATED 错误", func(t *testing.T) {
		mockLogger.reset()

		googleResp := GoogleErrorResponse{}
		googleResp.Error.Code = 401
		googleResp.Error.Message = "Request had invalid authentication credentials"
		googleResp.Error.Status = "UNAUTHENTICATED"

		respBody, _ := json.Marshal(googleResp)
		err := converter.Convert(401, respBody)

		if err == nil {
			t.Fatal("错误不应为 nil")
		}

		if err.Code != domainerror.CodeUnauthorized {
			t.Errorf("期望错误代码 %v, 实际 %v", domainerror.CodeUnauthorized, err.Code)
		}

		if err.HTTPStatus != 401 {
			t.Errorf("期望 HTTP 状态码 401, 实际 %d", err.HTTPStatus)
		}
	})

	t.Run("转换 PERMISSION_DENIED 错误", func(t *testing.T) {
		mockLogger.reset()

		googleResp := GoogleErrorResponse{}
		googleResp.Error.Code = 403
		googleResp.Error.Message = "Permission denied on resource (project)"
		googleResp.Error.Status = "PERMISSION_DENIED"
		googleResp.Error.Details = []GoogleErrorDetail{
			{
				Type:   "type.googleapis.com/google.rpc.ErrorInfo",
				Reason: "ACCESS_DENIED",
				Domain: "googleapis.com",
			},
		}

		respBody, _ := json.Marshal(googleResp)
		err := converter.Convert(403, respBody)

		if err == nil {
			t.Fatal("错误不应为 nil")
		}

		if err.Code != domainerror.CodeBadRequest {
			t.Errorf("期望错误代码 %v, 实际 %v", domainerror.CodeBadRequest, err.Code)
		}
	})

	t.Run("转换 RESOURCE_EXHAUSTED 速率限制错误", func(t *testing.T) {
		mockLogger.reset()

		googleResp := GoogleErrorResponse{}
		googleResp.Error.Code = 429
		googleResp.Error.Message = "Resource has been exhausted"
		googleResp.Error.Status = "RESOURCE_EXHAUSTED"

		respBody, _ := json.Marshal(googleResp)
		err := converter.Convert(429, respBody)

		if err == nil {
			t.Fatal("错误不应为 nil")
		}

		if err.Code != domainerror.CodeRateLimited {
			t.Errorf("期望错误代码 %v, 实际 %v", domainerror.CodeRateLimited, err.Code)
		}

		if !err.Retryable {
			t.Error("速率限制错误应可重试")
		}
	})

	t.Run("转换 INTERNAL 服务器错误", func(t *testing.T) {
		mockLogger.reset()

		googleResp := GoogleErrorResponse{}
		googleResp.Error.Code = 500
		googleResp.Error.Message = "Internal error encountered"
		googleResp.Error.Status = "INTERNAL"

		respBody, _ := json.Marshal(googleResp)
		err := converter.Convert(500, respBody)

		if err == nil {
			t.Fatal("错误不应为 nil")
		}

		if err.Code != domainerror.CodeBackendError {
			t.Errorf("期望错误代码 %v, 实际 %v", domainerror.CodeBackendError, err.Code)
		}

		if !err.Retryable {
			t.Error("服务器内部错误应可重试")
		}
	})

	t.Run("转换 UNAVAILABLE 服务不可用错误", func(t *testing.T) {
		mockLogger.reset()

		googleResp := GoogleErrorResponse{}
		googleResp.Error.Code = 503
		googleResp.Error.Message = "Service unavailable"
		googleResp.Error.Status = "UNAVAILABLE"

		respBody, _ := json.Marshal(googleResp)
		err := converter.Convert(503, respBody)

		if err == nil {
			t.Fatal("错误不应为 nil")
		}

		if err.Code != domainerror.CodeBackendError {
			t.Errorf("期望错误代码 %v, 实际 %v", domainerror.CodeBackendError, err.Code)
		}

		if !err.Retryable {
			t.Error("服务不可用错误应可重试")
		}
	})
}

// TestErrorConverter_Supports 测试协议支持检查
func TestErrorConverter_Supports(t *testing.T) {
	converter := &ErrorConverter{}

	tests := []struct {
		name     string
		protocol types.Protocol
		expected bool
	}{
		{
			name:     "支持 Google 协议",
			protocol: types.ProtocolGoogle,
			expected: true,
		},
		{
			name:     "不支持 OpenAI 协议",
			protocol: types.ProtocolOpenAI,
			expected: false,
		},
		{
			name:     "不支持 Anthropic 协议",
			protocol: types.ProtocolAnthropic,
			expected: false,
		},
		{
			name:     "不支持 Azure 协议",
			protocol: types.ProtocolAzure,
			expected: false,
		},
		{
			name:     "不支持未知协议",
			protocol: types.Protocol("unknown"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.Supports(tt.protocol)

			if result != tt.expected {
				t.Errorf("期望 %v, 实际 %v", tt.expected, result)
			}
		})
	}
}

// TestErrorConverter_Protocol 测试协议返回
func TestErrorConverter_Protocol(t *testing.T) {
	converter := &ErrorConverter{}

	result := converter.Protocol()

	if result != types.ProtocolGoogle {
		t.Errorf("期望协议 %v, 实际 %v", types.ProtocolGoogle, result)
	}
}

// TestErrorConverter_Name 测试策略名称返回
func TestErrorConverter_Name(t *testing.T) {
	converter := &ErrorConverter{}

	result := converter.Name()

	expected := "GoogleVertexAIErrorConverter"
	if result != expected {
		t.Errorf("期望名称 %s, 实际 %s", expected, result)
	}
}

// TestErrorConverter_LoggerDebugCall 测试日志记录功能
func TestErrorConverter_LoggerDebugCall(t *testing.T) {
	mockLogger := &MockLogger{}
	converter := NewErrorConverter(mockLogger)

	// 构造有效的 Google 错误响应
	googleResp := GoogleErrorResponse{}
	googleResp.Error.Code = 400
	googleResp.Error.Message = "Test error message"
	googleResp.Error.Status = "INVALID_ARGUMENT"

	respBody, _ := json.Marshal(googleResp)
	converter.Convert(400, respBody)

	// 验证 Debug 日志被调用
	if len(mockLogger.debugMessages) == 0 {
		t.Error("应该调用 Debug 日志")
	}

	// 验证日志消息内容
	found := false
	for _, msg := range mockLogger.debugMessages {
		if msg != "" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Debug 日志消息不应为空")
	}
}

// TestErrorConverter_LoggerNotCalledForNilLogger 测试 nil 日志器安全处理
func TestErrorConverter_LoggerNotCalledForNilLogger(t *testing.T) {
	// 这个测试验证 nil 日志器不会导致 panic
	// 使用 NewErrorConverter(nil) 会自动使用 NopLogger
	t.Run("nil_日志器不会导致_panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("不应该 panic: %v", r)
			}
		}()

		// 使用构造函数，它会自动处理 nil logger
		converter := NewErrorConverter(nil)
		err := converter.Convert(500, []byte("test error"))

		if err == nil {
			t.Error("错误不应为 nil")
		}
	})
}
