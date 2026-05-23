package dashboard

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/auth"
)

// PendingApprovalsCounter returns the number of pending character approvals
// for the given campaign. Implementations typically wrap ApprovalStore.
// Errors are logged and the count degrades to 0 — Campaign Home must keep
// rendering even when the underlying store is unreachable (med-40).
type PendingApprovalsCounter interface {
	CountPendingApprovals(ctx context.Context, campaignID uuid.UUID) (int, error)
}

// DMQueueCounter returns the number of pending DM queue items for the given
// campaign. Same degrade-on-error contract as PendingApprovalsCounter.
type DMQueueCounter interface {
	CountPendingDMQueue(ctx context.Context, campaignID uuid.UUID) (int, error)
}

// CampaignLookup returns the (id, status) of the campaign the DM is currently
// operating on. Phase 115 uses this to drive the Pause/Resume button state in
// the dashboard shell. Implementations typically key on the DM's discord user
// id resolved from the request context. Either return id=="" with no error
// when the DM has no active campaign, or propagate errors if the backing
// store is unreachable — the handler will silently degrade in that case.
type CampaignLookup interface {
	LookupActiveCampaign(ctx context.Context, dmUserID string) (id, status string, err error)
}

// EncounterLister returns active encounters and saved templates for a campaign.
type EncounterLister interface {
	ListActiveEncounterNames(ctx context.Context, campaignID uuid.UUID) ([]string, error)
	ListSavedEncounterNames(ctx context.Context, campaignID uuid.UUID) ([]string, error)
}

// EncounterCampaignResolver resolves the campaign that owns a given encounter.
// Used by ServeWebSocket to verify the authenticated DM owns the encounter's
// campaign before subscribing.
type EncounterCampaignResolver interface {
	GetEncounterCampaignID(ctx context.Context, encounterID uuid.UUID) (uuid.UUID, error)
}

// Handler serves the DM dashboard pages.
type Handler struct {
	logger *slog.Logger
	hub    *Hub
	// Phase 115: optional campaign lookup so the Pause/Resume button can
	// render with the right label and endpoint target.
	campaignLookup CampaignLookup
	// med-40 / Phase 15: optional counters for the live Campaign Home
	// pending DM-queue / pending-approval cards. When unset both fall back
	// to 0 (the original placeholder behaviour).
	approvalsCounter PendingApprovalsCounter
	dmQueueCounter   DMQueueCounter
	// Finding 13: optional encounter lister for active/saved encounter data.
	encounterLister EncounterLister
	// J-C01: optional encounter-campaign resolver for WebSocket ownership check.
	encounterCampaignResolver EncounterCampaignResolver
	// SR-016: WebSocket origin policy. wsInsecureSkipVerify=true keeps the
	// historical permissive dev behaviour (any Origin accepted). When false,
	// ServeWebSocket relies on nhooyr/websocket's built-in same-host check
	// plus wsAllowedOrigins (matched via filepath.Match against the request
	// Origin host) to reject cross-origin upgrade attempts with HTTP 403.
	// A-H03: Defaults to false (strict). Local-dev wiring must opt in via
	// SetWebSocketOriginPolicy(nil, true).
	wsAllowedOrigins     []string
	wsInsecureSkipVerify bool
}

// SetCampaignLookup wires a CampaignLookup so the Pause/Resume button can
// reflect current state. Pass nil to disable (button still renders but is
// inert). Safe to call at any time before the handler starts serving.
func (h *Handler) SetCampaignLookup(lookup CampaignLookup) {
	h.campaignLookup = lookup
}

// SetCounters wires the live Campaign Home pending-approval and
// pending-DM-queue counts. Either or both may be nil; nil counters keep the
// historical 0-placeholder behaviour. med-40 closes the gap that the cards
// always read 0 even after Phase 16 shipped a real approval store.
func (h *Handler) SetCounters(approvals PendingApprovalsCounter, dmQueue DMQueueCounter) {
	h.approvalsCounter = approvals
	h.dmQueueCounter = dmQueue
}

// SetEncounterLister wires the encounter lister for Campaign Home active/saved
// encounter data. When nil, the cards show empty lists.
func (h *Handler) SetEncounterLister(lister EncounterLister) {
	h.encounterLister = lister
}

// SetEncounterCampaignResolver wires the encounter-campaign ownership resolver
// used by ServeWebSocket to reject connections to encounters the DM does not own.
func (h *Handler) SetEncounterCampaignResolver(resolver EncounterCampaignResolver) {
	h.encounterCampaignResolver = resolver
}

// NewHandler creates a new dashboard Handler with an optional Hub for WebSocket support.
func NewHandler(logger *slog.Logger, hub *Hub) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		logger: logger,
		hub:    hub,
		// A-H03: default to strict origin checking. Local-dev and test
		// callers that need the permissive behaviour must explicitly call
		// SetWebSocketOriginPolicy(nil, true).
		wsInsecureSkipVerify: false,
	}
}

// SetWebSocketOriginPolicy configures the WebSocket upgrade origin check.
//
// SR-016: production deploys should pass insecureSkipVerify=false plus a
// non-empty allowedOrigins list (e.g. []string{"dashboard.example.com"})
// so cross-origin upgrade attempts are rejected with HTTP 403 by
// nhooyr/websocket's authenticateOrigin. Dev/local deploys may pass
// insecureSkipVerify=true to keep the historical permissive behaviour.
//
// allowedOrigins entries are matched case-insensitively against the
// request Origin host via filepath.Match (the nhooyr default). Same-host
// requests are always authorised regardless of this list.
func (h *Handler) SetWebSocketOriginPolicy(allowedOrigins []string, insecureSkipVerify bool) {
	h.wsAllowedOrigins = allowedOrigins
	h.wsInsecureSkipVerify = insecureSkipVerify
}

// lookupCounts resolves the live Campaign Home counts. Best-effort: any
// missing counter, missing/invalid campaign id, or backing-store error
// degrades silently to 0 so the dashboard always renders. (med-40)
func (h *Handler) lookupCounts(ctx context.Context, campaignID string) (approvals, dmQueue int) {
	if campaignID == "" {
		return 0, 0
	}
	cid, err := uuid.Parse(campaignID)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("dashboard counts: invalid campaign id", "id", campaignID, "error", err)
		}
		return 0, 0
	}
	if h.approvalsCounter != nil {
		n, err := h.approvalsCounter.CountPendingApprovals(ctx, cid)
		if err != nil {
			if h.logger != nil {
				h.logger.Warn("dashboard pending approvals count failed", "error", err)
			}
		} else {
			approvals = n
		}
	}
	if h.dmQueueCounter != nil {
		n, err := h.dmQueueCounter.CountPendingDMQueue(ctx, cid)
		if err != nil {
			if h.logger != nil {
				h.logger.Warn("dashboard dm-queue count failed", "error", err)
			}
		} else {
			dmQueue = n
		}
	}
	return approvals, dmQueue
}

// lookupCampaign is a best-effort wrapper around the configured CampaignLookup
// that tolerates nil lookup and errors by returning empty strings.
func (h *Handler) lookupCampaign(ctx context.Context, dmUserID string) (string, string) {
	if h.campaignLookup == nil {
		return "", ""
	}
	id, status, err := h.campaignLookup.LookupActiveCampaign(ctx, dmUserID)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("dashboard campaign lookup failed", "error", err)
		}
		return "", ""
	}
	return id, status
}

// lookupEncounters resolves active and saved encounter names for the Campaign
// Home cards. Best-effort: nil lister or errors degrade to empty slices.
func (h *Handler) lookupEncounters(ctx context.Context, campaignID string) ([]string, []string) {
	if h.encounterLister == nil || campaignID == "" {
		return []string{}, []string{}
	}
	cid, err := uuid.Parse(campaignID)
	if err != nil {
		return []string{}, []string{}
	}
	active, err := h.encounterLister.ListActiveEncounterNames(ctx, cid)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("dashboard active encounters lookup failed", "error", err)
		}
		active = []string{}
	}
	saved, err := h.encounterLister.ListSavedEncounterNames(ctx, cid)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("dashboard saved encounters lookup failed", "error", err)
		}
		saved = []string{}
	}
	return active, saved
}

// ServeDashboard serves the dashboard shell with Campaign Home as the default view.
func (h *Handler) ServeDashboard(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	_ = userID
	http.Redirect(w, r, "/dashboard/app/#home", http.StatusFound)
}
