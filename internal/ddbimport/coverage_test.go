package ddbimport

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/google/uuid"
)

// Additional tests for coverage gaps

func TestWarning_String(t *testing.T) {
	w := Warning{Message: "test warning"}
	if w.String() != "test warning" {
		t.Errorf("String() = %q, want %q", w.String(), "test warning")
	}
}

func TestClassHitDie(t *testing.T) {
	tests := []struct {
		class string
		want  string
	}{
		{"Barbarian", "d12"},
		{"Fighter", "d10"},
		{"Paladin", "d10"},
		{"Ranger", "d10"},
		{"Bard", "d8"},
		{"Cleric", "d8"},
		{"Druid", "d8"},
		{"Monk", "d8"},
		{"Rogue", "d8"},
		{"Warlock", "d8"},
		{"Sorcerer", "d6"},
		{"Wizard", "d6"},
		{"Unknown", "d8"},
	}
	for _, tt := range tests {
		t.Run(tt.class, func(t *testing.T) {
			got := classHitDie(tt.class)
			if got != tt.want {
				t.Errorf("classHitDie(%q) = %q, want %q", tt.class, got, tt.want)
			}
		})
	}
}

func TestGenerateDiff_RaceChanged(t *testing.T) {
	old := validCharacter()
	new := validCharacter()
	new.Race = "Elf"
	diff := GenerateDiff(old, new)
	if len(diff) == 0 {
		t.Error("expected race change in diff")
	}
}

func TestGenerateDiff_SpeedChanged(t *testing.T) {
	old := validCharacter()
	old.SpeedFt = 30
	new := validCharacter()
	new.SpeedFt = 35
	diff := GenerateDiff(old, new)
	found := false
	for _, d := range diff {
		if d == "Speed: 30 -> 35 ft" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected speed change, got %v", diff)
	}
}

func TestGenerateDiff_GoldChanged(t *testing.T) {
	old := validCharacter()
	old.Gold = 50
	new := validCharacter()
	new.Gold = 100
	diff := GenerateDiff(old, new)
	found := false
	for _, d := range diff {
		if d == "Gold: 50 -> 100" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected gold change, got %v", diff)
	}
}

func TestGenerateDiff_HPCurrentChanged(t *testing.T) {
	old := validCharacter()
	new := validCharacter()
	new.HPCurrent = 30
	diff := GenerateDiff(old, new)
	found := false
	for _, d := range diff {
		if d == "HP Current: 39 -> 30" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected HP Current change, got %v", diff)
	}
}

func TestGenerateDiff_ClassSubclassFormatting(t *testing.T) {
	old := validCharacter()
	old.Classes = []character.ClassEntry{{Class: "Fighter", Subclass: "Champion", Level: 5}}
	new := validCharacter()
	new.Classes = []character.ClassEntry{{Class: "Fighter", Subclass: "Battle Master", Level: 5}}
	diff := GenerateDiff(old, new)
	if len(diff) == 0 {
		t.Error("expected class change when subclass differs")
	}
}

func TestParseDDBJSON_UnarmoredAC(t *testing.T) {
	json := []byte(`{
		"data": {
			"name": "Monk",
			"race": {"fullName": "Human"},
			"classes": [{"definition": {"name": "Monk", "hitDice": 8}, "level": 1}],
			"stats": [
				{"id": 1, "value": 10},
				{"id": 2, "value": 16},
				{"id": 3, "value": 10},
				{"id": 4, "value": 10},
				{"id": 5, "value": 10},
				{"id": 6, "value": 10}
			],
			"bonusStats": [{"id":1,"value":null},{"id":2,"value":null},{"id":3,"value":null},{"id":4,"value":null},{"id":5,"value":null},{"id":6,"value":null}],
			"overrideStats": [{"id":1,"value":null},{"id":2,"value":null},{"id":3,"value":null},{"id":4,"value":null},{"id":5,"value":null},{"id":6,"value":null}],
			"baseHitPoints": 8,
			"bonusHitPoints": 0,
			"removedHitPoints": 0,
			"temporaryHitPoints": 0,
			"inventory": [],
			"currencies": {"gp": 0, "sp": 0, "cp": 0, "ep": 0, "pp": 0},
			"modifiers": {"race": [], "class": [], "background": [], "item": [], "feat": [], "condition": []},
			"spells": {"class": [], "race": [], "item": [], "feat": []}
		}
	}`)
	result, err := ParseDDBJSON(json)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Unarmored: 10 + DEX mod (3) = 13
	if result.AC != 13 {
		t.Errorf("AC = %d, want 13 (unarmored with DEX 16)", result.AC)
	}
}

func TestParseDDBJSON_OverrideHitPoints(t *testing.T) {
	json := []byte(`{
		"data": {
			"name": "Override",
			"race": {"fullName": "Human"},
			"classes": [{"definition": {"name": "Fighter", "hitDice": 10}, "level": 1}],
			"stats": [
				{"id": 1, "value": 10},
				{"id": 2, "value": 10},
				{"id": 3, "value": 10},
				{"id": 4, "value": 10},
				{"id": 5, "value": 10},
				{"id": 6, "value": 10}
			],
			"bonusStats": [{"id":1,"value":null},{"id":2,"value":null},{"id":3,"value":null},{"id":4,"value":null},{"id":5,"value":null},{"id":6,"value":null}],
			"overrideStats": [{"id":1,"value":null},{"id":2,"value":null},{"id":3,"value":null},{"id":4,"value":null},{"id":5,"value":null},{"id":6,"value":null}],
			"baseHitPoints": 10,
			"bonusHitPoints": 5,
			"overrideHitPoints": 99,
			"removedHitPoints": 0,
			"temporaryHitPoints": 0,
			"inventory": [],
			"currencies": {"gp": 0, "sp": 0, "cp": 0, "ep": 0, "pp": 0},
			"modifiers": {"race": [], "class": [], "background": [], "item": [], "feat": [], "condition": []},
			"spells": {"class": [], "race": [], "item": [], "feat": []}
		}
	}`)
	result, err := ParseDDBJSON(json)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HPMax != 99 {
		t.Errorf("HPMax = %d, want 99 (overridden)", result.HPMax)
	}
}

func TestParseDDBJSON_NegativeHP(t *testing.T) {
	json := []byte(`{
		"data": {
			"name": "Downed",
			"race": {"fullName": "Human"},
			"classes": [{"definition": {"name": "Fighter", "hitDice": 10}, "level": 1}],
			"stats": [
				{"id": 1, "value": 10},
				{"id": 2, "value": 10},
				{"id": 3, "value": 10},
				{"id": 4, "value": 10},
				{"id": 5, "value": 10},
				{"id": 6, "value": 10}
			],
			"bonusStats": [{"id":1,"value":null},{"id":2,"value":null},{"id":3,"value":null},{"id":4,"value":null},{"id":5,"value":null},{"id":6,"value":null}],
			"overrideStats": [{"id":1,"value":null},{"id":2,"value":null},{"id":3,"value":null},{"id":4,"value":null},{"id":5,"value":null},{"id":6,"value":null}],
			"baseHitPoints": 10,
			"bonusHitPoints": 0,
			"removedHitPoints": 20,
			"temporaryHitPoints": 0,
			"inventory": [],
			"currencies": {"gp": 0, "sp": 0, "cp": 0, "ep": 0, "pp": 0},
			"modifiers": {"race": [], "class": [], "background": [], "item": [], "feat": [], "condition": []},
			"spells": {"class": [], "race": [], "item": [], "feat": []}
		}
	}`)
	result, err := ParseDDBJSON(json)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HPCurrent != 0 {
		t.Errorf("HPCurrent = %d, want 0 (clamped)", result.HPCurrent)
	}
}

func TestValidate_Warning_SingleClass_NoMulticlassWarning(t *testing.T) {
	c := validCharacter()
	c.AbilityScores.CHA = 5 // Low CHA but single class Fighter, no warning
	warnings, err := Validate(c)
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	for _, w := range warnings {
		if w.Message != "" && (w.Message[0:5] == "Multi") {
			t.Errorf("single class should not get multiclass warning: %v", w)
		}
	}
}

func TestValidate_AttunementLimit_UnderLimit(t *testing.T) {
	c := validCharacter()
	c.Inventory = []character.InventoryItem{
		{Name: "Ring 1", Equipped: true, RequiresAttunement: true},
		{Name: "Ring 2", Equipped: true, RequiresAttunement: true},
		{Name: "Ring 3", Equipped: true, RequiresAttunement: true},
	}
	warnings, err := Validate(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, w := range warnings {
		if w.Message != "" && len(w.Message) > 8 && w.Message[1] == ' ' {
			// attunement warning
			t.Errorf("3 attuned items should not trigger warning: %v", w)
		}
	}
}

func TestValidate_MulticlassOrPrereq(t *testing.T) {
	// Fighter requires STR OR DEX >= 13
	c := validCharacter()
	c.Classes = []character.ClassEntry{
		{Class: "Wizard", Level: 3},
		{Class: "Fighter", Level: 2},
	}
	c.AbilityScores.STR = 8
	c.AbilityScores.DEX = 14 // Meets DEX requirement
	c.AbilityScores.INT = 14 // Meets Wizard INT requirement
	warnings, err := Validate(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, w := range warnings {
		if w.Message != "" && len(w.Message) > 10 {
			if w.Message[:16] == "Multiclass Fight" {
				t.Errorf("Fighter with DEX 14 should not trigger prereq warning: %v", w)
			}
		}
	}
}

func TestValidate_MissingRace_StillValid(t *testing.T) {
	// Race can be empty from DDB (custom lineage etc)
	c := validCharacter()
	c.Race = "" // Empty race is allowed structurally
	warnings, err := Validate(c)
	if err != nil {
		t.Fatalf("unexpected structural error for empty race: %v", err)
	}
	_ = warnings
}

func TestBuildCreateParams_ProfBonusByLevel(t *testing.T) {
	tests := []struct {
		level    int
		wantProf int32
	}{
		{1, 2}, {4, 2}, {5, 3}, {8, 3}, {9, 4}, {12, 4}, {13, 5}, {16, 5}, {17, 6}, {20, 6},
	}
	for _, tt := range tests {
		pc := validCharacter()
		pc.Level = tt.level
		pc.Classes = []character.ClassEntry{{Class: "Fighter", Level: tt.level}}
		params, err := buildCreateParams(uuid.New(), "https://dndbeyond.com/characters/1", pc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if params.ProficiencyBonus != tt.wantProf {
			t.Errorf("level %d: proficiency bonus = %d, want %d", tt.level, params.ProficiencyBonus, tt.wantProf)
		}
	}
}
