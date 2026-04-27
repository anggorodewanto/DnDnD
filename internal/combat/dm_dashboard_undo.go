package combat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// undoLastActionRequest is the JSON body for POST .../undo-last-action.
type undoLastActionRequest struct {
	Reason string `json:"reason"`
}

// errUnknownActionType is returned when undo cannot dispatch on the action type.
var errUnknownActionType = errors.New("unknown action_type for undo")

// withTurnLock acquires the per-turn advisory lock when h.db is set, then runs fn.
// The lock is released when fn returns. When h.db is nil (unit test path), fn runs unlocked.
func (h *DMDashboardHandler) withTurnLock(ctx context.Context, turnID uuid.UUID, fn func() error) error {
	if h.db == nil {
		return fn()
	}
	tx, err := AcquireTurnLock(ctx, h.db, turnID)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(); err != nil {
		return err
	}
	return tx.Commit()
}

// postCorrection sends a DM correction message via the configured combat-log poster.
// Best-effort: when no poster is configured this is a no-op.
func (h *DMDashboardHandler) postCorrection(ctx context.Context, encounterID uuid.UUID, message string) {
	if h.poster == nil {
		return
	}
	h.poster.PostCorrection(ctx, encounterID, message)
}

// correctionMsg formats a DM correction message with an optional parenthesized reason.
func correctionMsg(base, reason string) string {
	if reason == "" {
		return base
	}
	return base + " (" + reason + ")"
}

// UndoLastAction handles POST /api/combat/{encounterID}/undo-last-action.
// Reverts the most recent non-undo mutation in the current turn from its before_state.
func (h *DMDashboardHandler) UndoLastAction(w http.ResponseWriter, r *http.Request) {
	encounterID, err := parseEncounterID(r)
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}

	var req undoLastActionRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // body is optional

	turn, err := h.svc.store.GetActiveTurnByEncounterID(r.Context(), encounterID)
	if err != nil {
		http.Error(w, "no active turn", http.StatusNotFound)
		return
	}

	logs, err := h.svc.store.ListActionLogByTurnID(r.Context(), turn.ID)
	if err != nil {
		http.Error(w, "failed to list action log", http.StatusInternalServerError)
		return
	}

	target := mostRecentUndoable(logs)
	if target == nil {
		http.Error(w, "nothing to undo for current turn", http.StatusNotFound)
		return
	}

	if err := h.withTurnLock(r.Context(), turn.ID, func() error {
		return h.applyUndo(r.Context(), encounterID, turn.ID, *target, req.Reason)
	}); err != nil {
		if errors.Is(err, errUnknownActionType) {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "undone", "action_type": target.ActionType})
}

// mostRecentUndoable returns a pointer to the latest non-undo log entry, or nil.
// logs are assumed sorted ascending by created_at.
func mostRecentUndoable(logs []refdata.ActionLog) *refdata.ActionLog {
	for i := len(logs) - 1; i >= 0; i-- {
		if logs[i].ActionType == "dm_override_undo" {
			continue
		}
		return &logs[i]
	}
	return nil
}

// undoBeforeState describes the recognized fields in action_log.before_state.
type undoBeforeState struct {
	HpCurrent   *int32           `json:"hp_current"`
	TempHp      *int32           `json:"temp_hp"`
	IsAlive     *bool            `json:"is_alive"`
	PositionCol *string          `json:"position_col"`
	PositionRow *int32           `json:"position_row"`
	AltitudeFt  *int32           `json:"altitude_ft"`
	Conditions  json.RawMessage  `json:"conditions"`
}

// applyUndo restores the recognized fields from log.BeforeState and writes a dm_override_undo entry.
func (h *DMDashboardHandler) applyUndo(ctx context.Context, encounterID, turnID uuid.UUID, log refdata.ActionLog, reason string) error {
	var bs undoBeforeState
	if err := json.Unmarshal(log.BeforeState, &bs); err != nil {
		return fmt.Errorf("parsing before_state: %w", err)
	}

	if log.ActorID == uuid.Nil {
		return fmt.Errorf("undo target has no actor: %w", errUnknownActionType)
	}
	combatant, err := h.svc.store.GetCombatant(ctx, log.ActorID)
	if err != nil {
		return fmt.Errorf("getting actor: %w", err)
	}

	currentSnapshot, err := snapshotCombatantState(combatant)
	if err != nil {
		return err
	}

	dispatched, err := h.dispatchUndo(ctx, combatant, bs, log.ActionType)
	if err != nil {
		return err
	}
	if !dispatched {
		return errUnknownActionType
	}

	// Log the undo
	_, _ = h.svc.store.CreateActionLog(ctx, refdata.CreateActionLogParams{
		TurnID:      turnID,
		EncounterID: encounterID,
		ActionType:  "dm_override_undo",
		ActorID:     log.ActorID,
		Description: nullString(reason),
		BeforeState: currentSnapshot,
		AfterState:  log.BeforeState,
	})

	base := fmt.Sprintf("⚠️ **DM Correction:** %s — undo %s", combatant.DisplayName, log.ActionType)
	h.postCorrection(ctx, encounterID, correctionMsg(base, reason))
	return nil
}

// dispatchUndo applies the recognized field restorations. Returns whether any restoration was applied.
// actionType is currently unused but reserved for future per-type logic.
func (h *DMDashboardHandler) dispatchUndo(ctx context.Context, c refdata.Combatant, bs undoBeforeState, _ string) (bool, error) {
	applied := false

	if bs.HpCurrent != nil || bs.TempHp != nil || bs.IsAlive != nil {
		hpCurrent := c.HpCurrent
		if bs.HpCurrent != nil {
			hpCurrent = *bs.HpCurrent
		}
		tempHp := c.TempHp
		if bs.TempHp != nil {
			tempHp = *bs.TempHp
		}
		isAlive := c.IsAlive
		if bs.IsAlive != nil {
			isAlive = *bs.IsAlive
		}
		// Phase 118: an undo that decreases HP (e.g. undoing a heal) is a
		// damage event; route through applyDamageHP so concentration save
		// + unconscious hooks fire. Pure restore-to-higher (undoing a
		// damage) skips the hooks.
		if hpCurrent < c.HpCurrent {
			if _, err := h.svc.applyDamageHP(ctx, c.EncounterID, c.ID, c.HpCurrent, hpCurrent, tempHp, isAlive); err != nil {
				return false, fmt.Errorf("restoring HP: %w", err)
			}
		} else {
			if _, err := h.svc.store.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
				ID: c.ID, HpCurrent: hpCurrent, TempHp: tempHp, IsAlive: isAlive,
			}); err != nil {
				return false, fmt.Errorf("restoring HP: %w", err)
			}
		}
		applied = true
	}

	if bs.PositionCol != nil || bs.PositionRow != nil || bs.AltitudeFt != nil {
		col := c.PositionCol
		if bs.PositionCol != nil {
			col = *bs.PositionCol
		}
		row := c.PositionRow
		if bs.PositionRow != nil {
			row = *bs.PositionRow
		}
		alt := c.AltitudeFt
		if bs.AltitudeFt != nil {
			alt = *bs.AltitudeFt
		}
		if _, err := h.svc.store.UpdateCombatantPosition(ctx, refdata.UpdateCombatantPositionParams{
			ID: c.ID, PositionCol: col, PositionRow: row, AltitudeFt: alt,
		}); err != nil {
			return false, fmt.Errorf("restoring position: %w", err)
		}
		applied = true
	}

	if len(bs.Conditions) > 0 {
		if _, err := h.svc.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
			ID: c.ID, Conditions: bs.Conditions, ExhaustionLevel: c.ExhaustionLevel,
		}); err != nil {
			return false, fmt.Errorf("restoring conditions: %w", err)
		}
		applied = true
	}

	return applied, nil
}

// snapshotCombatantState marshals the relevant combatant fields into a before_state JSON.
func snapshotCombatantState(c refdata.Combatant) (json.RawMessage, error) {
	snapshot := struct {
		HpCurrent   int32           `json:"hp_current"`
		TempHp      int32           `json:"temp_hp"`
		IsAlive     bool            `json:"is_alive"`
		PositionCol string          `json:"position_col"`
		PositionRow int32           `json:"position_row"`
		AltitudeFt  int32           `json:"altitude_ft"`
		Conditions  json.RawMessage `json:"conditions"`
	}{
		HpCurrent:   c.HpCurrent,
		TempHp:      c.TempHp,
		IsAlive:     c.IsAlive,
		PositionCol: c.PositionCol,
		PositionRow: c.PositionRow,
		AltitudeFt:  c.AltitudeFt,
		Conditions:  c.Conditions,
	}
	b, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshalling snapshot: %w", err)
	}
	return b, nil
}

// --- Manual override endpoints ---

type overrideHPRequest struct {
	HpCurrent int32  `json:"hp_current"`
	TempHp    int32  `json:"temp_hp"`
	Reason    string `json:"reason"`
}

type overridePositionRequest struct {
	PositionCol string `json:"position_col"`
	PositionRow int32  `json:"position_row"`
	AltitudeFt  int32  `json:"altitude_ft"`
	Reason      string `json:"reason"`
}

type overrideConditionsRequest struct {
	Conditions json.RawMessage `json:"conditions"`
	Reason     string          `json:"reason"`
}

type overrideInitiativeRequest struct {
	InitiativeRoll  int32  `json:"initiative_roll"`
	InitiativeOrder int32  `json:"initiative_order"`
	Reason          string `json:"reason"`
}

type overrideSpellSlotsRequest struct {
	SpellSlots json.RawMessage `json:"spell_slots"`
	Reason     string          `json:"reason"`
}

// parseCombatantOverrideIDs returns the parsed encounter and combatant UUIDs.
func parseCombatantOverrideIDs(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	encounterID, err := parseEncounterID(r)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid encounter ID")
	}
	combatantID, err := uuid.Parse(chi.URLParam(r, "combatantID"))
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid combatant ID")
	}
	return encounterID, combatantID, nil
}

// runOverride wraps the common boilerplate for combatant overrides:
// parse IDs, decode body, look up active turn, acquire lock, call apply.
func (h *DMDashboardHandler) runOverride(
	w http.ResponseWriter, r *http.Request,
	decode func() error,
	apply func(ctx context.Context, encounterID, combatantID, turnID uuid.UUID) error,
) {
	encounterID, combatantID, err := parseCombatantOverrideIDs(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := decode(); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	turn, err := h.svc.store.GetActiveTurnByEncounterID(r.Context(), encounterID)
	if err != nil {
		http.Error(w, "no active turn", http.StatusNotFound)
		return
	}
	if err := h.withTurnLock(r.Context(), turn.ID, func() error {
		return apply(r.Context(), encounterID, combatantID, turn.ID)
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// logOverride writes a dm_override entry to the action log.
func (h *DMDashboardHandler) logOverride(ctx context.Context, turnID, encounterID, actorID uuid.UUID, reason string, before, after json.RawMessage) {
	_, _ = h.svc.store.CreateActionLog(ctx, refdata.CreateActionLogParams{
		TurnID:      turnID,
		EncounterID: encounterID,
		ActionType:  "dm_override",
		ActorID:     actorID,
		Description: nullString(reason),
		BeforeState: before,
		AfterState:  after,
	})
}

// OverrideCombatantHP handles POST /api/combat/{encounterID}/override/combatant/{combatantID}/hp.
func (h *DMDashboardHandler) OverrideCombatantHP(w http.ResponseWriter, r *http.Request) {
	var req overrideHPRequest
	h.runOverride(w, r,
		func() error { return json.NewDecoder(r.Body).Decode(&req) },
		func(ctx context.Context, encounterID, combatantID, turnID uuid.UUID) error {
			c, err := h.svc.store.GetCombatant(ctx, combatantID)
			if err != nil {
				return err
			}
			before, _ := snapshotCombatantState(c)
			isAlive := req.HpCurrent > 0
			// Phase 118: when the override decreases HP, route through
			// applyDamageHP so concentration save / unconscious hooks fire.
			// For non-decrease (heal / restore) use the raw store call.
			var updated refdata.Combatant
			if req.HpCurrent < c.HpCurrent {
				updated, err = h.svc.applyDamageHP(ctx, encounterID, combatantID, c.HpCurrent, req.HpCurrent, req.TempHp, isAlive)
			} else {
				updated, err = h.svc.store.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
					ID: combatantID, HpCurrent: req.HpCurrent, TempHp: req.TempHp, IsAlive: isAlive,
				})
			}
			if err != nil {
				return err
			}
			after, _ := snapshotCombatantState(updated)
			h.logOverride(ctx, turnID, encounterID, combatantID, req.Reason, before, after)

			base := fmt.Sprintf("⚠️ **DM Correction:** %s HP adjusted to %d", c.DisplayName, req.HpCurrent)
			h.postCorrection(ctx, encounterID, correctionMsg(base, req.Reason))
			return nil
		},
	)
}

// OverrideCombatantPosition handles POST .../override/combatant/{combatantID}/position.
//
// The position update is routed through Service.UpdateCombatantPosition (not
// the raw store) so the silence-zone concentration-break hook fires for
// DM-overridden moves, matching player-initiated moves.
func (h *DMDashboardHandler) OverrideCombatantPosition(w http.ResponseWriter, r *http.Request) {
	var req overridePositionRequest
	h.runOverride(w, r,
		func() error { return json.NewDecoder(r.Body).Decode(&req) },
		func(ctx context.Context, encounterID, combatantID, turnID uuid.UUID) error {
			c, err := h.svc.store.GetCombatant(ctx, combatantID)
			if err != nil {
				return err
			}
			before, _ := snapshotCombatantState(c)
			updated, err := h.svc.UpdateCombatantPosition(ctx, combatantID, req.PositionCol, req.PositionRow, req.AltitudeFt)
			if err != nil {
				return err
			}
			after, _ := snapshotCombatantState(updated)
			h.logOverride(ctx, turnID, encounterID, combatantID, req.Reason, before, after)

			base := fmt.Sprintf("⚠️ **DM Correction:** %s position adjusted to %s%d", c.DisplayName, req.PositionCol, req.PositionRow)
			h.postCorrection(ctx, encounterID, correctionMsg(base, req.Reason))
			return nil
		},
	)
}

// OverrideCombatantConditions handles POST .../override/combatant/{combatantID}/conditions.
func (h *DMDashboardHandler) OverrideCombatantConditions(w http.ResponseWriter, r *http.Request) {
	var req overrideConditionsRequest
	h.runOverride(w, r,
		func() error { return json.NewDecoder(r.Body).Decode(&req) },
		func(ctx context.Context, encounterID, combatantID, turnID uuid.UUID) error {
			c, err := h.svc.store.GetCombatant(ctx, combatantID)
			if err != nil {
				return err
			}
			before, _ := snapshotCombatantState(c)
			conditions := req.Conditions
			if len(conditions) == 0 {
				conditions = json.RawMessage(`[]`)
			}
			updated, err := h.svc.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
				ID: combatantID, Conditions: conditions, ExhaustionLevel: c.ExhaustionLevel,
			})
			if err != nil {
				return err
			}
			after, _ := snapshotCombatantState(updated)
			h.logOverride(ctx, turnID, encounterID, combatantID, req.Reason, before, after)

			base := fmt.Sprintf("⚠️ **DM Correction:** %s conditions adjusted", c.DisplayName)
			h.postCorrection(ctx, encounterID, correctionMsg(base, req.Reason))
			return nil
		},
	)
}

// OverrideCombatantInitiative handles POST .../override/combatant/{combatantID}/initiative.
func (h *DMDashboardHandler) OverrideCombatantInitiative(w http.ResponseWriter, r *http.Request) {
	var req overrideInitiativeRequest
	h.runOverride(w, r,
		func() error { return json.NewDecoder(r.Body).Decode(&req) },
		func(ctx context.Context, encounterID, combatantID, turnID uuid.UUID) error {
			c, err := h.svc.store.GetCombatant(ctx, combatantID)
			if err != nil {
				return err
			}
			before, _ := json.Marshal(map[string]int32{
				"initiative_roll":  c.InitiativeRoll,
				"initiative_order": c.InitiativeOrder,
			})
			if _, err := h.svc.store.UpdateCombatantInitiative(ctx, refdata.UpdateCombatantInitiativeParams{
				ID: combatantID, InitiativeRoll: req.InitiativeRoll, InitiativeOrder: req.InitiativeOrder,
			}); err != nil {
				return err
			}
			after, _ := json.Marshal(map[string]int32{
				"initiative_roll":  req.InitiativeRoll,
				"initiative_order": req.InitiativeOrder,
			})
			h.logOverride(ctx, turnID, encounterID, combatantID, req.Reason, before, after)

			base := fmt.Sprintf("⚠️ **DM Correction:** %s initiative adjusted to %d", c.DisplayName, req.InitiativeRoll)
			h.postCorrection(ctx, encounterID, correctionMsg(base, req.Reason))
			return nil
		},
	)
}

// OverrideCharacterSpellSlots handles POST .../override/character/{characterID}/spell-slots.
func (h *DMDashboardHandler) OverrideCharacterSpellSlots(w http.ResponseWriter, r *http.Request) {
	encounterID, err := parseEncounterID(r)
	if err != nil {
		http.Error(w, "invalid encounter ID", http.StatusBadRequest)
		return
	}
	characterID, err := uuid.Parse(chi.URLParam(r, "characterID"))
	if err != nil {
		http.Error(w, "invalid character ID", http.StatusBadRequest)
		return
	}

	var req overrideSpellSlotsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	turn, err := h.svc.store.GetActiveTurnByEncounterID(r.Context(), encounterID)
	if err != nil {
		http.Error(w, "no active turn", http.StatusNotFound)
		return
	}

	if err := h.withTurnLock(r.Context(), turn.ID, func() error {
		char, err := h.svc.store.GetCharacter(r.Context(), characterID)
		if err != nil {
			return fmt.Errorf("getting character: %w", err)
		}

		before := json.RawMessage(`null`)
		if char.SpellSlots.Valid {
			before = char.SpellSlots.RawMessage
		}

		newSlots := pqtype.NullRawMessage{RawMessage: req.SpellSlots, Valid: len(req.SpellSlots) > 0}
		if _, err := h.svc.store.UpdateCharacterSpellSlots(r.Context(), refdata.UpdateCharacterSpellSlotsParams{
			ID: characterID, SpellSlots: newSlots,
		}); err != nil {
			return fmt.Errorf("updating spell slots: %w", err)
		}

		// action_log.actor_id is NOT NULL FK to combatants(id); the spell-slot
		// override is a character-level mutation, so we attribute the audit row
		// to the active turn's combatant.
		_, _ = h.svc.store.CreateActionLog(r.Context(), refdata.CreateActionLogParams{
			TurnID:      turn.ID,
			EncounterID: encounterID,
			ActionType:  "dm_override",
			ActorID:     turn.CombatantID,
			Description: nullString(req.Reason),
			BeforeState: before,
			AfterState:  req.SpellSlots,
		})

		base := fmt.Sprintf("⚠️ **DM Correction:** %s spell slots adjusted", char.Name)
		h.postCorrection(r.Context(), encounterID, correctionMsg(base, req.Reason))
		return nil
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
