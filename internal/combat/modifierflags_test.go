package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Test helpers for Phase 38 ---

func makeGreataxe() refdata.Weapon {
	return refdata.Weapon{
		ID:         "greataxe",
		Name:       "Greataxe",
		Damage:     "1d12",
		DamageType: "slashing",
		WeaponType: "martial_melee",
		Properties: []string{"heavy", "two-handed"},
	}
}

// --- TDD Cycle 1: GWM applies -5 hit, +10 damage on heavy melee ---

// --- TDD Cycle 2: GWM rejects non-heavy weapon ---

func TestResolveAttack_GWM_NonHeavyMelee_Error(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 10 })

	input := AttackInput{
		AttackerName: "Grog",
		TargetName:   "Ogre",
		TargetAC:     15,
		Weapon:       makeLongsword(), // not heavy
		Scores:       AbilityScores{Str: 18, Dex: 10},
		ProfBonus:    3,
		DistanceFt:   5,
		GWM:          true,
	}

	_, err := ResolveAttack(input, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "heavy melee weapon")
}

func TestResolveAttack_GWM_RangedWeapon_Error(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 10 })

	input := AttackInput{
		AttackerName: "Grog",
		TargetName:   "Ogre",
		TargetAC:     15,
		Weapon:       makeHeavyCrossbow(), // heavy but ranged
		Scores:       AbilityScores{Str: 18, Dex: 10},
		ProfBonus:    3,
		DistanceFt:   5,
		GWM:          true,
	}

	_, err := ResolveAttack(input, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "heavy melee weapon")
}

// --- TDD Cycle 3: Sharpshooter applies -5 hit, +10 damage on ranged ---

func TestResolveAttack_Sharpshooter_Ranged_AppliesModifiers(t *testing.T) {
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18
		}
		return 6
	})

	input := AttackInput{
		AttackerName: "Legolas",
		TargetName:   "Orc",
		TargetAC:     14,
		Weapon:       makeLongbow(),
		Scores:       AbilityScores{Str: 10, Dex: 18},
		ProfBonus:    3,
		DistanceFt:   30,
		Sharpshooter: true,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	// Attack modifier: DEX(4) + prof(3) - 5 (SS) = 2
	// Roll: 18 + 2 = 20 vs AC 14 => hit
	assert.True(t, result.Hit)
	// Damage: 6 (d8) + 4 (DEX) + 10 (SS) = 20
	assert.Equal(t, 20, result.DamageTotal)
	assert.True(t, result.Sharpshooter)
}

func TestResolveAttack_Sharpshooter_MeleeWeapon_Error(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 10 })

	input := AttackInput{
		AttackerName: "Legolas",
		TargetName:   "Orc",
		TargetAC:     14,
		Weapon:       makeLongsword(),
		Scores:       AbilityScores{Str: 16, Dex: 14},
		ProfBonus:    3,
		DistanceFt:   5,
		Sharpshooter: true,
	}

	_, err := ResolveAttack(input, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ranged weapon")
}

// --- TDD Cycle 4: Reckless Attack grants advantage on melee STR attack ---

func TestResolveAttack_Reckless_MeleeSTR_Advantage(t *testing.T) {
	callIdx := 0
	roller := dice.NewRoller(func(max int) int {
		callIdx++
		if max == 20 {
			if callIdx == 1 {
				return 8
			}
			return 16 // advantage picks this
		}
		return 6
	})

	input := AttackInput{
		AttackerName: "Grog",
		TargetName:   "Ogre",
		TargetAC:     15,
		Weapon:       makeGreataxe(),
		Scores:       AbilityScores{Str: 18, Dex: 10},
		ProfBonus:    3,
		DistanceFt:   5,
		Reckless:     true,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.Equal(t, dice.Advantage, result.RollMode)
	assert.Contains(t, result.AdvantageReasons, "Reckless Attack")
	assert.True(t, result.Reckless)
}

func TestResolveAttack_Reckless_RangedWeapon_Error(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 10 })

	input := AttackInput{
		AttackerName: "Grog",
		TargetName:   "Ogre",
		TargetAC:     15,
		Weapon:       makeLongbow(),
		Scores:       AbilityScores{Str: 10, Dex: 18},
		ProfBonus:    3,
		DistanceFt:   30,
		Reckless:     true,
	}

	_, err := ResolveAttack(input, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "melee weapon")
}

func TestResolveAttack_Reckless_FinesseUsingDEX_Error(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 10 })

	input := AttackInput{
		AttackerName: "Grog",
		TargetName:   "Ogre",
		TargetAC:     15,
		Weapon:       makeRapier(), // finesse
		Scores:       AbilityScores{Str: 10, Dex: 18}, // DEX > STR => uses DEX
		ProfBonus:    3,
		DistanceFt:   5,
		Reckless:     true,
	}

	_, err := ResolveAttack(input, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "STR-based")
}

func TestResolveAttack_Reckless_FinesseUsingSTR_OK(t *testing.T) {
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 6
	})

	input := AttackInput{
		AttackerName: "Grog",
		TargetName:   "Ogre",
		TargetAC:     15,
		Weapon:       makeRapier(),                     // finesse
		Scores:       AbilityScores{Str: 18, Dex: 10}, // STR > DEX => uses STR
		ProfBonus:    3,
		DistanceFt:   5,
		Reckless:     true,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.Equal(t, dice.Advantage, result.RollMode)
	assert.Contains(t, result.AdvantageReasons, "Reckless Attack")
}

// --- TDD Cycle 1: GWM applies -5 hit, +10 damage on heavy melee ---

func TestResolveAttack_GWM_HeavyMelee_AppliesModifiers(t *testing.T) {
	// d20 rolls 18, damage rolls 8
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18
		}
		return 8
	})

	input := AttackInput{
		AttackerName: "Grog",
		TargetName:   "Ogre",
		TargetAC:     15,
		Weapon:       makeGreataxe(), // heavy melee
		Scores:       AbilityScores{Str: 18, Dex: 10},
		ProfBonus:    3,
		DistanceFt:   5,
		GWM:          true,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)

	// Attack modifier: STR(4) + prof(3) - 5 (GWM) = 2
	// Roll: 18 + 2 = 20 vs AC 15 => hit
	assert.True(t, result.Hit)
	// Damage: 8 (d12) + 4 (STR) + 10 (GWM) = 22
	assert.Equal(t, 22, result.DamageTotal)
	assert.True(t, result.GWM)
}

// --- Edge cases: GWM -5 causing a miss ---

func TestResolveAttack_GWM_MinusFiveCausesMiss(t *testing.T) {
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 12
		}
		return 6
	})

	input := AttackInput{
		AttackerName: "Grog",
		TargetName:   "Ogre",
		TargetAC:     15,
		Weapon:       makeGreataxe(),
		Scores:       AbilityScores{Str: 18, Dex: 10},
		ProfBonus:    3,
		DistanceFt:   5,
		GWM:          true,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	// Attack modifier: STR(4) + prof(3) - 5 (GWM) = 2
	// Roll: 12 + 2 = 14 vs AC 15 => miss
	assert.False(t, result.Hit)
	assert.Equal(t, 0, result.DamageTotal) // no damage on miss
}

// --- GWM + Reckless combo ---

func TestResolveAttack_GWM_And_Reckless(t *testing.T) {
	callIdx := 0
	roller := dice.NewRoller(func(max int) int {
		callIdx++
		if max == 20 {
			if callIdx == 1 {
				return 8
			}
			return 18 // advantage picks higher
		}
		return 10
	})

	input := AttackInput{
		AttackerName: "Grog",
		TargetName:   "Ogre",
		TargetAC:     15,
		Weapon:       makeGreataxe(),
		Scores:       AbilityScores{Str: 18, Dex: 10},
		ProfBonus:    3,
		DistanceFt:   5,
		GWM:          true,
		Reckless:     true,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	// Has advantage from reckless
	assert.Equal(t, dice.Advantage, result.RollMode)
	assert.Contains(t, result.AdvantageReasons, "Reckless Attack")
	// GWM -5/+10 applied
	assert.True(t, result.GWM)
	assert.True(t, result.Reckless)
	// Roll: 18 + 2 (STR4+prof3-5) = 20 vs AC 15 => hit
	assert.True(t, result.Hit)
	// Damage: 10 (d12) + 4 (STR) + 10 (GWM) = 24
	assert.Equal(t, 24, result.DamageTotal)
}

// --- HasBarbarianClass edge cases ---

func TestHasBarbarianClass(t *testing.T) {
	tests := []struct {
		name     string
		classes  json.RawMessage
		expected bool
	}{
		{"nil classes", nil, false},
		{"empty JSON", json.RawMessage(`[]`), false},
		{"invalid JSON", json.RawMessage(`not json`), false},
		{"barbarian present", json.RawMessage(`[{"class":"Barbarian","level":5}]`), true},
		{"barbarian case insensitive", json.RawMessage(`[{"class":"barbarian","level":3}]`), true},
		{"no barbarian", json.RawMessage(`[{"class":"Fighter","level":5}]`), false},
		{"multiclass with barbarian", json.RawMessage(`[{"class":"Fighter","level":3},{"class":"Barbarian","level":2}]`), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, HasBarbarianClass(tt.classes))
		})
	}
}

// --- TDD Cycle 5: Service.Attack validates feat/class prerequisites ---

func makeCharacterWithFeats(str, dex int, profBonus int32, equippedWeapon string, feats []CharacterFeature, classes []CharacterClass) refdata.Character {
	char := makeCharacter(str, dex, profBonus, equippedWeapon)
	if feats != nil {
		featsJSON, _ := json.Marshal(feats)
		char.Features = pqtype.NullRawMessage{RawMessage: featsJSON, Valid: true}
	}
	if classes != nil {
		classesJSON, _ := json.Marshal(classes)
		char.Classes = classesJSON
	}
	return char
}

func TestServiceAttack_GWM_RequiresFeat(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	// Character without GWM feat
	char := makeCharacterWithFeats(18, 10, 3, "greataxe", nil, nil)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeGreataxe(), nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Grog", PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Ogre", PositionCol: "B", PositionRow: 1, Ac: 15, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 2},
		GWM:      true,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Great Weapon Master")
	assert.Contains(t, err.Error(), "feat")
}

func TestServiceAttack_GWM_WithFeat_OK(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacterWithFeats(18, 10, 3, "greataxe",
		[]CharacterFeature{{Name: "Great Weapon Master", MechanicalEffect: "great-weapon-master"}},
		nil,
	)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeGreataxe(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18
		}
		return 8
	})

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Grog", PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Ogre", PositionCol: "B", PositionRow: 1, Ac: 15, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 2},
		GWM:      true,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.GWM)
	assert.True(t, result.Hit)
}

func TestServiceAttack_Sharpshooter_RequiresFeat(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacterWithFeats(10, 18, 3, "longbow", nil, nil)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeLongbow(), nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Legolas", PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Orc", PositionCol: "B", PositionRow: 1, Ac: 14, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
		Sharpshooter: true,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Sharpshooter")
	assert.Contains(t, err.Error(), "feat")
}

func TestServiceAttack_Reckless_RequiresBarbarian(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	// Fighter, not Barbarian
	char := makeCharacterWithFeats(18, 10, 3, "greataxe", nil,
		[]CharacterClass{{Class: "Fighter", Level: 5}},
	)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeGreataxe(), nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Grog", PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Ogre", PositionCol: "B", PositionRow: 1, Ac: 15, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 2},
		Reckless: true,
	}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Barbarian")
}

// --- TDD Cycle 6: FormatAttackLog notes active modifier flags ---

func TestFormatAttackLog_GWM(t *testing.T) {
	result := AttackResult{
		AttackerName: "Grog",
		TargetName:   "Ogre",
		WeaponName:   "Greataxe",
		DistanceFt:   5,
		IsMelee:      true,
		Hit:          true,
		D20Roll:      dice.D20Result{Chosen: 15, Total: 12, Modifier: -3},
		DamageTotal:  22,
		DamageType:   "slashing",
		DamageDice:   "1d12+4",
		GWM:          true,
	}
	log := FormatAttackLog(result)
	assert.Contains(t, log, "GWM")
}

func TestFormatAttackLog_Sharpshooter(t *testing.T) {
	result := AttackResult{
		AttackerName: "Legolas",
		TargetName:   "Orc",
		WeaponName:   "Longbow",
		DistanceFt:   30,
		IsMelee:      false,
		Hit:          true,
		D20Roll:      dice.D20Result{Chosen: 18, Total: 15, Modifier: -3},
		DamageTotal:  20,
		DamageType:   "piercing",
		DamageDice:   "1d8+4",
		Sharpshooter: true,
	}
	log := FormatAttackLog(result)
	assert.Contains(t, log, "Sharpshooter")
}

func TestFormatAttackLog_Reckless(t *testing.T) {
	result := AttackResult{
		AttackerName:     "Grog",
		TargetName:       "Ogre",
		WeaponName:       "Greataxe",
		DistanceFt:       5,
		IsMelee:          true,
		Hit:              true,
		D20Roll:          dice.D20Result{Chosen: 16, Total: 23, Modifier: 7},
		DamageTotal:      12,
		DamageType:       "slashing",
		DamageDice:       "1d12+4",
		Reckless:         true,
		RollMode:         dice.Advantage,
		AdvantageReasons: []string{"Reckless Attack"},
	}
	log := FormatAttackLog(result)
	assert.Contains(t, log, "Reckless")
}

func TestServiceAttack_Reckless_BarbarianClass_OK(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacterWithFeats(18, 10, 3, "greataxe", nil,
		[]CharacterClass{{Class: "Barbarian", Level: 5}},
	)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeGreataxe(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	callIdx := 0
	roller := dice.NewRoller(func(max int) int {
		callIdx++
		if max == 20 {
			if callIdx == 1 {
				return 8
			}
			return 16
		}
		return 6
	})

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Grog", PositionCol: "A", PositionRow: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Ogre", PositionCol: "B", PositionRow: 1, Ac: 15, IsAlive: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 2},
		Reckless: true,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Reckless)
	assert.Equal(t, dice.Advantage, result.RollMode)
	assert.Contains(t, result.AdvantageReasons, "Reckless Attack")
}
