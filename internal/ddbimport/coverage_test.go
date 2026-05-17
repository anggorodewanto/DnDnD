package ddbimport

import (
	"context"
	"fmt"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
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
		{Name: "Ring 1", IsAttuned: true, RequiresAttunement: true},
		{Name: "Ring 2", IsAttuned: true, RequiresAttunement: true},
		{Name: "Ring 3", IsAttuned: true, RequiresAttunement: true},
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

func TestParseFeatures_NoSubclass(t *testing.T) {
	classes := []ddbClass{
		{
			Definition: ddbClassDef{
				Name:    "Fighter",
				HitDice: 10,
				ClassFeatures: []ddbClassFeature{
					{Name: "Second Wind", RequiredLevel: 1, Description: "Regain HP"},
					{Name: "Extra Attack", RequiredLevel: 5, Description: "Attack twice"},
				},
			},
			Level: 3, // Should only get Second Wind (level 1), not Extra Attack (level 5)
		},
	}
	features := parseFeatures(classes)
	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d: %v", len(features), features)
	}
	if features[0].Name != "Second Wind" {
		t.Errorf("expected Second Wind, got %s", features[0].Name)
	}
}

func TestParseFeatures_HighLevelSubclassExcluded(t *testing.T) {
	classes := []ddbClass{
		{
			Definition: ddbClassDef{
				Name:    "Fighter",
				HitDice: 10,
			},
			SubclassDefinition: &ddbSubclass{
				Name: "Champion",
				ClassFeatures: []ddbClassFeature{
					{Name: "Improved Critical", RequiredLevel: 3, Description: "Crit on 19-20"},
					{Name: "Remarkable Athlete", RequiredLevel: 7, Description: "Half prof to checks"},
				},
			},
			Level: 5,
		},
	}
	features := parseFeatures(classes)
	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}
	if features[0].Name != "Improved Critical" {
		t.Errorf("expected Improved Critical, got %s", features[0].Name)
	}
}

func TestParseSpells_Empty(t *testing.T) {
	spells := &ddbSpells{}
	result := parseSpells(spells, nil)
	if len(result) != 0 {
		t.Errorf("expected 0 spells, got %d", len(result))
	}
}

func TestParseSpells_AllSources(t *testing.T) {
	spells := &ddbSpells{
		Class: []ddbSpellEntry{{Definition: ddbSpellDef{Name: "ClassSpell", Level: 1}}},
		Race:  []ddbSpellEntry{{Definition: ddbSpellDef{Name: "RaceSpell", Level: 0}}},
		Item:  []ddbSpellEntry{{Definition: ddbSpellDef{Name: "ItemSpell", Level: 2}}},
		Feat:  []ddbSpellEntry{{Definition: ddbSpellDef{Name: "FeatSpell", Level: 0}}},
	}
	result := parseSpells(spells, nil)
	if len(result) != 4 {
		t.Fatalf("expected 4 spells, got %d", len(result))
	}
	sources := map[string]bool{}
	for _, s := range result {
		sources[s.Source] = true
	}
	for _, src := range []string{"class", "race", "item", "feat"} {
		if !sources[src] {
			t.Errorf("missing source %q", src)
		}
	}
}

func TestBuildUpdateParams(t *testing.T) {
	id := uuid.New()
	params := refdata.CreateCharacterParams{
		Name:  "Test",
		Race:  "Human",
		Level: 5,
	}
	up := buildUpdateParams(id, params)
	if up.ID != id {
		t.Errorf("ID = %s, want %s", up.ID, id)
	}
	if up.Name != "Test" {
		t.Errorf("Name = %q, want %q", up.Name, "Test")
	}
	if up.Level != 5 {
		t.Errorf("Level = %d, want 5", up.Level)
	}
}

func TestGenerateDiff_AllAbilityScores(t *testing.T) {
	old := validCharacter()
	new := validCharacter()
	new.AbilityScores.DEX = 16
	new.AbilityScores.CON = 17
	new.AbilityScores.INT = 12
	new.AbilityScores.WIS = 14
	new.AbilityScores.CHA = 10
	diff := GenerateDiff(old, new)
	if len(diff) != 5 {
		t.Errorf("expected 5 ability score changes, got %d: %v", len(diff), diff)
	}
}

func TestService_Import_CreateError(t *testing.T) {
	client := &mockClient{
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) {
			return minimalDDBJSON(), nil
		},
	}
	store := &mockCharStore{
		CreateFunc: func(ctx context.Context, params refdata.CreateCharacterParams) (refdata.Character, error) {
			return refdata.Character{}, fmt.Errorf("db error")
		},
	}
	svc := NewService(client, store)
	result, err := svc.Import(context.Background(), uuid.New(), "https://www.dndbeyond.com/characters/12345")
	if err != nil {
		t.Fatalf("Import should succeed (staging); got %v", err)
	}
	_, err = svc.ApproveImport(context.Background(), result.PendingImportID)
	if err == nil {
		t.Fatal("expected error for create failure in ApproveImport")
	}
}

// TestService_ApproveImport_UpdateError covers the case where the underlying
// UpdateCharacterFull fails during DM approval. (Pre-Phase-90 fix this lived
// in TestService_Import_UpdateError because Import itself called Update;
// post-fix the only path that touches UpdateCharacterFull is ApproveImport.)
func TestService_ApproveImport_UpdateError(t *testing.T) {
	campaignID := uuid.New()
	existingID := uuid.New()
	ddbURL := "https://www.dndbeyond.com/characters/12345"

	client := &mockClient{
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) {
			return minimalDDBJSON(), nil
		},
	}
	store := &mockCharStore{
		GetByDdbURLFunc: func(ctx context.Context, params refdata.GetCharacterByDdbURLParams) (refdata.Character, error) {
			return refdata.Character{ID: existingID, CampaignID: campaignID}, nil
		},
		UpdateFunc: func(ctx context.Context, params refdata.UpdateCharacterFullParams) (refdata.Character, error) {
			return refdata.Character{}, fmt.Errorf("update failed")
		},
	}
	svc := NewService(client, store)
	result, err := svc.Import(context.Background(), campaignID, ddbURL)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.PendingImportID == uuid.Nil {
		t.Fatal("expected PendingImportID for re-sync with changes")
	}
	if _, err := svc.ApproveImport(context.Background(), result.PendingImportID); err == nil {
		t.Fatal("expected ApproveImport to surface UpdateCharacterFull error")
	}
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
