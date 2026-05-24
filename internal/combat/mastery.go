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
//   - "vex": a hit applies a vex_advantage condition to the ATTACKER, scoped to
//     the target, granting advantage on the attacker's next attack vs that same
//     target (mirrors the Help action's help_advantage; single-shot consume).
//   - "sap": a hit applies a sap_disadvantage condition to the TARGET, imposing
//     disadvantage on the target's next attack (single-shot consume).
func (s *Service) applyMasteryEffects(ctx context.Context, attacker, target refdata.Combatant, result *AttackResult, roller *dice.Roller) error {
	switch result.MasteryProperty {
	case "graze":
		return s.applyGrazeDamage(ctx, target, result)
	case "topple":
		return s.applyToppleSave(ctx, attacker, target, result, roller)
	case "vex":
		return s.applyVexAdvantage(ctx, attacker, target)
	case "sap":
		return s.applySapDisadvantage(ctx, attacker, target)
	default:
		return nil
	}
}

// applyVexAdvantage applies a vex_advantage condition to the attacker, scoped
// to the target it just hit. It reuses the same CombatCondition shape and
// single-shot/expiry convention as the Help action's help_advantage so the
// existing target-scoping (DetectAdvantage) and consume machinery applies. The
// next attack vs that target spends the grant; the condition also self-expires
// at the start of the attacker's next turn.
func (s *Service) applyVexAdvantage(ctx context.Context, attacker, target refdata.Combatant) error {
	cond := CombatCondition{
		Condition:         "vex_advantage",
		DurationRounds:    1,
		SourceCombatantID: attacker.ID.String(),
		TargetCombatantID: target.ID.String(),
		ExpiresOn:         "start_of_turn",
	}
	if _, _, err := s.ApplyCondition(ctx, attacker.ID, cond); err != nil {
		return fmt.Errorf("applying vex advantage: %w", err)
	}
	return nil
}

// applySapDisadvantage applies a sap_disadvantage condition to the target it
// just hit. The condition lives on the target so that when the target later
// makes an attack (where it is the attacker), DetectAdvantage adds disadvantage.
// It uses the same single-shot/expiry convention as the reckless/help markers:
// it self-expires at the start of the sapped creature's next turn and the
// next attack spends it.
func (s *Service) applySapDisadvantage(ctx context.Context, _ refdata.Combatant, target refdata.Combatant) error {
	cond := CombatCondition{
		Condition:         "sap_disadvantage",
		DurationRounds:    1,
		SourceCombatantID: target.ID.String(),
		ExpiresOn:         "start_of_turn",
	}
	if _, _, err := s.ApplyCondition(ctx, target.ID, cond); err != nil {
		return fmt.Errorf("applying sap disadvantage: %w", err)
	}
	return nil
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
