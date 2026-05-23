package exploration

import (
	"context"
	"encoding/json"
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

// DashboardHandler serves the exploration dashboard JSON endpoints consumed
// by the Svelte SPA panel.
//   - GET  /api/exploration?campaign_id=<id>            lists maps the DM can pick.
//   - POST /dashboard/exploration/start                 kicks off a new exploration encounter.
//   - POST /dashboard/exploration/transition-to-combat  flips an encounter to combat.
type DashboardHandler struct {
	svc  *Service
	maps MapLister
}

// NewDashboardHandler constructs a DashboardHandler.
func NewDashboardHandler(svc *Service, maps MapLister) *DashboardHandler {
	return &DashboardHandler{svc: svc, maps: maps}
}

// RegisterRoutes mounts the exploration dashboard routes on r.
func (h *DashboardHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/exploration", h.HandleGetData)
	r.Post("/dashboard/exploration/start", h.HandleStart)
	r.Post("/dashboard/exploration/transition-to-combat", h.HandleTransitionToCombat)
}

// explorationDataResponse is the JSON shape returned by GET /api/exploration.
type explorationDataResponse struct {
	CampaignID string         `json:"campaign_id"`
	Maps       []refdata.Map  `json:"maps"`
}

// HandleGetData returns the maps available to start an exploration encounter
// for the supplied campaign. The Svelte panel calls this on mount to drive
// the map selector dropdown.
func (h *DashboardHandler) HandleGetData(w http.ResponseWriter, r *http.Request) {
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
	if maps == nil {
		maps = []refdata.Map{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(explorationDataResponse{
		CampaignID: campaignIDStr,
		Maps:       maps,
	})
}

// startExplorationRequest is the JSON body accepted by HandleStart.
type startExplorationRequest struct {
	CampaignID   string   `json:"campaign_id"`
	MapID        string   `json:"map_id"`
	Name         string   `json:"name"`
	CharacterIDs []string `json:"character_ids"`
}

// HandleStart processes the POST to start a new exploration encounter from
// a JSON body. Returns JSON: {"encounter_id":..., "mode":..., "pcs":...}.
func (h *DashboardHandler) HandleStart(w http.ResponseWriter, r *http.Request) {
	var req startExplorationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.CampaignID == "" || req.MapID == "" {
		http.Error(w, "campaign_id and map_id are required", http.StatusBadRequest)
		return
	}
	campID, err := uuid.Parse(req.CampaignID)
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}
	mapID, err := uuid.Parse(req.MapID)
	if err != nil {
		http.Error(w, "invalid map_id", http.StatusBadRequest)
		return
	}

	charIDs := make([]uuid.UUID, 0, len(req.CharacterIDs))
	for _, s := range req.CharacterIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			http.Error(w, "invalid character_id "+s, http.StatusBadRequest)
			return
		}
		charIDs = append(charIDs, id)
	}

	name := strings.TrimSpace(req.Name)
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

// transitionToCombatRequest is the JSON body accepted by
// HandleTransitionToCombat. Overrides map character UUIDs to coordinates
// like "D5".
type transitionToCombatRequest struct {
	EncounterID string            `json:"encounter_id"`
	Overrides   map[string]string `json:"overrides"`
}

// HandleTransitionToCombat captures the current exploration encounter's PC
// positions, flips the encounter to combat mode, and applies any per-PC
// overrides supplied in the JSON body. Returns JSON:
//
//	{"mode": "combat", "positions": {"<character_id>": {"col": "D", "row": 5}, ...}}
//
// The merged map is what a combat-transition flow would feed into
// StartCombatInput.CharacterPositions.
func (h *DashboardHandler) HandleTransitionToCombat(w http.ResponseWriter, r *http.Request) {
	var req transitionToCombatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.EncounterID == "" {
		http.Error(w, "encounter_id is required", http.StatusBadRequest)
		return
	}
	encID, err := uuid.Parse(req.EncounterID)
	if err != nil {
		http.Error(w, "invalid encounter_id", http.StatusBadRequest)
		return
	}

	overrides := map[uuid.UUID]combat.Position{}
	for rawCharID, rawCoord := range req.Overrides {
		coord := strings.TrimSpace(rawCoord)
		if coord == "" {
			continue
		}
		charID, err := uuid.Parse(rawCharID)
		if err != nil {
			http.Error(w, "invalid override character id "+rawCharID, http.StatusBadRequest)
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

	result, err := h.svc.TransitionToCombat(r.Context(), encID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	merged := ApplyPositionOverrides(result.Positions, overrides)

	posMap := make(map[string]map[string]any, len(merged))
	for k, v := range merged {
		posMap[k.String()] = map[string]any{"col": v.Col, "row": v.Row}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"mode":      result.Encounter.Mode,
		"positions": posMap,
	})
}
