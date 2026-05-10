package combat

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// CombatLogPoster posts DM correction messages to the campaign's #combat-log Discord channel.
// Implementations should NEVER edit/delete prior messages — corrections are appended.
type CombatLogPoster interface {
	PostCorrection(ctx context.Context, encounterID uuid.UUID, message string)
}

// DMDashboardHandler serves DM Dashboard API endpoints for turn queue and action resolution.
//
// db (TxBeginner) is optional; when non-nil, mutating endpoints acquire a per-turn
// advisory lock via AcquireTurnLock before applying changes. Unit tests pass nil
// to skip the lock; integration tests provide a real *sql.DB.
//
// poster (CombatLogPoster) is optional; when non-nil, manual corrections are posted
// to the campaign's #combat-log Discord channel.
type DMDashboardHandler struct {
	svc    *Service
	db     TxBeginner
	poster CombatLogPoster
}

// NewDMDashboardHandler creates a DMDashboardHandler with no DB locking and no Discord poster.
func NewDMDashboardHandler(svc *Service) *DMDashboardHandler {
	return &DMDashboardHandler{svc: svc}
}

// NewDMDashboardHandlerWithDeps creates a DMDashboardHandler with optional DB locking
// and a Discord combat-log poster for DM corrections.
func NewDMDashboardHandlerWithDeps(svc *Service, db TxBeginner, poster CombatLogPoster) *DMDashboardHandler {
	return &DMDashboardHandler{svc: svc, db: db, poster: poster}
}

// RegisterRoutes mounts DM dashboard API routes on the given Chi router.
func (h *DMDashboardHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/combat", func(r chi.Router) {
		r.Post("/{encounterID}/advance-turn", h.AdvanceTurn)
		r.Get("/{encounterID}/pending-actions", h.ListPendingActions)
		r.Post("/{encounterID}/pending-actions/{actionID}/resolve", h.ResolvePendingAction)
		r.Get("/{encounterID}/action-log", h.ListActionLogViewer)
		r.Post("/{encounterID}/undo-last-action", h.UndoLastAction)
		r.Post("/{encounterID}/override/combatant/{combatantID}/hp", h.OverrideCombatantHP)
		r.Post("/{encounterID}/override/combatant/{combatantID}/position", h.OverrideCombatantPosition)
		r.Post("/{encounterID}/override/combatant/{combatantID}/conditions", h.OverrideCombatantConditions)
		r.Post("/{encounterID}/override/combatant/{combatantID}/initiative", h.OverrideCombatantInitiative)
		r.Post("/{encounterID}/override/character/{characterID}/spell-slots", h.OverrideCharacterSpellSlots)

		// Phase 118: voluntary concentration drop.
		r.Post("/{encounterID}/combatants/{combatantID}/concentration/drop", h.DropConcentration)
	})
}

// dropConcentrationResponse is the JSON payload returned by DropConcentration.
type dropConcentrationResponse struct {
	Broken              bool   `json:"broken"`
	SpellName           string `json:"spell_name"`
	ConditionsRemoved   int    `json:"conditions_removed"`
	SummonsDismissed    int    `json:"summons_dismissed"`
	ConsolidatedMessage string `json:"consolidated_message"`
}

// DropConcentration handles
// POST /api/combat/{encounterID}/combatants/{combatantID}/concentration/drop.
// Voluntary concentration drop initiated from the DM dashboard. Returns 409
// when the combatant is not currently concentrating.
func (h *DMDashboardHandler) DropConcentration(w http.ResponseWriter, r *http.Request) {
	encounterID, combatantID, err := parseCombatantOverrideIDs(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	combatant, err := h.svc.store.GetCombatant(r.Context(), combatantID)
	if err != nil {
		http.Error(w, "combatant not found", http.StatusNotFound)
		return
	}
	row, err := h.svc.store.GetCombatantConcentration(r.Context(), combatantID)
	if err != nil {
		http.Error(w, "lookup concentration: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !row.ConcentrationSpellID.Valid || !row.ConcentrationSpellName.Valid {
		http.Error(w, "combatant is not concentrating", http.StatusConflict)
		return
	}

	cleanup, err := h.svc.BreakConcentrationFully(r.Context(), BreakConcentrationFullyInput{
		EncounterID: encounterID,
		CasterID:    combatantID,
		CasterName:  combatant.DisplayName,
		SpellID:     row.ConcentrationSpellID.String,
		SpellName:   row.ConcentrationSpellName.String,
		Reason:      "voluntary drop",
	})
	if err != nil {
		http.Error(w, "break concentration: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if h.poster != nil {
		h.poster.PostCorrection(r.Context(), encounterID, cleanup.ConsolidatedMessage)
	}

	writeJSON(w, http.StatusOK, dropConcentrationResponse{
		Broken:              cleanup.Broken,
		SpellName:           cleanup.SpellName,
		ConditionsRemoved:   cleanup.ConditionsRemoved,
		SummonsDismissed:    cleanup.SummonsDismissed,
		ConsolidatedMessage: cleanup.ConsolidatedMessage,
	})
}

// parseEncounterID extracts and validates the encounterID URL parameter.
func parseEncounterID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "encounterID"))
}

// AdvanceTurn handles POST /api/combat/{encounterID}/advance-turn.
func (h *DMDashboardHandler) AdvanceTurn(w http.ResponseWriter, r *http.Request) {
	encounterID, err := parseEncounterID(r)
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	info, err := h.svc.AdvanceTurn(r.Context(), encounterID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, turnInfoResponse{
		TurnID:      info.Turn.ID.String(),
		CombatantID: info.CombatantID.String(),
		RoundNumber: info.RoundNumber,
		Skipped:     info.Skipped,
	})
}

// --- Pending Actions types ---

type pendingActionResponse struct {
	ID          string `json:"id"`
	EncounterID string `json:"encounter_id"`
	CombatantID string `json:"combatant_id"`
	ActionText  string `json:"action_text"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

type resolvePendingActionRequest struct {
	Outcome string          `json:"outcome"`
	Effects json.RawMessage `json:"effects"`
}

type resolveEffect struct {
	Type     string          `json:"type"`
	TargetID string          `json:"target_id"`
	Value    json.RawMessage `json:"value"`
}

type resolvePendingActionResponse struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Outcome     string `json:"outcome"`
}

// ListPendingActions handles GET /api/combat/{encounterID}/pending-actions.
func (h *DMDashboardHandler) ListPendingActions(w http.ResponseWriter, r *http.Request) {
	encounterID, err := parseEncounterID(r)
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	actions, err := h.svc.store.ListPendingActionsByEncounterID(r.Context(), encounterID)
	if err != nil {
		http.Error(w, "failed to list pending actions", http.StatusInternalServerError)
		return
	}

	resp := make([]pendingActionResponse, len(actions))
	for i, a := range actions {
		resp[i] = pendingActionResponse{
			ID:          a.ID.String(),
			EncounterID: a.EncounterID.String(),
			CombatantID: a.CombatantID.String(),
			ActionText:  a.ActionText,
			Status:      a.Status,
			CreatedAt:   a.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// ResolvePendingAction handles POST /api/combat/{encounterID}/pending-actions/{actionID}/resolve.
func (h *DMDashboardHandler) ResolvePendingAction(w http.ResponseWriter, r *http.Request) {
	encounterID, err := parseEncounterID(r)
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	actionID, err := uuid.Parse(chi.URLParam(r, "actionID"))
	if err != nil {
		http.Error(w, "invalid action ID", http.StatusBadRequest)
		return
	}

	var req resolvePendingActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	// Verify action exists and belongs to this encounter
	action, err := h.svc.store.GetPendingAction(r.Context(), actionID)
	if err != nil {
		http.Error(w, "action not found", http.StatusNotFound)
		return
	}
	if action.EncounterID != encounterID {
		http.Error(w, "action does not belong to this encounter", http.StatusBadRequest)
		return
	}
	if action.Status != "pending" {
		http.Error(w, "action is not pending", http.StatusConflict)
		return
	}

	// Apply effects
	if len(req.Effects) > 0 {
		var effects []resolveEffect
		if err := json.Unmarshal(req.Effects, &effects); err != nil {
			http.Error(w, "invalid effects format", http.StatusBadRequest)
			return
		}

		for _, eff := range effects {
			if err := h.applyEffect(r, eff); err != nil {
				http.Error(w, "failed to apply effect: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	// Mark action as resolved
	resolved, err := h.svc.store.UpdatePendingActionStatus(r.Context(), refdata.UpdatePendingActionStatusParams{
		ID:     actionID,
		Status: "resolved",
	})
	if err != nil {
		http.Error(w, "failed to resolve action", http.StatusInternalServerError)
		return
	}

	// Log to action_log
	turn, turnErr := h.svc.store.GetActiveTurnByEncounterID(r.Context(), encounterID)
	if turnErr == nil {
		h.svc.store.CreateActionLog(r.Context(), refdata.CreateActionLogParams{
			TurnID:      turn.ID,
			EncounterID: encounterID,
			ActionType:  "resolve_pending_action",
			ActorID:     action.CombatantID,
			Description: nullString(req.Outcome),
		})
	}

	writeJSON(w, http.StatusOK, resolvePendingActionResponse{
		ID:      resolved.ID.String(),
		Status:  resolved.Status,
		Outcome: req.Outcome,
	})
}

func (h *DMDashboardHandler) applyEffect(r *http.Request, eff resolveEffect) error {
	targetID, err := uuid.Parse(eff.TargetID)
	if err != nil {
		return err
	}

	switch eff.Type {
	case "damage":
		return h.applyDamageEffect(r, targetID, eff.Value)
	case "condition_add":
		return h.applyConditionEffect(r, targetID, eff.Value, addConditionAdapter)
	case "condition_remove":
		return h.applyConditionEffect(r, targetID, eff.Value, removeConditionAdapter)
	case "move":
		return h.applyMoveEffect(r, targetID, eff.Value)
	default:
		return nil
	}
}

func (h *DMDashboardHandler) applyDamageEffect(r *http.Request, targetID uuid.UUID, value json.RawMessage) error {
	var dmg struct {
		Amount int32 `json:"amount"`
	}
	if err := json.Unmarshal(value, &dmg); err != nil {
		return err
	}

	c, err := h.svc.store.GetCombatant(r.Context(), targetID)
	if err != nil {
		return err
	}

	// Route through ApplyDamage so Phase 42 (R/I/V, temp HP, exhaustion
	// HP-halving / level-6 death) and Phase 118 hooks (concentration save,
	// unconscious-at-0) both fire. The DM-dashboard pending-action effect
	// has no damage-type field, so an untyped amount is passed; R/I/V
	// resolution will treat it as untyped (no resistance/immunity matches).
	_, err = h.svc.ApplyDamage(r.Context(), ApplyDamageInput{
		EncounterID: c.EncounterID,
		Target:      c,
		RawDamage:   int(dmg.Amount),
	})
	return err
}

// conditionModifier transforms existing conditions given a condition name.
type conditionModifier func(json.RawMessage, string) (json.RawMessage, error)

// addConditionAdapter adapts AddCondition to the conditionModifier signature.
func addConditionAdapter(conditions json.RawMessage, name string) (json.RawMessage, error) {
	return AddCondition(conditions, CombatCondition{Condition: name})
}

// removeConditionAdapter adapts RemoveCondition to the conditionModifier signature.
func removeConditionAdapter(conditions json.RawMessage, name string) (json.RawMessage, error) {
	return RemoveCondition(conditions, name)
}

func (h *DMDashboardHandler) applyConditionEffect(r *http.Request, targetID uuid.UUID, value json.RawMessage, modify conditionModifier) error {
	var cond struct {
		Condition string `json:"condition"`
	}
	if err := json.Unmarshal(value, &cond); err != nil {
		return err
	}

	c, err := h.svc.store.GetCombatant(r.Context(), targetID)
	if err != nil {
		return err
	}

	newConds, err := modify(c.Conditions, cond.Condition)
	if err != nil {
		return err
	}

	_, err = h.svc.store.UpdateCombatantConditions(r.Context(), refdata.UpdateCombatantConditionsParams{
		ID:              targetID,
		Conditions:      newConds,
		ExhaustionLevel: c.ExhaustionLevel,
	})
	return err
}

func (h *DMDashboardHandler) applyMoveEffect(r *http.Request, targetID uuid.UUID, value json.RawMessage) error {
	var pos struct {
		Col string `json:"col"`
		Row int32  `json:"row"`
	}
	if err := json.Unmarshal(value, &pos); err != nil {
		return err
	}

	c, err := h.svc.store.GetCombatant(r.Context(), targetID)
	if err != nil {
		return err
	}

	_, err = h.svc.store.UpdateCombatantPosition(r.Context(), refdata.UpdateCombatantPositionParams{
		ID:          targetID,
		PositionCol: pos.Col,
		PositionRow: pos.Row,
		AltitudeFt:  c.AltitudeFt,
	})
	return err
}
