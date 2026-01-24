package azure

import (
	"encoding/json"
	"net/http"
	"strings"

	"llm-proxy/domain/port"
	"llm-proxy/domain/types"

	domainerror "llm-proxy/domain/error"
)

// ErrorConverter Azure OpenAI 协议的错误转换策略。
// 将 Azure OpenAI 错误响应转换为标准错误格式。
type ErrorConverter struct {
	logger port.Logger
}

// AzureError Azure OpenAI 错误响应结构。
// Azure 使用 `code` 字段而非 OpenAI 的 `type` 字段。
type AzureError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// AzureErrorResponse Azure OpenAI 完整错误响应结构。
type AzureErrorResponse struct {
	Error AzureError `json:"error"`
}

// NewErrorConverter 创建 Azure OpenAI 错误转换策略实例。
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

// Convert 将 Azure OpenAI 错误响应转换为标准错误格式。
//
// Azure OpenAI 错误响应格式：
//
//	{"error": {"code": "...", "message": "..."}}
//
// 与 OpenAI 的主要区别：
// - 使用 `code` 字段而非 `type` 字段
// - 特有 `content_filter` 错误码（内容过滤）
func (c *ErrorConverter) Convert(statusCode int, respBody []byte) *domainerror.LLMProxyError {
	if len(respBody) == 0 {
		return c.defaultError(statusCode)
	}

	// 尝试解析 Azure 错误格式
	var errorResp AzureErrorResponse
	var message, errorCode string

	if err := json.Unmarshal(respBody, &errorResp); err == nil {
		message = errorResp.Error.Message
		errorCode = errorResp.Error.Code
	} else {
		// 解析失败，使用原始响应和状态码映射
		message = strings.TrimSpace(string(respBody))
		// 无效 JSON 直接使用状态码映射，不经过错误码转换
		return &domainerror.LLMProxyError{
			Code:       domainerror.ErrorCode(getErrorCodeFromStatus(statusCode)),
			HTTPStatus: statusCode,
			Message:    message,
			Provider:   "azure",
			Retryable:  isRetryableStatus(statusCode),
		}
	}

	// 判断是否可重试
	retryable := isRetryableStatus(statusCode)

	c.logger.Debug("转换 Azure OpenAI 错误",
		port.Int("status_code", statusCode),
		port.String("message", message),
		port.String("error_code", errorCode),
		port.Bool("retryable", retryable),
	)

	return &domainerror.LLMProxyError{
		Code:       domainerror.ErrorCode(getErrorCodeFromCode(errorCode)),
		HTTPStatus: statusCode,
		Message:    message,
		Provider:   "azure",
		Retryable:  retryable,
	}
}

// getErrorCodeFromCode 将 Azure 错误码字符串转换为 ErrorCode。
//
// Azure 特有错误码：
// - "content_filter": 内容过滤错误（不应重试）
// - "429": 速率限制
// - 其他数字字符串: 根据 HTTP 状态码映射
func getErrorCodeFromCode(errorCode string) string {
	switch errorCode {
	case "content_filter":
		// 内容过滤错误，不应重试
		return string(domainerror.CodeInvalidRequest)
	case "429", "rate_limit_exceeded":
		return string(domainerror.CodeRateLimited)
	case "invalid_api_key", "authentication_error":
		return string(domainerror.CodeUnauthorized)
	case "invalid_request_error", "invalid_parameter":
		return string(domainerror.CodeInvalidRequest)
	case "not_found_error":
		return string(domainerror.CodeBadRequest)
	case "permission_error":
		return string(domainerror.CodeBadRequest)
	case "internal_server_error":
		return string(domainerror.CodeBackendError)
	default:
		// 处理纯数字状态码（如 "400"、"401"）
		return getErrorCodeFromStatusCodeString(errorCode)
	}
}

// getErrorCodeFromStatusCodeString 根据状态码字符串获取错误代码。
func getErrorCodeFromStatusCodeString(statusCodeStr string) string {
	switch statusCodeStr {
	case "400":
		return string(domainerror.CodeInvalidRequest)
	case "401":
		return string(domainerror.CodeUnauthorized)
	case "403":
		return string(domainerror.CodeBadRequest)
	case "404":
		return string(domainerror.CodeBadRequest)
	case "429":
		return string(domainerror.CodeRateLimited)
	case "500":
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
		Provider:   "azure",
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
		return "Azure OpenAI 请求参数无效"
	case http.StatusUnauthorized:
		return "Azure OpenAI 认证失败"
	case http.StatusForbidden:
		return "Azure OpenAI 权限不足"
	case http.StatusNotFound:
		return "Azure OpenAI 资源未找到"
	case http.StatusTooManyRequests:
		return "Azure OpenAI 请求频率超限"
	case http.StatusInternalServerError:
		return "Azure OpenAI 内部服务器错误"
	case http.StatusBadGateway:
		return "Azure OpenAI 网关错误"
	case http.StatusServiceUnavailable:
		return "Azure OpenAI 服务不可用"
	case http.StatusGatewayTimeout:
		return "Azure OpenAI 网关超时"
	default:
		return "Azure OpenAI 请求失败"
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
	return protocol == types.ProtocolAzure
}

// Protocol 返回支持的协议类型。
func (c *ErrorConverter) Protocol() types.Protocol {
	return types.ProtocolAzure
}

// Name 返回策略名称。
func (c *ErrorConverter) Name() string {
	return "AzureErrorConverter"
}
