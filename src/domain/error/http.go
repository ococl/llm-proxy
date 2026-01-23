package domainerror

import (
	"encoding/json"
	"errors"
	"net/http"
)

// APIErrorResponse represents the JSON error response format.
type APIErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail represents the error details in the response.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	TraceID string `json:"trace_id,omitempty"`
	Backend string `json:"backend,omitempty"`
}

// ToAPIResponse converts a LLMProxyError to an API error response.
func ToAPIResponse(err *LLMProxyError) APIErrorResponse {
	return APIErrorResponse{
		Error: ErrorDetail{
			Code:    string(err.Code),
			Message: err.Message,
			Type:    string(err.Type),
			TraceID: err.TraceID,
			Backend: err.BackendName,
		},
	}
}

// WriteJSONError writes a LLMProxyError as a JSON response.
func WriteJSONError(w http.ResponseWriter, err *LLMProxyError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.GetHTTPStatus())
	json.NewEncoder(w).Encode(ToAPIResponse(err))
}

// WriteError writes an error as a JSON response.
// If the error is not a LLMProxyError, it creates a generic internal error.
func WriteError(w http.ResponseWriter, err error) {
	var proxyErr *LLMProxyError
	if !errors.As(err, &proxyErr) {
		proxyErr = NewInternalError("内部错误", err)
	}
	WriteJSONError(w, proxyErr)
}

// WriteAPIError writes a simple API error response (for legacy compatibility).
func WriteAPIError(w http.ResponseWriter, code, message string, status int) {
	resp := APIErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// WriteAPIErrorWithTrace writes a simple API error response with trace ID.
func WriteAPIErrorWithTrace(w http.ResponseWriter, code, message, traceID string, status int) {
	resp := APIErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			TraceID: traceID,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// WriteBadRequest writes a bad request error response.
func WriteBadRequest(w http.ResponseWriter, message string) {
	WriteAPIError(w, "BAD_REQUEST", message, http.StatusBadRequest)
}

// WriteUnauthorized writes an unauthorized error response.
func WriteUnauthorized(w http.ResponseWriter, message string) {
	WriteAPIError(w, "UNAUTHORIZED", message, http.StatusUnauthorized)
}

// WriteRateLimited writes a rate limited error response.
func WriteRateLimited(w http.ResponseWriter) {
	WriteAPIError(w, "RATE_LIMITED", "请求过于频繁", http.StatusTooManyRequests)
}

// WriteConcurrencyLimit writes a concurrency limit error response.
func WriteConcurrencyLimit(w http.ResponseWriter) {
	WriteAPIError(w, "CONCURRENCY_LIMIT", "并发请求数超限", http.StatusServiceUnavailable)
}

// WriteBackendError writes a backend error response.
func WriteBackendError(w http.ResponseWriter, backend string, traceID string) {
	WriteAPIErrorWithTrace(w, "BACKEND_ERROR", "后端 "+backend+" 请求失败", traceID, http.StatusBadGateway)
}

// WriteNoBackend writes a no backend available error response.
func WriteNoBackend(w http.ResponseWriter, traceID string) {
	WriteAPIErrorWithTrace(w, "NO_BACKEND", "所有后端均失败", traceID, http.StatusBadGateway)
}
