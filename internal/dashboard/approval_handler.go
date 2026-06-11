package dashboard

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/ab/dndnd/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ApprovalHandler serves the character approval queue JSON API endpoints.
//
// The handler resolves the campaign for ListPendingApprovals from one of two
// sources, in order: a CampaignLookup wired via SetCampaignLookup (per-request,
// keyed by the authenticated DM's discord user id) or the construction-time
// campaignID. Production wires a lookup; tests usually pass a fixed campaignID.
type ApprovalHandler struct {
	logger         *slog.Logger
	store          ApprovalStore
	notifier       PlayerNotifier
	cardPoster     CharacterCardPoster
	hub            *Hub
	campaignID     uuid.UUID
	campaignLookup CampaignLookup
}

// NewApprovalHandler creates a new ApprovalHandler.
func NewApprovalHandler(logger *slog.Logger, store ApprovalStore, notifier PlayerNotifier, hub *Hub, campaignID uuid.UUID, cardPoster CharacterCardPoster) *ApprovalHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ApprovalHandler{
		logger:     logger,
		store:      store,
		notifier:   notifier,
		cardPoster: cardPoster,
		hub:        hub,
		campaignID: campaignID,
	}
}

// RegisterApprovalRoutes mounts approval API routes on the given router.
func (ah *ApprovalHandler) RegisterApprovalRoutes(r chi.Router) {
	r.Route("/dashboard/api/approvals", func(r chi.Router) {
		r.Get("/", ah.ListApprovals)
		r.Get("/{id}", ah.GetApproval)
		r.Post("/{id}/approve", ah.Approve)
		r.Post("/{id}/request-changes", ah.RequestChangesHandler)
		r.Post("/{id}/reject", ah.Reject)
	})
}

func (ah *ApprovalHandler) requireAuth(r *http.Request) (string, bool) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		return "", false
	}
	return userID, true
}

func (ah *ApprovalHandler) parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return uuid.UUID{}, false
	}
	return id, true
}

// checkCampaignOwnership verifies the approval belongs to the DM's campaign.
// Returns false (and writes 403) if the DM does not own the approval's campaign.
func (ah *ApprovalHandler) checkCampaignOwnership(w http.ResponseWriter, r *http.Request, dmUserID string, detail *ApprovalDetail) bool {
	dmCampaign, ok := ah.resolveCampaign(w, r, dmUserID)
	if !ok {
		return false
	}
	if detail.CampaignID != dmCampaign {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

func (ah *ApprovalHandler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// parseFeedbackRequest validates auth, parses the ID and feedback body.
// Returns the parsed ID, detail, and feedback string. On failure it writes the
// HTTP error response and returns ok=false.
func (ah *ApprovalHandler) parseFeedbackRequest(w http.ResponseWriter, r *http.Request) (detail *ApprovalDetail, feedback string, ok bool) {
	userID, authOK := ah.requireAuth(r)
	if !authOK {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, "", false
	}

	id, idOK := ah.parseID(w, r)
	if !idOK {
		return nil, "", false
	}

	var req feedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return nil, "", false
	}

	if req.Feedback == "" {
		http.Error(w, "feedback is required", http.StatusBadRequest)
		return nil, "", false
	}

	d, err := ah.store.GetApprovalDetail(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return nil, "", false
	}

	if !ah.checkCampaignOwnership(w, r, userID, d) {
		return nil, "", false
	}

	return d, req.Feedback, true
}

// SetCampaignLookup installs a per-request campaign lookup. When set, the
// lookup result replaces the construction-time campaignID for
// ListPendingApprovals so one handler instance can serve every DM. Reuses
// the same CampaignLookup interface as dashboard.Handler.
func (ah *ApprovalHandler) SetCampaignLookup(lookup CampaignLookup) {
	ah.campaignLookup = lookup
}

// resolveCampaign returns the campaign id ListPendingApprovals should query.
// On lookup error it logs and writes a 500; callers must check ok.
func (ah *ApprovalHandler) resolveCampaign(w http.ResponseWriter, r *http.Request, dmUserID string) (uuid.UUID, bool) {
	if ah.campaignLookup == nil {
		return ah.campaignID, true
	}
	idStr, _, err := ah.campaignLookup.LookupActiveCampaign(r.Context(), dmUserID)
	if err != nil {
		ah.logger.Error("failed to resolve campaign for DM", "error", err, "discord_user_id", dmUserID)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return uuid.Nil, false
	}
	if idStr == "" {
		// No active campaign yet (e.g. pre-/setup) — empty list, not error.
		return uuid.Nil, true
	}
	parsed, err := uuid.Parse(idStr)
	if err != nil {
		ah.logger.Error("campaign lookup returned invalid uuid", "error", err, "id", idStr)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return uuid.Nil, false
	}
	return parsed, true
}

// ListApprovals returns all pending approvals as JSON.
func (ah *ApprovalHandler) ListApprovals(w http.ResponseWriter, r *http.Request) {
	userID, ok := ah.requireAuth(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	campaignID, ok := ah.resolveCampaign(w, r, userID)
	if !ok {
		return
	}

	entries, err := ah.store.ListPendingApprovals(r.Context(), campaignID)
	if err != nil {
		ah.logger.Error("failed to list pending approvals", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	ah.writeJSON(w, http.StatusOK, entries)
}

// GetApproval returns a single approval detail as JSON.
func (ah *ApprovalHandler) GetApproval(w http.ResponseWriter, r *http.Request) {
	if _, ok := ah.requireAuth(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, ok := ah.parseID(w, r)
	if !ok {
		return
	}

	detail, err := ah.store.GetApprovalDetail(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	ah.writeJSON(w, http.StatusOK, detail)
}

// feedbackRequest is the JSON body for request-changes and reject.
type feedbackRequest struct {
	Feedback string `json:"feedback"`
}

// Approve approves a pending character. For retirement submissions (created_via="retire"),
// it transitions to "retired" status and updates the character card with a retired badge.
func (ah *ApprovalHandler) Approve(w http.ResponseWriter, r *http.Request) {
	userID, ok := ah.requireAuth(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, ok := ah.parseID(w, r)
	if !ok {
		return
	}

	// Get detail for notification
	detail, err := ah.store.GetApprovalDetail(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if !ah.checkCampaignOwnership(w, r, userID, detail) {
		return
	}

	isRetire := detail.CreatedVia == "retire"

	if isRetire {
		if err := ah.store.RetireCharacter(r.Context(), id); err != nil {
			ah.logger.Error("failed to retire character", "error", err, "id", id)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	} else {
		if err := ah.store.ApproveCharacter(r.Context(), id); err != nil {
			ah.logger.Error("failed to approve character", "error", err, "id", id)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Notify player
	if ah.notifier != nil {
		if err := ah.notifier.NotifyApproval(r.Context(), detail.DiscordUserID, detail.CharacterName); err != nil {
			ah.logger.Error("failed to notify player of approval", "error", err, "discord_user_id", detail.DiscordUserID)
		}
	}

	// Post/update character card
	if ah.cardPoster != nil {
		if isRetire {
			if err := ah.cardPoster.UpdateCardRetired(r.Context(), detail.CharacterID, detail.CharacterName, detail.DiscordUserID); err != nil {
				ah.logger.Error("failed to update character card with retired badge", "error", err, "character_id", detail.CharacterID)
			}
		} else {
			if err := ah.cardPoster.PostCharacterCard(r.Context(), detail.CharacterID, detail.CharacterName, detail.DiscordUserID); err != nil {
				ah.logger.Error("failed to post character card", "error", err, "character_id", detail.CharacterID)
			}
		}
	}

	// Broadcast update via WebSocket
	ah.broadcastUpdate("approval_updated", id)

	status := "approved"
	if isRetire {
		status = "retired"
	}
	ah.writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

// RequestChangesHandler requests changes on a pending character.
func (ah *ApprovalHandler) RequestChangesHandler(w http.ResponseWriter, r *http.Request) {
	detail, feedback, ok := ah.parseFeedbackRequest(w, r)
	if !ok {
		return
	}

	if err := ah.store.RequestChanges(r.Context(), detail.ID, feedback); err != nil {
		ah.logger.Error("failed to request changes", "error", err, "id", detail.ID)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var notifyErr string
	if ah.notifier != nil {
		if err := ah.notifier.NotifyChangesRequested(r.Context(), detail.DiscordUserID, detail.CharacterName, feedback); err != nil {
			ah.logger.Error("failed to notify player of changes requested", "error", err, "discord_user_id", detail.DiscordUserID)
			notifyErr = playerNotifyFailureMessage
		}
	}

	ah.broadcastUpdate("approval_updated", detail.ID)
	ah.writeStatusWithNotify(w, "changes_requested", notifyErr)
}

// Reject rejects a pending character.
func (ah *ApprovalHandler) Reject(w http.ResponseWriter, r *http.Request) {
	detail, feedback, ok := ah.parseFeedbackRequest(w, r)
	if !ok {
		return
	}

	if err := ah.store.RejectCharacter(r.Context(), detail.ID, feedback); err != nil {
		ah.logger.Error("failed to reject character", "error", err, "id", detail.ID)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var notifyErr string
	if ah.notifier != nil {
		if err := ah.notifier.NotifyRejection(r.Context(), detail.DiscordUserID, detail.CharacterName, feedback); err != nil {
			ah.logger.Error("failed to notify player of rejection", "error", err, "discord_user_id", detail.DiscordUserID)
			notifyErr = playerNotifyFailureMessage
		}
	}

	ah.broadcastUpdate("approval_updated", detail.ID)
	ah.writeStatusWithNotify(w, "rejected", notifyErr)
}

// playerNotifyFailureMessage is surfaced to the DM dashboard (in the JSON
// response body) when the player DM could not be delivered — e.g. the player
// has DMs closed. The status change is still persisted; this tells the DM to
// ask the player to check /character so the feedback isn't silently dropped.
const playerNotifyFailureMessage = "Player could not be notified (DM closed) — feedback saved; ask them to run /character."

// writeStatusWithNotify writes the standard {"status": ...} 200 response,
// adding a "notify_error" field only when the player DM failed so the
// dashboard can warn the DM without changing the HTTP status (the status
// transition has already been persisted).
func (ah *ApprovalHandler) writeStatusWithNotify(w http.ResponseWriter, status, notifyErr string) {
	body := map[string]string{"status": status}
	if notifyErr != "" {
		body["notify_error"] = notifyErr
	}
	ah.writeJSON(w, http.StatusOK, body)
}

func (ah *ApprovalHandler) broadcastUpdate(eventType string, id uuid.UUID) {
	if ah.hub == nil {
		return
	}
	msg, _ := json.Marshal(map[string]string{
		"type": eventType,
		"id":   id.String(),
	})
	ah.hub.Broadcast <- msg
}
