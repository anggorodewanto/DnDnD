package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/dmqueue"
)

// CampaignQueueLister returns pending dm-queue items scoped to a single
// campaign. The dashboard list view (F-12) uses this to aggregate every
// pending item — whisper, freeform, rest, etc. — into one screen instead
// of forcing the DM to browse Discord #dm-queue. Implementations typically
// wrap dmqueue.PgStore.ListPendingForCampaign; nil lister disables the
// endpoint at the wiring layer (no panel rendered).
type CampaignQueueLister interface {
	ListPendingForCampaign(ctx context.Context, campaignID uuid.UUID) ([]dmqueue.Item, error)
}

// DMQueueHandler serves the dm-queue list and per-item JSON endpoints
// consumed by the Svelte DM Queue panel.
type DMQueueHandler struct {
	logger         *slog.Logger
	notifier       dmqueue.Notifier
	lister         CampaignQueueLister
	campaignLookup CampaignLookup
}

// NewDMQueueHandler constructs a DMQueueHandler.
func NewDMQueueHandler(logger *slog.Logger, notifier dmqueue.Notifier) *DMQueueHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &DMQueueHandler{
		logger:   logger,
		notifier: notifier,
	}
}

// SetCampaignLister wires the campaign-scoped pending-items source used by
// the F-12 list endpoint. Pair with SetCampaignLookup so the handler can
// resolve the DM's active campaign per request. Either nil leaves the list
// endpoint returning an empty array (still 200) so the SPA renders cleanly
// in passthrough-auth dev mode.
func (h *DMQueueHandler) SetCampaignLister(lister CampaignQueueLister) {
	h.lister = lister
}

// SetCampaignLookup wires the per-request campaign resolver used by the
// F-12 list endpoint. Reuses the same CampaignLookup interface the
// dashboard.Handler uses.
func (h *DMQueueHandler) SetCampaignLookup(lookup CampaignLookup) {
	h.campaignLookup = lookup
}

// dmqueueItemDetail is the JSON projection of a single dm-queue item used
// by the Svelte detail view. The boolean discriminators tell the SPA which
// form to render without duplicating the kind-string switch on the client.
type dmqueueItemDetail struct {
	ID                    string `json:"id"`
	Kind                  string `json:"kind"`
	KindLabel             string `json:"kind_label"`
	PlayerName            string `json:"player_name"`
	Summary               string `json:"summary"`
	Status                string `json:"status"`
	Outcome               string `json:"outcome"`
	IsPending             bool   `json:"is_pending"`
	IsResolved            bool   `json:"is_resolved"`
	IsCancelled           bool   `json:"is_cancelled"`
	IsWhisper             bool   `json:"is_whisper"`
	IsSkillCheckNarration bool   `json:"is_skill_check_narration"`
}

// GetItemJSON renders GET /dashboard/queue/{itemID} as JSON for the Svelte
// resolver UI.
func (h *DMQueueHandler) GetItemJSON(w http.ResponseWriter, r *http.Request) {
	if !hasAuthUser(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	itemID := chi.URLParam(r, "itemID")
	item, ok := h.notifier.Get(itemID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	detail := dmqueueItemDetail{
		ID:                    item.ID,
		Kind:                  string(item.Event.Kind),
		KindLabel:             kindLabelFor(item.Event.Kind),
		PlayerName:            item.Event.PlayerName,
		Summary:               item.Event.Summary,
		Status:                string(item.Status),
		Outcome:               item.Outcome,
		IsPending:             item.Status == dmqueue.StatusPending,
		IsResolved:            item.Status == dmqueue.StatusResolved,
		IsCancelled:           item.Status == dmqueue.StatusCancelled,
		IsWhisper:             item.Event.Kind == dmqueue.KindPlayerWhisper,
		IsSkillCheckNarration: item.Event.Kind == dmqueue.KindSkillCheckNarration,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(detail); err != nil {
		h.logger.Error("dmqueue item encode", "error", err)
	}
}

// dmqueueListEntry is a JSON-friendly projection of dmqueue.Item used by
// the F-12 list endpoint. ResolvePath is consumed by the playtest-player
// REPL; the SPA panel navigates inline.
type dmqueueListEntry struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	KindLabel   string `json:"kind_label"`
	PlayerName  string `json:"player_name"`
	Summary     string `json:"summary"`
	Status      string `json:"status"`
	ResolvePath string `json:"resolve_path"`
}

// ServeList renders GET /dashboard/queue as a JSON array of pending items
// for the authenticated DM's active campaign. F-12: aggregates every
// pending dm-queue item type (whisper, freeform, rest, etc.) so the DM can
// browse them in one place instead of scrolling Discord #dm-queue.
//
// The route is mounted behind dmAuthMw (F-2) so non-DM authenticated users
// already receive a 403 before this handler runs. We still enforce the
// session-user check defensively in case the handler is mounted bare in
// tests.
func (h *DMQueueHandler) ServeList(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	entries := h.listForUser(r.Context(), userID)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(entries); err != nil {
		h.logger.Error("dmqueue list encode", "error", err)
	}
}

// listForUser resolves the DM's active campaign and returns its pending
// queue items as JSON-friendly entries. Best-effort: any missing lookup,
// missing lister, unknown campaign, or backing-store error degrades to an
// empty slice so the SPA renders cleanly.
func (h *DMQueueHandler) listForUser(ctx context.Context, dmUserID string) []dmqueueListEntry {
	if h.lister == nil || h.campaignLookup == nil {
		return []dmqueueListEntry{}
	}
	idStr, _, err := h.campaignLookup.LookupActiveCampaign(ctx, dmUserID)
	if err != nil {
		h.logger.Warn("dmqueue list: campaign lookup failed", "error", err)
		return []dmqueueListEntry{}
	}
	if idStr == "" {
		return []dmqueueListEntry{}
	}
	campaignID, err := uuid.Parse(idStr)
	if err != nil {
		h.logger.Warn("dmqueue list: invalid campaign id", "id", idStr, "error", err)
		return []dmqueueListEntry{}
	}
	items, err := h.lister.ListPendingForCampaign(ctx, campaignID)
	if err != nil {
		h.logger.Error("dmqueue list: lister failed", "error", err)
		return []dmqueueListEntry{}
	}
	out := make([]dmqueueListEntry, 0, len(items))
	for _, it := range items {
		out = append(out, dmqueueListEntry{
			ID:          it.ID,
			Kind:        string(it.Event.Kind),
			KindLabel:   kindLabelFor(it.Event.Kind),
			PlayerName:  it.Event.PlayerName,
			Summary:     it.Event.Summary,
			Status:      string(it.Status),
			ResolvePath: it.Event.ResolvePath,
		})
	}
	return out
}

// resolvePayload is the JSON body accepted by HandleResolve.
type resolvePayload struct {
	Outcome string `json:"outcome"`
}

// HandleResolve processes POST /dashboard/queue/{itemID}/resolve with a
// JSON `{ "outcome": string }` body.
func (h *DMQueueHandler) HandleResolve(w http.ResponseWriter, r *http.Request) {
	var payload resolvePayload
	itemID, ok := h.decodeJSONPost(w, r, &payload)
	if !ok {
		return
	}
	err := h.notifier.Resolve(r.Context(), itemID, payload.Outcome)
	h.respondAfterResolve(w, itemID, err, "dmqueue resolve", "resolve failed", nil)
}

// whisperReplyPayload is the JSON body accepted by HandleWhisperReply.
type whisperReplyPayload struct {
	Reply string `json:"reply"`
}

// HandleWhisperReply processes POST /dashboard/queue/{itemID}/reply for
// KindPlayerWhisper items. The JSON `reply` field is delivered to the
// whispering player as a Discord DM via the notifier's wired
// WhisperReplyDeliverer and the queue item is marked resolved with the
// reply text as its outcome.
func (h *DMQueueHandler) HandleWhisperReply(w http.ResponseWriter, r *http.Request) {
	var payload whisperReplyPayload
	itemID, ok := h.decodeJSONPost(w, r, &payload)
	if !ok {
		return
	}
	err := h.notifier.ResolveWhisper(r.Context(), itemID, payload.Reply)
	h.respondAfterResolve(w, itemID, err, "dmqueue whisper reply", "reply failed", map[error]string{
		dmqueue.ErrNotWhisperItem: "not a whisper item",
	})
}

// skillCheckNarrationPayload is the JSON body accepted by HandleSkillCheckNarration.
type skillCheckNarrationPayload struct {
	Narration string `json:"narration"`
}

// HandleSkillCheckNarration processes POST /dashboard/queue/{itemID}/narrate
// for KindSkillCheckNarration items. The JSON `narration` field is delivered
// to the originating Discord channel as a non-ephemeral follow-up via the
// notifier's wired SkillCheckNarrationDeliverer, and the queue item is then
// marked resolved with the narration text as its outcome.
func (h *DMQueueHandler) HandleSkillCheckNarration(w http.ResponseWriter, r *http.Request) {
	var payload skillCheckNarrationPayload
	itemID, ok := h.decodeJSONPost(w, r, &payload)
	if !ok {
		return
	}
	err := h.notifier.ResolveSkillCheckNarration(r.Context(), itemID, payload.Narration)
	h.respondAfterResolve(w, itemID, err, "dmqueue skill check narration", "narrate failed", map[error]string{
		dmqueue.ErrNotSkillCheckNarrationItem: "not a skill check narration item",
	})
}

// decodeJSONPost performs the auth check and JSON body decode common to
// every resolver POST handler. On failure it writes the appropriate
// response and returns ok=false. On success it returns the {itemID} URL
// parameter and populates dst from the request body.
func (h *DMQueueHandler) decodeJSONPost(w http.ResponseWriter, r *http.Request, dst any) (string, bool) {
	if !hasAuthUser(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", false
	}
	if r.Body == nil {
		http.Error(w, "missing body", http.StatusBadRequest)
		return "", false
	}
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return "", false
	}
	return chi.URLParam(r, "itemID"), true
}

// respondAfterResolve maps a notifier-resolve error to the appropriate HTTP
// response. On success the handler returns 204 No Content; the Svelte
// panel re-fetches the item to display the updated state rather than
// chasing a redirect. badRequestErrs maps kind-mismatch sentinel errors to
// their 400 Bad Request body.
func (h *DMQueueHandler) respondAfterResolve(w http.ResponseWriter, itemID string, err error, logTag, failMsg string, badRequestErrs map[error]string) {
	if err == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if errors.Is(err, dmqueue.ErrItemNotFound) {
		http.Error(w, "item not found", http.StatusNotFound)
		return
	}
	for sentinel, msg := range badRequestErrs {
		if errors.Is(err, sentinel) {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
	}
	h.logger.Error(logTag, "error", err, "item_id", itemID)
	http.Error(w, failMsg, http.StatusInternalServerError)
}

// hasAuthUser reports whether the request carries an authenticated discord user ID.
func hasAuthUser(r *http.Request) bool {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	return ok && userID != ""
}

func kindLabelFor(k dmqueue.EventKind) string {
	switch k {
	case dmqueue.KindFreeformAction:
		return "Freeform Action"
	case dmqueue.KindReactionDeclaration:
		return "Reaction Declaration"
	case dmqueue.KindRestRequest:
		return "Rest Request"
	case dmqueue.KindSkillCheckNarration:
		return "Skill Check Narration"
	case dmqueue.KindConsumable:
		return "Consumable Usage"
	case dmqueue.KindEnemyTurnReady:
		return "Enemy Turn Ready"
	case dmqueue.KindNarrativeTeleport:
		return "Narrative Teleport"
	case dmqueue.KindPlayerWhisper:
		return "Player Whisper"
	default:
		return "Notification"
	}
}
