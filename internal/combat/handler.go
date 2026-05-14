package combat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// Handler serves combat API endpoints over HTTP.
type Handler struct {
	svc                *Service
	roller             *dice.Roller
	enemyTurnNotifier  EnemyTurnNotifier
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
		r.Post("/{encounterID}/end", h.EndCombat)
		r.Get("/{encounterID}/hostiles-defeated", h.CheckHostilesDefeated)
		r.Patch("/{encounterID}/display-name", h.UpdateEncounterDisplayName)

		// Reaction declarations
		r.Post("/{encounterID}/reactions", h.DeclareReaction)
		r.Get("/{encounterID}/reactions", h.ListReactions)
		r.Get("/{encounterID}/reactions/panel", h.ListReactionsPanel)
		r.Post("/{encounterID}/reactions/{reactionID}/resolve", h.ResolveReaction)
		r.Post("/{encounterID}/reactions/{reactionID}/cancel", h.CancelReaction)
		r.Post("/{encounterID}/combatants/{combatantID}/reactions/cancel-all", h.CancelAllReactions)

		// Readied actions
		r.Post("/{encounterID}/combatants/{combatantID}/ready", h.ReadyAction)
		r.Get("/{encounterID}/combatants/{combatantID}/readied-actions", h.ListReadiedActions)

		// Enemy turn builder
		r.Get("/{encounterID}/enemy-turn/{combatantID}/plan", h.GetEnemyTurnPlan)
		r.Post("/{encounterID}/enemy-turn", h.ExecuteEnemyTurn)

		// Summoned creatures
		r.Post("/{encounterID}/command", h.CommandCreatureHandler)
		r.Post("/{encounterID}/summon", h.SummonCreatureHandler)

		// Legendary actions, lair actions, turn queue
		h.RegisterLegendaryRoutes(r)

		// Counterspell
		r.Post("/{encounterID}/reactions/{reactionID}/counterspell/trigger", h.TriggerCounterspell)
		r.Post("/{encounterID}/reactions/{reactionID}/counterspell/resolve", h.ResolveCounterspell)
		r.Post("/{encounterID}/reactions/{reactionID}/counterspell/check", h.ResolveCounterspellCheck)
		r.Post("/{encounterID}/reactions/{reactionID}/counterspell/pass", h.PassCounterspell)
		r.Post("/{encounterID}/reactions/{reactionID}/counterspell/forfeit", h.ForfeitCounterspell)
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
	DisplayName string `json:"display_name"`
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

// endCombatResponse is the JSON response for ending combat.
type endCombatResponse struct {
	Encounter         encounterResponse   `json:"encounter"`
	Combatants        []combatantResponse `json:"combatants"`
	Summary           string              `json:"summary"`
	Casualties        int                 `json:"casualties"`
	RoundsElapsed     int32               `json:"rounds_elapsed"`
	InitiativeTracker string              `json:"initiative_tracker"`
}

// hostilesDefeatedResponse is the JSON response for the hostiles-defeated check.
type hostilesDefeatedResponse struct {
	AllDefeated bool `json:"all_defeated"`
}

// reactionDeclarationResponse is the JSON representation of a reaction declaration.
type reactionDeclarationResponse struct {
	ID          string `json:"id"`
	EncounterID string `json:"encounter_id"`
	CombatantID string `json:"combatant_id"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

// declareReactionRequest is the JSON request body for declaring a reaction.
type declareReactionRequest struct {
	CombatantID string `json:"combatant_id"`
	Description string `json:"description"`
}

// cancelAllReactionsResponse is the JSON response for cancel-all.
type cancelAllReactionsResponse struct {
	Status string `json:"status"`
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// toEncounterResponse converts a refdata.Encounter to its JSON response representation.
func toEncounterResponse(enc refdata.Encounter) encounterResponse {
	return encounterResponse{
		ID:          enc.ID.String(),
		CampaignID:  enc.CampaignID.String(),
		Name:        enc.Name,
		DisplayName: EncounterDisplayName(enc),
		Status:      enc.Status,
		RoundNumber: enc.RoundNumber,
	}
}

// toCombatantResponses converts a slice of refdata.Combatant to their JSON response representations.
func toCombatantResponses(combatants []refdata.Combatant) []combatantResponse {
	resp := make([]combatantResponse, len(combatants))
	for i, c := range combatants {
		resp[i] = combatantResponse{
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
	return resp
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

	writeJSON(w, http.StatusOK, startCombatResponse{
		Encounter:         toEncounterResponse(result.Encounter),
		Combatants:        toCombatantResponses(result.Combatants),
		InitiativeTracker: result.InitiativeTracker,
		FirstTurn: turnInfoResponse{
			TurnID:      result.FirstTurn.Turn.ID.String(),
			CombatantID: result.FirstTurn.CombatantID.String(),
			RoundNumber: result.FirstTurn.RoundNumber,
			Skipped:     result.FirstTurn.Skipped,
		},
	})
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

// updateDisplayNameRequest is the JSON body for PATCH /api/combat/{encounterID}/display-name.
type updateDisplayNameRequest struct {
	DisplayName string `json:"display_name"`
}

// UpdateEncounterDisplayName handles PATCH /api/combat/{encounterID}/display-name.
// Phase 105: lets the DM set the player-facing encounter name at any time
// during combat. An empty string clears the override so the internal name is
// used in Discord messages.
func (h *Handler) UpdateEncounterDisplayName(w http.ResponseWriter, r *http.Request) {
	encounterIDStr := chi.URLParam(r, "encounterID")
	encounterID, err := uuid.Parse(encounterIDStr)
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	var req updateDisplayNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	enc, err := h.svc.UpdateEncounterDisplayName(r.Context(), encounterID, req.DisplayName)
	if err != nil {
		http.Error(w, "failed to update display name", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, toEncounterResponse(enc))
}

// EndCombat handles POST /api/combat/{encounterID}/end.
func (h *Handler) EndCombat(w http.ResponseWriter, r *http.Request) {
	encounterIDStr := chi.URLParam(r, "encounterID")
	encounterID, err := uuid.Parse(encounterIDStr)
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	result, err := h.svc.EndCombat(r.Context(), encounterID)
	if err != nil {
		if errors.Is(err, ErrEncounterNotActive) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, endCombatResponse{
		Encounter:         toEncounterResponse(result.Encounter),
		Combatants:        toCombatantResponses(result.Combatants),
		Summary:           result.Summary,
		Casualties:        result.Casualties,
		RoundsElapsed:     result.RoundsElapsed,
		InitiativeTracker: result.InitiativeTracker,
	})
}

// CheckHostilesDefeated handles GET /api/combat/{encounterID}/hostiles-defeated.
func (h *Handler) CheckHostilesDefeated(w http.ResponseWriter, r *http.Request) {
	encounterIDStr := chi.URLParam(r, "encounterID")
	encounterID, err := uuid.Parse(encounterIDStr)
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	defeated, err := h.svc.AllHostilesDefeated(r.Context(), encounterID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, hostilesDefeatedResponse{AllDefeated: defeated})
}

// toReactionDeclarationResponse converts a refdata.ReactionDeclaration to its JSON response.
func toReactionDeclarationResponse(d refdata.ReactionDeclaration) reactionDeclarationResponse {
	return reactionDeclarationResponse{
		ID:          d.ID.String(),
		EncounterID: d.EncounterID.String(),
		CombatantID: d.CombatantID.String(),
		Description: d.Description,
		Status:      d.Status,
	}
}

// toReactionDeclarationResponses converts a slice of declarations to their JSON responses.
func toReactionDeclarationResponses(decls []refdata.ReactionDeclaration) []reactionDeclarationResponse {
	resp := make([]reactionDeclarationResponse, len(decls))
	for i, d := range decls {
		resp[i] = toReactionDeclarationResponse(d)
	}
	return resp
}

// DeclareReaction handles POST /api/combat/{encounterID}/reactions.
func (h *Handler) DeclareReaction(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	var req declareReactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	combatantID, err := uuid.Parse(req.CombatantID)
	if err != nil {
		http.Error(w, "invalid combatant_id", http.StatusBadRequest)
		return
	}

	decl, err := h.svc.DeclareReaction(r.Context(), encounterID, combatantID, req.Description)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, toReactionDeclarationResponse(decl))
}

// ListReactions handles GET /api/combat/{encounterID}/reactions.
func (h *Handler) ListReactions(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	decls, err := h.svc.ListActiveReactions(r.Context(), encounterID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, toReactionDeclarationResponses(decls))
}

// ListReactionsPanel handles GET /api/combat/{encounterID}/reactions/panel.
// Returns enriched reaction data for the DM dashboard active reactions panel.
func (h *Handler) ListReactionsPanel(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	entries, err := h.svc.ListReactionsForPanel(r.Context(), encounterID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, entries)
}

// ResolveReaction handles POST /api/combat/{encounterID}/reactions/{reactionID}/resolve.
func (h *Handler) ResolveReaction(w http.ResponseWriter, r *http.Request) {
	_, reactionID, err := parseReactionRouteParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	decl, err := h.svc.ResolveReaction(r.Context(), reactionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, toReactionDeclarationResponse(decl))
}

// CancelReaction handles POST /api/combat/{encounterID}/reactions/{reactionID}/cancel.
func (h *Handler) CancelReaction(w http.ResponseWriter, r *http.Request) {
	_, reactionID, err := parseReactionRouteParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	decl, err := h.svc.CancelReaction(r.Context(), reactionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, toReactionDeclarationResponse(decl))
}

// CancelAllReactions handles POST /api/combat/{encounterID}/combatants/{combatantID}/reactions/cancel-all.
func (h *Handler) CancelAllReactions(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	combatantID, err := uuid.Parse(chi.URLParam(r, "combatantID"))
	if err != nil {
		http.Error(w, "invalid combatant ID", http.StatusBadRequest)
		return
	}

	if err := h.svc.CancelAllReactions(r.Context(), combatantID, encounterID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, cancelAllReactionsResponse{Status: "ok"})
}

// readyActionRequest is the JSON request body for readying an action.
type readyActionRequest struct {
	Description    string `json:"description"`
	SpellName      string `json:"spell_name,omitempty"`
	SpellSlotLevel int    `json:"spell_slot_level,omitempty"`
}

// readyActionResponse is the JSON response for readying an action.
type readyActionResponse struct {
	Declaration reactionDeclarationResponse `json:"declaration"`
	CombatLog   string                      `json:"combat_log"`
}

// ReadyAction handles POST /api/combat/{encounterID}/combatants/{combatantID}/ready.
func (h *Handler) ReadyAction(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	combatantID, err := uuid.Parse(chi.URLParam(r, "combatantID"))
	if err != nil {
		http.Error(w, "invalid combatant ID", http.StatusBadRequest)
		return
	}

	var req readyActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	combatant, err := h.svc.GetCombatant(r.Context(), combatantID)
	if err != nil {
		http.Error(w, "combatant not found", http.StatusNotFound)
		return
	}

	turn, err := h.svc.store.GetActiveTurnByEncounterID(r.Context(), encounterID)
	if err != nil {
		http.Error(w, "no active turn", http.StatusBadRequest)
		return
	}
	if turn.CombatantID != combatantID {
		http.Error(w, "not this combatant's turn", http.StatusBadRequest)
		return
	}

	result, err := h.svc.ReadyAction(r.Context(), ReadyActionCommand{
		Combatant:      combatant,
		Turn:           turn,
		Description:    req.Description,
		SpellName:      req.SpellName,
		SpellSlotLevel: req.SpellSlotLevel,
	})
	if err != nil {
		if errors.Is(err, ErrResourceSpent) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, readyActionResponse{
		Declaration: toReactionDeclarationResponse(result.Declaration),
		CombatLog:   result.CombatLog,
	})
}

// ListReadiedActions handles GET /api/combat/{encounterID}/combatants/{combatantID}/readied-actions.
func (h *Handler) ListReadiedActions(w http.ResponseWriter, r *http.Request) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	combatantID, err := uuid.Parse(chi.URLParam(r, "combatantID"))
	if err != nil {
		http.Error(w, "invalid combatant ID", http.StatusBadRequest)
		return
	}

	readied, err := h.svc.ListReadiedActions(r.Context(), combatantID, encounterID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, toReactionDeclarationResponses(readied))
}

// parseReactionRouteParams extracts and validates encounterID and reactionID from URL params.
func parseReactionRouteParams(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	encounterID, err := uuid.Parse(chi.URLParam(r, "encounterID"))
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid encounter ID")
	}
	reactionID, err := uuid.Parse(chi.URLParam(r, "reactionID"))
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid reaction ID")
	}
	return encounterID, reactionID, nil
}

// parseUUIDOrNil parses a UUID string, returning uuid.Nil on empty or invalid input.
func parseUUIDOrNil(s string) uuid.UUID {
	if s == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// --- Counterspell handler types ---

type triggerCounterspellRequest struct {
	EnemySpellName string `json:"enemy_spell_name"`
	EnemyCastLevel int    `json:"enemy_cast_level"`
	EnemyCasterID  string `json:"enemy_caster_id,omitempty"` // SR-046: combatant ID of the caster being counterspelled
	IsSubtle       bool   `json:"is_subtle,omitempty"`       // med-29 / Phase 72
}

type counterspellPromptResponse struct {
	DeclarationID  string `json:"declaration_id"`
	CasterName     string `json:"caster_name"`
	EnemySpellName string `json:"enemy_spell_name"`
	AvailableSlots []int  `json:"available_slots"`
}

type resolveCounterspellRequest struct {
	SlotLevel int `json:"slot_level"`
}

type resolveCounterspellCheckRequest struct {
	CheckTotal int `json:"check_total"`
}

type counterspellResultResponse struct {
	Outcome        string `json:"outcome"`
	CasterName     string `json:"caster_name"`
	EnemySpellName string `json:"enemy_spell_name"`
	EnemyCastLevel int    `json:"enemy_cast_level,omitempty"`
	SlotUsed       int    `json:"slot_used,omitempty"`
	DC             int    `json:"dc,omitempty"`
	CombatLog      string `json:"combat_log"`
}

func toCounterspellResultResponse(r CounterspellResult) counterspellResultResponse {
	return counterspellResultResponse{
		Outcome:        string(r.Outcome),
		CasterName:     r.CasterName,
		EnemySpellName: r.EnemySpellName,
		EnemyCastLevel: r.EnemyCastLevel,
		SlotUsed:       r.SlotUsed,
		DC:             r.DC,
		CombatLog:      FormatCounterspellLog(r),
	}
}

// TriggerCounterspell handles POST /api/combat/{encounterID}/reactions/{reactionID}/counterspell/trigger.
func (h *Handler) TriggerCounterspell(w http.ResponseWriter, r *http.Request) {
	_, reactionID, err := parseReactionRouteParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req triggerCounterspellRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	prompt, err := h.svc.TriggerCounterspell(r.Context(), reactionID, req.EnemySpellName, req.EnemyCastLevel, req.IsSubtle, parseUUIDOrNil(req.EnemyCasterID))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, counterspellPromptResponse{
		DeclarationID:  prompt.DeclarationID.String(),
		CasterName:     prompt.CasterName,
		EnemySpellName: prompt.EnemySpellName,
		AvailableSlots: prompt.AvailableSlots,
	})
}

// ResolveCounterspell handles POST /api/combat/{encounterID}/reactions/{reactionID}/counterspell/resolve.
func (h *Handler) ResolveCounterspell(w http.ResponseWriter, r *http.Request) {
	_, reactionID, err := parseReactionRouteParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req resolveCounterspellRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	result, err := h.svc.ResolveCounterspell(r.Context(), reactionID, req.SlotLevel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, toCounterspellResultResponse(result))
}

// ResolveCounterspellCheck handles POST /api/combat/{encounterID}/reactions/{reactionID}/counterspell/check.
func (h *Handler) ResolveCounterspellCheck(w http.ResponseWriter, r *http.Request) {
	_, reactionID, err := parseReactionRouteParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req resolveCounterspellCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	result, err := h.svc.ResolveCounterspellCheck(r.Context(), reactionID, req.CheckTotal)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, toCounterspellResultResponse(result))
}

// PassCounterspell handles POST /api/combat/{encounterID}/reactions/{reactionID}/counterspell/pass.
func (h *Handler) PassCounterspell(w http.ResponseWriter, r *http.Request) {
	_, reactionID, err := parseReactionRouteParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.svc.PassCounterspell(r.Context(), reactionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, toCounterspellResultResponse(result))
}

// ForfeitCounterspell handles POST /api/combat/{encounterID}/reactions/{reactionID}/counterspell/forfeit.
func (h *Handler) ForfeitCounterspell(w http.ResponseWriter, r *http.Request) {
	_, reactionID, err := parseReactionRouteParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.svc.ForfeitCounterspell(r.Context(), reactionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, toCounterspellResultResponse(result))
}
