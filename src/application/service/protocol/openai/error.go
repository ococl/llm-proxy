package openai

import (
	"encoding/json"
	"net/http"
	"strings"

	"llm-proxy/domain/port"
	"llm-proxy/domain/types"

	domainerror "llm-proxy/domain/error"
)

// ErrorConverter OpenAI 协议的错误转换策略。
// 将 OpenAI 错误响应转换为标准错误格式。
type ErrorConverter struct {
	logger port.Logger
}

// OpenAIError OpenAI 错误响应结构。
type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param,omitempty"`
	Code    string `json:"code,omitempty"`
}

// OpenAIErrorResponse OpenAI 完整错误响应结构。
type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error"`
}

// NewErrorConverter 创建 OpenAI 错误转换策略实例。
//
// 参数：
//   - logger: 日志记录器（可选）
//
// 返回：
//   - 初始化后的转换策略
func NewErrorConverter(logger port.Logger) *ErrorConverter {
	if logger == nil {
		logger = &port.NopLogger{}
	}
	return &ErrorConverter{
		logger: logger,
	}
}

// Convert 将 OpenAI 错误响应转换为标准错误格式。
//
// OpenAI 错误响应格式：
//
//	{"error": {"message": "...", "type": "...", "code": "..."}}
func (c *ErrorConverter) Convert(statusCode int, respBody []byte) *domainerror.LLMProxyError {
	if len(respBody) == 0 {
		return c.defaultError(statusCode)
	}

	// 尝试解析 OpenAI 错误格式
	var errorResp OpenAIErrorResponse
	var message, errorType string

	if err := json.Unmarshal(respBody, &errorResp); err == nil {
		message = errorResp.Error.Message
		errorType = errorResp.Error.Type
	} else {
		// 解析失败，使用原始响应
		message = strings.TrimSpace(string(respBody))
		errorType = getErrorTypeCodeFromStatus(statusCode)
	}

	// 判断是否可重试
	retryable := isRetryableStatus(statusCode)

	c.logger.Debug("转换 OpenAI 错误",
		port.Int("status_code", statusCode),
		port.String("message", message),
		port.String("error_type", errorType),
		port.Bool("retryable", retryable),
	)

	return &domainerror.LLMProxyError{
		Code:       domainerror.ErrorCode(getErrorCodeFromType(errorType)),
		HTTPStatus: statusCode,
		Message:    message,
		Provider:   "openai",
		Retryable:  retryable,
	}
}

// getErrorCodeFromType 将错误类型字符串转换为 ErrorCode。
func getErrorCodeFromType(errorType string) string {
	switch errorType {
	case "invalid_request_error":
		return string(domainerror.CodeInvalidRequest)
	case "authentication_error", "invalid_api_key":
		return string(domainerror.CodeUnauthorized)
	case "permission_error":
		return string(domainerror.CodeBadRequest)
	case "not_found_error":
		return string(domainerror.CodeBadRequest)
	case "rate_limit_error":
		return string(domainerror.CodeRateLimited)
	case "internal_server_error":
		return string(domainerror.CodeBackendError)
	default:
		return string(domainerror.CodeUnknown)
	}
}

// defaultError 返回默认错误。
func (c *ErrorConverter) defaultError(statusCode int) *domainerror.LLMProxyError {
	return &domainerror.LLMProxyError{
		Code:       domainerror.ErrorCode(getErrorCodeFromStatus(statusCode)),
		HTTPStatus: statusCode,
		Message:    getDefaultMessage(statusCode),
		Provider:   "openai",
		Retryable:  isRetryableStatus(statusCode),
	}
}

// getErrorCodeFromStatus 根据 HTTP 状态码获取错误代码。
func getErrorCodeFromStatus(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return string(domainerror.CodeInvalidRequest)
	case http.StatusUnauthorized:
		return string(domainerror.CodeUnauthorized)
	case http.StatusForbidden:
		return string(domainerror.CodeBadRequest)
	case http.StatusNotFound:
		return string(domainerror.CodeBadRequest)
	case http.StatusTooManyRequests:
		return string(domainerror.CodeRateLimited)
	case http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return string(domainerror.CodeBackendError)
	default:
		return string(domainerror.CodeUnknown)
	}
}

// getErrorTypeCodeFromStatus 根据 HTTP 状态码获取错误类型代码。
func getErrorTypeCodeFromStatus(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "invalid_request_error"
	case http.StatusUnauthorized:
		return "authentication_error"
	case http.StatusForbidden:
		return "permission_error"
	case http.StatusNotFound:
		return "not_found_error"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	case http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return "internal_server_error"
	default:
		return "unknown_error"
	}
}

// getDefaultMessage 返回默认错误消息。
func getDefaultMessage(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "OpenAI 请求参数无效"
	case http.StatusUnauthorized:
		return "OpenAI 认证失败"
	case http.StatusForbidden:
		return "OpenAI 权限不足"
	case http.StatusNotFound:
		return "OpenAI 资源未找到"
	case http.StatusTooManyRequests:
		return "OpenAI 请求频率超限"
	case http.StatusInternalServerError:
		return "OpenAI 内部服务器错误"
	case http.StatusBadGateway:
		return "OpenAI 网关错误"
	case http.StatusServiceUnavailable:
		return "OpenAI 服务不可用"
	case http.StatusGatewayTimeout:
		return "OpenAI 网关超时"
	default:
		return "OpenAI 请求失败"
	}
}

// isRetryableStatus 检查状态码是否可重试。
func isRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// Supports 检查是否支持指定协议。
func (c *ErrorConverter) Supports(protocol types.Protocol) bool {
	return protocol.IsOpenAICompatible()
}

// Protocol 返回支持的协议类型。
func (c *ErrorConverter) Protocol() types.Protocol {
	return types.ProtocolOpenAI
}

// Name 返回策略名称。
func (c *ErrorConverter) Name() string {
	return "OpenAIErrorConverter"
}
