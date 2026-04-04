package dashboard

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/stretchr/testify/assert"
)

func TestValidateDMSubmission_ValidInput(t *testing.T) {
	sub := DMCharacterSubmission{
		Name:       "Thorin",
		Race:       "Dwarf",
		Background: "Soldier",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
		},
		AbilityScores: character.AbilityScores{
			STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 8, CHA: 10,
		},
	}
	errs := ValidateDMSubmission(sub)
	assert.Empty(t, errs)
}

func TestValidateDMSubmission_MissingName(t *testing.T) {
	sub := DMCharacterSubmission{
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	errs := ValidateDMSubmission(sub)
	assert.Contains(t, errs, "name is required")
}

func TestValidateDMSubmission_MissingRace(t *testing.T) {
	sub := DMCharacterSubmission{
		Name: "Thorin",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	errs := ValidateDMSubmission(sub)
	assert.Contains(t, errs, "race is required")
}

func TestValidateDMSubmission_NoClasses(t *testing.T) {
	sub := DMCharacterSubmission{
		Name:          "Thorin",
		Race:          "Dwarf",
		Classes:       []character.ClassEntry{},
		AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	errs := ValidateDMSubmission(sub)
	assert.Contains(t, errs, "at least one class is required")
}

func TestValidateDMSubmission_ClassLevelZero(t *testing.T) {
	sub := DMCharacterSubmission{
		Name: "Thorin",
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 0},
		},
		AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	errs := ValidateDMSubmission(sub)
	assert.Contains(t, errs, "class entry 1: level must be at least 1")
}

func TestValidateDMSubmission_ClassNameEmpty(t *testing.T) {
	sub := DMCharacterSubmission{
		Name: "Thorin",
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "", Level: 1},
		},
		AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	errs := ValidateDMSubmission(sub)
	assert.Contains(t, errs, "class entry 1: class name is required")
}

func TestValidateDMSubmission_AbilityScoreTooLow(t *testing.T) {
	sub := DMCharacterSubmission{
		Name: "Thorin",
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{STR: 0, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	errs := ValidateDMSubmission(sub)
	assert.Contains(t, errs, "STR must be between 1 and 30")
}

func TestValidateDMSubmission_AbilityScoreTooHigh(t *testing.T) {
	sub := DMCharacterSubmission{
		Name: "Thorin",
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{STR: 10, DEX: 31, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	errs := ValidateDMSubmission(sub)
	assert.Contains(t, errs, "DEX must be between 1 and 30")
}

func TestValidateDMSubmission_TotalLevelOver20(t *testing.T) {
	sub := DMCharacterSubmission{
		Name: "Thorin",
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 15},
			{Class: "Rogue", Level: 10},
		},
		AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	errs := ValidateDMSubmission(sub)
	assert.Contains(t, errs, "total level cannot exceed 20")
}

func TestValidateDMSubmission_MultipleErrors(t *testing.T) {
	sub := DMCharacterSubmission{}
	errs := ValidateDMSubmission(sub)
	assert.True(t, len(errs) >= 3, "should have multiple errors, got %d", len(errs))
}

func TestDeriveDMStats_SingleClassFighter(t *testing.T) {
	sub := DMCharacterSubmission{
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
		},
		AbilityScores: character.AbilityScores{
			STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 8, CHA: 10,
		},
	}
	stats := DeriveDMStats(sub)

	// Fighter d10: level 1 = 10, levels 2-5 = 4 * 6 = 24. Total = 34 + CON(+2)*5 = 44
	assert.Equal(t, 44, stats.HPMax)
	// No armor: 10 + DEX(+1) = 11
	assert.Equal(t, 11, stats.AC)
	// Level 5 proficiency bonus = 3
	assert.Equal(t, 3, stats.ProficiencyBonus)
	// Total level
	assert.Equal(t, 5, stats.TotalLevel)
	// Speed: Human = 30
	assert.Equal(t, 30, stats.SpeedFt)
}

func TestDeriveDMStats_DwarfSpeed(t *testing.T) {
	sub := DMCharacterSubmission{
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{
			STR: 14, DEX: 10, CON: 14, INT: 10, WIS: 10, CHA: 10,
		},
	}
	stats := DeriveDMStats(sub)
	assert.Equal(t, 25, stats.SpeedFt)
}

func TestDeriveDMStats_WoodElfSpeed(t *testing.T) {
	sub := DMCharacterSubmission{
		Race: "Wood Elf",
		Classes: []character.ClassEntry{
			{Class: "Ranger", Level: 1},
		},
		AbilityScores: character.AbilityScores{
			STR: 10, DEX: 16, CON: 10, INT: 10, WIS: 14, CHA: 10,
		},
	}
	stats := DeriveDMStats(sub)
	assert.Equal(t, 35, stats.SpeedFt)
}

func TestDeriveDMStats_UnknownRaceDefaultsTo30(t *testing.T) {
	sub := DMCharacterSubmission{
		Race: "Homebrew Race",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{
			STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10,
		},
	}
	stats := DeriveDMStats(sub)
	assert.Equal(t, 30, stats.SpeedFt)
}

func TestDeriveDMStats_MulticlassFighterRogue(t *testing.T) {
	sub := DMCharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
			{Class: "Rogue", Level: 3},
		},
		AbilityScores: character.AbilityScores{
			STR: 14, DEX: 16, CON: 12, INT: 10, WIS: 10, CHA: 8,
		},
	}
	stats := DeriveDMStats(sub)

	// Fighter d10: level 1 = 10, levels 2-5 = 4*6 = 24
	// Rogue d8: 3*5 = 15
	// Total: 49 + CON(+1)*8 = 57
	assert.Equal(t, 57, stats.HPMax)
	// Level 8 proficiency bonus = 3
	assert.Equal(t, 3, stats.ProficiencyBonus)
	assert.Equal(t, 8, stats.TotalLevel)
}

func TestDeriveDMStats_SavingThrows(t *testing.T) {
	sub := DMCharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{
			STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 8, CHA: 10,
		},
	}
	stats := DeriveDMStats(sub)

	// Fighter saves: STR and CON
	assert.Contains(t, stats.SaveProficiencies, "str")
	assert.Contains(t, stats.SaveProficiencies, "con")
	assert.Len(t, stats.SaveProficiencies, 2)
}

func TestDeriveDMStats_SavingThrowValues(t *testing.T) {
	sub := DMCharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{
			STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 8, CHA: 10,
		},
	}
	stats := DeriveDMStats(sub)

	// STR save: +3 mod + 2 prof = +5
	assert.Equal(t, 5, stats.Saves["str"])
	// DEX save: +1 (not proficient)
	assert.Equal(t, 1, stats.Saves["dex"])
	// CON save: +2 mod + 2 prof = +4
	assert.Equal(t, 4, stats.Saves["con"])
}

func TestDeriveDMStats_WizardSaves(t *testing.T) {
	sub := DMCharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Wizard", Level: 1},
		},
		AbilityScores: character.AbilityScores{
			STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 12, CHA: 10,
		},
	}
	stats := DeriveDMStats(sub)

	assert.Contains(t, stats.SaveProficiencies, "int")
	assert.Contains(t, stats.SaveProficiencies, "wis")
}

func TestDeriveDMStats_EmptyClasses(t *testing.T) {
	sub := DMCharacterSubmission{
		AbilityScores: character.AbilityScores{
			STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10,
		},
	}
	stats := DeriveDMStats(sub)
	assert.Equal(t, 0, stats.HPMax)
	assert.Equal(t, 10, stats.AC)
	assert.Equal(t, 0, stats.TotalLevel)
}

func TestClassSaveProficiencies_AllClasses(t *testing.T) {
	tests := []struct {
		class string
		saves []string
	}{
		{"Barbarian", []string{"str", "con"}},
		{"Bard", []string{"dex", "cha"}},
		{"Cleric", []string{"wis", "cha"}},
		{"Druid", []string{"int", "wis"}},
		{"Fighter", []string{"str", "con"}},
		{"Monk", []string{"str", "dex"}},
		{"Paladin", []string{"wis", "cha"}},
		{"Ranger", []string{"str", "dex"}},
		{"Rogue", []string{"dex", "int"}},
		{"Sorcerer", []string{"con", "cha"}},
		{"Warlock", []string{"wis", "cha"}},
		{"Wizard", []string{"int", "wis"}},
		{"Unknown", nil},
	}
	for _, tc := range tests {
		t.Run(tc.class, func(t *testing.T) {
			classes := []character.ClassEntry{{Class: tc.class, Level: 1}}
			result := classSaveProficiencies(classes)
			assert.Equal(t, tc.saves, result)
		})
	}
}

func TestClassSkillProficiencies_AllClasses(t *testing.T) {
	tests := []struct {
		class  string
		skills []string
	}{
		{"Barbarian", []string{"athletics", "perception"}},
		{"Bard", []string{"performance", "persuasion", "deception"}},
		{"Cleric", []string{"insight", "medicine"}},
		{"Druid", []string{"nature", "perception"}},
		{"Fighter", []string{"athletics", "perception"}},
		{"Monk", []string{"acrobatics", "insight"}},
		{"Paladin", []string{"athletics", "persuasion"}},
		{"Ranger", []string{"perception", "survival", "stealth"}},
		{"Rogue", []string{"acrobatics", "stealth", "perception", "deception"}},
		{"Sorcerer", []string{"arcana", "persuasion"}},
		{"Warlock", []string{"arcana", "deception"}},
		{"Wizard", []string{"arcana", "investigation"}},
		{"Unknown", nil},
	}
	for _, tc := range tests {
		t.Run(tc.class, func(t *testing.T) {
			classes := []character.ClassEntry{{Class: tc.class, Level: 1}}
			result := classSkillProficiencies(classes)
			assert.Equal(t, tc.skills, result)
		})
	}
}

func TestRaceSpeed(t *testing.T) {
	tests := []struct {
		race  string
		speed int
	}{
		{"Human", 30},
		{"Elf", 30},
		{"High Elf", 30},
		{"Wood Elf", 35},
		{"Dwarf", 25},
		{"Hill Dwarf", 25},
		{"Mountain Dwarf", 25},
		{"Halfling", 25},
		{"Lightfoot Halfling", 25},
		{"Stout Halfling", 25},
		{"Gnome", 25},
		{"Forest Gnome", 25},
		{"Rock Gnome", 25},
		{"Dragonborn", 30},
		{"Half-Elf", 30},
		{"Half-Orc", 30},
		{"Tiefling", 30},
		{"Unknown", 30},
		{"", 30},
	}
	for _, tc := range tests {
		t.Run(tc.race, func(t *testing.T) {
			assert.Equal(t, tc.speed, raceSpeed(tc.race))
		})
	}
}

func TestClassSkillProficiencies_EmptyClasses(t *testing.T) {
	result := classSkillProficiencies(nil)
	assert.Nil(t, result)
}

func TestClassSaveProficiencies_EmptyClasses(t *testing.T) {
	result := classSaveProficiencies(nil)
	assert.Nil(t, result)
}

func TestDeriveDMStats_SkillModifiers_FighterProficiencies(t *testing.T) {
	sub := DMCharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{
			STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 8, CHA: 10,
		},
	}
	stats := DeriveDMStats(sub)

	// Skills field should be populated with all 18 skills
	assert.Len(t, stats.Skills, 18)

	// Fighter skill proficiencies: athletics, perception
	// athletics: STR mod(+3) + prof(+2) = +5
	assert.Equal(t, 5, stats.Skills["athletics"])

	// perception: WIS mod(-1) + prof(+2) = +1 (proficient)
	assert.Equal(t, 1, stats.Skills["perception"])

	// acrobatics: DEX mod(+1), not proficient = +1
	assert.Equal(t, 1, stats.Skills["acrobatics"])

	// arcana: INT mod(+0), not proficient = 0
	assert.Equal(t, 0, stats.Skills["arcana"])
}

func TestDeriveDMStats_SkillModifiers_AllSkillsPresent(t *testing.T) {
	sub := DMCharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Wizard", Level: 1},
		},
		AbilityScores: character.AbilityScores{
			STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10,
		},
	}
	stats := DeriveDMStats(sub)

	expectedSkills := []string{
		"acrobatics", "animal-handling", "arcana", "athletics",
		"deception", "history", "insight", "intimidation",
		"investigation", "medicine", "nature", "perception",
		"performance", "persuasion", "religion", "sleight-of-hand",
		"stealth", "survival",
	}
	for _, skill := range expectedSkills {
		_, ok := stats.Skills[skill]
		assert.True(t, ok, "missing skill: %s", skill)
	}
}

func TestDeriveDMStats_SkillModifiers_WithClassProficiency(t *testing.T) {
	sub := DMCharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Rogue", Level: 5},
		},
		AbilityScores: character.AbilityScores{
			STR: 10, DEX: 16, CON: 10, INT: 10, WIS: 10, CHA: 10,
		},
	}
	stats := DeriveDMStats(sub)

	// Rogue has proficiency in stealth (DEX-based)
	// DEX mod(+3) + prof bonus(+3) = +6
	assert.Equal(t, 6, stats.Skills["stealth"])

	// Non-proficient skill: athletics (STR-based)
	// STR mod(+0) = 0
	assert.Equal(t, 0, stats.Skills["athletics"])
}

func TestDeriveDMStats_HitDiceRemaining(t *testing.T) {
	sub := DMCharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
			{Class: "Rogue", Level: 3},
		},
		AbilityScores: character.AbilityScores{
			STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10,
		},
	}
	stats := DeriveDMStats(sub)

	assert.Equal(t, 5, stats.HitDiceRemaining["Fighter"])
	assert.Equal(t, 3, stats.HitDiceRemaining["Rogue"])
}
