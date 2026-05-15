package character

import (
	"reflect"
	"testing"
)

func TestMulticlassSpellSlots_FullCasterLevel1(t *testing.T) {
	// Level 1 full caster: 2 first-level slots
	got := MulticastSpellSlots(1)
	want := map[int]int{1: 2}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MulticastSpellSlots(1) = %v, want %v", got, want)
	}
}

func TestMulticlassSpellSlots_FullCasterLevel5(t *testing.T) {
	// Level 5: 4/3/2
	got := MulticastSpellSlots(5)
	want := map[int]int{1: 4, 2: 3, 3: 2}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MulticastSpellSlots(5) = %v, want %v", got, want)
	}
}

func TestMulticlassSpellSlots_FullCasterLevel20(t *testing.T) {
	got := MulticastSpellSlots(20)
	want := map[int]int{1: 4, 2: 3, 3: 3, 4: 3, 5: 3, 6: 2, 7: 2, 8: 1, 9: 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MulticastSpellSlots(20) = %v, want %v", got, want)
	}
}

func TestMulticlassSpellSlots_Level0(t *testing.T) {
	got := MulticastSpellSlots(0)
	if got != nil {
		t.Errorf("MulticastSpellSlots(0) = %v, want nil", got)
	}
}

func TestCalculateCasterLevel_FullCaster(t *testing.T) {
	classes := []ClassEntry{{Class: "wizard", Level: 5}}
	spellcasting := map[string]ClassSpellcasting{
		"wizard": {SlotProgression: "full"},
	}
	got := CalculateCasterLevel(classes, spellcasting)
	if got != 5 {
		t.Errorf("CalculateCasterLevel full caster = %d, want 5", got)
	}
}

func TestCalculateCasterLevel_HalfCaster(t *testing.T) {
	classes := []ClassEntry{{Class: "paladin", Level: 5}}
	spellcasting := map[string]ClassSpellcasting{
		"paladin": {SlotProgression: "half"},
	}
	got := CalculateCasterLevel(classes, spellcasting)
	if got != 2 {
		t.Errorf("CalculateCasterLevel half caster = %d, want 2", got)
	}
}

func TestCalculateCasterLevel_ThirdCaster(t *testing.T) {
	classes := []ClassEntry{{Class: "fighter", Subclass: "eldritch-knight", Level: 9}}
	spellcasting := map[string]ClassSpellcasting{
		"fighter": {SlotProgression: "third"},
	}
	got := CalculateCasterLevel(classes, spellcasting)
	if got != 3 {
		t.Errorf("CalculateCasterLevel third caster = %d, want 3", got)
	}
}

func TestCalculateCasterLevel_Multiclass(t *testing.T) {
	// Wizard 5 (full) + Paladin 4 (half) = 5 + 2 = 7
	classes := []ClassEntry{
		{Class: "wizard", Level: 5},
		{Class: "paladin", Level: 4},
	}
	spellcasting := map[string]ClassSpellcasting{
		"wizard":  {SlotProgression: "full"},
		"paladin": {SlotProgression: "half"},
	}
	got := CalculateCasterLevel(classes, spellcasting)
	if got != 7 {
		t.Errorf("CalculateCasterLevel multiclass = %d, want 7", got)
	}
}

func TestCalculateCasterLevel_NonCaster(t *testing.T) {
	classes := []ClassEntry{{Class: "barbarian", Level: 10}}
	spellcasting := map[string]ClassSpellcasting{
		"barbarian": {SlotProgression: "none"},
	}
	got := CalculateCasterLevel(classes, spellcasting)
	if got != 0 {
		t.Errorf("CalculateCasterLevel non-caster = %d, want 0", got)
	}
}

func TestCalculateCasterLevel_PactIgnored(t *testing.T) {
	// Warlock pact slots are separate, don't contribute to multiclass caster level
	classes := []ClassEntry{{Class: "warlock", Level: 5}}
	spellcasting := map[string]ClassSpellcasting{
		"warlock": {SlotProgression: "pact"},
	}
	got := CalculateCasterLevel(classes, spellcasting)
	if got != 0 {
		t.Errorf("CalculateCasterLevel pact = %d, want 0", got)
	}
}

func TestCalculateSpellSlots_SingleClassFullCaster(t *testing.T) {
	// Single-class wizard 5 should use own progression (same as multiclass table for full casters)
	classes := []ClassEntry{{Class: "wizard", Level: 5}}
	spellcasting := map[string]ClassSpellcasting{
		"wizard": {SlotProgression: "full"},
	}
	got := CalculateSpellSlots(classes, spellcasting)
	want := map[int]int{1: 4, 2: 3, 3: 2}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CalculateSpellSlots single full = %v, want %v", got, want)
	}
}

func TestCalculateSpellSlots_NonCasterReturnsNil(t *testing.T) {
	classes := []ClassEntry{{Class: "barbarian", Level: 10}}
	spellcasting := map[string]ClassSpellcasting{
		"barbarian": {SlotProgression: "none"},
	}
	got := CalculateSpellSlots(classes, spellcasting)
	if got != nil {
		t.Errorf("CalculateSpellSlots non-caster = %v, want nil", got)
	}
}

func TestCalculateCasterLevel_MissingSpellcastingEntry(t *testing.T) {
	classes := []ClassEntry{{Class: "monk", Level: 5}}
	spellcasting := map[string]ClassSpellcasting{} // monk not in map
	got := CalculateCasterLevel(classes, spellcasting)
	if got != 0 {
		t.Errorf("CalculateCasterLevel missing entry = %d, want 0", got)
	}
}

func TestMulticlassSpellSlots_OutOfRange(t *testing.T) {
	got := MulticastSpellSlots(21)
	if got != nil {
		t.Errorf("MulticastSpellSlots(21) = %v, want nil", got)
	}
	got = MulticastSpellSlots(-1)
	if got != nil {
		t.Errorf("MulticastSpellSlots(-1) = %v, want nil", got)
	}
}

func TestCalculateSpellSlots_MulticlassSlots(t *testing.T) {
	// Wizard 3 + Cleric 3 = caster level 6
	classes := []ClassEntry{
		{Class: "wizard", Level: 3},
		{Class: "cleric", Level: 3},
	}
	spellcasting := map[string]ClassSpellcasting{
		"wizard": {SlotProgression: "full"},
		"cleric": {SlotProgression: "full"},
	}
	got := CalculateSpellSlots(classes, spellcasting)
	want := map[int]int{1: 4, 2: 3, 3: 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CalculateSpellSlots multiclass = %v, want %v", got, want)
	}
}

func TestPactMagicSlotsForLevel(t *testing.T) {
	tests := []struct {
		name      string
		level     int
		wantSlots PactMagicSlots
	}{
		{"level 0 returns zero", 0, PactMagicSlots{}},
		{"level 1: 1 slot, 1st level", 1, PactMagicSlots{SlotLevel: 1, Current: 1, Max: 1}},
		{"level 2: 2 slots, 1st level", 2, PactMagicSlots{SlotLevel: 1, Current: 2, Max: 2}},
		{"level 3: 2 slots, 2nd level", 3, PactMagicSlots{SlotLevel: 2, Current: 2, Max: 2}},
		{"level 4: 2 slots, 2nd level", 4, PactMagicSlots{SlotLevel: 2, Current: 2, Max: 2}},
		{"level 5: 2 slots, 3rd level", 5, PactMagicSlots{SlotLevel: 3, Current: 2, Max: 2}},
		{"level 6: 2 slots, 3rd level", 6, PactMagicSlots{SlotLevel: 3, Current: 2, Max: 2}},
		{"level 7: 2 slots, 4th level", 7, PactMagicSlots{SlotLevel: 4, Current: 2, Max: 2}},
		{"level 8: 2 slots, 4th level", 8, PactMagicSlots{SlotLevel: 4, Current: 2, Max: 2}},
		{"level 9: 2 slots, 5th level", 9, PactMagicSlots{SlotLevel: 5, Current: 2, Max: 2}},
		{"level 10: 2 slots, 5th level", 10, PactMagicSlots{SlotLevel: 5, Current: 2, Max: 2}},
		{"level 16: 2 slots, 5th level", 16, PactMagicSlots{SlotLevel: 5, Current: 2, Max: 2}},
		{"level 17: 3 slots, 5th level", 17, PactMagicSlots{SlotLevel: 5, Current: 3, Max: 3}},
		{"level 18: 3 slots, 5th level", 18, PactMagicSlots{SlotLevel: 5, Current: 3, Max: 3}},
		{"level 19: 4 slots, 5th level", 19, PactMagicSlots{SlotLevel: 5, Current: 4, Max: 4}},
		{"level 20: 4 slots, 5th level", 20, PactMagicSlots{SlotLevel: 5, Current: 4, Max: 4}},
		{"level 21 out of range returns zero", 21, PactMagicSlots{}},
		{"level -1 out of range returns zero", -1, PactMagicSlots{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PactMagicSlotsForLevel(tc.level)
			if got != tc.wantSlots {
				t.Errorf("PactMagicSlotsForLevel(%d) = %+v, want %+v", tc.level, got, tc.wantSlots)
			}
		})
	}
}

func TestCalculateSpellSlots_SingleClassHalfCaster(t *testing.T) {
	tests := []struct {
		name  string
		level int
		want  map[int]int
	}{
		{"Paladin 3 gets 3 first-level slots", 3, map[int]int{1: 3}},
		{"Paladin 5 gets 4/2", 5, map[int]int{1: 4, 2: 2}},
		{"Paladin 9 gets 4/3/2", 9, map[int]int{1: 4, 2: 3, 3: 2}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			classes := []ClassEntry{{Class: "paladin", Level: tc.level}}
			spellcasting := map[string]ClassSpellcasting{
				"paladin": {SlotProgression: "half"},
			}
			got := CalculateSpellSlots(classes, spellcasting)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("CalculateSpellSlots(paladin %d) = %v, want %v", tc.level, got, tc.want)
			}
		})
	}
}
