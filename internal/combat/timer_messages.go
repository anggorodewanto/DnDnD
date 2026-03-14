package combat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ab/dndnd/internal/refdata"
)

// FormatNudgeMessage produces the 50% nudge message for a turn timer.
func FormatNudgeMessage(combatantName string, timeRemaining time.Duration) string {
	return fmt.Sprintf("\u23f0 @%s \u2014 it's still your turn! %s remaining. Use /recap to catch up.", combatantName, formatDuration(timeRemaining))
}

// FormatTacticalSummary produces the 75% warning with HP, AC, conditions,
// resources, adjacent enemies.
func FormatTacticalSummary(combatant refdata.Combatant, turn refdata.Turn, adjacentEnemies []refdata.Combatant, timeRemaining time.Duration) string {
	var b strings.Builder

	fmt.Fprintf(&b, "\u26a0\ufe0f @%s \u2014 your turn will be skipped in %s!\n", combatant.DisplayName, formatDuration(timeRemaining))
	fmt.Fprintf(&b, "\u2764\ufe0f HP: %d/%d | \U0001f6e1\ufe0f AC: %d", combatant.HpCurrent, combatant.HpMax, combatant.Ac)

	condStrs := formatConditionNames(combatant.Conditions)
	if len(condStrs) > 0 {
		fmt.Fprintf(&b, " | \u26a0\ufe0f Conditions: %s", strings.Join(condStrs, ", "))
	}
	b.WriteString("\n")

	resources := BuildResourceListWithInspiration(turn, combatant)
	if len(resources) > 0 {
		fmt.Fprintf(&b, "\U0001f4cb Available: %s\n", strings.Join(resources, " | "))
	}

	if len(adjacentEnemies) > 0 {
		var enemyNames []string
		for _, e := range adjacentEnemies {
			enemyNames = append(enemyNames, fmt.Sprintf("%s (%s)", e.DisplayName, e.ShortID))
		}
		fmt.Fprintf(&b, "\U0001f3af Adjacent enemies: %s\n", strings.Join(enemyNames, ", "))
	}

	b.WriteString("\U0001f4a1 Check #combat-map for current positions.")
	return b.String()
}

// formatDuration returns a human-friendly duration string.
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 && minutes > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return "< 1m"
}

// formatConditionNames extracts condition names from JSONB conditions array.
func formatConditionNames(raw json.RawMessage) []string {
	conds, err := parseConditions(raw)
	if err != nil {
		return nil
	}
	var names []string
	for _, c := range conds {
		names = append(names, capitalize(c.Condition))
	}
	return names
}

// capitalize returns the string with the first letter uppercased.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
