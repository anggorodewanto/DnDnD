package server

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/ab/dndnd/internal/errorlog"
)

// NewRouter creates a Chi router with panic recovery middleware and health endpoint.
// It returns both the router and the HealthHandler so callers can register subsystem checks.
// If recorder is non-nil, HTTP panics are also recorded in the error log (Finding 4).
func NewRouter(logger *slog.Logger, recorder errorlog.Recorder) (chi.Router, *HealthHandler) {
	r := chi.NewRouter()

	r.Use(PanicRecoveryWithRecorder(logger, recorder))

	health := NewHealthHandler()
	r.Get("/health", health.ServeHTTP)

	return r, health
}
