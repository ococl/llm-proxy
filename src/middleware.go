package main

import (
	"fmt"
	"net/http"
	"runtime/debug"
)

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
				LogGeneral("ERROR", "[%s] Panic recovered: %v\n%s", reqID, err, stackStr)
				http.Error(w, fmt.Sprintf("Internal Server Error"), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func extractRequestID(r *http.Request) string {
	reqID := r.Header.Get("X-Request-ID")
	if reqID == "" {
		reqID = "unknown"
	}
	return reqID
}
