package ddbimport

import (
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/character"
)

func validCharacter() *ParsedCharacter {
	return &ParsedCharacter{
		Name:  "Test Character",
		Race:  "Human",
		Level: 5,
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
		},
		AbilityScores: character.AbilityScores{
			STR: 16, DEX: 14, CON: 15, INT: 10, WIS: 12, CHA: 8,
		},
		HPMax:     44,
		HPCurrent: 39,
		AC:        18,
	}
}

func TestValidate_ValidCharacter(t *testing.T) {
	warnings, err := Validate(validCharacter())
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestValidate_MissingName(t *testing.T) {
	c := validCharacter()
	c.Name = ""
	_, err := Validate(c)
	if err == nil {
		t.Fatal("expected structural error for missing name")
	}
}

func TestValidate_LevelOutOfRange(t *testing.T) {
	tests := []struct {
		name  string
		level int
	}{
		{"level 0", 0},
		{"level 21", 21},
		{"level -1", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validCharacter()
			c.Level = tt.level
			_, err := Validate(c)
			if err == nil {
				t.Fatalf("expected structural error for level %d", tt.level)
			}
		})
	}
}

func TestValidate_AbilityScoreOutOfRange(t *testing.T) {
	tests := []struct {
		name     string
		modifier func(*ParsedCharacter)
	}{
		{"STR 0", func(c *ParsedCharacter) { c.AbilityScores.STR = 0 }},
		{"DEX 31", func(c *ParsedCharacter) { c.AbilityScores.DEX = 31 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validCharacter()
			tt.modifier(c)
			_, err := Validate(c)
			if err == nil {
				t.Fatal("expected structural error for ability score out of range")
			}
		})
	}
}

func TestValidate_HPZero(t *testing.T) {
	c := validCharacter()
	c.HPMax = 0
	_, err := Validate(c)
	if err == nil {
		t.Fatal("expected structural error for HP max 0")
	}
}

func TestValidate_NoClasses(t *testing.T) {
	c := validCharacter()
	c.Classes = nil
	_, err := Validate(c)
	if err == nil {
		t.Fatal("expected structural error for no classes")
	}
}

func TestValidate_Warning_HighAbilityScore(t *testing.T) {
	c := validCharacter()
	c.AbilityScores.STR = 22
	warnings, err := Validate(c)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for STR 22")
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w.Message, "STR 22") && strings.Contains(w.Message, "exceeds 20") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about STR 22 exceeding 20, got %v", warnings)
	}
}

func TestValidate_Warning_MulticlassPrereq(t *testing.T) {
	c := validCharacter()
	c.Classes = []character.ClassEntry{
		{Class: "Fighter", Level: 3},
		{Class: "Paladin", Level: 2},
	}
	c.AbilityScores.CHA = 10 // Below 13 minimum for Paladin
	c.AbilityScores.STR = 13
	warnings, err := Validate(c)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w.Message, "Paladin") && strings.Contains(w.Message, "CHA") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about Paladin CHA prereq, got %v", warnings)
	}
}

func TestValidate_Warning_AttunementLimit(t *testing.T) {
	c := validCharacter()
	c.Inventory = []character.InventoryItem{
		{Name: "Ring 1", IsAttuned: true, RequiresAttunement: true, IsMagic: true},
		{Name: "Ring 2", IsAttuned: true, RequiresAttunement: true, IsMagic: true},
		{Name: "Ring 3", IsAttuned: true, RequiresAttunement: true, IsMagic: true},
		{Name: "Ring 4", IsAttuned: true, RequiresAttunement: true, IsMagic: true},
	}
	warnings, err := Validate(c)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w.Message, "attuned") || strings.Contains(w.Message, "attunement") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about attunement limit, got %v", warnings)
	}
}

func TestValidate_Warning_OffListWizardSpell(t *testing.T) {
	c := validCharacter()
	c.Classes = []character.ClassEntry{{Class: "Wizard", Level: 3}}
	c.Spells = []SpellEntry{{Name: "Cure Wounds", Level: 1, Source: "class", OffList: true, Homebrew: true}}

	warnings, err := Validate(c)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}

	found := false
	for _, w := range warnings {
		if strings.Contains(w.Message, "Wizard spell list includes Cure Wounds") &&
			strings.Contains(w.Message, "not on Wizard spell list") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected off-list Cure Wounds warning, got %v", warnings)
	}
}

func TestValidate_NilCharacter(t *testing.T) {
	_, err := Validate(nil)
	if err == nil {
		t.Fatal("expected error for nil character")
	}
}
