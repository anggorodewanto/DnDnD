package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"html/template"
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

// DMQueueHandler serves the per-item DM queue resolver page.
// Phase 106a minimum viable: shows the item and a form to mark it resolved.
// F-12 extends this with a campaign-scoped JSON list endpoint that the
// Svelte dashboard panel consumes.
type DMQueueHandler struct {
	logger         *slog.Logger
	notifier       dmqueue.Notifier
	tmpl           *template.Template
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
		tmpl:     template.Must(template.New("dmqueue_item").Parse(dmqueueItemTemplate)),
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

type dmqueueItemView struct {
	Item                  dmqueue.Item
	KindLabel             string
	IsPending             bool
	IsResolved            bool
	IsCancelled           bool
	IsWhisper             bool
	IsSkillCheckNarration bool
}

// ServeItem renders GET /dashboard/queue/{itemID}.
func (h *DMQueueHandler) ServeItem(w http.ResponseWriter, r *http.Request) {
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

	view := dmqueueItemView{
		Item:                  item,
		KindLabel:             kindLabelFor(item.Event.Kind),
		IsPending:             item.Status == dmqueue.StatusPending,
		IsResolved:            item.Status == dmqueue.StatusResolved,
		IsCancelled:           item.Status == dmqueue.StatusCancelled,
		IsWhisper:             item.Event.Kind == dmqueue.KindPlayerWhisper,
		IsSkillCheckNarration: item.Event.Kind == dmqueue.KindSkillCheckNarration,
	}

	var buf bytes.Buffer
	if err := h.tmpl.Execute(&buf, view); err != nil {
		h.logger.Error("dmqueue item template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// dmqueueListEntry is a JSON-friendly projection of dmqueue.Item used by
// the F-12 list endpoint. Trimmed down to the fields the dashboard panel
// needs so the wire shape can evolve without leaking internal struct
// changes.
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

// HandleResolve processes POST /dashboard/queue/{itemID}/resolve with an "outcome" form field.
func (h *DMQueueHandler) HandleResolve(w http.ResponseWriter, r *http.Request) {
	itemID, ok := h.parseFormPost(w, r)
	if !ok {
		return
	}
	err := h.notifier.Resolve(r.Context(), itemID, r.FormValue("outcome"))
	h.respondAfterResolve(w, r, itemID, err, "dmqueue resolve", "resolve failed", nil)
}

// HandleSkillCheckNarration processes POST /dashboard/queue/{itemID}/narrate
// for KindSkillCheckNarration items. The "narration" form field is delivered
// to the originating Discord channel as a non-ephemeral follow-up via the
// notifier's wired SkillCheckNarrationDeliverer, and the queue item is then
// marked resolved with the narration text as its outcome.
func (h *DMQueueHandler) HandleSkillCheckNarration(w http.ResponseWriter, r *http.Request) {
	itemID, ok := h.parseFormPost(w, r)
	if !ok {
		return
	}
	err := h.notifier.ResolveSkillCheckNarration(r.Context(), itemID, r.FormValue("narration"))
	h.respondAfterResolve(w, r, itemID, err, "dmqueue skill check narration", "narrate failed", map[error]string{
		dmqueue.ErrNotSkillCheckNarrationItem: "not a skill check narration item",
	})
}

// HandleWhisperReply processes POST /dashboard/queue/{itemID}/reply for
// KindPlayerWhisper items. The "reply" form field is delivered to the
// whispering player as a Discord DM via the notifier's wired
// WhisperReplyDeliverer and the queue item is marked resolved with the
// reply text as its outcome.
func (h *DMQueueHandler) HandleWhisperReply(w http.ResponseWriter, r *http.Request) {
	itemID, ok := h.parseFormPost(w, r)
	if !ok {
		return
	}
	err := h.notifier.ResolveWhisper(r.Context(), itemID, r.FormValue("reply"))
	h.respondAfterResolve(w, r, itemID, err, "dmqueue whisper reply", "reply failed", map[error]string{
		dmqueue.ErrNotWhisperItem: "not a whisper item",
	})
}

// parseFormPost performs the auth check and form parsing common to every
// resolver POST handler. On failure it writes the appropriate response and
// returns ok=false. On success it returns the {itemID} URL parameter.
func (h *DMQueueHandler) parseFormPost(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !hasAuthUser(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", false
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return "", false
	}
	return chi.URLParam(r, "itemID"), true
}

// respondAfterResolve maps a notifier-resolve error to the appropriate HTTP
// response, redirecting to the item page on success. badRequestErrs maps
// kind-mismatch sentinel errors to their 400 Bad Request body.
func (h *DMQueueHandler) respondAfterResolve(w http.ResponseWriter, r *http.Request, itemID string, err error, logTag, failMsg string, badRequestErrs map[error]string) {
	if err == nil {
		http.Redirect(w, r, "/dashboard/queue/"+itemID, http.StatusSeeOther)
		return
	}
	if errors.Is(err, dmqueue.ErrItemNotFound) {
		http.NotFound(w, r)
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

const dmqueueItemTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>DnDnD — DM Queue Item</title>
<style>
body { font-family: system-ui, sans-serif; background: #1a1a2e; color: #e0e0e0; padding: 2rem; }
.item { max-width: 640px; margin: 0 auto; background: #16213e; padding: 1.5rem; border-radius: 8px; border: 1px solid #0f3460; }
h1 { color: #e94560; margin-bottom: 0.5rem; }
.meta { color: #a0a0c0; margin-bottom: 1rem; }
.summary { font-size: 1.1rem; margin-bottom: 1rem; }
.status { display: inline-block; padding: 0.25rem 0.75rem; border-radius: 4px; font-weight: bold; }
.status.pending { background: #e94560; color: white; }
.status.resolved { background: #2d6a4f; color: white; }
.status.cancelled { background: #555; color: white; }
form { margin-top: 1rem; }
input[type=text] { width: 100%; padding: 0.5rem; background: #0f3460; color: #e0e0e0; border: 1px solid #e94560; border-radius: 4px; }
button { margin-top: 0.5rem; padding: 0.5rem 1rem; background: #e94560; color: white; border: none; border-radius: 4px; cursor: pointer; }
.outcome { margin-top: 1rem; font-style: italic; }
a.back { display: inline-block; margin-top: 1rem; color: #a0a0c0; }
</style>
</head>
<body>
<div class="item">
<h1>{{.KindLabel}}</h1>
<div class="meta">{{.Item.Event.PlayerName}}{{if .IsPending}} — <span class="status pending">Pending</span>{{end}}{{if .IsResolved}} — <span class="status resolved">Resolved</span>{{end}}{{if .IsCancelled}} — <span class="status cancelled">Cancelled</span>{{end}}</div>
<div class="summary">{{.Item.Event.Summary}}</div>
{{if .IsPending}}
{{if .IsWhisper}}
<form method="post" action="/dashboard/queue/{{.Item.ID}}/reply">
  <label for="reply">Reply (sent as Discord DM)</label>
  <input type="text" id="reply" name="reply" placeholder="e.g. You catch the merchant's gaze mid-pull…" required>
  <button type="submit">Send Reply</button>
</form>
{{else if .IsSkillCheckNarration}}
<form method="post" action="/dashboard/queue/{{.Item.ID}}/narrate">
  <label for="narration">Narration (posted to channel)</label>
  <input type="text" id="narration" name="narration" placeholder="e.g. You spot the trap before stepping on it." required>
  <button type="submit">Send Narration</button>
</form>
{{else}}
<form method="post" action="/dashboard/queue/{{.Item.ID}}/resolve">
  <label for="outcome">Outcome summary</label>
  <input type="text" id="outcome" name="outcome" placeholder="e.g. table is flipped, enemies prone" required>
  <button type="submit">Resolve</button>
</form>
{{end}}
{{end}}
{{if .IsResolved}}<div class="outcome">Outcome: {{.Item.Outcome}}</div>{{end}}
<a class="back" href="/dashboard">&larr; Back to Campaign Home</a>
</div>
</body>
</html>`
