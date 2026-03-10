package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// NewRouter creates a Chi router with panic recovery middleware and health endpoint.
// It returns both the router and the HealthHandler so callers can register subsystem checks.
func NewRouter(logger *slog.Logger) (http.Handler, *HealthHandler) {
	r := chi.NewRouter()

	r.Use(PanicRecovery(logger))

	health := NewHealthHandler()
	r.Get("/health", health.ServeHTTP)

	return r, health
}
