package combat

import (
	"context"
	"fmt"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// applyMasteryEffects resolves the target-side consequence of a 2024 Weapon
// Mastery property that fired on an attack. It is a no-op unless
// result.MasteryProperty is set (which ResolveAttack only does when the
// attacker knows the weapon's mastery), so existing non-mastery attacks are
// completely unaffected.
//
//   - "graze": a miss carrying DamageTotal > 0 deals that flat ability-modifier
//     damage to the target through the standard ApplyDamage pipeline so
//     resistance / immunity / vulnerability and temp HP still apply.
//   - "topple": a hit forces the target to make a CON save vs
//     result.MasteryToppleSaveDC; on a failure the Prone condition is applied.
func (s *Service) applyMasteryEffects(ctx context.Context, attacker, target refdata.Combatant, result *AttackResult, roller *dice.Roller) error {
	switch result.MasteryProperty {
	case "graze":
		return s.applyGrazeDamage(ctx, target, result)
	case "topple":
		return s.applyToppleSave(ctx, attacker, target, result, roller)
	default:
		return nil
	}
}

// applyGrazeDamage applies the Graze miss-damage to the target. The damage was
// computed in ResolveAttack (ability modifier, min 0) and carried on
// result.DamageTotal. A zero total is a no-op.
func (s *Service) applyGrazeDamage(ctx context.Context, target refdata.Combatant, result *AttackResult) error {
	if result.DamageTotal <= 0 {
		return nil
	}
	if _, err := s.ApplyDamage(ctx, ApplyDamageInput{
		EncounterID: target.EncounterID,
		Target:      target,
		RawDamage:   result.DamageTotal,
		DamageType:  result.DamageType,
	}); err != nil {
		return fmt.Errorf("applying graze damage: %w", err)
	}
	return nil
}

// applyToppleSave resolves the target's CON save against the Topple DC and
// applies the Prone condition on a failure. The save uses the target's CON
// modifier (creature saving-throw bonus when present, else the CON ability
// modifier) via the shared resolveTargetConSave helper.
func (s *Service) applyToppleSave(ctx context.Context, attacker, target refdata.Combatant, result *AttackResult, roller *dice.Roller) error {
	conSaveBonus, err := s.resolveTargetConSave(ctx, target)
	if err != nil {
		return fmt.Errorf("resolving topple CON save: %w", err)
	}

	d20Result, err := roller.RollD20(conSaveBonus, dice.Normal)
	if err != nil {
		return fmt.Errorf("rolling topple CON save: %w", err)
	}

	if d20Result.Total >= result.MasteryToppleSaveDC {
		return nil // save succeeds → no Prone
	}

	prone := CombatCondition{
		Condition:         "prone",
		DurationRounds:    0,
		SourceCombatantID: attacker.ID.String(),
	}
	if _, _, err := s.ApplyCondition(ctx, target.ID, prone); err != nil {
		return fmt.Errorf("applying prone from topple: %w", err)
	}
	return nil
}
