package dashboard

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ab/dndnd/internal/auth"
)

// HomeResponse is the JSON payload returned by GET /api/dashboard/home.
// All fields degrade to zero values when the DM has no active campaign or
// the underlying stores are unreachable — the Svelte HomePanel must always
// render so the dashboard shell stays usable even mid-outage.
type HomeResponse struct {
	CampaignID       string   `json:"campaign_id"`
	CampaignStatus   string   `json:"campaign_status"`
	DMQueueCount     int      `json:"dm_queue_count"`
	PendingApprovals int      `json:"pending_approvals"`
	ActiveEncounters []string `json:"active_encounters"`
	SavedEncounters  []string `json:"saved_encounters"`
}

// HomeHandler serves GET /api/dashboard/home. It reuses the lookup helpers
// from the main dashboard Handler so the Svelte Campaign Home panel sees the
// exact same data shape the legacy template produced.
type HomeHandler struct {
	logger *slog.Logger
	dash   *Handler
}

// NewHomeHandler constructs a HomeHandler. The dash argument provides the
// CampaignLookup / counter / encounter dependencies; pass the same *Handler
// that other dashboard routes already share so a single set of setters wires
// them all.
func NewHomeHandler(logger *slog.Logger, dash *Handler) *HomeHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &HomeHandler{logger: logger, dash: dash}
}

// ServeHTTP returns the Campaign Home payload as JSON. Auth: same session-only
// middleware as /api/me — the response itself carries the empty campaign_id
// when the user is not yet a DM so the SPA can render an empty state.
func (h *HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	resp := HomeResponse{
		ActiveEncounters: []string{},
		SavedEncounters:  []string{},
	}

	if h.dash == nil {
		h.writeJSON(w, resp)
		return
	}

	ctx := r.Context()
	campaignID, status := h.dash.lookupCampaign(ctx, userID)
	resp.CampaignID = campaignID
	resp.CampaignStatus = status

	approvals, dmQueue := h.dash.lookupCounts(ctx, campaignID)
	resp.PendingApprovals = approvals
	resp.DMQueueCount = dmQueue

	active, saved := h.dash.lookupEncounters(ctx, campaignID)
	if active != nil {
		resp.ActiveEncounters = active
	}
	if saved != nil {
		resp.SavedEncounters = saved
	}

	h.writeJSON(w, resp)
}

func (h *HomeHandler) writeJSON(w http.ResponseWriter, resp HomeResponse) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("/api/dashboard/home encode failed", "error", err)
	}
}

// RegisterHomeRoute mounts GET /api/dashboard/home behind authMiddleware.
// The Svelte HomePanel calls this on mount to populate the Campaign Home cards.
func RegisterHomeRoute(r chi.Router, h *HomeHandler, authMiddleware func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/api/dashboard/home", h.ServeHTTP)
	})
}
