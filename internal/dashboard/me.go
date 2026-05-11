package dashboard

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ab/dndnd/internal/auth"
)

// MeResolver resolves the active campaign for a Discord user. It is the same
// shape as CampaignLookup but exposed as its own type so the /api/me endpoint
// can be wired and tested independently of the dashboard shell handler.
type MeResolver interface {
	LookupActiveCampaign(ctx context.Context, dmUserID string) (id, status string, err error)
}

// MeResponse is the JSON payload returned by GET /api/me. CampaignID is empty
// when the authenticated user has no active campaign yet (they hit the
// dashboard before running /setup, for example) — callers should treat the
// empty string as "no active campaign" and degrade UI accordingly.
//
// med-39 / Phase 21a wires this so dashboard/svelte/src/App.svelte can
// replace its hard-coded placeholder UUID with a real per-DM campaign id.
type MeResponse struct {
	DiscordUserID string `json:"discord_user_id"`
	CampaignID    string `json:"campaign_id"`
	Status        string `json:"status,omitempty"`
}

// MeHandler serves GET /api/me. The response carries the authenticated DM's
// active campaign id so the Svelte SPA can route per-campaign requests
// without the long-standing hard-coded placeholder UUID.
type MeHandler struct {
	logger   *slog.Logger
	resolver MeResolver
}

// NewMeHandler constructs a MeHandler. resolver may be nil — the handler
// then degrades to returning the user id with an empty campaign id (still a
// 200 OK so the SPA can render its "no campaign" empty-state).
func NewMeHandler(logger *slog.Logger, resolver MeResolver) *MeHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &MeHandler{logger: logger, resolver: resolver}
}

// ServeHTTP returns the authenticated user's id + active campaign id as
// JSON. 401 when no auth context is present, 200 otherwise (with empty
// campaign_id when no campaign is wired or the resolver returned an error).
func (h *MeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	resp := MeResponse{DiscordUserID: userID}
	if h.resolver != nil {
		id, status, err := h.resolver.LookupActiveCampaign(r.Context(), userID)
		if err != nil {
			h.logger.Warn("/api/me campaign lookup failed", "error", err)
		} else {
			resp.CampaignID = id
			resp.Status = status
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("/api/me encode failed", "error", err)
	}
}

// RegisterMeRoute mounts GET /api/me behind authMiddleware. The Svelte SPA
// fetches this on boot to discover the DM's active campaign id (med-39).
func RegisterMeRoute(r chi.Router, h *MeHandler, authMiddleware func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/api/me", h.ServeHTTP)
	})
}
