package charactercard

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatCard_BasicFighter(t *testing.T) {
	data := CardData{
		Name:    "Aria",
		ShortID: "AR",
		Level:   8,
		Race:    "Half-Elf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Subclass: "Champion", Level: 5},
			{Class: "Rogue", Subclass: "Thief", Level: 3},
		},
		HpCurrent: 38,
		HpMax:     45,
		AC:        16,
		SpeedFt:   30,
		AbilityScores: character.AbilityScores{
			STR: 10, DEX: 18, CON: 14, WIS: 14, INT: 12, CHA: 10,
		},
		EquippedMainHand: "Longbow",
		EquippedOffHand:  "Shortsword",
		SpellSlots: map[string]character.SlotInfo{
			"1": {Current: 3, Max: 4},
			"2": {Current: 1, Max: 2},
		},
		Conditions:    []ConditionInfo{{Name: "Blessed", RemainingRounds: 2}},
		Concentration: "",
		Gold:          47,
		Languages:     []string{"Common", "Elvish"},
	}

	result := FormatCard(data)

	assert.Contains(t, result, "Aria (AR)")
	assert.Contains(t, result, "Level 8")
	assert.Contains(t, result, "Half-Elf")
	assert.Contains(t, result, "Fighter 5 (Champion)")
	assert.Contains(t, result, "Rogue 3 (Thief)")
	assert.Contains(t, result, "HP: 38/45")
	assert.Contains(t, result, "AC: 16")
	assert.Contains(t, result, "Speed: 30ft")
	assert.Contains(t, result, "STR 10")
	assert.Contains(t, result, "DEX 18")
	assert.Contains(t, result, "CON 14")
	assert.Contains(t, result, "WIS 14")
	assert.Contains(t, result, "INT 12")
	assert.Contains(t, result, "CHA 10")
	assert.Contains(t, result, "Longbow (main)")
	assert.Contains(t, result, "Shortsword (off-hand)")
	assert.Contains(t, result, "1st: 3/4")
	assert.Contains(t, result, "2nd: 1/2")
	assert.Contains(t, result, "Blessed (2 rounds remaining)")
	assert.Contains(t, result, "Gold: 47gp")
	assert.Contains(t, result, "Common, Elvish")
}

func TestFormatCard_SingleClass(t *testing.T) {
	data := CardData{
		Name:    "Bob",
		ShortID: "BO",
		Level:   3,
		Race:    "Human",
		Classes: []character.ClassEntry{
			{Class: "Wizard", Subclass: "", Level: 3},
		},
		HpCurrent: 18,
		HpMax:     18,
		AC:        12,
		SpeedFt:   30,
		AbilityScores: character.AbilityScores{
			STR: 8, DEX: 14, CON: 12, WIS: 10, INT: 18, CHA: 10,
		},
		Gold:      100,
		Languages: []string{"Common"},
	}

	result := FormatCard(data)

	assert.Contains(t, result, "Bob (BO)")
	assert.Contains(t, result, "Level 3 Human Wizard 3")
	// No subclass shown when empty
	assert.NotContains(t, result, "()")
	// No equipped weapons
	assert.Contains(t, result, "Equipped: —")
	// No spell slots
	assert.Contains(t, result, "Spell Slots: —")
	// No conditions
	assert.Contains(t, result, "Conditions: —")
	// Concentration
	assert.Contains(t, result, "Concentration: —")
}

func TestFormatCard_TempHP(t *testing.T) {
	data := CardData{
		Name:    "Tank",
		ShortID: "TA",
		Level:   5,
		Race:    "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Barbarian", Level: 5},
		},
		HpCurrent: 50,
		HpMax:     50,
		TempHP:    10,
		AC:        15,
		SpeedFt:   25,
		AbilityScores: character.AbilityScores{
			STR: 18, DEX: 12, CON: 16, WIS: 10, INT: 8, CHA: 8,
		},
		Gold:      25,
		Languages: []string{"Common", "Dwarvish"},
	}

	result := FormatCard(data)
	assert.Contains(t, result, "HP: 50/50 (+10 temp)")
}

func TestFormatCard_Concentration(t *testing.T) {
	data := CardData{
		Name:    "Caster",
		ShortID: "CA",
		Level:   5,
		Race:    "Elf",
		Classes: []character.ClassEntry{
			{Class: "Wizard", Level: 5},
		},
		HpCurrent:     30,
		HpMax:         30,
		AC:            12,
		SpeedFt:       30,
		AbilityScores: character.AbilityScores{STR: 8, DEX: 14, CON: 12, WIS: 12, INT: 18, CHA: 10},
		Concentration: "Haste",
		Gold:          50,
		Languages:     []string{"Common", "Elvish"},
	}

	result := FormatCard(data)
	assert.Contains(t, result, "Concentration: Haste")
}

func TestFormatCard_Exhaustion(t *testing.T) {
	data := CardData{
		Name:    "Tired",
		ShortID: "TI",
		Level:   3,
		Race:    "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 3},
		},
		HpCurrent:     20,
		HpMax:         25,
		AC:            16,
		SpeedFt:       30,
		AbilityScores: character.AbilityScores{STR: 16, DEX: 14, CON: 14, WIS: 10, INT: 10, CHA: 10},
		Exhaustion:    3,
		Gold:          10,
		Languages:     []string{"Common"},
	}

	result := FormatCard(data)
	assert.Contains(t, result, "Exhaustion: 3")
}

func TestFormatCard_NoExhaustion(t *testing.T) {
	data := CardData{
		Name:    "Fresh",
		ShortID: "FR",
		Level:   1,
		Race:    "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		HpCurrent:     10,
		HpMax:         10,
		AC:            16,
		SpeedFt:       30,
		AbilityScores: character.AbilityScores{STR: 16, DEX: 14, CON: 14, WIS: 10, INT: 10, CHA: 10},
		Exhaustion:    0,
		Gold:          15,
		Languages:     []string{"Common"},
	}

	result := FormatCard(data)
	assert.NotContains(t, result, "Exhaustion")
}

func TestFormatCard_RetiredBadge(t *testing.T) {
	data := CardData{
		Name:    "Old",
		ShortID: "OL",
		Level:   10,
		Race:    "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 10},
		},
		HpCurrent:     50,
		HpMax:         50,
		AC:            18,
		SpeedFt:       30,
		AbilityScores: character.AbilityScores{STR: 18, DEX: 14, CON: 16, WIS: 12, INT: 10, CHA: 10},
		Gold:          200,
		Languages:     []string{"Common"},
		Retired:       true,
	}

	result := FormatCard(data)
	require.Contains(t, result, "RETIRED")
}

func TestFormatCard_SpellSlotOrdering(t *testing.T) {
	data := CardData{
		Name:    "Mage",
		ShortID: "MA",
		Level:   5,
		Race:    "Elf",
		Classes: []character.ClassEntry{
			{Class: "Wizard", Level: 5},
		},
		HpCurrent:     25,
		HpMax:         25,
		AC:            12,
		SpeedFt:       30,
		AbilityScores: character.AbilityScores{STR: 8, DEX: 14, CON: 12, WIS: 12, INT: 18, CHA: 10},
		SpellSlots: map[string]character.SlotInfo{
			"3": {Current: 2, Max: 2},
			"1": {Current: 4, Max: 4},
			"2": {Current: 3, Max: 3},
		},
		Gold:      50,
		Languages: []string{"Common"},
	}

	result := FormatCard(data)
	// Slots should be in order: 1st, 2nd, 3rd
	idx1 := strings.Index(result, "1st:")
	idx2 := strings.Index(result, "2nd:")
	idx3 := strings.Index(result, "3rd:")
	assert.Greater(t, idx2, idx1, "2nd should come after 1st")
	assert.Greater(t, idx3, idx2, "3rd should come after 2nd")
}

func TestFormatCard_HighLevelSlot(t *testing.T) {
	data := CardData{
		Name:    "Archmage",
		ShortID: "AC",
		Level:   17,
		Race:    "Elf",
		Classes: []character.ClassEntry{
			{Class: "Wizard", Level: 17},
		},
		HpCurrent:     80,
		HpMax:         80,
		AC:            12,
		SpeedFt:       30,
		AbilityScores: character.AbilityScores{STR: 8, DEX: 14, CON: 14, WIS: 14, INT: 20, CHA: 10},
		SpellSlots: map[string]character.SlotInfo{
			"4": {Current: 1, Max: 1},
			"9": {Current: 1, Max: 1},
		},
		Gold:      1000,
		Languages: []string{"Common"},
	}

	result := FormatCard(data)
	assert.Contains(t, result, "4th: 1/1")
	assert.Contains(t, result, "9th: 1/1")
}

func TestFormatCard_ConditionWithoutDuration(t *testing.T) {
	data := CardData{
		Name:    "Sick",
		ShortID: "SI",
		Level:   1,
		Race:    "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		HpCurrent:     8,
		HpMax:         10,
		AC:            16,
		SpeedFt:       30,
		AbilityScores: character.AbilityScores{STR: 16, DEX: 14, CON: 14, WIS: 10, INT: 10, CHA: 10},
		Conditions:    []ConditionInfo{{Name: "Poisoned"}},
		Gold:          5,
		Languages:     []string{"Common"},
	}

	result := FormatCard(data)
	assert.Contains(t, result, "Conditions: Poisoned")
	assert.NotContains(t, result, "rounds remaining")
}

func TestFormatCard_SpellCount(t *testing.T) {
	data := CardData{
		Name:    "Mage",
		ShortID: "MA",
		Level:   5,
		Race:    "Elf",
		Classes: []character.ClassEntry{
			{Class: "Wizard", Level: 5},
		},
		HpCurrent:     25,
		HpMax:         25,
		AC:            12,
		SpeedFt:       30,
		AbilityScores: character.AbilityScores{STR: 8, DEX: 14, CON: 12, WIS: 12, INT: 18, CHA: 10},
		SpellSlots: map[string]character.SlotInfo{
			"1": {Current: 4, Max: 4},
		},
		SpellCount:    8,
		PreparedCount: 5,
		Gold:          50,
		Languages:     []string{"Common"},
	}

	result := FormatCard(data)
	assert.Contains(t, result, "Spells: 5 prepared / 8 known")
}

func TestFormatCard_SpellCount_HomebrewOffListBadge(t *testing.T) {
	data := CardData{
		Name:    "Mage",
		ShortID: "MA",
		Level:   3,
		Race:    "Human",
		Classes: []character.ClassEntry{
			{Class: "Wizard", Level: 3},
		},
		HpCurrent:          18,
		HpMax:              18,
		AC:                 12,
		SpeedFt:            30,
		AbilityScores:      character.AbilityScores{STR: 8, DEX: 14, CON: 12, WIS: 10, INT: 16, CHA: 10},
		SpellCount:         1,
		HomebrewSpellCount: 1,
		Gold:               10,
		Languages:          []string{"Common"},
	}

	result := FormatCard(data)
	assert.Contains(t, result, "Spells: 1 known (1 homebrew/off-list)")
}

func TestExtractSpellCounts_CountsHomebrewDDBSpells(t *testing.T) {
	char := newTestCharacter()
	charData := map[string]any{
		"spells": []map[string]any{
			{"name": "Cure Wounds", "level": 1, "source": "class", "homebrew": true, "off_list": true},
		},
	}
	charDataJSON, _ := json.Marshal(charData)
	char.CharacterData = pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true}

	spellCount, preparedCount, homebrewCount := extractSpellCounts(char)

	assert.Equal(t, 1, spellCount)
	assert.Equal(t, 0, preparedCount)
	assert.Equal(t, 1, homebrewCount)
}

func TestFormatCard_SpellCount_ZeroPrepared(t *testing.T) {
	data := CardData{
		Name:    "Sorc",
		ShortID: "SO",
		Level:   3,
		Race:    "Human",
		Classes: []character.ClassEntry{
			{Class: "Sorcerer", Level: 3},
		},
		HpCurrent:     20,
		HpMax:         20,
		AC:            12,
		SpeedFt:       30,
		AbilityScores: character.AbilityScores{STR: 8, DEX: 14, CON: 12, WIS: 10, INT: 10, CHA: 18},
		SpellCount:    6,
		Gold:          30,
		Languages:     []string{"Common"},
	}

	result := FormatCard(data)
	assert.Contains(t, result, "Spells: 6 known")
}

func TestFormatCard_NoSpellCount(t *testing.T) {
	data := CardData{
		Name:    "Tank",
		ShortID: "TA",
		Level:   5,
		Race:    "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Barbarian", Level: 5},
		},
		HpCurrent:     50,
		HpMax:         50,
		AC:            15,
		SpeedFt:       25,
		AbilityScores: character.AbilityScores{STR: 18, DEX: 12, CON: 16, WIS: 10, INT: 8, CHA: 8},
		Gold:          25,
		Languages:     []string{"Common"},
	}

	result := FormatCard(data)
	assert.NotContains(t, result, "Spells:")
}

func TestFormatCard_OnlyOffHand(t *testing.T) {
	data := CardData{
		Name:    "Shield",
		ShortID: "SH",
		Level:   1,
		Race:    "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		HpCurrent:       10,
		HpMax:           10,
		AC:              18,
		SpeedFt:         30,
		AbilityScores:   character.AbilityScores{STR: 16, DEX: 14, CON: 14, WIS: 10, INT: 10, CHA: 10},
		EquippedOffHand: "Shield",
		Gold:            10,
		Languages:       []string{"Common"},
	}

	result := FormatCard(data)
	assert.Contains(t, result, "Shield (off-hand)")
	assert.NotContains(t, result, "(main)")
}
