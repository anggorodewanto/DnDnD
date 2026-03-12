package combat

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dice"
)

// Handler serves combat API endpoints over HTTP.
type Handler struct {
	svc    *Service
	roller *dice.Roller
}

// NewHandler creates a new combat Handler.
func NewHandler(svc *Service, roller *dice.Roller) *Handler {
	return &Handler{svc: svc, roller: roller}
}

// RegisterRoutes mounts combat API routes on the given Chi router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/combat", func(r chi.Router) {
		r.Post("/start", h.StartCombat)
		r.Get("/characters", h.ListCharacters)
	})
}

// startCombatRequest is the JSON request body for starting combat.
type startCombatRequest struct {
	TemplateID                 string              `json:"template_id"`
	CharacterIDs               []string            `json:"character_ids"`
	CharacterPositions         map[string]Position `json:"character_positions"`
	SurprisedCombatantShortIDs []string            `json:"surprised_combatant_short_ids,omitempty"`
}

// startCombatResponse is the JSON response for the start combat flow.
type startCombatResponse struct {
	Encounter         encounterResponse   `json:"encounter"`
	Combatants        []combatantResponse `json:"combatants"`
	InitiativeTracker string              `json:"initiative_tracker"`
	FirstTurn         turnInfoResponse    `json:"first_turn"`
}

// encounterResponse is the JSON representation of an encounter.
type encounterResponse struct {
	ID          string `json:"id"`
	CampaignID  string `json:"campaign_id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	RoundNumber int32  `json:"round_number"`
}

// combatantResponse is the JSON representation of a combatant.
type combatantResponse struct {
	ID              string `json:"id"`
	ShortID         string `json:"short_id"`
	DisplayName     string `json:"display_name"`
	InitiativeRoll  int32  `json:"initiative_roll"`
	InitiativeOrder int32  `json:"initiative_order"`
	HpMax           int32  `json:"hp_max"`
	HpCurrent       int32  `json:"hp_current"`
	Ac              int32  `json:"ac"`
	IsNpc           bool   `json:"is_npc"`
	IsAlive         bool   `json:"is_alive"`
}

// turnInfoResponse is the JSON representation of turn info.
type turnInfoResponse struct {
	TurnID      string `json:"turn_id"`
	CombatantID string `json:"combatant_id"`
	RoundNumber int32  `json:"round_number"`
	Skipped     bool   `json:"skipped"`
}

// characterListResponse is the JSON representation of a character for combat selection.
type characterListResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Race    string `json:"race"`
	Level   int32  `json:"level"`
	HpMax   int32  `json:"hp_max"`
	Ac      int32  `json:"ac"`
	SpeedFt int32  `json:"speed_ft"`
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// StartCombat handles POST /api/combat/start.
func (h *Handler) StartCombat(w http.ResponseWriter, r *http.Request) {
	var req startCombatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	templateID, err := uuid.Parse(req.TemplateID)
	if err != nil {
		http.Error(w, "invalid template_id", http.StatusBadRequest)
		return
	}

	charIDs := make([]uuid.UUID, len(req.CharacterIDs))
	for i, s := range req.CharacterIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			http.Error(w, "invalid character_id: "+s, http.StatusBadRequest)
			return
		}
		charIDs[i] = id
	}

	positions := make(map[uuid.UUID]Position, len(req.CharacterPositions))
	for k, v := range req.CharacterPositions {
		id, err := uuid.Parse(k)
		if err != nil {
			http.Error(w, "invalid character position key: "+k, http.StatusBadRequest)
			return
		}
		positions[id] = v
	}

	input := StartCombatInput{
		TemplateID:         templateID,
		CharacterIDs:       charIDs,
		CharacterPositions: positions,
		SurprisedShortIDs:  req.SurprisedCombatantShortIDs,
	}

	result, err := h.svc.StartCombat(r.Context(), input, h.roller)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := startCombatResponse{
		Encounter: encounterResponse{
			ID:          result.Encounter.ID.String(),
			CampaignID:  result.Encounter.CampaignID.String(),
			Name:        result.Encounter.Name,
			Status:      result.Encounter.Status,
			RoundNumber: result.Encounter.RoundNumber,
		},
		InitiativeTracker: result.InitiativeTracker,
		FirstTurn: turnInfoResponse{
			TurnID:      result.FirstTurn.Turn.ID.String(),
			CombatantID: result.FirstTurn.CombatantID.String(),
			RoundNumber: result.FirstTurn.RoundNumber,
			Skipped:     result.FirstTurn.Skipped,
		},
	}

	combatants := make([]combatantResponse, len(result.Combatants))
	for i, c := range result.Combatants {
		combatants[i] = combatantResponse{
			ID:              c.ID.String(),
			ShortID:         c.ShortID,
			DisplayName:     c.DisplayName,
			InitiativeRoll:  c.InitiativeRoll,
			InitiativeOrder: c.InitiativeOrder,
			HpMax:           c.HpMax,
			HpCurrent:       c.HpCurrent,
			Ac:              c.Ac,
			IsNpc:           c.IsNpc,
			IsAlive:         c.IsAlive,
		}
	}
	resp.Combatants = combatants

	writeJSON(w, http.StatusOK, resp)
}

// ListCharacters handles GET /api/combat/characters?campaign_id=X.
func (h *Handler) ListCharacters(w http.ResponseWriter, r *http.Request) {
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

	chars, err := h.svc.ListCharactersByCampaign(r.Context(), campaignID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := make([]characterListResponse, len(chars))
	for i, c := range chars {
		resp[i] = characterListResponse{
			ID:      c.ID.String(),
			Name:    c.Name,
			Race:    c.Race,
			Level:   c.Level,
			HpMax:   c.HpMax,
			Ac:      c.Ac,
			SpeedFt: c.SpeedFt,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
