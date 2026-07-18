package dice

import (
	"fmt"
	"strings"
)

// FormatValuedBreakdown renders a self-contained d20 roll breakdown: each die is
// named with its rolled value and the line ends in a single grand total, e.g.
//
//	d20(13) + 2 + 1d4(2) = 17
//
// Advantage/disadvantage rolls show both faces and the kept one:
//
//	d20(18/4→18) + 2 = 20
//
// The displayed modifier comes from the d20's own Modifier, which already folds
// in proficiency, feature bonuses, and the 2024 exhaustion penalty, so the
// arithmetic always closes: chosen + d20.Modifier + bonusTotal == grandTotal.
// A zero modifier is omitted; an empty bonusExpr drops the effect-die term.
//
// This replaces the old "<d20 breakdown> <bonus fragment>" concatenation for
// #roll-history, whose trailing " +N (die)" printed AFTER the d20's own
// "= total" and so implied a total that excluded the effect die.
func FormatValuedBreakdown(d20 D20Result, bonusExpr string, bonusTotal, grandTotal int) string {
	var b strings.Builder

	b.WriteString("d20(")
	if len(d20.Rolls) > 1 {
		fmt.Fprintf(&b, "%d/%d→%d", d20.Rolls[0], d20.Rolls[1], d20.Chosen)
	} else {
		fmt.Fprintf(&b, "%d", d20.Chosen)
	}
	b.WriteByte(')')

	if d20.Modifier > 0 {
		fmt.Fprintf(&b, " + %d", d20.Modifier)
	} else if d20.Modifier < 0 {
		fmt.Fprintf(&b, " - %d", -d20.Modifier)
	}

	if bonusExpr != "" {
		fmt.Fprintf(&b, " + %s(%d)", bonusExpr, bonusTotal)
	}

	fmt.Fprintf(&b, " = %d", grandTotal)
	return b.String()
}
