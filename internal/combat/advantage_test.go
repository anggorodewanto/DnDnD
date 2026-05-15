package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

func TestDetectAdvantage_AttackerConditions(t *testing.T) {
	tests := []struct {
		name          string
		condition     string
		expectMode    dice.RollMode
		expectAdvReason   string
		expectDisadvReason string
	}{
		{"blinded", "blinded", dice.Disadvantage, "", "attacker blinded"},
		{"invisible", "invisible", dice.Advantage, "attacker invisible", ""},
		{"poisoned", "poisoned", dice.Disadvantage, "", "attacker poisoned"},
		{"prone", "prone", dice.Disadvantage, "", "attacker prone"},
		{"restrained", "restrained", dice.Disadvantage, "", "attacker restrained"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := AdvantageInput{
				AttackerConditions: []CombatCondition{{Condition: tt.condition}},
			}
			mode, advReasons, disadvReasons := DetectAdvantage(input)
			assert.Equal(t, tt.expectMode, mode)
			if tt.expectAdvReason != "" {
				assert.Contains(t, advReasons, tt.expectAdvReason)
			} else {
				assert.Empty(t, advReasons)
			}
			if tt.expectDisadvReason != "" {
				assert.Contains(t, disadvReasons, tt.expectDisadvReason)
			} else {
				assert.Empty(t, disadvReasons)
			}
		})
	}
}

func TestDetectAdvantage_TargetConditions(t *testing.T) {
	tests := []struct {
		name          string
		condition     string
		distFt        int
		expectMode    dice.RollMode
		expectAdvReason   string
		expectDisadvReason string
	}{
		{"blinded target", "blinded", 5, dice.Advantage, "target blinded", ""},
		{"invisible target", "invisible", 5, dice.Disadvantage, "", "target invisible"},
		{"restrained target", "restrained", 5, dice.Advantage, "target restrained", ""},
		{"stunned target", "stunned", 5, dice.Advantage, "target stunned", ""},
		{"paralyzed target", "paralyzed", 5, dice.Advantage, "target paralyzed", ""},
		{"unconscious target", "unconscious", 5, dice.Advantage, "target unconscious", ""},
		{"petrified target", "petrified", 5, dice.Advantage, "target petrified", ""},
		{"prone target within 5ft", "prone", 5, dice.Advantage, "target prone within 5ft", ""},
		{"prone target beyond 5ft", "prone", 10, dice.Disadvantage, "", "target prone beyond 5ft"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := AdvantageInput{
				TargetConditions: []CombatCondition{{Condition: tt.condition}},
				DistanceFt:       tt.distFt,
			}
			mode, advReasons, disadvReasons := DetectAdvantage(input)
			assert.Equal(t, tt.expectMode, mode, "mode mismatch")
			if tt.expectAdvReason != "" {
				assert.Contains(t, advReasons, tt.expectAdvReason)
			}
			if tt.expectDisadvReason != "" {
				assert.Contains(t, disadvReasons, tt.expectDisadvReason)
			}
		})
	}
}

func TestDetectAdvantage_RangedWithHostileNearby(t *testing.T) {
	input := AdvantageInput{
		Weapon:              makeLongbow(),
		HostileNearAttacker: true,
	}
	mode, _, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Contains(t, disadvReasons, "hostile within 5ft")
}

func TestDetectAdvantage_MeleeWithHostileNearby_NoEffect(t *testing.T) {
	input := AdvantageInput{
		Weapon:              makeLongsword(),
		HostileNearAttacker: true,
	}
	mode, _, _ := DetectAdvantage(input)
	assert.Equal(t, dice.Normal, mode)
}

func TestDetectAdvantage_LongRange(t *testing.T) {
	input := AdvantageInput{
		Weapon:     makeLongbow(),
		DistanceFt: 200, // beyond normal 150
	}
	mode, _, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Contains(t, disadvReasons, "long range")
}

func TestDetectAdvantage_HeavyWeaponSmallCreature(t *testing.T) {
	input := AdvantageInput{
		Weapon:       makeHeavyCrossbow(),
		AttackerSize: "Small",
	}
	mode, _, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Contains(t, disadvReasons, "heavy weapon, Small creature")
}

func TestDetectAdvantage_HeavyWeaponTinyCreature(t *testing.T) {
	input := AdvantageInput{
		Weapon:       makeHeavyCrossbow(),
		AttackerSize: "Tiny",
	}
	mode, _, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Contains(t, disadvReasons, "heavy weapon, Tiny creature")
}

func TestDetectAdvantage_HeavyWeaponMediumCreature_NoEffect(t *testing.T) {
	input := AdvantageInput{
		Weapon:       makeHeavyCrossbow(),
		AttackerSize: "Medium",
	}
	mode, _, _ := DetectAdvantage(input)
	assert.Equal(t, dice.Normal, mode)
}

func TestDetectAdvantage_Cancellation(t *testing.T) {
	// Attacker invisible (adv) + target invisible (disadv) => cancel
	input := AdvantageInput{
		AttackerConditions: []CombatCondition{{Condition: "invisible"}},
		TargetConditions:   []CombatCondition{{Condition: "invisible"}},
	}
	mode, advReasons, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.AdvantageAndDisadvantage, mode)
	assert.NotEmpty(t, advReasons)
	assert.NotEmpty(t, disadvReasons)
}

func TestDetectAdvantage_MultipleCancellation(t *testing.T) {
	// Multiple advantage + multiple disadvantage still cancel
	input := AdvantageInput{
		AttackerConditions: []CombatCondition{{Condition: "invisible"}},
		TargetConditions:   []CombatCondition{{Condition: "blinded"}, {Condition: "restrained"}},
		Weapon:             makeLongbow(),
		HostileNearAttacker: true,
	}
	mode, advReasons, disadvReasons := DetectAdvantage(input)
	// 3 advantage sources (invisible, blinded target, restrained target) + 1 disadvantage (hostile nearby)
	assert.Equal(t, dice.AdvantageAndDisadvantage, mode)
	assert.Len(t, advReasons, 3)
	assert.Len(t, disadvReasons, 1)
}

func TestDetectAdvantage_NoConditions(t *testing.T) {
	input := AdvantageInput{}
	mode, advReasons, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.Normal, mode)
	assert.Empty(t, advReasons)
	assert.Empty(t, disadvReasons)
}

func TestDetectAdvantage_DMOverrideAdvantage(t *testing.T) {
	input := AdvantageInput{
		DMAdvantage: true,
	}
	mode, advReasons, _ := DetectAdvantage(input)
	assert.Equal(t, dice.Advantage, mode)
	assert.Contains(t, advReasons, "DM override")
}

func TestDetectAdvantage_DMOverrideDisadvantage(t *testing.T) {
	input := AdvantageInput{
		DMDisadvantage: true,
	}
	mode, _, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Contains(t, disadvReasons, "DM override")
}

func TestDetectAdvantage_DMBothOverrides(t *testing.T) {
	input := AdvantageInput{
		DMAdvantage:    true,
		DMDisadvantage: true,
	}
	mode, _, _ := DetectAdvantage(input)
	assert.Equal(t, dice.AdvantageAndDisadvantage, mode)
}

func TestResolveAttack_WithAdvantage(t *testing.T) {
	// Target is blinded => advantage. With advantage, roller picks higher of two d20s.
	callIdx := 0
	roller := dice.NewRoller(func(max int) int {
		callIdx++
		if max == 20 {
			if callIdx == 1 {
				return 8 // first d20
			}
			return 15 // second d20 (advantage picks this)
		}
		return 5 // damage
	})

	input := AttackInput{
		AttackerName:       "Aria",
		TargetName:         "Goblin #1",
		TargetAC:           13,
		Weapon:             makeLongsword(),
		Scores:             AbilityScores{Str: 16, Dex: 14},
		ProfBonus:          2,
		DistanceFt:         5,
		TargetConditions:   []CombatCondition{{Condition: "blinded"}},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.Equal(t, dice.Advantage, result.RollMode)
	assert.Contains(t, result.AdvantageReasons, "target blinded")
	assert.True(t, result.Hit)
}

func TestResolveAttack_WithDisadvantage(t *testing.T) {
	// Attacker is poisoned => disadvantage. With disadvantage, roller picks lower.
	callIdx := 0
	roller := dice.NewRoller(func(max int) int {
		callIdx++
		if max == 20 {
			if callIdx == 1 {
				return 15 // first d20
			}
			return 3 // second d20 (disadvantage picks this)
		}
		return 5
	})

	input := AttackInput{
		AttackerName:         "Aria",
		TargetName:           "Goblin #1",
		TargetAC:             13,
		Weapon:               makeLongsword(),
		Scores:               AbilityScores{Str: 16, Dex: 14},
		ProfBonus:            2,
		DistanceFt:           5,
		AttackerConditions:   []CombatCondition{{Condition: "poisoned"}},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.Equal(t, dice.Disadvantage, result.RollMode)
	assert.Contains(t, result.DisadvantageReasons, "attacker poisoned")
	// 3 + 3 + 2 = 8 vs AC 13 => miss
	assert.False(t, result.Hit)
}

func TestResolveAttack_AdvDisadvCancel(t *testing.T) {
	// Both advantage and disadvantage => normal roll
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 5
	})

	input := AttackInput{
		AttackerName:         "Aria",
		TargetName:           "Goblin #1",
		TargetAC:             13,
		Weapon:               makeLongsword(),
		Scores:               AbilityScores{Str: 16, Dex: 14},
		ProfBonus:            2,
		DistanceFt:           5,
		AttackerConditions:   []CombatCondition{{Condition: "poisoned"}},
		TargetConditions:     []CombatCondition{{Condition: "stunned"}},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.Equal(t, dice.AdvantageAndDisadvantage, result.RollMode)
	assert.NotEmpty(t, result.AdvantageReasons)
	assert.NotEmpty(t, result.DisadvantageReasons)
}

func TestFormatAttackLog_Advantage(t *testing.T) {
	result := AttackResult{
		AttackerName:     "Aria",
		TargetName:       "Goblin #1",
		WeaponName:       "Longsword",
		DistanceFt:       5,
		IsMelee:          true,
		Hit:              true,
		D20Roll:          dice.D20Result{Chosen: 15, Total: 20, Modifier: 5, Rolls: []int{8, 15}, Mode: dice.Advantage},
		DamageTotal:      10,
		DamageType:       "slashing",
		DamageDice:       "1d8+5",
		RollMode:         dice.Advantage,
		AdvantageReasons: []string{"target blinded"},
	}

	log := FormatAttackLog(result)
	assert.Contains(t, log, "(advantage")
	assert.Contains(t, log, "target blinded")
}

func TestFormatAttackLog_Disadvantage(t *testing.T) {
	result := AttackResult{
		AttackerName:        "Aria",
		TargetName:          "Goblin #1",
		WeaponName:          "Longbow",
		DistanceFt:          200,
		IsMelee:             false,
		Hit:                 false,
		D20Roll:             dice.D20Result{Chosen: 5, Total: 11, Modifier: 6, Rolls: []int{15, 5}, Mode: dice.Disadvantage},
		EffectiveAC:         13,
		RollMode:            dice.Disadvantage,
		DisadvantageReasons: []string{"hostile within 5ft"},
	}

	log := FormatAttackLog(result)
	assert.Contains(t, log, "(disadvantage")
	assert.Contains(t, log, "hostile within 5ft")
}

func TestFormatAttackLog_AdvDisadvCancel(t *testing.T) {
	result := AttackResult{
		AttackerName:        "Aria",
		TargetName:          "Goblin #1",
		WeaponName:          "Longsword",
		DistanceFt:          5,
		IsMelee:             true,
		Hit:                 true,
		D20Roll:             dice.D20Result{Chosen: 15, Total: 20, Modifier: 5},
		DamageTotal:         10,
		DamageType:          "slashing",
		DamageDice:          "1d8+5",
		RollMode:            dice.AdvantageAndDisadvantage,
		AdvantageReasons:    []string{"target stunned"},
		DisadvantageReasons: []string{"attacker poisoned"},
	}

	log := FormatAttackLog(result)
	assert.Contains(t, log, "advantage + disadvantage cancel")
	assert.Contains(t, log, "normal roll")
}

func TestServiceAttack_AdvantageFromTargetCondition(t *testing.T) {
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
	// With advantage, roller is called twice for d20: return 8, then 18
	callIdx := 0
	roller := dice.NewRoller(func(max int) int {
		callIdx++
		if max == 20 {
			if callIdx == 1 {
				return 8
			}
			return 18
		}
		return 5
	})

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin #1", PositionCol: "B", PositionRow: 1, Ac: 13, IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[{"condition":"stunned"}]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.NoError(t, err)
	assert.Equal(t, dice.Advantage, result.RollMode)
	assert.Contains(t, result.AdvantageReasons, "target stunned")
	assert.True(t, result.Hit)
}

func TestServiceAttack_DisadvantageFromAttackerCondition(t *testing.T) {
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
	// Disadvantage: two d20 rolls, take lower
	callIdx := 0
	roller := dice.NewRoller(func(max int) int {
		callIdx++
		if max == 20 {
			if callIdx == 1 {
				return 15
			}
			return 5 // takes this one (lower)
		}
		return 5
	})

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Aria", PositionCol: "A", PositionRow: 1, IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[{"condition":"poisoned"}]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Goblin #1", PositionCol: "B", PositionRow: 1, Ac: 13, IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.NoError(t, err)
	assert.Equal(t, dice.Disadvantage, result.RollMode)
	assert.Contains(t, result.DisadvantageReasons, "attacker poisoned")
}

func makeHeavyCrossbow() refdata.Weapon {
	return refdata.Weapon{
		ID:         "heavy-crossbow",
		Name:       "Heavy Crossbow",
		Damage:     "1d10",
		DamageType: "piercing",
		WeaponType: "martial_ranged",
		Properties: []string{"heavy", "loading", "two-handed"},
	}
}

// Phase 57: Hidden attacker/target advantage/disadvantage

func TestDetectAdvantage_AttackerHidden(t *testing.T) {
	input := AdvantageInput{
		AttackerHidden: true,
	}
	mode, advReasons, _ := DetectAdvantage(input)
	assert.Equal(t, dice.Advantage, mode)
	assert.Contains(t, advReasons, "attacker hidden")
}

func TestDetectAdvantage_TargetHidden(t *testing.T) {
	input := AdvantageInput{
		TargetHidden: true,
	}
	mode, _, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Contains(t, disadvReasons, "target hidden")
}

func TestDetectAdvantage_AttackerHidden_CancelsWithDisadvantage(t *testing.T) {
	input := AdvantageInput{
		AttackerHidden: true,
		AttackerConditions: []CombatCondition{{Condition: "poisoned"}},
	}
	mode, advReasons, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.AdvantageAndDisadvantage, mode)
	assert.Contains(t, advReasons, "attacker hidden")
	assert.Contains(t, disadvReasons, "attacker poisoned")
}

// SR-018: Help action advantage is target-scoped and single-shot. The
// help_advantage condition lives on the helped creature (the attacker here)
// and carries the named target's combatant ID. DetectAdvantage grants
// advantage only when the current attack targets that combatant.

func TestDetectAdvantage_HelpAdvantage_MatchingTarget(t *testing.T) {
	targetID := "goblin-uuid"
	input := AdvantageInput{
		AttackerConditions: []CombatCondition{{
			Condition:         "help_advantage",
			TargetCombatantID: targetID,
		}},
		TargetCombatantID: targetID,
	}
	mode, advReasons, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.Advantage, mode)
	assert.Contains(t, advReasons, "help advantage")
	assert.Empty(t, disadvReasons)
}

func TestDetectAdvantage_HelpAdvantage_DifferentTarget(t *testing.T) {
	input := AdvantageInput{
		AttackerConditions: []CombatCondition{{
			Condition:         "help_advantage",
			TargetCombatantID: "goblin-uuid",
		}},
		TargetCombatantID: "orc-uuid",
	}
	mode, advReasons, _ := DetectAdvantage(input)
	assert.Equal(t, dice.Normal, mode, "help advantage is target-scoped")
	assert.Empty(t, advReasons)
}

func TestDetectAdvantage_HelpAdvantage_EmptyTargetScope(t *testing.T) {
	// Defensive: a help_advantage with no target_combatant_id must not
	// grant universal advantage.
	input := AdvantageInput{
		AttackerConditions: []CombatCondition{{
			Condition:         "help_advantage",
			TargetCombatantID: "",
		}},
		TargetCombatantID: "goblin-uuid",
	}
	mode, advReasons, _ := DetectAdvantage(input)
	assert.Equal(t, dice.Normal, mode)
	assert.Empty(t, advReasons)
}

// C-C02: Reckless Attack advantage on attacks 2+ via attacker "reckless" condition.

func TestDetectAdvantage_RecklessCondition_MeleeSTR_Advantage(t *testing.T) {
	input := AdvantageInput{
		AttackerConditions: []CombatCondition{{Condition: "reckless"}},
		Weapon:             makeLongsword(),
		AbilityUsed:        "str",
	}
	mode, advReasons, _ := DetectAdvantage(input)
	assert.Equal(t, dice.Advantage, mode)
	assert.Contains(t, advReasons, "Reckless Attack (active)")
}

func TestDetectAdvantage_RecklessCondition_Ranged_NoAdvantage(t *testing.T) {
	input := AdvantageInput{
		AttackerConditions: []CombatCondition{{Condition: "reckless"}},
		Weapon:             makeLongbow(),
		AbilityUsed:        "dex",
	}
	mode, advReasons, _ := DetectAdvantage(input)
	assert.Equal(t, dice.Normal, mode)
	assert.Empty(t, advReasons)
}

func TestDetectAdvantage_RecklessCondition_MeleeDEX_NoAdvantage(t *testing.T) {
	input := AdvantageInput{
		AttackerConditions: []CombatCondition{{Condition: "reckless"}},
		Weapon:             makeLongsword(),
		AbilityUsed:        "dex",
	}
	mode, advReasons, _ := DetectAdvantage(input)
	assert.Equal(t, dice.Normal, mode)
	assert.Empty(t, advReasons)
}

// E-C03: Dodge condition imposes disadvantage on attacks against the dodging target.

func TestDetectAdvantage_TargetDodging_Disadvantage(t *testing.T) {
	input := AdvantageInput{
		TargetConditions: []CombatCondition{{Condition: "dodge"}},
		DistanceFt:       5,
	}
	mode, advReasons, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Empty(t, advReasons)
	assert.Contains(t, disadvReasons, "target dodging")
}