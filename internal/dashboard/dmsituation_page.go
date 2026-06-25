package dashboard

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/situation"
)

// SituationBuilder builds the aggregated DM view for one campaign. The
// production implementation is *situation.Service; tests supply a fake. A nil
// builder leaves the endpoint returning an empty Situation (still 200) so the
// SPA renders cleanly in passthrough-auth dev mode.
type SituationBuilder interface {
	Build(ctx context.Context, campaignID string) (situation.Situation, error)
}

// DMSituationHandler serves GET /api/dm/situation — the single aggregated
// "DM Console" payload (pending work, live state, recent timeline, next step)
// consumed by the Svelte DM Console panel and the AI DM. It exists so a DM
// answers "what needs my action / where are we / what just happened" from one
// place instead of six.
type DMSituationHandler struct {
	logger         *slog.Logger
	builder        SituationBuilder
	campaignLookup CampaignLookup
}

// NewDMSituationHandler constructs a DMSituationHandler.
func NewDMSituationHandler(logger *slog.Logger) *DMSituationHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &DMSituationHandler{logger: logger}
}

// SetBuilder wires the situation aggregator. Pair with SetCampaignLookup so the
// handler can resolve the DM's active campaign per request. Either nil leaves
// the endpoint returning an empty Situation (still 200).
func (h *DMSituationHandler) SetBuilder(b SituationBuilder) { h.builder = b }

// SetCampaignLookup wires the per-request campaign resolver, reusing the same
// CampaignLookup interface the dashboard.Handler uses.
func (h *DMSituationHandler) SetCampaignLookup(lookup CampaignLookup) { h.campaignLookup = lookup }

// ServeSituation renders GET /api/dm/situation as the JSON Situation for the
// authenticated DM's active campaign.
//
// The route is mounted behind dmAuthMw so non-DM authenticated users already
// receive a 403 before this handler runs; the session-user check here is a
// defensive backstop for bare test mounts. Every downstream failure degrades to
// an empty (or partial) Situation with HTTP 200 so the panel always renders.
func (h *DMSituationHandler) ServeSituation(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sit := h.buildForUser(r.Context(), userID)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(sit); err != nil {
		h.logger.Error("dm situation encode", "error", err)
	}
}

// buildForUser resolves the DM's active campaign and builds its Situation.
// Best-effort: any missing wiring, unknown campaign, or lookup error degrades
// to an empty Situation; a partial-build error from the aggregator is logged
// but the partial Situation is still returned so the panel renders.
func (h *DMSituationHandler) buildForUser(ctx context.Context, dmUserID string) situation.Situation {
	empty := situation.Situation{Pending: []situation.PendingItem{}, Timeline: []situation.TimelineEvent{}}
	if h.builder == nil || h.campaignLookup == nil {
		return empty
	}
	idStr, _, err := h.campaignLookup.LookupActiveCampaign(ctx, dmUserID)
	if err != nil {
		h.logger.Warn("dm situation: campaign lookup failed", "error", err)
		return empty
	}
	if idStr == "" {
		return empty
	}
	sit, err := h.builder.Build(ctx, idStr)
	if err != nil {
		// Best-effort: the aggregator joins per-source failures but still
		// returns whatever it could assemble. Log and serve the partial view.
		h.logger.Warn("dm situation: partial build", "error", err)
	}
	return sit
}
