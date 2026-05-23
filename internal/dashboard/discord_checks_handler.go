package dashboard

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/go-chi/chi/v5"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/discordcheck"
)

// discordChecksResponse mirrors discordcheck.Report on the wire and adds a
// Disabled flag so the Svelte banner can distinguish "Discord is off in this
// deploy" from "checks ran and everything is OK". When Disabled is true the
// banner must stay hidden regardless of the (empty) Results slice.
type discordChecksResponse struct {
	Results  []discordcheck.Result `json:"results"`
	RanAt    string                `json:"ran_at,omitempty"`
	Disabled bool                  `json:"disabled,omitempty"`
}

// DiscordChecksHandler serves GET /api/dashboard/discord-checks. The latest
// Report is read out of a holder pointer so a single startup run can publish
// findings the dashboard reads on every page load without re-hitting Discord.
type DiscordChecksHandler struct {
	logger *slog.Logger
	holder *atomic.Pointer[discordcheck.Report]
}

// NewDiscordChecksHandler constructs a handler that reads from the given
// report holder. A nil holder is treated the same as "no report stored" —
// the endpoint responds with the disabled envelope so the SPA renders no
// banner instead of erroring.
func NewDiscordChecksHandler(logger *slog.Logger, holder *atomic.Pointer[discordcheck.Report]) *DiscordChecksHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &DiscordChecksHandler{logger: logger, holder: holder}
}

// ServeHTTP returns the latest discordcheck.Report as JSON. 401 when no auth
// context is present, 200 otherwise. The response envelope always has a
// non-null results array so the Svelte banner can safely iterate.
func (h *DiscordChecksHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	resp := discordChecksResponse{Results: []discordcheck.Result{}}

	if h.holder == nil {
		resp.Disabled = true
		h.writeJSON(w, resp)
		return
	}

	report := h.holder.Load()
	if report == nil {
		resp.Disabled = true
		h.writeJSON(w, resp)
		return
	}

	resp.Results = report.Results
	if resp.Results == nil {
		resp.Results = []discordcheck.Result{}
	}
	resp.RanAt = report.RanAt.Format("2006-01-02T15:04:05Z07:00")

	h.writeJSON(w, resp)
}

func (h *DiscordChecksHandler) writeJSON(w http.ResponseWriter, resp discordChecksResponse) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("/api/dashboard/discord-checks encode failed", "error", err)
	}
}

// RegisterDiscordChecksRoute mounts GET /api/dashboard/discord-checks behind
// authMiddleware. The Svelte DiscordHealthBanner calls this on mount to
// decide whether to surface a configuration warning at the top of the UI.
func RegisterDiscordChecksRoute(r chi.Router, h *DiscordChecksHandler, authMiddleware func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/api/dashboard/discord-checks", h.ServeHTTP)
	})
}
