package dashboard

import (
	"bytes"
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/errorlog"
)

// CampaignLookup returns the (id, status) of the campaign the DM is currently
// operating on. Phase 115 uses this to drive the Pause/Resume button state in
// the dashboard shell. Implementations typically key on the DM's discord user
// id resolved from the request context. Either return id=="" with no error
// when the DM has no active campaign, or propagate errors if the backing
// store is unreachable — the handler will silently degrade in that case.
type CampaignLookup interface {
	LookupActiveCampaign(ctx context.Context, dmUserID string) (id, status string, err error)
}

// NavEntry represents a sidebar navigation entry.
type NavEntry struct {
	Label string
	Icon  string
	Path  string
}

// SidebarNav is the list of navigation entries shown in the dashboard sidebar.
var SidebarNav = []NavEntry{
	{Label: "Campaign Home", Icon: "🏠", Path: "/dashboard"},
	{Label: "Character Approval", Icon: "📋", Path: "/dashboard/approvals"},
	{Label: "Encounter Builder", Icon: "⚔️", Path: "/dashboard/encounters"},
	{Label: "Stat Block Library", Icon: "📊", Path: "/dashboard/statblocks"},
	{Label: "Asset Library", Icon: "🖼️", Path: "/dashboard/assets"},
	{Label: "Map Editor", Icon: "🗺️", Path: "/dashboard/map"},
	{Label: "Exploration", Icon: "🧭", Path: "/dashboard/exploration"},
	{Label: "Character Overview", Icon: "👤", Path: "/dashboard/characters"},
	{Label: "Create Character", Icon: "➕", Path: "/dashboard/characters/new"},
	// Phase 112: error notification badge + panel. The label is rewritten
	// in-flight to "Errors (N)" by navWithErrorBadge when N > 0.
	{Label: "Errors", Icon: "⚠️", Path: "/dashboard/errors"},
}

// CampaignHomeData holds the data for the Campaign Home view.
type CampaignHomeData struct {
	Nav              []NavEntry
	DMQueueCount     int
	PendingApprovals int
	ActiveEncounters []string
	SavedEncounters  []string
	// Phase 115: drive the Pause/Resume button label + data-campaign-id.
	// CampaignID is empty when no CampaignLookup is wired or the DM has no
	// active campaign; in that case the button renders disabled-looking
	// with no endpoint attached.
	CampaignID     string
	CampaignStatus string
}

// PauseButtonLabel returns the correct label for the dashboard Pause/Resume
// toggle based on the current campaign status.
func (d CampaignHomeData) PauseButtonLabel() string {
	if d.CampaignStatus == "paused" {
		return "Resume Campaign"
	}
	return "Pause Campaign"
}

// Handler serves the DM dashboard pages.
type Handler struct {
	logger   *slog.Logger
	tmpl     *template.Template
	hub      *Hub
	errorLog *errorLogAdapter
	// Phase 115: optional campaign lookup so the Pause/Resume button can
	// render with the right label and endpoint target.
	campaignLookup CampaignLookup
}

// SetCampaignLookup wires a CampaignLookup so the Pause/Resume button can
// reflect current state. Pass nil to disable (button still renders but is
// inert). Safe to call at any time before the handler starts serving.
func (h *Handler) SetCampaignLookup(lookup CampaignLookup) {
	h.campaignLookup = lookup
}

// SetErrorReader wires the 24h error count into the Campaign Home sidebar
// badge. Pass nil reader to disable (default when Phase 112 isn't wired).
func (h *Handler) SetErrorReader(reader errorlog.Reader, clock func() time.Time) {
	if reader == nil {
		h.errorLog = nil
		return
	}
	if clock == nil {
		clock = time.Now
	}
	h.errorLog = &errorLogAdapter{reader: reader, clock: clock}
}

// NewHandler creates a new dashboard Handler with an optional Hub for WebSocket support.
func NewHandler(logger *slog.Logger, hub *Hub) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	tmpl := template.Must(template.New("dashboard").Parse(dashboardTemplate))
	return &Handler{
		logger: logger,
		tmpl:   tmpl,
		hub:    hub,
	}
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

// ServeDashboard serves the dashboard shell with Campaign Home as the default view.
func (h *Handler) ServeDashboard(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	errorCount := 0
	if h.errorLog != nil {
		errorCount = h.errorLog.count(r.Context())
	}

	campaignID, campaignStatus := h.lookupCampaign(r.Context(), userID)

	data := CampaignHomeData{
		Nav:              navWithErrorBadge(errorCount),
		DMQueueCount:     0,
		PendingApprovals: 0,
		ActiveEncounters: []string{},
		SavedEncounters:  []string{},
		CampaignID:       campaignID,
		CampaignStatus:   campaignStatus,
	}

	var buf bytes.Buffer
	if err := h.tmpl.Execute(&buf, data); err != nil {
		h.logger.Error("failed to render dashboard template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(buf.Bytes())
}

const dashboardTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DnDnD — DM Dashboard</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: system-ui, -apple-system, sans-serif; display: flex; min-height: 100vh; background: #1a1a2e; color: #e0e0e0; }
        .sidebar { width: 250px; background: #16213e; padding: 1rem 0; border-right: 1px solid #0f3460; }
        .sidebar h2 { padding: 0 1rem 1rem; color: #e94560; font-size: 1.2rem; }
        .sidebar nav a { display: flex; align-items: center; gap: 0.75rem; padding: 0.75rem 1rem; color: #e0e0e0; text-decoration: none; transition: background 0.2s; }
        .sidebar nav a:hover, .sidebar nav a.active { background: #0f3460; }
        .sidebar nav a .icon { font-size: 1.2rem; }
        .main { flex: 1; padding: 2rem; }
        .main h1 { color: #e94560; margin-bottom: 1.5rem; }
        .cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 1rem; margin-bottom: 2rem; }
        .card { background: #16213e; border-radius: 8px; padding: 1.5rem; border: 1px solid #0f3460; }
        .card h3 { color: #e94560; margin-bottom: 0.5rem; }
        .quick-actions { display: flex; gap: 1rem; flex-wrap: wrap; }
        .quick-actions button { padding: 0.75rem 1.5rem; background: #e94560; color: white; border: none; border-radius: 6px; cursor: pointer; font-size: 1rem; }
        .quick-actions button:hover { background: #c73852; }
    </style>
</head>
<body>
    <div class="sidebar">
        <h2>DnDnD Dashboard</h2>
        <nav>
            {{range .Nav}}<a href="{{.Path}}" class="nav-entry"><span class="icon">{{.Icon}}</span><span>{{.Label}}</span></a>
            {{end}}
        </nav>
    </div>
    <div class="main">
        <h1>Campaign Home</h1>
        <div class="cards">
            <div class="card" id="dm-queue">
                <h3>Pending dm-queue Items</h3>
                <p class="count">{{.DMQueueCount}}</p>
            </div>
            <div class="card" id="pending-approvals">
                <h3>Pending Character Approvals</h3>
                <p class="count">{{.PendingApprovals}}</p>
            </div>
            <div class="card" id="active-encounters">
                <h3>Active Encounters</h3>
                {{if .ActiveEncounters}}
                <ul>{{range .ActiveEncounters}}<li>{{.}}</li>{{end}}</ul>
                {{else}}
                <p>No active encounters</p>
                {{end}}
            </div>
            <div class="card" id="saved-encounters">
                <h3>Saved Encounters</h3>
                {{if .SavedEncounters}}
                <ul>{{range .SavedEncounters}}<li>{{.}}</li>{{end}}</ul>
                {{else}}
                <p>No saved encounters</p>
                {{end}}
            </div>
        </div>
        <div class="quick-actions">
            <button id="btn-new-encounter">New Encounter</button>
            <button id="btn-narrate">Narrate</button>
            <button id="btn-pause" data-campaign-id="{{.CampaignID}}" data-campaign-status="{{.CampaignStatus}}">{{.PauseButtonLabel}}</button>
        </div>
    </div>
    <script>
(function() {
    var wsURL = (location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '/dashboard/ws';
    var backoff = 1000;
    var maxBackoff = 30000;
    var ws;

    function connect() {
        ws = new WebSocket(wsURL);
        ws.onopen = function() {
            console.log('WebSocket connected');
            backoff = 1000;
        };
        ws.onmessage = function(event) {
            var data = JSON.parse(event.data);
            window.dispatchEvent(new CustomEvent('ws-message', { detail: data }));
        };
        ws.onclose = function() {
            console.log('WebSocket closed, reconnecting in ' + backoff + 'ms');
            setTimeout(function() {
                backoff = Math.min(backoff * 2, maxBackoff);
                connect();
            }, backoff);
        };
        ws.onerror = function(err) {
            console.error('WebSocket error', err);
            ws.close();
        };
    }

    connect();
})();

// Phase 115: Pause/Resume button toggle. Calls /api/campaigns/{id}/pause or
// /resume based on current data-campaign-status, then flips the label and
// attribute in-place so the next click does the opposite.
(function() {
    var btn = document.getElementById('btn-pause');
    if (!btn) return;
    btn.addEventListener('click', function() {
        var id = btn.getAttribute('data-campaign-id');
        if (!id) return;
        var status = btn.getAttribute('data-campaign-status') || 'active';
        var action = status === 'paused' ? 'resume' : 'pause';
        var nextStatus = action === 'pause' ? 'paused' : 'active';
        btn.disabled = true;
        fetch('/api/campaigns/' + id + '/' + action, { method: 'POST' })
            .then(function(resp) {
                if (!resp.ok) throw new Error('Request failed: ' + resp.status);
                btn.setAttribute('data-campaign-status', nextStatus);
                btn.textContent = nextStatus === 'paused' ? 'Resume Campaign' : 'Pause Campaign';
            })
            .catch(function(err) { console.error('Pause/Resume failed', err); })
            .finally(function() { btn.disabled = false; });
    });
})();
    </script>
</body>
</html>`
