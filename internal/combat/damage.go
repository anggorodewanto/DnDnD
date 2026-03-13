package combat

import "strings"

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
