package http

import (
	"net/http"

	domainerror "llm-proxy/domain/error"
	"llm-proxy/domain/port"
)

type ErrorPresenter struct {
	logger port.Logger
}

func NewErrorPresenter(logger port.Logger) *ErrorPresenter {
	return &ErrorPresenter{
		logger: logger,
	}
}

func (ep *ErrorPresenter) WriteError(w http.ResponseWriter, r *http.Request, err error) {
	reqID := extractReqID(r)

	llmErr, ok := err.(*domainerror.LLMProxyError)
	if !ok {
		llmErr = domainerror.NewInternalError("unexpected error", err)
	}

	if reqID != "" {
		llmErr = llmErr.WithReqID(reqID)
	}

	ep.logError(llmErr)

	domainerror.WriteJSONError(w, llmErr)
}

func (ep *ErrorPresenter) WriteJSONError(w http.ResponseWriter, message string, statusCode int, reqID string) {
	llmErr := domainerror.NewWithStatus(
		domainerror.ErrorTypeInternal,
		domainerror.CodeInternal,
		message,
		statusCode,
	)
	if reqID != "" {
		llmErr = llmErr.WithReqID(reqID)
	}
	domainerror.WriteJSONError(w, llmErr)
}

func (ep *ErrorPresenter) logError(err *domainerror.LLMProxyError) {
	fields := []port.Field{
		port.String("error_type", string(err.Type)),
		port.String("error_code", string(err.Code)),
		port.String("message", err.Message),
	}

	if err.ReqID != "" {
		fields = append(fields, port.String("req_id", err.ReqID))
	}
	if err.BackendName != "" {
		fields = append(fields, port.String("backend", err.BackendName))
	}

	switch err.Type {
	case domainerror.ErrorTypeValidation, domainerror.ErrorTypeClient:
		ep.logger.Warn("request validation failed", fields...)
	case domainerror.ErrorTypeBackend:
		ep.logger.Error("backend error", fields...)
		if err.Cause != nil {
			ep.logger.Error("error cause", port.Error(err.Cause))
		}
	case domainerror.ErrorTypeInternal:
		ep.logger.Error("internal error", fields...)
		if err.Cause != nil {
			ep.logger.Error("error cause", port.Error(err.Cause))
		}
	default:
		ep.logger.Error("unknown error", fields...)
		if err.Cause != nil {
			ep.logger.Error("error cause", port.Error(err.Cause))
		}
	}
}

func extractReqID(r *http.Request) string {
	if reqID := r.Header.Get("X-Request-ID"); reqID != "" {
		return reqID
	}
	if reqID := r.Header.Get("X-Trace-ID"); reqID != "" {
		return reqID
	}
	if reqID := r.Context().Value("req_id"); reqID != nil {
		if id, ok := reqID.(string); ok {
			return id
		}
	}
	return ""
}
