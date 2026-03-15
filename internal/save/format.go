package save

import (
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/dice"
)

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
