package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"llm-proxy/domain/error"
)

// ErrorResponseConverter 负责将不同 LLM 提供商的错误响应转换为标准格式。
// 支持 OpenAI、Anthropic 等主流 API 的错误格式标准化。
type ErrorResponseConverter struct{}

// NewErrorResponseConverter 创建一个新的错误响应转换器。
func NewErrorResponseConverter() *ErrorResponseConverter {
	return &ErrorResponseConverter{}
}

// APIError 标准化 API 错误结构。
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param,omitempty"`
	Line    int    `json:"line,omitempty"`
}

// StandardizedErrorResponse 标准化的错误响应格式。
type StandardizedErrorResponse struct {
	Error   APIError `json:"error"`
	ReqID   string   `json:"req_id,omitempty"`
	Backend string   `json:"backend,omitempty"`
}

// ConvertError 将各种来源的错误转换为标准化格式。
func (ec *ErrorResponseConverter) ConvertError(err error, backendID string, reqID string) *StandardizedErrorResponse {
	if err == nil {
		return nil
	}

	// 如果是已知的 LLM Proxy 错误，直接转换
	var proxyErr *domainerror.LLMProxyError
	if errors.As(err, &proxyErr) {
		return ec.convertProxyError(proxyErr, backendID, reqID)
	}

	// 通用错误处理
	return &StandardizedErrorResponse{
		Error: APIError{
			Code:    "INTERNAL_ERROR",
			Message: err.Error(),
			Type:    "internal_server_error",
		},
		ReqID:   reqID,
		Backend: backendID,
	}
}

// ConvertAPIResponse 将上游 API 的错误响应转换为标准化格式。
func (ec *ErrorResponseConverter) ConvertAPIResponse(
	statusCode int,
	body []byte,
	backendID string,
	reqID string,
) *StandardizedErrorResponse {
	var apiErr APIError

	// 尝试解析不同格式的错误响应
	if openAIErr := ec.parseOpenAIError(body); openAIErr != nil {
		apiErr = *openAIErr
	} else if anthropicErr := ec.parseAnthropicError(body); anthropicErr != nil {
		apiErr = *anthropicErr
	} else {
		// 使用状态码生成通用错误
		apiErr = ec.errorFromStatusCode(statusCode)
	}

	return &StandardizedErrorResponse{
		Error:   apiErr,
		ReqID:   reqID,
		Backend: backendID,
	}
}

// parseOpenAIError 解析 OpenAI 格式的错误响应。
func (ec *ErrorResponseConverter) parseOpenAIError(body []byte) *APIError {
	var response struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Type    string `json:"type"`
			Param   string `json:"param"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil
	}

	if response.Error.Message == "" {
		return nil
	}

	return &APIError{
		Code:    ec.normalizeErrorCode(response.Error.Code, response.Error.Type),
		Message: response.Error.Message,
		Type:    response.Error.Type,
		Param:   response.Error.Param,
	}
}

// parseAnthropicError 解析 Anthropic 格式的错误响应。
func (ec *ErrorResponseConverter) parseAnthropicError(body []byte) *APIError {
	var response struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil
	}

	if response.Error.Message == "" {
		return nil
	}

	return &APIError{
		Code:    ec.normalizeErrorCode("", response.Error.Type),
		Message: response.Error.Message,
		Type:    response.Error.Type,
	}
}

// convertProxyError 将 LLM Proxy 错误转换为标准化格式。
func (ec *ErrorResponseConverter) convertProxyError(proxyErr *domainerror.LLMProxyError, backendID, reqID string) *StandardizedErrorResponse {
	return &StandardizedErrorResponse{
		Error: APIError{
			Code:    string(proxyErr.Code),
			Message: proxyErr.Message,
			Type:    string(proxyErr.Type),
		},
		ReqID:   reqID,
		Backend: backendID,
	}
}

// errorFromStatusCode 根据 HTTP 状态码生成错误信息。
func (ec *ErrorResponseConverter) errorFromStatusCode(statusCode int) APIError {
	switch statusCode {
	case http.StatusBadRequest:
		return APIError{Code: "INVALID_REQUEST", Message: "无效的请求参数", Type: "invalid_request_error"}
	case http.StatusUnauthorized:
		return APIError{Code: "AUTHENTICATION_ERROR", Message: "认证失败，请检查 API 密钥", Type: "authentication_error"}
	case http.StatusForbidden:
		return APIError{Code: "PERMISSION_DENIED", Message: "没有权限访问此资源", Type: "permission_denied_error"}
	case http.StatusNotFound:
		return APIError{Code: "NOT_FOUND", Message: "请求的资源不存在", Type: "not_found_error"}
	case http.StatusTooManyRequests:
		return APIError{Code: "RATE_LIMIT_ERROR", Message: "请求过于频繁，请稍后重试", Type: "rate_limit_error"}
	case http.StatusServiceUnavailable:
		return APIError{Code: "SERVICE_UNAVAILABLE", Message: "服务暂时不可用，请稍后重试", Type: "service_unavailable_error"}
	default:
		if statusCode >= 500 {
			return APIError{Code: "INTERNAL_ERROR", Message: "上游服务错误", Type: "internal_server_error"}
		}
		return APIError{Code: "UNKNOWN_ERROR", Message: fmt.Sprintf("未知错误，状态码: %d", statusCode), Type: "unknown_error"}
	}
}

// normalizeErrorCode 标准化错误代码。
func (ec *ErrorResponseConverter) normalizeErrorCode(code, errorType string) string {
	if code != "" && code != "null" {
		return code
	}

	switch errorType {
	case "invalid_request_error":
		return "INVALID_REQUEST"
	case "authentication_error":
		return "AUTHENTICATION_ERROR"
	case "permission_denied_error":
		return "PERMISSION_DENIED"
	case "rate_limit_error":
		return "RATE_LIMIT_ERROR"
	case "context_length_exceeded":
		return "CONTEXT_LENGTH_EXCEEDED"
	case "content_filter_error":
		return "CONTENT_FILTER_ERROR"
	case "service_unavailable_error":
		return "SERVICE_UNAVAILABLE"
	case "internal_server_error":
		return "INTERNAL_ERROR"
	default:
		return strings.ToUpper(strings.ReplaceAll(errorType, "_", " "))
	}
}

// IsRetryableError 判断错误是否可重试。
func (ec *ErrorResponseConverter) IsRetryableError(statusCode int, apiErr *APIError) bool {
	if statusCode >= 500 {
		return true
	}
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	switch apiErr.Type {
	case "rate_limit_error", "service_unavailable_error", "internal_server_error":
		return true
	case "authentication_error", "permission_denied_error", "invalid_request_error":
		return false
	default:
		return statusCode >= 500
	}
}
