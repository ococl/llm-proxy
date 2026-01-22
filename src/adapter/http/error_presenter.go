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
	traceID := extractTraceID(r)

	llmErr, ok := err.(*domainerror.LLMProxyError)
	if !ok {
		llmErr = domainerror.NewInternalError("unexpected error", err)
	}

	if traceID != "" {
		llmErr = llmErr.WithTraceID(traceID)
	}

	ep.logError(llmErr)

	domainerror.WriteJSONError(w, llmErr)
}

func (ep *ErrorPresenter) WriteJSONError(w http.ResponseWriter, message string, statusCode int, traceID string) {
	llmErr := domainerror.NewWithStatus(
		domainerror.ErrorTypeInternal,
		domainerror.CodeInternal,
		message,
		statusCode,
	)
	if traceID != "" {
		llmErr = llmErr.WithTraceID(traceID)
	}
	domainerror.WriteJSONError(w, llmErr)
}

func (ep *ErrorPresenter) logError(err *domainerror.LLMProxyError) {
	fields := []port.Field{
		port.String("error_type", string(err.Type)),
		port.String("error_code", string(err.Code)),
		port.String("message", err.Message),
	}

	if err.TraceID != "" {
		fields = append(fields, port.String("trace_id", err.TraceID))
	}
	if err.BackendName != "" {
		fields = append(fields, port.String("backend", err.BackendName))
	}

	switch err.Type {
	case domainerror.ErrorTypeValidation, domainerror.ErrorTypeClient:
		ep.logger.Warn("request validation failed", fields...)
	case domainerror.ErrorTypeBackend:
		ep.logger.Error("backend error", fields...)
	case domainerror.ErrorTypeInternal:
		ep.logger.Error("internal error", fields...)
		if err.Cause != nil {
			ep.logger.Error("error cause", port.Error(err.Cause))
		}
	default:
		ep.logger.Error("unknown error", fields...)
	}
}

func extractTraceID(r *http.Request) string {
	if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
		return traceID
	}
	if traceID := r.Header.Get("X-Request-ID"); traceID != "" {
		return traceID
	}
	if traceID := r.Context().Value("trace_id"); traceID != nil {
		if id, ok := traceID.(string); ok {
			return id
		}
	}
	return ""
}
