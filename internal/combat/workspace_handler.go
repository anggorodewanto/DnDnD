package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// WorkspaceStore defines the database operations needed by the workspace handler.
type WorkspaceStore interface {
	ListEncountersByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error)
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	GetMapByID(ctx context.Context, id uuid.UUID) (refdata.Map, error)
	ListEncounterZonesByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.EncounterZone, error)
	UpdateCombatantHP(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error)
	UpdateCombatantConditions(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error)
	GetCombatantByID(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
	GetActiveTurnByEncounterID(ctx context.Context, encounterID uuid.UUID) (refdata.Turn, error)
	UpdateCombatantPosition(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error)
	DeleteCombatant(ctx context.Context, id uuid.UUID) error
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	GetCreature(ctx context.Context, id string) (refdata.Creature, error)
}

// WorkspaceHandler serves combat workspace API endpoints.
type WorkspaceHandler struct {
	store WorkspaceStore
}

// NewWorkspaceHandler creates a new WorkspaceHandler.
func NewWorkspaceHandler(store WorkspaceStore) *WorkspaceHandler {
	return &WorkspaceHandler{store: store}
}

// RegisterRoutes mounts workspace API routes on the given Chi router.
func (h *WorkspaceHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/combat", func(r chi.Router) {
		r.Get("/workspace", h.GetWorkspace)
		r.Patch("/{encounterID}/combatants/{combatantID}/hp", h.UpdateCombatantHP)
		r.Patch("/{encounterID}/combatants/{combatantID}/conditions", h.UpdateCombatantConditions)
		r.Patch("/{encounterID}/combatants/{combatantID}/position", h.UpdateCombatantPosition)
		r.Delete("/{encounterID}/combatants/{combatantID}", h.DeleteCombatant)
	})
}

// --- Response types ---

type workspaceResponse struct {
	Encounters []workspaceEncounterResponse `json:"encounters"`
}

type workspaceEncounterResponse struct {
	ID                      string                       `json:"id"`
	Name                    string                       `json:"name"`
	DisplayName             string                       `json:"display_name"`
	Status                  string                       `json:"status"`
	RoundNumber             int32                        `json:"round_number"`
	Combatants              []workspaceCombatantResponse `json:"combatants"`
	Map                     *workspaceMapResponse        `json:"map"`
	Zones                   []workspaceZoneResponse      `json:"zones"`
	ActiveTurnCombatantID   string                       `json:"active_turn_combatant_id"`
	ActiveTurnCombatantName string                       `json:"active_turn_combatant_name"`
}

type workspaceCombatantResponse struct {
	ID              string          `json:"id"`
	ShortID         string          `json:"short_id"`
	DisplayName     string          `json:"display_name"`
	HpMax           int32           `json:"hp_max"`
	HpCurrent       int32           `json:"hp_current"`
	TempHp          int32           `json:"temp_hp"`
	Ac              int32           `json:"ac"`
	PositionCol     string          `json:"position_col"`
	PositionRow     int32           `json:"position_row"`
	AltitudeFt      int32           `json:"altitude_ft"`
	IsNpc           bool            `json:"is_npc"`
	IsAlive         bool            `json:"is_alive"`
	IsVisible       bool            `json:"is_visible"`
	Conditions      json.RawMessage `json:"conditions"`
	ExhaustionLevel int32           `json:"exhaustion_level"`
	InitiativeRoll  int32           `json:"initiative_roll"`
	InitiativeOrder int32           `json:"initiative_order"`
	SpeedFt         int32           `json:"speed_ft"`
}

type workspaceMapResponse struct {
	ID            string          `json:"id"`
	WidthSquares  int32           `json:"width_squares"`
	HeightSquares int32           `json:"height_squares"`
	TiledJson     json.RawMessage `json:"tiled_json"`
}

type workspaceZoneResponse struct {
	ID           string          `json:"id"`
	SourceSpell  string          `json:"source_spell"`
	Shape        string          `json:"shape"`
	OriginCol    string          `json:"origin_col"`
	OriginRow    int32           `json:"origin_row"`
	Dimensions   json.RawMessage `json:"dimensions"`
	ZoneType     string          `json:"zone_type"`
	OverlayColor string          `json:"overlay_color"`
}

// --- Request types ---

type updateHPRequest struct {
	HpCurrent int32 `json:"hp_current"`
	TempHp    int32 `json:"temp_hp"`
	IsAlive   bool  `json:"is_alive"`
}

type updateConditionsRequest struct {
	Conditions []string `json:"conditions"`
}

type updatePositionRequest struct {
	PositionCol string `json:"position_col"`
	PositionRow int32  `json:"position_row"`
}

// --- Handlers ---

// GetWorkspace handles GET /api/combat/workspace?campaign_id=X.
func (h *WorkspaceHandler) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	campaignIDStr := r.URL.Query().Get("campaign_id")
	if campaignIDStr == "" {
		http.Error(w, "campaign_id query parameter required", http.StatusBadRequest)
		return
	}

	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}

	allEncounters, err := h.store.ListEncountersByCampaignID(r.Context(), campaignID)
	if err != nil {
		http.Error(w, "failed to list encounters", http.StatusInternalServerError)
		return
	}

	// Filter to active encounters only
	var activeEncounters []refdata.Encounter
	for _, enc := range allEncounters {
		if enc.Status == "active" {
			activeEncounters = append(activeEncounters, enc)
		}
	}

	resp := workspaceResponse{
		Encounters: make([]workspaceEncounterResponse, 0, len(activeEncounters)),
	}

	for _, enc := range activeEncounters {
		encResp, err := h.buildEncounterResponse(r.Context(), enc)
		if err != nil {
			http.Error(w, "failed to build encounter data", http.StatusInternalServerError)
			return
		}
		resp.Encounters = append(resp.Encounters, encResp)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *WorkspaceHandler) buildEncounterResponse(ctx context.Context, enc refdata.Encounter) (workspaceEncounterResponse, error) {
	combatants, err := h.store.ListCombatantsByEncounterID(ctx, enc.ID)
	if err != nil {
		return workspaceEncounterResponse{}, err
	}

	combResp := make([]workspaceCombatantResponse, len(combatants))
	for i, c := range combatants {
		combResp[i] = toWorkspaceCombatantResponse(c)
		combResp[i].SpeedFt = h.resolveSpeedFt(ctx, c)
	}

	var mapResp *workspaceMapResponse
	if enc.MapID.Valid {
		m, err := h.store.GetMapByID(ctx, enc.MapID.UUID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return workspaceEncounterResponse{}, err
		}
		if err == nil {
			mapResp = &workspaceMapResponse{
				ID:            m.ID.String(),
				WidthSquares:  m.WidthSquares,
				HeightSquares: m.HeightSquares,
				TiledJson:     m.TiledJson,
			}
		}
	}

	zones, err := h.store.ListEncounterZonesByEncounterID(ctx, enc.ID)
	if err != nil {
		return workspaceEncounterResponse{}, err
	}

	zoneResp := make([]workspaceZoneResponse, len(zones))
	for i, z := range zones {
		zoneResp[i] = workspaceZoneResponse{
			ID:           z.ID.String(),
			SourceSpell:  z.SourceSpell,
			Shape:        z.Shape,
			OriginCol:    z.OriginCol,
			OriginRow:    z.OriginRow,
			Dimensions:   z.Dimensions,
			ZoneType:     z.ZoneType,
			OverlayColor: z.OverlayColor,
		}
	}

	activeTurnCombatantID := ""
	activeTurnCombatantName := ""
	if enc.CurrentTurnID.Valid {
		turn, err := h.store.GetActiveTurnByEncounterID(ctx, enc.ID)
		if err == nil {
			activeTurnCombatantID = turn.CombatantID.String()
			for _, c := range combatants {
				if c.ID == turn.CombatantID {
					activeTurnCombatantName = c.DisplayName
					break
				}
			}
		}
	}

	return workspaceEncounterResponse{
		ID:                      enc.ID.String(),
		Name:                    enc.Name,
		DisplayName:             EncounterDisplayName(enc),
		Status:                  enc.Status,
		RoundNumber:             enc.RoundNumber,
		Combatants:              combResp,
		Map:                     mapResp,
		Zones:                   zoneResp,
		ActiveTurnCombatantID:   activeTurnCombatantID,
		ActiveTurnCombatantName: activeTurnCombatantName,
	}, nil
}

// resolveSpeedFt looks up the speed for a combatant from its source (character or creature).
// Falls back to 30 if lookup fails.
func (h *WorkspaceHandler) resolveSpeedFt(ctx context.Context, c refdata.Combatant) int32 {
	if c.CharacterID.Valid {
		char, err := h.store.GetCharacter(ctx, c.CharacterID.UUID)
		if err == nil {
			return char.SpeedFt
		}
	}
	if c.CreatureRefID.Valid {
		creature, err := h.store.GetCreature(ctx, c.CreatureRefID.String)
		if err == nil {
			return parseCreatureWalkSpeed(creature.Speed)
		}
	}
	return 30
}

// parseCreatureWalkSpeed extracts the walk speed from a creature speed JSON like {"walk":30,"fly":60}.
func parseCreatureWalkSpeed(speedJSON json.RawMessage) int32 {
	if len(speedJSON) == 0 {
		return 30
	}
	var speeds map[string]int32
	if err := json.Unmarshal(speedJSON, &speeds); err != nil {
		return 30
	}
	if walk, ok := speeds["walk"]; ok {
		return walk
	}
	return 30
}

func toWorkspaceCombatantResponse(c refdata.Combatant) workspaceCombatantResponse {
	return workspaceCombatantResponse{
		ID:              c.ID.String(),
		ShortID:         c.ShortID,
		DisplayName:     c.DisplayName,
		HpMax:           c.HpMax,
		HpCurrent:       c.HpCurrent,
		TempHp:          c.TempHp,
		Ac:              c.Ac,
		PositionCol:     c.PositionCol,
		PositionRow:     c.PositionRow,
		AltitudeFt:      c.AltitudeFt,
		IsNpc:           c.IsNpc,
		IsAlive:         c.IsAlive,
		IsVisible:       c.IsVisible,
		Conditions:      c.Conditions,
		ExhaustionLevel: c.ExhaustionLevel,
		InitiativeRoll:  c.InitiativeRoll,
		InitiativeOrder: c.InitiativeOrder,
	}
}

// UpdateCombatantHP handles PATCH /api/combat/{encounterID}/combatants/{combatantID}/hp.
func (h *WorkspaceHandler) UpdateCombatantHP(w http.ResponseWriter, r *http.Request) {
	_, combatantID, err := parseWorkspaceRouteParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req updateHPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	c, err := h.store.UpdateCombatantHP(r.Context(), refdata.UpdateCombatantHPParams{
		ID:        combatantID,
		HpCurrent: req.HpCurrent,
		TempHp:    req.TempHp,
		IsAlive:   req.IsAlive,
	})
	if err != nil {
		http.Error(w, "failed to update HP", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, toWorkspaceCombatantResponse(c))
}

// UpdateCombatantConditions handles PATCH /api/combat/{encounterID}/combatants/{combatantID}/conditions.
func (h *WorkspaceHandler) UpdateCombatantConditions(w http.ResponseWriter, r *http.Request) {
	_, combatantID, err := parseWorkspaceRouteParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req updateConditionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	existing, err := h.store.GetCombatantByID(r.Context(), combatantID)
	if err != nil {
		http.Error(w, "failed to fetch combatant", http.StatusInternalServerError)
		return
	}

	conditionsJSON, err := json.Marshal(req.Conditions)
	if err != nil {
		http.Error(w, "failed to serialize conditions", http.StatusInternalServerError)
		return
	}

	c, err := h.store.UpdateCombatantConditions(r.Context(), refdata.UpdateCombatantConditionsParams{
		ID:              combatantID,
		Conditions:      conditionsJSON,
		ExhaustionLevel: existing.ExhaustionLevel,
	})
	if err != nil {
		http.Error(w, "failed to update conditions", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, toWorkspaceCombatantResponse(c))
}

// UpdateCombatantPosition handles PATCH /api/combat/{encounterID}/combatants/{combatantID}/position.
func (h *WorkspaceHandler) UpdateCombatantPosition(w http.ResponseWriter, r *http.Request) {
	_, combatantID, err := parseWorkspaceRouteParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req updatePositionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.PositionCol == "" {
		http.Error(w, "position_col is required", http.StatusBadRequest)
		return
	}

	existing, err := h.store.GetCombatantByID(r.Context(), combatantID)
	if err != nil {
		http.Error(w, "combatant not found", http.StatusNotFound)
		return
	}

	c, err := h.store.UpdateCombatantPosition(r.Context(), refdata.UpdateCombatantPositionParams{
		ID:          combatantID,
		PositionCol: req.PositionCol,
		PositionRow: req.PositionRow,
		AltitudeFt:  existing.AltitudeFt,
	})
	if err != nil {
		http.Error(w, "failed to update position", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, toWorkspaceCombatantResponse(c))
}

// DeleteCombatant handles DELETE /api/combat/{encounterID}/combatants/{combatantID}.
func (h *WorkspaceHandler) DeleteCombatant(w http.ResponseWriter, r *http.Request) {
	_, combatantID, err := parseWorkspaceRouteParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteCombatant(r.Context(), combatantID); err != nil {
		http.Error(w, "failed to delete combatant", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseWorkspaceRouteParams(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		return uuid.Nil, uuid.Nil, errors.New("invalid encounter ID")
	}
	combatantID, err := uuid.Parse(chi.URLParam(r, "combatantID"))
	if err != nil {
		return uuid.Nil, uuid.Nil, errors.New("invalid combatant ID")
	}
	return encounterID, combatantID, nil
}
