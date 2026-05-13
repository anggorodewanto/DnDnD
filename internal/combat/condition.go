package combat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// AddCondition adds a condition to the JSONB array.
func AddCondition(conditions json.RawMessage, condition CombatCondition) (json.RawMessage, error) {
	conds, err := parseConditions(conditions)
	if err != nil {
		return nil, err
	}
	return json.Marshal(append(conds, condition))
}

// HasCondition checks if a condition is present in the JSONB array.
func HasCondition(conditions json.RawMessage, conditionName string) bool {
	conds, err := parseConditions(conditions)
	if err != nil {
		return false
	}
	for _, c := range conds {
		if c.Condition == conditionName {
			return true
		}
	}
	return false
}

// GetCondition retrieves a specific condition by name from the JSONB array.
func GetCondition(conditions json.RawMessage, conditionName string) (CombatCondition, bool) {
	conds, err := parseConditions(conditions)
	if err != nil {
		return CombatCondition{}, false
	}
	for _, c := range conds {
		if c.Condition == conditionName {
			return c, true
		}
	}
	return CombatCondition{}, false
}

// ListConditions lists all conditions from the JSONB array.
func ListConditions(conditions json.RawMessage) ([]CombatCondition, error) {
	return parseConditions(conditions)
}

// RemoveCondition removes a condition by name from the JSONB array.
func RemoveCondition(conditions json.RawMessage, conditionName string) (json.RawMessage, error) {
	conds, err := parseConditions(conditions)
	if err != nil {
		return nil, err
	}
	filtered := make([]CombatCondition, 0, len(conds))
	for _, c := range conds {
		if c.Condition != conditionName {
			filtered = append(filtered, c)
		}
	}
	return json.Marshal(filtered)
}

// isExpired checks whether a condition has expired based on the current round,
// the triggering combatant, and the timing phase.
func isExpired(c CombatCondition, currentRound int, triggerCombatantID string, timing string) bool {
	// Indefinite conditions never auto-expire
	if c.DurationRounds <= 0 {
		return false
	}
	// Only expire conditions placed by the trigger combatant
	if c.SourceCombatantID != triggerCombatantID {
		return false
	}
	// Check timing — default to "start_of_turn" if empty
	expiresOn := c.ExpiresOn
	if expiresOn == "" {
		expiresOn = "start_of_turn"
	}
	if expiresOn != timing {
		return false
	}
	// Check if enough rounds have passed
	return currentRound >= c.StartedRound+c.DurationRounds
}

// ExpiredConditionInfo holds information about an expired condition.
type ExpiredConditionInfo struct {
	Condition         string
	SourceCombatantID string
	Message           string
}

// CheckExpiredConditions checks all conditions for expiration and returns
// updated conditions JSONB and info about expired conditions.
func CheckExpiredConditions(conditions json.RawMessage, currentRound int, triggerCombatantID string, timing string) (json.RawMessage, []ExpiredConditionInfo, error) {
	conds, err := parseConditions(conditions)
	if err != nil {
		return nil, nil, err
	}

	var remaining []CombatCondition
	var expired []ExpiredConditionInfo

	for _, c := range conds {
		if isExpired(c, currentRound, triggerCombatantID, timing) {
			expired = append(expired, ExpiredConditionInfo{
				Condition:         c.Condition,
				SourceCombatantID: c.SourceCombatantID,
				Message:           fmt.Sprintf("⏱️ %s has expired", c.Condition),
			})
			continue
		}
		remaining = append(remaining, c)
	}

	if remaining == nil {
		remaining = []CombatCondition{}
	}
	updated, err := json.Marshal(remaining)
	if err != nil {
		return nil, nil, err
	}
	return updated, expired, nil
}

// ApplyCondition applies a condition to a combatant and returns the updated
// combatant and combat log messages. If the combatant has a creature_ref_id,
// condition immunities are checked first; immune conditions are blocked with
// a log message.
func (s *Service) ApplyCondition(ctx context.Context, combatantID uuid.UUID, condition CombatCondition) (refdata.Combatant, []string, error) {
	c, err := s.store.GetCombatant(ctx, combatantID)
	if err != nil {
		return refdata.Combatant{}, nil, fmt.Errorf("getting combatant: %w", err)
	}

	// Check condition immunity for creatures
	if c.CreatureRefID.Valid && c.CreatureRefID.String != "" {
		creature, err := s.store.GetCreature(ctx, c.CreatureRefID.String)
		if err != nil {
			return refdata.Combatant{}, nil, fmt.Errorf("getting creature for immunity check: %w", err)
		}
		if CheckConditionImmunity(condition.Condition, creature.ConditionImmunities) {
			msg := fmt.Sprintf("🛡️ %s is immune to %s", c.DisplayName, condition.Condition)
			return c, []string{msg}, nil
		}
	}

	newConds, err := AddCondition(c.Conditions, condition)
	if err != nil {
		return refdata.Combatant{}, nil, fmt.Errorf("adding condition: %w", err)
	}

	updated, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              combatantID,
		Conditions:      newConds,
		ExhaustionLevel: c.ExhaustionLevel,
	})
	if err != nil {
		return refdata.Combatant{}, nil, fmt.Errorf("updating conditions: %w", err)
	}

	msgs := []string{formatConditionApplied(condition, c.DisplayName)}

	// Phase 17: refresh the persistent character card so the new condition
	// (and any exhaustion-level change carried in the same row update) is
	// immediately visible. NPC combatants are silently skipped.
	s.notifyCardUpdate(ctx, updated)

	// C-31 — fall damage on prone-while-airborne. Apply 1d6 per 10ft via
	// FallDamage and reset altitude to 0 when the new condition is prone
	// and the combatant was airborne. The damage line is appended to msgs
	// so the combat log surfaces the fall.
	if condition.Condition == "prone" && c.AltitudeFt > 0 {
		fallMsg, fallUpdated, ferr := s.applyFallDamageOnProne(ctx, updated)
		if ferr != nil {
			return updated, msgs, ferr
		}
		if fallMsg != "" {
			msgs = append(msgs, fallMsg)
		}
		updated = fallUpdated
	}

	// Phase 118: incapacitating conditions auto-break concentration on the
	// affected combatant (no save). Apply this hook after the condition has
	// been persisted so the cleanup logic sees the up-to-date row.
	if !incapacitatingConditions[condition.Condition] {
		return updated, msgs, nil
	}
	extra, err := s.maybeAutoBreakConcentration(ctx, updated, fmt.Sprintf("incapacitated — %s", condition.Condition))
	if err != nil {
		return updated, msgs, err
	}
	msgs = append(msgs, extra...)
	return updated, msgs, nil
}

// maybeAutoBreakConcentration looks up the target's concentration spell from
// the authoritative columns and, if present, fires BreakConcentrationFully
// with the supplied reason. Returns the consolidated 💨 line as a single-
// element slice (or nil when nothing was broken). Used by the incapacitation
// hook in ApplyCondition and any future trigger that observes a target
// rather than driving a /cast through the spellcasting flow.
func (s *Service) maybeAutoBreakConcentration(ctx context.Context, target refdata.Combatant, reason string) ([]string, error) {
	cleanup, err := s.breakStoredConcentration(ctx, target, reason)
	if err != nil {
		return nil, err
	}
	if cleanup == nil {
		return nil, nil
	}
	return []string{cleanup.ConsolidatedMessage}, nil
}

// ApplyConditionWithLog applies a condition and persists the log message to action_log.
func (s *Service) ApplyConditionWithLog(ctx context.Context, combatantID uuid.UUID, condition CombatCondition, encounterID uuid.UUID, turnID uuid.UUID) (refdata.Combatant, []string, error) {
	updated, msgs, err := s.ApplyCondition(ctx, combatantID, condition)
	if err != nil {
		return updated, msgs, err
	}
	if err := s.logConditionMessages(ctx, msgs, "condition_applied", combatantID, encounterID, turnID); err != nil {
		return updated, msgs, err
	}
	return updated, msgs, nil
}

// RemoveConditionFromCombatant removes a condition from a combatant and returns
// the updated combatant and combat log messages.
func (s *Service) RemoveConditionFromCombatant(ctx context.Context, combatantID uuid.UUID, conditionName string) (refdata.Combatant, []string, error) {
	c, err := s.store.GetCombatant(ctx, combatantID)
	if err != nil {
		return refdata.Combatant{}, nil, fmt.Errorf("getting combatant: %w", err)
	}

	newConds, err := RemoveCondition(c.Conditions, conditionName)
	if err != nil {
		return refdata.Combatant{}, nil, fmt.Errorf("removing condition: %w", err)
	}

	updated, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              combatantID,
		Conditions:      newConds,
		ExhaustionLevel: c.ExhaustionLevel,
	})
	if err != nil {
		return refdata.Combatant{}, nil, fmt.Errorf("updating conditions: %w", err)
	}

	msg := fmt.Sprintf("🟢 %s removed from %s", conditionName, c.DisplayName)
	return updated, []string{msg}, nil
}

// maybeClearTurnedOnDamage clears the `turned` condition from a target that
// just took positive damage. Mirrors the markRageTookDamage /
// maybeEndRageOnUnconscious post-damage-effects hook in damage.go: the call
// site gates on `adjusted > 0`, so healing, immunity, and full temp-HP
// absorption never reach here. The clearing is source-agnostic — both damage
// from the original turner and from a third party trigger removal, matching
// the Turn Undead spec rule that any damage to a Turned creature ends the
// condition. Best-effort: lookup / persistence errors are swallowed so the
// damage write is never blocked by cleanup. (SR-023)
func (s *Service) maybeClearTurnedOnDamage(ctx context.Context, target refdata.Combatant) {
	if !HasCondition(target.Conditions, "turned") {
		return
	}
	_, _, _ = s.RemoveConditionFromCombatant(ctx, target.ID, "turned")
}

// RemoveConditionWithLog removes a condition and persists the log message to action_log.
func (s *Service) RemoveConditionWithLog(ctx context.Context, combatantID uuid.UUID, conditionName string, encounterID uuid.UUID, turnID uuid.UUID) (refdata.Combatant, []string, error) {
	updated, msgs, err := s.RemoveConditionFromCombatant(ctx, combatantID, conditionName)
	if err != nil {
		return updated, msgs, err
	}
	if err := s.logConditionMessages(ctx, msgs, "condition_removed", combatantID, encounterID, turnID); err != nil {
		return updated, msgs, err
	}
	return updated, msgs, nil
}

// ProcessTurnStart checks ALL combatants in the encounter for conditions that
// expire at the start of the given combatant's turn (where source_combatant_id
// matches the current combatant), auto-removes expired conditions, and returns
// log messages.
func (s *Service) ProcessTurnStart(ctx context.Context, encounterID uuid.UUID, combatant refdata.Combatant, currentRound int32) ([]string, error) {
	return s.processExpiredConditions(ctx, encounterID, combatant.ID, int(currentRound), "start_of_turn", nil)
}

// ProcessTurnStartWithLog is like ProcessTurnStart but also persists messages to the action_log.
func (s *Service) ProcessTurnStartWithLog(ctx context.Context, encounterID uuid.UUID, combatant refdata.Combatant, currentRound int32, turnID uuid.UUID) ([]string, error) {
	return s.processExpiredConditions(ctx, encounterID, combatant.ID, int(currentRound), "start_of_turn", &turnID)
}

// ProcessTurnEnd checks ALL combatants in the encounter for conditions that
// expire at the end of the given combatant's turn, auto-removes expired
// conditions, and returns log messages.
func (s *Service) ProcessTurnEnd(ctx context.Context, encounterID uuid.UUID, combatantID uuid.UUID, currentRound int32) ([]string, error) {
	return s.processExpiredConditions(ctx, encounterID, combatantID, int(currentRound), "end_of_turn", nil)
}

// ProcessTurnEndWithLog is like ProcessTurnEnd but also persists messages to the action_log.
func (s *Service) ProcessTurnEndWithLog(ctx context.Context, encounterID uuid.UUID, combatantID uuid.UUID, currentRound int32, turnID uuid.UUID) ([]string, error) {
	return s.processExpiredConditions(ctx, encounterID, combatantID, int(currentRound), "end_of_turn", &turnID)
}

// logConditionMessages persists a slice of messages to the action_log.
func (s *Service) logConditionMessages(ctx context.Context, msgs []string, actionType string, actorID uuid.UUID, encounterID uuid.UUID, turnID uuid.UUID) error {
	for _, msg := range msgs {
		if _, err := s.store.CreateActionLog(ctx, refdata.CreateActionLogParams{
			TurnID:      turnID,
			EncounterID: encounterID,
			ActionType:  actionType,
			ActorID:     actorID,
			Description: nullString(msg),
			BeforeState: json.RawMessage(`{}`),
			AfterState:  json.RawMessage(`{}`),
		}); err != nil {
			return fmt.Errorf("logging %s: %w", actionType, err)
		}
	}
	return nil
}

// formatConditionApplied builds the log message for a newly applied condition.
func formatConditionApplied(condition CombatCondition, displayName string) string {
	if condition.DurationRounds <= 0 {
		return fmt.Sprintf("🔴 %s applied to %s (indefinite)", condition.Condition, displayName)
	}

	timing := condition.ExpiresOn
	if timing == "" {
		timing = "start_of_turn"
	}
	timingLabel := "start"
	if timing == "end_of_turn" {
		timingLabel = "end"
	}

	if condition.SourceCombatantID != "" {
		return fmt.Sprintf("🔴 %s applied to %s by source (%d rounds, expires at %s of source's turn)",
			condition.Condition, displayName, condition.DurationRounds, timingLabel)
	}
	return fmt.Sprintf("🔴 %s applied to %s (%d rounds, expires at %s of turn)",
		condition.Condition, displayName, condition.DurationRounds, timingLabel)
}

// processExpiredConditions is the shared implementation for ProcessTurnStart and ProcessTurnEnd.
func (s *Service) processExpiredConditions(ctx context.Context, encounterID uuid.UUID, triggerCombatantID uuid.UUID, currentRound int, timing string, turnID *uuid.UUID) ([]string, error) {
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return nil, fmt.Errorf("listing combatants: %w", err)
	}

	// Build a map of combatant IDs to display names for source lookup
	nameByID := make(map[string]string, len(combatants))
	for _, c := range combatants {
		nameByID[c.ID.String()] = c.DisplayName
	}

	var allMsgs []string
	triggerID := triggerCombatantID.String()

	for _, c := range combatants {
		updated, expired, err := CheckExpiredConditions(c.Conditions, currentRound, triggerID, timing)
		if err != nil {
			return nil, fmt.Errorf("checking expired conditions for %s: %w", c.DisplayName, err)
		}
		if len(expired) == 0 {
			continue
		}

		// Build enriched messages with target name and source name
		var msgs []string
		for _, info := range expired {
			sourceName := nameByID[info.SourceCombatantID]
			if sourceName == "" {
				sourceName = "unknown"
			}
			msg := fmt.Sprintf("⏱️ %s on %s has expired (placed by %s).", info.Condition, c.DisplayName, sourceName)
			msgs = append(msgs, msg)

			// Persist to action_log if turnID is available
			if turnID != nil {
				if _, err := s.store.CreateActionLog(ctx, refdata.CreateActionLogParams{
					TurnID:      *turnID,
					EncounterID: encounterID,
					ActionType:  "condition_expired",
					ActorID:     c.ID,
					Description: nullString(msg),
					BeforeState: json.RawMessage(`{}`),
					AfterState:  json.RawMessage(`{}`),
				}); err != nil {
					return nil, fmt.Errorf("logging condition expiration for %s: %w", c.DisplayName, err)
				}
			}
		}

		if _, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
			ID:              c.ID,
			Conditions:      updated,
			ExhaustionLevel: c.ExhaustionLevel,
		}); err != nil {
			return nil, fmt.Errorf("updating conditions for %s: %w", c.DisplayName, err)
		}

		allMsgs = append(allMsgs, msgs...)
	}

	return allMsgs, nil
}
