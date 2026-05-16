package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/portal"
	"github.com/go-chi/chi/v5"
)

// CharCreateServicer is the interface for character creation.
type CharCreateServicer interface {
	CreateCharacter(ctx context.Context, campaignID string, sub DMCharacterSubmission) (portal.CreateCharacterResult, error)
}

type abilityMethodLister interface {
	AllowedAbilityScoreMethods(ctx context.Context, campaignID string) ([]portal.AbilityScoreMethod, error)
}

// RefDataForCreate provides reference data for the character creation form.
type RefDataForCreate interface {
	ListRaces(ctx context.Context) ([]portal.RaceInfo, error)
	ListClasses(ctx context.Context) ([]portal.ClassInfo, error)
	ListEquipment(ctx context.Context) ([]portal.EquipmentItem, error)
	ListSpellsByClass(ctx context.Context, class string) ([]portal.SpellInfo, error)
}

// CharCreateHandler serves the DM character creation page and API.
type CharCreateHandler struct {
	logger          *slog.Logger
	svc             CharCreateServicer
	refData         RefDataForCreate
	tmpl            *template.Template
	featureProvider FeatureProvider
	dmVerifier      DMVerifier
}

// SetDMVerifier sets the verifier used to check campaign ownership.
func (h *CharCreateHandler) SetDMVerifier(v DMVerifier) {
	h.dmVerifier = v
}

// SetFeatureProvider sets the feature provider for preview/features endpoints.
func (h *CharCreateHandler) SetFeatureProvider(fp FeatureProvider) {
	h.featureProvider = fp
}

// NewCharCreateHandler creates a new CharCreateHandler.
func NewCharCreateHandler(logger *slog.Logger, svc CharCreateServicer, refData RefDataForCreate) *CharCreateHandler {
	if logger == nil {
		logger = slog.Default()
	}
	tmpl := template.Must(template.New("charcreate").Parse(charCreatePageTemplate))
	return &CharCreateHandler{
		logger:  logger,
		svc:     svc,
		refData: refData,
		tmpl:    tmpl,
	}
}

// RegisterCharCreateRoutes mounts character creation routes on the given router.
func (h *CharCreateHandler) RegisterCharCreateRoutes(r chi.Router) {
	r.Get("/dashboard/characters/new", h.ServeCreatePage)
	r.Route("/dashboard/api/characters", func(r chi.Router) {
		r.Post("/", h.HandleCreate)
		r.Post("/preview", h.HandlePreview)
		r.Get("/ref/races", h.HandleListRefRaces)
		r.Get("/ref/classes", h.HandleListRefClasses)
		r.Get("/ref/equipment", h.HandleListRefEquipment)
		r.Get("/ref/starting-equipment", h.HandleListRefStartingEquipment)
		r.Get("/ref/spells", h.HandleListRefSpells)
		r.Get("/ability-methods", h.HandleAbilityMethods)
	})
}

// charCreatePageData holds data for the character creation page template.
type charCreatePageData struct {
	Nav []NavEntry
}

// ServeCreatePage renders the character creation wizard HTML page.
func (h *CharCreateHandler) ServeCreatePage(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuthHelper(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	data := charCreatePageData{
		Nav: SidebarNav,
	}

	var buf bytes.Buffer
	if err := h.tmpl.Execute(&buf, data); err != nil {
		h.logger.Error("failed to render character creation template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(buf.Bytes())
}

// dmCreateRequest is the JSON body for DM character creation.
type dmCreateRequest struct {
	CampaignID string `json:"campaign_id"`
	DMCharacterSubmission
}

// HandleCreate creates a new DM character.
func (h *CharCreateHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireAuthHelper(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req dmCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if h.dmVerifier != nil && req.CampaignID != "" {
		owns, err := h.dmVerifier.IsCampaignDM(r.Context(), userID, req.CampaignID)
		if err != nil || !owns {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	result, err := h.svc.CreateCharacter(r.Context(), req.CampaignID, req.DMCharacterSubmission)
	if err != nil {
		if strings.HasPrefix(err.Error(), "validation") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.logger.Error("creating dm character", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	writeJSONResponse(w, http.StatusCreated, map[string]string{
		"character_id":        result.CharacterID,
		"player_character_id": result.PlayerCharacterID,
	})
}

// HandlePreview returns derived stats without saving.
func (h *CharCreateHandler) HandlePreview(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuthHelper(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var sub DMCharacterSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	stats := DeriveDMStats(sub)
	if h.featureProvider != nil {
		stats.Features = CollectFeatures(
			sub.Classes,
			h.featureProvider.ClassFeatures(),
			h.featureProvider.SubclassFeatures(),
			h.featureProvider.RacialTraits(sub.Race),
		)
	}
	writeJSONResponse(w, http.StatusOK, stats)
}

// HandleAbilityMethods returns campaign-enabled ability score methods.
func (h *CharCreateHandler) HandleAbilityMethods(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuthHelper(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	methods := portal.DefaultAbilityScoreMethods()
	if lister, ok := h.svc.(abilityMethodLister); ok {
		allowed, err := lister.AllowedAbilityScoreMethods(r.Context(), r.URL.Query().Get("campaign_id"))
		if err != nil {
			h.logger.Error("listing ability methods", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		methods = allowed
	}
	writeJSONResponse(w, http.StatusOK, methods)
}

// HandleListRefRaces returns available races as JSON.
func (h *CharCreateHandler) HandleListRefRaces(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuthHelper(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if h.refData == nil {
		writeJSONResponse(w, http.StatusOK, []portal.RaceInfo{})
		return
	}

	races, err := h.refData.ListRaces(r.Context())
	if err != nil {
		h.logger.Error("listing races for char creation", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if races == nil {
		races = []portal.RaceInfo{}
	}
	writeJSONResponse(w, http.StatusOK, races)
}

// HandleListRefClasses returns available classes as JSON.
func (h *CharCreateHandler) HandleListRefClasses(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuthHelper(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if h.refData == nil {
		writeJSONResponse(w, http.StatusOK, []portal.ClassInfo{})
		return
	}

	classes, err := h.refData.ListClasses(r.Context())
	if err != nil {
		h.logger.Error("listing classes for char creation", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if classes == nil {
		classes = []portal.ClassInfo{}
	}
	writeJSONResponse(w, http.StatusOK, classes)
}

// HandleListRefEquipment returns available equipment (weapons + armor) as JSON.
func (h *CharCreateHandler) HandleListRefEquipment(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuthHelper(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if h.refData == nil {
		writeJSONResponse(w, http.StatusOK, []portal.EquipmentItem{})
		return
	}

	equipment, err := h.refData.ListEquipment(r.Context())
	if err != nil {
		h.logger.Error("listing equipment for char creation", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if equipment == nil {
		equipment = []portal.EquipmentItem{}
	}
	writeJSONResponse(w, http.StatusOK, equipment)
}

// HandleListRefStartingEquipment returns starting equipment packs for a class.
func (h *CharCreateHandler) HandleListRefStartingEquipment(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuthHelper(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	class := r.URL.Query().Get("class")
	if class == "" {
		http.Error(w, "class parameter is required", http.StatusBadRequest)
		return
	}

	packs := portal.StartingEquipmentPacks(class)
	if packs == nil {
		packs = []portal.EquipmentPack{}
	}
	writeJSONResponse(w, http.StatusOK, packs)
}

// HandleListRefSpells returns spells filtered by class as JSON.
func (h *CharCreateHandler) HandleListRefSpells(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuthHelper(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	class := r.URL.Query().Get("class")
	if class == "" {
		http.Error(w, "class parameter is required", http.StatusBadRequest)
		return
	}

	if h.refData == nil {
		writeJSONResponse(w, http.StatusOK, []portal.SpellInfo{})
		return
	}

	spells, err := h.refData.ListSpellsByClass(r.Context(), class)
	if err != nil {
		h.logger.Error("listing spells for char creation", "error", err, "class", class)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if spells == nil {
		spells = []portal.SpellInfo{}
	}

	// Filter by max_level if provided
	if maxLevelStr := r.URL.Query().Get("max_level"); maxLevelStr != "" {
		maxLevel, err := strconv.Atoi(maxLevelStr)
		if err == nil {
			spells = filterSpellsByMaxLevel(spells, maxLevel)
		}
	}

	writeJSONResponse(w, http.StatusOK, spells)
}

// filterSpellsByMaxLevel returns only spells at or below the given spell level.
// Cantrips (level 0) are always included.
func filterSpellsByMaxLevel(spells []portal.SpellInfo, maxLevel int) []portal.SpellInfo {
	filtered := make([]portal.SpellInfo, 0, len(spells))
	for _, sp := range spells {
		if sp.Level <= maxLevel {
			filtered = append(filtered, sp)
		}
	}
	return filtered
}

// requireAuthHelper extracts the discord user ID from the request context.
func requireAuthHelper(r *http.Request) (string, bool) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		return "", false
	}
	return userID, true
}

// writeJSONResponse writes a JSON response with the given status code.
func writeJSONResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

const charCreatePageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DnDnD — Create Character</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: system-ui, -apple-system, sans-serif; display: flex; min-height: 100vh; background: #1a1a2e; color: #e0e0e0; }
        .sidebar { width: 250px; background: #16213e; padding: 1rem 0; border-right: 1px solid #0f3460; }
        .sidebar h2 { padding: 0 1rem 1rem; color: #e94560; font-size: 1.2rem; }
        .sidebar nav a { display: flex; align-items: center; gap: 0.75rem; padding: 0.75rem 1rem; color: #e0e0e0; text-decoration: none; transition: background 0.2s; }
        .sidebar nav a:hover, .sidebar nav a.active { background: #0f3460; }
        .sidebar nav a .icon { font-size: 1.2rem; }
        .main { flex: 1; padding: 2rem; max-width: 900px; }
        .main h1 { color: #e94560; margin-bottom: 1.5rem; }
        .wizard-steps { display: flex; gap: 0.5rem; margin-bottom: 2rem; flex-wrap: wrap; }
        .wizard-steps .step { padding: 0.5rem 1rem; background: #16213e; border-radius: 6px; border: 1px solid #0f3460; color: #999; font-size: 0.9rem; }
        .wizard-steps .step.active { border-color: #e94560; color: #e94560; font-weight: bold; }
        .wizard-steps .step.done { border-color: #27ae60; color: #27ae60; }
        .form-group { margin-bottom: 1.5rem; }
        .form-group label { display: block; margin-bottom: 0.5rem; color: #e94560; font-weight: bold; }
        .form-group input, .form-group select { width: 100%; padding: 0.75rem; background: #16213e; border: 1px solid #0f3460; border-radius: 6px; color: #e0e0e0; font-size: 1rem; }
        .form-group input:focus, .form-group select:focus { outline: none; border-color: #e94560; }
        .ability-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 1rem; }
        .ability-item { text-align: center; }
        .ability-item label { font-size: 0.9rem; }
        .ability-item input { text-align: center; width: 80px; margin: 0 auto; display: block; }
        .class-entries { display: flex; flex-direction: column; gap: 0.75rem; }
        .class-entry { display: flex; gap: 0.75rem; align-items: center; background: #16213e; padding: 0.75rem; border-radius: 6px; border: 1px solid #0f3460; }
        .class-entry select, .class-entry input { flex: 1; }
        .preview-panel { background: #16213e; border-radius: 8px; padding: 1.5rem; border: 1px solid #0f3460; margin-top: 1.5rem; }
        .preview-panel h2 { color: #e94560; margin-bottom: 1rem; }
        .stat-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 0.75rem; }
        .stat-item { background: #1a1a2e; padding: 0.75rem; border-radius: 6px; text-align: center; }
        .stat-item .label { color: #999; font-size: 0.8rem; text-transform: uppercase; }
        .stat-item .value { font-size: 1.2rem; font-weight: bold; color: #e94560; }
        .btn { padding: 0.75rem 1.5rem; border: none; border-radius: 6px; cursor: pointer; font-size: 1rem; color: white; }
        .btn-primary { background: #e94560; }
        .btn-primary:hover { background: #c73852; }
        .btn-secondary { background: #0f3460; }
        .btn-secondary:hover { background: #0a2540; }
        .btn-success { background: #27ae60; }
        .btn-success:hover { background: #219a52; }
        .actions { display: flex; gap: 1rem; margin-top: 1.5rem; }
        .section { display: none; }
        .section.active { display: block; }
        .item-list { display: flex; flex-wrap: wrap; gap: 0.5rem; margin-top: 0.5rem; }
        .item-tag { background: #0f3460; padding: 0.4rem 0.8rem; border-radius: 4px; font-size: 0.85rem; display: flex; align-items: center; gap: 0.4rem; }
        .item-tag .remove { cursor: pointer; color: #e94560; font-weight: bold; }
        .method-tabs { display:flex; gap:0.5rem; flex-wrap:wrap; margin-bottom:1rem; }
        .method-tabs button.active { background:#e94560; border-color:#e94560; color:white; }
        .feature-card { background: #16213e; border: 1px solid #0f3460; border-radius: 6px; padding: 0.75rem; margin-bottom: 0.5rem; }
        .feature-card .feat-name { color: #e94560; font-weight: bold; }
        .feature-card .feat-source { color: #999; font-size: 0.8rem; }
        .feature-card .feat-desc { margin-top: 0.25rem; font-size: 0.9rem; }
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
        <h1>Create Character</h1>
        <div class="wizard-steps">
            <div class="step active" data-step="1">1. Basics</div>
            <div class="step" data-step="2">2. Classes</div>
            <div class="step" data-step="3">3. Ability Scores</div>
            <div class="step" data-step="4">4. Equipment</div>
            <div class="step" data-step="5">5. Spells</div>
            <div class="step" data-step="6">6. Features</div>
            <div class="step" data-step="7">7. Review</div>
        </div>
        <div id="step-1" class="section active">
            <div class="form-group">
                <label for="char-name">Character Name</label>
                <input type="text" id="char-name" placeholder="Enter character name">
            </div>
            <div class="form-group">
                <label for="char-race">Race</label>
                <select id="char-race"><option value="">Select race...</option></select>
            </div>
            <div class="form-group">
                <label for="char-background">Background</label>
                <input type="text" id="char-background" placeholder="Enter background">
            </div>
            <div class="actions">
                <button class="btn btn-primary" onclick="goToStep(2)">Next: Classes</button>
            </div>
        </div>
        <div id="step-2" class="section">
            <div id="class-entries" class="class-entries"></div>
            <button class="btn btn-secondary" onclick="addClass()" style="margin-top:0.75rem">+ Add Class</button>
            <div class="actions">
                <button class="btn btn-secondary" onclick="goToStep(1)">Back</button>
                <button class="btn btn-primary" onclick="goToStep(3)">Next: Ability Scores</button>
            </div>
        </div>
        <div id="step-3" class="section">
            <div class="method-tabs" id="ability-method-tabs"></div>
            <button class="btn btn-secondary" id="roll-abilities-btn" onclick="rollAbilityScores()" style="display:none;margin-bottom:1rem">Roll 4d6</button>
            <div class="ability-grid" id="ability-grid"></div>
            <div id="preview-panel" class="preview-panel" style="display:none">
                <h2>Derived Stats Preview</h2>
                <div class="stat-grid" id="preview-stats"></div>
            </div>
            <div class="actions">
                <button class="btn btn-secondary" onclick="goToStep(2)">Back</button>
                <button class="btn btn-primary" onclick="computePreview()">Preview Stats</button>
                <button class="btn btn-primary" onclick="goToStep(4)">Next: Equipment</button>
            </div>
        </div>
        <div id="step-4" class="section">
            <button class="btn btn-secondary" onclick="loadStartingEquipment()" style="margin-bottom:1rem">Load Starting Equipment for Class</button>
            <div class="form-group">
                <label for="equip-select">Add Equipment</label>
                <select id="equip-select"><option value="">Select item...</option></select>
                <button class="btn btn-secondary" onclick="addEquipment()" style="margin-top:0.5rem">+ Add</button>
            </div>
            <div id="equipment-list" class="item-list"></div>
            <div class="form-group" style="margin-top:1rem">
                <label for="worn-armor-select">Worn Armor</label>
                <select id="worn-armor-select"><option value="">None</option></select>
            </div>
            <div class="form-group">
                <label for="equipped-weapon-select">Equipped Weapon</label>
                <select id="equipped-weapon-select"><option value="">None</option></select>
            </div>
            <div class="actions">
                <button class="btn btn-secondary" onclick="goToStep(3)">Back</button>
                <button class="btn btn-primary" onclick="goToStep(5)">Next: Spells</button>
            </div>
        </div>
        <div id="step-5" class="section">
            <div id="spells-section">
                <div class="form-group">
                    <label for="spell-class-select">Class Spell List</label>
                    <select id="spell-class-select" onchange="loadSpells(this.value)"><option value="">Select class...</option></select>
                </div>
                <div class="form-group">
                    <label for="spell-select">Add Spell</label>
                    <select id="spell-select"><option value="">Select spell...</option></select>
                    <button class="btn btn-secondary" onclick="addSpell()" style="margin-top:0.5rem">+ Add</button>
                </div>
                <div id="spell-list" class="item-list"></div>
            </div>
            <div class="actions">
                <button class="btn btn-secondary" onclick="goToStep(4)">Back</button>
                <button class="btn btn-primary" onclick="goToStep(6)">Next: Features</button>
            </div>
        </div>
        <div id="step-6" class="section">
            <p style="color:#999;margin-bottom:1rem">Features are auto-populated from your class, subclass, and race choices.</p>
            <div id="features-list"></div>
            <div class="actions">
                <button class="btn btn-secondary" onclick="goToStep(5)">Back</button>
                <button class="btn btn-primary" onclick="goToStep(7)">Next: Review</button>
            </div>
        </div>
        <div id="step-7" class="section">
            <div id="review-summary"></div>
            <div class="actions">
                <button class="btn btn-secondary" onclick="goToStep(6)">Back</button>
                <button class="btn btn-success" onclick="submitCharacter()">Create Character</button>
            </div>
        </div>
        <div id="result-message" style="display:none; margin-top:1rem; padding:1rem; background:#16213e; border-radius:8px; border:1px solid #27ae60;"></div>
    </div>
    <script>
var selectedEquipment = [];
var selectedSpells = [];
var allEquipment = [];
var allSpellsCache = {};
var cachedPreview = null;
var abilityMethod = 'roll';
var abilityRolls = {};
var abilityMethods = ['point_buy','standard_array','roll'];

(function() {
    var abilities = ['STR','DEX','CON','INT','WIS','CHA'];
    var grid = document.getElementById('ability-grid');
    abilities.forEach(function(ab) {
        var d = document.createElement('div');
        d.className = 'ability-item';
        d.innerHTML = '<label>' + ab + '</label><input type="number" id="score-' + ab.toLowerCase() + '" value="10" min="1" max="30">';
        grid.appendChild(d);
    });
    renderAbilityMethods();

    addClass();

    var campaignID = new URLSearchParams(location.search).get('campaign_id') || '';
    fetch('/dashboard/api/characters/ability-methods?campaign_id=' + encodeURIComponent(campaignID)).then(function(r){return r.json()}).then(function(methods){
        abilityMethods = methods && methods.length ? methods : abilityMethods;
        if (abilityMethods.indexOf(abilityMethod) < 0) abilityMethod = abilityMethods[0];
        renderAbilityMethods();
    });

    fetch('/dashboard/api/characters/ref/races').then(function(r){return r.json()}).then(function(races){
        var sel = document.getElementById('char-race');
        (races||[]).forEach(function(r){ var o = document.createElement('option'); o.value = r.name; o.textContent = r.name; sel.appendChild(o); });
    });

    fetch('/dashboard/api/characters/ref/equipment').then(function(r){return r.json()}).then(function(items){
        allEquipment = items || [];
        var sel = document.getElementById('equip-select');
        allEquipment.forEach(function(it){
            var o = document.createElement('option');
            o.value = it.id;
            o.textContent = it.name + ' (' + it.category + ')';
            sel.appendChild(o);
        });
    });
})();

var classOptions = [];
fetch('/dashboard/api/characters/ref/classes').then(function(r){return r.json()}).then(function(classes){
    classOptions = classes || [];
});

function addClass() {
    var container = document.getElementById('class-entries');
    var idx = container.children.length;
    var d = document.createElement('div');
    d.className = 'class-entry';
    d.innerHTML = '<select class="class-select" onchange="updateSubclasses(this)"><option value="">Class...</option></select>' +
        '<select class="subclass-select"><option value="">Subclass...</option></select>' +
        '<input type="number" class="level-input" value="1" min="1" max="20" placeholder="Level">' +
        (idx > 0 ? '<button class="btn btn-secondary" onclick="this.parentElement.remove()" style="padding:0.25rem 0.5rem;font-size:0.8rem">X</button>' : '');
    container.appendChild(d);
    var sel = d.querySelector('.class-select');
    classOptions.forEach(function(c){ var o = document.createElement('option'); o.value = c.name; o.textContent = c.name; sel.appendChild(o); });
}

function updateSubclasses(sel) {
    var subSel = sel.parentElement.querySelector('.subclass-select');
    subSel.innerHTML = '<option value="">Subclass...</option>';
    var cls = classOptions.find(function(c){ return c.name === sel.value; });
    if (cls && cls.subclasses) {
        var subs = typeof cls.subclasses === 'string' ? JSON.parse(cls.subclasses) : cls.subclasses;
        if (typeof subs === 'object' && !Array.isArray(subs)) {
            Object.keys(subs).forEach(function(k){ var s = subs[k]; var name = s.name || k; var o = document.createElement('option'); o.value = name; o.textContent = name; subSel.appendChild(o); });
        } else {
            (subs||[]).forEach(function(s){ var name = s.name || s; var o = document.createElement('option'); o.value = name; o.textContent = name; subSel.appendChild(o); });
        }
    }
}

function renderAbilityMethods() {
    var tabs = document.getElementById('ability-method-tabs');
    tabs.innerHTML = '';
    abilityMethods.forEach(function(method) {
        var btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'btn btn-secondary' + (method === abilityMethod ? ' active' : '');
        btn.textContent = method === 'point_buy' ? 'Point Buy' : method === 'standard_array' ? 'Standard Array' : 'Roll';
        btn.onclick = function() { setAbilityMethod(method); };
        tabs.appendChild(btn);
    });
    document.getElementById('roll-abilities-btn').style.display = abilityMethod === 'roll' ? '' : 'none';
    Array.prototype.forEach.call(document.querySelectorAll('#ability-grid input'), function(input) {
        input.readOnly = abilityMethod === 'standard_array' || abilityMethod === 'roll';
    });
}

function setAbilityMethod(method) {
    abilityMethod = method;
    abilityRolls = {};
    if (method === 'point_buy') setScores({str:8,dex:8,con:8,int:8,wis:8,cha:8});
    if (method === 'standard_array') setScores({str:15,dex:14,con:13,int:12,wis:10,cha:8});
    renderAbilityMethods();
}

function setScores(scores) {
    Object.keys(scores).forEach(function(ab) {
        document.getElementById('score-' + ab).value = scores[ab];
    });
}

function rollAbilityScores() {
    var scores = {};
    abilityRolls = {};
    ['str','dex','con','int','wis','cha'].forEach(function(ab) {
        var dice = [];
        for (var i = 0; i < 4; i++) dice.push(Math.floor(Math.random() * 6) + 1);
        var sorted = dice.slice().sort(function(a,b){ return a-b; });
        scores[ab] = sorted[1] + sorted[2] + sorted[3];
        abilityRolls[ab] = dice;
    });
    setScores(scores);
}

function addEquipment() {
    var sel = document.getElementById('equip-select');
    var id = sel.value;
    if (!id || selectedEquipment.indexOf(id) >= 0) return;
    selectedEquipment.push(id);
    renderEquipmentList();
}

function removeEquipment(id) {
    selectedEquipment = selectedEquipment.filter(function(e){ return e !== id; });
    renderEquipmentList();
}

function renderEquipmentList() {
    var container = document.getElementById('equipment-list');
    container.innerHTML = '';
    selectedEquipment.forEach(function(id) {
        var item = allEquipment.find(function(e){ return e.id === id; });
        var name = item ? item.name : id;
        var tag = document.createElement('span');
        tag.className = 'item-tag';
        tag.innerHTML = name + ' <span class="remove" onclick="removeEquipment(\'' + id + '\')">&times;</span>';
        container.appendChild(tag);
    });
    updateEquipSelects();
}

function updateEquipSelects() {
    var armorSel = document.getElementById('worn-armor-select');
    var weaponSel = document.getElementById('equipped-weapon-select');
    var oldArmor = armorSel.value;
    var oldWeapon = weaponSel.value;
    armorSel.innerHTML = '<option value="">None</option>';
    weaponSel.innerHTML = '<option value="">None</option>';
    selectedEquipment.forEach(function(id) {
        var item = allEquipment.find(function(e){ return e.id === id; });
        var name = item ? item.name : id;
        var cat = item ? item.category : '';
        if (cat === 'armor' || id === 'shield') {
            var o = document.createElement('option'); o.value = id; o.textContent = name; armorSel.appendChild(o);
        }
        if (cat === 'weapon') {
            var o = document.createElement('option'); o.value = id; o.textContent = name; weaponSel.appendChild(o);
        }
    });
    armorSel.value = oldArmor;
    weaponSel.value = oldWeapon;
}

function loadStartingEquipment() {
    var entries = document.querySelectorAll('.class-entry');
    var cls = '';
    if (entries.length > 0) cls = entries[0].querySelector('.class-select').value;
    if (!cls) { alert('Select a class first.'); return; }
    fetch('/dashboard/api/characters/ref/starting-equipment?class=' + encodeURIComponent(cls))
        .then(function(r){return r.json()})
        .then(function(packs) {
            if (!packs || packs.length === 0) return;
            var pack = packs[0];
            (pack.guaranteed || []).forEach(function(item) {
                var id = item.split(':')[0];
                if (selectedEquipment.indexOf(id) < 0) selectedEquipment.push(id);
            });
            (pack.choices || []).forEach(function(choice) {
                if (choice.options && choice.options.length > 0) {
                    var id = choice.options[0].split(':')[0].split(',')[0];
                    if (selectedEquipment.indexOf(id) < 0) selectedEquipment.push(id);
                }
            });
            renderEquipmentList();
        });
}

function loadSpells(className) {
    if (!className) return;
    var maxLevel = cachedPreview ? cachedPreview.max_spell_level : 0;
    var url = '/dashboard/api/characters/ref/spells?class=' + encodeURIComponent(className);
    if (maxLevel > 0) url += '&max_level=' + maxLevel;
    var cacheKey = className + '_' + maxLevel;
    if (allSpellsCache[cacheKey]) {
        populateSpellSelect(allSpellsCache[cacheKey]);
        return;
    }
    fetch(url)
        .then(function(r){return r.json()})
        .then(function(spells){
            allSpellsCache[cacheKey] = spells || [];
            populateSpellSelect(allSpellsCache[cacheKey]);
        });
}

function populateSpellSelect(spells) {
    var sel = document.getElementById('spell-select');
    sel.innerHTML = '<option value="">Select spell...</option>';
    spells.forEach(function(sp){
        var o = document.createElement('option');
        o.value = sp.id;
        o.textContent = sp.name + ' (Lvl ' + sp.level + ')';
        sel.appendChild(o);
    });
}

function addSpell() {
    var sel = document.getElementById('spell-select');
    var id = sel.value;
    if (!id || selectedSpells.indexOf(id) >= 0) return;
    selectedSpells.push(id);
    renderSpellList();
}

function removeSpell(id) {
    selectedSpells = selectedSpells.filter(function(s){ return s !== id; });
    renderSpellList();
}

function renderSpellList() {
    var container = document.getElementById('spell-list');
    container.innerHTML = '';
    selectedSpells.forEach(function(id) {
        var tag = document.createElement('span');
        tag.className = 'item-tag';
        tag.innerHTML = id + ' <span class="remove" onclick="removeSpell(\'' + id + '\')">&times;</span>';
        container.appendChild(tag);
    });
}

function populateSpellClassSelect() {
    var sel = document.getElementById('spell-class-select');
    sel.innerHTML = '<option value="">Select class...</option>';
    var entries = document.querySelectorAll('.class-entry');
    entries.forEach(function(e) {
        var cls = e.querySelector('.class-select').value;
        if (cls) {
            var o = document.createElement('option');
            o.value = cls; o.textContent = cls;
            sel.appendChild(o);
        }
    });
}

function goToStep(n) {
    if (n === 5) populateSpellClassSelect();
    if (n === 6) renderFeatures();
    if (n === 7) renderReview();
    document.querySelectorAll('.section').forEach(function(s){s.classList.remove('active')});
    document.getElementById('step-' + n).classList.add('active');
    document.querySelectorAll('.wizard-steps .step').forEach(function(s){
        var sn = parseInt(s.dataset.step);
        s.classList.toggle('active', sn === n);
        s.classList.toggle('done', sn < n);
    });
}

function renderFeatures() {
    var container = document.getElementById('features-list');
    container.innerHTML = '<p style="color:#999">Loading features...</p>';
    var data = gatherData();
    fetch('/dashboard/api/characters/preview', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify(data)})
        .then(function(r){return r.json()})
        .then(function(stats) {
            cachedPreview = stats;
            var features = stats.features || [];
            if (features.length === 0) {
                container.innerHTML = '<p style="color:#999">No features available for current class selection.</p>';
                return;
            }
            var html = '';
            features.forEach(function(f) {
                html += '<div class="feature-card">';
                html += '<div class="feat-name">' + f.name + '</div>';
                html += '<div class="feat-source">' + f.source + ' (Level ' + f.level + ')</div>';
                html += '<div class="feat-desc">' + (f.description || '') + '</div>';
                html += '</div>';
            });
            container.innerHTML = html;
        })
        .catch(function() {
            container.innerHTML = '<p style="color:#999">Could not load features.</p>';
        });
}

function renderReview() {
    var data = gatherData();
    var html = '<h2 style="color:#e94560;margin-bottom:1rem">Review</h2>';
    html += '<p><strong>Name:</strong> ' + (data.name || '(none)') + '</p>';
    html += '<p><strong>Race:</strong> ' + (data.race || '(none)') + '</p>';
    html += '<p><strong>Background:</strong> ' + (data.background || '(none)') + '</p>';
    html += '<p><strong>Classes:</strong> ';
    data.classes.forEach(function(c,i){ if(i>0) html+=' / '; html += c.class + ' ' + c.level + (c.subclass ? ' (' + c.subclass + ')' : ''); });
    html += '</p>';
    html += '<p><strong>Equipment:</strong> ' + (selectedEquipment.length > 0 ? selectedEquipment.join(', ') : '(none)') + '</p>';
    if (data.worn_armor) html += '<p><strong>Worn Armor:</strong> ' + data.worn_armor + '</p>';
    if (data.equipped_weapon) html += '<p><strong>Equipped Weapon:</strong> ' + data.equipped_weapon + '</p>';
    html += '<p><strong>Spells:</strong> ' + (selectedSpells.length > 0 ? selectedSpells.join(', ') : '(none)') + '</p>';
    document.getElementById('review-summary').innerHTML = html;
}

function gatherData() {
    var entries = document.querySelectorAll('.class-entry');
    var classes = [];
    entries.forEach(function(e){
        var cls = e.querySelector('.class-select').value;
        var sub = e.querySelector('.subclass-select').value;
        var lvl = parseInt(e.querySelector('.level-input').value) || 1;
        if (cls) classes.push({class:cls, subclass:sub, level:lvl});
    });
    return {
        name: document.getElementById('char-name').value,
        race: document.getElementById('char-race').value,
        background: document.getElementById('char-background').value,
        classes: classes,
        ability_scores: {
            str: parseInt(document.getElementById('score-str').value) || 10,
            dex: parseInt(document.getElementById('score-dex').value) || 10,
            con: parseInt(document.getElementById('score-con').value) || 10,
            int: parseInt(document.getElementById('score-int').value) || 10,
            wis: parseInt(document.getElementById('score-wis').value) || 10,
            cha: parseInt(document.getElementById('score-cha').value) || 10,
        },
        ability_method: abilityMethod,
        ability_rolls: abilityRolls,
        equipment: selectedEquipment,
        spells: selectedSpells,
        equipped_weapon: document.getElementById('equipped-weapon-select').value,
        worn_armor: document.getElementById('worn-armor-select').value
    };
}

function computePreview() {
    var data = gatherData();
    fetch('/dashboard/api/characters/preview', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify(data)})
        .then(function(r){return r.json()})
        .then(function(stats){
            cachedPreview = stats;
            var panel = document.getElementById('preview-panel');
            panel.style.display = '';
            var html = stat('HP', stats.hp_max) + stat('AC', stats.ac) +
                stat('Speed', stats.speed_ft + ' ft') + stat('Prof', '+' + stats.proficiency_bonus) +
                stat('Level', stats.total_level);
            if (stats.saves) {
                ['str','dex','con','int','wis','cha'].forEach(function(ab){
                    var v = stats.saves[ab]; var sign = v >= 0 ? '+' : '';
                    html += stat(ab.toUpperCase() + ' Save', sign + v);
                });
            }
            if (stats.spell_slots) {
                Object.keys(stats.spell_slots).sort().forEach(function(lvl){
                    html += stat('Slot Lvl ' + lvl, stats.spell_slots[lvl]);
                });
            }
            document.getElementById('preview-stats').innerHTML = html;
        });
}

function stat(label, value) {
    return '<div class="stat-item"><div class="label">' + label + '</div><div class="value">' + value + '</div></div>';
}

function submitCharacter() {
    var data = gatherData();
    var payload = {campaign_id: new URLSearchParams(location.search).get('campaign_id') || '', name: data.name, race: data.race, background: data.background, classes: data.classes, ability_scores: data.ability_scores, equipment: data.equipment, spells: data.spells, equipped_weapon: data.equipped_weapon, worn_armor: data.worn_armor};
    fetch('/dashboard/api/characters', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify(payload)})
        .then(function(r){
            if (!r.ok) return r.text().then(function(t){throw new Error(t)});
            return r.json();
        })
        .then(function(result){
            var msg = document.getElementById('result-message');
            msg.style.display = '';
            msg.style.borderColor = '#27ae60';
            msg.textContent = 'Character created! ID: ' + result.character_id;
        })
        .catch(function(err){
            var msg = document.getElementById('result-message');
            msg.style.display = '';
            msg.style.borderColor = '#e74c3c';
            msg.textContent = 'Error: ' + err.message;
        });
}
    </script>
</body>
</html>`
