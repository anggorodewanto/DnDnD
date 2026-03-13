package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// --- TDD Cycle 1: Resistance halves damage ---

func TestApplyDamageResistances_ResistanceHalvesDamage(t *testing.T) {
	dmg, reason := ApplyDamageResistances(10, "fire",
		[]string{"fire"}, nil, nil, nil)
	assert.Equal(t, 5, dmg)
	assert.Contains(t, reason, "resistance")
}

// --- TDD Cycle 2: Resistance rounds down ---

func TestApplyDamageResistances_ResistanceRoundsDown(t *testing.T) {
	dmg, _ := ApplyDamageResistances(7, "fire",
		[]string{"fire"}, nil, nil, nil)
	assert.Equal(t, 3, dmg) // 7/2 = 3.5 -> 3
}

// --- TDD Cycle 3: Immunity reduces to zero ---

func TestApplyDamageResistances_ImmunityReducesToZero(t *testing.T) {
	dmg, reason := ApplyDamageResistances(20, "poison",
		nil, []string{"poison"}, nil, nil)
	assert.Equal(t, 0, dmg)
	assert.Contains(t, reason, "immune")
}

// --- TDD Cycle 4: Vulnerability doubles damage ---

func TestApplyDamageResistances_VulnerabilityDoublesDamage(t *testing.T) {
	dmg, reason := ApplyDamageResistances(10, "fire",
		nil, nil, []string{"fire"}, nil)
	assert.Equal(t, 20, dmg)
	assert.Contains(t, reason, "vulnerable")
}

// --- TDD Cycle 5: Resistance + Vulnerability cancel ---

func TestApplyDamageResistances_ResistanceAndVulnerabilityCancel(t *testing.T) {
	dmg, reason := ApplyDamageResistances(10, "fire",
		[]string{"fire"}, nil, []string{"fire"}, nil)
	assert.Equal(t, 10, dmg)
	assert.Contains(t, reason, "cancel")
}

// --- TDD Cycle 6: Immunity trumps vulnerability ---

func TestApplyDamageResistances_ImmunityTrumpsVulnerability(t *testing.T) {
	dmg, reason := ApplyDamageResistances(10, "fire",
		nil, []string{"fire"}, []string{"fire"}, nil)
	assert.Equal(t, 0, dmg)
	assert.Contains(t, reason, "immune")
}

// --- TDD Cycle 7: Immunity trumps resistance ---

func TestApplyDamageResistances_ImmunityTrumpsResistance(t *testing.T) {
	dmg, _ := ApplyDamageResistances(10, "fire",
		[]string{"fire"}, []string{"fire"}, nil, nil)
	assert.Equal(t, 0, dmg)
}

// --- TDD Cycle 8: No modifiers - normal damage ---

func TestApplyDamageResistances_NoModifiers(t *testing.T) {
	dmg, reason := ApplyDamageResistances(10, "fire",
		nil, nil, nil, nil)
	assert.Equal(t, 10, dmg)
	assert.Empty(t, reason)
}

// --- TDD Cycle 9: Petrified grants resistance to all damage ---

func TestApplyDamageResistances_PetrifiedResistance(t *testing.T) {
	conds := []CombatCondition{{Condition: "petrified"}}
	dmg, reason := ApplyDamageResistances(10, "fire",
		nil, nil, nil, conds)
	assert.Equal(t, 5, dmg)
	assert.Contains(t, reason, "resistance")
}

// --- TDD Cycle 10: Petrified + existing resistance doesn't double-stack ---

func TestApplyDamageResistances_PetrifiedNoDoubleStack(t *testing.T) {
	conds := []CombatCondition{{Condition: "petrified"}}
	dmg, _ := ApplyDamageResistances(10, "fire",
		[]string{"fire"}, nil, nil, conds)
	// Both petrified and resistance -> still just halved, not quartered
	assert.Equal(t, 5, dmg)
}

// --- TDD Cycle 11: Petrified + vulnerability cancel ---

func TestApplyDamageResistances_PetrifiedAndVulnerabilityCancel(t *testing.T) {
	conds := []CombatCondition{{Condition: "petrified"}}
	dmg, reason := ApplyDamageResistances(10, "fire",
		nil, nil, []string{"fire"}, conds)
	assert.Equal(t, 10, dmg)
	assert.Contains(t, reason, "cancel")
}

// --- TDD Cycle 12: Case insensitive damage type matching ---

func TestApplyDamageResistances_CaseInsensitive(t *testing.T) {
	dmg, _ := ApplyDamageResistances(10, "Fire",
		[]string{"fire"}, nil, nil, nil)
	assert.Equal(t, 5, dmg)
}

// --- TDD Cycle 13: AbsorbTempHP - damage fully absorbed ---

func TestAbsorbTempHP_FullyAbsorbed(t *testing.T) {
	remaining, newTemp := AbsorbTempHP(5, 10)
	assert.Equal(t, 0, remaining)
	assert.Equal(t, 5, newTemp)
}

// --- TDD Cycle 14: AbsorbTempHP - damage exceeds temp HP ---

func TestAbsorbTempHP_Exceeds(t *testing.T) {
	remaining, newTemp := AbsorbTempHP(15, 10)
	assert.Equal(t, 5, remaining)
	assert.Equal(t, 0, newTemp)
}

// --- TDD Cycle 15: AbsorbTempHP - exact match ---

func TestAbsorbTempHP_ExactMatch(t *testing.T) {
	remaining, newTemp := AbsorbTempHP(10, 10)
	assert.Equal(t, 0, remaining)
	assert.Equal(t, 0, newTemp)
}

// --- TDD Cycle 16: AbsorbTempHP - no temp HP ---

func TestAbsorbTempHP_NoTempHP(t *testing.T) {
	remaining, newTemp := AbsorbTempHP(10, 0)
	assert.Equal(t, 10, remaining)
	assert.Equal(t, 0, newTemp)
}

// --- TDD Cycle 17: GrantTempHP - new is higher ---

func TestGrantTempHP_NewHigher(t *testing.T) {
	result := GrantTempHP(5, 10)
	assert.Equal(t, 10, result)
}

// --- TDD Cycle 18: GrantTempHP - current is higher ---

func TestGrantTempHP_CurrentHigher(t *testing.T) {
	result := GrantTempHP(10, 5)
	assert.Equal(t, 10, result)
}

// --- TDD Cycle 19: GrantTempHP - equal ---

func TestGrantTempHP_Equal(t *testing.T) {
	result := GrantTempHP(7, 7)
	assert.Equal(t, 7, result)
}

// --- TDD Cycle 20: GrantTempHP - from zero ---

func TestGrantTempHP_FromZero(t *testing.T) {
	result := GrantTempHP(0, 10)
	assert.Equal(t, 10, result)
}

// --- TDD Cycle 21: ExhaustionEffectiveSpeed - level 0 ---

func TestExhaustionEffectiveSpeed_Level0(t *testing.T) {
	assert.Equal(t, 30, ExhaustionEffectiveSpeed(30, 0))
}

// --- TDD Cycle 22: ExhaustionEffectiveSpeed - level 1 (no effect) ---

func TestExhaustionEffectiveSpeed_Level1(t *testing.T) {
	assert.Equal(t, 30, ExhaustionEffectiveSpeed(30, 1))
}

// --- TDD Cycle 23: ExhaustionEffectiveSpeed - level 2 halved ---

func TestExhaustionEffectiveSpeed_Level2(t *testing.T) {
	assert.Equal(t, 15, ExhaustionEffectiveSpeed(30, 2))
}

// --- TDD Cycle 24: ExhaustionEffectiveSpeed - level 3 still halved ---

func TestExhaustionEffectiveSpeed_Level3(t *testing.T) {
	assert.Equal(t, 15, ExhaustionEffectiveSpeed(30, 3))
}

// --- TDD Cycle 25: ExhaustionEffectiveSpeed - level 4 still halved ---

func TestExhaustionEffectiveSpeed_Level4(t *testing.T) {
	assert.Equal(t, 15, ExhaustionEffectiveSpeed(30, 4))
}

// --- TDD Cycle 26: ExhaustionEffectiveSpeed - level 5 zero ---

func TestExhaustionEffectiveSpeed_Level5(t *testing.T) {
	assert.Equal(t, 0, ExhaustionEffectiveSpeed(30, 5))
}

// --- TDD Cycle 27: ExhaustionEffectiveSpeed - level 6 zero ---

func TestExhaustionEffectiveSpeed_Level6(t *testing.T) {
	assert.Equal(t, 0, ExhaustionEffectiveSpeed(30, 6))
}

// --- TDD Cycle 28: ExhaustionEffectiveMaxHP - level 0 ---

func TestExhaustionEffectiveMaxHP_Level0(t *testing.T) {
	assert.Equal(t, 100, ExhaustionEffectiveMaxHP(100, 0))
}

// --- TDD Cycle 29: ExhaustionEffectiveMaxHP - level 3 no effect ---

func TestExhaustionEffectiveMaxHP_Level3(t *testing.T) {
	assert.Equal(t, 100, ExhaustionEffectiveMaxHP(100, 3))
}

// --- TDD Cycle 30: ExhaustionEffectiveMaxHP - level 4 halved ---

func TestExhaustionEffectiveMaxHP_Level4(t *testing.T) {
	assert.Equal(t, 50, ExhaustionEffectiveMaxHP(100, 4))
}

// --- TDD Cycle 31: ExhaustionEffectiveMaxHP - level 5 halved ---

func TestExhaustionEffectiveMaxHP_Level5(t *testing.T) {
	assert.Equal(t, 50, ExhaustionEffectiveMaxHP(100, 5))
}

// --- TDD Cycle 32: ExhaustionRollEffect - level 0 ---

func TestExhaustionRollEffect_Level0(t *testing.T) {
	check, attack, save := ExhaustionRollEffect(0)
	assert.False(t, check)
	assert.False(t, attack)
	assert.False(t, save)
}

// --- TDD Cycle 33: ExhaustionRollEffect - level 1 ---

func TestExhaustionRollEffect_Level1(t *testing.T) {
	check, attack, save := ExhaustionRollEffect(1)
	assert.True(t, check)
	assert.False(t, attack)
	assert.False(t, save)
}

// --- TDD Cycle 34: ExhaustionRollEffect - level 2 ---

func TestExhaustionRollEffect_Level2(t *testing.T) {
	check, attack, save := ExhaustionRollEffect(2)
	assert.True(t, check)
	assert.False(t, attack)
	assert.False(t, save)
}

// --- TDD Cycle 35: ExhaustionRollEffect - level 3 ---

func TestExhaustionRollEffect_Level3(t *testing.T) {
	check, attack, save := ExhaustionRollEffect(3)
	assert.True(t, check)
	assert.True(t, attack)
	assert.True(t, save)
}

// --- TDD Cycle 36: ExhaustionRollEffect - level 5 ---

func TestExhaustionRollEffect_Level5(t *testing.T) {
	check, attack, save := ExhaustionRollEffect(5)
	assert.True(t, check)
	assert.True(t, attack)
	assert.True(t, save)
}

// --- TDD Cycle 37: IsExhaustedToDeath ---

func TestIsExhaustedToDeath_Level6(t *testing.T) {
	assert.True(t, IsExhaustedToDeath(6))
}

func TestIsExhaustedToDeath_Level5(t *testing.T) {
	assert.False(t, IsExhaustedToDeath(5))
}

func TestIsExhaustedToDeath_Level0(t *testing.T) {
	assert.False(t, IsExhaustedToDeath(0))
}

// --- TDD Cycle 38: CheckConditionImmunity ---

func TestCheckConditionImmunity_Immune(t *testing.T) {
	assert.True(t, CheckConditionImmunity("frightened", []string{"frightened", "poisoned"}))
}

func TestCheckConditionImmunity_NotImmune(t *testing.T) {
	assert.False(t, CheckConditionImmunity("stunned", []string{"frightened", "poisoned"}))
}

func TestCheckConditionImmunity_EmptyImmunities(t *testing.T) {
	assert.False(t, CheckConditionImmunity("frightened", nil))
}

func TestCheckConditionImmunity_CaseInsensitive(t *testing.T) {
	assert.True(t, CheckConditionImmunity("Frightened", []string{"frightened"}))
}

// --- TDD Cycle 39: Immunity + Resistance + Vulnerability all present ---

func TestApplyDamageResistances_ImmunityTrumpsAll(t *testing.T) {
	dmg, reason := ApplyDamageResistances(10, "fire",
		[]string{"fire"}, []string{"fire"}, []string{"fire"}, nil)
	assert.Equal(t, 0, dmg)
	assert.Contains(t, reason, "immune")
}

// --- TDD Cycle 40: Zero damage input ---

func TestApplyDamageResistances_ZeroDamage(t *testing.T) {
	dmg, _ := ApplyDamageResistances(0, "fire",
		[]string{"fire"}, nil, nil, nil)
	assert.Equal(t, 0, dmg)
}

// --- TDD Cycle 41: Vulnerability with zero damage ---

func TestApplyDamageResistances_VulnerabilityZeroDamage(t *testing.T) {
	dmg, _ := ApplyDamageResistances(0, "fire",
		nil, nil, []string{"fire"}, nil)
	assert.Equal(t, 0, dmg)
}

// --- TDD Cycle 42: AbsorbTempHP - zero damage ---

func TestAbsorbTempHP_ZeroDamage(t *testing.T) {
	remaining, newTemp := AbsorbTempHP(0, 10)
	assert.Equal(t, 0, remaining)
	assert.Equal(t, 10, newTemp)
}

// --- TDD Cycle 43: Odd speed with exhaustion ---

func TestExhaustionEffectiveSpeed_OddSpeed(t *testing.T) {
	assert.Equal(t, 12, ExhaustionEffectiveSpeed(25, 2)) // 25/2 = 12
}

// --- TDD Cycle 44: Odd max HP with exhaustion ---

func TestExhaustionEffectiveMaxHP_OddMaxHP(t *testing.T) {
	assert.Equal(t, 50, ExhaustionEffectiveMaxHP(101, 4)) // 101/2 = 50
}

// --- TDD Cycle 45: Different damage types don't trigger resistance ---

func TestApplyDamageResistances_DifferentType(t *testing.T) {
	dmg, reason := ApplyDamageResistances(10, "cold",
		[]string{"fire"}, nil, nil, nil)
	assert.Equal(t, 10, dmg)
	assert.Empty(t, reason)
}

// --- TDD Cycle 46: Petrified with immunity - immunity wins ---

func TestApplyDamageResistances_PetrifiedWithImmunity(t *testing.T) {
	conds := []CombatCondition{{Condition: "petrified"}}
	dmg, reason := ApplyDamageResistances(10, "poison",
		nil, []string{"poison"}, nil, conds)
	assert.Equal(t, 0, dmg)
	assert.Contains(t, reason, "immune")
}

// --- TDD Cycle 47: EffectiveSpeedWithExhaustion integrates conditions and exhaustion ---

func TestEffectiveSpeedWithExhaustion_GrappledOverridesExhaustion(t *testing.T) {
	conds := []CombatCondition{{Condition: "grappled"}}
	assert.Equal(t, 0, EffectiveSpeedWithExhaustion(30, conds, 0))
}

func TestEffectiveSpeedWithExhaustion_ExhaustionLevel2Halves(t *testing.T) {
	assert.Equal(t, 15, EffectiveSpeedWithExhaustion(30, nil, 2))
}

func TestEffectiveSpeedWithExhaustion_ExhaustionLevel5Zero(t *testing.T) {
	assert.Equal(t, 0, EffectiveSpeedWithExhaustion(30, nil, 5))
}

func TestEffectiveSpeedWithExhaustion_NoEffects(t *testing.T) {
	assert.Equal(t, 30, EffectiveSpeedWithExhaustion(30, nil, 0))
}

// --- TDD Cycle 48: CheckSaveWithExhaustion integrates exhaustion level 3+ ---

func TestCheckSaveWithExhaustion_Level3DisadvOnSave(t *testing.T) {
	_, mode, reasons := CheckSaveWithExhaustion(nil, "wis", 3)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Contains(t, reasons, "exhaustion (level 3+): disadvantage on saving throws")
}

func TestCheckSaveWithExhaustion_Level2NoSaveEffect(t *testing.T) {
	_, mode, reasons := CheckSaveWithExhaustion(nil, "wis", 2)
	assert.Equal(t, dice.Normal, mode)
	assert.Empty(t, reasons)
}

func TestCheckSaveWithExhaustion_Level3PlusConditions(t *testing.T) {
	conds := []CombatCondition{{Condition: "restrained"}}
	// Restrained: disadv on DEX save. Exhaustion 3: disadv on saves.
	// Both disadvantage -> still disadvantage.
	_, mode, _ := CheckSaveWithExhaustion(conds, "dex", 3)
	assert.Equal(t, dice.Disadvantage, mode)
}

func TestCheckSaveWithExhaustion_AutoFailTrumpsExhaustion(t *testing.T) {
	conds := []CombatCondition{{Condition: "paralyzed"}}
	autoFail, _, _ := CheckSaveWithExhaustion(conds, "str", 3)
	assert.True(t, autoFail)
}

// --- TDD Cycle 49: CheckAbilityCheckWithExhaustion ---

func TestCheckAbilityCheckWithExhaustion_Level1Disadv(t *testing.T) {
	_, mode, reasons := CheckAbilityCheckWithExhaustion(nil, AbilityCheckContext{}, 1)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Contains(t, reasons, "exhaustion (level 1+): disadvantage on ability checks")
}

func TestCheckAbilityCheckWithExhaustion_Level0NoEffect(t *testing.T) {
	_, mode, _ := CheckAbilityCheckWithExhaustion(nil, AbilityCheckContext{}, 0)
	assert.Equal(t, dice.Normal, mode)
}

func TestCheckAbilityCheckWithExhaustion_Level1PlusPoisoned(t *testing.T) {
	conds := []CombatCondition{{Condition: "poisoned"}}
	_, mode, reasons := CheckAbilityCheckWithExhaustion(conds, AbilityCheckContext{}, 1)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Len(t, reasons, 2) // both poisoned and exhaustion
}

func TestCheckAbilityCheckWithExhaustion_AutoFailStillReturnsAutoFail(t *testing.T) {
	conds := []CombatCondition{{Condition: "blinded"}}
	autoFail, _, _ := CheckAbilityCheckWithExhaustion(conds, AbilityCheckContext{RequiresSight: true}, 3)
	assert.True(t, autoFail)
}

// --- TDD Cycle 51: applyDisadvantage with Advantage mode ---

func TestApplyDisadvantage_AdvantageBecomesCancel(t *testing.T) {
	// Test through CheckSaveWithExhaustion: dodge gives advantage on DEX save,
	// exhaustion 3 adds disadvantage -> cancel
	conds := []CombatCondition{{Condition: "dodge"}}
	_, mode, _ := CheckSaveWithExhaustion(conds, "dex", 3)
	assert.Equal(t, dice.AdvantageAndDisadvantage, mode)
}

// --- TDD Cycle 50: ApplyCondition with condition immunity ---

func TestApplyCondition_ConditionImmunityBlocks(t *testing.T) {
	combatantID := uuid.New()
	ms := defaultMockStore()
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:            combatantID,
			DisplayName:   "Iron Golem",
			Conditions:    json.RawMessage(`[]`),
			CreatureRefID: sql.NullString{String: "iron-golem", Valid: true},
		}, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:                  "iron-golem",
			ConditionImmunities: []string{"frightened", "poisoned", "charmed"},
		}, nil
	}

	svc := NewService(ms)
	cond := CombatCondition{Condition: "frightened", DurationRounds: 1}
	_, msgs, err := svc.ApplyCondition(context.Background(), combatantID, cond)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], "immune")
	assert.Contains(t, msgs[0], "frightened")
}

func TestApplyCondition_NoImmunityAppliesNormally(t *testing.T) {
	combatantID := uuid.New()
	ms := defaultMockStore()
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:            combatantID,
			DisplayName:   "Iron Golem",
			Conditions:    json.RawMessage(`[]`),
			CreatureRefID: sql.NullString{String: "iron-golem", Valid: true},
		}, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:                  "iron-golem",
			ConditionImmunities: []string{"frightened"},
		}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)
	cond := CombatCondition{Condition: "stunned", DurationRounds: 1}
	_, msgs, err := svc.ApplyCondition(context.Background(), combatantID, cond)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], "stunned")
	assert.Contains(t, msgs[0], "applied")
}

func TestApplyCondition_PCNoCreatureRefAppliesNormally(t *testing.T) {
	combatantID := uuid.New()
	ms := defaultMockStore()
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          combatantID,
			DisplayName: "Aragorn",
			Conditions:  json.RawMessage(`[]`),
			// No CreatureRefID for PCs
		}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)
	cond := CombatCondition{Condition: "stunned", DurationRounds: 1}
	_, msgs, err := svc.ApplyCondition(context.Background(), combatantID, cond)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], "stunned")
	assert.Contains(t, msgs[0], "applied")
}

