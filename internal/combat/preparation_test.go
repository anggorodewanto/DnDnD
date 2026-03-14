package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TDD Cycle 1: IsPreparedCaster identifies prepared caster classes
func TestIsPreparedCaster(t *testing.T) {
	tests := []struct {
		class string
		want  bool
	}{
		{"cleric", true},
		{"druid", true},
		{"paladin", true},
		{"wizard", false},
		{"bard", false},
		{"sorcerer", false},
		{"warlock", false},
		{"ranger", false},
		{"fighter", false},
		{"Cleric", true},   // case insensitive
		{"PALADIN", true},  // case insensitive
	}
	for _, tc := range tests {
		t.Run(tc.class, func(t *testing.T) {
			assert.Equal(t, tc.want, IsPreparedCaster(tc.class))
		})
	}
}

// TDD Cycle 4: ParsePreparedSpells extracts prepared spells from character_data
func TestParsePreparedSpells(t *testing.T) {
	tests := []struct {
		name    string
		data    json.RawMessage
		want    []string
		wantErr bool
	}{
		{
			name: "valid prepared spells",
			data: json.RawMessage(`{"prepared_spells":["bless","cure-wounds","shield-of-faith"]}`),
			want: []string{"bless", "cure-wounds", "shield-of-faith"},
		},
		{
			name: "no prepared_spells key",
			data: json.RawMessage(`{"other":"data"}`),
			want: nil,
		},
		{
			name: "empty data",
			data: nil,
			want: nil,
		},
		{
			name: "empty object",
			data: json.RawMessage(`{}`),
			want: nil,
		},
		{
			name: "empty array",
			data: json.RawMessage(`{"prepared_spells":[]}`),
			want: []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParsePreparedSpells(tc.data)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TDD Cycle 5: BuildCharacterDataWithPreparedSpells merges prepared spells into character_data
func TestBuildCharacterDataWithPreparedSpells(t *testing.T) {
	tests := []struct {
		name     string
		existing json.RawMessage
		spells   []string
	}{
		{
			name:     "from empty",
			existing: nil,
			spells:   []string{"bless", "cure-wounds"},
		},
		{
			name:     "preserves other data",
			existing: json.RawMessage(`{"notes":"hello"}`),
			spells:   []string{"shield-of-faith"},
		},
		{
			name:     "replaces existing prepared",
			existing: json.RawMessage(`{"prepared_spells":["bless"]}`),
			spells:   []string{"cure-wounds", "shield-of-faith"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := BuildCharacterDataWithPreparedSpells(tc.existing, tc.spells)
			require.NoError(t, err)

			// Parse back and verify
			got, err := ParsePreparedSpells(result)
			require.NoError(t, err)
			assert.Equal(t, tc.spells, got)

			// If existing had other data, verify it's preserved
			if tc.existing != nil {
				var m map[string]any
				require.NoError(t, json.Unmarshal(result, &m))
				if tc.name == "preserves other data" {
					assert.Equal(t, "hello", m["notes"])
				}
			}
		})
	}
}

// TDD Cycle 2: MaxPreparedSpells computes max prepared count
// (moved below)

// TDD Cycle 3: AlwaysPreparedSpells returns subclass always-prepared spells
func TestAlwaysPreparedSpells(t *testing.T) {
	tests := []struct {
		name       string
		className  string
		subclass   string
		classLevel int
		wantIDs    []string
	}{
		{
			name: "Life Domain cleric level 1",
			className: "cleric", subclass: "life", classLevel: 1,
			wantIDs: []string{"bless", "cure-wounds"},
		},
		{
			name: "Life Domain cleric level 3",
			className: "cleric", subclass: "life", classLevel: 3,
			wantIDs: []string{"bless", "cure-wounds", "lesser-restoration", "spiritual-weapon"},
		},
		{
			name: "Life Domain cleric level 5",
			className: "cleric", subclass: "life", classLevel: 5,
			wantIDs: []string{"bless", "cure-wounds", "lesser-restoration", "spiritual-weapon", "beacon-of-hope", "revivify"},
		},
		{
			name: "Life Domain cleric level 9",
			className: "cleric", subclass: "life", classLevel: 9,
			wantIDs: []string{"bless", "cure-wounds", "lesser-restoration", "spiritual-weapon", "beacon-of-hope", "revivify", "death-ward", "guardian-of-faith", "mass-cure-wounds", "raise-dead"},
		},
		{
			name: "Oath of Devotion paladin level 3",
			className: "paladin", subclass: "devotion", classLevel: 3,
			wantIDs: []string{"protection-from-evil-and-good", "sanctuary"},
		},
		{
			name: "Oath of Devotion paladin level 5",
			className: "paladin", subclass: "devotion", classLevel: 5,
			wantIDs: []string{"protection-from-evil-and-good", "sanctuary", "lesser-restoration", "zone-of-truth"},
		},
		{
			name: "Oath of Devotion paladin level 9",
			className: "paladin", subclass: "devotion", classLevel: 9,
			wantIDs: []string{"protection-from-evil-and-good", "sanctuary", "lesser-restoration", "zone-of-truth", "beacon-of-hope", "dispel-magic"},
		},
		{
			name: "Circle of the Land druid level 3",
			className: "druid", subclass: "land", classLevel: 3,
			wantIDs: []string{"hold-person", "spike-growth"},
		},
		{
			name: "Circle of the Land druid level 5",
			className: "druid", subclass: "land", classLevel: 5,
			wantIDs: []string{"hold-person", "spike-growth", "sleet-storm", "slow"},
		},
		{
			name: "unknown subclass",
			className: "cleric", subclass: "unknown", classLevel: 5,
			wantIDs: nil,
		},
		{
			name: "non-prepared caster",
			className: "wizard", subclass: "evocation", classLevel: 5,
			wantIDs: nil,
		},
		{
			name: "paladin level 2 no subclass spells yet",
			className: "paladin", subclass: "devotion", classLevel: 2,
			wantIDs: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := AlwaysPreparedSpells(tc.className, tc.subclass, tc.classLevel)
			assert.Equal(t, tc.wantIDs, got)
		})
	}
}


// TDD Cycle 6: ValidateSpellPreparation validates the preparation list
func TestValidateSpellPreparation(t *testing.T) {
	classSpells := []refdata.Spell{
		{ID: "bless", Name: "Bless", Level: 1, Classes: []string{"cleric"}},
		{ID: "cure-wounds", Name: "Cure Wounds", Level: 1, Classes: []string{"cleric"}},
		{ID: "shield-of-faith", Name: "Shield of Faith", Level: 1, Classes: []string{"cleric"}},
		{ID: "aid", Name: "Aid", Level: 2, Classes: []string{"cleric"}},
		{ID: "silence", Name: "Silence", Level: 2, Classes: []string{"cleric"}},
		{ID: "revivify", Name: "Revivify", Level: 3, Classes: []string{"cleric"}},
	}

	tests := []struct {
		name         string
		selected     []string
		maxPrepared  int
		slotLevels   map[int]bool // available slot levels
		classSpells  []refdata.Spell
		alwaysIDs    []string // always-prepared (excluded from count)
		wantErr      string
	}{
		{
			name:        "valid preparation",
			selected:    []string{"bless", "cure-wounds"},
			maxPrepared: 4,
			slotLevels:  map[int]bool{1: true, 2: true},
			classSpells: classSpells,
			wantErr:     "",
		},
		{
			name:        "exceeds max prepared",
			selected:    []string{"bless", "cure-wounds", "shield-of-faith", "aid", "silence"},
			maxPrepared: 3,
			slotLevels:  map[int]bool{1: true, 2: true},
			classSpells: classSpells,
			wantErr:     "too many spells prepared",
		},
		{
			name:        "always-prepared excluded from count",
			selected:    []string{"cure-wounds", "shield-of-faith", "aid"},
			maxPrepared: 3,
			slotLevels:  map[int]bool{1: true, 2: true},
			classSpells: classSpells,
			alwaysIDs:   []string{"bless", "cure-wounds"}, // cure-wounds is always prepared, not counted
			wantErr:     "",
		},
		{
			name:        "spell not on class list",
			selected:    []string{"fireball"},
			maxPrepared: 4,
			slotLevels:  map[int]bool{1: true, 2: true, 3: true},
			classSpells: classSpells,
			wantErr:     "not on your class spell list",
		},
		{
			name:        "spell level exceeds available slots",
			selected:    []string{"revivify"},
			maxPrepared: 4,
			slotLevels:  map[int]bool{1: true, 2: true},
			classSpells: classSpells,
			wantErr:     "no spell slots of level 3",
		},
		{
			name:        "empty preparation is valid",
			selected:    []string{},
			maxPrepared: 4,
			slotLevels:  map[int]bool{1: true},
			classSpells: classSpells,
			wantErr:     "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSpellPreparation(tc.selected, tc.maxPrepared, tc.slotLevels, tc.classSpells, tc.alwaysIDs)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

// TDD Cycle 7: AvailableSlotLevels extracts which spell levels have max > 0
func TestAvailableSlotLevels(t *testing.T) {
	tests := []struct {
		name  string
		slots map[int]SlotInfo
		want  map[int]bool
	}{
		{
			name:  "level 5 full caster",
			slots: map[int]SlotInfo{1: {Current: 4, Max: 4}, 2: {Current: 3, Max: 3}, 3: {Current: 2, Max: 2}},
			want:  map[int]bool{1: true, 2: true, 3: true},
		},
		{
			name:  "includes expended slots",
			slots: map[int]SlotInfo{1: {Current: 0, Max: 4}, 2: {Current: 0, Max: 3}},
			want:  map[int]bool{1: true, 2: true},
		},
		{
			name:  "nil slots",
			slots: nil,
			want:  map[int]bool{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := AvailableSlotLevels(tc.slots)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TDD Cycle 8: Service.PrepareSpells orchestrates the full prepare flow
func TestService_PrepareSpells(t *testing.T) {
	charID := uuid.New()

	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "cleric", Level: 5}})
	scoresJSON, _ := json.Marshal(AbilityScores{Str: 10, Dex: 10, Con: 14, Int: 10, Wis: 16, Cha: 10})
	slotsJSON, _ := json.Marshal(map[string]SlotInfo{
		"1": {Current: 4, Max: 4},
		"2": {Current: 3, Max: 3},
		"3": {Current: 2, Max: 2},
	})

	classSpells := []refdata.Spell{
		{ID: "bless", Name: "Bless", Level: 1},
		{ID: "cure-wounds", Name: "Cure Wounds", Level: 1},
		{ID: "shield-of-faith", Name: "Shield of Faith", Level: 1},
		{ID: "aid", Name: "Aid", Level: 2},
		{ID: "spiritual-weapon", Name: "Spiritual Weapon", Level: 2},
		{ID: "revivify", Name: "Revivify", Level: 3},
		{ID: "beacon-of-hope", Name: "Beacon of Hope", Level: 3},
	}

	makeChar := func() refdata.Character {
		return refdata.Character{
			ID:            charID,
			Name:          "Brother Thomas",
			Classes:       classesJSON,
			Level:         5,
			AbilityScores: scoresJSON,
			SpellSlots:    pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}
	}

	t.Run("successful preparation", func(t *testing.T) {
		var savedData pqtype.NullRawMessage
		ms := &mockStore{
			getCharacterFn: func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
				return makeChar(), nil
			},
			listSpellsByClassFn: func(_ context.Context, class string) ([]refdata.Spell, error) {
				return classSpells, nil
			},
			updateCharacterDataFn: func(_ context.Context, arg refdata.UpdateCharacterDataParams) (refdata.Character, error) {
				savedData = arg.CharacterData
				ch := makeChar()
				ch.CharacterData = arg.CharacterData
				return ch, nil
			},
		}
		svc := NewService(ms)

		result, err := svc.PrepareSpells(context.Background(), PrepareSpellsInput{
			CharacterID: charID,
			ClassName:   "cleric",
			Subclass:    "life",
			Selected:    []string{"shield-of-faith", "aid", "revivify"},
		})
		require.NoError(t, err)
		// revivify is always-prepared at level 5 (life domain), so only 2 count
		assert.Equal(t, 2, result.PreparedCount)
		assert.Equal(t, 8, result.MaxPrepared) // WIS mod (3) + level (5) = 8

		// Verify always-prepared spells are listed
		assert.Contains(t, result.AlwaysPrepared, "bless")
		assert.Contains(t, result.AlwaysPrepared, "cure-wounds")

		// Verify data was saved
		assert.True(t, savedData.Valid)
		prepared, err := ParsePreparedSpells(savedData.RawMessage)
		require.NoError(t, err)
		assert.Equal(t, []string{"shield-of-faith", "aid", "revivify"}, prepared)
	})

	t.Run("non-prepared caster rejected", func(t *testing.T) {
		ms := &mockStore{}
		svc := NewService(ms)

		_, err := svc.PrepareSpells(context.Background(), PrepareSpellsInput{
			CharacterID: charID,
			ClassName:   "wizard",
			Subclass:    "evocation",
			Selected:    []string{"fireball"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a prepared caster")
	})

	t.Run("exceeds max prepared", func(t *testing.T) {
		ms := &mockStore{
			getCharacterFn: func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
				return makeChar(), nil
			},
			listSpellsByClassFn: func(_ context.Context, class string) ([]refdata.Spell, error) {
				// Return enough spells to fill more than max
				spells := make([]refdata.Spell, 20)
				for i := 0; i < 20; i++ {
					spells[i] = refdata.Spell{ID: fmt.Sprintf("spell-%d", i), Name: fmt.Sprintf("Spell %d", i), Level: 1}
				}
				return spells, nil
			},
		}
		svc := NewService(ms)

		// Try to prepare 9 spells (max is 8 = WIS mod 3 + level 5)
		selected := make([]string, 9)
		for i := 0; i < 9; i++ {
			selected[i] = fmt.Sprintf("spell-%d", i)
		}
		_, err := svc.PrepareSpells(context.Background(), PrepareSpellsInput{
			CharacterID: charID,
			ClassName:   "cleric",
			Subclass:    "life",
			Selected:    selected,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too many spells prepared")
	})

	t.Run("always-prepared excluded from count", func(t *testing.T) {
		var savedData pqtype.NullRawMessage
		ms := &mockStore{
			getCharacterFn: func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
				return makeChar(), nil
			},
			listSpellsByClassFn: func(_ context.Context, class string) ([]refdata.Spell, error) {
				return classSpells, nil
			},
			updateCharacterDataFn: func(_ context.Context, arg refdata.UpdateCharacterDataParams) (refdata.Character, error) {
				savedData = arg.CharacterData
				ch := makeChar()
				ch.CharacterData = arg.CharacterData
				return ch, nil
			},
		}
		svc := NewService(ms)

		// Include always-prepared "bless" plus 8 regular spells — should still fit
		// because bless doesn't count against the limit
		spellsForLevel1 := make([]refdata.Spell, 10)
		for i := 0; i < 10; i++ {
			spellsForLevel1[i] = refdata.Spell{ID: fmt.Sprintf("spell-%d", i), Name: fmt.Sprintf("Spell %d", i), Level: 1}
		}
		// Override listSpellsByClassFn to return all needed spells
		ms.listSpellsByClassFn = func(_ context.Context, class string) ([]refdata.Spell, error) {
			all := append(classSpells, spellsForLevel1...)
			return all, nil
		}

		selected := []string{"bless"} // always-prepared, doesn't count
		for i := 0; i < 8; i++ {
			selected = append(selected, fmt.Sprintf("spell-%d", i))
		}
		result, err := svc.PrepareSpells(context.Background(), PrepareSpellsInput{
			CharacterID: charID,
			ClassName:   "cleric",
			Subclass:    "life",
			Selected:    selected,
		})
		require.NoError(t, err)
		assert.Equal(t, 8, result.PreparedCount) // 8 non-always + 1 always = 9 total but only 8 counted
		_ = savedData
	})
}

// TDD Cycle 9: Service.GetPreparationInfo returns current state for the /prepare UI
func TestService_GetPreparationInfo(t *testing.T) {
	charID := uuid.New()

	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "cleric", Level: 5}})
	scoresJSON, _ := json.Marshal(AbilityScores{Str: 10, Dex: 10, Con: 14, Int: 10, Wis: 16, Cha: 10})
	slotsJSON, _ := json.Marshal(map[string]SlotInfo{
		"1": {Current: 4, Max: 4},
		"2": {Current: 3, Max: 3},
		"3": {Current: 2, Max: 2},
	})
	charDataJSON, _ := BuildCharacterDataWithPreparedSpells(nil, []string{"shield-of-faith", "aid"})

	classSpells := []refdata.Spell{
		{ID: "bless", Name: "Bless", Level: 1, School: "enchantment"},
		{ID: "cure-wounds", Name: "Cure Wounds", Level: 1, School: "evocation"},
		{ID: "shield-of-faith", Name: "Shield of Faith", Level: 1, School: "abjuration"},
		{ID: "aid", Name: "Aid", Level: 2, School: "abjuration"},
		{ID: "revivify", Name: "Revivify", Level: 3, School: "necromancy"},
	}

	ms := &mockStore{
		getCharacterFn: func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
			return refdata.Character{
				ID:            charID,
				Name:          "Brother Thomas",
				Classes:       classesJSON,
				Level:         5,
				AbilityScores: scoresJSON,
				SpellSlots:    pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
				CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
			}, nil
		},
		listSpellsByClassFn: func(_ context.Context, class string) ([]refdata.Spell, error) {
			return classSpells, nil
		},
	}
	svc := NewService(ms)

	info, err := svc.GetPreparationInfo(context.Background(), charID, "cleric", "life")
	require.NoError(t, err)

	assert.Equal(t, 8, info.MaxPrepared)
	assert.Equal(t, []string{"shield-of-faith", "aid"}, info.CurrentPrepared)
	assert.Len(t, info.ClassSpells, 5)
	assert.Contains(t, info.AlwaysPrepared, "bless")
	assert.Contains(t, info.AlwaysPrepared, "cure-wounds")
	assert.Equal(t, map[int]bool{1: true, 2: true, 3: true}, info.AvailableSlotLevels)
}

// TDD Cycle 10: FormatPreparationMessage produces readable output
func TestFormatPreparationMessage(t *testing.T) {
	info := PreparationInfo{
		MaxPrepared:     8,
		CurrentPrepared: []string{"shield-of-faith", "aid"},
		AlwaysPrepared:  []string{"bless", "cure-wounds"},
		ClassSpells: []refdata.Spell{
			{ID: "bless", Name: "Bless", Level: 1, School: "enchantment"},
			{ID: "cure-wounds", Name: "Cure Wounds", Level: 1, School: "evocation"},
			{ID: "shield-of-faith", Name: "Shield of Faith", Level: 1, School: "abjuration"},
			{ID: "aid", Name: "Aid", Level: 2, School: "abjuration"},
		},
		AvailableSlotLevels: map[int]bool{1: true, 2: true, 3: true},
	}

	msg := FormatPreparationMessage("Brother Thomas", info)
	assert.Contains(t, msg, "Brother Thomas")
	assert.Contains(t, msg, "2 / 8")
	assert.Contains(t, msg, "Always Prepared")
	assert.Contains(t, msg, "bless")
	assert.Contains(t, msg, "shield-of-faith")
}

// TDD Cycle 11: LongRestPrepareReminder returns hint for prepared casters
func TestLongRestPrepareReminder(t *testing.T) {
	tests := []struct {
		name    string
		classes []CharacterClass
		want    string
	}{
		{
			name:    "cleric gets reminder",
			classes: []CharacterClass{{Class: "cleric", Level: 5}},
			want:    "You can change your prepared spells with `/prepare`.",
		},
		{
			name:    "paladin gets reminder",
			classes: []CharacterClass{{Class: "paladin", Level: 3}},
			want:    "You can change your prepared spells with `/prepare`.",
		},
		{
			name:    "wizard no reminder",
			classes: []CharacterClass{{Class: "wizard", Level: 5}},
			want:    "",
		},
		{
			name:    "multiclass with cleric gets reminder",
			classes: []CharacterClass{{Class: "fighter", Level: 5}, {Class: "cleric", Level: 3}},
			want:    "You can change your prepared spells with `/prepare`.",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := LongRestPrepareReminder(tc.classes)
			assert.Equal(t, tc.want, got)
		})
	}
}

// Edge case tests for coverage
func TestParsePreparedSpells_InvalidJSON(t *testing.T) {
	_, err := ParsePreparedSpells(json.RawMessage(`{invalid`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing character_data")
}

func TestParsePreparedSpells_InvalidSpellsJSON(t *testing.T) {
	_, err := ParsePreparedSpells(json.RawMessage(`{"prepared_spells":"not-array"}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing prepared_spells")
}

func TestBuildCharacterDataWithPreparedSpells_InvalidExisting(t *testing.T) {
	_, err := BuildCharacterDataWithPreparedSpells(json.RawMessage(`{invalid`), []string{"bless"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing existing character_data")
}

func TestService_PrepareSpells_ErrorPaths(t *testing.T) {
	charID := uuid.New()

	t.Run("character not found", func(t *testing.T) {
		ms := &mockStore{
			getCharacterFn: func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
				return refdata.Character{}, fmt.Errorf("not found")
			},
		}
		svc := NewService(ms)
		_, err := svc.PrepareSpells(context.Background(), PrepareSpellsInput{
			CharacterID: charID,
			ClassName:   "cleric",
			Subclass:    "life",
			Selected:    []string{},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "getting character")
	})

	t.Run("invalid classes JSON", func(t *testing.T) {
		ms := &mockStore{
			getCharacterFn: func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
				return refdata.Character{ID: charID, Classes: json.RawMessage(`invalid`)}, nil
			},
		}
		svc := NewService(ms)
		_, err := svc.PrepareSpells(context.Background(), PrepareSpellsInput{
			CharacterID: charID,
			ClassName:   "cleric",
			Subclass:    "life",
			Selected:    []string{},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing classes")
	})

	t.Run("no class levels", func(t *testing.T) {
		classesJSON, _ := json.Marshal([]CharacterClass{{Class: "fighter", Level: 5}})
		ms := &mockStore{
			getCharacterFn: func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
				return refdata.Character{ID: charID, Classes: classesJSON}, nil
			},
		}
		svc := NewService(ms)
		_, err := svc.PrepareSpells(context.Background(), PrepareSpellsInput{
			CharacterID: charID,
			ClassName:   "cleric",
			Subclass:    "life",
			Selected:    []string{},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no levels in cleric")
	})

	t.Run("invalid ability scores", func(t *testing.T) {
		classesJSON, _ := json.Marshal([]CharacterClass{{Class: "cleric", Level: 5}})
		ms := &mockStore{
			getCharacterFn: func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
				return refdata.Character{ID: charID, Classes: classesJSON, AbilityScores: json.RawMessage(`invalid`)}, nil
			},
		}
		svc := NewService(ms)
		_, err := svc.PrepareSpells(context.Background(), PrepareSpellsInput{
			CharacterID: charID,
			ClassName:   "cleric",
			Subclass:    "life",
			Selected:    []string{},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing ability scores")
	})

	t.Run("listSpellsByClass error", func(t *testing.T) {
		classesJSON, _ := json.Marshal([]CharacterClass{{Class: "cleric", Level: 5}})
		scoresJSON, _ := json.Marshal(AbilityScores{Wis: 16})
		slotsJSON, _ := json.Marshal(map[string]SlotInfo{"1": {Current: 4, Max: 4}})
		ms := &mockStore{
			getCharacterFn: func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
				return refdata.Character{
					ID: charID, Classes: classesJSON, AbilityScores: scoresJSON,
					SpellSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
				}, nil
			},
			listSpellsByClassFn: func(_ context.Context, class string) ([]refdata.Spell, error) {
				return nil, fmt.Errorf("db error")
			},
		}
		svc := NewService(ms)
		_, err := svc.PrepareSpells(context.Background(), PrepareSpellsInput{
			CharacterID: charID,
			ClassName:   "cleric",
			Subclass:    "life",
			Selected:    []string{},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "listing class spells")
	})

	t.Run("updateCharacterData error", func(t *testing.T) {
		classesJSON, _ := json.Marshal([]CharacterClass{{Class: "cleric", Level: 5}})
		scoresJSON, _ := json.Marshal(AbilityScores{Wis: 16})
		slotsJSON, _ := json.Marshal(map[string]SlotInfo{"1": {Current: 4, Max: 4}})
		ms := &mockStore{
			getCharacterFn: func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
				return refdata.Character{
					ID: charID, Classes: classesJSON, AbilityScores: scoresJSON,
					SpellSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
				}, nil
			},
			listSpellsByClassFn: func(_ context.Context, class string) ([]refdata.Spell, error) {
				return []refdata.Spell{{ID: "bless", Level: 1}}, nil
			},
			updateCharacterDataFn: func(_ context.Context, arg refdata.UpdateCharacterDataParams) (refdata.Character, error) {
				return refdata.Character{}, fmt.Errorf("db error")
			},
		}
		svc := NewService(ms)
		_, err := svc.PrepareSpells(context.Background(), PrepareSpellsInput{
			CharacterID: charID,
			ClassName:   "cleric",
			Subclass:    "life",
			Selected:    []string{"bless"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "updating character data")
	})
}

func TestService_GetPreparationInfo_ErrorPaths(t *testing.T) {
	charID := uuid.New()

	t.Run("character not found", func(t *testing.T) {
		ms := &mockStore{
			getCharacterFn: func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
				return refdata.Character{}, fmt.Errorf("not found")
			},
		}
		svc := NewService(ms)
		_, err := svc.GetPreparationInfo(context.Background(), charID, "cleric", "life")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "getting character")
	})

	t.Run("no class levels", func(t *testing.T) {
		classesJSON, _ := json.Marshal([]CharacterClass{{Class: "fighter", Level: 5}})
		ms := &mockStore{
			getCharacterFn: func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
				return refdata.Character{ID: charID, Classes: classesJSON}, nil
			},
		}
		svc := NewService(ms)
		_, err := svc.GetPreparationInfo(context.Background(), charID, "cleric", "life")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no levels in cleric")
	})

	t.Run("invalid classes JSON", func(t *testing.T) {
		ms := &mockStore{
			getCharacterFn: func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
				return refdata.Character{ID: charID, Classes: json.RawMessage(`invalid`)}, nil
			},
		}
		svc := NewService(ms)
		_, err := svc.GetPreparationInfo(context.Background(), charID, "cleric", "life")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing classes")
	})

	t.Run("invalid ability scores", func(t *testing.T) {
		classesJSON, _ := json.Marshal([]CharacterClass{{Class: "cleric", Level: 5}})
		ms := &mockStore{
			getCharacterFn: func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
				return refdata.Character{ID: charID, Classes: classesJSON, AbilityScores: json.RawMessage(`invalid`)}, nil
			},
		}
		svc := NewService(ms)
		_, err := svc.GetPreparationInfo(context.Background(), charID, "cleric", "life")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing ability scores")
	})

	t.Run("listSpellsByClass error", func(t *testing.T) {
		classesJSON, _ := json.Marshal([]CharacterClass{{Class: "cleric", Level: 5}})
		scoresJSON, _ := json.Marshal(AbilityScores{Wis: 16})
		slotsJSON, _ := json.Marshal(map[string]SlotInfo{"1": {Current: 4, Max: 4}})
		ms := &mockStore{
			getCharacterFn: func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
				return refdata.Character{
					ID: charID, Classes: classesJSON, AbilityScores: scoresJSON,
					SpellSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
				}, nil
			},
			listSpellsByClassFn: func(_ context.Context, class string) ([]refdata.Spell, error) {
				return nil, fmt.Errorf("db error")
			},
		}
		svc := NewService(ms)
		_, err := svc.GetPreparationInfo(context.Background(), charID, "cleric", "life")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "listing class spells")
	})
}

func TestMaxPreparedSpells(t *testing.T) {
	tests := []struct {
		name       string
		abilityMod int
		classLevel int
		want       int
	}{
		{"normal cleric", 3, 5, 8},
		{"low wisdom cleric", -1, 1, 1}, // minimum 1
		{"high level paladin", 4, 10, 14},
		{"negative mod low level", -2, 1, 1}, // minimum 1
		{"zero mod", 0, 3, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, MaxPreparedSpells(tc.abilityMod, tc.classLevel))
		})
	}
}
