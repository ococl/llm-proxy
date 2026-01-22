package http

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"llm-proxy/domain/port"
)

type RecoveryMiddleware struct {
	logger port.Logger
}

func NewRecoveryMiddleware(logger port.Logger) *RecoveryMiddleware {
	return &RecoveryMiddleware{logger: logger}
}

func (rm *RecoveryMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				reqID := extractRequestID(r)
				stack := debug.Stack()
				stackStr := string(stack)
				if len(stackStr) > 500 {
					stackStr = stackStr[:500] + "..."
				}

				rm.logger.Error("Panic recovered",
					port.String("reqID", reqID),
					port.String("error", fmt.Sprintf("%v", err)),
					port.String("stack", stackStr),
				)

				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func extractRequestID(r *http.Request) string {
	reqID := r.Header.Get("X-Request-ID")
	if reqID == "" {
		reqID = r.Header.Get("X-Trace-ID")
	}
	if reqID == "" {
		reqID = "unknown"
	}
	return reqID
}
