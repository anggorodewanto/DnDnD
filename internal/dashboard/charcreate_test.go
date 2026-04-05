package dashboard

import (
	"encoding/json"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestDMCharacterSubmission_HasEquipmentSpellsLanguages(t *testing.T) {
	sub := DMCharacterSubmission{
		Name: "Thorin",
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
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
	var decoded DMCharacterSubmission
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, sub.Equipment, decoded.Equipment)
	assert.Equal(t, sub.Spells, decoded.Spells)
	assert.Equal(t, sub.Languages, decoded.Languages)
}

func TestCollectFeatures_SingleClassBarbarianLevel3(t *testing.T) {
	// features_by_level for barbarian (simplified)
	classFeatures := map[string]map[string][]character.Feature{
		"Barbarian": {
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
		"Barbarian": {
			"Berserker": {
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
		"Fighter": {
			"1": {{Name: "Fighting Style", Source: "Fighter", Level: 1, Description: "Choose style"}},
			"2": {{Name: "Action Surge", Source: "Fighter", Level: 2, Description: "Extra action"}},
		},
		"Wizard": {
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

	assert.Len(t, features, 4) // Darkvision, Fighting Style, Action Surge, Arcane Recovery
	assert.Equal(t, "Darkvision", features[0].Name) // racial traits first
}

func TestCollectFeatures_EmptyClasses(t *testing.T) {
	features := CollectFeatures(nil, nil, nil, nil)
	assert.Empty(t, features)
}

func TestCollectFeatures_IgnoresHigherLevelFeatures(t *testing.T) {
	classFeatures := map[string]map[string][]character.Feature{
		"Fighter": {
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
		"Fighter": {
			"Champion": {
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
		"Fighter": {
			"1": {{Name: "Fighting Style", Source: "Fighter", Level: 1, Description: "Choose style"}},
		},
	}
	// subclassFeatures has class but not the specific subclass
	subclassFeatures := map[string]map[string]map[string][]character.Feature{
		"Fighter": {
			"Champion": {
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
		"Fighter": {
			"1": {{Name: "Fighting Style", Source: "Fighter", Level: 1, Description: "Choose style"}},
		},
	}
	subclassFeatures := map[string]map[string]map[string][]character.Feature{
		"Fighter": {
			"Champion": {
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
		"Fighter": {
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
	sub := DMCharacterSubmission{
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{
			STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 10, CHA: 10,
		},
		WornArmor: "chain-mail",
	}
	stats := DeriveDMStats(sub)
	// Chain mail: AC 16, no DEX bonus
	assert.Equal(t, 16, stats.AC)
}

func TestDeriveDMStats_WornArmorWithShieldAffectsAC(t *testing.T) {
	sub := DMCharacterSubmission{
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{
			STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 10, CHA: 10,
		},
		WornArmor:       "chain-mail",
		EquippedWeapon:  "longsword",
	}
	stats := DeriveDMStats(sub)
	// Chain mail: AC 16, no shield
	assert.Equal(t, 16, stats.AC)
}

func TestDeriveDMStats_LeatherArmorWithDex(t *testing.T) {
	sub := DMCharacterSubmission{
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Rogue", Level: 1},
		},
		AbilityScores: character.AbilityScores{
			STR: 10, DEX: 16, CON: 12, INT: 10, WIS: 10, CHA: 10,
		},
		WornArmor: "leather",
	}
	stats := DeriveDMStats(sub)
	// Leather: AC 11 + DEX(+3) = 14
	assert.Equal(t, 14, stats.AC)
}

func TestDeriveDMStats_ShieldInEquipmentAddsAC(t *testing.T) {
	sub := DMCharacterSubmission{
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{
			STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 10, CHA: 10,
		},
		WornArmor: "chain-mail",
		Equipment: []string{"chain-mail", "shield", "longsword"},
	}
	stats := DeriveDMStats(sub)
	// Chain mail 16 + shield 2 = 18
	assert.Equal(t, 18, stats.AC)
}

func TestDMCharacterSubmission_EquippedWeaponWornArmor_JSON(t *testing.T) {
	sub := DMCharacterSubmission{
		Name: "Test",
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores:  character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
		EquippedWeapon: "longsword",
		WornArmor:      "chain-mail",
	}
	data, err := json.Marshal(sub)
	require.NoError(t, err)
	var decoded DMCharacterSubmission
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

func TestDeriveDMStats_MaxSpellLevel_Wizard3(t *testing.T) {
	sub := DMCharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Wizard", Level: 3},
		},
		AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 16, WIS: 10, CHA: 10},
	}
	stats := DeriveDMStats(sub)
	assert.Equal(t, 2, stats.MaxSpellLevel)
}

func TestDeriveDMStats_MaxSpellLevel_Fighter5(t *testing.T) {
	sub := DMCharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
		},
		AbilityScores: character.AbilityScores{STR: 16, DEX: 10, CON: 14, INT: 10, WIS: 10, CHA: 10},
	}
	stats := DeriveDMStats(sub)
	assert.Equal(t, 0, stats.MaxSpellLevel)
}

func TestDeriveDMStats_SpellSlots_FullCasterWizard(t *testing.T) {
	sub := DMCharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Wizard", Level: 3},
		},
		AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 16, WIS: 10, CHA: 10},
	}
	stats := DeriveDMStats(sub)

	// Wizard is a full caster, level 3 => caster level 3
	// Spell slots: {1:4, 2:2}
	require.NotNil(t, stats.SpellSlots)
	assert.Equal(t, 4, stats.SpellSlots[1])
	assert.Equal(t, 2, stats.SpellSlots[2])
}

func TestDeriveDMStats_SpellSlots_NonCasterFighter(t *testing.T) {
	sub := DMCharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
		},
		AbilityScores: character.AbilityScores{STR: 16, DEX: 10, CON: 14, INT: 10, WIS: 10, CHA: 10},
	}
	stats := DeriveDMStats(sub)
	assert.Nil(t, stats.SpellSlots)
}

func TestDeriveDMStats_SpellSlots_MulticlassHalfCaster(t *testing.T) {
	sub := DMCharacterSubmission{
		Classes: []character.ClassEntry{
			{Class: "Paladin", Level: 4},
			{Class: "Fighter", Level: 2},
		},
		AbilityScores: character.AbilityScores{STR: 16, DEX: 10, CON: 14, INT: 10, WIS: 10, CHA: 14},
	}
	stats := DeriveDMStats(sub)

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
