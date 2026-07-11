package check

import (
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/ab/dndnd/internal/dice"
)

// titleCase capitalizes the first letter of each word.
func titleCase(s string) string {
	return cases.Title(language.English).String(s)
}

// BonusFragment renders the effect-dice suffix (e.g. " +3 (1d4)") shared by the
// Discord check response and the #roll-history log breakdown, or "" when no
// bonus dice were rolled. Both call sites format the fragment identically but
// only the response is test-pinned, so keeping the string in one place stops
// the log breakdown from silently drifting.
func (r SingleCheckResult) BonusFragment() string {
	if r.BonusExpression == "" {
		return ""
	}
	return fmt.Sprintf(" +%d (%s)", r.BonusTotal, r.BonusExpression)
}

// FormatSingleCheckResult formats a single check result for Discord display.
func FormatSingleCheckResult(charName string, result SingleCheckResult) string {
	skillLabel := titleCase(result.Skill)

	if result.AutoFail {
		var b strings.Builder
		fmt.Fprintf(&b, "**%s** — %s Check: **Auto-fail**", charName, skillLabel)
		for _, r := range result.ConditionReasons {
			fmt.Fprintf(&b, "\n> %s", r)
		}
		return b.String()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "**%s** — %s Check: **%d** (%s)", charName, skillLabel, result.Total, result.D20Result.Breakdown)

	b.WriteString(result.BonusFragment())

	if result.D20Result.Mode != dice.Normal {
		fmt.Fprintf(&b, " [%s]", result.D20Result.Mode)
	}

	for _, r := range result.ConditionReasons {
		fmt.Fprintf(&b, "\n> %s", r)
	}

	return b.String()
}

// FormatGroupCheckResult formats a group check result for Discord display.
func FormatGroupCheckResult(skill string, result GroupCheckResult) string {
	skillLabel := titleCase(skill)
	total := len(result.Results)

	var b strings.Builder
	outcome := "FAILURE"
	if result.Success {
		outcome = "SUCCESS"
	}
	fmt.Fprintf(&b, "**Group %s Check (DC %d): %s** (%d/%d passed)", skillLabel, result.DC, outcome, result.Passed, total)

	for _, r := range result.Results {
		mark := "FAIL"
		if r.Passed {
			mark = "PASS"
		}
		fmt.Fprintf(&b, "\n> %s: %d [%s]", r.Name, r.D20.Total, mark)
	}

	return b.String()
}

// BonusFragment renders the initiator's effect-dice suffix (e.g. " +3 (1d4)")
// for the contested-check response and #roll-history log, or "" when the
// initiator rolled no bonus dice. Mirrors SingleCheckResult.BonusFragment so the
// contested and single paths format the fragment identically. Only the initiator
// can take a bonus (Guidance applies to the roller's own check).
func (r ContestedCheckResult) BonusFragment() string {
	if r.InitiatorBonusExpression == "" {
		return ""
	}
	return fmt.Sprintf(" +%d (%s)", r.InitiatorBonusTotal, r.InitiatorBonusExpression)
}

// FormatContestedCheckResult formats a contested check result for Discord display.
func FormatContestedCheckResult(skill, initiatorName, opponentName string, result ContestedCheckResult) string {
	skillLabel := titleCase(skill)

	var b strings.Builder
	fmt.Fprintf(&b, "**Contested %s Check**\n", skillLabel)
	fmt.Fprintf(&b, "> %s: %d\n", initiatorName, result.InitiatorTotal)
	fmt.Fprintf(&b, "> %s: %d\n", opponentName, result.OpponentTotal)

	if result.Tie {
		b.WriteString("**Result: Tie** (status quo maintained)")
	} else {
		fmt.Fprintf(&b, "**%s wins!**", result.Winner)
	}

	return b.String()
}
