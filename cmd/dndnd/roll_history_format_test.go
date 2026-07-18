package main

import (
	"testing"

	"github.com/ab/dndnd/internal/dice"
)

// TestFormatRollLogEntry_SelfContained pins the #roll-history line for a /check
// with an effect die to the approved, unambiguous single-total format:
//
//	Forge Anvilbearer — Sleight of Hand: d20(13) + 2 + 1d4(2) = 17
//
// The old rendering read "... `d20+2+1d4` = 13 + 2 = 15 +2 (1d4)" — two "="s
// and a total (15) that excluded the effect die.
func TestFormatRollLogEntry_SelfContained(t *testing.T) {
	entry := dice.RollLogEntry{
		Roller:        "Forge Anvilbearer",
		Purpose:       "Sleight of Hand",
		Expression:    "d20+2+1d4", // retained for audit; must NOT render
		Breakdown:     "d20(13) + 2 + 1d4(2) = 17",
		Total:         17,
		SelfContained: true,
	}

	got := formatRollLogEntry(entry)
	want := "Forge Anvilbearer — Sleight of Hand: d20(13) + 2 + 1d4(2) = 17"
	if got != want {
		t.Errorf("formatRollLogEntry() = %q, want %q", got, want)
	}
}

// TestFormatRollLogEntry_LegacyUnchanged guards the non-self-contained path
// (/roll, /attack) so it keeps the backtick-expression + "= breakdown" form.
func TestFormatRollLogEntry_LegacyUnchanged(t *testing.T) {
	entry := dice.RollLogEntry{
		Roller:     "Grukk",
		Purpose:    "attack",
		Expression: "1d20+4",
		Breakdown:  "15 + 4 = 19",
		Total:      19,
	}

	got := formatRollLogEntry(entry)
	want := "Grukk — attack `1d20+4` = 15 + 4 = 19"
	if got != want {
		t.Errorf("formatRollLogEntry() = %q, want %q", got, want)
	}
}
