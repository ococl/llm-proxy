package cohere

import (
	"net/http"

	"llm-proxy/domain/port"
	"llm-proxy/domain/types"

	domainerror "llm-proxy/domain/error"
)

// ErrorConverter Cohere 协议的错误转换策略。
// Cohere 使用 OpenAI 兼容的错误格式。
type ErrorConverter struct {
	logger port.Logger
}

// NewErrorConverter 创建 Cohere 错误转换策略实例。
func NewErrorConverter(logger port.Logger) *ErrorConverter {
	if logger == nil {
		logger = &port.NopLogger{}
	}
	return &ErrorConverter{
		logger: logger,
	}
}

// Convert 将 Cohere 错误响应转换为标准错误格式。
// Cohere 错误响应格式与 OpenAI 相同。
func (c *ErrorConverter) Convert(statusCode int, respBody []byte) *domainerror.LLMProxyError {
	if len(respBody) == 0 {
		return c.defaultError(statusCode)
	}

	c.logger.Debug("转换 Cohere 错误",
		port.Int("status_code", statusCode),
		port.Bool("retryable", isRetryableStatus(statusCode)),
	)

	return &domainerror.LLMProxyError{
		Code:       domainerror.ErrorCode(getErrorCodeFromStatus(statusCode)),
		HTTPStatus: statusCode,
		Message:    string(respBody),
		Provider:   "cohere",
		Retryable:  isRetryableStatus(statusCode),
	}
}

// defaultError 返回默认错误。
func (c *ErrorConverter) defaultError(statusCode int) *domainerror.LLMProxyError {
	return &domainerror.LLMProxyError{
		Code:       domainerror.ErrorCode(getErrorCodeFromStatus(statusCode)),
		HTTPStatus: statusCode,
		Message:    getDefaultMessage(statusCode),
		Provider:   "cohere",
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

// getDefaultMessage 返回默认错误消息。
func getDefaultMessage(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "Cohere 请求参数无效"
	case http.StatusUnauthorized:
		return "Cohere 认证失败"
	case http.StatusForbidden:
		return "Cohere 权限不足"
	case http.StatusNotFound:
		return "Cohere 资源未找到"
	case http.StatusTooManyRequests:
		return "Cohere 请求频率超限"
	case http.StatusInternalServerError:
		return "Cohere 内部服务器错误"
	case http.StatusBadGateway:
		return "Cohere 网关错误"
	case http.StatusServiceUnavailable:
		return "Cohere 服务不可用"
	case http.StatusGatewayTimeout:
		return "Cohere 网关超时"
	default:
		return "Cohere 请求失败"
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
	return protocol == types.ProtocolCohere
}

// Protocol 返回支持的协议类型。
func (c *ErrorConverter) Protocol() types.Protocol {
	return types.ProtocolCohere
}

// Name 返回策略名称。
func (c *ErrorConverter) Name() string {
	return "CohereErrorConverter"
}
