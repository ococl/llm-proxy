package domainerror

import (
	"errors"
	"fmt"
)

// ErrorType represents the category of an error.
type ErrorType string

const (
	// ErrorTypeClient represents client-side errors (4xx).
	ErrorTypeClient ErrorType = "client"
	// ErrorTypeBackend represents backend errors (5xx).
	ErrorTypeBackend ErrorType = "backend"
	// ErrorTypeProtocol represents protocol conversion errors.
	ErrorTypeProtocol ErrorType = "protocol"
	// ErrorTypeConfig represents configuration errors.
	ErrorTypeConfig ErrorType = "config"
	// ErrorTypeInternal represents internal errors.
	ErrorTypeInternal ErrorType = "internal"
	// ErrorTypeValidation represents validation errors.
	ErrorTypeValidation ErrorType = "validation"
	// ErrorTypeRateLimit represents rate limit errors.
	ErrorTypeRateLimit ErrorType = "rate_limit"
	// ErrorTypeConcurrency represents concurrency limit errors.
	ErrorTypeConcurrency ErrorType = "concurrency"
)

// ErrorCode represents a specific error code.
type ErrorCode string

const (
	// Client errors
	CodeBadRequest       ErrorCode = "BAD_REQUEST"
	CodeUnauthorized     ErrorCode = "UNAUTHORIZED"
	CodeMissingModel     ErrorCode = "MISSING_MODEL"
	CodeInvalidJSON      ErrorCode = "INVALID_JSON"
	CodeUnknownModel     ErrorCode = "UNKNOWN_MODEL"
	CodeRateLimited      ErrorCode = "RATE_LIMITED"
	CodeConcurrencyLimit ErrorCode = "CONCURRENCY_LIMIT"
	CodeInvalidRequest   ErrorCode = "INVALID_REQUEST"

	// Backend errors
	CodeNoBackend      ErrorCode = "NO_BACKEND"
	CodeBackendTimeout ErrorCode = "BACKEND_TIMEOUT"
	CodeBackendUnavail ErrorCode = "BACKEND_UNAVAILABLE"
	CodeBackendError   ErrorCode = "BACKEND_ERROR"

	// Protocol errors
	CodeProtocolConvert ErrorCode = "PROTOCOL_CONVERSION_ERROR"
	CodeInvalidProtocol ErrorCode = "INVALID_PROTOCOL"

	// Config errors
	CodeConfigLoad       ErrorCode = "CONFIG_LOAD_ERROR"
	CodeConfigValidation ErrorCode = "CONFIG_VALIDATION_ERROR"

	// Internal errors
	CodeInternal ErrorCode = "INTERNAL_ERROR"
	CodeUnknown  ErrorCode = "UNKNOWN_ERROR"
)

// LLMProxyError represents a structured error in the LLM proxy.
type LLMProxyError struct {
	Type        ErrorType
	Code        ErrorCode
	Message     string
	Cause       error
	TraceID     string
	BackendName string
	HTTPStatus  int
}

// Error implements the error interface.
func (e *LLMProxyError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s:%s] %s: %v", e.Type, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s:%s] %s", e.Type, e.Code, e.Message)
}

// Unwrap returns the underlying cause.
func (e *LLMProxyError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target.
func (e *LLMProxyError) Is(target error) bool {
	t, ok := target.(*LLMProxyError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// WithTraceID adds a trace ID to the error.
func (e *LLMProxyError) WithTraceID(traceID string) *LLMProxyError {
	e.TraceID = traceID
	return e
}

// WithBackend adds a backend name to the error.
func (e *LLMProxyError) WithBackend(backend string) *LLMProxyError {
	e.BackendName = backend
	return e
}

// WithCause adds a cause to the error.
func (e *LLMProxyError) WithCause(cause error) *LLMProxyError {
	e.Cause = cause
	return e
}

// GetHTTPStatus returns the HTTP status code for the error.
func (e *LLMProxyError) GetHTTPStatus() int {
	if e.HTTPStatus > 0 {
		return e.HTTPStatus
	}
	// Default status codes based on error type
	switch e.Type {
	case ErrorTypeClient, ErrorTypeValidation:
		return 400
	case ErrorTypeRateLimit:
		return 429
	case ErrorTypeConcurrency:
		return 503
	case ErrorTypeBackend:
		return 502
	case ErrorTypeInternal, ErrorTypeProtocol, ErrorTypeConfig:
		return 500
	default:
		return 500
	}
}

// New creates a new LLMProxyError.
func New(errType ErrorType, code ErrorCode, message string) *LLMProxyError {
	return &LLMProxyError{
		Type:    errType,
		Code:    code,
		Message: message,
	}
}

// NewWithStatus creates a new LLMProxyError with a custom HTTP status.
func NewWithStatus(errType ErrorType, code ErrorCode, message string, status int) *LLMProxyError {
	return &LLMProxyError{
		Type:       errType,
		Code:       code,
		Message:    message,
		HTTPStatus: status,
	}
}

// Wrap wraps an existing error with LLMProxyError context.
func Wrap(err error, errType ErrorType, code ErrorCode, message string) *LLMProxyError {
	return &LLMProxyError{
		Type:    errType,
		Code:    code,
		Message: message,
		Cause:   err,
	}
}

// IsType checks if an error is of a specific type.
func IsType(err error, errType ErrorType) bool {
	var proxyErr *LLMProxyError
	if errors.As(err, &proxyErr) {
		return proxyErr.Type == errType
	}
	return false
}

// IsCode checks if an error has a specific code.
func IsCode(err error, code ErrorCode) bool {
	var proxyErr *LLMProxyError
	if errors.As(err, &proxyErr) {
		return proxyErr.Code == code
	}
	return false
}

// GetTraceID extracts the trace ID from an error.
func GetTraceID(err error) string {
	var proxyErr *LLMProxyError
	if errors.As(err, &proxyErr) {
		return proxyErr.TraceID
	}
	return ""
}

// GetBackendName extracts the backend name from an error.
func GetBackendName(err error) string {
	var proxyErr *LLMProxyError
	if errors.As(err, &proxyErr) {
		return proxyErr.BackendName
	}
	return ""
}

// Common error constructors

// NewBadRequest creates a bad request error.
func NewBadRequest(message string) *LLMProxyError {
	return New(ErrorTypeClient, CodeBadRequest, message)
}

// NewInvalidRequest creates an invalid request error.
func NewInvalidRequest(format string, args ...interface{}) *LLMProxyError {
	message := fmt.Sprintf(format, args...)
	return New(ErrorTypeValidation, CodeBadRequest, message)
}

// NewUnauthorized creates an unauthorized error.
func NewUnauthorized(message string) *LLMProxyError {
	return NewWithStatus(ErrorTypeClient, CodeUnauthorized, message, 401)
}

// NewMissingModel creates a missing model error.
func NewMissingModel() *LLMProxyError {
	return New(ErrorTypeValidation, CodeMissingModel, "缺少 model 字段")
}

// NewInvalidJSON creates an invalid JSON error.
func NewInvalidJSON(cause error) *LLMProxyError {
	return Wrap(cause, ErrorTypeValidation, CodeInvalidJSON, "无效的 JSON 请求体")
}

// NewUnknownModel creates an unknown model error.
func NewUnknownModel(model string) *LLMProxyError {
	return New(ErrorTypeValidation, CodeUnknownModel, fmt.Sprintf("未知的模型别名: %s", model))
}

// NewNoBackend creates a no backend error.
func NewNoBackend() *LLMProxyError {
	return NewWithStatus(ErrorTypeBackend, CodeNoBackend, "所有后端均失败", 502)
}

// NewRateLimited creates a rate limited error.
func NewRateLimited(message string) *LLMProxyError {
	return NewWithStatus(ErrorTypeRateLimit, CodeRateLimited, message, 429)
}

// NewConcurrencyLimit creates a concurrency limit error.
func NewConcurrencyLimit(message string) *LLMProxyError {
	return NewWithStatus(ErrorTypeConcurrency, CodeConcurrencyLimit, message, 503)
}

// NewBackendError creates a backend error.
func NewBackendError(backend string, cause error) *LLMProxyError {
	return Wrap(cause, ErrorTypeBackend, CodeBackendError, fmt.Sprintf("后端 %s 请求失败", backend)).WithBackend(backend)
}

// NewProtocolError creates a protocol conversion error.
func NewProtocolError(message string, cause error) *LLMProxyError {
	return Wrap(cause, ErrorTypeProtocol, CodeProtocolConvert, message)
}

// NewConfigError creates a configuration error.
func NewConfigError(message string, cause error) *LLMProxyError {
	return Wrap(cause, ErrorTypeConfig, CodeConfigLoad, message)
}

// NewInternalError creates an internal error.
func NewInternalError(message string, cause error) *LLMProxyError {
	return Wrap(cause, ErrorTypeInternal, CodeInternal, message)
}
