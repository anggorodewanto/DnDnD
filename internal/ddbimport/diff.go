package ddbimport

import (
	"fmt"
	"strings"
)

// GenerateDiff compares two ParsedCharacters and returns a list of human-readable changes.
func GenerateDiff(old, new *ParsedCharacter) []string {
	var changes []string

	if old.Name != new.Name {
		changes = append(changes, fmt.Sprintf("Name: %s -> %s", old.Name, new.Name))
	}
	if old.Race != new.Race {
		changes = append(changes, fmt.Sprintf("Race: %s -> %s", old.Race, new.Race))
	}
	if old.Level != new.Level {
		changes = append(changes, fmt.Sprintf("Level: %d -> %d", old.Level, new.Level))
	}

	// Classes
	oldClasses := formatClassList(old)
	newClasses := formatClassList(new)
	if oldClasses != newClasses {
		changes = append(changes, fmt.Sprintf("Classes: %s -> %s", oldClasses, newClasses))
	}

	// Ability scores
	for _, s := range []struct {
		name     string
		oldVal   int
		newVal   int
	}{
		{"STR", old.AbilityScores.STR, new.AbilityScores.STR},
		{"DEX", old.AbilityScores.DEX, new.AbilityScores.DEX},
		{"CON", old.AbilityScores.CON, new.AbilityScores.CON},
		{"INT", old.AbilityScores.INT, new.AbilityScores.INT},
		{"WIS", old.AbilityScores.WIS, new.AbilityScores.WIS},
		{"CHA", old.AbilityScores.CHA, new.AbilityScores.CHA},
	} {
		if s.oldVal != s.newVal {
			changes = append(changes, fmt.Sprintf("%s: %d -> %d", s.name, s.oldVal, s.newVal))
		}
	}

	if old.HPMax != new.HPMax {
		changes = append(changes, fmt.Sprintf("HP Max: %d -> %d", old.HPMax, new.HPMax))
	}
	if old.HPCurrent != new.HPCurrent {
		changes = append(changes, fmt.Sprintf("HP Current: %d -> %d", old.HPCurrent, new.HPCurrent))
	}
	if old.AC != new.AC {
		changes = append(changes, fmt.Sprintf("AC: %d -> %d", old.AC, new.AC))
	}
	if old.SpeedFt != new.SpeedFt {
		changes = append(changes, fmt.Sprintf("Speed: %d -> %d ft", old.SpeedFt, new.SpeedFt))
	}
	if old.Gold != new.Gold {
		changes = append(changes, fmt.Sprintf("Gold: %d -> %d", old.Gold, new.Gold))
	}

	return changes
}

func formatClassList(pc *ParsedCharacter) string {
	var parts []string
	for _, c := range pc.Classes {
		if c.Subclass != "" {
			parts = append(parts, fmt.Sprintf("%s (%s) %d", c.Class, c.Subclass, c.Level))
		} else {
			parts = append(parts, fmt.Sprintf("%s %d", c.Class, c.Level))
		}
	}
	return strings.Join(parts, " / ")
}

// FormatDiff formats a list of changes into a readable message.
func FormatDiff(changes []string) string {
	if len(changes) == 0 {
		return "No changes detected."
	}

	var b strings.Builder
	b.WriteString("**Changes detected:**\n")
	for _, c := range changes {
		b.WriteString(fmt.Sprintf("- %s\n", c))
	}
	return b.String()
}
