package save

import (
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/dice"
)

// BonusFragment renders the effect-dice suffix (e.g. " +3 (1d4)") for the save
// response and #roll-history log, or "" when no bonus dice were rolled. Kept in
// one place so both call sites stay in sync (mirrors check.SingleCheckResult).
func (r SaveResult) BonusFragment() string {
	if r.BonusExpression == "" {
		return ""
	}
	return fmt.Sprintf(" +%d (%s)", r.BonusTotal, r.BonusExpression)
}

// FormatSaveResult formats a saving throw result for Discord display.
func FormatSaveResult(charName string, result SaveResult) string {
	abilityLabel := strings.ToUpper(result.Ability)

	if result.AutoFail {
		var b strings.Builder
		fmt.Fprintf(&b, "**%s** — %s Save: **Auto-fail**", charName, abilityLabel)
		for _, r := range result.ConditionReasons {
			fmt.Fprintf(&b, "\n> %s", r)
		}
		return b.String()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "**%s** — %s Save: **%d** (%s)", charName, abilityLabel, result.Total, result.D20Result.Breakdown)

	b.WriteString(result.BonusFragment())

	if result.D20Result.Mode != dice.Normal {
		fmt.Fprintf(&b, " [%s]", result.D20Result.Mode)
	}

	for _, r := range result.ConditionReasons {
		fmt.Fprintf(&b, "\n> %s", r)
	}

	for _, r := range result.FeatureReasons {
		fmt.Fprintf(&b, "\n> %s", r)
	}

	return b.String()
}
