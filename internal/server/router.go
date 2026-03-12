package server

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
)

// NewRouter creates a Chi router with panic recovery middleware and health endpoint.
// It returns both the router and the HealthHandler so callers can register subsystem checks.
func NewRouter(logger *slog.Logger) (chi.Router, *HealthHandler) {
	r := chi.NewRouter()

	r.Use(PanicRecovery(logger))

	health := NewHealthHandler()
	r.Get("/health", health.ServeHTTP)

	return r, health
}
