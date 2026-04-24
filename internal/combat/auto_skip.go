package combat

import "fmt"

// FormatAutoSkipMessage produces the combat log message when a combatant's
// turn is auto-skipped.
//
// Phase 114 \u2014 the "surprised" condition uses the spec-mandated shape
// "\u23ed\ufe0f <name> is surprised \u2014 turn skipped", while other incapacitating
// conditions (stunned, paralyzed, \u2026) keep the pre-existing
// "\u23ed\ufe0f  <name>'s turn is auto-skipped (<cond> \u2014 can't take actions)" format
// that downstream tests and telemetry already depend on.
func FormatAutoSkipMessage(displayName string, conditionName string) string {
	if conditionName == "surprised" {
		return fmt.Sprintf("\u23ed\ufe0f %s is surprised \u2014 turn skipped", displayName)
	}
	return fmt.Sprintf("\u23ed\ufe0f  %s's turn is auto-skipped (%s \u2014 can't take actions)", displayName, conditionName)
}

// GetIncapacitatingConditionName returns the name of the first incapacitating
// condition found in the slice, or empty string if none.
func GetIncapacitatingConditionName(conds []CombatCondition) string {
	for _, c := range conds {
		if incapacitatingConditions[c.Condition] {
			return c.Condition
		}
	}
	return ""
}
