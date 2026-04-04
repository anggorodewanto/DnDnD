package ddbimport

import (
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/character"
)

func TestGenerateDiff_NoDifferences(t *testing.T) {
	a := validCharacter()
	b := validCharacter()
	diff := GenerateDiff(a, b)
	if len(diff) != 0 {
		t.Errorf("expected no differences, got %v", diff)
	}
}

func TestGenerateDiff_NameChanged(t *testing.T) {
	old := validCharacter()
	new := validCharacter()
	new.Name = "New Name"
	diff := GenerateDiff(old, new)
	found := false
	for _, d := range diff {
		if strings.Contains(d, "Name") && strings.Contains(d, "New Name") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected name change in diff, got %v", diff)
	}
}

func TestGenerateDiff_LevelChanged(t *testing.T) {
	old := validCharacter()
	new := validCharacter()
	new.Level = 6
	new.Classes = []character.ClassEntry{{Class: "Fighter", Level: 6}}
	diff := GenerateDiff(old, new)
	found := false
	for _, d := range diff {
		if strings.Contains(d, "Level") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected level change in diff, got %v", diff)
	}
}

func TestGenerateDiff_AbilityScoreChanged(t *testing.T) {
	old := validCharacter()
	new := validCharacter()
	new.AbilityScores.STR = 18
	diff := GenerateDiff(old, new)
	found := false
	for _, d := range diff {
		if strings.Contains(d, "STR") && strings.Contains(d, "18") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected STR change in diff, got %v", diff)
	}
}

func TestGenerateDiff_HPChanged(t *testing.T) {
	old := validCharacter()
	new := validCharacter()
	new.HPMax = 50
	diff := GenerateDiff(old, new)
	found := false
	for _, d := range diff {
		if strings.Contains(d, "HP Max") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected HP Max change in diff, got %v", diff)
	}
}

func TestGenerateDiff_ACChanged(t *testing.T) {
	old := validCharacter()
	new := validCharacter()
	new.AC = 20
	diff := GenerateDiff(old, new)
	found := false
	for _, d := range diff {
		if strings.Contains(d, "AC") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected AC change in diff, got %v", diff)
	}
}

func TestGenerateDiff_MultipleChanges(t *testing.T) {
	old := validCharacter()
	new := validCharacter()
	new.Level = 6
	new.HPMax = 55
	new.AC = 19
	diff := GenerateDiff(old, new)
	if len(diff) < 3 {
		t.Errorf("expected at least 3 changes, got %d: %v", len(diff), diff)
	}
}

func TestFormatDiff(t *testing.T) {
	changes := []string{"Level: 5 -> 6", "HP Max: 44 -> 55"}
	result := FormatDiff(changes)
	if !strings.Contains(result, "Level: 5 -> 6") {
		t.Errorf("formatted diff missing change: %s", result)
	}
}

func TestFormatDiff_Empty(t *testing.T) {
	result := FormatDiff(nil)
	if !strings.Contains(result, "No changes") {
		t.Errorf("expected 'No changes' message, got: %s", result)
	}
}
