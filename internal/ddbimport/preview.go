package ddbimport

import (
	"fmt"
	"strings"
)

// FormatPreview generates a human-readable preview of an imported character.
func FormatPreview(pc *ParsedCharacter) string {
	var b strings.Builder

	fmt.Fprintf(&b, "**%s** — %s\n", pc.Name, pc.Race)
	fmt.Fprintf(&b, "Level %d — %s\n", pc.Level, formatClassList(pc))

	// HP and AC
	fmt.Fprintf(&b, "HP: %d/%d | AC: %d | Speed: %d ft\n", pc.HPCurrent, pc.HPMax, pc.AC, pc.SpeedFt)

	// Ability scores
	fmt.Fprintf(&b, "STR: %d | DEX: %d | CON: %d | INT: %d | WIS: %d | CHA: %d\n",
		pc.AbilityScores.STR, pc.AbilityScores.DEX, pc.AbilityScores.CON,
		pc.AbilityScores.INT, pc.AbilityScores.WIS, pc.AbilityScores.CHA)

	return b.String()
}

// FormatPreviewWithWarnings generates a preview with advisory warnings appended.
func FormatPreviewWithWarnings(pc *ParsedCharacter, warnings []Warning) string {
	preview := FormatPreview(pc)

	if len(warnings) == 0 {
		return preview
	}

	var b strings.Builder
	b.WriteString(preview)
	b.WriteString("\n**Warnings:**\n")
	for _, w := range warnings {
		fmt.Fprintf(&b, "- %s\n", w.Message)
	}

	return b.String()
}
