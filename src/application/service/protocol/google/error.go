package google

import (
	"encoding/json"
	"strings"

	"llm-proxy/domain/port"
	"llm-proxy/domain/types"

	domainerror "llm-proxy/domain/error"
)

// ErrorConverter Google Vertex AI 协议的错误转换策略。
// 负责将 Google Vertex AI 错误响应转换为标准错误格式。
type ErrorConverter struct {
	logger port.Logger
}

// NewErrorConverter 创建 Google Vertex AI 错误转换策略实例。
func NewErrorConverter(logger port.Logger) *ErrorConverter {
	if logger == nil {
		logger = &port.NopLogger{}
	}
	return &ErrorConverter{
		logger: logger,
	}
}

// GoogleErrorResponse Google Vertex AI 错误响应格式。
type GoogleErrorResponse struct {
	Error struct {
		Code    int                 `json:"code"`
		Message string              `json:"message"`
		Status  string              `json:"status"`
		Details []GoogleErrorDetail `json:"details,omitempty"`
	} `json:"error"`
}

// GoogleErrorDetail 错误详情。
type GoogleErrorDetail struct {
	Type     string            `json:"@type"`
	Reason   string            `json:"reason"`
	Domain   string            `json:"domain"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Convert 将 Google Vertex AI 错误响应转换为标准错误格式。
func (c *ErrorConverter) Convert(statusCode int, respBody []byte) *domainerror.LLMProxyError {
	if len(respBody) == 0 {
		return c.defaultError(statusCode)
	}

	// 解析 Google Vertex AI 错误响应
	var googleErr GoogleErrorResponse
	if err := json.Unmarshal(respBody, &googleErr); err != nil {
		c.logger.Debug("解析 Google Vertex AI 错误响应失败",
			port.String("error", err.Error()),
		)
		// 返回原始错误
		return &domainerror.LLMProxyError{
			Code:       domainerror.CodeBackendError,
			HTTPStatus: statusCode,
			Message:    string(respBody),
			Provider:   "google",
			Retryable:  c.isRetryableStatus(statusCode),
		}
	}

	// 转换错误类型
	errType := c.convertErrorType(googleErr.Error.Status)

	// 构建详细消息
	var details strings.Builder
	details.WriteString(googleErr.Error.Message)
	if len(googleErr.Error.Details) > 0 {
		for _, detail := range googleErr.Error.Details {
			if detail.Reason != "" {
				details.WriteString("\n原因: ")
				details.WriteString(detail.Reason)
			}
			if detail.Domain != "" {
				details.WriteString(" (")
				details.WriteString(detail.Domain)
				details.WriteString(")")
			}
		}
	}

	c.logger.Debug("Google Vertex AI 错误转换完成",
		port.Int("status_code", statusCode),
		port.String("google_status", googleErr.Error.Status),
		port.String("error_type", string(errType)),
	)

	return &domainerror.LLMProxyError{
		Code:       domainerror.ErrorCode(errType),
		HTTPStatus: statusCode,
		Message:    details.String(),
		Provider:   "google",
		Retryable:  c.isRetryableStatus(statusCode),
	}
}

// convertErrorType 转换 Google 错误状态为标准错误代码。
func (c *ErrorConverter) convertErrorType(googleStatus string) string {
	switch strings.ToUpper(googleStatus) {
	case "INVALID_ARGUMENT":
		return string(domainerror.CodeInvalidRequest)
	case "UNAUTHENTICATED":
		return string(domainerror.CodeUnauthorized)
	case "PERMISSION_DENIED":
		return string(domainerror.CodeBadRequest)
	case "NOT_FOUND":
		return string(domainerror.CodeBadRequest)
	case "ALREADY_EXISTS":
		return string(domainerror.CodeBadRequest)
	case "RESOURCE_EXHAUSTED":
		return string(domainerror.CodeRateLimited)
	case "FAILED_PRECONDITION":
		return string(domainerror.CodeBadRequest)
	case "ABORTED":
		return string(domainerror.CodeBackendError)
	case "OUT_OF_RANGE":
		return string(domainerror.CodeBadRequest)
	case "UNIMPLEMENTED":
		return string(domainerror.CodeBackendError)
	case "INTERNAL":
		return string(domainerror.CodeBackendError)
	case "UNAVAILABLE":
		return string(domainerror.CodeBackendError)
	case "DATA_LOSS":
		return string(domainerror.CodeBackendError)
	default:
		return string(domainerror.CodeBackendError)
	}
}

// defaultError 返回默认错误。
func (c *ErrorConverter) defaultError(statusCode int) *domainerror.LLMProxyError {
	errCode := c.convertStatusCode(statusCode)

	return &domainerror.LLMProxyError{
		Code:       domainerror.ErrorCode(errCode),
		HTTPStatus: statusCode,
		Message:    c.httpStatusMessage(statusCode),
		Provider:   "google",
		Retryable:  c.isRetryableStatus(statusCode),
	}
}

// convertStatusCode 将 HTTP 状态码转换为错误代码。
func (c *ErrorConverter) convertStatusCode(statusCode int) string {
	switch statusCode {
	case 400:
		return string(domainerror.CodeInvalidRequest)
	case 401:
		return string(domainerror.CodeUnauthorized)
	case 403:
		return string(domainerror.CodeBadRequest)
	case 404:
		return string(domainerror.CodeBadRequest)
	case 409:
		return string(domainerror.CodeBadRequest)
	case 429:
		return string(domainerror.CodeRateLimited)
	case 499:
		return string(domainerror.CodeBadRequest)
	case 500, 502, 503, 504:
		return string(domainerror.CodeBackendError)
	default:
		if statusCode >= 400 && statusCode < 500 {
			return string(domainerror.CodeInvalidRequest)
		}
		return string(domainerror.CodeBackendError)
	}
}

// isRetryableStatus 检查状态码是否可重试。
func (c *ErrorConverter) isRetryableStatus(statusCode int) bool {
	// 429 (rate limit) 和 5xx 服务器错误可重试
	return statusCode == 429 || (statusCode >= 500 && statusCode < 600)
}

// httpStatusMessage 返回 HTTP 状态码对应的默认消息。
func (c *ErrorConverter) httpStatusMessage(statusCode int) string {
	switch statusCode {
	case 400:
		return "Google Vertex AI 请求参数无效"
	case 401:
		return "Google Vertex AI 认证失败"
	case 403:
		return "Google Vertex AI 权限不足"
	case 404:
		return "Google Vertex AI 资源未找到"
	case 429:
		return "Google Vertex AI 请求频率超限"
	case 500:
		return "Google Vertex AI 内部服务器错误"
	case 502:
		return "Google Vertex AI 网关错误"
	case 503:
		return "Google Vertex AI 服务不可用"
	case 504:
		return "Google Vertex AI 网关超时"
	default:
		return "Google Vertex AI 请求失败"
	}
}

// Supports 检查是否支持指定协议。
func (c *ErrorConverter) Supports(protocol types.Protocol) bool {
	return protocol == types.ProtocolGoogle
}

// Protocol 返回支持的协议类型。
func (c *ErrorConverter) Protocol() types.Protocol {
	return types.ProtocolGoogle
}

// Name 返回策略名称。
func (c *ErrorConverter) Name() string {
	return "GoogleVertexAIErrorConverter"
}
