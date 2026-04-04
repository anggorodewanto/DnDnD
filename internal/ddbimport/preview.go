package ddbimport

import (
	"fmt"
	"strings"
)

// FormatPreview generates a human-readable preview of an imported character.
func FormatPreview(pc *ParsedCharacter) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("**%s** — %s\n", pc.Name, pc.Race))
	b.WriteString(fmt.Sprintf("Level %d — %s\n", pc.Level, formatClassList(pc)))

	// HP and AC
	b.WriteString(fmt.Sprintf("HP: %d/%d | AC: %d | Speed: %d ft\n", pc.HPCurrent, pc.HPMax, pc.AC, pc.SpeedFt))

	// Ability scores
	b.WriteString(fmt.Sprintf("STR: %d | DEX: %d | CON: %d | INT: %d | WIS: %d | CHA: %d\n",
		pc.AbilityScores.STR, pc.AbilityScores.DEX, pc.AbilityScores.CON,
		pc.AbilityScores.INT, pc.AbilityScores.WIS, pc.AbilityScores.CHA))

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
		b.WriteString(fmt.Sprintf("- %s\n", w.Message))
	}

	return b.String()
}
