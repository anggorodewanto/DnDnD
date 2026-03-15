package check

import (
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// titleCase capitalizes the first letter of each word.
func titleCase(s string) string {
	return cases.Title(language.English).String(s)
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

	if result.D20Result.Mode != 0 {
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
