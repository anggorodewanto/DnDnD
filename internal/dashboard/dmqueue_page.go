package dashboard

import (
	"bytes"
	"errors"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/dmqueue"
)

// DMQueueHandler serves the per-item DM queue resolver page.
// Phase 106a minimum viable: shows the item and a form to mark it resolved.
// Future phases will expand this into a dedicated resolver panel.
type DMQueueHandler struct {
	logger   *slog.Logger
	notifier dmqueue.Notifier
	tmpl     *template.Template
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

type dmqueueItemView struct {
	Item       dmqueue.Item
	KindLabel  string
	IsPending  bool
	IsResolved bool
	IsCancelled bool
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
		Item:        item,
		KindLabel:   kindLabelFor(item.Event.Kind),
		IsPending:   item.Status == dmqueue.StatusPending,
		IsResolved:  item.Status == dmqueue.StatusResolved,
		IsCancelled: item.Status == dmqueue.StatusCancelled,
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

// HandleResolve processes POST /dashboard/queue/{itemID}/resolve with an "outcome" form field.
func (h *DMQueueHandler) HandleResolve(w http.ResponseWriter, r *http.Request) {
	if !hasAuthUser(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	itemID := chi.URLParam(r, "itemID")
	outcome := r.FormValue("outcome")

	if err := h.notifier.Resolve(r.Context(), itemID, outcome); err != nil {
		if errors.Is(err, dmqueue.ErrItemNotFound) {
			http.NotFound(w, r)
			return
		}
		h.logger.Error("dmqueue resolve", "error", err, "item_id", itemID)
		http.Error(w, "resolve failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/dashboard/queue/"+itemID, http.StatusSeeOther)
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
<form method="post" action="/dashboard/queue/{{.Item.ID}}/resolve">
  <label for="outcome">Outcome summary</label>
  <input type="text" id="outcome" name="outcome" placeholder="e.g. table is flipped, enemies prone" required>
  <button type="submit">Resolve</button>
</form>
{{end}}
{{if .IsResolved}}<div class="outcome">Outcome: {{.Item.Outcome}}</div>{{end}}
<a class="back" href="/dashboard">&larr; Back to Campaign Home</a>
</div>
</body>
</html>`
