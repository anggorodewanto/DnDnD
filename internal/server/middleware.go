package server

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

var panicResponseBody = []byte(`{"error":"internal server error"}`)

// PanicRecovery returns middleware that recovers from panics, logs the stack
// trace at ERROR level, and returns a 500 JSON error response.
func PanicRecovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					stack := debug.Stack()
					logger.Error("panic recovered",
						"error", err,
						"stack", string(stack),
						"method", r.Method,
						"path", r.URL.Path,
					)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write(panicResponseBody)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
