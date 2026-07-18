package combat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// MoveSpellMarkerResult is the outcome of relocating a concentration-scoped
// spell marker (Hex, Hunter's Mark) from a downed target to a new creature.
type MoveSpellMarkerResult struct {
	CombatLog     string
	NewTarget     refdata.Combatant
	OldTargetName string // "" when no prior marked target remained in the encounter
	Turn          refdata.Turn
}

// MoveHex handles /bonus hex: as a bonus action the caster moves their Hex
// curse to a new creature. Per RAW this is allowed once the current target has
// dropped to 0 HP (or has already left the encounter). Mirrors MoveHuntersMark.
func (s *Service) MoveHex(ctx context.Context, caster refdata.Combatant, turn refdata.Turn, newTargetID uuid.UUID) (MoveSpellMarkerResult, error) {
	return s.moveSpellMarker(ctx, caster, turn, newTargetID, hexConditionName, hexSpellID, "Hex")
}

// MoveHuntersMark handles /bonus hunters-mark: the ranger moves their Hunter's
// Mark to a new creature as a bonus action once the current target drops to 0
// HP. Mirrors MoveHex.
func (s *Service) MoveHuntersMark(ctx context.Context, caster refdata.Combatant, turn refdata.Turn, newTargetID uuid.UUID) (MoveSpellMarkerResult, error) {
	return s.moveSpellMarker(ctx, caster, turn, newTargetID, huntersMarkConditionName, huntersMarkSpellID, "Hunter's Mark")
}

// moveSpellMarker is the shared body for MoveHex / MoveHuntersMark. Order is
// deliberate: every read-only validation runs (and can reject) BEFORE any
// mutation, so a rejected move never strips a marker or spends the bonus
// action. It:
//  1. gates on the bonus action being available,
//  2. requires the caster to be concentrating on the spell,
//  3. finds the caster's current marker; a still-standing (>0 HP) marked target
//     blocks the move (RAW: the curse only relocates off a target that dropped
//     to 0 HP), the new target already carrying it is a no-op error, and no
//     marker at all (original target gone) is allowed,
//  4. spends the bonus action, strips the old marker, stamps the new target,
//     and returns the combat-log line.
func (s *Service) moveSpellMarker(ctx context.Context, caster refdata.Combatant, turn refdata.Turn, newTargetID uuid.UUID, condName, spellID, spellName string) (MoveSpellMarkerResult, error) {
	if err := ValidateResource(turn, ResourceBonusAction); err != nil {
		return MoveSpellMarkerResult{}, err
	}

	if newTargetID == caster.ID {
		return MoveSpellMarkerResult{}, fmt.Errorf("cannot move %s onto yourself", spellName)
	}

	conc, err := s.store.GetCombatantConcentration(ctx, caster.ID)
	if err != nil {
		return MoveSpellMarkerResult{}, fmt.Errorf("looking up concentration: %w", err)
	}
	if !conc.ConcentrationSpellID.Valid || conc.ConcentrationSpellID.String != spellID {
		return MoveSpellMarkerResult{}, fmt.Errorf("%s is not concentrating on %s", caster.DisplayName, spellName)
	}

	oldTarget, err := s.findSpellMarkedTarget(ctx, caster, newTargetID, condName, spellID, spellName)
	if err != nil {
		return MoveSpellMarkerResult{}, err
	}

	updatedTurn, err := UseResource(turn, ResourceBonusAction)
	if err != nil {
		return MoveSpellMarkerResult{}, fmt.Errorf("using bonus action: %w", err)
	}
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return MoveSpellMarkerResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	oldName := ""
	if oldTarget != nil {
		oldName = oldTarget.DisplayName
		if err := s.stripSpellMarker(ctx, *oldTarget, caster.ID, condName, spellID); err != nil {
			return MoveSpellMarkerResult{}, err
		}
	}

	marker := newSpellMarkerCond(condName, spellID, caster.ID.String())
	updatedTarget, _, err := s.ApplyCondition(ctx, newTargetID, marker)
	if err != nil {
		return MoveSpellMarkerResult{}, fmt.Errorf("applying %s marker: %w", spellName, err)
	}

	return MoveSpellMarkerResult{
		CombatLog:     formatMoveSpellMarkerLog(caster.DisplayName, spellName, oldName, updatedTarget.DisplayName),
		NewTarget:     updatedTarget,
		OldTargetName: oldName,
		Turn:          updatedTurn,
	}, nil
}

// findSpellMarkedTarget scans the encounter for the combatant currently
// carrying the caster's (condName, spellID) marker. It returns nil when none is
// marked (the original target has left the encounter — the move is still
// allowed). It rejects the move when the marked target is still above 0 HP (RAW
// gate) or when the new target already carries the marker. The scan doubles as
// the new target's existence check, so no separate GetCombatant read is needed.
func (s *Service) findSpellMarkedTarget(ctx context.Context, caster refdata.Combatant, newTargetID uuid.UUID, condName, spellID, spellName string) (*refdata.Combatant, error) {
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, caster.EncounterID)
	if err != nil {
		return nil, fmt.Errorf("listing combatants: %w", err)
	}
	for i := range combatants {
		c := combatants[i]
		if !targetMarkedBySpell(c.Conditions, caster.ID, condName, spellID) {
			continue
		}
		if c.ID == newTargetID {
			return nil, fmt.Errorf("%s already carries your %s", c.DisplayName, spellName)
		}
		if c.HpCurrent > 0 {
			return nil, fmt.Errorf("%s still has %d HP — you can only move %s once its target drops to 0 HP", c.DisplayName, c.HpCurrent, spellName)
		}
		return &c, nil
	}
	return nil, nil
}

// stripSpellMarker removes the caster's (condName, spellID) marker from a single
// combatant and persists the change, leaving that combatant's other conditions
// (e.g. unconscious/prone from being downed) intact.
func (s *Service) stripSpellMarker(ctx context.Context, target refdata.Combatant, casterID uuid.UUID, condName, spellID string) error {
	conds, err := parseConditions(target.Conditions)
	if err != nil {
		return fmt.Errorf("parsing conditions for %s: %w", target.DisplayName, err)
	}
	filtered := stripSpellMarkerConds(conds, condName, spellID, casterID.String())
	encoded, err := json.Marshal(filtered)
	if err != nil {
		return fmt.Errorf("marshalling conditions for %s: %w", target.DisplayName, err)
	}
	if _, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              target.ID,
		Conditions:      encoded,
		ExhaustionLevel: target.ExhaustionLevel,
	}); err != nil {
		return fmt.Errorf("updating conditions for %s: %w", target.DisplayName, err)
	}
	return nil
}

// formatMoveSpellMarkerLog renders the #combat-log line for a marker move. The
// "from <old>" clause is omitted when the previous target was already gone.
func formatMoveSpellMarkerLog(casterName, spellName, oldTargetName, newTargetName string) string {
	if oldTargetName == "" {
		return fmt.Sprintf("\U0001f501 %s moves %s onto %s.", casterName, spellName, newTargetName)
	}
	return fmt.Sprintf("\U0001f501 %s moves %s from %s onto %s.", casterName, spellName, oldTargetName, newTargetName)
}
