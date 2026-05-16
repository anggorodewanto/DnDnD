package levelup

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/character"
)

func TestCalculateLevelUp_SingleClassFighterLevel5to6(t *testing.T) {
	oldClasses := []character.ClassEntry{{Class: "fighter", Level: 5}}
	newClasses := []character.ClassEntry{{Class: "fighter", Level: 6}}
	hitDice := map[string]string{"fighter": "d10"}
	spellcasting := map[string]character.ClassSpellcasting{}
	attacksPerAction := map[string]map[int]int{
		"fighter": {1: 1, 5: 2, 11: 3, 20: 4},
	}
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}

	result := CalculateLevelUp(oldClasses, newClasses, hitDice, spellcasting, attacksPerAction, scores)

	if result.NewLevel != 6 {
		t.Errorf("NewLevel = %d, want 6", result.NewLevel)
	}
	if result.NewHPMax != character.CalculateHP(newClasses, hitDice, scores) {
		t.Errorf("NewHPMax = %d, want %d", result.NewHPMax, character.CalculateHP(newClasses, hitDice, scores))
	}
	if result.NewProficiencyBonus != 3 {
		t.Errorf("NewProficiencyBonus = %d, want 3", result.NewProficiencyBonus)
	}
	if result.NewAttacksPerAction != 2 {
		t.Errorf("NewAttacksPerAction = %d, want 2", result.NewAttacksPerAction)
	}
	if result.HPGained <= 0 {
		t.Errorf("HPGained = %d, want > 0", result.HPGained)
	}
}

func TestCalculateLevelUp_MulticlassWizardPaladin(t *testing.T) {
	oldClasses := []character.ClassEntry{
		{Class: "wizard", Level: 5},
		{Class: "paladin", Level: 3},
	}
	newClasses := []character.ClassEntry{
		{Class: "wizard", Level: 5},
		{Class: "paladin", Level: 4},
	}
	hitDice := map[string]string{"wizard": "d6", "paladin": "d10"}
	spellcasting := map[string]character.ClassSpellcasting{
		"wizard":  {SlotProgression: "full"},
		"paladin": {SlotProgression: "half"},
	}
	attacksPerAction := map[string]map[int]int{
		"wizard":  {1: 1},
		"paladin": {1: 1, 5: 2},
	}
	scores := character.AbilityScores{STR: 14, DEX: 10, CON: 12, INT: 18, WIS: 13, CHA: 14}

	result := CalculateLevelUp(oldClasses, newClasses, hitDice, spellcasting, attacksPerAction, scores)

	// Total level: 5+4=9
	if result.NewLevel != 9 {
		t.Errorf("NewLevel = %d, want 9", result.NewLevel)
	}
	// Caster level: 5 (wizard full) + 2 (paladin 4/2) = 7
	wantSlots := character.MulticastSpellSlots(7)
	if len(result.NewSpellSlots) != len(wantSlots) {
		t.Errorf("NewSpellSlots length = %d, want %d", len(result.NewSpellSlots), len(wantSlots))
	}
	// Attacks per action: max across classes. Wizard has 1 at all levels, Paladin has 1 at level 4
	if result.NewAttacksPerAction != 1 {
		t.Errorf("NewAttacksPerAction = %d, want 1", result.NewAttacksPerAction)
	}
}

func TestIsASILevel(t *testing.T) {
	tests := []struct {
		name    string
		classID string
		level   int
		want    bool
	}{
		// Standard ASI levels for generic class
		{"generic_1", "", 1, false},
		{"generic_4", "", 4, true},
		{"generic_6", "", 6, false},
		{"generic_8", "", 8, true},
		{"generic_10", "", 10, false},
		{"generic_12", "", 12, true},
		{"generic_14", "", 14, false},
		{"generic_16", "", 16, true},
		{"generic_19", "", 19, true},
		{"generic_20", "", 20, false},
		// Fighter gets extra ASI at 6 and 14
		{"fighter_4", "fighter", 4, true},
		{"fighter_6", "fighter", 6, true},
		{"fighter_8", "fighter", 8, true},
		{"fighter_14", "fighter", 14, true},
		{"fighter_5", "fighter", 5, false},
		// Rogue gets extra ASI at 10
		{"rogue_4", "rogue", 4, true},
		{"rogue_10", "rogue", 10, true},
		{"rogue_9", "rogue", 9, false},
		{"rogue_14", "rogue", 14, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsASILevel(tt.classID, tt.level)
			if got != tt.want {
				t.Errorf("IsASILevel(%q, %d) = %v, want %v", tt.classID, tt.level, got, tt.want)
			}
		})
	}
}

func TestNeedsSubclassSelection(t *testing.T) {
	tests := []struct {
		name          string
		classLevel    int
		subclassLevel int
		hasSubclass   bool
		want          bool
	}{
		{"at threshold without subclass", 3, 3, false, true},
		{"at threshold with subclass", 3, 3, true, false},
		{"below threshold", 2, 3, false, false},
		{"above threshold without subclass", 5, 3, false, true},
		{"cleric at 1", 1, 1, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsSubclassSelection(tt.classLevel, tt.subclassLevel, tt.hasSubclass)
			if got != tt.want {
				t.Errorf("NeedsSubclassSelection(%d, %d, %v) = %v, want %v",
					tt.classLevel, tt.subclassLevel, tt.hasSubclass, got, tt.want)
			}
		})
	}
}

func TestApplyASI_Plus2(t *testing.T) {
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}
	result, err := ApplyASI(scores, ASIChoice{
		Type:    ASIPlus2,
		Ability: "str",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.STR != 18 {
		t.Errorf("STR = %d, want 18", result.STR)
	}
}

func TestApplyASI_Plus1Plus1(t *testing.T) {
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}
	result, err := ApplyASI(scores, ASIChoice{
		Type:     ASIPlus1Plus1,
		Ability:  "str",
		Ability2: "dex",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.STR != 17 {
		t.Errorf("STR = %d, want 17", result.STR)
	}
	if result.DEX != 15 {
		t.Errorf("DEX = %d, want 15", result.DEX)
	}
}

func TestApplyASI_RejectsWhenPlus2Exceeds20(t *testing.T) {
	scores := character.AbilityScores{STR: 19, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}
	_, err := ApplyASI(scores, ASIChoice{
		Type:    ASIPlus2,
		Ability: "str",
	})
	if err == nil {
		t.Fatal("expected error when +2 would exceed 20")
	}
}

func TestApplyASI_AlreadyAt20(t *testing.T) {
	scores := character.AbilityScores{STR: 20, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}
	_, err := ApplyASI(scores, ASIChoice{
		Type:    ASIPlus2,
		Ability: "str",
	})
	if err == nil {
		t.Error("expected error for score already at 20")
	}
}

func TestApplyASI_InvalidAbility(t *testing.T) {
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}
	_, err := ApplyASI(scores, ASIChoice{
		Type:    ASIPlus2,
		Ability: "xyz",
	})
	if err == nil {
		t.Error("expected error for invalid ability")
	}
}

func TestCheckFeatPrerequisites_NoPrereqs(t *testing.T) {
	prereqs := FeatPrerequisites{}
	scores := character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10}
	ok, reason := CheckFeatPrerequisites(prereqs, scores, nil, false)
	if !ok {
		t.Errorf("expected ok for no prereqs, got reason: %s", reason)
	}
}

func TestCheckFeatPrerequisites_AbilityMet(t *testing.T) {
	prereqs := FeatPrerequisites{
		Ability: map[string]int{"dex": 13},
	}
	scores := character.AbilityScores{STR: 10, DEX: 14, CON: 10, INT: 10, WIS: 10, CHA: 10}
	ok, _ := CheckFeatPrerequisites(prereqs, scores, nil, false)
	if !ok {
		t.Error("expected ok for met ability prereq")
	}
}

func TestCheckFeatPrerequisites_AbilityNotMet(t *testing.T) {
	prereqs := FeatPrerequisites{
		Ability: map[string]int{"dex": 13},
	}
	scores := character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10}
	ok, reason := CheckFeatPrerequisites(prereqs, scores, nil, false)
	if ok {
		t.Error("expected fail for unmet ability prereq")
	}
	if reason == "" {
		t.Error("expected a reason string")
	}
}

func TestCheckFeatPrerequisites_SpellcastingRequired(t *testing.T) {
	prereqs := FeatPrerequisites{Spellcasting: true}
	scores := character.AbilityScores{}

	ok, _ := CheckFeatPrerequisites(prereqs, scores, nil, true)
	if !ok {
		t.Error("expected ok for spellcaster")
	}

	ok, reason := CheckFeatPrerequisites(prereqs, scores, nil, false)
	if ok {
		t.Errorf("expected fail for non-spellcaster, reason=%s", reason)
	}
}

func TestCheckFeatPrerequisites_ProficiencyRequired(t *testing.T) {
	prereqs := FeatPrerequisites{Proficiency: "heavy_armor"}
	scores := character.AbilityScores{}
	proficiencies := []string{"light", "medium", "heavy_armor"}

	ok, _ := CheckFeatPrerequisites(prereqs, scores, proficiencies, false)
	if !ok {
		t.Error("expected ok for met proficiency prereq")
	}

	ok, _ = CheckFeatPrerequisites(prereqs, scores, []string{"light"}, false)
	if ok {
		t.Error("expected fail for unmet proficiency prereq")
	}
}

func TestApplyASI_Plus1Plus1_SameAbility(t *testing.T) {
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}
	_, err := ApplyASI(scores, ASIChoice{
		Type:     ASIPlus1Plus1,
		Ability:  "str",
		Ability2: "str",
	})
	if err == nil {
		t.Error("expected error for same ability in +1/+1")
	}
}

func TestAttacksForLevel_FighterProgression(t *testing.T) {
	table := map[int]int{1: 1, 5: 2, 11: 3, 20: 4}
	tests := []struct {
		level int
		want  int
	}{
		{1, 1}, {4, 1}, {5, 2}, {10, 2}, {11, 3}, {19, 3}, {20, 4},
	}
	for _, tt := range tests {
		got := attacksForLevel(table, tt.level)
		if got != tt.want {
			t.Errorf("attacksForLevel(%d) = %d, want %d", tt.level, got, tt.want)
		}
	}
}

func TestMaxAttacksPerAction_MulticlassHighest(t *testing.T) {
	classes := []character.ClassEntry{
		{Class: "fighter", Level: 11},
		{Class: "wizard", Level: 5},
	}
	attacksMap := map[string]map[int]int{
		"fighter": {1: 1, 5: 2, 11: 3, 20: 4},
		"wizard":  {1: 1},
	}
	got := maxAttacksPerAction(classes, attacksMap)
	if got != 3 {
		t.Errorf("maxAttacksPerAction = %d, want 3", got)
	}
}

func TestFindLeveledClass_ExistingClassLevelUp(t *testing.T) {
	old := []character.ClassEntry{{Class: "fighter", Level: 5}}
	new := []character.ClassEntry{{Class: "fighter", Level: 6}}
	name, level := findLeveledClass(old, new)
	if name != "fighter" || level != 6 {
		t.Errorf("findLeveledClass = (%s, %d), want (fighter, 6)", name, level)
	}
}

func TestFindLeveledClass_NewMulticlass(t *testing.T) {
	old := []character.ClassEntry{{Class: "fighter", Level: 5}}
	new := []character.ClassEntry{{Class: "fighter", Level: 5}, {Class: "cleric", Level: 1}}
	name, level := findLeveledClass(old, new)
	if name != "cleric" || level != 1 {
		t.Errorf("findLeveledClass = (%s, %d), want (cleric, 1)", name, level)
	}
}

func TestFindLeveledClass_Empty(t *testing.T) {
	name, level := findLeveledClass(nil, nil)
	if name != "" || level != 0 {
		t.Errorf("findLeveledClass empty = (%s, %d), want (\"\", 0)", name, level)
	}
}

func TestCheckFeatPrerequisites_AbilityOr_OneMet(t *testing.T) {
	prereqs := FeatPrerequisites{
		AbilityOr: map[string]int{"int": 13, "wis": 13},
	}
	scores := character.AbilityScores{INT: 8, WIS: 14}
	ok, _ := CheckFeatPrerequisites(prereqs, scores, nil, false)
	if !ok {
		t.Error("expected ok when one of ability_or is met")
	}
}

func TestCheckFeatPrerequisites_AbilityOr_NoneMet(t *testing.T) {
	prereqs := FeatPrerequisites{
		AbilityOr: map[string]int{"int": 13, "wis": 13},
	}
	scores := character.AbilityScores{INT: 8, WIS: 10}
	ok, reason := CheckFeatPrerequisites(prereqs, scores, nil, false)
	if ok {
		t.Error("expected fail when none of ability_or is met")
	}
	if reason == "" {
		t.Error("expected reason")
	}
}

func TestApplyASI_Plus1Plus1_Ability2AtCap(t *testing.T) {
	scores := character.AbilityScores{STR: 16, DEX: 20, CON: 14, INT: 10, WIS: 12, CHA: 8}
	_, err := ApplyASI(scores, ASIChoice{
		Type:     ASIPlus1Plus1,
		Ability:  "str",
		Ability2: "dex",
	})
	if err == nil {
		t.Error("expected error for ability2 already at 20")
	}
}

func TestApplyASI_Plus1Plus1_Ability2Invalid(t *testing.T) {
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}
	_, err := ApplyASI(scores, ASIChoice{
		Type:     ASIPlus1Plus1,
		Ability:  "str",
		Ability2: "xyz",
	})
	if err == nil {
		t.Error("expected error for invalid ability2 name")
	}
}

func TestApplyASI_UnsupportedType(t *testing.T) {
	scores := character.AbilityScores{STR: 16}
	_, err := ApplyASI(scores, ASIChoice{Type: "unknown"})
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}

func TestSetScore_AllAbilities(t *testing.T) {
	var s character.AbilityScores
	setScore(&s, "str", 10)
	setScore(&s, "dex", 12)
	setScore(&s, "con", 14)
	setScore(&s, "int", 16)
	setScore(&s, "wis", 18)
	setScore(&s, "cha", 20)
	if s.STR != 10 || s.DEX != 12 || s.CON != 14 || s.INT != 16 || s.WIS != 18 || s.CHA != 20 {
		t.Errorf("unexpected scores: %+v", s)
	}
	// Test capping at 20
	setScore(&s, "str", 25)
	if s.STR != 20 {
		t.Errorf("STR = %d, want 20 (capped)", s.STR)
	}
}

func TestToInt(t *testing.T) {
	// float64 (from JSON)
	if v, ok := toInt(float64(5)); !ok || v != 5 {
		t.Errorf("toInt(float64(5)) = (%d, %v)", v, ok)
	}
	// int
	if v, ok := toInt(int(3)); !ok || v != 3 {
		t.Errorf("toInt(int(3)) = (%d, %v)", v, ok)
	}
	// json.Number
	if v, ok := toInt(json.Number("7")); !ok || v != 7 {
		t.Errorf("toInt(json.Number(7)) = (%d, %v)", v, ok)
	}
	// string -> false
	if _, ok := toInt("hello"); ok {
		t.Error("expected false for string")
	}
}

func TestMaxAttacksPerAction_NoEntries(t *testing.T) {
	classes := []character.ClassEntry{{Class: "wizard", Level: 5}}
	attacksMap := map[string]map[int]int{} // no entries
	got := maxAttacksPerAction(classes, attacksMap)
	if got != 1 {
		t.Errorf("maxAttacksPerAction empty = %d, want 1", got)
	}
}

func TestFormatASIDeniedMessage(t *testing.T) {
	msg := FormatASIDeniedMessage("Aria", "Choose STR instead")
	if !strings.Contains(msg, "Aria") || !strings.Contains(msg, "Choose STR") {
		t.Errorf("unexpected message: %s", msg)
	}
}

func TestCalculateLevelUp_WarlockPactMagic(t *testing.T) {
	oldClasses := []character.ClassEntry{
		{Class: "warlock", Level: 4},
		{Class: "fighter", Level: 3},
	}
	newClasses := []character.ClassEntry{
		{Class: "warlock", Level: 5},
		{Class: "fighter", Level: 3},
	}
	hitDice := map[string]string{"warlock": "d8", "fighter": "d10"}
	spellcasting := map[string]character.ClassSpellcasting{
		"warlock": {SlotProgression: "pact"},
	}
	attacksPerAction := map[string]map[int]int{
		"warlock": {1: 1},
		"fighter": {1: 1, 5: 2, 11: 3, 20: 4},
	}
	scores := character.AbilityScores{STR: 14, DEX: 10, CON: 12, INT: 10, WIS: 10, CHA: 16}

	result := CalculateLevelUp(oldClasses, newClasses, hitDice, spellcasting, attacksPerAction, scores)

	if result.NewLevel != 8 {
		t.Errorf("NewLevel = %d, want 8", result.NewLevel)
	}
	// Warlock pact slots at level 5: 2 slots at 3rd level
	if result.NewPactSlots.SlotLevel != 3 {
		t.Errorf("PactSlots.SlotLevel = %d, want 3", result.NewPactSlots.SlotLevel)
	}
	if result.NewPactSlots.Max != 2 {
		t.Errorf("PactSlots.Max = %d, want 2", result.NewPactSlots.Max)
	}
}
