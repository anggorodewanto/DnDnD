package dashboard

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ab/dndnd/internal/errorlog"
)

// ErrorsHandler exposes the recent error log as JSON for the Svelte dashboard
// Errors panel.
type ErrorsHandler struct {
	logger *slog.Logger
	reader errorlog.Reader
	clock  func() time.Time
}

// NewErrorsHandler constructs an ErrorsHandler. Pass time.Now for clock
// unless a test needs determinism.
func NewErrorsHandler(logger *slog.Logger, reader errorlog.Reader, clock func() time.Time) *ErrorsHandler {
	if logger == nil {
		logger = slog.Default()
	}
	if clock == nil {
		clock = time.Now
	}
	return &ErrorsHandler{
		logger: logger,
		reader: reader,
		clock:  clock,
	}
}

// errorEntryView is the JSON shape returned by GET /api/errors. Fields mirror
// errorlog.Entry minus Detail (which is intended for separate drill-down).
type errorEntryView struct {
	Timestamp string `json:"timestamp"`
	Command   string `json:"command"`
	UserID    string `json:"user_id"`
	Summary   string `json:"summary"`
}

// errorsListResponse is the wrapper object {errors: [...]} returned by the
// JSON endpoint so the SPA can extend the payload (counts, paging) without a
// breaking change.
type errorsListResponse struct {
	Errors []errorEntryView `json:"errors"`
}

// ServeJSON handles GET /api/errors. Returns the most recent entries from
// the last 24h (limit 100) wrapped in {errors: [...]}. The 24h cap matches
// the Campaign Home badge so the panel and badge never disagree about which
// window of failures the DM is reviewing.
func (h *ErrorsHandler) ServeJSON(w http.ResponseWriter, r *http.Request) {
	if !hasAuthUser(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	entries := h.recentErrors(r.Context())
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(errorsListResponse{Errors: entries}); err != nil {
		h.logger.Error("errors json encode failed", "error", err)
	}
}

// recentErrors fetches the recent entries and converts them to the wire shape.
// Best-effort: any reader error degrades to an empty slice so the panel always
// renders.
func (h *ErrorsHandler) recentErrors(ctx context.Context) []errorEntryView {
	if h.reader == nil {
		return []errorEntryView{}
	}
	since := h.clock().Add(-24 * time.Hour)
	raw, err := h.reader.ListRecent(ctx, 100)
	if err != nil {
		h.logger.Error("errorlog list-recent failed", "error", err)
		return []errorEntryView{}
	}
	out := make([]errorEntryView, 0, len(raw))
	for _, e := range raw {
		if e.CreatedAt.Before(since) {
			continue
		}
		out = append(out, errorEntryView{
			Timestamp: e.CreatedAt.UTC().Format(time.RFC3339),
			Command:   e.Command,
			UserID:    e.UserID,
			Summary:   e.Summary,
		})
	}
	return out
}

// RegisterErrorsRoutes mounts GET /api/errors behind authMiddleware (the DM
// auth chain in production).
func RegisterErrorsRoutes(r chi.Router, h *ErrorsHandler, authMiddleware func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/api/errors", h.ServeJSON)
	})
}

// MountErrorsRoutes is the one-call convenience wiring for main.go: it
// builds an ErrorsHandler and mounts GET /api/errors behind authMiddleware.
func MountErrorsRoutes(r chi.Router, dash *Handler, reader errorlog.Reader, clock func() time.Time, authMiddleware func(http.Handler) http.Handler) *ErrorsHandler {
	_ = dash
	if clock == nil {
		clock = time.Now
	}
	handler := NewErrorsHandler(nil, reader, clock)
	RegisterErrorsRoutes(r, handler, authMiddleware)
	return handler
}
