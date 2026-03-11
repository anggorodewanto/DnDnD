package dashboard

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/ab/dndnd/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ApprovalHandler serves the character approval queue API endpoints and page.
type ApprovalHandler struct {
	logger       *slog.Logger
	store        ApprovalStore
	notifier     PlayerNotifier
	hub          *Hub
	campaignID   uuid.UUID
	approvalTmpl *template.Template
}

// NewApprovalHandler creates a new ApprovalHandler.
func NewApprovalHandler(logger *slog.Logger, store ApprovalStore, notifier PlayerNotifier, hub *Hub, campaignID uuid.UUID) *ApprovalHandler {
	if logger == nil {
		logger = slog.Default()
	}
	tmpl := template.Must(template.New("approvals").Parse(approvalPageTemplate))
	return &ApprovalHandler{
		logger:       logger,
		store:        store,
		notifier:     notifier,
		hub:          hub,
		campaignID:   campaignID,
		approvalTmpl: tmpl,
	}
}

// RegisterApprovalRoutes mounts approval page and API routes on the given router.
func (ah *ApprovalHandler) RegisterApprovalRoutes(r chi.Router) {
	r.Get("/dashboard/approvals", ah.ServeApprovalPage)
	r.Route("/dashboard/api/approvals", func(r chi.Router) {
		r.Get("/", ah.ListApprovals)
		r.Get("/{id}", ah.GetApproval)
		r.Post("/{id}/approve", ah.Approve)
		r.Post("/{id}/request-changes", ah.RequestChangesHandler)
		r.Post("/{id}/reject", ah.Reject)
	})
}

// approvalPageData holds data for the approval queue page template.
type approvalPageData struct {
	Nav []NavEntry
}

// ServeApprovalPage renders the Character Approval Queue HTML page.
func (ah *ApprovalHandler) ServeApprovalPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := ah.requireAuth(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	data := approvalPageData{
		Nav: SidebarNav,
	}

	var buf bytes.Buffer
	if err := ah.approvalTmpl.Execute(&buf, data); err != nil {
		ah.logger.Error("failed to render approval template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(buf.Bytes())
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

func (ah *ApprovalHandler) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ListApprovals returns all pending approvals as JSON.
func (ah *ApprovalHandler) ListApprovals(w http.ResponseWriter, r *http.Request) {
	if _, ok := ah.requireAuth(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	entries, err := ah.store.ListPendingApprovals(r.Context(), ah.campaignID)
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

// Approve approves a pending character.
func (ah *ApprovalHandler) Approve(w http.ResponseWriter, r *http.Request) {
	if _, ok := ah.requireAuth(r); !ok {
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

	if err := ah.store.ApproveCharacter(r.Context(), id); err != nil {
		ah.logger.Error("failed to approve character", "error", err, "id", id)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Notify player
	if ah.notifier != nil {
		_ = ah.notifier.NotifyApproval(r.Context(), detail.DiscordUserID, detail.CharacterName)
	}

	// Broadcast update via WebSocket
	ah.broadcastUpdate("approval_updated", id)

	ah.writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

// RequestChangesHandler requests changes on a pending character.
func (ah *ApprovalHandler) RequestChangesHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := ah.requireAuth(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, ok := ah.parseID(w, r)
	if !ok {
		return
	}

	var req feedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Feedback == "" {
		http.Error(w, "feedback is required", http.StatusBadRequest)
		return
	}

	// Get detail for notification
	detail, err := ah.store.GetApprovalDetail(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if err := ah.store.RequestChanges(r.Context(), id, req.Feedback); err != nil {
		ah.logger.Error("failed to request changes", "error", err, "id", id)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Notify player
	if ah.notifier != nil {
		_ = ah.notifier.NotifyChangesRequested(r.Context(), detail.DiscordUserID, detail.CharacterName, req.Feedback)
	}

	// Broadcast update
	ah.broadcastUpdate("approval_updated", id)

	ah.writeJSON(w, http.StatusOK, map[string]string{"status": "changes_requested"})
}

// Reject rejects a pending character.
func (ah *ApprovalHandler) Reject(w http.ResponseWriter, r *http.Request) {
	if _, ok := ah.requireAuth(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, ok := ah.parseID(w, r)
	if !ok {
		return
	}

	var req feedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Feedback == "" {
		http.Error(w, "feedback is required", http.StatusBadRequest)
		return
	}

	// Get detail for notification
	detail, err := ah.store.GetApprovalDetail(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if err := ah.store.RejectCharacter(r.Context(), id, req.Feedback); err != nil {
		ah.logger.Error("failed to reject character", "error", err, "id", id)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Notify player
	if ah.notifier != nil {
		_ = ah.notifier.NotifyRejection(r.Context(), detail.DiscordUserID, detail.CharacterName, req.Feedback)
	}

	// Broadcast update
	ah.broadcastUpdate("approval_updated", id)

	ah.writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
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

const approvalPageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DnDnD — Character Approval Queue</title>
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
        .approval-list { display: flex; flex-direction: column; gap: 1rem; }
        .approval-card { background: #16213e; border-radius: 8px; padding: 1.5rem; border: 1px solid #0f3460; cursor: pointer; transition: border-color 0.2s; }
        .approval-card:hover { border-color: #e94560; }
        .approval-card h3 { color: #e94560; margin-bottom: 0.5rem; }
        .approval-card .meta { color: #999; font-size: 0.9rem; }
        .detail-panel { background: #16213e; border-radius: 8px; padding: 2rem; border: 1px solid #0f3460; display: none; }
        .detail-panel.visible { display: block; }
        .detail-panel h2 { color: #e94560; margin-bottom: 1rem; }
        .stat-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 0.75rem; margin: 1rem 0; }
        .stat-item { background: #1a1a2e; padding: 0.75rem; border-radius: 6px; text-align: center; }
        .stat-item .label { color: #999; font-size: 0.8rem; text-transform: uppercase; }
        .stat-item .value { font-size: 1.2rem; font-weight: bold; color: #e94560; }
        .actions { display: flex; gap: 1rem; margin-top: 1.5rem; flex-wrap: wrap; }
        .btn { padding: 0.75rem 1.5rem; border: none; border-radius: 6px; cursor: pointer; font-size: 1rem; color: white; }
        .btn-approve { background: #27ae60; }
        .btn-approve:hover { background: #219a52; }
        .btn-changes { background: #f39c12; }
        .btn-changes:hover { background: #d68910; }
        .btn-reject { background: #e74c3c; }
        .btn-reject:hover { background: #c0392b; }
        .btn-back { background: #0f3460; }
        .btn-back:hover { background: #0a2540; }
        .feedback-input { width: 100%; padding: 0.75rem; border-radius: 6px; border: 1px solid #0f3460; background: #1a1a2e; color: #e0e0e0; font-size: 1rem; margin-top: 0.5rem; resize: vertical; min-height: 80px; }
        .empty-state { text-align: center; color: #999; padding: 3rem; }
        .status-badge { display: inline-block; padding: 0.25rem 0.75rem; border-radius: 12px; font-size: 0.8rem; font-weight: bold; text-transform: uppercase; }
        .status-pending { background: #f39c12; color: #000; }
        .status-approved { background: #27ae60; color: #fff; }
        .status-rejected { background: #e74c3c; color: #fff; }
        .status-changes_requested { background: #3498db; color: #fff; }
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
        <h1>Character Approval Queue</h1>
        <div id="approval-list" class="approval-list">
            <div class="empty-state">Loading...</div>
        </div>
        <div id="detail-panel" class="detail-panel">
            <button class="btn btn-back" onclick="hideDetail()">Back to List</button>
            <h2 id="detail-name"></h2>
            <p id="detail-meta"></p>
            <div class="stat-grid" id="detail-stats"></div>
            <div id="detail-extra"></div>
            <div class="actions">
                <button class="btn btn-approve" onclick="approveCharacter()">Approve</button>
                <button class="btn btn-changes" onclick="showFeedback('changes')">Request Changes</button>
                <button class="btn btn-reject" onclick="showFeedback('reject')">Reject</button>
            </div>
            <div id="feedback-section" style="display:none; margin-top: 1rem;">
                <textarea id="feedback-input" class="feedback-input" placeholder="Enter feedback for the player..."></textarea>
                <div class="actions" style="margin-top: 0.5rem;">
                    <button id="feedback-submit" class="btn btn-changes" onclick="submitFeedback()">Submit</button>
                    <button class="btn btn-back" onclick="cancelFeedback()">Cancel</button>
                </div>
            </div>
        </div>
    </div>
    <script>
(function() {
    var currentId = null;
    var feedbackAction = null;
    var apiBase = '/dashboard/api/approvals';

    function loadList() {
        fetch(apiBase)
            .then(function(r) { return r.json(); })
            .then(function(entries) {
                var list = document.getElementById('approval-list');
                if (!entries || entries.length === 0) {
                    list.innerHTML = '<div class="empty-state">No pending approvals</div>';
                    return;
                }
                list.innerHTML = entries.map(function(e) {
                    return '<div class="approval-card" onclick="window.showDetail(\'' + e.id + '\')">' +
                        '<h3>' + escapeHtml(e.character_name) + '</h3>' +
                        '<p class="meta">Player: ' + escapeHtml(e.discord_user_id) +
                        ' | Via: ' + escapeHtml(e.created_via) +
                        ' | <span class="status-badge status-' + e.status + '">' + e.status + '</span></p>' +
                        '</div>';
                }).join('');
            });
    }

    function escapeHtml(str) {
        var div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }

    window.showDetail = function(id) {
        currentId = id;
        fetch(apiBase + '/' + id)
            .then(function(r) { return r.json(); })
            .then(function(d) {
                document.getElementById('detail-name').textContent = d.character_name;
                document.getElementById('detail-meta').innerHTML =
                    'Player: ' + escapeHtml(d.discord_user_id) +
                    ' | Via: ' + escapeHtml(d.created_via) +
                    ' | Race: ' + escapeHtml(d.race) +
                    ' | Level: ' + d.level;
                document.getElementById('detail-stats').innerHTML =
                    stat('HP', d.hp_current + '/' + d.hp_max) +
                    stat('AC', d.ac) +
                    stat('Speed', d.speed_ft + ' ft');
                var extra = '';
                if (d.classes) extra += '<p><strong>Classes:</strong> ' + escapeHtml(d.classes) + '</p>';
                if (d.ability_scores) extra += '<p><strong>Abilities:</strong> ' + escapeHtml(d.ability_scores) + '</p>';
                if (d.languages) extra += '<p><strong>Languages:</strong> ' + escapeHtml(d.languages) + '</p>';
                if (d.ddb_url) extra += '<p><a href="' + escapeHtml(d.ddb_url) + '" target="_blank">D&amp;D Beyond Sheet</a></p>';
                document.getElementById('detail-extra').innerHTML = extra;
                document.getElementById('approval-list').style.display = 'none';
                document.getElementById('detail-panel').classList.add('visible');
                cancelFeedback();
            });
    };

    function stat(label, value) {
        return '<div class="stat-item"><div class="label">' + label + '</div><div class="value">' + value + '</div></div>';
    }

    window.hideDetail = function() {
        document.getElementById('detail-panel').classList.remove('visible');
        document.getElementById('approval-list').style.display = '';
        currentId = null;
        loadList();
    };

    window.approveCharacter = function() {
        if (!currentId) return;
        fetch(apiBase + '/' + currentId + '/approve', { method: 'POST' })
            .then(function() { hideDetail(); });
    };

    window.showFeedback = function(action) {
        feedbackAction = action;
        document.getElementById('feedback-section').style.display = '';
        document.getElementById('feedback-input').value = '';
        var btn = document.getElementById('feedback-submit');
        if (action === 'reject') {
            btn.className = 'btn btn-reject';
            btn.textContent = 'Reject';
        } else {
            btn.className = 'btn btn-changes';
            btn.textContent = 'Request Changes';
        }
    };

    window.cancelFeedback = function() {
        document.getElementById('feedback-section').style.display = 'none';
        feedbackAction = null;
    };

    window.submitFeedback = function() {
        if (!currentId || !feedbackAction) return;
        var feedback = document.getElementById('feedback-input').value.trim();
        if (!feedback) { alert('Feedback is required'); return; }
        var endpoint = feedbackAction === 'reject' ? '/reject' : '/request-changes';
        fetch(apiBase + '/' + currentId + endpoint, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ feedback: feedback })
        }).then(function() { hideDetail(); });
    };

    // WebSocket live updates
    window.addEventListener('ws-message', function(e) {
        if (e.detail && e.detail.type === 'approval_updated') {
            if (!currentId) loadList();
        }
    });

    loadList();
})();
    </script>
</body>
</html>`
