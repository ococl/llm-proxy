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
				stack := debug.Stack()
				LogGeneral("ERROR", "Panic recovered: %v\n%s", err, stack)
				http.Error(w, fmt.Sprintf("Internal Server Error"), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
