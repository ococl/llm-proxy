package errors

import (
	"encoding/json"
	"net/http"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	TraceID string `json:"trace_id,omitempty"`
}

var (
	ErrBadRequest       = APIError{Code: "BAD_REQUEST", Message: "无效的请求"}
	ErrUnauthorized     = APIError{Code: "UNAUTHORIZED", Message: "无效的 API Key"}
	ErrMissingModel     = APIError{Code: "MISSING_MODEL", Message: "缺少 model 字段"}
	ErrInvalidJSON      = APIError{Code: "INVALID_JSON", Message: "无效的 JSON 请求体"}
	ErrUnknownModel     = APIError{Code: "UNKNOWN_MODEL", Message: "未知的模型别名"}
	ErrNoBackend        = APIError{Code: "NO_BACKEND", Message: "所有后端均失败"}
	ErrRateLimited      = APIError{Code: "RATE_LIMITED", Message: "请求过于频繁"}
	ErrConcurrencyLimit = APIError{Code: "CONCURRENCY_LIMIT", Message: "并发请求数超限"}
)

func WriteJSONError(w http.ResponseWriter, err APIError, status int, traceID string) {
	resp := err
	if traceID != "" {
		resp.TraceID = traceID
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func WriteJSONErrorWithMsg(w http.ResponseWriter, err APIError, status int, traceID, msg string) {
	resp := err
	resp.Message = msg
	if traceID != "" {
		resp.TraceID = traceID
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
