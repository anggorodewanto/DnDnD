package exploration

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/refdata"
)

// MapLister is the narrow dashboard dependency on the map store.
type MapLister interface {
	ListMapsByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Map, error)
}

// DashboardHandler serves the Phase 110 exploration dashboard page (Q4a).
//   - GET  /dashboard/exploration?campaign_id=<id> lists maps and lets the DM
//     pick one to start exploration.
//   - POST /dashboard/exploration/start kicks off a new exploration encounter.
type DashboardHandler struct {
	svc  *Service
	maps MapLister
	tmpl *template.Template
}

// NewDashboardHandler constructs a DashboardHandler.
func NewDashboardHandler(svc *Service, maps MapLister) *DashboardHandler {
	return &DashboardHandler{
		svc:  svc,
		maps: maps,
		tmpl: template.Must(template.New("exploration_dashboard").Parse(dashboardTemplate)),
	}
}

// RegisterRoutes mounts the exploration dashboard routes on r.
func (h *DashboardHandler) RegisterRoutes(r chi.Router) {
	r.Get("/dashboard/exploration", h.ServePage)
	r.Post("/dashboard/exploration/start", h.HandleStart)
	r.Post("/dashboard/exploration/transition-to-combat", h.HandleTransitionToCombat)
}

type dashboardView struct {
	CampaignID string
	Maps       []refdata.Map
}

// ServePage renders the GET page.
func (h *DashboardHandler) ServePage(w http.ResponseWriter, r *http.Request) {
	campaignIDStr := r.URL.Query().Get("campaign_id")
	if campaignIDStr == "" {
		http.Error(w, "campaign_id is required", http.StatusBadRequest)
		return
	}
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		http.Error(w, "campaign_id is not a valid UUID", http.StatusBadRequest)
		return
	}
	maps, err := h.maps.ListMapsByCampaignID(r.Context(), campaignID)
	if err != nil {
		http.Error(w, "failed to list maps", http.StatusInternalServerError)
		return
	}
	view := dashboardView{
		CampaignID: campaignIDStr,
		Maps:       maps,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.Execute(w, view); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
}

// HandleStart processes the POST to start a new exploration encounter.
// Form fields: campaign_id, map_id, name (optional).
// character_ids may be repeated; each entry is a UUID string.
func (h *DashboardHandler) HandleStart(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	campIDStr := r.Form.Get("campaign_id")
	mapIDStr := r.Form.Get("map_id")
	name := strings.TrimSpace(r.Form.Get("name"))
	if campIDStr == "" || mapIDStr == "" {
		http.Error(w, "campaign_id and map_id are required", http.StatusBadRequest)
		return
	}
	campID, err := uuid.Parse(campIDStr)
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}
	mapID, err := uuid.Parse(mapIDStr)
	if err != nil {
		http.Error(w, "invalid map_id", http.StatusBadRequest)
		return
	}

	var charIDs []uuid.UUID
	for _, s := range r.Form["character_ids"] {
		id, err := uuid.Parse(s)
		if err != nil {
			http.Error(w, "invalid character_id "+s, http.StatusBadRequest)
			return
		}
		charIDs = append(charIDs, id)
	}

	if name == "" {
		name = "Exploration"
	}

	out, err := h.svc.StartExploration(r.Context(), StartInput{
		CampaignID:   campID,
		MapID:        mapID,
		Name:         name,
		CharacterIDs: charIDs,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"encounter_id": out.Encounter.ID,
		"mode":         out.Encounter.Mode,
		"pcs":          out.PCs,
	})
}

// HandleTransitionToCombat captures the current exploration encounter's PC
// positions and applies any per-PC override_<characterID>=<coord> form fields
// supplied by the DM (Phase 110 Q3/Q4 clarification). Returns JSON:
//
//	{"positions": {"<character_id>": {"col": "D", "row": 5}, ...}}
//
// The merged map is what a combat-transition flow would feed into
// StartCombatInput.CharacterPositions.
func (h *DashboardHandler) HandleTransitionToCombat(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	encIDStr := r.Form.Get("encounter_id")
	if encIDStr == "" {
		http.Error(w, "encounter_id is required", http.StatusBadRequest)
		return
	}
	encID, err := uuid.Parse(encIDStr)
	if err != nil {
		http.Error(w, "invalid encounter_id", http.StatusBadRequest)
		return
	}

	// Parse overrides: any form field starting with "override_" and whose
	// suffix parses as a UUID is treated as a character override.
	overrides := map[uuid.UUID]combat.Position{}
	for key, vals := range r.Form {
		if !strings.HasPrefix(key, "override_") || len(vals) == 0 {
			continue
		}
		coord := strings.TrimSpace(vals[0])
		if coord == "" {
			continue
		}
		charID, err := uuid.Parse(strings.TrimPrefix(key, "override_"))
		if err != nil {
			http.Error(w, "invalid override character id "+key, http.StatusBadRequest)
			return
		}
		col, row, err := renderer.ParseCoordinate(coord)
		if err != nil {
			http.Error(w, "invalid override coordinate "+coord, http.StatusBadRequest)
			return
		}
		overrides[charID] = combat.Position{
			Col: renderer.ColumnLabel(col),
			Row: int32(row + 1),
		}
	}

	base, err := h.svc.CapturePositions(r.Context(), encID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	merged := ApplyPositionOverrides(base, overrides)

	posMap := make(map[string]map[string]any, len(merged))
	for k, v := range merged {
		posMap[k.String()] = map[string]any{"col": v.Col, "row": v.Row}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"positions": posMap})
}

// dashboardTemplate is a minimal HTML page: a table of maps with a
// "Start Exploration" button per map, submitting a POST.
const dashboardTemplate = `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Exploration</title>
<style>
 body { font-family: sans-serif; padding: 2em; }
 table { border-collapse: collapse; }
 td, th { border: 1px solid #ccc; padding: 0.5em 1em; }
 button { cursor: pointer; }
</style>
</head><body>
<h1>Exploration Mode</h1>
<p>Campaign: <code>{{.CampaignID}}</code></p>
{{if .Maps}}
<table>
  <thead><tr><th>Name</th><th>Size</th><th>Action</th></tr></thead>
  <tbody>
  {{range .Maps}}
    <tr>
      <td>{{.Name}}</td>
      <td>{{.WidthSquares}}x{{.HeightSquares}}</td>
      <td>
        <form method="POST" action="/dashboard/exploration/start">
          <input type="hidden" name="campaign_id" value="{{$.CampaignID}}">
          <input type="hidden" name="map_id" value="{{.ID}}">
          <input type="text" name="name" placeholder="Encounter name" value="Exploring {{.Name}}">
          <button type="submit">Start Exploration</button>
        </form>
      </td>
    </tr>
  {{end}}
  </tbody>
</table>
{{else}}
<p>No maps found for this campaign. Create one first.</p>
{{end}}

<h2>Transition to Combat</h2>
<p>When a combat encounter begins, PC positions carry over from exploration.
The DM may override any per-PC position by supplying an
<code>override_&lt;character_id&gt;</code> field with a coordinate (e.g. <code>D5</code>).</p>
<form method="POST" action="/dashboard/exploration/transition-to-combat">
  <label>Encounter ID: <input type="text" name="encounter_id" required></label>
  <br>
  <label>Override character_id: <input type="text" name="override_char_id_placeholder" disabled placeholder="override_<character_id>"></label>
  <label>Coord: <input type="text" name="override_coord_placeholder" disabled placeholder="D5"></label>
  <br>
  <small>Add <code>override_&lt;uuid&gt;=&lt;coord&gt;</code> fields via JS or a client tool to apply per-PC overrides. Omit overrides to carry over captured positions as-is.</small>
  <br>
  <button type="submit">Capture Positions &amp; Apply Overrides</button>
</form>
</body></html>
`
