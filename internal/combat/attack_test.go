package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

func TestHasProperty(t *testing.T) {
	weapon := refdata.Weapon{Properties: []string{"finesse", "light"}}

	if !HasProperty(weapon, "finesse") {
		t.Error("expected weapon to have finesse property")
	}
	if !HasProperty(weapon, "light") {
		t.Error("expected weapon to have light property")
	}
	if HasProperty(weapon, "heavy") {
		t.Error("expected weapon NOT to have heavy property")
	}
}

func TestIsRangedWeapon(t *testing.T) {
	tests := []struct {
		name     string
		weapon   refdata.Weapon
		expected bool
	}{
		{
			name:     "simple_ranged",
			weapon:   refdata.Weapon{WeaponType: "simple_ranged"},
			expected: true,
		},
		{
			name:     "martial_ranged",
			weapon:   refdata.Weapon{WeaponType: "martial_ranged"},
			expected: true,
		},
		{
			name:     "simple_melee",
			weapon:   refdata.Weapon{WeaponType: "simple_melee"},
			expected: false,
		},
		{
			name:     "martial_melee",
			weapon:   refdata.Weapon{WeaponType: "martial_melee"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRangedWeapon(tt.weapon)
			if got != tt.expected {
				t.Errorf("IsRangedWeapon(%s) = %v, want %v", tt.weapon.WeaponType, got, tt.expected)
			}
		})
	}
}

func TestAttackModifier(t *testing.T) {
	tests := []struct {
		name      string
		scores    AbilityScores
		weapon    refdata.Weapon
		profBonus int
		expected  int
	}{
		{
			name:      "melee weapon uses STR",
			scores:    AbilityScores{Str: 16, Dex: 14},
			weapon:    refdata.Weapon{WeaponType: "simple_melee"},
			profBonus: 2,
			expected:  5, // STR mod 3 + prof 2
		},
		{
			name:      "ranged weapon uses DEX",
			scores:    AbilityScores{Str: 10, Dex: 18},
			weapon:    refdata.Weapon{WeaponType: "simple_ranged"},
			profBonus: 2,
			expected:  6, // DEX mod 4 + prof 2
		},
		{
			name:      "finesse weapon uses higher of STR/DEX (DEX higher)",
			scores:    AbilityScores{Str: 10, Dex: 18},
			weapon:    refdata.Weapon{WeaponType: "simple_melee", Properties: []string{"finesse"}},
			profBonus: 2,
			expected:  6, // DEX mod 4 + prof 2
		},
		{
			name:      "finesse weapon uses higher of STR/DEX (STR higher)",
			scores:    AbilityScores{Str: 20, Dex: 14},
			weapon:    refdata.Weapon{WeaponType: "simple_melee", Properties: []string{"finesse"}},
			profBonus: 3,
			expected:  8, // STR mod 5 + prof 3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AttackModifier(tt.scores, tt.weapon, tt.profBonus)
			if got != tt.expected {
				t.Errorf("AttackModifier() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestDamageModifier(t *testing.T) {
	tests := []struct {
		name     string
		scores   AbilityScores
		weapon   refdata.Weapon
		expected int
	}{
		{
			name:     "melee uses STR",
			scores:   AbilityScores{Str: 16, Dex: 14},
			weapon:   refdata.Weapon{WeaponType: "martial_melee"},
			expected: 3,
		},
		{
			name:     "ranged uses DEX",
			scores:   AbilityScores{Str: 10, Dex: 18},
			weapon:   refdata.Weapon{WeaponType: "simple_ranged"},
			expected: 4,
		},
		{
			name:     "finesse uses higher (DEX)",
			scores:   AbilityScores{Str: 10, Dex: 18},
			weapon:   refdata.Weapon{WeaponType: "simple_melee", Properties: []string{"finesse"}},
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DamageModifier(tt.scores, tt.weapon)
			if got != tt.expected {
				t.Errorf("DamageModifier() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestUnarmedStrike(t *testing.T) {
	w := UnarmedStrike()
	if w.ID != "unarmed-strike" {
		t.Errorf("expected id unarmed-strike, got %s", w.ID)
	}
	if w.Name != "Unarmed Strike" {
		t.Errorf("expected name Unarmed Strike, got %s", w.Name)
	}
	if w.Damage != "0" {
		t.Errorf("expected damage 0, got %s", w.Damage)
	}
	if w.WeaponType != "simple_melee" {
		t.Errorf("expected weapon type simple_melee, got %s", w.WeaponType)
	}
}

func TestWeaponReach(t *testing.T) {
	tests := []struct {
		name     string
		weapon   refdata.Weapon
		expected int
	}{
		{
			name:     "melee weapon default 5ft",
			weapon:   refdata.Weapon{WeaponType: "simple_melee"},
			expected: 5,
		},
		{
			name:     "ranged weapon uses long range",
			weapon:   refdata.Weapon{WeaponType: "simple_ranged", RangeNormalFt: sql.NullInt32{Int32: 80, Valid: true}, RangeLongFt: sql.NullInt32{Int32: 320, Valid: true}},
			expected: 320,
		},
		{
			name:     "ranged weapon with only normal range",
			weapon:   refdata.Weapon{WeaponType: "simple_ranged", RangeNormalFt: sql.NullInt32{Int32: 80, Valid: true}},
			expected: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaxRange(tt.weapon)
			if got != tt.expected {
				t.Errorf("MaxRange() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// --- ResolveAttack Tests ---

func makeCharacter(str, dex int, profBonus int32, equippedWeapon ...string) refdata.Character {
	scores := AbilityScores{Str: str, Dex: dex, Con: 10, Int: 10, Wis: 10, Cha: 10}
	scoresJSON, _ := json.Marshal(scores)
	wpn := ""
	if len(equippedWeapon) > 0 {
		wpn = equippedWeapon[0]
	}
	return refdata.Character{
		ID:               uuid.New(),
		AbilityScores:    scoresJSON,
		ProficiencyBonus: profBonus,
		EquippedMainHand: sql.NullString{String: wpn, Valid: wpn != ""},
	}
}

func makeLongsword() refdata.Weapon {
	return refdata.Weapon{
		ID:         "longsword",
		Name:       "Longsword",
		Damage:     "1d8",
		DamageType: "slashing",
		WeaponType: "martial_melee",
	}
}

func makeLongbow() refdata.Weapon {
	return refdata.Weapon{
		ID:            "longbow",
		Name:          "Longbow",
		Damage:        "1d8",
		DamageType:    "piercing",
		WeaponType:    "martial_ranged",
		RangeNormalFt: sql.NullInt32{Int32: 150, Valid: true},
		RangeLongFt:   sql.NullInt32{Int32: 600, Valid: true},
	}
}

func makeRapier() refdata.Weapon {
	return refdata.Weapon{
		ID:         "rapier",
		Name:       "Rapier",
		Damage:     "1d8",
		DamageType: "piercing",
		WeaponType: "martial_melee",
		Properties: []string{"finesse"},
	}
}

func TestResolveAttack_OverrideDmgMod(t *testing.T) {
	// With OverrideDmgMod=0, damage should not include ability modifier
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 5 // d6 rolls 5
	})

	zeroDmg := 0
	input := AttackInput{
		AttackerName:   "Aria",
		TargetName:     "Goblin #1",
		TargetAC:       10,
		Weapon:         refdata.Weapon{ID: "shortsword", Name: "Shortsword", Damage: "1d6", DamageType: "piercing", WeaponType: "martial_melee", Properties: []string{"finesse", "light"}},
		Scores:         AbilityScores{Str: 16, Dex: 14},
		ProfBonus:      2,
		DistanceFt:     5,
		OverrideDmgMod: &zeroDmg,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, 5, result.DamageTotal) // 5 (d6) + 0 (overridden), not +3 STR
}

func TestResolveAttack_BasicHit(t *testing.T) {
	// d20 rolls 14, damage rolls 6
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 14
		}
		return 6
	})

	input := AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		TargetAC:     13,
		Weapon:       makeLongsword(),
		Scores:       AbilityScores{Str: 16, Dex: 14},
		ProfBonus:    2,
		DistanceFt:   5,
		Cover:        CoverNone,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.False(t, result.CriticalHit)
	assert.Equal(t, 14, result.D20Roll.Chosen)
	assert.Equal(t, 19, result.D20Roll.Total) // 14 + 3(STR) + 2(prof)
	assert.Greater(t, result.DamageTotal, 0)
	assert.Equal(t, "slashing", result.DamageType)
}

func TestResolveAttack_BasicMiss(t *testing.T) {
	// d20 rolls 5, total = 5+3+2 = 10 vs AC 15 => miss
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 5
		}
		return 4
	})

	input := AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		TargetAC:     15,
		Weapon:       makeLongsword(),
		Scores:       AbilityScores{Str: 16, Dex: 14},
		ProfBonus:    2,
		DistanceFt:   5,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.False(t, result.Hit)
	assert.Equal(t, 0, result.DamageTotal)
}

func TestResolveAttack_NatTwentyCrit(t *testing.T) {
	// d20 rolls 20 (nat 20), damage dice are doubled
	callCount := 0
	roller := dice.NewRoller(func(max int) int {
		callCount++
		if max == 20 {
			return 20
		}
		return 4 // each d8 rolls 4
	})

	input := AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		TargetAC:     25, // nat 20 always hits regardless of AC
		Weapon:       makeLongsword(),
		Scores:       AbilityScores{Str: 16, Dex: 14},
		ProfBonus:    2,
		DistanceFt:   5,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.True(t, result.CriticalHit)
	// 2d8 (doubled) + 3 STR = 8 + 3 = 11
	assert.Equal(t, 11, result.DamageTotal)
}

func TestResolveAttack_AutoCritParalyzed(t *testing.T) {
	// Auto-crit: melee within 5ft against paralyzed target, no d20 roll needed
	roller := dice.NewRoller(func(max int) int {
		return 4
	})

	input := AttackInput{
		AttackerName:   "Aria",
		TargetName:     "Goblin #1",
		TargetAC:       15,
		Weapon:         makeLongsword(),
		Scores:         AbilityScores{Str: 16, Dex: 14},
		ProfBonus:      2,
		DistanceFt:     5,
		AutoCrit:       true,
		AutoCritReason: "target paralyzed within 5ft",
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.True(t, result.CriticalHit)
	assert.True(t, result.AutoCrit)
	assert.Equal(t, 11, result.DamageTotal) // 2d8(4+4) + 3
}

func TestResolveAttack_RangedWithDistance(t *testing.T) {
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 5
	})

	input := AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		TargetAC:     13,
		Weapon:       makeLongbow(),
		Scores:       AbilityScores{Str: 10, Dex: 18},
		ProfBonus:    2,
		DistanceFt:   35,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, 35, result.DistanceFt)
}

func TestResolveAttack_RangeRejected(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 10 })

	input := AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		TargetAC:     13,
		Weapon:       makeLongbow(),
		Scores:       AbilityScores{Str: 10, Dex: 18},
		ProfBonus:    2,
		DistanceFt:   650, // beyond long range (600ft)
	}

	_, err := ResolveAttack(input, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestResolveAttack_CoverBonus(t *testing.T) {
	// d20 rolls 13, total = 13+3+2=18 vs AC 13 + 2 (half cover) = 15 => hit
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 13
		}
		return 5
	})

	input := AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		TargetAC:     13,
		Weapon:       makeLongsword(),
		Scores:       AbilityScores{Str: 16, Dex: 14},
		ProfBonus:    2,
		DistanceFt:   5,
		Cover:        CoverHalf,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	// effective AC = 13 + 2 = 15, attack roll = 13+3+2 = 18 => hit
	assert.True(t, result.Hit)
	assert.Equal(t, 15, result.EffectiveAC)
}

func TestResolveAttack_CoverCausesMiss(t *testing.T) {
	// d20 rolls 10, total = 10+3+2=15 vs AC 13 + 5 (three-quarter cover) = 18 => miss
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 10
		}
		return 5
	})

	input := AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		TargetAC:     13,
		Weapon:       makeLongsword(),
		Scores:       AbilityScores{Str: 16, Dex: 14},
		ProfBonus:    2,
		DistanceFt:   5,
		Cover:        CoverThreeQuarters,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.False(t, result.Hit)
	assert.Equal(t, 18, result.EffectiveAC)
}

func TestResolveAttack_UnarmedStrike(t *testing.T) {
	// Unarmed strike: 1 + STR mod = 1 + 3 = 4 damage
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 1
	})

	input := AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		TargetAC:     10,
		Weapon:       UnarmedStrike(),
		Scores:       AbilityScores{Str: 16, Dex: 14},
		ProfBonus:    2,
		DistanceFt:   5,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, 4, result.DamageTotal) // 1 + 3 STR mod
	assert.Equal(t, "bludgeoning", result.DamageType)
}

func TestResolveAttack_UnarmedStrikeCrit(t *testing.T) {
	// Unarmed crit: 2*(1) + STR mod = 2 + 3 = 5 damage
	roller := dice.NewRoller(func(max int) int {
		return 20
	})

	input := AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		TargetAC:     25,
		Weapon:       UnarmedStrike(),
		Scores:       AbilityScores{Str: 16, Dex: 14},
		ProfBonus:    2,
		DistanceFt:   5,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.True(t, result.CriticalHit)
	assert.Equal(t, 5, result.DamageTotal) // 2*(1) + 3
}

func TestResolveAttack_FinesseAutoSelect(t *testing.T) {
	// DEX 18 > STR 10, so rapier should use DEX
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 5
	})

	input := AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		TargetAC:     10,
		Weapon:       makeRapier(),
		Scores:       AbilityScores{Str: 10, Dex: 18},
		ProfBonus:    2,
		DistanceFt:   5,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	// Attack roll: 15 + 4(DEX) + 2(prof) = 21
	assert.Equal(t, 21, result.D20Roll.Total)
	// Damage: 5 + 4(DEX) = 9
	assert.Equal(t, 9, result.DamageTotal)
}

func TestResolveAttack_InLongRange(t *testing.T) {
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 5
	})

	input := AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		TargetAC:     13,
		Weapon:       makeLongbow(),
		Scores:       AbilityScores{Str: 10, Dex: 18},
		ProfBonus:    2,
		DistanceFt:   200, // beyond 150 normal, within 600 long
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.InLongRange)
}

// --- FormatAttackLog Tests ---

func TestFormatAttackLog_Hit(t *testing.T) {
	result := AttackResult{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		WeaponName:   "Longbow",
		DistanceFt:   35,
		Hit:          true,
		D20Roll:      dice.D20Result{Chosen: 12, Total: 14, Modifier: 2},
		EffectiveAC:  13,
		DamageTotal:  8,
		DamageType:   "piercing",
		DamageDice:   "1d8+2",
	}

	log := FormatAttackLog(result)
	assert.Contains(t, log, "Aria attacks Goblin #1 with Longbow (35ft)")
	assert.Contains(t, log, "HIT")
	assert.Contains(t, log, "piercing")
}

func TestFormatAttackLog_Miss(t *testing.T) {
	result := AttackResult{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		WeaponName:   "Longsword",
		DistanceFt:   5,
		Hit:          false,
		D20Roll:      dice.D20Result{Chosen: 5, Total: 10, Modifier: 5},
		EffectiveAC:  15,
	}

	log := FormatAttackLog(result)
	assert.Contains(t, log, "MISS")
	assert.NotContains(t, log, "Damage")
}

func TestFormatAttackLog_Crit(t *testing.T) {
	result := AttackResult{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		WeaponName:   "Longsword",
		DistanceFt:   5,
		Hit:          true,
		CriticalHit:  true,
		D20Roll:      dice.D20Result{Chosen: 20, Total: 25, Modifier: 5, CriticalHit: true},
		DamageTotal:  18,
		DamageType:   "slashing",
		DamageDice:   "2d8+5",
	}

	log := FormatAttackLog(result)
	assert.Contains(t, log, "NAT 20")
	assert.Contains(t, log, "CRITICAL HIT")
}

func TestFormatAttackLog_AutoCrit(t *testing.T) {
	result := AttackResult{
		AttackerName:   "Aria",
		TargetName:     "Goblin #1",
		WeaponName:     "Longsword",
		DistanceFt:     5,
		Hit:            true,
		CriticalHit:    true,
		AutoCrit:       true,
		AutoCritReason: "target paralyzed within 5ft",
		DamageTotal:    18,
		DamageType:     "slashing",
		DamageDice:     "2d8+5",
	}

	log := FormatAttackLog(result)
	assert.Contains(t, log, "auto-crit")
	assert.Contains(t, log, "paralyzed")
	assert.NotContains(t, log, "Roll to hit") // auto-crit skips the roll line
}

func TestFormatAttackLog_MeleeNoDistance(t *testing.T) {
	result := AttackResult{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		WeaponName:   "Longsword",
		DistanceFt:   5,
		Hit:          true,
		D20Roll:      dice.D20Result{Chosen: 15, Total: 20, Modifier: 5},
		DamageTotal:  10,
		DamageType:   "slashing",
		DamageDice:   "1d8+5",
		IsMelee:      true,
	}

	log := FormatAttackLog(result)
	// Melee at 5ft shouldn't show distance
	assert.NotContains(t, log, "(5ft)")
}

// --- Service.Attack integration test ---

func TestServiceAttack_BasicFlow(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 14, 2, "longsword")
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		if id == "longsword" {
			return makeLongsword(), nil
		}
		return refdata.Weapon{}, sql.ErrNoRows
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 6
	})

	attacker := refdata.Combatant{
		ID:          attackerID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Aria",
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin #1",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          13,
		IsAlive:     true,
		IsNpc:       true,
		Conditions:  json.RawMessage(`[]`),
	}
	turn := refdata.Turn{
		ID:               turnID,
		EncounterID:      encounterID,
		CombatantID:      attackerID,
		AttacksRemaining: 1,
	}

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "Longsword", result.WeaponName)
}

func TestServiceAttack_NoAttacksRemaining(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	ms := defaultMockStore()
	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	turn := refdata.Turn{
		ID:               turnID,
		CombatantID:      attackerID,
		AttacksRemaining: 0, // no attacks left
	}

	_, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, IsAlive: true, Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:     turn,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "attack")
}

func TestServiceAttack_WeaponOverride(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacter(16, 14, 2, "longsword")
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		if id == "handaxe" {
			return refdata.Weapon{
				ID: "handaxe", Name: "Handaxe", Damage: "1d6", DamageType: "slashing",
				WeaponType: "simple_melee",
			}, nil
		}
		return refdata.Weapon{}, sql.ErrNoRows
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 4
	})

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker:       refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:         refdata.Combatant{ID: targetID, DisplayName: "Goblin #1", PositionCol: "B", PositionRow: 1, Ac: 13, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Turn:           refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
		WeaponOverride: "handaxe",
	}, roller)
	require.NoError(t, err)
	assert.Equal(t, "Handaxe", result.WeaponName)
}

func TestServiceAttack_UnarmedWhenNoWeaponEquipped(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacter(16, 14, 2, "") // no weapon equipped
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 1
	})

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin #1", PositionCol: "B", PositionRow: 1, Ac: 10, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.NoError(t, err)
	assert.Equal(t, "Unarmed Strike", result.WeaponName)
	assert.Equal(t, 4, result.DamageTotal) // 1 + 3 STR mod
}

func TestCheckAutoCrit(t *testing.T) {
	tests := []struct {
		name       string
		conditions json.RawMessage
		distFt     int
		isMelee    bool
		expectCrit bool
	}{
		{
			name:       "paralyzed within 5ft melee",
			conditions: json.RawMessage(`[{"condition":"paralyzed"}]`),
			distFt:     5,
			isMelee:    true,
			expectCrit: true,
		},
		{
			name:       "unconscious within 5ft melee",
			conditions: json.RawMessage(`[{"condition":"unconscious"}]`),
			distFt:     5,
			isMelee:    true,
			expectCrit: true,
		},
		{
			name:       "paralyzed but ranged",
			conditions: json.RawMessage(`[{"condition":"paralyzed"}]`),
			distFt:     5,
			isMelee:    false,
			expectCrit: false,
		},
		{
			name:       "paralyzed but beyond 5ft",
			conditions: json.RawMessage(`[{"condition":"paralyzed"}]`),
			distFt:     10,
			isMelee:    true,
			expectCrit: false,
		},
		{
			name:       "no conditions",
			conditions: json.RawMessage(`[]`),
			distFt:     5,
			isMelee:    true,
			expectCrit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crit, _ := CheckAutoCrit(tt.conditions, tt.distFt, tt.isMelee)
			assert.Equal(t, tt.expectCrit, crit)
		})
	}
}

func TestDamageExpression(t *testing.T) {
	tests := []struct {
		name     string
		weapon   refdata.Weapon
		mod      int
		expected string
	}{
		{
			name:     "positive modifier",
			weapon:   refdata.Weapon{ID: "longsword", Damage: "1d8"},
			mod:      3,
			expected: "1d8+3",
		},
		{
			name:     "zero modifier",
			weapon:   refdata.Weapon{ID: "longsword", Damage: "1d8"},
			mod:      0,
			expected: "1d8",
		},
		{
			name:     "negative modifier",
			weapon:   refdata.Weapon{ID: "longsword", Damage: "1d8"},
			mod:      -1,
			expected: "1d8-1",
		},
		{
			name:     "unarmed strike returns empty",
			weapon:   UnarmedStrike(),
			mod:      3,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DamageExpression(tt.weapon, tt.mod)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestNormalRange(t *testing.T) {
	tests := []struct {
		name     string
		weapon   refdata.Weapon
		expected int
	}{
		{
			name:     "melee returns 5",
			weapon:   refdata.Weapon{WeaponType: "simple_melee"},
			expected: 5,
		},
		{
			name:     "ranged with normal range",
			weapon:   refdata.Weapon{WeaponType: "simple_ranged", RangeNormalFt: sql.NullInt32{Int32: 80, Valid: true}},
			expected: 80,
		},
		{
			name:     "ranged without normal range",
			weapon:   refdata.Weapon{WeaponType: "simple_ranged"},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalRange(tt.weapon)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestIsInLongRange(t *testing.T) {
	bow := makeLongbow() // normal 150, long 600
	assert.False(t, IsInLongRange(bow, 100))  // within normal
	assert.False(t, IsInLongRange(bow, 150))  // at normal
	assert.True(t, IsInLongRange(bow, 200))   // in long
	assert.True(t, IsInLongRange(bow, 600))   // at max long
	assert.False(t, IsInLongRange(bow, 601))  // beyond long

	melee := makeLongsword()
	assert.False(t, IsInLongRange(melee, 5))
}

func TestColToIndex(t *testing.T) {
	assert.Equal(t, 0, colToIndex("A"))
	assert.Equal(t, 1, colToIndex("B"))
	assert.Equal(t, 25, colToIndex("Z"))
	assert.Equal(t, 0, colToIndex("a")) // lowercase
	assert.Equal(t, 0, colToIndex(""))  // empty
}

func TestBuildCritDiceDisplay(t *testing.T) {
	weapon := refdata.Weapon{Damage: "1d8"}
	assert.Equal(t, "2d8+3", buildCritDiceDisplay(weapon, 3))
	assert.Equal(t, "2d8", buildCritDiceDisplay(weapon, 0))
	assert.Equal(t, "2d8-1", buildCritDiceDisplay(weapon, -1))
}

func TestServiceAttack_NPCAttacker(t *testing.T) {
	ctx := context.Background()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	ms := defaultMockStore()
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 1
	})

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, DisplayName: "Goblin #1", IsNpc: true, PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Aria", PositionCol: "B", PositionRow: 1, Ac: 16, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.NoError(t, err)
	assert.Equal(t, "Unarmed Strike", result.WeaponName)
}

func TestFormatAttackLog_RangedWithLongRange(t *testing.T) {
	result := AttackResult{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		WeaponName:   "Longbow",
		DistanceFt:   200,
		IsMelee:      false,
		Hit:          true,
		InLongRange:  true,
		D20Roll:      dice.D20Result{Chosen: 15, Total: 21, Modifier: 6},
		DamageTotal:  9,
		DamageType:   "piercing",
		DamageDice:   "1d8+4",
	}

	log := FormatAttackLog(result)
	assert.Contains(t, log, "(200ft)")
}

func TestResolveAttack_MeleeOutOfRange(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 10 })

	input := AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		TargetAC:     13,
		Weapon:       makeLongsword(),
		Scores:       AbilityScores{Str: 16, Dex: 14},
		ProfBonus:    2,
		DistanceFt:   10, // melee reach is 5ft
	}

	_, err := ResolveAttack(input, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestMaxRange_RangedNoRanges(t *testing.T) {
	weapon := refdata.Weapon{WeaponType: "simple_ranged"} // no range values set
	assert.Equal(t, 5, MaxRange(weapon))
}

func TestResolveAttack_NatOneAlwaysMisses(t *testing.T) {
	// Nat 1 always misses even with huge modifier vs low AC
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 1
		}
		return 6
	})

	input := AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		TargetAC:     5, // very low AC
		Weapon:       makeLongsword(),
		Scores:       AbilityScores{Str: 20, Dex: 14}, // +5 STR mod
		ProfBonus:    6,                                 // high prof
		DistanceFt:   5,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.False(t, result.Hit, "nat 1 should always miss")
	assert.True(t, result.D20Roll.CriticalFail)
	assert.Equal(t, 0, result.DamageTotal)
}

func TestCheckAutoCrit_BadJSON(t *testing.T) {
	crit, reason := CheckAutoCrit(json.RawMessage(`invalid`), 5, true)
	assert.False(t, crit)
	assert.Equal(t, "", reason)
}

func TestServiceAttack_AutoCritParalyzedFlow(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacter(16, 14, 2, "longsword")
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeLongsword(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 4 })

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin #1", PositionCol: "B", PositionRow: 1, Ac: 13, IsAlive: true, Conditions: json.RawMessage(`[{"condition":"paralyzed"}]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.AutoCrit)
	assert.True(t, result.CriticalHit)
	assert.True(t, result.Hit)
}

func TestServiceAttack_RangeRejectedFlow(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacter(16, 14, 2, "longsword")
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeLongsword(), nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	// Attacker at A1, target at Z1 => far apart, melee weapon can't reach
	_, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin #1", PositionCol: "Z", PositionRow: 1, Ac: 13, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestResolveWeaponDamage_CritWithWeapon(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 4 })
	weapon := makeLongsword()
	dmg, damageDice, rollResult := resolveWeaponDamage(weapon, 3, true, false, roller)
	assert.Greater(t, dmg, 0)
	assert.Equal(t, "2d8+3", damageDice) // doubled dice
	assert.NotNil(t, rollResult)
}

func TestResolveWeaponDamage_NormalWithWeapon(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 5 })
	weapon := makeLongsword()
	dmg, damageDice, rollResult := resolveWeaponDamage(weapon, 3, false, false, roller)
	assert.Equal(t, 8, dmg) // 5 + 3
	assert.Equal(t, "1d8+3", damageDice)
	assert.NotNil(t, rollResult)
}

func TestResolveWeaponDamage_UnarmedNormal(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 1 })
	weapon := UnarmedStrike()
	dmg, _, _ := resolveWeaponDamage(weapon, 3, false, false, roller)
	assert.Equal(t, 4, dmg) // 1 + 3
}

func TestResolveWeaponDamage_UnarmedCrit(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 1 })
	weapon := UnarmedStrike()
	dmg, _, _ := resolveWeaponDamage(weapon, 3, true, false, roller)
	assert.Equal(t, 5, dmg) // 2 + 3
}

func TestResolveWeaponDamage_UnarmedNegativeMod(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 1 })
	weapon := UnarmedStrike()
	dmg, _, _ := resolveWeaponDamage(weapon, -5, false, false, roller)
	assert.Equal(t, 0, dmg) // 1 + (-5) = -4, clamped to 0
}

func TestServiceAttack_GetCharacterError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin #1", PositionCol: "B", PositionRow: 1, Ac: 13, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolving weapon")
}

func TestServiceAttack_GetWeaponError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacter(16, 14, 2, "nonexistent-weapon")
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	// getWeaponFn defaults to sql.ErrNoRows

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin #1", PositionCol: "B", PositionRow: 1, Ac: 13, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting weapon")
}

func TestServiceAttack_UpdateTurnError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacter(16, 14, 2, "longsword")
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeLongsword(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 5
	})

	_, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin #1", PositionCol: "B", PositionRow: 1, Ac: 13, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

func TestServiceAttack_BadAbilityScoresJSON(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := refdata.Character{
		ID:               charID,
		AbilityScores:    json.RawMessage(`invalid`),
		ProficiencyBonus: 2,
		EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin #1", PositionCol: "B", PositionRow: 1, Ac: 13, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing ability scores")
}

// --- OffhandAttack Tests ---

func makeShortsword() refdata.Weapon {
	return refdata.Weapon{
		ID:         "shortsword",
		Name:       "Shortsword",
		Damage:     "1d6",
		DamageType: "piercing",
		WeaponType: "martial_melee",
		Properties: []string{"finesse", "light"},
	}
}

func makeDagger() refdata.Weapon {
	return refdata.Weapon{
		ID:         "dagger",
		Name:       "Dagger",
		Damage:     "1d4",
		DamageType: "piercing",
		WeaponType: "simple_melee",
		Properties: []string{"finesse", "light", "thrown"},
	}
}

func TestServiceOffhandAttack_HappyPath_NoTWFStyle(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacter(16, 14, 2, "shortsword")
	char.ID = charID
	char.EquippedOffHand = sql.NullString{String: "dagger", Valid: true}
	// No TWF fighting style

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		switch id {
		case "shortsword":
			return makeShortsword(), nil
		case "dagger":
			return makeDagger(), nil
		}
		return refdata.Weapon{}, sql.ErrNoRows
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 3 // d4 rolls 3
	})

	result, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin #1", PositionCol: "B", PositionRow: 1, Ac: 13, IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "Dagger", result.WeaponName)
	// Damage = d4(3) + 0 (no TWF style, no ability mod) = 3
	assert.Equal(t, 3, result.DamageTotal)
}

func TestServiceOffhandAttack_HappyPath_WithTWFStyle(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacter(16, 14, 2, "shortsword")
	char.ID = charID
	char.EquippedOffHand = sql.NullString{String: "dagger", Valid: true}
	char.Features = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`[{"name":"Two-Weapon Fighting","mechanical_effect":"two_weapon_fighting"}]`),
		Valid:      true,
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		switch id {
		case "shortsword":
			return makeShortsword(), nil
		case "dagger":
			return makeDagger(), nil
		}
		return refdata.Weapon{}, sql.ErrNoRows
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 3 // d4 rolls 3
	})

	result, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin #1", PositionCol: "B", PositionRow: 1, Ac: 13, IsAlive: true, IsNpc: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "Dagger", result.WeaponName)
	// Dagger is finesse: uses higher of STR(3)/DEX(2) = STR mod 3
	// Damage = d4(3) + 3 (TWF style adds ability mod) = 6
	assert.Equal(t, 6, result.DamageTotal)
}

func TestServiceOffhandAttack_BonusActionAlreadyUsed(t *testing.T) {
	ctx := context.Background()
	svc := NewService(defaultMockStore())
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true}, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: uuid.New(), BonusActionUsed: true},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bonus action")
}

func TestServiceOffhandAttack_NPCAttacker(t *testing.T) {
	ctx := context.Background()
	svc := NewService(defaultMockStore())
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: refdata.Combatant{ID: uuid.New(), IsNpc: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: uuid.New()},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a character")
}

func TestServiceOffhandAttack_NoMainHandWeapon(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	char := makeCharacter(16, 14, 2, "") // no main hand weapon
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: uuid.New()},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no main hand weapon")
}

func TestServiceOffhandAttack_MainHandNotLight(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	char := makeCharacter(16, 14, 2, "longsword")
	char.ID = charID
	char.EquippedOffHand = sql.NullString{String: "dagger", Valid: true}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		switch id {
		case "longsword":
			return makeLongsword(), nil // longsword is NOT light
		case "dagger":
			return makeDagger(), nil
		}
		return refdata.Weapon{}, sql.ErrNoRows
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: uuid.New()},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not light")
}

func TestServiceOffhandAttack_NoOffHandWeapon(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	char := makeCharacter(16, 14, 2, "shortsword")
	char.ID = charID
	// No off-hand weapon

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		if id == "shortsword" {
			return makeShortsword(), nil
		}
		return refdata.Weapon{}, sql.ErrNoRows
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: uuid.New()},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no off-hand weapon")
}

func TestServiceOffhandAttack_OffHandNotLight(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	char := makeCharacter(16, 14, 2, "shortsword")
	char.ID = charID
	char.EquippedOffHand = sql.NullString{String: "longsword", Valid: true}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		switch id {
		case "shortsword":
			return makeShortsword(), nil
		case "longsword":
			return makeLongsword(), nil // NOT light
		}
		return refdata.Weapon{}, sql.ErrNoRows
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: uuid.New()},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not light")
}

func TestServiceOffhandAttack_GetCharacterError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: uuid.New()},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

func TestServiceOffhandAttack_UpdateTurnActionsError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	char := makeCharacter(16, 14, 2, "shortsword")
	char.ID = charID
	char.EquippedOffHand = sql.NullString{String: "dagger", Valid: true}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		switch id {
		case "shortsword":
			return makeShortsword(), nil
		case "dagger":
			return makeDagger(), nil
		}
		return refdata.Weapon{}, sql.ErrNoRows
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

	_, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: uuid.New(), DisplayName: "Goblin", PositionCol: "B", PositionRow: 1, Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: uuid.New()},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

func TestServiceOffhandAttack_GetOffHandWeaponError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	char := makeCharacter(16, 14, 2, "shortsword")
	char.ID = charID
	char.EquippedOffHand = sql.NullString{String: "nonexistent", Valid: true}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		if id == "shortsword" {
			return makeShortsword(), nil
		}
		return refdata.Weapon{}, sql.ErrNoRows
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: uuid.New()},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting off-hand weapon")
}

func TestServiceOffhandAttack_GetMainHandWeaponError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	char := makeCharacter(16, 14, 2, "nonexistent")
	char.ID = charID
	char.EquippedOffHand = sql.NullString{String: "dagger", Valid: true}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	// Default getWeaponFn returns sql.ErrNoRows

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: uuid.New()},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting main hand weapon")
}

func TestServiceOffhandAttack_BadAbilityScores(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	char := refdata.Character{
		ID:               charID,
		AbilityScores:    json.RawMessage(`invalid`),
		ProficiencyBonus: 2,
		EquippedMainHand: sql.NullString{String: "shortsword", Valid: true},
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: uuid.New()},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing ability scores")
}

// --- Phase 37: Weapon Properties Tests ---

func makeVersatileLongsword() refdata.Weapon {
	return refdata.Weapon{
		ID:              "longsword",
		Name:            "Longsword",
		Damage:          "1d8",
		DamageType:      "slashing",
		WeaponType:      "martial_melee",
		Properties:      []string{"versatile"},
		VersatileDamage: sql.NullString{String: "1d10", Valid: true},
	}
}

func makeJavelin() refdata.Weapon {
	return refdata.Weapon{
		ID:            "javelin",
		Name:          "Javelin",
		Damage:        "1d6",
		DamageType:    "piercing",
		WeaponType:    "simple_melee",
		Properties:    []string{"thrown"},
		RangeNormalFt: sql.NullInt32{Int32: 30, Valid: true},
		RangeLongFt:   sql.NullInt32{Int32: 120, Valid: true},
	}
}

func makeHeavyCrossbowFull() refdata.Weapon {
	return refdata.Weapon{
		ID:            "heavy-crossbow",
		Name:          "Heavy Crossbow",
		Damage:        "1d10",
		DamageType:    "piercing",
		WeaponType:    "martial_ranged",
		Properties:    []string{"heavy", "loading", "ammunition"},
		RangeNormalFt: sql.NullInt32{Int32: 100, Valid: true},
		RangeLongFt:   sql.NullInt32{Int32: 400, Valid: true},
	}
}

func makeGlaive() refdata.Weapon {
	return refdata.Weapon{
		ID:         "glaive",
		Name:       "Glaive",
		Damage:     "1d10",
		DamageType: "slashing",
		WeaponType: "martial_melee",
		Properties: []string{"heavy", "reach", "two-handed"},
	}
}

func TestVersatileDamageExpression(t *testing.T) {
	weapon := makeVersatileLongsword()
	// Two-handed: should use versatile_damage (1d10)
	got := VersatileDamageExpression(weapon, 3)
	assert.Equal(t, "1d10+3", got)

	// Weapon without versatile_damage: fall back to base damage
	weapon2 := makeLongsword()
	got2 := VersatileDamageExpression(weapon2, 3)
	assert.Equal(t, "1d8+3", got2)
}

func TestResolveAttack_VersatileTwoHanded(t *testing.T) {
	weapon := makeVersatileLongsword()
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 8 // max on d10
	})

	result, err := ResolveAttack(AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin",
		TargetAC:     12,
		Weapon:       weapon,
		Scores:       AbilityScores{Str: 16, Dex: 10},
		ProfBonus:    2,
		DistanceFt:   5,
		TwoHanded:    true,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	// Versatile 1d10+3 with roll of 8 = 11
	assert.Equal(t, 11, result.DamageTotal)
}

func TestResolveAttack_VersatileOneHanded(t *testing.T) {
	weapon := makeVersatileLongsword()
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 6
	})

	result, err := ResolveAttack(AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin",
		TargetAC:     12,
		Weapon:       weapon,
		Scores:       AbilityScores{Str: 16, Dex: 10},
		ProfBonus:    2,
		DistanceFt:   5,
		TwoHanded:    false,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	// Base 1d8+3 with roll of 6 = 9
	assert.Equal(t, 9, result.DamageTotal)
}

func TestResolveAttack_VersatileTwoHandedRejectedWhenOffHandOccupied(t *testing.T) {
	weapon := makeVersatileLongsword()
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := ResolveAttack(AttackInput{
		AttackerName:    "Aria",
		TargetName:      "Goblin",
		TargetAC:        12,
		Weapon:          weapon,
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		TwoHanded:       true,
		OffHandOccupied: true,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "off-hand")
}

// --- Reach Tests ---

func TestMaxRange_ReachWeapon(t *testing.T) {
	weapon := makeGlaive()
	assert.Equal(t, 10, MaxRange(weapon))
}

func TestNormalRange_ReachWeapon(t *testing.T) {
	weapon := makeGlaive()
	assert.Equal(t, 10, NormalRange(weapon))
}

func TestResolveAttack_ReachWeaponAt10ft(t *testing.T) {
	weapon := makeGlaive()
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 6
	})

	result, err := ResolveAttack(AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin",
		TargetAC:     12,
		Weapon:       weapon,
		Scores:       AbilityScores{Str: 16, Dex: 10},
		ProfBonus:    2,
		DistanceFt:   10,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
}

func TestResolveAttack_ReachWeaponOutOfRange(t *testing.T) {
	weapon := makeGlaive()
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := ResolveAttack(AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin",
		TargetAC:     12,
		Weapon:       weapon,
		Scores:       AbilityScores{Str: 16, Dex: 10},
		ProfBonus:    2,
		DistanceFt:   15,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

// --- Loading Tests ---

func TestResolveAttack_LoadingWeaponLimitsToOneAttack(t *testing.T) {
	weapon := makeHeavyCrossbowFull()
	// Loading flag should be set in AttackInput
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 6
	})

	result, err := ResolveAttack(AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin",
		TargetAC:     12,
		Weapon:       weapon,
		Scores:       AbilityScores{Str: 10, Dex: 16},
		ProfBonus:    2,
		DistanceFt:   30,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	// Loading limit is enforced at the service level, not in ResolveAttack
}

func TestLoadingWeaponLimitsAttacks(t *testing.T) {
	// Loading weapons should cap attacks remaining to 1
	assert.Equal(t, int32(1), ApplyLoadingLimit(3, true, false))
}

func TestLoadingWeaponCrossbowExpertOverride(t *testing.T) {
	// Crossbow Expert ignores loading limit
	assert.Equal(t, int32(3), ApplyLoadingLimit(3, true, true))
}

func TestLoadingWeaponNotLoading(t *testing.T) {
	// Non-loading weapon: no limit
	assert.Equal(t, int32(3), ApplyLoadingLimit(3, false, false))
}

// --- Thrown Weapon Tests ---

func TestMaxRange_ThrownMeleeWeapon(t *testing.T) {
	weapon := makeJavelin()
	// Thrown melee weapon should use its range values when thrown
	assert.Equal(t, 120, ThrownMaxRange(weapon))
}

func TestNormalRange_ThrownMeleeWeapon(t *testing.T) {
	weapon := makeJavelin()
	assert.Equal(t, 30, ThrownNormalRange(weapon))
}

func TestIsInLongRange_ThrownWeapon(t *testing.T) {
	weapon := makeJavelin()
	// 50ft is beyond normal 30ft but within long 120ft
	assert.True(t, IsThrownInLongRange(weapon, 50))
	// 20ft is within normal range
	assert.False(t, IsThrownInLongRange(weapon, 20))
	// 130ft is beyond long range
	assert.False(t, IsThrownInLongRange(weapon, 130))
}

func TestResolveAttack_ThrownWeaponAtRange(t *testing.T) {
	weapon := makeJavelin()
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 4
	})

	result, err := ResolveAttack(AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin",
		TargetAC:     12,
		Weapon:       weapon,
		Scores:       AbilityScores{Str: 16, Dex: 10},
		ProfBonus:    2,
		DistanceFt:   20,
		Thrown:        true,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
}

func TestResolveAttack_ThrownWeaponBeyondLongRange(t *testing.T) {
	weapon := makeJavelin()
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := ResolveAttack(AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin",
		TargetAC:     12,
		Weapon:       weapon,
		Scores:       AbilityScores{Str: 16, Dex: 10},
		ProfBonus:    2,
		DistanceFt:   130,
		Thrown:        true,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestResolveAttack_ThrownWeaponInLongRangeDisadvantage(t *testing.T) {
	weapon := makeJavelin()
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 4
	})

	result, err := ResolveAttack(AttackInput{
		AttackerName: "Aria",
		TargetName:   "Goblin",
		TargetAC:     12,
		Weapon:       weapon,
		Scores:       AbilityScores{Str: 16, Dex: 10},
		ProfBonus:    2,
		DistanceFt:   50, // beyond normal 30, within long 120
		Thrown:        true,
	}, roller)
	require.NoError(t, err)
	// Should have disadvantage from long range
	assert.Equal(t, dice.Disadvantage, result.RollMode)
}

// --- Improvised Weapon Tests ---

func TestImprovised_DamageAndType(t *testing.T) {
	w := ImprovisedWeapon()
	assert.Equal(t, "1d4", w.Damage)
	assert.Equal(t, "bludgeoning", w.DamageType)
	assert.Equal(t, "Improvised Weapon", w.Name)
}

func TestResolveAttack_ImprovisedNoProficiency(t *testing.T) {
	weapon := ImprovisedWeapon()
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 10
		}
		return 3
	})

	result, err := ResolveAttack(AttackInput{
		AttackerName:    "Aria",
		TargetName:      "Goblin",
		TargetAC:        12,
		Weapon:          weapon,
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		IsImprovised:    true,
		HasTavernBrawler: false,
	}, roller)
	require.NoError(t, err)
	// Attack mod: STR mod (3) only, no prof bonus
	// d20 roll: 10 + 3 = 13 >= 12, hit
	assert.True(t, result.Hit)
}

func TestResolveAttack_ImprovisedWithTavernBrawlerGetsProficiency(t *testing.T) {
	weapon := ImprovisedWeapon()
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 8 // 8 + 3 = 11 miss without prof, 8 + 3 + 2 = 13 hit with prof
		}
		return 3
	})

	result, err := ResolveAttack(AttackInput{
		AttackerName:    "Aria",
		TargetName:      "Goblin",
		TargetAC:        12,
		Weapon:          weapon,
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		IsImprovised:    true,
		HasTavernBrawler: true,
	}, roller)
	require.NoError(t, err)
	// With Tavern Brawler: 8 + 3 + 2 = 13 >= 12, hit
	assert.True(t, result.Hit)
}

func TestResolveAttack_ImprovisedThrown(t *testing.T) {
	weapon := ImprovisedWeapon()
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 3
	})

	result, err := ResolveAttack(AttackInput{
		AttackerName:     "Aria",
		TargetName:       "Goblin",
		TargetAC:         12,
		Weapon:           weapon,
		Scores:           AbilityScores{Str: 16, Dex: 10},
		ProfBonus:        2,
		DistanceFt:       15,
		IsImprovised:     true,
		ImprovisedThrown: true,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
}

func TestResolveAttack_ImprovisedThrownBeyondLongRange(t *testing.T) {
	weapon := ImprovisedWeapon()
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := ResolveAttack(AttackInput{
		AttackerName:     "Aria",
		TargetName:       "Goblin",
		TargetAC:         12,
		Weapon:           weapon,
		Scores:           AbilityScores{Str: 16, Dex: 10},
		ProfBonus:        2,
		DistanceFt:       65,
		IsImprovised:     true,
		ImprovisedThrown: true,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

// --- HasFeat Tests ---

func TestHasFeat(t *testing.T) {
	features := pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`[{"name":"Crossbow Expert","mechanical_effect":"crossbow-expert"},{"name":"Tavern Brawler","mechanical_effect":"tavern-brawler"}]`),
		Valid:      true,
	}
	assert.True(t, HasFeat(features, "crossbow-expert"))
	assert.True(t, HasFeat(features, "tavern-brawler"))
	assert.False(t, HasFeat(features, "sharpshooter"))
}

func TestHasFeat_NullFeatures(t *testing.T) {
	features := pqtype.NullRawMessage{Valid: false}
	assert.False(t, HasFeat(features, "crossbow-expert"))
}

func TestHasFeat_InvalidJSON(t *testing.T) {
	features := pqtype.NullRawMessage{RawMessage: json.RawMessage(`invalid`), Valid: true}
	assert.False(t, HasFeat(features, "crossbow-expert"))
}

// --- Ammunition Tests ---

func TestInventoryItem_ParseAndDeduct(t *testing.T) {
	inv := json.RawMessage(`[{"name":"Arrows","quantity":18,"type":"ammunition"}]`)
	items, err := ParseInventory(inv)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Arrows", items[0].Name)
	assert.Equal(t, 18, items[0].Quantity)

	items, err = DeductAmmunition(items, "Arrows")
	require.NoError(t, err)
	assert.Equal(t, 17, items[0].Quantity)
}

func TestDeductAmmunition_Empty(t *testing.T) {
	items := []InventoryItem{{Name: "Arrows", Quantity: 0, Type: "ammunition"}}
	_, err := DeductAmmunition(items, "Arrows")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "No arrows remaining")
}

func TestDeductAmmunition_NotFound(t *testing.T) {
	items := []InventoryItem{{Name: "Bolts", Quantity: 10, Type: "ammunition"}}
	_, err := DeductAmmunition(items, "Arrows")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "No arrows remaining")
}

func TestRecoverAmmunition(t *testing.T) {
	items := []InventoryItem{{Name: "Arrows", Quantity: 10, Type: "ammunition"}}
	// Used 8 arrows (had 18, now 10): recover half of 8 = 4
	items = RecoverAmmunition(items, "Arrows", 8)
	assert.Equal(t, 14, items[0].Quantity)
}

func TestRecoverAmmunition_RoundsDown(t *testing.T) {
	items := []InventoryItem{{Name: "Arrows", Quantity: 10, Type: "ammunition"}}
	// Used 7 arrows: recover half of 7 = 3 (rounded down)
	items = RecoverAmmunition(items, "Arrows", 7)
	assert.Equal(t, 13, items[0].Quantity)
}

func TestParseInventory_Empty(t *testing.T) {
	items, err := ParseInventory(nil)
	require.NoError(t, err)
	assert.Nil(t, items)
}

func TestParseInventory_InvalidJSON(t *testing.T) {
	_, err := ParseInventory(json.RawMessage(`invalid`))
	require.Error(t, err)
}

func TestRecoverAmmunition_NotFound(t *testing.T) {
	items := []InventoryItem{{Name: "Bolts", Quantity: 10, Type: "ammunition"}}
	items = RecoverAmmunition(items, "Arrows", 5)
	// No change since "Arrows" not found
	assert.Equal(t, 10, items[0].Quantity)
}

func TestThrownMaxRange_NoRangeValues(t *testing.T) {
	weapon := refdata.Weapon{WeaponType: "simple_melee", Properties: []string{"thrown"}}
	assert.Equal(t, 5, ThrownMaxRange(weapon))
}

func TestThrownMaxRange_OnlyNormalRange(t *testing.T) {
	weapon := refdata.Weapon{
		WeaponType:    "simple_melee",
		Properties:    []string{"thrown"},
		RangeNormalFt: sql.NullInt32{Int32: 20, Valid: true},
	}
	assert.Equal(t, 20, ThrownMaxRange(weapon))
}

func TestThrownNormalRange_NoRangeValues(t *testing.T) {
	weapon := refdata.Weapon{WeaponType: "simple_melee", Properties: []string{"thrown"}}
	assert.Equal(t, 5, ThrownNormalRange(weapon))
}

func TestResolveWeaponDamage_VersatileTwoHanded(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 8 })
	weapon := makeVersatileLongsword()
	dmg, damageDice, rollResult := resolveWeaponDamage(weapon, 3, false, true, roller)
	assert.Equal(t, 11, dmg)         // 8 + 3
	assert.Equal(t, "1d10+3", damageDice)
	assert.NotNil(t, rollResult)
}

func TestResolveWeaponDamage_VersatileTwoHandedCrit(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 8 })
	weapon := makeVersatileLongsword()
	dmg, damageDice, rollResult := resolveWeaponDamage(weapon, 3, true, true, roller)
	assert.Greater(t, dmg, 0)
	assert.Equal(t, "2d10+3", damageDice) // doubled versitle dice
	assert.NotNil(t, rollResult)
}

func TestResolveAttack_ImprovisedThrownLongRange(t *testing.T) {
	weapon := ImprovisedWeapon()
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 3
	})

	result, err := ResolveAttack(AttackInput{
		AttackerName:     "Aria",
		TargetName:       "Goblin",
		TargetAC:         12,
		Weapon:           weapon,
		Scores:           AbilityScores{Str: 16, Dex: 10},
		ProfBonus:        2,
		DistanceFt:       30, // beyond normal 20, within long 60
		IsImprovised:     true,
		ImprovisedThrown: true,
	}, roller)
	require.NoError(t, err)
	// Should have disadvantage from long range
	assert.Equal(t, dice.Disadvantage, result.RollMode)
}

func TestServiceAttack_ImprovisedNPC(t *testing.T) {
	ctx := context.Background()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	ms := defaultMockStore()
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 3
	})

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker:     refdata.Combatant{ID: attackerID, DisplayName: "NPC", PositionCol: "A", PositionRow: 1, IsNpc: true, Conditions: json.RawMessage(`[]`)},
		Target:       refdata.Combatant{ID: targetID, DisplayName: "Goblin", PositionCol: "B", PositionRow: 1, Ac: 12, Conditions: json.RawMessage(`[]`)},
		Turn:         refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1},
		IsImprovised: true,
	}, roller)
	require.NoError(t, err)
	assert.Equal(t, "Improvised Weapon", result.WeaponName)
}

func TestGetAmmunitionForWeapon(t *testing.T) {
	longbow := makeLongbow()
	longbow.Properties = []string{"ammunition"}
	// Longbow uses Arrows
	assert.Equal(t, "Arrows", GetAmmunitionName(longbow))

	crossbow := makeHeavyCrossbowFull()
	assert.Equal(t, "Bolts", GetAmmunitionName(crossbow))
}

// --- Service-Level Phase 37 Tests ---

func TestServiceAttack_VersatileTwoHanded(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 10, 2, "longsword")
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		if id == "longsword" {
			return makeVersatileLongsword(), nil
		}
		return refdata.Weapon{}, fmt.Errorf("not found")
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 8
	})

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker:  refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, Conditions: json.RawMessage(`[]`)},
		Target:    refdata.Combatant{ID: targetID, DisplayName: "Goblin", PositionCol: "B", PositionRow: 1, Ac: 12, Conditions: json.RawMessage(`[]`)},
		Turn:      refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1},
		TwoHanded: true,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	// Versatile 1d10+3, roll of 8 = 11
	assert.Equal(t, 11, result.DamageTotal)
}

func TestServiceAttack_VersatileTwoHanded_OffHandOccupied(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 10, 2, "longsword")
	char.ID = charID
	char.EquippedOffHand = sql.NullString{String: "shield", Valid: true}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeVersatileLongsword(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.Attack(ctx, AttackCommand{
		Attacker:  refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, Conditions: json.RawMessage(`[]`)},
		Target:    refdata.Combatant{ID: targetID, DisplayName: "Goblin", PositionCol: "B", PositionRow: 1, Ac: 12, Conditions: json.RawMessage(`[]`)},
		Turn:      refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1},
		TwoHanded: true,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "off-hand")
}

func TestServiceAttack_LoadingWeaponLimitsAttacks(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(10, 16, 2, "heavy-crossbow")
	char.ID = charID
	char.Inventory = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`[{"name":"Bolts","quantity":20,"type":"ammunition"}]`),
		Valid:      true,
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeHeavyCrossbowFull(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 6
	})

	// Fighter with 2 attacks, but loading weapon should cap to 1
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 2}

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin", PositionCol: "B", PositionRow: 1, Ac: 12, Conditions: json.RawMessage(`[]`)},
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	// After loading limit (capped to 1) and using 1, should have 0 remaining
	assert.Equal(t, int32(0), result.RemainingTurn.AttacksRemaining)
}

func TestServiceAttack_LoadingWeaponCrossbowExpertOverride(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(10, 16, 2, "heavy-crossbow")
	char.ID = charID
	char.Features = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`[{"name":"Crossbow Expert","mechanical_effect":"crossbow-expert"}]`),
		Valid:      true,
	}
	char.Inventory = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`[{"name":"Bolts","quantity":20,"type":"ammunition"}]`),
		Valid:      true,
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeHeavyCrossbowFull(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 6
	})

	// Fighter with 2 attacks, Crossbow Expert: loading limit should NOT apply
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 2}

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin", PositionCol: "B", PositionRow: 1, Ac: 12, Conditions: json.RawMessage(`[]`)},
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	// Crossbow Expert means no loading limit: 2 - 1 = 1 remaining
	assert.Equal(t, int32(1), result.RemainingTurn.AttacksRemaining)
}

func TestServiceAttack_AmmunitionDeduction(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(10, 16, 2, "longbow")
	char.ID = charID
	char.Inventory = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`[{"name":"Arrows","quantity":18,"type":"ammunition"}]`),
		Valid:      true,
	}

	var savedInventory pqtype.NullRawMessage
	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		bow := makeLongbow()
		bow.Properties = []string{"ammunition"}
		return bow, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.updateCharacterInventoryFn = func(ctx context.Context, id uuid.UUID, inv pqtype.NullRawMessage) error {
		savedInventory = inv
		return nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 6
	})

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin", PositionCol: "C", PositionRow: 1, Ac: 12, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)

	// Verify inventory was updated
	require.True(t, savedInventory.Valid)
	var items []InventoryItem
	err = json.Unmarshal(savedInventory.RawMessage, &items)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, 17, items[0].Quantity) // 18 - 1 = 17
}

func TestServiceAttack_AmmunitionEmpty(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(10, 16, 2, "longbow")
	char.ID = charID
	char.Inventory = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`[{"name":"Arrows","quantity":0,"type":"ammunition"}]`),
		Valid:      true,
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		bow := makeLongbow()
		bow.Properties = []string{"ammunition"}
		return bow, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin", PositionCol: "C", PositionRow: 1, Ac: 12, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "No arrows remaining")
}

func TestServiceAttack_ImprovisedService(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 10, 2)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 3
	})

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker:     refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, Conditions: json.RawMessage(`[]`)},
		Target:       refdata.Combatant{ID: targetID, DisplayName: "Goblin", PositionCol: "B", PositionRow: 1, Ac: 12, Conditions: json.RawMessage(`[]`)},
		Turn:         refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1},
		IsImprovised: true,
	}, roller)
	require.NoError(t, err)
	assert.Equal(t, "Improvised Weapon", result.WeaponName)
	assert.Equal(t, "bludgeoning", result.DamageType)
}

func TestServiceAttack_ThrownWeapon(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 10, 2, "javelin")
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeJavelin(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 4
	})

	// Throw javelin at 20ft (within normal range 30ft)
	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin", PositionCol: "E", PositionRow: 1, Ac: 12, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1},
		Thrown:    true,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "Javelin", result.WeaponName)
}

func TestServiceAttack_AmmunitionUpdateInventoryError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(10, 16, 2, "longbow")
	char.ID = charID
	char.Inventory = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`[{"name":"Arrows","quantity":18,"type":"ammunition"}]`),
		Valid:      true,
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		bow := makeLongbow()
		bow.Properties = []string{"ammunition"}
		return bow, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.updateCharacterInventoryFn = func(ctx context.Context, id uuid.UUID, inv pqtype.NullRawMessage) error {
		return fmt.Errorf("db error")
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin", PositionCol: "C", PositionRow: 1, Ac: 12, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating inventory")
}

func TestServiceAttack_AmmunitionInvalidInventoryJSON(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(10, 16, 2, "longbow")
	char.ID = charID
	char.Inventory = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`invalid`),
		Valid:      true,
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		bow := makeLongbow()
		bow.Properties = []string{"ammunition"}
		return bow, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin", PositionCol: "C", PositionRow: 1, Ac: 12, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing inventory")
}

func TestServiceAttack_ImprovisedGetCharacterError(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("char not found")
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.Attack(ctx, AttackCommand{
		Attacker:     refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, Conditions: json.RawMessage(`[]`)},
		Target:       refdata.Combatant{ID: targetID, DisplayName: "Goblin", PositionCol: "B", PositionRow: 1, Ac: 12, Conditions: json.RawMessage(`[]`)},
		Turn:         refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1},
		IsImprovised: true,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

func TestServiceAttack_ImprovisedBadAbilityScores(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := refdata.Character{
		ID:               charID,
		AbilityScores:    json.RawMessage(`invalid`),
		ProficiencyBonus: 2,
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.Attack(ctx, AttackCommand{
		Attacker:     refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, Conditions: json.RawMessage(`[]`)},
		Target:       refdata.Combatant{ID: targetID, DisplayName: "Goblin", PositionCol: "B", PositionRow: 1, Ac: 12, Conditions: json.RawMessage(`[]`)},
		Turn:         refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1},
		IsImprovised: true,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing ability scores")
}

// --- HasFightingStyle Tests ---

func TestHasFightingStyle(t *testing.T) {
	tests := []struct {
		name     string
		features pqtype.NullRawMessage
		style    string
		expected bool
	}{
		{
			name: "TWF style present",
			features: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`[{"name":"Two-Weapon Fighting","mechanical_effect":"two_weapon_fighting"}]`),
				Valid:      true,
			},
			style:    "two_weapon_fighting",
			expected: true,
		},
		{
			name:     "no features (null)",
			features: pqtype.NullRawMessage{Valid: false},
			style:    "two_weapon_fighting",
			expected: false,
		},
		{
			name: "other features but not TWF",
			features: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`[{"name":"Defense","mechanical_effect":"defense"}]`),
				Valid:      true,
			},
			style:    "two_weapon_fighting",
			expected: false,
		},
		{
			name: "invalid JSON",
			features: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`invalid`),
				Valid:      true,
			},
			style:    "two_weapon_fighting",
			expected: false,
		},
		{
			name: "empty array",
			features: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`[]`),
				Valid:      true,
			},
			style:    "two_weapon_fighting",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasFightingStyle(tt.features, tt.style)
			assert.Equal(t, tt.expected, got)
		})
	}
}
