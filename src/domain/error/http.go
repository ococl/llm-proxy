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
