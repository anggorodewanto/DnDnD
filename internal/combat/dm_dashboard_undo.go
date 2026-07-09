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

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/httpjson"
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

// mostRecentUndoable returns a pointer to the latest log entry, or nil.
// logs are assumed sorted ascending by created_at.
// Undo entries are themselves undoable (undo-of-undo = redo).
func mostRecentUndoable(logs []refdata.ActionLog) *refdata.ActionLog {
	if len(logs) == 0 {
		return nil
	}
	return &logs[len(logs)-1]
}

// undoBeforeState describes the recognized fields in action_log.before_state.
type undoBeforeState struct {
	HpCurrent   *int32          `json:"hp_current"`
	TempHp      *int32          `json:"temp_hp"`
	IsAlive     *bool           `json:"is_alive"`
	PositionCol *string         `json:"position_col"`
	PositionRow *int32          `json:"position_row"`
	AltitudeFt  *int32          `json:"altitude_ft"`
	Conditions  json.RawMessage `json:"conditions"`
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
		// damage event; route through ApplyDamage (Override=true) so
		// concentration save + unconscious hooks fire while skipping
		// R/I/V and temp-HP absorption — undo asserts an exact prior HP
		// state. Pure restore-to-higher (undoing damage) skips the hooks.
		if hpCurrent < c.HpCurrent {
			if _, err := h.svc.ApplyDamage(ctx, ApplyDamageInput{
				EncounterID: c.EncounterID,
				Target:      c,
				RawDamage:   int(c.HpCurrent - hpCurrent),
				Override:    true,
			}); err != nil {
				return false, fmt.Errorf("restoring HP: %w", err)
			}
			// ApplyDamage wrote HP+isAlive but not the explicit tempHp the
			// undo wants to restore. Issue a follow-up store call so the
			// snapshot's tempHp / isAlive are honored exactly.
			if _, err := h.svc.store.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
				ID: c.ID, HpCurrent: hpCurrent, TempHp: tempHp, IsAlive: isAlive,
			}); err != nil {
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

// overrideInitiativeRequest is the JSON body for
// POST .../override/combatant/{combatantID}/initiative.
//
// InitiativeRoll and InitiativeOrder are pointers so an omitted field means
// "leave that value unchanged" (APP-3). Before this, both were plain int32, so
// omitting initiative_order silently wrote 0 — jumping that combatant to the
// front of the order.
type overrideInitiativeRequest struct {
	InitiativeRoll  *int32 `json:"initiative_roll"`
	InitiativeOrder *int32 `json:"initiative_order"`
	Reason          string `json:"reason"`
}

// errOverrideConflict marks an override request that conflicts with existing
// combat state (e.g. a duplicate initiative_order). runOverride maps it to a
// 409 instead of the default 500.
var errOverrideConflict = errors.New("override conflict")

// overrideSlotsRequest is the JSON body for POST .../override/character/{characterID}/slots.
// A nil SpellSlots or PactMagicSlots pointer means "leave that store untouched";
// each store is updated independently so a warlock's pact magic can be fixed
// without touching leveled spell slots (and vice versa).
type overrideSlotsRequest struct {
	SpellSlots     *json.RawMessage `json:"spell_slots"`
	PactMagicSlots *json.RawMessage `json:"pact_magic_slots"`
	Reason         string           `json:"reason"`
}

// overrideExhaustionRequest is the JSON body for POST .../exhaustion.
//
// Two modes:
//   - delta != 0: relative adjustment (current + delta), clamped to [0, 6].
//     Used by the forced-march / starvation / DM-decrement flow.
//   - delta == 0: absolute set to ExhaustionLevel, clamped to [0, 6]. Used
//     when the DM wants to assert a specific level (e.g. greater
//     restoration => 0).
//
// SR-019: spec line 1365 "applied by DM from dashboard".
type overrideExhaustionRequest struct {
	ExhaustionLevel int32  `json:"exhaustion_level"`
	Delta           int32  `json:"delta"`
	Reason          string `json:"reason"`
}

// maxExhaustionLevel is the SRD upper bound (6 = death). Higher values are
// silently clamped so the DM cannot put a combatant into an undefined state.
const maxExhaustionLevel int32 = 6

// resolveExhaustionLevel applies the override request's delta-or-absolute
// rule, then clamps to [0, maxExhaustionLevel]. Pure for testability.
func resolveExhaustionLevel(current int32, req overrideExhaustionRequest) int32 {
	next := req.ExhaustionLevel
	if req.Delta != 0 {
		next = current + req.Delta
	}
	if next < 0 {
		return 0
	}
	if next > maxExhaustionLevel {
		return maxExhaustionLevel
	}
	return next
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
		http.Error(w, httpjson.DecodeError(err), http.StatusBadRequest)
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
		status := http.StatusInternalServerError
		if errors.Is(err, errOverrideConflict) {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
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
			// ApplyDamage (Override=true) so concentration save / unconscious
			// hooks fire. R/I/V and temp HP are skipped — DM is asserting
			// an exact target HP. For non-decrease (heal / restore) use
			// the raw store call.
			var updated refdata.Combatant
			if req.HpCurrent < c.HpCurrent {
				if _, err = h.svc.ApplyDamage(ctx, ApplyDamageInput{
					EncounterID: encounterID,
					Target:      c,
					RawDamage:   int(c.HpCurrent - req.HpCurrent),
					Override:    true,
				}); err != nil {
					return err
				}
				// ApplyDamage updated HP+isAlive but not the override's
				// requested tempHp. Issue a follow-up store call so the
				// requested HP / temp HP / isAlive are honored exactly.
				updated, err = h.svc.store.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
					ID: combatantID, HpCurrent: req.HpCurrent, TempHp: req.TempHp, IsAlive: isAlive,
				})
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

// rejectDuplicateInitiativeOrder returns errOverrideConflict when another
// combatant in the encounter already holds initiative_order `order`. The
// combatant being edited (self) is excluded so re-sending its own order is a
// no-op (APP-3). A store error is surfaced as-is (mapped to 500).
func (h *DMDashboardHandler) rejectDuplicateInitiativeOrder(ctx context.Context, encounterID, selfID uuid.UUID, order int32) error {
	combatants, err := h.svc.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return err
	}
	for _, other := range combatants {
		if other.ID == selfID || other.InitiativeOrder != order {
			continue
		}
		return fmt.Errorf("%w: initiative_order %d is already used by %s", errOverrideConflict, order, other.DisplayName)
	}
	return nil
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
			// A nil field leaves the current value unchanged (APP-3).
			roll := c.InitiativeRoll
			if req.InitiativeRoll != nil {
				roll = *req.InitiativeRoll
			}
			order := c.InitiativeOrder
			if req.InitiativeOrder != nil {
				order = *req.InitiativeOrder
				if err := h.rejectDuplicateInitiativeOrder(ctx, encounterID, combatantID, order); err != nil {
					return err
				}
			}
			before, _ := json.Marshal(map[string]int32{
				"initiative_roll":  c.InitiativeRoll,
				"initiative_order": c.InitiativeOrder,
			})
			if _, err := h.svc.store.UpdateCombatantInitiative(ctx, refdata.UpdateCombatantInitiativeParams{
				ID: combatantID, InitiativeRoll: roll, InitiativeOrder: order,
			}); err != nil {
				return err
			}
			after, _ := json.Marshal(map[string]int32{
				"initiative_roll":  roll,
				"initiative_order": order,
			})
			h.logOverride(ctx, turnID, encounterID, combatantID, req.Reason, before, after)

			base := fmt.Sprintf("⚠️ **DM Correction:** %s initiative adjusted to %d", c.DisplayName, roll)
			h.postCorrection(ctx, encounterID, correctionMsg(base, req.Reason))
			return nil
		},
	)
}

// OverrideCombatantExhaustion handles POST .../override/combatant/{combatantID}/exhaustion.
//
// SR-019: the mutation surface for exhaustion. Per spec line 1365,
// exhaustion is "applied by DM from dashboard" — increment via the
// forced-march hook (`delta: 1`), decrement on greater restoration
// (`delta: -1`) or remove entirely (`exhaustion_level: 0`, `delta: 0`).
// Long-rest auto-decrement is wired through rest.LongRest; this endpoint
// powers the manual DM path. Value is clamped to [0, 6].
func (h *DMDashboardHandler) OverrideCombatantExhaustion(w http.ResponseWriter, r *http.Request) {
	var req overrideExhaustionRequest
	h.runOverride(w, r,
		func() error { return json.NewDecoder(r.Body).Decode(&req) },
		func(ctx context.Context, encounterID, combatantID, turnID uuid.UUID) error {
			c, err := h.svc.store.GetCombatant(ctx, combatantID)
			if err != nil {
				return fmt.Errorf("getting combatant: %w", err)
			}
			before, _ := snapshotExhaustionState(c)
			next := resolveExhaustionLevel(c.ExhaustionLevel, req)
			updated, err := h.svc.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
				ID:              combatantID,
				Conditions:      c.Conditions,
				ExhaustionLevel: next,
			})
			if err != nil {
				return fmt.Errorf("updating exhaustion: %w", err)
			}
			after, _ := snapshotExhaustionState(updated)
			h.logOverride(ctx, turnID, encounterID, combatantID, req.Reason, before, after)

			base := fmt.Sprintf("⚠️ **DM Correction:** %s exhaustion adjusted to level %d", c.DisplayName, next)
			h.postCorrection(ctx, encounterID, correctionMsg(base, req.Reason))
			return nil
		},
	)
}

// snapshotExhaustionState marshals the combatant's exhaustion level for the
// dm_override audit row. Kept separate from snapshotCombatantState so the
// undo path stays scoped to HP/position/conditions (this override is not
// yet covered by undoBeforeState; that's a follow-up if SR-019 acceptance
// asks for it).
func snapshotExhaustionState(c refdata.Combatant) (json.RawMessage, error) {
	snapshot := struct {
		ExhaustionLevel int32 `json:"exhaustion_level"`
	}{ExhaustionLevel: c.ExhaustionLevel}
	return json.Marshal(snapshot)
}

// OverrideCharacterSlots handles POST .../override/character/{characterID}/slots.
// It adjusts a character's leveled spell slots and/or Warlock pact-magic slots
// mid-combat. Each store is optional: omit a field to leave it untouched.
//
// Payloads are parsed and validated BEFORE the turn lock is acquired so that a
// malformed/invalid request returns HTTP 400 — only real DB failures inside the
// lock map to 500.
func (h *DMDashboardHandler) OverrideCharacterSlots(w http.ResponseWriter, r *http.Request) {
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

	var req overrideSlotsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	spellSlots, hasSpell, err := parseSpellSlotsOverride(req.SpellSlots)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	pactSlots, hasPact, err := parsePactSlotsOverride(req.PactMagicSlots)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

		before := snapshotCharacterSlots(char)

		if hasSpell {
			if _, err := h.svc.store.UpdateCharacterSpellSlots(r.Context(), refdata.UpdateCharacterSpellSlotsParams{
				ID:         characterID,
				SpellSlots: pqtype.NullRawMessage{RawMessage: spellSlots, Valid: true},
			}); err != nil {
				return fmt.Errorf("updating spell slots: %w", err)
			}
		}

		if hasPact {
			if _, err := h.svc.store.UpdateCharacterPactMagicSlots(r.Context(), refdata.UpdateCharacterPactMagicSlotsParams{
				ID:             characterID,
				PactMagicSlots: pqtype.NullRawMessage{RawMessage: pactSlots, Valid: true},
			}); err != nil {
				return fmt.Errorf("updating pact magic slots: %w", err)
			}
		}

		// action_log.actor_id is NOT NULL FK to combatants(id); the slot
		// override is a character-level mutation, so we attribute the audit row
		// to the active turn's combatant.
		_, _ = h.svc.store.CreateActionLog(r.Context(), refdata.CreateActionLogParams{
			TurnID:      turn.ID,
			EncounterID: encounterID,
			ActionType:  "dm_override",
			ActorID:     turn.CombatantID,
			Description: nullString(req.Reason),
			BeforeState: before,
			AfterState:  requestSlotsPayload(req),
		})

		base := fmt.Sprintf("⚠️ **DM Correction:** %s slots adjusted", char.Name)
		h.postCorrection(r.Context(), encounterID, correctionMsg(base, req.Reason))
		return nil
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// parseSpellSlotsOverride parses and validates the optional spell_slots payload,
// returning the storage-shaped JSON ({"1":{"current":2,"max":4}}) to persist.
// A nil pointer means "leave the leveled spell-slot store untouched".
func parseSpellSlotsOverride(raw *json.RawMessage) (out []byte, present bool, err error) {
	if raw == nil {
		return nil, false, nil
	}
	slots, err := character.ParseSpellSlotsJSON(*raw)
	if err != nil {
		return nil, false, err
	}
	if err := character.ValidateSpellSlots(slots); err != nil {
		return nil, false, err
	}
	out, err = character.MarshalSpellSlotsJSON(slots)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}

// parsePactSlotsOverride parses and validates the optional pact_magic_slots
// payload. A nil pointer means "leave the Warlock pact-magic store untouched".
func parsePactSlotsOverride(raw *json.RawMessage) (out []byte, present bool, err error) {
	if raw == nil {
		return nil, false, nil
	}
	var pact character.PactMagicSlots
	if err := json.Unmarshal(*raw, &pact); err != nil {
		return nil, false, err
	}
	if err := character.ValidatePactSlots(pact); err != nil {
		return nil, false, err
	}
	out, err = json.Marshal(pact)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}

// characterSlotsSnapshot is the before/after audit shape for a slots override:
// it carries both slot stores so the diff is symmetric. Either field is the
// JSON literal null when the corresponding store is empty/omitted.
type characterSlotsSnapshot struct {
	SpellSlots     json.RawMessage `json:"spell_slots"`
	PactMagicSlots json.RawMessage `json:"pact_magic_slots"`
}

// snapshotCharacterSlots captures both slot stores for the dm_override before-state.
func snapshotCharacterSlots(c refdata.Character) json.RawMessage {
	snap := characterSlotsSnapshot{
		SpellSlots:     json.RawMessage(`null`),
		PactMagicSlots: json.RawMessage(`null`),
	}
	if c.SpellSlots.Valid {
		snap.SpellSlots = c.SpellSlots.RawMessage
	}
	if c.PactMagicSlots.Valid {
		snap.PactMagicSlots = c.PactMagicSlots.RawMessage
	}
	b, _ := json.Marshal(snap)
	return b
}

// requestSlotsPayload renders the request's slot fields as the audit after-state,
// mirroring snapshotCharacterSlots so before/after diffs line up.
func requestSlotsPayload(req overrideSlotsRequest) json.RawMessage {
	snap := characterSlotsSnapshot{
		SpellSlots:     json.RawMessage(`null`),
		PactMagicSlots: json.RawMessage(`null`),
	}
	if req.SpellSlots != nil {
		snap.SpellSlots = *req.SpellSlots
	}
	if req.PactMagicSlots != nil {
		snap.PactMagicSlots = *req.PactMagicSlots
	}
	b, _ := json.Marshal(snap)
	return b
}

// overrideFeatureUsesRequest is the JSON body for
// POST .../override/character/{characterID}/feature-uses. Feature names a
// limited-use class resource in the character's feature_uses map (e.g. "rage");
// Current is the new remaining-uses value. Current is a pointer so an omitted
// field (nil) is rejected while an explicit 0 (no uses left) is honoured. The
// stored Max + Recharge metadata on the row is preserved.
type overrideFeatureUsesRequest struct {
	Feature string `json:"feature"`
	Current *int   `json:"current"`
	Reason  string `json:"reason"`
}

// characterFeatureUseSnapshot is the before/after audit shape for a feature-use
// override: the feature key plus its remaining uses and the (preserved) max.
type characterFeatureUseSnapshot struct {
	Feature string `json:"feature"`
	Current int    `json:"current"`
	Max     int    `json:"max"`
}

// snapshotFeatureUse renders a feature-use audit snapshot as JSON.
func snapshotFeatureUse(feature string, current, max int) json.RawMessage {
	b, _ := json.Marshal(characterFeatureUseSnapshot{Feature: feature, Current: current, Max: max})
	return b
}

// errInvalidFeatureOverride marks a feature-use override that is well-formed
// JSON but semantically invalid for the target character — the feature is not
// present in its feature_uses map, or Current exceeds the feature's Max. Mapped
// to HTTP 400 (the request itself is bad, not a server fault).
var errInvalidFeatureOverride = errors.New("invalid feature-use override")

// OverrideCharacterFeatureUses handles
// POST .../override/character/{characterID}/feature-uses. It sets a character's
// remaining uses of a limited-use feature (e.g. Barbarian rage) mid-combat,
// preserving the stored Max/Recharge metadata via Service.SetFeaturePool, and
// audits the change as a dm_override (action_log + #combat-log).
//
// Body shape is validated BEFORE the turn lock so malformed/invalid requests
// return 400. Inside the lock the character is read once: an unknown feature or
// an out-of-range Current also map to 400 (via errInvalidFeatureOverride); only
// real DB failures map to 500.
func (h *DMDashboardHandler) OverrideCharacterFeatureUses(w http.ResponseWriter, r *http.Request) {
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

	var req overrideFeatureUsesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Feature == "" {
		http.Error(w, "feature is required", http.StatusBadRequest)
		return
	}
	if req.Current == nil {
		http.Error(w, "current is required", http.StatusBadRequest)
		return
	}
	if *req.Current < 0 {
		http.Error(w, "current must not be negative", http.StatusBadRequest)
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

		featureUses, oldCurrent, err := ParseFeatureUses(char, req.Feature)
		if err != nil {
			return fmt.Errorf("parsing feature_uses: %w", err)
		}
		fu, ok := featureUses[req.Feature]
		if !ok {
			return fmt.Errorf("%w: %s has no %q feature", errInvalidFeatureOverride, char.Name, req.Feature)
		}
		// Max < 0 means an unlimited pool (e.g. a level-20 barbarian's rage):
		// skip the upper-bound check in that case.
		if fu.Max >= 0 && *req.Current > fu.Max {
			return fmt.Errorf("%w: current %d exceeds %s max of %d", errInvalidFeatureOverride, *req.Current, req.Feature, fu.Max)
		}

		before := snapshotFeatureUse(req.Feature, oldCurrent, fu.Max)
		if _, err := h.svc.SetFeaturePool(r.Context(), char, req.Feature, featureUses, *req.Current); err != nil {
			return fmt.Errorf("setting feature pool: %w", err)
		}
		after := snapshotFeatureUse(req.Feature, *req.Current, fu.Max)

		// action_log.actor_id is NOT NULL FK to combatants(id); this is a
		// character-level mutation, so attribute the audit row to the active
		// turn's combatant (mirrors OverrideCharacterSlots).
		h.logOverride(r.Context(), turn.ID, encounterID, turn.CombatantID, req.Reason, before, after)

		base := fmt.Sprintf("⚠️ **DM Correction:** %s %s uses set to %d", char.Name, req.Feature, *req.Current)
		h.postCorrection(r.Context(), encounterID, correctionMsg(base, req.Reason))
		return nil
	}); err != nil {
		if errors.Is(err, errInvalidFeatureOverride) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
