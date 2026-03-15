package save

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
)

func fixedRoller(val int) *dice.Roller {
	return dice.NewRoller(func(max int) int { return val })
}

func TestSave_AutoFailParalyzedDEX(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.Save(SaveInput{
		Scores:  character.AbilityScores{DEX: 14},
		Ability: "dex",
		Conditions: []combat.CombatCondition{
			{Condition: "paralyzed"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AutoFail {
		t.Error("expected auto-fail for paralyzed DEX save")
	}
	if result.Total != 0 {
		t.Errorf("expected total 0 for auto-fail, got %d", result.Total)
	}
	if len(result.ConditionReasons) == 0 {
		t.Error("expected condition reasons")
	}
}

func TestSave_AutoFailStunnedSTR(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.Save(SaveInput{
		Scores:  character.AbilityScores{STR: 10},
		Ability: "str",
		Conditions: []combat.CombatCondition{
			{Condition: "stunned"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AutoFail {
		t.Error("expected auto-fail for stunned STR save")
	}
}

func TestSave_AutoFailUnconsciousDEX(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.Save(SaveInput{
		Scores:  character.AbilityScores{DEX: 10},
		Ability: "dex",
		Conditions: []combat.CombatCondition{
			{Condition: "unconscious"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AutoFail {
		t.Error("expected auto-fail for unconscious DEX save")
	}
}

func TestSave_AutoFailPetrifiedSTR(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.Save(SaveInput{
		Scores:  character.AbilityScores{STR: 10},
		Ability: "str",
		Conditions: []combat.CombatCondition{
			{Condition: "petrified"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AutoFail {
		t.Error("expected auto-fail for petrified STR save")
	}
}

func TestSave_NoAutoFailParalyzedWIS(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.Save(SaveInput{
		Scores:  character.AbilityScores{WIS: 10},
		Ability: "wis",
		Conditions: []combat.CombatCondition{
			{Condition: "paralyzed"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AutoFail {
		t.Error("paralyzed should NOT auto-fail WIS save")
	}
}

func TestSave_RestrainedDisadvDEX(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.Save(SaveInput{
		Scores:  character.AbilityScores{DEX: 14},
		Ability: "dex",
		Conditions: []combat.CombatCondition{
			{Condition: "restrained"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.D20Result.Mode != dice.Disadvantage {
		t.Errorf("expected disadvantage, got %v", result.D20Result.Mode)
	}
	if len(result.ConditionReasons) == 0 {
		t.Error("expected condition reasons")
	}
}

func TestSave_DodgeAdvDEX(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.Save(SaveInput{
		Scores:  character.AbilityScores{DEX: 14},
		Ability: "dex",
		Conditions: []combat.CombatCondition{
			{Condition: "dodge"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.D20Result.Mode != dice.Advantage {
		t.Errorf("expected advantage, got %v", result.D20Result.Mode)
	}
}

func TestSave_ExhaustionLevel3Disadv(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.Save(SaveInput{
		Scores:          character.AbilityScores{WIS: 10},
		Ability:         "wis",
		ExhaustionLevel: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.D20Result.Mode != dice.Disadvantage {
		t.Errorf("expected disadvantage from exhaustion 3+, got %v", result.D20Result.Mode)
	}
}

func TestSave_ExhaustionLevel2NoEffect(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.Save(SaveInput{
		Scores:          character.AbilityScores{WIS: 10},
		Ability:         "wis",
		ExhaustionLevel: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.D20Result.Mode != dice.Normal {
		t.Errorf("expected normal for exhaustion 2, got %v", result.D20Result.Mode)
	}
}

func TestSave_FeatureEffectAuraOfProtection(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.Save(SaveInput{
		Scores:          character.AbilityScores{DEX: 14}, // +2 mod
		Ability:         "dex",
		ProficientSaves: []string{},
		ProfBonus:       3,
		FeatureEffects: []combat.FeatureDefinition{
			{
				Name: "Aura of Protection",
				Effects: []combat.Effect{
					{
						Type:     combat.EffectModifySave,
						Trigger:  combat.TriggerOnSave,
						Modifier: 3, // CHA mod
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// d20=10, DEX mod=2, no prof, aura=3 => total=15
	if result.Total != 15 {
		t.Errorf("expected total 15, got %d", result.Total)
	}
	if result.FeatureBonus != 3 {
		t.Errorf("expected feature bonus 3, got %d", result.FeatureBonus)
	}
	if len(result.FeatureReasons) == 0 {
		t.Error("expected feature reasons")
	}
}

func TestSave_PlayerAdvPlusConditionDisadvCancel(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.Save(SaveInput{
		Scores:   character.AbilityScores{DEX: 10},
		Ability:  "dex",
		RollMode: dice.Advantage,
		Conditions: []combat.CombatCondition{
			{Condition: "restrained"}, // disadvantage on DEX
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Advantage (player) + Disadvantage (condition) => cancel
	if result.D20Result.Mode != dice.Normal {
		t.Errorf("expected normal (cancelled), got %v", result.D20Result.Mode)
	}
}

func TestSave_CaseInsensitiveAbility(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.Save(SaveInput{
		Scores:  character.AbilityScores{STR: 16},
		Ability: "STR",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ability != "str" {
		t.Errorf("expected ability str, got %s", result.Ability)
	}
}

func TestSave_EmptyFeatureEffects(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.Save(SaveInput{
		Scores:         character.AbilityScores{WIS: 10},
		Ability:        "wis",
		FeatureEffects: nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FeatureBonus != 0 {
		t.Errorf("expected no feature bonus, got %d", result.FeatureBonus)
	}
}

func TestSave_InvalidAbility(t *testing.T) {
	svc := NewService(fixedRoller(10))

	_, err := svc.Save(SaveInput{
		Scores:  character.AbilityScores{},
		Ability: "luck",
	})
	if err == nil {
		t.Error("expected error for invalid ability")
	}
}

func TestSave_AllSixAbilities(t *testing.T) {
	svc := NewService(fixedRoller(10))
	abilities := []string{"str", "dex", "con", "int", "wis", "cha"}
	for _, ab := range abilities {
		result, err := svc.Save(SaveInput{
			Scores:  character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
			Ability: ab,
		})
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", ab, err)
		}
		if result.Ability != ab {
			t.Errorf("expected ability %s, got %s", ab, result.Ability)
		}
	}
}

func TestCombineRollModes(t *testing.T) {
	tests := []struct {
		name      string
		requested dice.RollMode
		condition dice.RollMode
		expected  dice.RollMode
	}{
		{"normal+normal", dice.Normal, dice.Normal, dice.Normal},
		{"adv+normal", dice.Advantage, dice.Normal, dice.Advantage},
		{"normal+disadv", dice.Normal, dice.Disadvantage, dice.Disadvantage},
		{"adv+disadv", dice.Advantage, dice.Disadvantage, dice.AdvantageAndDisadvantage},
		{"disadv+disadv", dice.Disadvantage, dice.Disadvantage, dice.Disadvantage},
		{"adv+adv", dice.Advantage, dice.Advantage, dice.Advantage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dice.CombineRollModes(tc.requested, tc.condition)
			if got != tc.expected {
				t.Errorf("CombineRollModes(%v, %v) = %v, want %v", tc.requested, tc.condition, got, tc.expected)
			}
		})
	}
}

func TestSave_FeatureEffectWithCondition(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.Save(SaveInput{
		Scores:          character.AbilityScores{WIS: 14}, // +2 mod
		Ability:         "wis",
		ProficientSaves: []string{"wis"},
		ProfBonus:       3,
		FeatureEffects: []combat.FeatureDefinition{
			{
				Name: "Aura of Protection",
				Effects: []combat.Effect{
					{
						Type:     combat.EffectModifySave,
						Trigger:  combat.TriggerOnSave,
						Modifier: 4,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// d20=10, WIS mod=2, prof=3, aura=4 => total=19
	if result.Total != 19 {
		t.Errorf("expected total 19, got %d", result.Total)
	}
	if result.Modifier != 5 {
		t.Errorf("expected modifier 5, got %d", result.Modifier)
	}
	if result.FeatureBonus != 4 {
		t.Errorf("expected feature bonus 4, got %d", result.FeatureBonus)
	}
}

func TestSave_WithProficiency(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.Save(SaveInput{
		Scores:          character.AbilityScores{CON: 16}, // +3 mod
		Ability:         "con",
		ProficientSaves: []string{"con", "str"},
		ProfBonus:       2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// d20=10, CON mod=3, prof=2 => total=15
	if result.Total != 15 {
		t.Errorf("expected total 15, got %d", result.Total)
	}
	if result.Modifier != 5 {
		t.Errorf("expected modifier 5, got %d", result.Modifier)
	}
}

func TestSave_BasicNoProficiency(t *testing.T) {
	svc := NewService(fixedRoller(10))

	result, err := svc.Save(SaveInput{
		Scores:  character.AbilityScores{DEX: 14}, // +2 mod
		Ability: "dex",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// d20=10, DEX mod=2, no prof => total=12
	if result.Total != 12 {
		t.Errorf("expected total 12, got %d", result.Total)
	}
	if result.Modifier != 2 {
		t.Errorf("expected modifier 2, got %d", result.Modifier)
	}
	if result.Ability != "dex" {
		t.Errorf("expected ability dex, got %s", result.Ability)
	}
}
