package combat

import "fmt"

// FormatAutoSkipMessage produces the combat log message when a combatant's
// turn is auto-skipped due to being incapacitated.
func FormatAutoSkipMessage(displayName string, conditionName string) string {
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
