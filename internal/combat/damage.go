package combat

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// ApplyDamageResistances applies resistance, immunity, and vulnerability rules
// to raw damage. Returns the final damage amount and a reason string.
// Rules:
//   - Immunity trumps all (damage = 0)
//   - Resistance + Vulnerability cancel out (normal damage)
//   - Resistance alone halves damage (rounded down)
//   - Vulnerability alone doubles damage
//   - Petrified condition grants resistance to all damage types
func ApplyDamageResistances(rawDamage int, damageType string, resistances, immunities, vulnerabilities []string, conditions []CombatCondition) (int, string) {
	dt := strings.ToLower(damageType)

	isImmune := containsCI(immunities, dt)
	if isImmune {
		return 0, "immune to " + dt
	}

	isResistant := containsCI(resistances, dt) || hasCondition(conditions, "petrified")
	isVulnerable := containsCI(vulnerabilities, dt)

	if isResistant && isVulnerable {
		return rawDamage, "resistance and vulnerability cancel"
	}
	if isResistant {
		return rawDamage / 2, "resistance to " + dt
	}
	if isVulnerable {
		return rawDamage * 2, "vulnerable to " + dt
	}

	return rawDamage, ""
}

// containsCI checks if the slice contains the target string (case-insensitive).
func containsCI(slice []string, target string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, target) {
			return true
		}
	}
	return false
}

// AbsorbTempHP applies damage to temporary HP first, returning remaining damage
// and new temp HP value.
func AbsorbTempHP(damage int, tempHP int) (remainingDamage int, newTempHP int) {
	if damage <= tempHP {
		return 0, tempHP - damage
	}
	return damage - tempHP, 0
}

// GrantTempHP returns the higher of current temp HP or new temp HP (no stacking).
func GrantTempHP(currentTempHP int, newTempHP int) int {
	if newTempHP > currentTempHP {
		return newTempHP
	}
	return currentTempHP
}

// ExhaustionEffectiveSpeed returns effective speed after exhaustion effects.
// Level 2: speed halved. Level 5+: speed reduced to 0.
func ExhaustionEffectiveSpeed(baseSpeed int, exhaustionLevel int) int {
	if exhaustionLevel >= 5 {
		return 0
	}
	if exhaustionLevel >= 2 {
		return baseSpeed / 2
	}
	return baseSpeed
}

// ExhaustionEffectiveMaxHP returns effective max HP after exhaustion effects.
// Level 4+: max HP halved.
func ExhaustionEffectiveMaxHP(baseMaxHP int, exhaustionLevel int) int {
	if exhaustionLevel >= 4 {
		return baseMaxHP / 2
	}
	return baseMaxHP
}

// ExhaustionRollEffect returns the roll effects from exhaustion.
// Level 1+: disadvantage on ability checks.
// Level 3+: disadvantage on attack rolls and saving throws.
func ExhaustionRollEffect(exhaustionLevel int) (checkDisadv bool, attackDisadv bool, saveDisadv bool) {
	if exhaustionLevel >= 3 {
		return true, true, true
	}
	if exhaustionLevel >= 1 {
		return true, false, false
	}
	return false, false, false
}

// IsExhaustedToDeath returns true if exhaustion level is 6 (death).
func IsExhaustedToDeath(exhaustionLevel int) bool {
	return exhaustionLevel >= 6
}

// CheckConditionImmunity returns true if the creature is immune to the given condition.
func CheckConditionImmunity(conditionName string, immunities []string) bool {
	return containsCI(immunities, conditionName)
}

// ApplyDamageInput holds inputs for the centralized damage pipeline. Every
// production damage call site funnels through Service.ApplyDamage with this
// struct so that Phase 42 (resistance / immunity / vulnerability, temp HP,
// exhaustion HP-halving and exhaustion-6 death) is uniformly applied before
// the underlying applyDamageHP write.
type ApplyDamageInput struct {
	EncounterID uuid.UUID
	Target      refdata.Combatant
	RawDamage   int
	DamageType  string
	// Override skips R/I/V + temp HP absorption. Used for DM dashboard
	// overrides and undo paths where the caller is asserting an exact HP
	// outcome (or restoring a previous state) and per-target damage typing
	// is not meaningful.
	Override bool
}

// ApplyDamageResult holds outputs of Service.ApplyDamage.
type ApplyDamageResult struct {
	Updated      refdata.Combatant
	FinalDamage  int
	AbsorbedTemp int
	NewTempHP    int32
	NewHP        int32
	IsAlive      bool
	Reason       string
	// Killed is true when the damage event triggered exhaustion-6 death.
	Killed bool
}

// ApplyDamage is the single seam for every production damage write. It
// resolves the target's resistances / immunities / vulnerabilities, runs
// temp-HP absorption, applies exhaustion HP-halving (level 4+) and
// exhaustion-6 death, then funnels the resulting HP delta through
// applyDamageHP so concentration saves and unconscious-at-0-HP still fire.
//
// For PCs, R/I/V default to empty (the PC schema does not yet carry
// per-character damage modifiers — race/feat-driven resistances will plumb
// through here once Phase 45 wires them). For NPCs, R/I/V come from the
// referenced creature row.
//
// Override=true short-circuits R/I/V + temp HP and treats RawDamage as the
// exact HP delta. This is used by DM dashboard overrides and undo paths.
func (s *Service) ApplyDamage(ctx context.Context, input ApplyDamageInput) (ApplyDamageResult, error) {
	if input.RawDamage < 0 {
		return ApplyDamageResult{}, errors.New("ApplyDamage: RawDamage must be non-negative")
	}

	target := input.Target

	// Exhaustion 6 = instant death regardless of incoming damage.
	if IsExhaustedToDeath(int(target.ExhaustionLevel)) {
		updated, err := s.applyDamageHP(ctx, input.EncounterID, target.ID, target.HpCurrent, 0, 0, false)
		if err != nil {
			return ApplyDamageResult{}, fmt.Errorf("ApplyDamage exhaustion-6 kill: %w", err)
		}
		return ApplyDamageResult{
			Updated:     updated,
			FinalDamage: int(target.HpCurrent),
			NewHP:       0,
			NewTempHP:   0,
			IsAlive:     false,
			Reason:      "exhaustion 6: dead",
			Killed:      true,
		}, nil
	}

	// Cap currentHP to the exhaustion-effective max (level 4+ halves max HP).
	// The cap only applies when exhaustion is in the halving band; we never
	// rewrite HP based on HpMax outside that case (test fixtures may leave
	// HpMax=0 without expecting a cap).
	currentHP := int(target.HpCurrent)
	if int(target.ExhaustionLevel) >= 4 {
		effectiveMaxHP := ExhaustionEffectiveMaxHP(int(target.HpMax), int(target.ExhaustionLevel))
		if currentHP > effectiveMaxHP {
			currentHP = effectiveMaxHP
		}
	}

	adjusted := input.RawDamage
	reason := ""
	tempHP := int(target.TempHp)
	absorbed := 0

	if !input.Override {
		resistances, immunities, vulnerabilities, conditions, err := s.resolveDamageProfile(ctx, target)
		if err != nil {
			return ApplyDamageResult{}, fmt.Errorf("ApplyDamage resolving profile: %w", err)
		}
		adjusted, reason = ApplyDamageResistances(input.RawDamage, input.DamageType, resistances, immunities, vulnerabilities, conditions)
		var remaining int
		remaining, tempHP = AbsorbTempHP(adjusted, tempHP)
		absorbed = adjusted - remaining
		adjusted = remaining
	}

	newHP := currentHP - adjusted
	if newHP < 0 {
		newHP = 0
	}

	// PCs at 0 HP go unconscious + dying (still alive); NPCs die outright.
	isAlive := newHP > 0 || !target.IsNpc

	updated, err := s.applyDamageHP(ctx, input.EncounterID, target.ID, target.HpCurrent, int32(newHP), int32(tempHP), isAlive)
	if err != nil {
		return ApplyDamageResult{}, fmt.Errorf("ApplyDamage persisting HP: %w", err)
	}

	return ApplyDamageResult{
		Updated:      updated,
		FinalDamage:  adjusted,
		AbsorbedTemp: absorbed,
		NewTempHP:    int32(tempHP),
		NewHP:        int32(newHP),
		IsAlive:      isAlive,
		Reason:       reason,
	}, nil
}

// resolveDamageProfile returns the target's R/I/V slices and parsed conditions.
// PCs default to empty R/I/V (not yet schema-tracked); NPCs pull from the
// referenced creature row, falling back to empty on missing creature.
func (s *Service) resolveDamageProfile(ctx context.Context, target refdata.Combatant) ([]string, []string, []string, []CombatCondition, error) {
	conditions, err := ListConditions(target.Conditions)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("parsing conditions: %w", err)
	}
	if !target.IsNpc || !target.CreatureRefID.Valid {
		return nil, nil, nil, conditions, nil
	}
	creature, err := s.store.GetCreature(ctx, target.CreatureRefID.String)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, nil, conditions, nil
		}
		return nil, nil, nil, nil, fmt.Errorf("looking up creature %s: %w", target.CreatureRefID.String, err)
	}
	return creature.DamageResistances, creature.DamageImmunities, creature.DamageVulnerabilities, conditions, nil
}
