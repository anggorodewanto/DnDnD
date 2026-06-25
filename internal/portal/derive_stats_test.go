package portal

import (
	"encoding/json"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDMSubmission_ValidInput(t *testing.T) {
	sub := CharacterSubmission{
		Name:       "Thorin",
		Race:       "Dwarf",
		Background: "Soldier",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
		},
		AbilityScores: PointBuyScores{
			STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 8, CHA: 10,
		},
	}
	errs := ValidateSubmissionMode(sub, ModeDM)
	assert.Empty(t, errs)
}

func TestValidateDMSubmission_MissingName(t *testing.T) {
	sub := CharacterSubmission{
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: PointBuyScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	errs := ValidateSubmissionMode(sub, ModeDM)
	assert.Contains(t, errs, "name is required")
}

func TestValidateDMSubmission_MissingRace(t *testing.T) {
	sub := CharacterSubmission{
		Name: "Thorin",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: PointBuyScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	errs := ValidateSubmissionMode(sub, ModeDM)
	assert.Contains(t, errs, "race is required")
}

func TestValidateDMSubmission_NoClasses(t *testing.T) {
	sub := CharacterSubmission{
		Name:          "Thorin",
		Race:          "Dwarf",
		Classes:       []character.ClassEntry{},
		AbilityScores: PointBuyScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	errs := ValidateSubmissionMode(sub, ModeDM)
	assert.Contains(t, errs, "class is required")
}

func TestValidateDMSubmission_ClassLevelZero(t *testing.T) {
	sub := CharacterSubmission{
		Name: "Thorin",
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 0},
		},
		AbilityScores: PointBuyScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	errs := ValidateSubmissionMode(sub, ModeDM)
	assert.Contains(t, errs, "class entry 1: level must be at least 1")
}

func TestValidateDMSubmission_ClassNameEmpty(t *testing.T) {
	sub := CharacterSubmission{
		Name: "Thorin",
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "", Level: 1},
		},
		AbilityScores: PointBuyScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	errs := ValidateSubmissionMode(sub, ModeDM)
	assert.Contains(t, errs, "class entry 1: class name is required")
}

func TestValidateDMSubmission_AbilityScoreTooLow(t *testing.T) {
	sub := CharacterSubmission{
		Name: "Thorin",
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: PointBuyScores{STR: 0, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	errs := ValidateSubmissionMode(sub, ModeDM)
	assert.Contains(t, errs, "STR must be between 1 and 30")
}

func TestValidateDMSubmission_AbilityScoreTooHigh(t *testing.T) {
	sub := CharacterSubmission{
		Name: "Thorin",
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: PointBuyScores{STR: 10, DEX: 31, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	errs := ValidateSubmissionMode(sub, ModeDM)
	assert.Contains(t, errs, "DEX must be between 1 and 30")
}

func TestValidateDMSubmission_TotalLevelOver20(t *testing.T) {
	sub := CharacterSubmission{
		Name: "Thorin",
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 15},
			{Class: "Rogue", Level: 10},
		},
		AbilityScores: PointBuyScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}
	errs := ValidateSubmissionMode(sub, ModeDM)
	assert.Contains(t, errs, "total level cannot exceed 20")
}

func TestValidateDMSubmission_MultipleErrors(t *testing.T) {
	sub := CharacterSubmission{}
	errs := ValidateSubmissionMode(sub, ModeDM)
	assert.True(t, len(errs) >= 3, "should have multiple errors, got %d", len(errs))
}

func TestDeriveDMStats_SingleClassFighter(t *testing.T) {
	sub := CharacterSubmission{
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
		},
		AbilityScores: PointBuyScores{
			STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 8, CHA: 10,
		},
	}
	stats := DeriveStats(sub, nil)

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
	sub := CharacterSubmission{
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: PointBuyScores{
			STR: 14, DEX: 10, CON: 14, INT: 10, WIS: 10, CHA: 10,
		},
	}
	stats := DeriveStats(sub, nil)
	assert.Equal(t, 25, stats.SpeedFt)
}

func TestDeriveDMStats_WoodElfSpeed(t *testing.T) {
	sub := CharacterSubmission{
		Race: "Wood Elf",
		Classes: []character.ClassEntry{
			{Class: "Ranger", Level: 1},
		},
		AbilityScores: PointBuyScores{
			STR: 10, DEX: 16, CON: 10, INT: 10, WIS: 14, CHA: 10,
		},
	}
	stats := DeriveStats(sub, nil)
	assert.Equal(t, 35, stats.SpeedFt)
}

func TestDeriveDMStats_UnknownRaceDefaultsTo30(t *testing.T) {
	sub := CharacterSubmission{
		Race: "Homebrew Race",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: PointBuyScores{
			STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10,
		},
	}
	stats := DeriveStats(sub, nil)
	assert.Equal(t, 30, stats.SpeedFt)
}

func TestDeriveDMStats_MulticlassFighterRogue(t *testing.T) {
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
			{Class: "Rogue", Level: 3},
		},
		AbilityScores: PointBuyScores{
			STR: 14, DEX: 16, CON: 12, INT: 10, WIS: 10, CHA: 8,
		},
	}
	stats := DeriveStats(sub, nil)

	// Fighter d10: level 1 = 10, levels 2-5 = 4*6 = 24
	// Rogue d8: 3*5 = 15
	// Total: 49 + CON(+1)*8 = 57
	assert.Equal(t, 57, stats.HPMax)
	// Level 8 proficiency bonus = 3
	assert.Equal(t, 3, stats.ProficiencyBonus)
	assert.Equal(t, 8, stats.TotalLevel)
}

func TestDeriveDMStats_SavingThrows(t *testing.T) {
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: PointBuyScores{
			STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 8, CHA: 10,
		},
	}
	stats := DeriveStats(sub, nil)

	// Fighter saves: STR and CON
	assert.Contains(t, stats.SaveProficiencies, "str")
	assert.Contains(t, stats.SaveProficiencies, "con")
	assert.Len(t, stats.SaveProficiencies, 2)
}

func TestDeriveDMStats_SavingThrowValues(t *testing.T) {
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: PointBuyScores{
			STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 8, CHA: 10,
		},
	}
	stats := DeriveStats(sub, nil)

	// STR save: +3 mod + 2 prof = +5
	assert.Equal(t, 5, stats.Saves["str"])
	// DEX save: +1 (not proficient)
	assert.Equal(t, 1, stats.Saves["dex"])
	// CON save: +2 mod + 2 prof = +4
	assert.Equal(t, 4, stats.Saves["con"])
}

func TestDeriveDMStats_WizardSaves(t *testing.T) {
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Wizard", Level: 1},
		},
		AbilityScores: PointBuyScores{
			STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 12, CHA: 10,
		},
	}
	stats := DeriveStats(sub, nil)

	assert.Contains(t, stats.SaveProficiencies, "int")
	assert.Contains(t, stats.SaveProficiencies, "wis")
}

func TestDeriveDMStats_EmptyClasses(t *testing.T) {
	sub := CharacterSubmission{
		AbilityScores: PointBuyScores{
			STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10,
		},
	}
	stats := DeriveStats(sub, nil)
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

// TestBackgroundSkillProficiencies_AllBuilderBackgrounds locks the Go map to
// the exact 13 slugs the builder offers (CharacterBuilder.svelte BACKGROUNDS /
// backgrounds.js BACKGROUND_SKILLS). Any drift — a missing background or a
// space-vs-hyphen slug mismatch — drops the background's locked skills, which
// then get rejected as off-list class picks at submit time (the guild-artisan /
// folk-hero 400 regression).
func TestBackgroundSkillProficiencies_AllBuilderBackgrounds(t *testing.T) {
	tests := []struct {
		background string
		skills     []string
	}{
		{"acolyte", []string{"insight", "religion"}},
		{"charlatan", []string{"deception", "sleight-of-hand"}},
		{"criminal", []string{"deception", "stealth"}},
		{"entertainer", []string{"acrobatics", "performance"}},
		{"folk-hero", []string{"animal-handling", "survival"}},
		{"guild-artisan", []string{"insight", "persuasion"}},
		{"hermit", []string{"medicine", "religion"}},
		{"noble", []string{"history", "persuasion"}},
		{"outlander", []string{"athletics", "survival"}},
		{"sage", []string{"arcana", "history"}},
		{"sailor", []string{"athletics", "perception"}},
		{"soldier", []string{"athletics", "intimidation"}},
		{"urchin", []string{"sleight-of-hand", "stealth"}},
	}
	for _, tc := range tests {
		t.Run(tc.background, func(t *testing.T) {
			assert.Equal(t, tc.skills, backgroundSkillProficiencies(tc.background),
				"background %q must grant exactly its two PHB skills", tc.background)
		})
	}

	t.Run("unknown", func(t *testing.T) {
		assert.Nil(t, backgroundSkillProficiencies("not-a-background"))
	})
	t.Run("empty", func(t *testing.T) {
		assert.Nil(t, backgroundSkillProficiencies(""))
	})
}

// TestValidateSkillSelection_BarbarianGuildArtisan is the friend's exact 400
// case: a hill-dwarf barbarian/guild-artisan whose insight+persuasion come from
// the background. With the background skills locked, the selection is legal.
func TestValidateSkillSelection_BarbarianGuildArtisan(t *testing.T) {
	submitted := []string{"insight", "persuasion", "athletics", "intimidation"}
	locked := backgroundSkillProficiencies("guild-artisan") // insight, persuasion
	classFrom, classChoose := classSkillChoices([]character.ClassEntry{{Class: "barbarian", Level: 3}})
	err := validateSkillSelection(submitted, locked, classFrom, classChoose, 0)
	assert.NoError(t, err)
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
			assert.Equal(t, tc.speed, raceSpeedWithLookup(tc.race, nil))
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

func TestDMCharacterSubmission_HasEquipmentSpellsLanguages(t *testing.T) {
	sub := CharacterSubmission{
		Name: "Thorin",
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: PointBuyScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
		Equipment:     []string{"longsword", "chain-mail", "shield"},
		Spells:        []string{"fire-bolt", "shield"},
		Languages:     []string{"Common", "Dwarvish"},
	}
	assert.Equal(t, []string{"longsword", "chain-mail", "shield"}, sub.Equipment)
	assert.Equal(t, []string{"fire-bolt", "shield"}, sub.Spells)
	assert.Equal(t, []string{"Common", "Dwarvish"}, sub.Languages)

	// JSON round-trip
	data, err := json.Marshal(sub)
	require.NoError(t, err)
	var decoded CharacterSubmission
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, sub.Equipment, decoded.Equipment)
	assert.Equal(t, sub.Spells, decoded.Spells)
	assert.Equal(t, sub.Languages, decoded.Languages)
}

func TestCollectFeatures_SingleClassBarbarianLevel3(t *testing.T) {
	// features_by_level for barbarian (simplified)
	classFeatures := map[string]map[string][]character.Feature{
		"barbarian": {
			"1": {
				{Name: "Rage", Source: "Barbarian", Level: 1, Description: "Enter rage"},
				{Name: "Unarmored Defense", Source: "Barbarian", Level: 1, Description: "AC formula"},
			},
			"2": {
				{Name: "Reckless Attack", Source: "Barbarian", Level: 2, Description: "Advantage on attacks"},
			},
			"3": {
				{Name: "Primal Path", Source: "Barbarian", Level: 3, Description: "Choose subclass"},
			},
		},
	}
	subclassFeatures := map[string]map[string]map[string][]character.Feature{
		"barbarian": {
			"berserker": {
				"3": {{Name: "Frenzy", Source: "Barbarian (Berserker)", Level: 3, Description: "Bonus action attack"}},
			},
		},
	}

	classes := []character.ClassEntry{
		{Class: "Barbarian", Subclass: "Berserker", Level: 3},
	}

	features := CollectFeatures(classes, classFeatures, subclassFeatures, nil)

	assert.Len(t, features, 5) // Rage, Unarmored Defense, Reckless Attack, Primal Path, Frenzy
	names := make([]string, len(features))
	for i, f := range features {
		names[i] = f.Name
	}
	assert.Contains(t, names, "Rage")
	assert.Contains(t, names, "Unarmored Defense")
	assert.Contains(t, names, "Reckless Attack")
	assert.Contains(t, names, "Primal Path")
	assert.Contains(t, names, "Frenzy")
}

func TestCollectFeatures_MulticlassWithRacialTraits(t *testing.T) {
	classFeatures := map[string]map[string][]character.Feature{
		"fighter": {
			"1": {{Name: "Fighting Style", Source: "Fighter", Level: 1, Description: "Choose style"}},
			"2": {{Name: "Action Surge", Source: "Fighter", Level: 2, Description: "Extra action"}},
		},
		"wizard": {
			"1": {{Name: "Arcane Recovery", Source: "Wizard", Level: 1, Description: "Recover slots"}},
		},
	}
	subclassFeatures := map[string]map[string]map[string][]character.Feature{}
	racialTraits := []character.Feature{
		{Name: "Darkvision", Source: "Elf", Level: 0, Description: "See in dark"},
	}

	classes := []character.ClassEntry{
		{Class: "Fighter", Level: 2},
		{Class: "Wizard", Level: 1},
	}

	features := CollectFeatures(classes, classFeatures, subclassFeatures, racialTraits)

	assert.Len(t, features, 4)                      // Darkvision, Fighting Style, Action Surge, Arcane Recovery
	assert.Equal(t, "Darkvision", features[0].Name) // racial traits first
}

func TestCollectFeatures_EmptyClasses(t *testing.T) {
	features := CollectFeatures(nil, nil, nil, nil)
	assert.Empty(t, features)
}

func TestCollectFeatures_IgnoresHigherLevelFeatures(t *testing.T) {
	classFeatures := map[string]map[string][]character.Feature{
		"fighter": {
			"1": {{Name: "Fighting Style", Source: "Fighter", Level: 1, Description: "Choose style"}},
			"5": {{Name: "Extra Attack", Source: "Fighter", Level: 5, Description: "Two attacks"}},
		},
	}
	classes := []character.ClassEntry{
		{Class: "Fighter", Level: 3},
	}

	features := CollectFeatures(classes, classFeatures, nil, nil)
	assert.Len(t, features, 1)
	assert.Equal(t, "Fighting Style", features[0].Name)
}

func TestCollectFeatures_SubclassWithNoClassFeatures(t *testing.T) {
	classFeatures := map[string]map[string][]character.Feature{}
	subclassFeatures := map[string]map[string]map[string][]character.Feature{
		"fighter": {
			"champion": {
				"3": {{Name: "Improved Critical", Source: "Fighter (Champion)", Level: 3, Description: "Crit on 19-20"}},
			},
		},
	}
	classes := []character.ClassEntry{
		{Class: "Fighter", Subclass: "Champion", Level: 3},
	}
	features := CollectFeatures(classes, classFeatures, subclassFeatures, nil)
	assert.Len(t, features, 1)
	assert.Equal(t, "Improved Critical", features[0].Name)
}

func TestCollectFeatures_SubclassNotInMap(t *testing.T) {
	classFeatures := map[string]map[string][]character.Feature{
		"fighter": {
			"1": {{Name: "Fighting Style", Source: "Fighter", Level: 1, Description: "Choose style"}},
		},
	}
	// subclassFeatures has class but not the specific subclass
	subclassFeatures := map[string]map[string]map[string][]character.Feature{
		"fighter": {
			"champion": {
				"3": {{Name: "Improved Critical", Source: "Fighter (Champion)", Level: 3, Description: "Crit on 19-20"}},
			},
		},
	}
	classes := []character.ClassEntry{
		{Class: "Fighter", Subclass: "BattleMaster", Level: 3},
	}
	features := CollectFeatures(classes, classFeatures, subclassFeatures, nil)
	assert.Len(t, features, 1) // Only class feature at level 1
}

func TestCollectFeatures_NoSubclassSelected(t *testing.T) {
	classFeatures := map[string]map[string][]character.Feature{
		"fighter": {
			"1": {{Name: "Fighting Style", Source: "Fighter", Level: 1, Description: "Choose style"}},
		},
	}
	subclassFeatures := map[string]map[string]map[string][]character.Feature{
		"fighter": {
			"champion": {
				"3": {{Name: "Improved Critical", Source: "Fighter (Champion)", Level: 3, Description: "Crit on 19-20"}},
			},
		},
	}
	classes := []character.ClassEntry{
		{Class: "Fighter", Level: 3}, // no subclass
	}
	features := CollectFeatures(classes, classFeatures, subclassFeatures, nil)
	assert.Len(t, features, 1) // Only class feature
}

func TestCollectFeatures_NilSubclassFeatures(t *testing.T) {
	classFeatures := map[string]map[string][]character.Feature{
		"fighter": {
			"1": {{Name: "Fighting Style", Source: "Fighter", Level: 1, Description: "Choose style"}},
		},
	}
	classes := []character.ClassEntry{
		{Class: "Fighter", Subclass: "Champion", Level: 3},
	}
	// nil subclassFeatures map
	features := CollectFeatures(classes, classFeatures, nil, nil)
	assert.Len(t, features, 1)
}

func TestClassSpellcasting_EldritchKnight(t *testing.T) {
	sc := classSpellcasting("Eldritch Knight")
	assert.Equal(t, "third", sc.SlotProgression)
	assert.Equal(t, "int", sc.SpellAbility)
}

func TestClassSpellcasting_ArcaneTrickster(t *testing.T) {
	sc := classSpellcasting("Arcane Trickster")
	assert.Equal(t, "third", sc.SlotProgression)
	assert.Equal(t, "int", sc.SpellAbility)
}

func TestDeriveDMStats_WornArmorAffectsAC(t *testing.T) {
	sub := CharacterSubmission{
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: PointBuyScores{
			STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 10, CHA: 10,
		},
		WornArmor: "chain-mail",
	}
	stats := DeriveStats(sub, nil)
	// Chain mail: AC 16, no DEX bonus
	assert.Equal(t, 16, stats.AC)
}

func TestDeriveDMStats_WornArmorWithShieldAffectsAC(t *testing.T) {
	sub := CharacterSubmission{
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: PointBuyScores{
			STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 10, CHA: 10,
		},
		WornArmor:      "chain-mail",
		EquippedWeapon: "longsword",
	}
	stats := DeriveStats(sub, nil)
	// Chain mail: AC 16, no shield
	assert.Equal(t, 16, stats.AC)
}

func TestDeriveDMStats_LeatherArmorWithDex(t *testing.T) {
	sub := CharacterSubmission{
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Rogue", Level: 1},
		},
		AbilityScores: PointBuyScores{
			STR: 10, DEX: 16, CON: 12, INT: 10, WIS: 10, CHA: 10,
		},
		WornArmor: "leather",
	}
	stats := DeriveStats(sub, nil)
	// Leather: AC 11 + DEX(+3) = 14
	assert.Equal(t, 14, stats.AC)
}

func TestDeriveDMStats_ShieldInEquipmentAddsAC(t *testing.T) {
	sub := CharacterSubmission{
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: PointBuyScores{
			STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 10, CHA: 10,
		},
		WornArmor: "chain-mail",
		Equipment: []string{"chain-mail", "shield", "longsword"},
	}
	stats := DeriveStats(sub, nil)
	// Chain mail 16 + shield 2 = 18
	assert.Equal(t, 18, stats.AC)
}

func TestDMCharacterSubmission_EquippedWeaponWornArmor_JSON(t *testing.T) {
	sub := CharacterSubmission{
		Name: "Test",
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores:  PointBuyScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
		EquippedWeapon: "longsword",
		WornArmor:      "chain-mail",
	}
	data, err := json.Marshal(sub)
	require.NoError(t, err)
	var decoded CharacterSubmission
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "longsword", decoded.EquippedWeapon)
	assert.Equal(t, "chain-mail", decoded.WornArmor)
}

func TestLookupArmorInfo_KnownArmor(t *testing.T) {
	tests := []struct {
		id     string
		acBase int
	}{
		{"chain-mail", 16},
		{"leather", 11},
		{"plate", 18},
		{"studded-leather", 12},
	}
	for _, tc := range tests {
		info := lookupArmorInfo(tc.id)
		require.NotNil(t, info, "armor %s should exist", tc.id)
		assert.Equal(t, tc.acBase, info.ACBase, "armor %s AC", tc.id)
	}
}

func TestLookupArmorInfo_Empty(t *testing.T) {
	assert.Nil(t, lookupArmorInfo(""))
}

func TestLookupArmorInfo_Unknown(t *testing.T) {
	assert.Nil(t, lookupArmorInfo("magic-robe"))
}

func TestHasEquipmentItem(t *testing.T) {
	eq := []string{"longsword", "shield", "chain-mail"}
	assert.True(t, hasEquipmentItem(eq, "shield"))
	assert.True(t, hasEquipmentItem(eq, "Shield"))
	assert.False(t, hasEquipmentItem(eq, "dagger"))
	assert.False(t, hasEquipmentItem(nil, "shield"))
}

func TestCollectFeatures_ClassNotInFeaturesMap(t *testing.T) {
	classFeatures := map[string]map[string][]character.Feature{
		"Wizard": {
			"1": {{Name: "Arcane Recovery", Source: "Wizard", Level: 1, Description: "Recover slots"}},
		},
	}
	classes := []character.ClassEntry{
		{Class: "UnknownClass", Level: 3},
	}
	features := CollectFeatures(classes, classFeatures, nil, nil)
	assert.Empty(t, features)
}

func TestMaxSpellLevelForClasses_Wizard3(t *testing.T) {
	classes := []character.ClassEntry{{Class: "Wizard", Level: 3}}
	// Wizard level 3 = caster level 3 => has 2nd level slots => max spell level 2
	assert.Equal(t, 2, MaxSpellLevelForClasses(classes))
}

func TestMaxSpellLevelForClasses_Fighter5(t *testing.T) {
	classes := []character.ClassEntry{{Class: "Fighter", Level: 5}}
	// Fighter is not a caster => max spell level 0 (cantrips only... actually 0 means no casting)
	assert.Equal(t, 0, MaxSpellLevelForClasses(classes))
}

func TestMaxSpellLevelForClasses_Paladin4(t *testing.T) {
	classes := []character.ClassEntry{{Class: "Paladin", Level: 4}}
	// Paladin level 4 = half caster => caster level 2 => 1st level slots => max 1
	assert.Equal(t, 1, MaxSpellLevelForClasses(classes))
}

func TestMaxSpellLevelForClasses_Wizard5(t *testing.T) {
	classes := []character.ClassEntry{{Class: "Wizard", Level: 5}}
	// Wizard level 5 = caster level 5 => has 3rd level slots => max 3
	assert.Equal(t, 3, MaxSpellLevelForClasses(classes))
}

func TestMaxSpellLevelForClasses_Paladin1(t *testing.T) {
	classes := []character.ClassEntry{{Class: "Paladin", Level: 1}}
	// Half-casters get no leveled slots until character level 2 (ISSUE-006), so
	// a level-1 Paladin must derive max spell level 0, not 1.
	assert.Equal(t, 0, MaxSpellLevelForClasses(classes))
}

func TestMaxSpellLevelForClasses_Ranger1(t *testing.T) {
	classes := []character.ClassEntry{{Class: "Ranger", Level: 1}}
	assert.Equal(t, 0, MaxSpellLevelForClasses(classes))
}

func TestDeriveDMStats_Level1HalfCasterHasNoSpellSlots(t *testing.T) {
	// A freshly-built level-1 Paladin must store no leveled spell slots and a
	// max spell level of 0 (ISSUE-006: the old (level+1)/2 formula granted a
	// phantom first-level slot at L1).
	for _, class := range []string{"Paladin", "Ranger"} {
		t.Run(class, func(t *testing.T) {
			sub := CharacterSubmission{
				Race: "Human",
				Classes: []character.ClassEntry{
					{Class: class, Level: 1},
				},
				AbilityScores: PointBuyScores{
					STR: 14, DEX: 12, CON: 14, INT: 10, WIS: 12, CHA: 14,
				},
			}
			stats := DeriveStats(sub, nil)
			assert.Empty(t, stats.SpellSlots, "level-1 half-caster should have no leveled spell slots")
			assert.Equal(t, 0, stats.MaxSpellLevel, "level-1 half-caster should have max spell level 0")
		})
	}
}

func TestSpellSlotsForClasses_Level1HalfCasterIsNil(t *testing.T) {
	// The stored-shape helper must return nil (so spell_slots stays NULL) for a
	// level-1 half-caster, and the standard 2 first-level slots at level 2.
	assert.Nil(t, spellSlotsForClasses([]character.ClassEntry{{Class: "Paladin", Level: 1}}))
	assert.Nil(t, spellSlotsForClasses([]character.ClassEntry{{Class: "Ranger", Level: 1}}))

	l2 := spellSlotsForClasses([]character.ClassEntry{{Class: "Paladin", Level: 2}})
	assert.Equal(t, map[string]character.SlotInfo{"1": {Current: 2, Max: 2}}, l2)
}

func TestDeriveDMStats_SkillModifiers_FighterProficiencies(t *testing.T) {
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: PointBuyScores{
			STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 8, CHA: 10,
		},
	}
	stats := DeriveStats(sub, nil)

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
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Wizard", Level: 1},
		},
		AbilityScores: PointBuyScores{
			STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10,
		},
	}
	stats := DeriveStats(sub, nil)

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
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Rogue", Level: 5},
		},
		AbilityScores: PointBuyScores{
			STR: 10, DEX: 16, CON: 10, INT: 10, WIS: 10, CHA: 10,
		},
	}
	stats := DeriveStats(sub, nil)

	// Rogue has proficiency in stealth (DEX-based)
	// DEX mod(+3) + prof bonus(+3) = +6
	assert.Equal(t, 6, stats.Skills["stealth"])

	// Non-proficient skill: athletics (STR-based)
	// STR mod(+0) = 0
	assert.Equal(t, 0, stats.Skills["athletics"])
}

func TestDeriveDMStats_MaxSpellLevel_Wizard3(t *testing.T) {
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Wizard", Level: 3},
		},
		AbilityScores: PointBuyScores{STR: 10, DEX: 10, CON: 10, INT: 16, WIS: 10, CHA: 10},
	}
	stats := DeriveStats(sub, nil)
	assert.Equal(t, 2, stats.MaxSpellLevel)
}

func TestDeriveDMStats_MaxSpellLevel_Fighter5(t *testing.T) {
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
		},
		AbilityScores: PointBuyScores{STR: 16, DEX: 10, CON: 14, INT: 10, WIS: 10, CHA: 10},
	}
	stats := DeriveStats(sub, nil)
	assert.Equal(t, 0, stats.MaxSpellLevel)
}

// TestDeriveDMStats_MaxSpellLevel_Warlock3 covers the pact-magic bug: a
// single-class warlock has no standard spell slots ("pact" progression), so
// MaxSpellLevel was stuck at 0 and the builder blocked every leveled spell.
// A level-3 warlock can cast up to 2nd-level spells.
func TestDeriveDMStats_MaxSpellLevel_Warlock3(t *testing.T) {
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Warlock", Level: 3},
		},
		AbilityScores: PointBuyScores{STR: 8, DEX: 14, CON: 12, INT: 10, WIS: 12, CHA: 16},
	}
	stats := DeriveStats(sub, nil)
	assert.Equal(t, 2, stats.MaxSpellLevel)

	require.NotNil(t, stats.PactMagicSlots, "warlock should have pact magic slots")
	assert.Equal(t, 2, stats.PactMagicSlots.SlotLevel)
	assert.Equal(t, 2, stats.PactMagicSlots.Current)
	assert.Equal(t, 2, stats.PactMagicSlots.Max)
}

// TestDeriveDMStats_PactMagicSlots_NonCasterNil ensures non-pact classes leave
// the PactMagicSlots field unset.
func TestDeriveDMStats_PactMagicSlots_NonCasterNil(t *testing.T) {
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
		},
		AbilityScores: PointBuyScores{STR: 16, DEX: 10, CON: 14, INT: 10, WIS: 10, CHA: 10},
	}
	stats := DeriveStats(sub, nil)
	assert.Nil(t, stats.PactMagicSlots)
}

// TestDeriveDMStats_PactMagicSlots_FullCasterNil ensures standard full casters
// do not get pact magic slots.
func TestDeriveDMStats_PactMagicSlots_FullCasterNil(t *testing.T) {
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Wizard", Level: 3},
		},
		AbilityScores: PointBuyScores{STR: 8, DEX: 14, CON: 12, INT: 16, WIS: 10, CHA: 10},
	}
	stats := DeriveStats(sub, nil)
	assert.Nil(t, stats.PactMagicSlots)
}

// TestDeriveDMStats_MaxSpellLevel_WarlockWizardMulticlass verifies the pact
// slot level is combined (via max) with the standard-caster slot level. A
// Warlock 5 grants 3rd-level pact slots; a Wizard 3 grants 2nd-level standard
// slots — the higher (3) must win.
func TestDeriveDMStats_MaxSpellLevel_WarlockWizardMulticlass(t *testing.T) {
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Warlock", Level: 5},
			{Class: "Wizard", Level: 3},
		},
		AbilityScores: PointBuyScores{STR: 8, DEX: 14, CON: 12, INT: 14, WIS: 10, CHA: 16},
	}
	stats := DeriveStats(sub, nil)
	// Warlock 5 pact slot level = 3; standard caster level (warlock contributes
	// 0, wizard 3) => max standard slot level 2. Combined max = 3.
	assert.Equal(t, 3, stats.MaxSpellLevel)
	require.NotNil(t, stats.PactMagicSlots)
	assert.Equal(t, 3, stats.PactMagicSlots.SlotLevel)
}

func TestDeriveDMStats_SpellSlots_FullCasterWizard(t *testing.T) {
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Wizard", Level: 3},
		},
		AbilityScores: PointBuyScores{STR: 10, DEX: 10, CON: 10, INT: 16, WIS: 10, CHA: 10},
	}
	stats := DeriveStats(sub, nil)

	// Wizard is a full caster, level 3 => caster level 3
	// Spell slots: {1:4, 2:2}
	require.NotNil(t, stats.SpellSlots)
	assert.Equal(t, 4, stats.SpellSlots[1])
	assert.Equal(t, 2, stats.SpellSlots[2])
}

func TestDeriveDMStats_SpellSlots_NonCasterFighter(t *testing.T) {
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
		},
		AbilityScores: PointBuyScores{STR: 16, DEX: 10, CON: 14, INT: 10, WIS: 10, CHA: 10},
	}
	stats := DeriveStats(sub, nil)
	assert.Nil(t, stats.SpellSlots)
}

func TestDeriveDMStats_SpellSlots_MulticlassHalfCaster(t *testing.T) {
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Paladin", Level: 4},
			{Class: "Fighter", Level: 2},
		},
		AbilityScores: PointBuyScores{STR: 16, DEX: 10, CON: 14, INT: 10, WIS: 10, CHA: 14},
	}
	stats := DeriveStats(sub, nil)

	// Paladin half-caster level 4 => caster level 2, Fighter none => total caster level 2
	require.NotNil(t, stats.SpellSlots)
	assert.Equal(t, 3, stats.SpellSlots[1])
}

func TestClassSpellcasting_AllClasses(t *testing.T) {
	tests := []struct {
		class       string
		progression string
		ability     string
	}{
		{"Bard", "full", "cha"},
		{"Cleric", "full", "wis"},
		{"Druid", "full", "wis"},
		{"Sorcerer", "full", "cha"},
		{"Wizard", "full", "int"},
		{"Paladin", "half", "cha"},
		{"Ranger", "half", "wis"},
		{"Warlock", "pact", "cha"},
		{"Fighter", "none", ""},
		{"Barbarian", "none", ""},
		{"Rogue", "none", ""},
		{"Monk", "none", ""},
	}
	for _, tc := range tests {
		t.Run(tc.class, func(t *testing.T) {
			sc := classSpellcasting(tc.class)
			assert.Equal(t, tc.progression, sc.SlotProgression)
			assert.Equal(t, tc.ability, sc.SpellAbility)
		})
	}
}

func TestClassSpellcastingMap_FiltersNonCasters(t *testing.T) {
	classes := []character.ClassEntry{
		{Class: "Fighter", Level: 5},
		{Class: "Wizard", Level: 3},
	}
	m := classSpellcastingMap(classes)
	assert.Len(t, m, 1)
	assert.Equal(t, "full", m["Wizard"].SlotProgression)
	_, hasFighter := m["Fighter"]
	assert.False(t, hasFighter)
}

func TestDeriveDMStats_HitDiceRemaining(t *testing.T) {
	sub := CharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
			{Class: "Rogue", Level: 3},
		},
		AbilityScores: PointBuyScores{
			STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10,
		},
	}
	stats := DeriveStats(sub, nil)

	assert.Equal(t, 5, stats.HitDiceRemaining["Fighter"])
	assert.Equal(t, 3, stats.HitDiceRemaining["Rogue"])
}

func TestDeriveDMStats_BackgroundSkillProficiencies_Acolyte(t *testing.T) {
	sub := CharacterSubmission{
		Race:       "Human",
		Background: "Acolyte",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: PointBuyScores{
			STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 14, CHA: 10,
		},
	}
	stats := DeriveStats(sub, nil)

	// Fighter class skills: athletics, perception
	// Acolyte background skills: insight, religion
	// insight: WIS mod(+2) + prof(+2) = +4
	assert.Equal(t, 4, stats.Skills["insight"])
	// religion: INT mod(+0) + prof(+2) = +2
	assert.Equal(t, 2, stats.Skills["religion"])
}

// --- ISSUE-004: Unarmored Defense (Barbarian / Monk) ---

func TestDeriveStats_UnarmoredBarbarian_UsesConDefense(t *testing.T) {
	sub := CharacterSubmission{
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Barbarian", Level: 1},
		},
		// DEX 14 (+2), CON 16 (+3): 10 + 2 + 3 = 15
		AbilityScores: PointBuyScores{STR: 16, DEX: 14, CON: 16, INT: 8, WIS: 10, CHA: 10},
	}
	stats := DeriveStats(sub, nil)
	assert.Equal(t, 15, stats.AC, "unarmored barbarian = 10 + DEX + CON")
}

func TestDeriveStats_UnarmoredMonk_UsesWisDefense(t *testing.T) {
	sub := CharacterSubmission{
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Monk", Level: 1},
		},
		// DEX 16 (+3), WIS 14 (+2): 10 + 3 + 2 = 15
		AbilityScores: PointBuyScores{STR: 10, DEX: 16, CON: 12, INT: 8, WIS: 14, CHA: 10},
	}
	stats := DeriveStats(sub, nil)
	assert.Equal(t, 15, stats.AC, "unarmored monk = 10 + DEX + WIS")
}

func TestDeriveStats_UnarmoredBarbarian_WithShield_AddsTwo(t *testing.T) {
	sub := CharacterSubmission{
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Barbarian", Level: 1},
		},
		AbilityScores: PointBuyScores{STR: 16, DEX: 14, CON: 16, INT: 8, WIS: 10, CHA: 10},
		Equipment:     []string{"shield"},
	}
	stats := DeriveStats(sub, nil)
	assert.Equal(t, 17, stats.AC, "unarmored barbarian + shield = 15 + 2")
}

func TestDeriveStats_ArmoredBarbarian_IgnoresUnarmoredDefense(t *testing.T) {
	sub := CharacterSubmission{
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Barbarian", Level: 1},
		},
		// chain-mail is AC 16 flat (no DEX); UD would only give 15 → armor wins.
		AbilityScores: PointBuyScores{STR: 16, DEX: 14, CON: 16, INT: 8, WIS: 10, CHA: 10},
		WornArmor:     "chain-mail",
	}
	stats := DeriveStats(sub, nil)
	assert.Equal(t, 16, stats.AC, "armored barbarian uses armor AC, not Unarmored Defense")
}

func TestDeriveStats_Fighter_NoUnarmoredDefense(t *testing.T) {
	sub := CharacterSubmission{
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		// 10 + DEX(+2) = 12; CON must not be folded in.
		AbilityScores: PointBuyScores{STR: 16, DEX: 14, CON: 16, INT: 8, WIS: 10, CHA: 10},
	}
	stats := DeriveStats(sub, nil)
	assert.Equal(t, 12, stats.AC, "fighter has no Unarmored Defense")
}

func TestUnarmoredDefenseFormula(t *testing.T) {
	barb := []character.ClassEntry{{Class: "Barbarian", Level: 1}}
	monk := []character.ClassEntry{{Class: "Monk", Level: 1}}
	fighter := []character.ClassEntry{{Class: "Fighter", Level: 1}}

	tests := []struct {
		name      string
		classes   []character.ClassEntry
		wornArmor string
		hasShield bool
		want      string
	}{
		{"unarmored barbarian", barb, "", false, "10 + DEX + CON"},
		{"unarmored barbarian + shield", barb, "", true, "10 + DEX + CON"},
		{"armored barbarian", barb, "chain-mail", false, ""},
		{"unarmored monk", monk, "", false, "10 + DEX + WIS"},
		{"unarmored monk + shield (UD void)", monk, "", true, ""},
		{"armored monk", monk, "leather", false, ""},
		{"fighter", fighter, "", false, ""},
		// Multiclass barbarian+monk prefers barbarian (shield-compatible).
		{"barbarian+monk", append(append([]character.ClassEntry{}, barb...), monk...), "", false, "10 + DEX + CON"},
		{"barbarian+monk + shield", append(append([]character.ClassEntry{}, barb...), monk...), "", true, "10 + DEX + CON"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := unarmoredDefenseFormula(tc.classes, tc.wornArmor, tc.hasShield)
			assert.Equal(t, tc.want, got)
		})
	}
}
