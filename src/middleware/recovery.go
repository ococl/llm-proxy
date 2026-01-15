package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"llm-proxy/logging"
)

func extractRequestID(r *http.Request) string {
	reqID := r.Header.Get("X-Request-ID")
	if reqID == "" {
		reqID = "unknown"
	}
	return reqID
}

func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				reqID := extractRequestID(r)
				stack := debug.Stack()
				stackStr := string(stack)
				if len(stackStr) > 500 {
					stackStr = stackStr[:500] + "..."
				}
				logging.SystemSugar.Errorw("Panic recovered", "reqID", reqID, "error", err, "stack", stackStr)
				http.Error(w, fmt.Sprintf("Internal Server Error"), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
