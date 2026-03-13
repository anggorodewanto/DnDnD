package combat

import (
	"testing"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
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
	got := monkLevelFromJSON(classesJSON)
	if got != 8 {
		t.Errorf("monkLevelFromJSON = %d, want 8", got)
	}
}

func TestMonkLevelFromJSON_NotMonk(t *testing.T) {
	classesJSON := []byte(`[{"class":"Fighter","level":5}]`)
	got := monkLevelFromJSON(classesJSON)
	if got != 0 {
		t.Errorf("monkLevelFromJSON (fighter) = %d, want 0", got)
	}
}

func TestMonkLevelFromJSON_Empty(t *testing.T) {
	got := monkLevelFromJSON(nil)
	if got != 0 {
		t.Errorf("monkLevelFromJSON (nil) = %d, want 0", got)
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

// Edge: monkLevelFromJSON with bad JSON
func TestMonkLevelFromJSON_BadJSON(t *testing.T) {
	got := monkLevelFromJSON([]byte("bad json"))
	if got != 0 {
		t.Errorf("monkLevelFromJSON(bad) = %d, want 0", got)
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
