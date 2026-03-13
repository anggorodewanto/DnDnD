package combat

import (
	"context"
	"testing"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/stretchr/testify/assert"
)

// TDD Cycle 2: SmiteDamageFormula returns dice count and formatted string
func TestSmiteDamageFormula(t *testing.T) {
	tests := []struct {
		name      string
		slotLevel int
		isUndead  bool
		isCrit    bool
		wantCount int
		wantStr   string
	}{
		{"1st level", 1, false, false, 2, "2d8"},
		{"2nd level", 2, false, false, 3, "3d8"},
		{"4th level cap", 4, false, false, 5, "5d8"},
		{"1st level undead", 1, true, false, 3, "3d8"},
		{"2nd level undead", 2, true, false, 4, "4d8"},
		{"4th level undead capped at +1", 4, true, false, 6, "6d8"},
		{"1st level crit", 1, false, true, 4, "4d8"},
		{"2nd level crit", 2, false, true, 6, "6d8"},
		{"1st level undead crit", 1, true, true, 6, "6d8"},
		{"4th level crit", 4, false, true, 10, "10d8"},
		{"4th level undead crit", 4, true, true, 12, "12d8"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			count, str := SmiteDamageFormula(tc.slotLevel, tc.isUndead, tc.isCrit)
			assert.Equal(t, tc.wantCount, count, "dice count")
			assert.Equal(t, tc.wantStr, str, "dice string")
		})
	}
}

// TDD Cycle 3: AvailableSmiteSlots returns sorted available slot levels
func TestAvailableSmiteSlots(t *testing.T) {
	tests := []struct {
		name   string
		slots  map[string]SlotInfo
		expect []int
	}{
		{"all slots available", map[string]SlotInfo{
			"1": {Current: 3, Max: 4},
			"2": {Current: 2, Max: 3},
			"3": {Current: 1, Max: 2},
		}, []int{1, 2, 3}},
		{"some depleted", map[string]SlotInfo{
			"1": {Current: 0, Max: 4},
			"2": {Current: 2, Max: 3},
			"3": {Current: 0, Max: 2},
		}, []int{2}},
		{"none available", map[string]SlotInfo{
			"1": {Current: 0, Max: 4},
			"2": {Current: 0, Max: 3},
		}, nil},
		{"nil map", nil, nil},
		{"empty map", map[string]SlotInfo{}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := AvailableSmiteSlots(tc.slots)
			assert.Equal(t, tc.expect, got)
		})
	}
}

// TDD Cycle 4: IsSmiteEligible checks melee hit
func TestIsSmiteEligible(t *testing.T) {
	tests := []struct {
		name    string
		result  AttackResult
		want    bool
	}{
		{"melee hit", AttackResult{Hit: true, IsMelee: true}, true},
		{"melee miss", AttackResult{Hit: false, IsMelee: true}, false},
		{"ranged hit", AttackResult{Hit: true, IsMelee: false}, false},
		{"ranged miss", AttackResult{Hit: false, IsMelee: false}, false},
		{"melee crit", AttackResult{Hit: true, IsMelee: true, CriticalHit: true}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsSmiteEligible(tc.result))
		})
	}
}

// TDD Cycle 5: FormatSmiteCombatLog
func TestFormatSmiteCombatLog(t *testing.T) {
	t.Run("basic 2nd level", func(t *testing.T) {
		log := FormatSmiteCombatLog(2, false, false, "3d8", 14)
		assert.Equal(t, "⚡ Divine Smite (2nd-level slot) — 3d8 radiant: 14", log)
	})
	t.Run("1st level", func(t *testing.T) {
		log := FormatSmiteCombatLog(1, false, false, "2d8", 9)
		assert.Equal(t, "⚡ Divine Smite (1st-level slot) — 2d8 radiant: 9", log)
	})
	t.Run("3rd level", func(t *testing.T) {
		log := FormatSmiteCombatLog(3, false, false, "4d8", 20)
		assert.Equal(t, "⚡ Divine Smite (3rd-level slot) — 4d8 radiant: 20", log)
	})
	t.Run("4th level", func(t *testing.T) {
		log := FormatSmiteCombatLog(4, false, false, "5d8", 25)
		assert.Equal(t, "⚡ Divine Smite (4th-level slot) — 5d8 radiant: 25", log)
	})
	t.Run("crit", func(t *testing.T) {
		log := FormatSmiteCombatLog(1, false, true, "4d8", 22)
		assert.Equal(t, "⚡ Divine Smite (1st-level slot, crit) — 4d8 radiant (doubled): 22", log)
	})
	t.Run("undead crit", func(t *testing.T) {
		log := FormatSmiteCombatLog(1, true, true, "6d8", 28)
		assert.Equal(t, "⚡ Divine Smite (1st-level slot, crit) — 6d8 radiant (doubled) +2d8 vs undead: 28", log)
	})
	t.Run("undead no crit", func(t *testing.T) {
		log := FormatSmiteCombatLog(1, true, false, "3d8", 15)
		assert.Equal(t, "⚡ Divine Smite (1st-level slot) — 3d8 radiant +1d8 vs undead: 15", log)
	})
}

// TDD Cycle 6: ParseSpellSlots parses character spell_slots JSON
func TestParseSpellSlots(t *testing.T) {
	t.Run("valid slots", func(t *testing.T) {
		raw := []byte(`{"1": {"current": 3, "max": 4}, "2": {"current": 2, "max": 3}}`)
		slots, err := ParseSpellSlots(raw)
		assert.NoError(t, err)
		assert.Equal(t, SlotInfo{Current: 3, Max: 4}, slots["1"])
		assert.Equal(t, SlotInfo{Current: 2, Max: 3}, slots["2"])
	})
	t.Run("nil", func(t *testing.T) {
		slots, err := ParseSpellSlots(nil)
		assert.NoError(t, err)
		assert.Nil(t, slots)
	})
	t.Run("empty", func(t *testing.T) {
		slots, err := ParseSpellSlots([]byte{})
		assert.NoError(t, err)
		assert.Nil(t, slots)
	})
}

// TDD Cycle 7: HasFeatureByName checks feature list by name
func TestHasFeatureByName(t *testing.T) {
	features := []byte(`[{"name":"Divine Smite","mechanical_effect":"expend_spell_slot_2d8_radiant_plus_1d8_per_slot_level"},{"name":"Lay on Hands","mechanical_effect":"lay_on_hands"}]`)
	assert.True(t, HasFeatureByName(features, "Divine Smite"))
	assert.True(t, HasFeatureByName(features, "divine smite"))
	assert.False(t, HasFeatureByName(features, "Fireball"))
	assert.False(t, HasFeatureByName(nil, "Divine Smite"))
}

// TDD Cycle 8: DivineSmiteCommand and DivineSmiteResult struct existence
func TestDivineSmiteStructs(t *testing.T) {
	cmd := DivineSmiteCommand{
		SlotLevel:  1,
		IsCritical: false,
	}
	assert.Equal(t, 1, cmd.SlotLevel)

	result := DivineSmiteResult{
		SmiteDamage:    14,
		SmiteDice:      "2d8",
		SlotLevel:      1,
		IsUndead:       false,
		IsCritical:     false,
		SlotsRemaining: map[string]SlotInfo{"1": {Current: 2, Max: 3}},
		CombatLog:      "test",
	}
	assert.Equal(t, 14, result.SmiteDamage)
}

// Edge case: DivineSmite with NPC attacker
func TestDivineSmite_NPCAttacker(t *testing.T) {
	svc := NewService(nil)
	_, err := svc.DivineSmite(context.Background(), DivineSmiteCommand{
		Attacker:     refdata.Combatant{}, // no CharacterID
		AttackResult: AttackResult{Hit: true, IsMelee: true},
	}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "character (not NPC)")
}

// Edge case: ParseSpellSlots with invalid JSON
func TestParseSpellSlots_InvalidJSON(t *testing.T) {
	_, err := ParseSpellSlots([]byte(`{bad json}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing spell_slots")
}

// Edge case: HasFeatureByName with invalid JSON
func TestHasFeatureByName_InvalidJSON(t *testing.T) {
	assert.False(t, HasFeatureByName([]byte(`{bad json}`), "Divine Smite"))
}

// Edge case: AvailableSmiteSlots with non-numeric keys
func TestAvailableSmiteSlots_NonNumericKey(t *testing.T) {
	slots := map[string]SlotInfo{
		"abc": {Current: 2, Max: 3},
		"1":   {Current: 1, Max: 2},
	}
	got := AvailableSmiteSlots(slots)
	assert.Equal(t, []int{1}, got)
}

// Edge case: isUndeadOrFiend
func TestIsUndeadOrFiend(t *testing.T) {
	assert.True(t, isUndeadOrFiend("undead"))
	assert.True(t, isUndeadOrFiend("Undead"))
	assert.True(t, isUndeadOrFiend("fiend"))
	assert.True(t, isUndeadOrFiend("Fiend"))
	assert.False(t, isUndeadOrFiend("humanoid"))
	assert.False(t, isUndeadOrFiend(""))
}

// Edge case: ordinal for various levels
func TestOrdinal(t *testing.T) {
	assert.Equal(t, "1st", ordinal(1))
	assert.Equal(t, "2nd", ordinal(2))
	assert.Equal(t, "3rd", ordinal(3))
	assert.Equal(t, "4th", ordinal(4))
	assert.Equal(t, "5th", ordinal(5))
	assert.Equal(t, "9th", ordinal(9))
}

// TDD Cycle 1: SmiteDiceCount returns base dice count for slot level
func TestSmiteDiceCount(t *testing.T) {
	tests := []struct {
		slotLevel int
		expected  int
	}{
		{1, 2}, // 1st level = 2d8
		{2, 3}, // 2nd level = 3d8
		{3, 4}, // 3rd level = 4d8
		{4, 5}, // 4th level = 5d8 (max)
		{5, 5}, // 5th level = still 5d8 (cap)
		{6, 5}, // 6th level = still 5d8 (cap)
	}
	for _, tc := range tests {
		got := SmiteDiceCount(tc.slotLevel)
		assert.Equal(t, tc.expected, got, "slot level %d", tc.slotLevel)
	}
}
