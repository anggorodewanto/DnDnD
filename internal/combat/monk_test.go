package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TDD Cycle 1: IsMonkWeapon

func TestIsMonkWeapon_UnarmedStrike(t *testing.T) {
	weapon := UnarmedStrike()
	if !IsMonkWeapon(weapon) {
		t.Error("unarmed strike should be a monk weapon")
	}
}

func TestIsMonkWeapon_Shortsword(t *testing.T) {
	weapon := refdata.Weapon{
		ID:         "shortsword",
		Name:       "Shortsword",
		Damage:     "1d6",
		DamageType: "piercing",
		WeaponType: "martial_melee",
		Properties: []string{"finesse", "light"},
	}
	if !IsMonkWeapon(weapon) {
		t.Error("shortsword should be a monk weapon")
	}
}

func TestIsMonkWeapon_SimpleMelee(t *testing.T) {
	// Dagger: simple melee, light, finesse, thrown — no heavy or two-handed
	weapon := refdata.Weapon{
		ID:         "dagger",
		Name:       "Dagger",
		Damage:     "1d4",
		DamageType: "piercing",
		WeaponType: "simple_melee",
		Properties: []string{"finesse", "light", "thrown"},
	}
	if !IsMonkWeapon(weapon) {
		t.Error("dagger (simple melee, no heavy/two-handed) should be a monk weapon")
	}
}

func TestIsMonkWeapon_SimpleMeleeHeavy(t *testing.T) {
	// Greatclub: simple melee, two-handed — NOT a monk weapon
	weapon := refdata.Weapon{
		ID:         "greatclub",
		Name:       "Greatclub",
		Damage:     "1d8",
		DamageType: "bludgeoning",
		WeaponType: "simple_melee",
		Properties: []string{"two-handed"},
	}
	if IsMonkWeapon(weapon) {
		t.Error("greatclub (two-handed) should NOT be a monk weapon")
	}
}

func TestIsMonkWeapon_MartialMeleeNotShortsword(t *testing.T) {
	// Longsword: martial melee — NOT a monk weapon
	weapon := refdata.Weapon{
		ID:         "longsword",
		Name:       "Longsword",
		Damage:     "1d8",
		DamageType: "slashing",
		WeaponType: "martial_melee",
		Properties: []string{"versatile"},
	}
	if IsMonkWeapon(weapon) {
		t.Error("longsword (martial, not shortsword) should NOT be a monk weapon")
	}
}

func TestIsMonkWeapon_Ranged(t *testing.T) {
	weapon := refdata.Weapon{
		ID:         "shortbow",
		Name:       "Shortbow",
		WeaponType: "simple_ranged",
	}
	if IsMonkWeapon(weapon) {
		t.Error("ranged weapon should NOT be a monk weapon")
	}
}

// TDD Cycle 3: Monk DEX/STR auto-select (abilityModForWeapon with MonkLevel)

func TestAbilityModForWeapon_MonkDexHigher(t *testing.T) {
	// Monk with DEX 16 (+3), STR 10 (+0) using a club (simple melee, no finesse)
	scores := AbilityScores{Str: 10, Dex: 16}
	weapon := refdata.Weapon{
		ID:         "club",
		Name:       "Club",
		Damage:     "1d4",
		WeaponType: "simple_melee",
	}
	input := AttackInput{Scores: scores, Weapon: weapon, MonkLevel: 5}
	got := abilityModForWeapon(input.Scores, input.Weapon, input.MonkLevel)
	if got != 3 {
		t.Errorf("monk with DEX 16 using club: got %d, want 3", got)
	}
}

func TestAbilityModForWeapon_MonkStrHigher(t *testing.T) {
	// Monk with STR 16 (+3), DEX 10 (+0) using a club
	scores := AbilityScores{Str: 16, Dex: 10}
	weapon := refdata.Weapon{
		ID:         "club",
		Name:       "Club",
		Damage:     "1d4",
		WeaponType: "simple_melee",
	}
	input := AttackInput{Scores: scores, Weapon: weapon, MonkLevel: 5}
	got := abilityModForWeapon(input.Scores, input.Weapon, input.MonkLevel)
	if got != 3 {
		t.Errorf("monk with STR 16 using club: got %d, want 3", got)
	}
}

func TestAbilityModForWeapon_NonMonkUnchanged(t *testing.T) {
	// Non-monk with club uses STR only
	scores := AbilityScores{Str: 10, Dex: 16}
	weapon := refdata.Weapon{
		ID:         "club",
		Name:       "Club",
		Damage:     "1d4",
		WeaponType: "simple_melee",
	}
	got := abilityModForWeapon(scores, weapon, 0)
	if got != 0 {
		t.Errorf("non-monk with STR 10 using club: got %d, want 0 (STR mod)", got)
	}
}

func TestAbilityModForWeapon_MonkNonMonkWeapon(t *testing.T) {
	// Monk using a longsword (not a monk weapon) — uses STR normally
	scores := AbilityScores{Str: 10, Dex: 16}
	weapon := refdata.Weapon{
		ID:         "longsword",
		Name:       "Longsword",
		Damage:     "1d8",
		WeaponType: "martial_melee",
		Properties: []string{"versatile"},
	}
	got := abilityModForWeapon(scores, weapon, 5)
	if got != 0 {
		t.Errorf("monk with longsword (non-monk weapon): got %d, want 0 (STR mod)", got)
	}
}

// TDD Cycle 4: MonkDamageExpression — martial arts die upgrade

func TestMonkDamageExpression_UnarmedStrike(t *testing.T) {
	// Monk level 5: unarmed strike should use 1d6 martial arts die
	weapon := UnarmedStrike()
	expr := MonkDamageExpression(weapon, 5)
	if expr != "1d6" {
		t.Errorf("MonkDamageExpression(unarmed, 5) = %q, want %q", expr, "1d6")
	}
}

func TestMonkDamageExpression_UnarmedLevel1(t *testing.T) {
	weapon := UnarmedStrike()
	expr := MonkDamageExpression(weapon, 1)
	if expr != "1d4" {
		t.Errorf("MonkDamageExpression(unarmed, 1) = %q, want %q", expr, "1d4")
	}
}

func TestMonkDamageExpression_WeaponWithSmallerDie(t *testing.T) {
	// Club does 1d4, monk level 5 martial arts die is 1d6 -> use 1d6
	weapon := refdata.Weapon{
		ID:         "club",
		Name:       "Club",
		Damage:     "1d4",
		WeaponType: "simple_melee",
	}
	expr := MonkDamageExpression(weapon, 5)
	if expr != "1d6" {
		t.Errorf("MonkDamageExpression(club 1d4, level 5) = %q, want %q", expr, "1d6")
	}
}

func TestMonkDamageExpression_WeaponWithLargerDie(t *testing.T) {
	// Shortsword does 1d6, monk level 1 martial arts die is 1d4 -> keep 1d6
	weapon := refdata.Weapon{
		ID:         "shortsword",
		Name:       "Shortsword",
		Damage:     "1d6",
		WeaponType: "martial_melee",
		Properties: []string{"finesse", "light"},
	}
	expr := MonkDamageExpression(weapon, 1)
	if expr != "1d6" {
		t.Errorf("MonkDamageExpression(shortsword 1d6, level 1) = %q, want %q", expr, "1d6")
	}
}

func TestMonkDamageExpression_WeaponEqualDie(t *testing.T) {
	// Shortsword 1d6, monk level 5 martial arts 1d6 -> keep 1d6
	weapon := refdata.Weapon{
		ID:         "shortsword",
		Name:       "Shortsword",
		Damage:     "1d6",
		WeaponType: "martial_melee",
		Properties: []string{"finesse", "light"},
	}
	expr := MonkDamageExpression(weapon, 5)
	if expr != "1d6" {
		t.Errorf("MonkDamageExpression(shortsword 1d6, level 5) = %q, want %q", expr, "1d6")
	}
}

// TDD Cycle 5: Unarmored Movement feature

func TestUnarmoredMovementSpeed(t *testing.T) {
	tests := []struct {
		level int
		want  int
	}{
		{2, 10},
		{5, 10},
		{6, 15},
		{9, 15},
		{10, 20},
		{13, 20},
		{14, 25},
		{17, 25},
		{18, 30},
		{20, 30},
	}
	for _, tt := range tests {
		got := UnarmoredMovementBonus(tt.level)
		if got != tt.want {
			t.Errorf("UnarmoredMovementBonus(%d) = %d, want %d", tt.level, got, tt.want)
		}
	}
}

func TestUnarmoredMovementFeature(t *testing.T) {
	fd := UnarmoredMovementFeature(6)
	if fd.Name != "Unarmored Movement" {
		t.Errorf("Name = %q, want %q", fd.Name, "Unarmored Movement")
	}
	if len(fd.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(fd.Effects))
	}
	e := fd.Effects[0]
	if e.Type != EffectModifySpeed {
		t.Errorf("Type = %q, want %q", e.Type, EffectModifySpeed)
	}
	if e.Modifier != 15 {
		t.Errorf("Modifier = %d, want 15 for level 6", e.Modifier)
	}
	if !e.Conditions.NotWearingArmor {
		t.Error("expected NotWearingArmor condition")
	}
}

func TestNotWearingArmorCondition(t *testing.T) {
	effect := Effect{
		Type:     EffectModifySpeed,
		Trigger:  TriggerOnTurnStart,
		Modifier: 10,
		Conditions: EffectConditions{
			NotWearingArmor: true,
		},
	}

	t.Run("not wearing armor passes", func(t *testing.T) {
		if !EvaluateConditions(effect, EffectContext{WearingArmor: false}) {
			t.Error("should pass without armor")
		}
	})
	t.Run("wearing armor fails", func(t *testing.T) {
		if EvaluateConditions(effect, EffectContext{WearingArmor: true}) {
			t.Error("should fail with armor")
		}
	})
}

func TestUnarmoredMovementApplies(t *testing.T) {
	features := []FeatureDefinition{UnarmoredMovementFeature(2)}
	// No armor: should get +10 speed
	result := ProcessEffects(features, TriggerOnTurnStart, EffectContext{WearingArmor: false})
	if result.SpeedModifier != 10 {
		t.Errorf("no armor: SpeedModifier = %d, want 10", result.SpeedModifier)
	}
	// With armor: should NOT get speed bonus
	result = ProcessEffects(features, TriggerOnTurnStart, EffectContext{WearingArmor: true})
	if result.SpeedModifier != 0 {
		t.Errorf("with armor: SpeedModifier = %d, want 0", result.SpeedModifier)
	}
}

func TestUnarmoredMovement_ShieldBlocksBonus(t *testing.T) {
	features := []FeatureDefinition{UnarmoredMovementFeature(2)}
	// Shield equipped, no armor: should NOT get speed bonus
	result := ProcessEffects(features, TriggerOnTurnStart, EffectContext{WearingArmor: false, HasShield: true})
	if result.SpeedModifier != 0 {
		t.Errorf("with shield: SpeedModifier = %d, want 0", result.SpeedModifier)
	}
	// No shield, no armor: should get speed bonus
	result = ProcessEffects(features, TriggerOnTurnStart, EffectContext{WearingArmor: false, HasShield: false})
	if result.SpeedModifier != 10 {
		t.Errorf("no shield no armor: SpeedModifier = %d, want 10", result.SpeedModifier)
	}
}

// TDD Cycle 6: BuildFeatureDefinitions for monk

func TestBuildFeatureDefinitions_Monk(t *testing.T) {
	classes := []CharacterClass{{Class: "Monk", Level: 5}}
	features := []CharacterFeature{
		{Name: "Martial Arts", MechanicalEffect: "martial_arts_d4,bonus_action_unarmed_strike"},
		{Name: "Unarmored Movement", MechanicalEffect: "speed_plus_10"},
	}

	defs := BuildFeatureDefinitions(classes, features)

	found := map[string]bool{}
	for _, d := range defs {
		found[d.Name] = true
	}
	if !found["Unarmored Movement"] {
		t.Error("expected Unarmored Movement feature")
	}
}

func TestBuildFeatureDefinitions_MonkLevel10Speed(t *testing.T) {
	classes := []CharacterClass{{Class: "Monk", Level: 10}}
	features := []CharacterFeature{
		{Name: "Unarmored Movement", MechanicalEffect: "speed_plus_10"},
	}
	defs := BuildFeatureDefinitions(classes, features)

	// Find the unarmored movement feature and check speed modifier
	for _, d := range defs {
		if d.Name == "Unarmored Movement" {
			if len(d.Effects) == 0 {
				t.Fatal("no effects")
			}
			if d.Effects[0].Modifier != 20 {
				t.Errorf("speed modifier = %d, want 20 for monk level 10", d.Effects[0].Modifier)
			}
			return
		}
	}
	t.Error("Unarmored Movement feature not found")
}

// TDD Cycle 7: Integration test — monk martial arts damage in resolveWeaponDamage

func TestResolveAttack_MonkUnarmedStrikeUsesMartialArtsDie(t *testing.T) {
	callNum := 0
	roller := dice.NewRoller(func(max int) int {
		callNum++
		if max == 20 {
			return 15
		}
		return 3 // damage die
	})
	scores := AbilityScores{Str: 10, Dex: 16} // DEX +3
	weapon := UnarmedStrike()

	input := AttackInput{
		AttackerName: "Monk",
		TargetName:   "Goblin",
		TargetAC:     12,
		Weapon:       weapon,
		Scores:       scores,
		ProfBonus:    2,
		DistanceFt:   5,
		MonkLevel:    5,
	}

	result, err := ResolveAttack(input, roller)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Hit {
		t.Fatal("expected hit")
	}
	// Monk level 5: martial arts die is 1d6, DEX mod +3
	// Damage should be die roll (3) + 3 = 6
	if result.DamageTotal != 6 {
		t.Errorf("DamageTotal = %d, want 6 (1d6+3)", result.DamageTotal)
	}
}

func TestResolveAttack_MonkClubUpgradedDie(t *testing.T) {
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18
		}
		return 4 // damage die
	})
	scores := AbilityScores{Str: 10, Dex: 14} // DEX +2
	weapon := refdata.Weapon{
		ID:         "club",
		Name:       "Club",
		Damage:     "1d4",
		DamageType: "bludgeoning",
		WeaponType: "simple_melee",
	}

	input := AttackInput{
		AttackerName: "Monk",
		TargetName:   "Orc",
		TargetAC:     10,
		Weapon:       weapon,
		Scores:       scores,
		ProfBonus:    2,
		DistanceFt:   5,
		MonkLevel:    5,
	}

	result, err := ResolveAttack(input, roller)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Hit {
		t.Fatal("expected hit")
	}
	// Club 1d4, monk level 5 martial arts 1d6 -> upgraded to 1d6
	// Damage: 4 + 2 = 6
	if result.DamageTotal != 6 {
		t.Errorf("DamageTotal = %d, want 6 (1d6+2 upgraded from 1d4)", result.DamageTotal)
	}
}

func TestResolveAttack_NonMonkUnarmedStrikeFlat(t *testing.T) {
	roller := dice.NewRoller(func(max int) int {
		return 15
	})
	scores := AbilityScores{Str: 14, Dex: 10} // STR +2
	weapon := UnarmedStrike()

	input := AttackInput{
		AttackerName: "Fighter",
		TargetName:   "Goblin",
		TargetAC:     12,
		Weapon:       weapon,
		Scores:       scores,
		ProfBonus:    2,
		DistanceFt:   5,
		MonkLevel:    0, // Not a monk
	}

	result, err := ResolveAttack(input, roller)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Hit {
		t.Fatal("expected hit")
	}
	// Non-monk unarmed: 1 + STR mod = 1 + 2 = 3
	if result.DamageTotal != 3 {
		t.Errorf("DamageTotal = %d, want 3 (1+STR)", result.DamageTotal)
	}
}

// TDD Cycle 8: ValidateMartialArtsBonusAttack

func TestValidateMartialArtsBonusAttack_Valid(t *testing.T) {
	err := ValidateMartialArtsBonusAttack(5, true)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateMartialArtsBonusAttack_NotMonk(t *testing.T) {
	err := ValidateMartialArtsBonusAttack(0, true)
	if err == nil {
		t.Error("expected error for non-monk")
	}
}

func TestValidateMartialArtsBonusAttack_NoAttackAction(t *testing.T) {
	err := ValidateMartialArtsBonusAttack(5, false)
	if err == nil {
		t.Error("expected error when attack action not used")
	}
}

func TestFormatMartialArtsBonusAttack(t *testing.T) {
	msg := FormatMartialArtsBonusAttack("Kira")
	if msg == "" {
		t.Error("expected non-empty format string")
	}
}

// TDD Cycle 9: splitMechanicalEffects

func TestSplitMechanicalEffects(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"martial_arts_d4,bonus_action_unarmed_strike", 2},
		{"speed_plus_10", 1},
		{"", 0},
		{"a,b,c", 3},
	}
	for _, tt := range tests {
		got := splitMechanicalEffects(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitMechanicalEffects(%q) returned %d items, want %d", tt.input, len(got), tt.want)
		}
	}
}

// TDD Cycle 10: MonkLevelFromJSON

func TestMonkLevelFromJSON(t *testing.T) {
	classesJSON := []byte(`[{"class":"Monk","level":8}]`)
	got := ClassLevelFromJSON(classesJSON, "Monk")
	if got != 8 {
		t.Errorf("ClassLevelFromJSON = %d, want 8", got)
	}
}

func TestMonkLevelFromJSON_NotMonk(t *testing.T) {
	classesJSON := []byte(`[{"class":"Fighter","level":5}]`)
	got := ClassLevelFromJSON(classesJSON, "Monk")
	if got != 0 {
		t.Errorf("ClassLevelFromJSON (fighter) = %d, want 0", got)
	}
}

func TestMonkLevelFromJSON_Empty(t *testing.T) {
	got := ClassLevelFromJSON(nil, "Monk")
	if got != 0 {
		t.Errorf("ClassLevelFromJSON (nil) = %d, want 0", got)
	}
}

// TDD Cycle 11: MartialArtsDieSides

func TestMartialArtsDieSides(t *testing.T) {
	tests := []struct {
		level int
		want  int
	}{
		{1, 4}, {4, 4}, {5, 6}, {10, 6}, {11, 8}, {16, 8}, {17, 10}, {20, 10},
	}
	for _, tt := range tests {
		got := MartialArtsDieSides(tt.level)
		if got != tt.want {
			t.Errorf("MartialArtsDieSides(%d) = %d, want %d", tt.level, got, tt.want)
		}
	}
}

// TDD Cycle 12: Edge cases

func TestIsMonkWeapon_SimpleMeleeWithHeavy(t *testing.T) {
	// Hypothetical simple melee with heavy — NOT a monk weapon
	weapon := refdata.Weapon{
		ID:         "heavy-club",
		Name:       "Heavy Club",
		WeaponType: "simple_melee",
		Properties: []string{"heavy"},
	}
	if IsMonkWeapon(weapon) {
		t.Error("simple melee with heavy should NOT be a monk weapon")
	}
}

func TestMonkDamageExpression_Level17(t *testing.T) {
	weapon := UnarmedStrike()
	expr := MonkDamageExpression(weapon, 17)
	if expr != "1d10" {
		t.Errorf("MonkDamageExpression(unarmed, 17) = %q, want %q", expr, "1d10")
	}
}

// Edge: MonkDamageExpression with unparseable damage
func TestMonkDamageExpression_UnparseableDamage(t *testing.T) {
	weapon := refdata.Weapon{
		ID:         "weird",
		Name:       "Weird",
		Damage:     "bad-damage",
		WeaponType: "simple_melee",
	}
	// Should fall back to martial arts die
	expr := MonkDamageExpression(weapon, 5)
	if expr != "1d6" {
		t.Errorf("MonkDamageExpression(bad damage, 5) = %q, want %q", expr, "1d6")
	}
}

// Edge: ClassLevelFromJSON with bad JSON
func TestMonkLevelFromJSON_BadJSON(t *testing.T) {
	got := ClassLevelFromJSON([]byte("bad json"), "Monk")
	if got != 0 {
		t.Errorf("ClassLevelFromJSON(bad) = %d, want 0", got)
	}
}

// Edge: ResolveAttack monk unarmed strike critical hit
func TestResolveAttack_MonkUnarmedCritical(t *testing.T) {
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 20 // nat 20
		}
		return 3 // damage die
	})
	scores := AbilityScores{Str: 10, Dex: 14} // DEX +2

	input := AttackInput{
		AttackerName: "Monk",
		TargetName:   "Goblin",
		TargetAC:     15,
		Weapon:       UnarmedStrike(),
		Scores:       scores,
		ProfBonus:    2,
		DistanceFt:   5,
		MonkLevel:    1,
	}

	result, err := ResolveAttack(input, roller)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.CriticalHit {
		t.Fatal("expected critical hit")
	}
	// Monk level 1: 1d4 martial arts die, critical doubles to 2d4
	// Damage: 3+3+2 = 8 (two dice + mod)
	if result.DamageTotal < 4 {
		t.Errorf("DamageTotal = %d, expected at least 4 on crit", result.DamageTotal)
	}
}

// Edge: ResolveAttack monk auto-crit (paralyzed target)
func TestResolveAttack_MonkUnarmedAutoCrit(t *testing.T) {
	roller := dice.NewRoller(func(max int) int {
		return 3
	})
	scores := AbilityScores{Str: 10, Dex: 14}

	input := AttackInput{
		AttackerName:   "Monk",
		TargetName:     "Goblin",
		TargetAC:       15,
		Weapon:         UnarmedStrike(),
		Scores:         scores,
		ProfBonus:      2,
		DistanceFt:     5,
		MonkLevel:      5,
		AutoCrit:       true,
		AutoCritReason: "target paralyzed within 5ft",
	}

	result, err := ResolveAttack(input, roller)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.CriticalHit {
		t.Fatal("expected critical hit")
	}
	if !result.AutoCrit {
		t.Fatal("expected auto-crit")
	}
}

// TDD Cycle 2: MartialArtsDie

func TestMartialArtsDie(t *testing.T) {
	tests := []struct {
		level int
		want  string
	}{
		{1, "1d4"},
		{2, "1d4"},
		{4, "1d4"},
		{5, "1d6"},
		{6, "1d6"},
		{10, "1d6"},
		{11, "1d8"},
		{12, "1d8"},
		{16, "1d8"},
		{17, "1d10"},
		{18, "1d10"},
		{20, "1d10"},
	}
	for _, tt := range tests {
		got := MartialArtsDie(tt.level)
		if got != tt.want {
			t.Errorf("MartialArtsDie(%d) = %q, want %q", tt.level, got, tt.want)
		}
	}
}

// TDD Cycle 13: UnarmoredMovementBonus returns 0 for levels below 2

func TestUnarmoredMovementBonus_Level1_ReturnsZero(t *testing.T) {
	got := UnarmoredMovementBonus(1)
	if got != 0 {
		t.Errorf("UnarmoredMovementBonus(1) = %d, want 0 (starts at level 2)", got)
	}
}

func TestUnarmoredMovementBonus_Level0_ReturnsZero(t *testing.T) {
	got := UnarmoredMovementBonus(0)
	if got != 0 {
		t.Errorf("UnarmoredMovementBonus(0) = %d, want 0", got)
	}
}

// TDD Cycle 14: MartialArtsBonusAttack service method integration tests

func makeMonkCharacter(str, dex int, monkLevel int) refdata.Character {
	scores := AbilityScores{Str: str, Dex: dex, Con: 10, Int: 10, Wis: 16, Cha: 10}
	scoresJSON, _ := json.Marshal(scores)
	classes := []CharacterClass{{Class: "Monk", Level: monkLevel}}
	classesJSON, _ := json.Marshal(classes)
	return refdata.Character{
		ID:               uuid.New(),
		AbilityScores:    scoresJSON,
		ProficiencyBonus: 2,
		Classes:          classesJSON,
	}
}

func TestServiceMartialArtsBonusAttack_HappyPath(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeMonkCharacter(10, 16, 5) // DEX +3, Monk 5
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15 // attack roll
		}
		return 4 // damage die
	})

	result, err := svc.MartialArtsBonusAttack(ctx, MartialArtsBonusAttackCommand{
		Attacker: refdata.Combatant{
			ID:          attackerID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Kira",
			PositionCol: "A", PositionRow: 1,
			IsAlive: true, Conditions: json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{
			ID:          targetID,
			DisplayName: "Goblin #1",
			PositionCol: "B", PositionRow: 1,
			Ac: 13, IsAlive: true, IsNpc: true,
			Conditions: json.RawMessage(`[]`),
		},
		Turn: refdata.Turn{
			ID:               turnID,
			CombatantID:      attackerID,
			AttacksRemaining: 1,
			ActionUsed:       true, // Attack action was used
		},
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	// Monk level 5: martial arts die 1d6, DEX +3
	// Damage = d6(4) + 3 = 7
	assert.Equal(t, 7, result.DamageTotal)
	assert.Equal(t, "bludgeoning", result.DamageType)
}

func TestServiceMartialArtsBonusAttack_NotACharacter(t *testing.T) {
	ctx := context.Background()
	svc := NewService(defaultMockStore())
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.MartialArtsBonusAttack(ctx, MartialArtsBonusAttackCommand{
		Attacker: refdata.Combatant{
			ID:    uuid.New(),
			IsNpc: true,
			// No CharacterID — NPC
			Conditions: json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:   refdata.Turn{ID: uuid.New(), ActionUsed: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a character")
}

func TestServiceMartialArtsBonusAttack_NotAMonk(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	// Fighter, not a monk
	scores := AbilityScores{Str: 16, Dex: 10, Con: 10, Int: 10, Wis: 10, Cha: 10}
	scoresJSON, _ := json.Marshal(scores)
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Fighter", Level: 5}})

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			AbilityScores:    scoresJSON,
			ProficiencyBonus: 2,
			Classes:          classesJSON,
		}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.MartialArtsBonusAttack(ctx, MartialArtsBonusAttackCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:   refdata.Turn{ID: uuid.New(), ActionUsed: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires Monk class")
}

func TestServiceMartialArtsBonusAttack_NoAttackAction(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeMonkCharacter(10, 16, 5)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.MartialArtsBonusAttack(ctx, MartialArtsBonusAttackCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn: refdata.Turn{
			ID:         uuid.New(),
			ActionUsed: false, // No attack action used
		},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires the Attack action")
}

func TestServiceMartialArtsBonusAttack_BonusActionAlreadyUsed(t *testing.T) {
	ctx := context.Background()
	svc := NewService(defaultMockStore())
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.MartialArtsBonusAttack(ctx, MartialArtsBonusAttackCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:   refdata.Turn{ID: uuid.New(), BonusActionUsed: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bonus action")
}

func TestServiceMartialArtsBonusAttack_Miss(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeMonkCharacter(10, 16, 5)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 1 // nat 1 miss
		}
		return 3
	})

	result, err := svc.MartialArtsBonusAttack(ctx, MartialArtsBonusAttackCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Kira",
			PositionCol: "A", PositionRow: 1,
			IsAlive: true, Conditions: json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{
			ID: uuid.New(), DisplayName: "Goblin",
			PositionCol: "B", PositionRow: 1,
			Ac: 20, IsAlive: true, IsNpc: true,
			Conditions: json.RawMessage(`[]`),
		},
		Turn: refdata.Turn{
			ID:               uuid.New(),
			AttacksRemaining: 1,
			ActionUsed:       true,
		},
	}, roller)
	require.NoError(t, err)
	assert.False(t, result.Hit)
	assert.Equal(t, 0, result.DamageTotal)
}

func TestServiceMartialArtsBonusAttack_GetCharacterError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.MartialArtsBonusAttack(ctx, MartialArtsBonusAttackCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:   refdata.Turn{ID: uuid.New(), ActionUsed: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

func TestServiceMartialArtsBonusAttack_BadAbilityScores(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Monk", Level: 5}})

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			AbilityScores:    []byte(`bad-json`),
			ProficiencyBonus: 2,
			Classes:          classesJSON,
		}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.MartialArtsBonusAttack(ctx, MartialArtsBonusAttackCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:   refdata.Turn{ID: uuid.New(), ActionUsed: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing ability scores")
}

func TestServiceMartialArtsBonusAttack_UpdateTurnError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeMonkCharacter(10, 16, 5)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 3
	})

	_, err := svc.MartialArtsBonusAttack(ctx, MartialArtsBonusAttackCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Kira",
			PositionCol: "A", PositionRow: 1,
			IsAlive: true, Conditions: json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{
			ID: uuid.New(), DisplayName: "Goblin",
			PositionCol: "B", PositionRow: 1,
			Ac: 13, IsAlive: true, IsNpc: true,
			Conditions: json.RawMessage(`[]`),
		},
		Turn: refdata.Turn{
			ID:               uuid.New(),
			AttacksRemaining: 1,
			ActionUsed:       true,
		},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// --- Phase 48b: Ki Abilities ---

// TDD Cycle 1: parseKiUses extracts ki points from feature_uses

func TestParseKiUses_WithKiPoints(t *testing.T) {
	featureUsesJSON := json.RawMessage(`{"ki":{"current":3,"max":3,"recharge":"long"}}`)
	char := refdata.Character{
		FeatureUses: pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
	}
	uses, remaining, err := ParseFeatureUses(char, FeatureKeyKi)
	require.NoError(t, err)
	assert.Equal(t, 3, remaining)
	assert.Equal(t, 3, uses["ki"].Current)
}

func TestParseKiUses_Empty(t *testing.T) {
	char := refdata.Character{}
	uses, remaining, err := ParseFeatureUses(char, FeatureKeyKi)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining)
	assert.NotNil(t, uses)
}

func TestParseKiUses_BadJSON(t *testing.T) {
	char := refdata.Character{
		FeatureUses: pqtype.NullRawMessage{RawMessage: json.RawMessage(`bad`), Valid: true},
	}
	_, _, err := ParseFeatureUses(char, FeatureKeyKi)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing feature_uses")
}

// TDD Cycle 2: ValidateKiAbility

func TestValidateKiAbility_Valid(t *testing.T) {
	err := ValidateKiAbility(5, 3, "flurry-of-blows")
	assert.NoError(t, err)
}

func TestValidateKiAbility_NotMonk(t *testing.T) {
	err := ValidateKiAbility(0, 3, "flurry-of-blows")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires Monk class")
}

func TestValidateKiAbility_NoKiPoints(t *testing.T) {
	err := ValidateKiAbility(5, 0, "flurry-of-blows")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no ki points remaining")
}

// TDD Cycle 3: FlurryOfBlows service method

func makeMonkCharacterWithKi(str, dex, wis int, monkLevel int, kiPoints int) refdata.Character {
	scores := AbilityScores{Str: str, Dex: dex, Con: 10, Int: 10, Wis: wis, Cha: 10}
	scoresJSON, _ := json.Marshal(scores)
	classes := []CharacterClass{{Class: "Monk", Level: monkLevel}}
	classesJSON, _ := json.Marshal(classes)
	featureUses := map[string]character.FeatureUse{"ki": {Current: kiPoints, Max: kiPoints, Recharge: "long"}}
	featureUsesJSON, _ := json.Marshal(featureUses)
	return refdata.Character{
		ID:               uuid.New(),
		AbilityScores:    scoresJSON,
		ProficiencyBonus: 2,
		Classes:          classesJSON,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
	}
}

func TestServiceFlurryOfBlows_HappyPath(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}
	var updatedFeatureUses pqtype.NullRawMessage
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		updatedFeatureUses = arg.FeatureUses
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 3
	})

	result, err := svc.FlurryOfBlows(ctx, FlurryOfBlowsCommand{
		Attacker: refdata.Combatant{
			ID:          attackerID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Kira",
			PositionCol: "A", PositionRow: 1,
			IsAlive: true, Conditions: json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{
			ID:          targetID,
			DisplayName: "Goblin #1",
			PositionCol: "B", PositionRow: 1,
			Ac: 13, IsAlive: true, IsNpc: true,
			Conditions: json.RawMessage(`[]`),
		},
		Turn: refdata.Turn{
			ID:               turnID,
			CombatantID:      attackerID,
			AttacksRemaining: 1,
			ActionUsed:       true,
		},
	}, roller)
	require.NoError(t, err)
	assert.Len(t, result.Attacks, 2, "flurry of blows should make 2 attacks")
	assert.Equal(t, 4, result.KiRemaining, "should deduct 1 ki point")
	// Verify ki was persisted
	var uses map[string]character.FeatureUse
	require.NoError(t, json.Unmarshal(updatedFeatureUses.RawMessage, &uses))
	assert.Equal(t, 4, uses["ki"].Current)
}

func TestServiceFlurryOfBlows_BonusActionSpent(t *testing.T) {
	ctx := context.Background()
	svc := NewService(defaultMockStore())
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.FlurryOfBlows(ctx, FlurryOfBlowsCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:   refdata.Turn{ID: uuid.New(), BonusActionUsed: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bonus action")
}

func TestServiceFlurryOfBlows_NotACharacter(t *testing.T) {
	ctx := context.Background()
	svc := NewService(defaultMockStore())
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.FlurryOfBlows(ctx, FlurryOfBlowsCommand{
		Attacker: refdata.Combatant{ID: uuid.New(), IsNpc: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: uuid.New(), ActionUsed: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a character")
}

func TestServiceFlurryOfBlows_NoKiRemaining(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeMonkCharacterWithKi(10, 16, 16, 5, 0) // 0 ki
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.FlurryOfBlows(ctx, FlurryOfBlowsCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:   refdata.Turn{ID: uuid.New(), ActionUsed: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no ki points remaining")
}

func TestServiceFlurryOfBlows_NoAttackAction(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.FlurryOfBlows(ctx, FlurryOfBlowsCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:   refdata.Turn{ID: uuid.New(), ActionUsed: false}, // no attack action
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires the Attack action")
}

// TDD Cycle 4: PatientDefense

func TestServicePatientDefense_HappyPath(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()

	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}
	var appliedConditions json.RawMessage
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, DisplayName: "Kira", Conditions: json.RawMessage(`[]`)}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		appliedConditions = arg.Conditions
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)

	result, err := svc.PatientDefense(ctx, KiAbilityCommand{
		Combatant: refdata.Combatant{
			ID:          attackerID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Kira",
			Conditions:  json.RawMessage(`[]`),
		},
		Turn: refdata.Turn{ID: uuid.New(), CombatantID: attackerID},
	})
	require.NoError(t, err)
	assert.Equal(t, 4, result.KiRemaining)
	assert.Contains(t, result.CombatLog, "Patient Defense")

	// Verify dodge condition was applied
	assert.True(t, HasCondition(appliedConditions, "dodge"))
}

func TestServicePatientDefense_NoKi(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeMonkCharacterWithKi(10, 16, 16, 5, 0)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)
	_, err := svc.PatientDefense(ctx, KiAbilityCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no ki points remaining")
}

// TDD Cycle 5: StepOfTheWind

func TestServiceStepOfTheWind_Dash(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()

	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{
			ID:                  arg.ID,
			BonusActionUsed:     arg.BonusActionUsed,
			MovementRemainingFt: arg.MovementRemainingFt,
		}, nil
	}

	svc := NewService(ms)

	result, err := svc.StepOfTheWind(ctx, StepOfTheWindCommand{
		KiAbilityCommand: KiAbilityCommand{
			Combatant: refdata.Combatant{
				ID:          attackerID,
				CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
				DisplayName: "Kira",
				Conditions:  json.RawMessage(`[]`),
			},
			Turn: refdata.Turn{ID: uuid.New(), CombatantID: attackerID, MovementRemainingFt: 30},
		},
		Mode: "dash",
	})
	require.NoError(t, err)
	assert.Equal(t, 4, result.KiRemaining)
	assert.Contains(t, result.CombatLog, "Step of the Wind")
	assert.Contains(t, result.CombatLog, "dash")
	// Dash doubles remaining movement
	assert.Equal(t, int32(60), result.Turn.MovementRemainingFt)
}

func TestServiceStepOfTheWind_Disengage(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()

	char := makeMonkCharacterWithKi(10, 16, 16, 5, 3)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{
			ID:              arg.ID,
			BonusActionUsed: arg.BonusActionUsed,
			HasDisengaged:   arg.HasDisengaged,
		}, nil
	}

	svc := NewService(ms)

	result, err := svc.StepOfTheWind(ctx, StepOfTheWindCommand{
		KiAbilityCommand: KiAbilityCommand{
			Combatant: refdata.Combatant{
				ID:          attackerID,
				CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
				DisplayName: "Kira",
				Conditions:  json.RawMessage(`[]`),
			},
			Turn: refdata.Turn{ID: uuid.New(), CombatantID: attackerID, MovementRemainingFt: 30},
		},
		Mode: "disengage",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.KiRemaining)
	assert.Contains(t, result.CombatLog, "disengage")
	assert.True(t, result.Turn.HasDisengaged)
}

func TestServiceStepOfTheWind_InvalidMode(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeMonkCharacterWithKi(10, 16, 16, 5, 3)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}

	svc := NewService(ms)

	_, err := svc.StepOfTheWind(ctx, StepOfTheWindCommand{
		KiAbilityCommand: KiAbilityCommand{
			Combatant: refdata.Combatant{
				ID:          uuid.New(),
				CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
				DisplayName: "Kira",
				Conditions:  json.RawMessage(`[]`),
			},
			Turn: refdata.Turn{ID: uuid.New(), MovementRemainingFt: 30},
		},
		Mode: "attack",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid mode")
}

// TDD Cycle 6: StunningStrike

func TestStunningStrikeDC(t *testing.T) {
	// DC = 8 + proficiency + WIS mod
	// WIS 16 = +3, prof 2 -> DC = 13
	dc := StunningStrikeDC(2, 16)
	assert.Equal(t, 13, dc)
}

func TestStunningStrikeDC_HighLevel(t *testing.T) {
	// WIS 20 = +5, prof 6 -> DC = 19
	dc := StunningStrikeDC(6, 20)
	assert.Equal(t, 19, dc)
}

func TestServiceStunningStrike_TargetFailsSave(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()

	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5) // WIS 16 (+3), prof 2 -> DC 13
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == targetID {
			return refdata.Combatant{
				ID:          targetID,
				DisplayName: "Goblin",
				Conditions:  json.RawMessage(`[]`),
			}, nil
		}
		return refdata.Combatant{ID: id, Conditions: json.RawMessage(`[]`)}, nil
	}
	var appliedConditions json.RawMessage
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		appliedConditions = arg.Conditions
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)
	// Roller returns 5 on d20 -> save = 5 + 0 (CON 10) = 5, fails DC 13
	roller := dice.NewRoller(func(max int) int { return 5 })

	result, err := svc.StunningStrike(ctx, StunningStrikeCommand{
		Attacker: refdata.Combatant{
			ID:          attackerID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Kira",
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{
			ID:          targetID,
			DisplayName: "Goblin",
			Conditions:  json.RawMessage(`[]`),
		},
		CurrentRound: 3,
	}, roller)
	require.NoError(t, err)
	assert.False(t, result.SaveSucceeded)
	assert.True(t, result.Stunned)
	assert.Equal(t, 4, result.KiRemaining)
	assert.Contains(t, result.CombatLog, "stunned")

	// Verify stunned was applied
	assert.True(t, HasCondition(appliedConditions, "stunned"))
}

func TestServiceStunningStrike_TargetPassesSave(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()

	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5) // DC 13
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`)}, nil
	}

	svc := NewService(ms)
	// Roller returns 18 on d20 -> save = 18 + 0 = 18, passes DC 13
	roller := dice.NewRoller(func(max int) int { return 18 })

	result, err := svc.StunningStrike(ctx, StunningStrikeCommand{
		Attacker: refdata.Combatant{
			ID:          attackerID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Kira",
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{
			ID:          targetID,
			DisplayName: "Goblin",
			Conditions:  json.RawMessage(`[]`),
		},
		CurrentRound: 3,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.SaveSucceeded)
	assert.False(t, result.Stunned)
	assert.Equal(t, 4, result.KiRemaining)
	assert.Contains(t, result.CombatLog, "resists")
}

func TestServiceStunningStrike_NoKi(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeMonkCharacterWithKi(10, 16, 16, 5, 0)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.StunningStrike(ctx, StunningStrikeCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{
			ID:         uuid.New(),
			Conditions: json.RawMessage(`[]`),
		},
		CurrentRound: 1,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no ki points remaining")
}

func TestServiceStunningStrike_NotMonk(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	// Fighter, not monk
	scores := AbilityScores{Str: 16, Dex: 10, Con: 10, Int: 10, Wis: 10, Cha: 10}
	scoresJSON, _ := json.Marshal(scores)
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Fighter", Level: 5}})
	featureUsesJSON, _ := json.Marshal(map[string]character.FeatureUse{"ki": {Current: 5, Max: 5, Recharge: "long"}})

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			AbilityScores:    scoresJSON,
			ProficiencyBonus: 2,
			Classes:          classesJSON,
			FeatureUses:      pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
		}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.StunningStrike(ctx, StunningStrikeCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{
			ID:         uuid.New(),
			Conditions: json.RawMessage(`[]`),
		},
		CurrentRound: 1,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires Monk class")
}

// TDD Cycle 7: StunningStrike with creature save bonus

func TestServiceStunningStrike_CreatureWithConSaveBonus(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()

	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5) // DC 13
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:            targetID,
			DisplayName:   "Ogre",
			CreatureRefID: sql.NullString{String: "ogre", Valid: true},
			Conditions:    json.RawMessage(`[]`),
		}, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		savesJSON, _ := json.Marshal(map[string]int{"con": 5})
		return refdata.Creature{
			ID:            "ogre",
			AbilityScores: json.RawMessage(`{"str":19,"dex":8,"con":16,"int":5,"wis":7,"cha":7}`),
			SavingThrows:  pqtype.NullRawMessage{RawMessage: savesJSON, Valid: true},
		}, nil
	}

	svc := NewService(ms)
	// Roller returns 7 -> save = 7 + 5 (con save bonus) = 12, fails DC 13
	roller := dice.NewRoller(func(max int) int { return 7 })

	result, err := svc.StunningStrike(ctx, StunningStrikeCommand{
		Attacker: refdata.Combatant{
			ID:          attackerID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Kira",
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{
			ID:            targetID,
			DisplayName:   "Ogre",
			CreatureRefID: sql.NullString{String: "ogre", Valid: true},
			Conditions:    json.RawMessage(`[]`),
		},
		CurrentRound: 3,
	}, roller)
	require.NoError(t, err)
	assert.False(t, result.SaveSucceeded)
	assert.True(t, result.Stunned)
}

// TDD Cycle 8: Edge cases

func TestServiceFlurryOfBlows_GetCharacterError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.FlurryOfBlows(ctx, FlurryOfBlowsCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:   refdata.Turn{ID: uuid.New(), ActionUsed: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

func TestServiceFlurryOfBlows_BadAbilityScores(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Monk", Level: 5}})
	featureUsesJSON, _ := json.Marshal(map[string]character.FeatureUse{"ki": {Current: 5, Max: 5, Recharge: "long"}})

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			AbilityScores:    []byte(`bad-json`),
			ProficiencyBonus: 2,
			Classes:          classesJSON,
			FeatureUses:      pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
		}, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.FlurryOfBlows(ctx, FlurryOfBlowsCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:   refdata.Turn{ID: uuid.New(), ActionUsed: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing ability scores")
}

func TestServiceFlurryOfBlows_UpdateFeatureUsesError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.FlurryOfBlows(ctx, FlurryOfBlowsCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:   refdata.Turn{ID: uuid.New(), ActionUsed: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating feature_uses")
}

func TestServicePatientDefense_BonusActionSpent(t *testing.T) {
	ctx := context.Background()
	svc := NewService(defaultMockStore())

	_, err := svc.PatientDefense(ctx, KiAbilityCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Turn: refdata.Turn{ID: uuid.New(), BonusActionUsed: true},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bonus action")
}

func TestServicePatientDefense_NotACharacter(t *testing.T) {
	ctx := context.Background()
	svc := NewService(defaultMockStore())

	_, err := svc.PatientDefense(ctx, KiAbilityCommand{
		Combatant: refdata.Combatant{
			ID:         uuid.New(),
			IsNpc:      true,
			Conditions: json.RawMessage(`[]`),
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a character")
}

func TestServiceStepOfTheWind_BonusActionSpent(t *testing.T) {
	ctx := context.Background()
	svc := NewService(defaultMockStore())

	_, err := svc.StepOfTheWind(ctx, StepOfTheWindCommand{
		KiAbilityCommand: KiAbilityCommand{
			Combatant: refdata.Combatant{
				ID:          uuid.New(),
				CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
				Conditions:  json.RawMessage(`[]`),
			},
			Turn: refdata.Turn{ID: uuid.New(), BonusActionUsed: true},
		},
		Mode: "dash",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bonus action")
}

func TestServiceStepOfTheWind_NotACharacter(t *testing.T) {
	ctx := context.Background()
	svc := NewService(defaultMockStore())

	_, err := svc.StepOfTheWind(ctx, StepOfTheWindCommand{
		KiAbilityCommand: KiAbilityCommand{
			Combatant: refdata.Combatant{
				ID:         uuid.New(),
				IsNpc:      true,
				Conditions: json.RawMessage(`[]`),
			},
			Turn: refdata.Turn{ID: uuid.New()},
		},
		Mode: "dash",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a character")
}

func TestServiceStepOfTheWind_NoKi(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeMonkCharacterWithKi(10, 16, 16, 5, 0)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)

	_, err := svc.StepOfTheWind(ctx, StepOfTheWindCommand{
		KiAbilityCommand: KiAbilityCommand{
			Combatant: refdata.Combatant{
				ID:          uuid.New(),
				CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
				Conditions:  json.RawMessage(`[]`),
			},
			Turn: refdata.Turn{ID: uuid.New()},
		},
		Mode: "dash",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no ki points remaining")
}

func TestServiceStunningStrike_NotACharacter(t *testing.T) {
	ctx := context.Background()
	svc := NewService(defaultMockStore())
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.StunningStrike(ctx, StunningStrikeCommand{
		Attacker: refdata.Combatant{
			ID:         uuid.New(),
			IsNpc:      true,
			Conditions: json.RawMessage(`[]`),
		},
		Target:       refdata.Combatant{ID: uuid.New(), Conditions: json.RawMessage(`[]`)},
		CurrentRound: 1,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a character")
}

func TestServiceStunningStrike_GetCharacterError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.StunningStrike(ctx, StunningStrikeCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target:       refdata.Combatant{ID: uuid.New(), Conditions: json.RawMessage(`[]`)},
		CurrentRound: 1,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

func TestServiceStunningStrike_BadAbilityScores(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Monk", Level: 5}})
	featureUsesJSON, _ := json.Marshal(map[string]character.FeatureUse{"ki": {Current: 5, Max: 5, Recharge: "long"}})

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			AbilityScores:    []byte(`bad-json`),
			ProficiencyBonus: 2,
			Classes:          classesJSON,
			FeatureUses:      pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
		}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.StunningStrike(ctx, StunningStrikeCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target:       refdata.Combatant{ID: uuid.New(), Conditions: json.RawMessage(`[]`)},
		CurrentRound: 1,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing ability scores")
}

func TestServiceStunningStrike_UpdateFeatureUsesError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.StunningStrike(ctx, StunningStrikeCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target:       refdata.Combatant{ID: uuid.New(), Conditions: json.RawMessage(`[]`)},
		CurrentRound: 1,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating feature_uses")
}

// Test resolveTargetConSave with PC target
func TestServiceStunningStrike_PCTarget(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetCharID := uuid.New()
	targetID := uuid.New()

	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5) // DC 13
	char.ID = charID

	// Target PC with CON 14 (+2)
	targetScores := AbilityScores{Str: 10, Dex: 10, Con: 14, Int: 10, Wis: 10, Cha: 10}
	targetScoresJSON, _ := json.Marshal(targetScores)

	ms := defaultMockStore()
	callNum := 0
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		callNum++
		if id == charID {
			return char, nil
		}
		return refdata.Character{
			ID:            targetCharID,
			AbilityScores: targetScoresJSON,
		}, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          targetID,
			CharacterID: uuid.NullUUID{UUID: targetCharID, Valid: true},
			DisplayName: "Ally",
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}

	svc := NewService(ms)
	// Roll 10 -> save = 10 + 2 = 12, fails DC 13
	roller := dice.NewRoller(func(max int) int { return 10 })

	result, err := svc.StunningStrike(ctx, StunningStrikeCommand{
		Attacker: refdata.Combatant{
			ID:          attackerID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Kira",
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{
			ID:          targetID,
			CharacterID: uuid.NullUUID{UUID: targetCharID, Valid: true},
			DisplayName: "Ally",
			Conditions:  json.RawMessage(`[]`),
		},
		CurrentRound: 3,
	}, roller)
	require.NoError(t, err)
	assert.False(t, result.SaveSucceeded)
	assert.True(t, result.Stunned)
}

// Test resolveTargetConSave with NPC target (no creature ref)
func TestServiceStunningStrike_NPCNoCreatureRef(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()

	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5) // DC 13
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          targetID,
			DisplayName: "Bandit",
			IsNpc:       true,
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}

	svc := NewService(ms)
	// Roll 12 -> save = 12 + 0 = 12, fails DC 13
	roller := dice.NewRoller(func(max int) int { return 12 })

	result, err := svc.StunningStrike(ctx, StunningStrikeCommand{
		Attacker: refdata.Combatant{
			ID:          attackerID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Kira",
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{
			ID:          targetID,
			DisplayName: "Bandit",
			IsNpc:       true,
			Conditions:  json.RawMessage(`[]`),
		},
		CurrentRound: 2,
	}, roller)
	require.NoError(t, err)
	assert.False(t, result.SaveSucceeded)
	assert.True(t, result.Stunned)
}

// Test FlurryOfBlows update turn error
func TestServiceFlurryOfBlows_UpdateTurnError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.FlurryOfBlows(ctx, FlurryOfBlowsCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Kira",
			PositionCol: "A", PositionRow: 1,
			Conditions: json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{
			ID: uuid.New(), DisplayName: "Goblin",
			PositionCol: "B", PositionRow: 1,
			Ac: 13, Conditions: json.RawMessage(`[]`),
		},
		Turn: refdata.Turn{ID: uuid.New(), ActionUsed: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// Test that FlurryOfBlows replaces martial arts bonus (uses bonus action)
func TestServiceFlurryOfBlows_ReplacesMaritalArtBonus(t *testing.T) {
	// FlurryOfBlows and MartialArtsBonusAttack both use bonus action.
	// If bonus action already used, FlurryOfBlows should fail.
	ctx := context.Background()
	svc := NewService(defaultMockStore())
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.FlurryOfBlows(ctx, FlurryOfBlowsCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:   refdata.Turn{ID: uuid.New(), BonusActionUsed: true, ActionUsed: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bonus action")
}

// Test FlurryOfBlows with not a monk
func TestServiceFlurryOfBlows_NotAMonk(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	scores := AbilityScores{Str: 16, Dex: 10, Con: 10, Int: 10, Wis: 10, Cha: 10}
	scoresJSON, _ := json.Marshal(scores)
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Fighter", Level: 5}})
	featureUsesJSON, _ := json.Marshal(map[string]character.FeatureUse{"ki": {Current: 5, Max: 5, Recharge: "long"}})

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			AbilityScores:    scoresJSON,
			ProficiencyBonus: 2,
			Classes:          classesJSON,
			FeatureUses:      pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
		}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.FlurryOfBlows(ctx, FlurryOfBlowsCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:   refdata.Turn{ID: uuid.New(), ActionUsed: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires Monk class")
}

// TDD Cycle 9: Coverage gaps for spendKi and resolveTargetConSave

func TestServicePatientDefense_GetCharacterError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)

	_, err := svc.PatientDefense(ctx, KiAbilityCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

func TestServicePatientDefense_UpdateFeatureUsesError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)

	_, err := svc.PatientDefense(ctx, KiAbilityCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating feature_uses")
}

func TestServicePatientDefense_UpdateTurnError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)

	_, err := svc.PatientDefense(ctx, KiAbilityCommand{
		Combatant: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Turn: refdata.Turn{ID: uuid.New()},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

// Test resolveTargetConSave with creature that has no saving_throws field (uses ability score)
func TestServiceStunningStrike_CreatureNoSaveBonus(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()

	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5) // DC 13
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:            targetID,
			DisplayName:   "Wolf",
			CreatureRefID: sql.NullString{String: "wolf", Valid: true},
			Conditions:    json.RawMessage(`[]`),
		}, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "wolf",
			AbilityScores: json.RawMessage(`{"str":12,"dex":15,"con":12,"int":3,"wis":12,"cha":6}`),
			// No SavingThrows set -> uses CON mod (+1)
		}, nil
	}

	svc := NewService(ms)
	// Roll 11 -> save = 11 + 1 (CON 12 mod) = 12, fails DC 13
	roller := dice.NewRoller(func(max int) int { return 11 })

	result, err := svc.StunningStrike(ctx, StunningStrikeCommand{
		Attacker: refdata.Combatant{
			ID:          attackerID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Kira",
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{
			ID:            targetID,
			DisplayName:   "Wolf",
			CreatureRefID: sql.NullString{String: "wolf", Valid: true},
			Conditions:    json.RawMessage(`[]`),
		},
		CurrentRound: 2,
	}, roller)
	require.NoError(t, err)
	assert.False(t, result.SaveSucceeded)
	assert.Equal(t, 12, result.SaveTotal) // 11 + 1
}

// Test resolveTargetConSave error cases
func TestServiceStunningStrike_GetCreatureError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{}, fmt.Errorf("not found")
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.StunningStrike(ctx, StunningStrikeCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Kira",
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{
			ID:            uuid.New(),
			DisplayName:   "Unknown",
			CreatureRefID: sql.NullString{String: "unknown", Valid: true},
			Conditions:    json.RawMessage(`[]`),
		},
		CurrentRound: 1,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting creature for save")
}

// Test FlurryOfBlows bad feature_uses JSON
func TestServiceFlurryOfBlows_BadFeatureUses(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Monk", Level: 5}})
	scoresJSON, _ := json.Marshal(AbilityScores{Str: 10, Dex: 16, Con: 10, Int: 10, Wis: 16, Cha: 10})

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:               charID,
			AbilityScores:    scoresJSON,
			ProficiencyBonus: 2,
			Classes:          classesJSON,
			FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`bad`), Valid: true},
		}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.FlurryOfBlows(ctx, FlurryOfBlowsCommand{
		Attacker: refdata.Combatant{
			ID:          uuid.New(),
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		},
		Target: refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:   refdata.Turn{ID: uuid.New(), ActionUsed: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing feature_uses")
}

// Test StepOfTheWind with update turn error
func TestServiceStepOfTheWind_UpdateTurnError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5)
	char.ID = charID

	callCount := 0
	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		callCount++
		if callCount == 1 {
			// First call in spendKi succeeds
			return refdata.Turn{ID: arg.ID, BonusActionUsed: true, MovementRemainingFt: 30}, nil
		}
		// Second call for dash/disengage update fails
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)

	_, err := svc.StepOfTheWind(ctx, StepOfTheWindCommand{
		KiAbilityCommand: KiAbilityCommand{
			Combatant: refdata.Combatant{
				ID:          uuid.New(),
				CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
				DisplayName: "Kira",
				Conditions:  json.RawMessage(`[]`),
			},
			Turn: refdata.Turn{ID: uuid.New(), MovementRemainingFt: 30},
		},
		Mode: "dash",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

func TestServiceStepOfTheWind_Dash_AddsBaseSpeedNotRemaining(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()

	char := makeMonkCharacterWithKi(10, 16, 16, 5, 5)
	char.ID = charID
	// Base speed defaults to 30 (SpeedFt unset => resolveBaseSpeed returns 30)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{
			ID:                  arg.ID,
			BonusActionUsed:     arg.BonusActionUsed,
			MovementRemainingFt: arg.MovementRemainingFt,
		}, nil
	}

	svc := NewService(ms)

	// Monk has 30ft base speed, already moved 20ft, so 10ft remaining.
	// Step of the Wind dash should add base speed (30), not remaining (10).
	// Expected: 10 + 30 = 40, NOT 10 + 10 = 20.
	result, err := svc.StepOfTheWind(ctx, StepOfTheWindCommand{
		KiAbilityCommand: KiAbilityCommand{
			Combatant: refdata.Combatant{
				ID:          attackerID,
				CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
				DisplayName: "Kira",
				Conditions:  json.RawMessage(`[]`),
			},
			Turn: refdata.Turn{ID: uuid.New(), CombatantID: attackerID, MovementRemainingFt: 10},
		},
		Mode: "dash",
	})
	require.NoError(t, err)
	assert.Equal(t, int32(40), result.Turn.MovementRemainingFt, "dash should add base speed (30), not remaining movement (10)")
}

// Suppress unused imports
var _ = sql.ErrNoRows
