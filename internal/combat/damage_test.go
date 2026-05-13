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

// --- crit-03: Service.ApplyDamage wrapper integration tests ---

// applyDamageMockStore returns a mockStore wired for ApplyDamage tests.
// Captures applied HP / temp HP / isAlive on the combatant update path.
func applyDamageMockStore() (*mockStore, *refdata.UpdateCombatantHPParams) {
	captured := &refdata.UpdateCombatantHPParams{}
	ms := defaultMockStore()
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		*captured = arg
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, TempHp: arg.TempHp, IsAlive: arg.IsAlive, Conditions: json.RawMessage(`[]`)}, nil
	}
	return ms, captured
}

func TestApplyDamage_NPCResistanceHalvesIncomingDamage(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	ms, captured := applyDamageMockStore()
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "fire-elemental", DamageResistances: []string{"slashing"}}, nil
	}
	svc := NewService(ms)
	target := refdata.Combatant{
		ID: combatantID, EncounterID: encounterID,
		HpMax: 30, HpCurrent: 30, TempHp: 0, IsAlive: true, IsNpc: true,
		CreatureRefID: sql.NullString{String: "fire-elemental", Valid: true},
		Conditions:    json.RawMessage(`[]`),
	}

	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 10, DamageType: "slashing",
	})
	assert.NoError(t, err)
	// 10 / 2 = 5 final damage
	assert.Equal(t, 5, res.FinalDamage)
	assert.Contains(t, res.Reason, "resistance")
	assert.Equal(t, int32(25), captured.HpCurrent)
	assert.True(t, captured.IsAlive)
}

func TestApplyDamage_NPCImmunityZeroes(t *testing.T) {
	encounterID := uuid.New()
	ms, captured := applyDamageMockStore()
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "skeleton", DamageImmunities: []string{"poison"}}, nil
	}
	svc := NewService(ms)
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		HpMax: 20, HpCurrent: 20, IsAlive: true, IsNpc: true,
		CreatureRefID: sql.NullString{String: "skeleton", Valid: true},
		Conditions:    json.RawMessage(`[]`),
	}

	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 50, DamageType: "poison",
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, res.FinalDamage)
	assert.Contains(t, res.Reason, "immune")
	assert.Equal(t, int32(20), captured.HpCurrent, "immune NPC must take 0 damage")
}

func TestApplyDamage_NPCVulnerabilityDoubles(t *testing.T) {
	encounterID := uuid.New()
	ms, captured := applyDamageMockStore()
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "troll", DamageVulnerabilities: []string{"fire"}}, nil
	}
	svc := NewService(ms)
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		HpMax: 50, HpCurrent: 50, IsAlive: true, IsNpc: true,
		CreatureRefID: sql.NullString{String: "troll", Valid: true},
		Conditions:    json.RawMessage(`[]`),
	}

	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 8, DamageType: "fire",
	})
	assert.NoError(t, err)
	assert.Equal(t, 16, res.FinalDamage)
	assert.Equal(t, int32(34), captured.HpCurrent)
}

func TestApplyDamage_TempHPAbsorbsBeforeCurrent(t *testing.T) {
	encounterID := uuid.New()
	ms, captured := applyDamageMockStore()
	svc := NewService(ms)
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		HpMax: 30, HpCurrent: 30, TempHp: 12, IsAlive: true, IsNpc: false,
		Conditions: json.RawMessage(`[]`),
	}

	// 7 damage entirely soaked by 12 temp HP -> hp_current unchanged, temp -> 5
	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 7, DamageType: "slashing",
	})
	assert.NoError(t, err)
	assert.Equal(t, 7, res.AbsorbedTemp)
	assert.Equal(t, 0, res.FinalDamage)
	assert.Equal(t, int32(30), captured.HpCurrent)
	assert.Equal(t, int32(5), captured.TempHp)
}

func TestApplyDamage_TempHPPartialSpillsIntoHP(t *testing.T) {
	encounterID := uuid.New()
	ms, captured := applyDamageMockStore()
	svc := NewService(ms)
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		HpMax: 30, HpCurrent: 30, TempHp: 5, IsAlive: true, IsNpc: false,
		Conditions: json.RawMessage(`[]`),
	}

	// 10 damage: 5 absorbed by temp -> 5 spills into HP
	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 10, DamageType: "slashing",
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, res.AbsorbedTemp)
	assert.Equal(t, 5, res.FinalDamage)
	assert.Equal(t, int32(25), captured.HpCurrent)
	assert.Equal(t, int32(0), captured.TempHp)
}

func TestApplyDamage_ResistanceAppliedBeforeTempHP(t *testing.T) {
	encounterID := uuid.New()
	ms, captured := applyDamageMockStore()
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "bear", DamageResistances: []string{"slashing"}}, nil
	}
	svc := NewService(ms)
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		HpMax: 30, HpCurrent: 30, TempHp: 4, IsAlive: true, IsNpc: true,
		CreatureRefID: sql.NullString{String: "bear", Valid: true},
		Conditions:    json.RawMessage(`[]`),
	}
	// 20 raw -> resistance halves to 10 -> 4 absorbed by temp -> 6 to HP
	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 20, DamageType: "slashing",
	})
	assert.NoError(t, err)
	assert.Equal(t, 4, res.AbsorbedTemp)
	assert.Equal(t, 6, res.FinalDamage)
	assert.Equal(t, int32(24), captured.HpCurrent)
	assert.Equal(t, int32(0), captured.TempHp)
}

func TestApplyDamage_ExhaustionLevel4HalvesMaxHP(t *testing.T) {
	encounterID := uuid.New()
	ms, captured := applyDamageMockStore()
	svc := NewService(ms)
	// Exhaustion 4: effective max HP is halved (40/2 = 20). Current HP is 35
	// (above the new effective max), so it must be capped to 20 before damage.
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		HpMax: 40, HpCurrent: 35, IsAlive: true, IsNpc: false,
		ExhaustionLevel: 4,
		Conditions:      json.RawMessage(`[]`),
	}

	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 5, DamageType: "slashing",
	})
	assert.NoError(t, err)
	// Effective HP = min(35, 20) = 20 -> 20 - 5 = 15.
	assert.Equal(t, int32(15), captured.HpCurrent)
	assert.Equal(t, 5, res.FinalDamage)
}

func TestApplyDamage_ExhaustionLevel6KillsImmediately(t *testing.T) {
	encounterID := uuid.New()
	ms, captured := applyDamageMockStore()
	svc := NewService(ms)
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		HpMax: 40, HpCurrent: 40, IsAlive: true, IsNpc: false,
		ExhaustionLevel: 6,
		Conditions:      json.RawMessage(`[]`),
	}

	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 1, DamageType: "slashing",
	})
	assert.NoError(t, err)
	assert.True(t, res.Killed)
	assert.Equal(t, int32(0), captured.HpCurrent)
	assert.False(t, captured.IsAlive)
}

func TestApplyDamage_PCAtZeroStaysAliveDying(t *testing.T) {
	encounterID := uuid.New()
	ms, captured := applyDamageMockStore()
	svc := NewService(ms)
	// Damage drops the PC to 0 with overflow well below HpMax so the
	// Phase 43 instant-death rule does NOT fire (overflow < HpMax).
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		HpMax: 20, HpCurrent: 5, IsAlive: true, IsNpc: false,
		Conditions: json.RawMessage(`[]`),
	}

	_, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 12, DamageType: "fire",
	})
	assert.NoError(t, err)
	assert.Equal(t, int32(0), captured.HpCurrent)
	assert.True(t, captured.IsAlive, "PCs at 0 HP go dying, not dead")
}

func TestApplyDamage_NPCDiesAtZero(t *testing.T) {
	encounterID := uuid.New()
	ms, captured := applyDamageMockStore()
	svc := NewService(ms)
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		HpMax: 10, HpCurrent: 5, IsAlive: true, IsNpc: true,
		Conditions: json.RawMessage(`[]`),
	}

	_, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 100, DamageType: "fire",
	})
	assert.NoError(t, err)
	assert.Equal(t, int32(0), captured.HpCurrent)
	assert.False(t, captured.IsAlive, "NPCs at 0 HP die")
}

func TestApplyDamage_OverrideSkipsRIVAndTempHP(t *testing.T) {
	encounterID := uuid.New()
	ms, captured := applyDamageMockStore()
	// Even though the creature has resistance to slashing, Override=true must
	// honor the raw damage value end-to-end.
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "x", DamageResistances: []string{"slashing"}}, nil
	}
	svc := NewService(ms)
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		HpMax: 30, HpCurrent: 30, TempHp: 10, IsAlive: true, IsNpc: true,
		CreatureRefID: sql.NullString{String: "x", Valid: true},
		Conditions:    json.RawMessage(`[]`),
	}

	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 8, DamageType: "slashing",
		Override: true,
	})
	assert.NoError(t, err)
	assert.Equal(t, 8, res.FinalDamage, "override must not halve")
	assert.Equal(t, 0, res.AbsorbedTemp, "override must not consume temp HP")
	assert.Equal(t, int32(22), captured.HpCurrent)
	assert.Equal(t, int32(10), captured.TempHp, "temp HP untouched in override")
}

func TestApplyDamage_PCNoCreatureRefHasNoRIV(t *testing.T) {
	encounterID := uuid.New()
	ms, captured := applyDamageMockStore()
	// PCs have no creature_ref_id -> no R/I/V lookup. getCreature must NOT be
	// invoked. Wire it to fail loudly to verify.
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		t.Fatalf("getCreature must not be called for PC; called with %s", id)
		return refdata.Creature{}, nil
	}
	svc := NewService(ms)
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		HpMax: 30, HpCurrent: 30, IsAlive: true, IsNpc: false,
		Conditions: json.RawMessage(`[]`),
	}

	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 7, DamageType: "fire",
	})
	assert.NoError(t, err)
	assert.Equal(t, 7, res.FinalDamage)
	assert.Equal(t, int32(23), captured.HpCurrent)
}

func TestApplyDamage_PetrifiedConditionGrantsResistance(t *testing.T) {
	encounterID := uuid.New()
	ms, captured := applyDamageMockStore()
	svc := NewService(ms)
	conds := json.RawMessage(`[{"condition":"petrified"}]`)
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		HpMax: 30, HpCurrent: 30, IsAlive: true, IsNpc: false,
		Conditions: conds,
	}

	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 10, DamageType: "fire",
	})
	assert.NoError(t, err)
	// petrified -> resistance to all damage -> 10/2 = 5
	assert.Equal(t, 5, res.FinalDamage)
	assert.Equal(t, int32(25), captured.HpCurrent)
}

func TestApplyDamage_NegativeDamageRejected(t *testing.T) {
	svc := NewService(defaultMockStore())
	_, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		Target:    refdata.Combatant{Conditions: json.RawMessage(`[]`)},
		RawDamage: -1,
	})
	assert.Error(t, err)
}

func TestApplyDamage_InvalidConditionsJSONErrors(t *testing.T) {
	svc := NewService(defaultMockStore())
	_, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		Target:    refdata.Combatant{Conditions: json.RawMessage(`not-json`), HpMax: 10, HpCurrent: 10},
		RawDamage: 1,
	})
	assert.Error(t, err)
}

func TestApplyDamage_NPCCreatureLookupErrorPropagates(t *testing.T) {
	encounterID := uuid.New()
	ms, _ := applyDamageMockStore()
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{}, fmt.Errorf("transient db error")
	}
	svc := NewService(ms)
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		HpMax: 30, HpCurrent: 30, IsAlive: true, IsNpc: true,
		CreatureRefID: sql.NullString{String: "x", Valid: true},
		Conditions:    json.RawMessage(`[]`),
	}

	_, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 5, DamageType: "fire",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transient db error")
}

func TestApplyDamage_NPCMissingCreatureRowSkipsRIV(t *testing.T) {
	encounterID := uuid.New()
	ms, captured := applyDamageMockStore()
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{}, sql.ErrNoRows
	}
	svc := NewService(ms)
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		HpMax: 20, HpCurrent: 20, IsAlive: true, IsNpc: true,
		CreatureRefID: sql.NullString{String: "homebrew-missing", Valid: true},
		Conditions:    json.RawMessage(`[]`),
	}
	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 7, DamageType: "fire",
	})
	assert.NoError(t, err)
	// No R/I/V → 7 damage straight through
	assert.Equal(t, 7, res.FinalDamage)
	assert.Equal(t, int32(13), captured.HpCurrent)
}

// TestApplyDamage_FiresConcentrationSave verifies the wrapper still funnels
// through applyDamageHP which enqueues a concentration save when the target
// is concentrating and damage > 0.
func TestApplyDamage_FiresConcentrationSave(t *testing.T) {
	encounterID := uuid.New()
	ms, _ := applyDamageMockStore()
	saveCreated := false
	var capturedSave refdata.CreatePendingSaveParams
	ms.getCombatantConcentrationFn = func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
		return refdata.GetCombatantConcentrationRow{
			ConcentrationSpellID:   sql.NullString{String: "bless", Valid: true},
			ConcentrationSpellName: sql.NullString{String: "Bless", Valid: true},
		}, nil
	}
	ms.createPendingSaveFn = func(ctx context.Context, arg refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
		saveCreated = true
		capturedSave = arg
		return refdata.PendingSafe{ID: uuid.New(), EncounterID: arg.EncounterID, CombatantID: arg.CombatantID}, nil
	}
	svc := NewService(ms)
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		HpMax: 30, HpCurrent: 30, IsAlive: true, IsNpc: false,
		Conditions: json.RawMessage(`[]`),
	}
	_, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 8, DamageType: "fire",
	})
	assert.NoError(t, err)
	assert.True(t, saveCreated, "concentration save must be enqueued")
	assert.Equal(t, "concentration", capturedSave.Source)
}

// --- SR-021: Rage damage resistance has a consumer in damage.go ---
//
// Rage declares `EffectGrantResistance` over bludgeoning/piercing/slashing
// when the combatant is raging. Before SR-021, ProcessorResult.Resistances
// was populated but never read by the damage pipeline, so a raging
// barbarian took full slashing damage. These tests pin the contract:
// raging halves B/P/S, leaves non-physical types alone, and is a no-op
// when the combatant is not actually raging.

// ragingBarbarianFixture builds a PC combatant + matching character row
// representing a level-3 raging barbarian with the canonical "rage"
// mechanical_effect feature. The mockStore's getCharacterFn is wired
// to return the character when ApplyDamage's resolveDamageProfile
// looks it up by CharacterID.
func ragingBarbarianFixture(t *testing.T, ms *mockStore, isRaging bool) (refdata.Combatant, uuid.UUID) {
	t.Helper()
	charID := uuid.New()
	classesJSON := []byte(`[{"class":"Barbarian","level":3}]`)
	featuresJSON := []byte(`[{"name":"Rage","mechanical_effect":"rage"}]`)
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		if id != charID {
			return refdata.Character{}, sql.ErrNoRows
		}
		return refdata.Character{
			ID:       charID,
			Classes:  classesJSON,
			Features: pqtype.NullRawMessage{RawMessage: featuresJSON, Valid: true},
		}, nil
	}
	target := refdata.Combatant{
		ID:          uuid.New(),
		HpMax:       30,
		HpCurrent:   30,
		IsAlive:     true,
		IsNpc:       false,
		IsRaging:    isRaging,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		Conditions:  json.RawMessage(`[]`),
	}
	return target, charID
}

func TestApplyDamage_RagingBarbarianHalvesSlashing(t *testing.T) {
	encounterID := uuid.New()
	ms, captured := applyDamageMockStore()
	target, _ := ragingBarbarianFixture(t, ms, true)
	target.EncounterID = encounterID
	svc := NewService(ms)

	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 10, DamageType: "slashing",
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, res.FinalDamage, "rage must halve slashing")
	assert.Contains(t, res.Reason, "resistance")
	assert.Equal(t, int32(25), captured.HpCurrent)
}

func TestApplyDamage_RagingBarbarianFireUnaffected(t *testing.T) {
	encounterID := uuid.New()
	ms, captured := applyDamageMockStore()
	target, _ := ragingBarbarianFixture(t, ms, true)
	target.EncounterID = encounterID
	svc := NewService(ms)

	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 10, DamageType: "fire",
	})
	assert.NoError(t, err)
	assert.Equal(t, 10, res.FinalDamage, "rage must NOT resist fire")
	assert.Equal(t, int32(20), captured.HpCurrent)
}

func TestApplyDamage_NonRagingBarbarianTakesFullSlashing(t *testing.T) {
	encounterID := uuid.New()
	ms, captured := applyDamageMockStore()
	target, _ := ragingBarbarianFixture(t, ms, false)
	target.EncounterID = encounterID
	svc := NewService(ms)

	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 10, DamageType: "slashing",
	})
	assert.NoError(t, err)
	assert.Equal(t, 10, res.FinalDamage, "non-raging barb takes full slashing")
	assert.Equal(t, int32(20), captured.HpCurrent)
}

