package ddbimport

import (
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/character"
)

func TestFormatPreview(t *testing.T) {
	pc := &ParsedCharacter{
		Name:  "Thorin Ironforge",
		Race:  "Human",
		Level: 5,
		Classes: []character.ClassEntry{
			{Class: "Fighter", Subclass: "Champion", Level: 5},
		},
		AbilityScores: character.AbilityScores{
			STR: 16, DEX: 14, CON: 15, INT: 10, WIS: 12, CHA: 8,
		},
		HPMax:     44,
		HPCurrent: 39,
		AC:        18,
		SpeedFt:   30,
	}

	result := FormatPreview(pc)

	checks := []string{
		"Thorin Ironforge",
		"Human",
		"Fighter",
		"Champion",
		"Level 5",
		"HP: 39/44",
		"AC: 18",
		"STR: 16",
		"DEX: 14",
	}

	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("preview missing %q:\n%s", check, result)
		}
	}
}

func TestFormatPreview_MultiClass(t *testing.T) {
	pc := &ParsedCharacter{
		Name:  "Multi",
		Race:  "Half-Elf",
		Level: 5,
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 3},
			{Class: "Rogue", Level: 2},
		},
		AbilityScores: character.AbilityScores{STR: 14, DEX: 16, CON: 12, INT: 10, WIS: 10, CHA: 10},
		HPMax:         30,
		HPCurrent:     30,
		AC:            15,
	}

	result := FormatPreview(pc)
	if !strings.Contains(result, "Fighter 3") {
		t.Errorf("missing Fighter 3 in preview:\n%s", result)
	}
	if !strings.Contains(result, "Rogue 2") {
		t.Errorf("missing Rogue 2 in preview:\n%s", result)
	}
}

func TestFormatPreview_WithWarnings(t *testing.T) {
	pc := &ParsedCharacter{
		Name:  "Strong One",
		Race:  "Human",
		Level: 5,
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
		},
		AbilityScores: character.AbilityScores{STR: 22, DEX: 14, CON: 15, INT: 10, WIS: 12, CHA: 8},
		HPMax:         44,
		HPCurrent:     44,
		AC:            18,
	}

	warnings := []Warning{
		{Message: "STR 22 — exceeds 20 without a detected magic item source"},
	}

	result := FormatPreviewWithWarnings(pc, warnings)
	if !strings.Contains(result, "Warning") || !strings.Contains(result, "STR 22") {
		t.Errorf("preview should include warnings:\n%s", result)
	}
}
