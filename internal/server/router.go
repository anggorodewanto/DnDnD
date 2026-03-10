package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// NewRouter creates a Chi router with panic recovery middleware and health endpoint.
func NewRouter(logger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(PanicRecovery(logger))

	health := NewHealthHandler()
	r.Get("/health", health.ServeHTTP)

	return r
}
