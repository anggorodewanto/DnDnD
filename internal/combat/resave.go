package combat

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// EndOfTurnResaveResult reports the outcome of a bearer's 2024 end-of-turn
// repeat saving throw against a "save ends" condition (COV-19).
type EndOfTurnResaveResult struct {
	// Resolved is true when a save-ends condition was found and re-rolled.
	Resolved bool
	// Success is true when the save met the DC (condition ended; concentration
	// may have dropped). False leaves the condition in place to re-save next turn.
	Success   bool
	Natural   int
	Total     int
	DC        int
	Ability   string
	Condition string
	// Messages holds the combat-log lines for the resolution.
	Messages []string
}

// firstSaveEndsCondition returns the first condition carrying end-of-turn
// re-save metadata (SaveEndsAbility set), or (zero, false) if the bearer holds
// no "save ends" condition.
func firstSaveEndsCondition(conds []CombatCondition) (CombatCondition, bool) {
	for _, c := range conds {
		if c.SaveEndsAbility != "" {
			return c, true
		}
	}
	return CombatCondition{}, false
}

// applySaveEndsOutcome applies the 2024 "save ends" resolution for one bearer
// and one condition given an already-decided success. On failure the condition
// persists (a "still held" log line, no state change). On success the condition
// is removed and — when this was the last combatant still held by the source
// spell — the caster's now-purposeless concentration is dropped (the spell
// ends). Returns the combat-log lines. Early-return style keeps the failure and
// no-concentration paths flat.
func (s *Service) applySaveEndsOutcome(ctx context.Context, encounterID uuid.UUID, bearer refdata.Combatant, cond CombatCondition, success bool) ([]string, error) {
	ability := strings.ToUpper(cond.SaveEndsAbility)
	if !success {
		return []string{fmt.Sprintf("🔒 %s fails the %s save — still %s.", bearer.DisplayName, ability, cond.Condition)}, nil
	}

	_, removeMsgs, err := s.RemoveConditionFromCombatant(ctx, bearer.ID, cond.Condition)
	if err != nil {
		return nil, fmt.Errorf("removing %s on successful re-save: %w", cond.Condition, err)
	}
	msgs := append([]string{fmt.Sprintf("✨ %s succeeds on the %s save — %s ends.", bearer.DisplayName, ability, cond.Condition)}, removeMsgs...)

	// Only a concentration spell scoped to a caster can drop concentration here.
	if cond.SourceSpell == "" || cond.SourceCombatantID == "" {
		return msgs, nil
	}

	stillHeld, err := s.anyCombatantBearsSpellCondition(ctx, encounterID, cond.SourceSpell, cond.SourceCombatantID, bearer.ID)
	if err != nil {
		return msgs, err
	}
	if stillHeld {
		return msgs, nil // other targets remain held → the spell (and concentration) continues
	}

	casterID, err := uuid.Parse(cond.SourceCombatantID)
	if err != nil {
		return msgs, nil // unscoped/unknown caster → nothing to drop
	}
	caster, err := s.store.GetCombatant(ctx, casterID)
	if err != nil {
		return msgs, fmt.Errorf("getting caster for re-save concentration drop: %w", err)
	}
	drop, err := s.breakStoredConcentration(ctx, caster, "target saved")
	if err != nil {
		return msgs, err
	}
	if drop != nil && drop.Broken {
		msgs = append(msgs, drop.ConsolidatedMessage)
	}
	return msgs, nil
}

// anyCombatantBearsSpellCondition reports whether any combatant OTHER than
// excludeID still carries a condition sourced from spellID + sourceCombatantID.
// Used to decide whether a save-ends success ends the whole spell (drop
// concentration) or only frees one of several held targets.
func (s *Service) anyCombatantBearsSpellCondition(ctx context.Context, encounterID uuid.UUID, spellID, sourceCombatantID string, excludeID uuid.UUID) (bool, error) {
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return false, fmt.Errorf("listing combatants for save-ends scope: %w", err)
	}
	for _, c := range combatants {
		if c.ID == excludeID {
			continue
		}
		conds, err := parseConditions(c.Conditions)
		if err != nil {
			continue
		}
		for _, cond := range conds {
			if cond.SourceSpell == spellID && cond.SourceCombatantID == sourceCombatantID {
				return true, nil
			}
		}
	}
	return false, nil
}

// rollNPCEndOfTurnResave rolls the honest, server-side end-of-turn repeat save
// for an NPC bearer and applies the outcome. The DM triggers this by running
// the NPC's turn (ExecuteEnemyTurn), keeping NPC saves DM-driven. Returns nil
// when the bearer holds no save-ends condition (a no-op for ordinary NPC turns).
// Saving throws do not crit in 2024 (no nat-20/nat-1 auto), so success is a
// plain total-vs-DC comparison.
func (s *Service) rollNPCEndOfTurnResave(ctx context.Context, encounterID uuid.UUID, bearer refdata.Combatant) (*EndOfTurnResaveResult, error) {
	conds, err := parseConditions(bearer.Conditions)
	if err != nil {
		return nil, nil // malformed conditions must not block turn flow
	}
	cond, ok := firstSaveEndsCondition(conds)
	if !ok {
		return nil, nil
	}

	bonus, err := s.resolveCombatantSaveBonus(ctx, bearer, cond.SaveEndsAbility)
	if err != nil {
		return nil, fmt.Errorf("resolving %s save bonus for re-save: %w", cond.SaveEndsAbility, err)
	}
	d20, err := s.roller.Roll("1d20")
	if err != nil {
		return nil, fmt.Errorf("rolling end-of-turn re-save d20: %w", err)
	}
	natural := d20.Total
	total := natural + bonus
	success := total >= cond.SaveEndsDC

	msgs, err := s.applySaveEndsOutcome(ctx, encounterID, bearer, cond, success)
	if err != nil {
		return nil, err
	}
	head := fmt.Sprintf("🎲 %s end-of-turn %s re-save: %d vs DC %d.", bearer.DisplayName, strings.ToUpper(cond.SaveEndsAbility), total, cond.SaveEndsDC)
	return &EndOfTurnResaveResult{
		Resolved:  true,
		Success:   success,
		Natural:   natural,
		Total:     total,
		DC:        cond.SaveEndsDC,
		Ability:   cond.SaveEndsAbility,
		Condition: cond.Condition,
		Messages:  append([]string{head}, msgs...),
	}, nil
}

// ResolveEndOfTurnResave is the exported entry point for the /save handler: when
// a player rolls their own end-of-turn repeat save, it resolves any matching
// save-ends condition on their combatant (COV-19), removing it — and dropping
// the enemy caster's concentration — on a success. Returns nil when the player
// holds no save-ends condition for the rolled ability, so the handler can call
// it unconditionally.
func (s *Service) ResolveEndOfTurnResave(ctx context.Context, encounterID, combatantID uuid.UUID, ability string, total int, autoFail bool) (*EndOfTurnResaveResult, error) {
	return s.resolveEndOfTurnResaveForCombatant(ctx, encounterID, combatantID, ability, total, autoFail)
}

// resolveEndOfTurnResaveForCombatant resolves a bearer's end-of-turn re-save
// from an externally-rolled total — the PC path, where the player rolls their
// own /save. It matches the rolled ability to a save-ends condition on the
// combatant; a mismatch (or no such condition) is a no-op (nil result) so the
// /save handler can call it unconditionally. autoFail forces a failure (natural
// 1 / auto-fail effects).
func (s *Service) resolveEndOfTurnResaveForCombatant(ctx context.Context, encounterID uuid.UUID, combatantID uuid.UUID, ability string, total int, autoFail bool) (*EndOfTurnResaveResult, error) {
	bearer, err := s.store.GetCombatant(ctx, combatantID)
	if err != nil {
		return nil, fmt.Errorf("getting combatant for re-save: %w", err)
	}
	conds, err := parseConditions(bearer.Conditions)
	if err != nil {
		return nil, nil
	}

	ability = strings.ToLower(ability)
	if ability == "" {
		return nil, nil
	}
	var cond CombatCondition
	found := false
	for _, c := range conds {
		if strings.ToLower(c.SaveEndsAbility) == ability {
			cond = c
			found = true
			break
		}
	}
	if !found {
		return nil, nil
	}

	success := !autoFail && total >= cond.SaveEndsDC
	msgs, err := s.applySaveEndsOutcome(ctx, encounterID, bearer, cond, success)
	if err != nil {
		return nil, err
	}
	return &EndOfTurnResaveResult{
		Resolved:  true,
		Success:   success,
		Total:     total,
		DC:        cond.SaveEndsDC,
		Ability:   cond.SaveEndsAbility,
		Condition: cond.Condition,
		Messages:  msgs,
	}, nil
}
