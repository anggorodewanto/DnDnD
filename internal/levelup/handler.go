package levelup

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dashboard"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// LevelUpRequest is the JSON body for the level-up API.
type LevelUpRequest struct {
	CharacterID uuid.UUID `json:"character_id"`
	ClassID     string    `json:"class_id"`
	NewLevel    int       `json:"new_level"`
}

// LevelUpResponse is the JSON response after a level-up.
type LevelUpResponse struct {
	NewLevel            int  `json:"new_level"`
	HPGained            int  `json:"hp_gained"`
	NewHPMax            int  `json:"new_hp_max"`
	NewProficiencyBonus int  `json:"new_proficiency_bonus"`
	NewAttacksPerAction int  `json:"new_attacks_per_action"`
	GrantsASI           bool `json:"grants_asi"`
	NeedsSubclass       bool `json:"needs_subclass"`
}

// ASIApprovalRequest is the JSON body for approving an ASI choice.
type ASIApprovalRequest struct {
	CharacterID uuid.UUID `json:"character_id"`
	Choice      ASIChoice `json:"choice"`
}

// ASIDenyRequest is the JSON body for denying an ASI choice.
type ASIDenyRequest struct {
	CharacterID uuid.UUID `json:"character_id"`
	Reason      string    `json:"reason"`
}

// FeatApplyRequest is the JSON body for applying a feat.
type FeatApplyRequest struct {
	CharacterID uuid.UUID `json:"character_id"`
	Feat        FeatInfo  `json:"feat"`
}

// FeatPrereqCheckRequest is the JSON body for checking feat prerequisites.
type FeatPrereqCheckRequest struct {
	Prerequisites      FeatPrerequisites       `json:"prerequisites"`
	Scores             character.AbilityScores `json:"scores"`
	ArmorProficiencies []string                `json:"armor_proficiencies"`
	IsSpellcaster      bool                    `json:"is_spellcaster"`
}

// FeatPrereqCheckResponse is the JSON response for a feat prerequisite check.
type FeatPrereqCheckResponse struct {
	Eligible bool   `json:"eligible"`
	Reason   string `json:"reason,omitempty"`
}

// Handler serves the level-up API endpoints and dashboard page.
type Handler struct {
	service *Service
	logger  *slog.Logger
	hub     *dashboard.Hub
	tmpl    *template.Template
}

// NewHandler creates a new level-up Handler.
func NewHandler(service *Service, hub *dashboard.Hub) *Handler {
	tmpl := template.Must(template.New("levelup").Parse(levelUpTemplate))
	return &Handler{
		service: service,
		logger:  slog.Default(),
		hub:     hub,
		tmpl:    tmpl,
	}
}

// RegisterRoutes mounts level-up API routes and the dashboard page on the given Chi router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/levelup", func(r chi.Router) {
		r.Post("/", h.HandleLevelUp)
		r.Post("/asi/approve", h.HandleApproveASI)
		r.Post("/asi/deny", h.HandleDenyASI)
		r.Post("/feat/apply", h.HandleApplyFeat)
		r.Post("/feat/check", h.HandleCheckFeatPrereqs)
	})
	r.Get("/dashboard/levelup", h.ServeLevelUpPage)
}

// HandleLevelUp processes a level-up request from the dashboard.
func (h *Handler) HandleLevelUp(w http.ResponseWriter, r *http.Request) {
	var req LevelUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.CharacterID == uuid.Nil || req.ClassID == "" || req.NewLevel < 1 || req.NewLevel > 20 {
		http.Error(w, "character_id, class_id, and new_level (1-20) are required", http.StatusBadRequest)
		return
	}

	details, err := h.service.ApplyLevelUp(r.Context(), req.CharacterID, req.ClassID, req.NewLevel)
	if err != nil {
		h.logger.Error("level-up failed", "error", err)
		http.Error(w, "level-up failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, LevelUpResponse{
		NewLevel:            details.NewLevel,
		HPGained:            details.HPGained,
		NewHPMax:            details.NewHPMax,
		NewProficiencyBonus: details.NewProficiencyBonus,
		NewAttacksPerAction: details.NewAttacksPerAction,
		GrantsASI:           details.GrantsASI,
		NeedsSubclass:       details.NeedsSubclass,
	})
}

// HandleApproveASI processes an ASI approval from the DM.
func (h *Handler) HandleApproveASI(w http.ResponseWriter, r *http.Request) {
	var req ASIApprovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.ApproveASI(r.Context(), req.CharacterID, req.Choice); err != nil {
		h.logger.Error("ASI approval failed", "error", err)
		http.Error(w, "ASI approval failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

// HandleDenyASI processes an ASI denial from the DM.
func (h *Handler) HandleDenyASI(w http.ResponseWriter, r *http.Request) {
	var req ASIDenyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.DenyASI(r.Context(), req.CharacterID, req.Reason); err != nil {
		h.logger.Error("ASI denial failed", "error", err)
		http.Error(w, "ASI denial failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "denied"})
}

// HandleApplyFeat processes a feat application request.
func (h *Handler) HandleApplyFeat(w http.ResponseWriter, r *http.Request) {
	var req FeatApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.ApplyFeat(r.Context(), req.CharacterID, req.Feat); err != nil {
		h.logger.Error("feat application failed", "error", err)
		if errors.Is(err, ErrInvalidFeatChoices) {
			http.Error(w, "feat application failed: "+err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "feat application failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "applied"})
}

// HandleCheckFeatPrereqs checks whether a character meets feat prerequisites.
func (h *Handler) HandleCheckFeatPrereqs(w http.ResponseWriter, r *http.Request) {
	var req FeatPrereqCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	eligible, reason := CheckFeatPrerequisites(req.Prerequisites, req.Scores, req.ArmorProficiencies, req.IsSpellcaster)

	writeJSON(w, http.StatusOK, FeatPrereqCheckResponse{
		Eligible: eligible,
		Reason:   reason,
	})
}

// LevelUpPageData holds the template data for the level-up dashboard page.
type LevelUpPageData struct {
	Nav []dashboard.NavEntry
}

// ServeLevelUpPage renders the level-up dashboard page.
func (h *Handler) ServeLevelUpPage(w http.ResponseWriter, r *http.Request) {
	data := LevelUpPageData{
		Nav: dashboard.SidebarNav,
	}

	var buf bytes.Buffer
	if err := h.tmpl.Execute(&buf, data); err != nil {
		h.logger.Error("failed to render level-up template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(buf.Bytes())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// levelUpTemplate is the Go HTML template for the level-up dashboard page.
// NOTE: DDB-imported characters should re-import via Phase 90 instead of leveling here.
const levelUpTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DnDnD — Level Up</title>
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
        .form-group { margin-bottom: 1rem; }
        .form-group label { display: block; margin-bottom: 0.25rem; font-weight: 600; }
        .form-group input, .form-group select { width: 100%; padding: 0.5rem; border-radius: 4px; border: 1px solid #0f3460; background: #16213e; color: #e0e0e0; font-size: 1rem; }
        .btn { padding: 0.75rem 1.5rem; background: #e94560; color: white; border: none; border-radius: 6px; cursor: pointer; font-size: 1rem; }
        .btn:hover { background: #c73852; }
        .btn-secondary { background: #0f3460; }
        .btn-secondary:hover { background: #1a4a8a; }
        .result-card { background: #16213e; border-radius: 8px; padding: 1.5rem; border: 1px solid #0f3460; margin-top: 1rem; display: none; }
        .result-card h3 { color: #e94560; margin-bottom: 0.5rem; }
        .result-card .stat { margin: 0.25rem 0; }
        .result-card .stat .label { color: #888; }
        .asi-section, .feat-section { margin-top: 1.5rem; padding: 1rem; border: 1px solid #0f3460; border-radius: 8px; display: none; }
        .asi-section h3, .feat-section h3 { color: #e94560; margin-bottom: 0.5rem; }
        .radio-group { display: flex; gap: 1rem; margin: 0.5rem 0; }
        .radio-group label { cursor: pointer; padding: 0.5rem 1rem; border: 1px solid #0f3460; border-radius: 4px; }
        .radio-group input:checked + span { color: #e94560; font-weight: bold; }
        .alert { padding: 0.75rem 1rem; border-radius: 4px; margin-top: 1rem; }
        .alert-success { background: #1b4332; border: 1px solid #2d6a4f; }
        .alert-warning { background: #533a1b; border: 1px solid #6a4f2d; }
        .alert-error { background: #4a1b1b; border: 1px solid #6a2d2d; }
    </style>
</head>
<body>
    <div class="sidebar">
        <h2>DnDnD Dashboard</h2>
        <nav>
            {{range .Nav}}<a href="{{.Path}}" class="nav-entry"><span class="icon">{{.Icon}}</span><span>{{.Label}}</span></a>
            {{end}}
            <a href="/dashboard/levelup" class="nav-entry active"><span class="icon">&#x2B06;</span><span>Level Up</span></a>
        </nav>
    </div>
    <div class="main">
        <h1>Level Up Character</h1>

        <div class="alert alert-warning" style="display:block; margin-bottom: 1rem;">
            <strong>Heads up:</strong> A Svelte equivalent of this widget now lives at
            <a href="/dashboard/app/#levelup" style="color:#e94560;">/dashboard/app/#levelup</a>.
            This page is kept as a fallback (F-16).
        </div>

        <div class="form-group">
            <label for="character-id">Character ID</label>
            <input type="text" id="character-id" placeholder="Enter character UUID">
        </div>
        <div class="form-group">
            <label for="class-id">Class</label>
            <input type="text" id="class-id" placeholder="e.g. fighter, wizard, cleric">
        </div>
        <div class="form-group">
            <label for="new-level">New Class Level</label>
            <input type="number" id="new-level" min="1" max="20" value="1">
        </div>
        <button class="btn" id="btn-levelup" onclick="doLevelUp()">Apply Level Up</button>

        <div class="result-card" id="result-card">
            <h3>Level Up Result</h3>
            <div class="stat"><span class="label">New Total Level:</span> <span id="res-level"></span></div>
            <div class="stat"><span class="label">HP Gained:</span> <span id="res-hp-gained"></span></div>
            <div class="stat"><span class="label">New HP Max:</span> <span id="res-hp-max"></span></div>
            <div class="stat"><span class="label">Proficiency Bonus:</span> +<span id="res-prof"></span></div>
            <div class="stat"><span class="label">Attacks per Action:</span> <span id="res-attacks"></span></div>
        </div>

        <div class="asi-section" id="asi-section">
            <h3>ASI / Feat Choice Pending</h3>
            <p>This level grants an Ability Score Improvement. The player will be prompted in Discord to choose:</p>
            <ul>
                <li>+2 to one ability score</li>
                <li>+1 to two different ability scores</li>
                <li>A feat (with prerequisite check)</li>
            </ul>
            <p>The choice will appear in the DM queue for approval.</p>
        </div>

        <div class="alert alert-warning" id="subclass-alert" style="display:none;">
            <strong>Subclass Selection Needed:</strong> This character needs to choose a subclass. Work with the player to select one.
        </div>

        <div id="status-msg"></div>
    </div>

    <script>
    function doLevelUp() {
        var charID = document.getElementById('character-id').value.trim();
        var classID = document.getElementById('class-id').value.trim();
        var newLevel = parseInt(document.getElementById('new-level').value, 10);

        if (!charID || !classID || !newLevel) {
            showStatus('Please fill in all fields.', 'error');
            return;
        }

        fetch('/api/levelup', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ character_id: charID, class_id: classID, new_level: newLevel })
        })
        .then(function(resp) {
            if (!resp.ok) return resp.text().then(function(t) { throw new Error(t); });
            return resp.json();
        })
        .then(function(data) {
            document.getElementById('res-level').textContent = data.new_level;
            document.getElementById('res-hp-gained').textContent = '+' + data.hp_gained;
            document.getElementById('res-hp-max').textContent = data.new_hp_max;
            document.getElementById('res-prof').textContent = data.new_proficiency_bonus;
            document.getElementById('res-attacks').textContent = data.new_attacks_per_action;
            document.getElementById('result-card').style.display = 'block';

            if (data.grants_asi) {
                document.getElementById('asi-section').style.display = 'block';
            } else {
                document.getElementById('asi-section').style.display = 'none';
            }

            if (data.needs_subclass) {
                document.getElementById('subclass-alert').style.display = 'block';
            } else {
                document.getElementById('subclass-alert').style.display = 'none';
            }

            showStatus('Level up applied successfully!', 'success');
        })
        .catch(function(err) {
            showStatus('Error: ' + err.message, 'error');
        });
    }

    function showStatus(msg, type) {
        var el = document.getElementById('status-msg');
        el.innerHTML = '<div class="alert alert-' + type + '">' + msg + '</div>';
    }
    </script>
</body>
</html>`
