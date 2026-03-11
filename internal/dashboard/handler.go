package dashboard

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/ab/dndnd/internal/auth"
)

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
	{Label: "Character Overview", Icon: "👤", Path: "/dashboard/characters"},
}

// CampaignHomeData holds the data for the Campaign Home view.
type CampaignHomeData struct {
	Nav                []NavEntry
	DMQueueCount       int
	PendingApprovals   int
	ActiveEncounters   []string
	SavedEncounters    []string
}

// Handler serves the DM dashboard pages.
type Handler struct {
	logger *slog.Logger
	tmpl   *template.Template
	hub    *Hub
}

// NewHandler creates a new dashboard Handler.
func NewHandler(logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	tmpl := template.Must(template.New("dashboard").Parse(dashboardTemplate))
	return &Handler{
		logger: logger,
		tmpl:   tmpl,
	}
}

// ServeDashboard serves the dashboard shell with Campaign Home as the default view.
func (h *Handler) ServeDashboard(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	data := CampaignHomeData{
		Nav:              SidebarNav,
		DMQueueCount:     0,
		PendingApprovals: 0,
		ActiveEncounters: []string{},
		SavedEncounters:  []string{},
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
            <button id="btn-pause">Pause Campaign</button>
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
    </script>
</body>
</html>`
