package character

import (
	"testing"
)

func TestHitDieValue(t *testing.T) {
	tests := []struct {
		die  string
		want int
	}{
		{"d6", 6}, {"d8", 8}, {"d10", 10}, {"d12", 12}, {"d20", 0}, {"", 0},
	}
	for _, tt := range tests {
		got := HitDieValue(tt.die)
		if got != tt.want {
			t.Errorf("HitDieValue(%q) = %d, want %d", tt.die, got, tt.want)
		}
	}
}

func TestAbilityScoresGet(t *testing.T) {
	scores := AbilityScores{STR: 10, DEX: 12, CON: 14, INT: 16, WIS: 18, CHA: 20}
	tests := []struct {
		ability string
		want    int
	}{
		{"str", 10}, {"STR", 10},
		{"dex", 12}, {"DEX", 12},
		{"con", 14}, {"CON", 14},
		{"int", 16}, {"INT", 16},
		{"wis", 18}, {"WIS", 18},
		{"cha", 20}, {"CHA", 20},
		{"unknown", 0},
	}
	for _, tt := range tests {
		got := scores.Get(tt.ability)
		if got != tt.want {
			t.Errorf("Get(%q) = %d, want %d", tt.ability, got, tt.want)
		}
	}
}

func TestProficiencyBonus(t *testing.T) {
	tests := []struct {
		level int
		want  int
	}{
		{1, 2}, {4, 2},
		{5, 3}, {8, 3},
		{9, 4}, {12, 4},
		{13, 5}, {16, 5},
		{17, 6}, {20, 6},
	}
	for _, tt := range tests {
		got := ProficiencyBonus(tt.level)
		if got != tt.want {
			t.Errorf("ProficiencyBonus(%d) = %d, want %d", tt.level, got, tt.want)
		}
	}
}

func TestAbilityModifier(t *testing.T) {
	tests := []struct {
		score int
		want  int
	}{
		{1, -5}, {3, -4}, {8, -1}, {9, -1},
		{10, 0}, {11, 0}, {12, 1}, {14, 2},
		{15, 2}, {20, 5}, {30, 10},
	}
	for _, tt := range tests {
		got := AbilityModifier(tt.score)
		if got != tt.want {
			t.Errorf("AbilityModifier(%d) = %d, want %d", tt.score, got, tt.want)
		}
	}
}

func TestProficiencyBonus_InvalidLevel(t *testing.T) {
	if got := ProficiencyBonus(0); got != 0 {
		t.Errorf("ProficiencyBonus(0) = %d, want 0", got)
	}
	if got := ProficiencyBonus(-1); got != 0 {
		t.Errorf("ProficiencyBonus(-1) = %d, want 0", got)
	}
	if got := ProficiencyBonus(21); got != 0 {
		t.Errorf("ProficiencyBonus(21) = %d, want 0", got)
	}
}

func TestTotalLevel(t *testing.T) {
	classes := []ClassEntry{
		{Class: "fighter", Level: 5},
		{Class: "wizard", Level: 3},
	}
	got := TotalLevel(classes)
	if got != 8 {
		t.Errorf("TotalLevel() = %d, want 8", got)
	}
}

func TestTotalLevel_SingleClass(t *testing.T) {
	classes := []ClassEntry{{Class: "rogue", Level: 10}}
	got := TotalLevel(classes)
	if got != 10 {
		t.Errorf("TotalLevel() = %d, want 10", got)
	}
}

func TestTotalLevel_Empty(t *testing.T) {
	got := TotalLevel(nil)
	if got != 0 {
		t.Errorf("TotalLevel(nil) = %d, want 0", got)
	}
}

func TestCalculateHP_EmptyClasses(t *testing.T) {
	got := CalculateHP(nil, nil, AbilityScores{})
	if got != 0 {
		t.Errorf("CalculateHP empty = %d, want 0", got)
	}
}

func TestCalculateHP_SingleClass(t *testing.T) {
	// Level 5 fighter (d10), CON 14 (+2)
	// HP = 10 (max at level 1) + 4*(6+1) + 5*2 = 10 + 28 + 10 = 38
	// Wait: average for d10 = 5, so 5+1=6 per subsequent level
	// HP = 10 + 4*6 + 5*2 = 10 + 24 + 10 = 44
	classes := []ClassEntry{{Class: "fighter", Level: 5}}
	hitDice := map[string]string{"fighter": "d10"}
	scores := AbilityScores{CON: 14}

	got := CalculateHP(classes, hitDice, scores)
	// First level: max die = 10
	// Levels 2-5: average+1 = 5+1 = 6 each = 24
	// CON bonus: +2 * 5 levels = 10
	// Total: 10 + 24 + 10 = 44
	if got != 44 {
		t.Errorf("CalculateHP single class fighter 5 = %d, want 44", got)
	}
}

func TestCalculateHP_Multiclass(t *testing.T) {
	// Fighter 3 / Wizard 2, CON 12 (+1)
	// Fighter is first class: level 1 = max d10 = 10
	// Fighter levels 2-3: 6 each = 12
	// Wizard levels 1-2: average d6 = 3+1 = 4 each = 8
	// CON: +1 * 5 = 5
	// Total: 10 + 12 + 8 + 5 = 35
	classes := []ClassEntry{
		{Class: "fighter", Level: 3},
		{Class: "wizard", Level: 2},
	}
	hitDice := map[string]string{"fighter": "d10", "wizard": "d6"}
	scores := AbilityScores{CON: 12}

	got := CalculateHP(classes, hitDice, scores)
	if got != 35 {
		t.Errorf("CalculateHP multiclass = %d, want 35", got)
	}
}

func TestCalculateHP_NegativeCON(t *testing.T) {
	// Level 1 wizard, CON 8 (-1)
	// HP = 6 + (-1) = 5
	classes := []ClassEntry{{Class: "wizard", Level: 1}}
	hitDice := map[string]string{"wizard": "d6"}
	scores := AbilityScores{CON: 8}

	got := CalculateHP(classes, hitDice, scores)
	if got != 5 {
		t.Errorf("CalculateHP low CON = %d, want 5", got)
	}
}

func TestCalculateHP_MinimumHP(t *testing.T) {
	// Ensure HP is at least 1 even with very low CON
	classes := []ClassEntry{{Class: "wizard", Level: 1}}
	hitDice := map[string]string{"wizard": "d6"}
	scores := AbilityScores{CON: 1} // -5 modifier

	got := CalculateHP(classes, hitDice, scores)
	// HP = 6 + (-5) = 1 (minimum 1)
	if got != 1 {
		t.Errorf("CalculateHP minimum = %d, want 1", got)
	}
}

func TestCalculateHP_MinimumHP_Floor(t *testing.T) {
	// Level 3 wizard with CON 1 (-5 modifier) to produce hp < 1
	// HP = 6 (max d6 at level 1) + 2*4 (avg for levels 2-3) + (-5)*3 = 6 + 8 - 15 = -1
	// Should be floored to 1
	classes := []ClassEntry{{Class: "wizard", Level: 3}}
	hitDice := map[string]string{"wizard": "d6"}
	scores := AbilityScores{CON: 1} // -5 modifier

	got := CalculateHP(classes, hitDice, scores)
	if got != 1 {
		t.Errorf("CalculateHP floor = %d, want 1", got)
	}
}

func TestCalculateAC_NoArmor(t *testing.T) {
	// No armor, no formula: 10 + DEX mod
	scores := AbilityScores{DEX: 14}
	got := CalculateAC(scores, nil, false, "")
	if got != 12 {
		t.Errorf("CalculateAC no armor = %d, want 12", got)
	}
}

func TestCalculateAC_LightArmor(t *testing.T) {
	// Leather armor: ac_base=11, dex bonus, no cap
	scores := AbilityScores{DEX: 16}
	armor := &ArmorInfo{ACBase: 11, DexBonus: true, DexMax: 0}
	got := CalculateAC(scores, armor, false, "")
	if got != 14 {
		t.Errorf("CalculateAC light armor = %d, want 14", got)
	}
}

func TestCalculateAC_MediumArmor(t *testing.T) {
	// Scale mail: ac_base=14, dex bonus, max 2
	scores := AbilityScores{DEX: 18} // +4 mod, but capped at +2
	armor := &ArmorInfo{ACBase: 14, DexBonus: true, DexMax: 2}
	got := CalculateAC(scores, armor, false, "")
	if got != 16 {
		t.Errorf("CalculateAC medium armor = %d, want 16", got)
	}
}

func TestCalculateAC_HeavyArmor(t *testing.T) {
	// Chain mail: ac_base=16, no dex bonus
	scores := AbilityScores{DEX: 20}
	armor := &ArmorInfo{ACBase: 16, DexBonus: false}
	got := CalculateAC(scores, armor, false, "")
	if got != 16 {
		t.Errorf("CalculateAC heavy armor = %d, want 16", got)
	}
}

func TestCalculateAC_WithShield(t *testing.T) {
	// Chain mail + shield
	scores := AbilityScores{DEX: 10}
	armor := &ArmorInfo{ACBase: 16, DexBonus: false}
	got := CalculateAC(scores, armor, true, "")
	if got != 18 {
		t.Errorf("CalculateAC with shield = %d, want 18", got)
	}
}

func TestCalculateAC_MagicBonus(t *testing.T) {
	// Plate (AC 18) + Cloak of Protection (+1) + Ring of Protection (+1)
	scores := AbilityScores{DEX: 10}
	armor := &ArmorInfo{ACBase: 18, DexBonus: false}
	got := CalculateAC(scores, armor, false, "", 1, 1)
	if got != 20 {
		t.Errorf("CalculateAC with magic bonuses = %d, want 20", got)
	}
}

func TestCalculateAC_UnarmoredDefense_Monk(t *testing.T) {
	// Monk: 10 + DEX + WIS, no armor
	scores := AbilityScores{DEX: 16, WIS: 14}
	got := CalculateAC(scores, nil, false, "10 + DEX + WIS")
	// 10 + 3 + 2 = 15
	if got != 15 {
		t.Errorf("CalculateAC monk unarmored = %d, want 15", got)
	}
}

func TestCalculateAC_UnarmoredDefense_Barbarian(t *testing.T) {
	// Barbarian: 10 + DEX + CON, no armor
	scores := AbilityScores{DEX: 14, CON: 16}
	got := CalculateAC(scores, nil, false, "10 + DEX + CON")
	// 10 + 2 + 3 = 15
	if got != 15 {
		t.Errorf("CalculateAC barbarian unarmored = %d, want 15", got)
	}
}

func TestCalculateAC_UnarmoredDefense_WithShield(t *testing.T) {
	// Monk unarmored defense + shield
	scores := AbilityScores{DEX: 16, WIS: 14}
	got := CalculateAC(scores, nil, true, "10 + DEX + WIS")
	// 10 + 3 + 2 + 2 = 17
	if got != 17 {
		t.Errorf("CalculateAC monk + shield = %d, want 17", got)
	}
}

func TestCalculateAC_FormulaIgnoredWhenArmored(t *testing.T) {
	// Per 5e rules: Unarmored Defense does NOT apply when wearing armor.
	// Even if formula AC would be higher, armor AC is used.
	scores := AbilityScores{DEX: 14, CON: 20}
	armor := &ArmorInfo{ACBase: 11, DexBonus: true, DexMax: 0}
	// Armor AC: 11 + 2 = 13
	// Formula AC would be: 10 + 2 + 5 = 17 (but ignored)
	// Result: 13 (armor only)
	got := CalculateAC(scores, armor, false, "10 + DEX + CON")
	if got != 13 {
		t.Errorf("CalculateAC formula ignored when armored = %d, want 13", got)
	}
}

func TestCalculateAC_NoArmorNoFormula_LowDex(t *testing.T) {
	scores := AbilityScores{DEX: 8}
	got := CalculateAC(scores, nil, false, "")
	if got != 9 {
		t.Errorf("CalculateAC low dex = %d, want 9", got)
	}
}

func TestCalculateAC_FormulaWithINT(t *testing.T) {
	scores := AbilityScores{INT: 16}
	got := CalculateAC(scores, nil, false, "10 + INT")
	if got != 13 {
		t.Errorf("CalculateAC INT formula = %d, want 13", got)
	}
}

func TestCalculateAC_FormulaWithSTR(t *testing.T) {
	scores := AbilityScores{STR: 18}
	got := CalculateAC(scores, nil, false, "10 + STR")
	if got != 14 {
		t.Errorf("CalculateAC STR formula = %d, want 14", got)
	}
}

func TestCalculateAC_FormulaWithCHA(t *testing.T) {
	scores := AbilityScores{CHA: 14}
	got := CalculateAC(scores, nil, false, "10 + CHA")
	if got != 12 {
		t.Errorf("CalculateAC CHA formula = %d, want 12", got)
	}
}

func TestCalculateAC_ArmorBetterThanFormula(t *testing.T) {
	// When armor is equipped, formula is ignored entirely (same result here)
	scores := AbilityScores{DEX: 10, CON: 10}
	armor := &ArmorInfo{ACBase: 18, DexBonus: false}
	// Armor AC: 18, formula ignored
	got := CalculateAC(scores, armor, false, "10 + DEX + CON")
	if got != 18 {
		t.Errorf("CalculateAC armor when formula present = %d, want 18", got)
	}
}
