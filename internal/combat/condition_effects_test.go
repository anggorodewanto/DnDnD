package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

func mustMarshal(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// --- TDD Cycle 1: Save effects — auto-fail STR/DEX for paralyzed/stunned/unconscious/petrified ---

func TestCheckSaveConditionEffects_ParalyzedAutoFailSTR(t *testing.T) {
	conds := []CombatCondition{{Condition: "paralyzed"}}
	autoFail, _, reasons := CheckSaveConditionEffects(conds, "str")
	assert.True(t, autoFail)
	assert.Contains(t, reasons, "paralyzed: auto-fail STR save")
}

func TestCheckSaveConditionEffects_ParalyzedAutoFailDEX(t *testing.T) {
	conds := []CombatCondition{{Condition: "paralyzed"}}
	autoFail, _, reasons := CheckSaveConditionEffects(conds, "dex")
	assert.True(t, autoFail)
	assert.Contains(t, reasons, "paralyzed: auto-fail DEX save")
}

func TestCheckSaveConditionEffects_StunnedAutoFailSTR(t *testing.T) {
	conds := []CombatCondition{{Condition: "stunned"}}
	autoFail, _, _ := CheckSaveConditionEffects(conds, "str")
	assert.True(t, autoFail)
}

func TestCheckSaveConditionEffects_UnconsciousAutoFailDEX(t *testing.T) {
	conds := []CombatCondition{{Condition: "unconscious"}}
	autoFail, _, _ := CheckSaveConditionEffects(conds, "dex")
	assert.True(t, autoFail)
}

func TestCheckSaveConditionEffects_PetrifiedAutoFailSTR(t *testing.T) {
	conds := []CombatCondition{{Condition: "petrified"}}
	autoFail, _, _ := CheckSaveConditionEffects(conds, "str")
	assert.True(t, autoFail)
}

func TestCheckSaveConditionEffects_ParalyzedNoEffectOnWIS(t *testing.T) {
	conds := []CombatCondition{{Condition: "paralyzed"}}
	autoFail, mode, _ := CheckSaveConditionEffects(conds, "wis")
	assert.False(t, autoFail)
	assert.Equal(t, dice.Normal, mode)
}

func TestCheckSaveConditionEffects_RestrainedDisadvDEX(t *testing.T) {
	conds := []CombatCondition{{Condition: "restrained"}}
	autoFail, mode, reasons := CheckSaveConditionEffects(conds, "dex")
	assert.False(t, autoFail)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Contains(t, reasons, "restrained: disadvantage on DEX save")
}

func TestCheckSaveConditionEffects_RestrainedNoEffectOnSTR(t *testing.T) {
	conds := []CombatCondition{{Condition: "restrained"}}
	autoFail, mode, _ := CheckSaveConditionEffects(conds, "str")
	assert.False(t, autoFail)
	assert.Equal(t, dice.Normal, mode)
}

func TestCheckSaveConditionEffects_DodgeAdvDEX(t *testing.T) {
	conds := []CombatCondition{{Condition: "dodge"}}
	autoFail, mode, reasons := CheckSaveConditionEffects(conds, "dex")
	assert.False(t, autoFail)
	assert.Equal(t, dice.Advantage, mode)
	assert.Contains(t, reasons, "dodge: advantage on DEX save")
}

func TestCheckSaveConditionEffects_DodgeNoEffectOnSTR(t *testing.T) {
	conds := []CombatCondition{{Condition: "dodge"}}
	autoFail, mode, _ := CheckSaveConditionEffects(conds, "str")
	assert.False(t, autoFail)
	assert.Equal(t, dice.Normal, mode)
}

func TestCheckSaveConditionEffects_NoConds(t *testing.T) {
	autoFail, mode, reasons := CheckSaveConditionEffects(nil, "dex")
	assert.False(t, autoFail)
	assert.Equal(t, dice.Normal, mode)
	assert.Empty(t, reasons)
}

func TestCheckSaveConditionEffects_AutoFailTakesPriority(t *testing.T) {
	// Paralyzed (auto-fail DEX) + restrained (disadv DEX) + dodge (adv DEX)
	// auto-fail should win
	conds := []CombatCondition{
		{Condition: "paralyzed"},
		{Condition: "restrained"},
		{Condition: "dodge"},
	}
	autoFail, _, _ := CheckSaveConditionEffects(conds, "dex")
	assert.True(t, autoFail)
}

func TestCheckSaveConditionEffects_AdvAndDisadvCancel(t *testing.T) {
	// restrained (disadv DEX) + dodge (adv DEX) = normal
	conds := []CombatCondition{
		{Condition: "restrained"},
		{Condition: "dodge"},
	}
	autoFail, mode, _ := CheckSaveConditionEffects(conds, "dex")
	assert.False(t, autoFail)
	assert.Equal(t, dice.AdvantageAndDisadvantage, mode)
}

// --- TDD Cycle 2: Ability check effects ---

func TestCheckAbilityCheckEffects_BlindedAutoFailSight(t *testing.T) {
	conds := []CombatCondition{{Condition: "blinded"}}
	autoFail, _, reasons := CheckAbilityCheckEffects(conds, true, false, false)
	assert.True(t, autoFail)
	assert.Contains(t, reasons, "blinded: auto-fail (requires sight)")
}

func TestCheckAbilityCheckEffects_BlindedNoEffectNonSight(t *testing.T) {
	conds := []CombatCondition{{Condition: "blinded"}}
	autoFail, mode, _ := CheckAbilityCheckEffects(conds, false, false, false)
	assert.False(t, autoFail)
	assert.Equal(t, dice.Normal, mode)
}

func TestCheckAbilityCheckEffects_DeafenedAutoFailHearing(t *testing.T) {
	conds := []CombatCondition{{Condition: "deafened"}}
	autoFail, _, reasons := CheckAbilityCheckEffects(conds, false, true, false)
	assert.True(t, autoFail)
	assert.Contains(t, reasons, "deafened: auto-fail (requires hearing)")
}

func TestCheckAbilityCheckEffects_DeafenedNoEffectNonHearing(t *testing.T) {
	conds := []CombatCondition{{Condition: "deafened"}}
	autoFail, mode, _ := CheckAbilityCheckEffects(conds, false, false, false)
	assert.False(t, autoFail)
	assert.Equal(t, dice.Normal, mode)
}

func TestCheckAbilityCheckEffects_FrightenedDisadvWhenVisible(t *testing.T) {
	conds := []CombatCondition{{Condition: "frightened"}}
	autoFail, mode, reasons := CheckAbilityCheckEffects(conds, false, false, true)
	assert.False(t, autoFail)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Contains(t, reasons, "frightened: disadvantage on ability checks (fear source visible)")
}

func TestCheckAbilityCheckEffects_FrightenedNoEffectWhenNotVisible(t *testing.T) {
	conds := []CombatCondition{{Condition: "frightened"}}
	autoFail, mode, reasons := CheckAbilityCheckEffects(conds, false, false, false)
	assert.False(t, autoFail)
	assert.Equal(t, dice.Normal, mode)
	assert.Empty(t, reasons)
}

func TestCheckAbilityCheckEffects_PoisonedDisadv(t *testing.T) {
	conds := []CombatCondition{{Condition: "poisoned"}}
	autoFail, mode, reasons := CheckAbilityCheckEffects(conds, false, false, false)
	assert.False(t, autoFail)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Contains(t, reasons, "poisoned: disadvantage on ability checks")
}

func TestCheckAbilityCheckEffects_NoConds(t *testing.T) {
	autoFail, mode, reasons := CheckAbilityCheckEffects(nil, false, false, false)
	assert.False(t, autoFail)
	assert.Equal(t, dice.Normal, mode)
	assert.Empty(t, reasons)
}

func TestCheckAbilityCheckEffects_MultiplePoisonedAndFrightenedVisible(t *testing.T) {
	conds := []CombatCondition{
		{Condition: "poisoned"},
		{Condition: "frightened"},
	}
	autoFail, mode, reasons := CheckAbilityCheckEffects(conds, false, false, true)
	assert.False(t, autoFail)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Len(t, reasons, 2)
}

func TestCheckAbilityCheckEffects_MultiplePoisonedAndFrightenedNotVisible(t *testing.T) {
	conds := []CombatCondition{
		{Condition: "poisoned"},
		{Condition: "frightened"},
	}
	autoFail, mode, reasons := CheckAbilityCheckEffects(conds, false, false, false)
	assert.False(t, autoFail)
	assert.Equal(t, dice.Disadvantage, mode)
	// Only poisoned should contribute, not frightened (fear source not visible)
	assert.Len(t, reasons, 1)
	assert.Contains(t, reasons, "poisoned: disadvantage on ability checks")
}

// --- TDD Cycle 3: Speed effects ---

func TestEffectiveSpeed_GrappledZero(t *testing.T) {
	conds := []CombatCondition{{Condition: "grappled"}}
	assert.Equal(t, 0, EffectiveSpeed(30, conds))
}

func TestEffectiveSpeed_RestrainedZero(t *testing.T) {
	conds := []CombatCondition{{Condition: "restrained"}}
	assert.Equal(t, 0, EffectiveSpeed(30, conds))
}

func TestEffectiveSpeed_NormalSpeed(t *testing.T) {
	conds := []CombatCondition{{Condition: "frightened"}}
	assert.Equal(t, 30, EffectiveSpeed(30, conds))
}

func TestEffectiveSpeed_NoConds(t *testing.T) {
	assert.Equal(t, 30, EffectiveSpeed(30, nil))
}

// --- TDD Cycle 4: Action blocking ---

func TestIsIncapacitated_Incapacitated(t *testing.T) {
	conds := []CombatCondition{{Condition: "incapacitated"}}
	assert.True(t, IsIncapacitated(conds))
}

func TestIsIncapacitated_Stunned(t *testing.T) {
	conds := []CombatCondition{{Condition: "stunned"}}
	assert.True(t, IsIncapacitated(conds))
}

func TestIsIncapacitated_Paralyzed(t *testing.T) {
	conds := []CombatCondition{{Condition: "paralyzed"}}
	assert.True(t, IsIncapacitated(conds))
}

func TestIsIncapacitated_Unconscious(t *testing.T) {
	conds := []CombatCondition{{Condition: "unconscious"}}
	assert.True(t, IsIncapacitated(conds))
}

func TestIsIncapacitated_Petrified(t *testing.T) {
	conds := []CombatCondition{{Condition: "petrified"}}
	assert.True(t, IsIncapacitated(conds))
}

func TestIsIncapacitated_NotIncapacitated(t *testing.T) {
	conds := []CombatCondition{{Condition: "frightened"}}
	assert.False(t, IsIncapacitated(conds))
}

func TestIsIncapacitated_NoConds(t *testing.T) {
	assert.False(t, IsIncapacitated(nil))
}

func TestCanAct_Incapacitated(t *testing.T) {
	conds := []CombatCondition{{Condition: "incapacitated"}}
	canAct, reason := CanAct(conds)
	assert.False(t, canAct)
	assert.Equal(t, "cannot act: incapacitated", reason)
}

func TestCanAct_Stunned(t *testing.T) {
	conds := []CombatCondition{{Condition: "stunned"}}
	canAct, reason := CanAct(conds)
	assert.False(t, canAct)
	assert.Equal(t, "cannot act: stunned", reason)
}

func TestCanAct_Normal(t *testing.T) {
	conds := []CombatCondition{{Condition: "frightened"}}
	canAct, reason := CanAct(conds)
	assert.True(t, canAct)
	assert.Empty(t, reason)
}

func TestCanAct_NoConds(t *testing.T) {
	canAct, reason := CanAct(nil)
	assert.True(t, canAct)
	assert.Empty(t, reason)
}

// --- TDD Cycle 5: Charmed restriction ---

func TestIsCharmedBy_MatchingSource(t *testing.T) {
	conds := []CombatCondition{
		{Condition: "charmed", SourceCombatantID: "abc-123"},
	}
	assert.True(t, IsCharmedBy(conds, "abc-123"))
}

func TestIsCharmedBy_DifferentSource(t *testing.T) {
	conds := []CombatCondition{
		{Condition: "charmed", SourceCombatantID: "abc-123"},
	}
	assert.False(t, IsCharmedBy(conds, "xyz-789"))
}

func TestIsCharmedBy_NotCharmed(t *testing.T) {
	conds := []CombatCondition{
		{Condition: "frightened", SourceCombatantID: "abc-123"},
	}
	assert.False(t, IsCharmedBy(conds, "abc-123"))
}

func TestIsCharmedBy_NoConds(t *testing.T) {
	assert.False(t, IsCharmedBy(nil, "abc-123"))
}

// --- TDD Cycle 6: Frightened movement validation ---

func TestValidateFrightenedMovement_MovingCloserBlocked(t *testing.T) {
	conds := []CombatCondition{
		{Condition: "frightened", SourceCombatantID: "fear-src"},
	}
	// Fear source at (5, 5), current at (3, 3), target at (4, 4) is closer
	fearSources := map[string][2]int{"fear-src": {5, 5}}
	err := ValidateFrightenedMovement(conds, 3, 3, 4, 4, fearSources)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot move closer to source of fear")
}

func TestValidateFrightenedMovement_MovingAwayAllowed(t *testing.T) {
	conds := []CombatCondition{
		{Condition: "frightened", SourceCombatantID: "fear-src"},
	}
	// Fear source at (5, 5), current at (3, 3), target at (2, 2) is farther
	fearSources := map[string][2]int{"fear-src": {5, 5}}
	err := ValidateFrightenedMovement(conds, 3, 3, 2, 2, fearSources)
	assert.NoError(t, err)
}

func TestValidateFrightenedMovement_SameDistanceAllowed(t *testing.T) {
	conds := []CombatCondition{
		{Condition: "frightened", SourceCombatantID: "fear-src"},
	}
	// Fear source at (5, 5), current at (3, 3), target at (3, 2) — same Chebyshev distance
	fearSources := map[string][2]int{"fear-src": {5, 5}}
	err := ValidateFrightenedMovement(conds, 3, 3, 3, 2, fearSources)
	assert.NoError(t, err)
}

func TestValidateFrightenedMovement_NotFrightened(t *testing.T) {
	conds := []CombatCondition{
		{Condition: "poisoned"},
	}
	fearSources := map[string][2]int{}
	err := ValidateFrightenedMovement(conds, 3, 3, 4, 4, fearSources)
	assert.NoError(t, err)
}

func TestValidateFrightenedMovement_NoSourceTracked(t *testing.T) {
	conds := []CombatCondition{
		{Condition: "frightened"}, // no source_combatant_id
	}
	fearSources := map[string][2]int{}
	err := ValidateFrightenedMovement(conds, 3, 3, 4, 4, fearSources)
	assert.NoError(t, err)
}

func TestValidateFrightenedMovement_SourceNotInMap(t *testing.T) {
	conds := []CombatCondition{
		{Condition: "frightened", SourceCombatantID: "missing-id"},
	}
	fearSources := map[string][2]int{} // source not in map
	err := ValidateFrightenedMovement(conds, 3, 3, 4, 4, fearSources)
	assert.NoError(t, err) // no fear source position known, allow move
}

// --- TDD Cycle 7: IsIncapacitatedRaw / CanActRaw from JSON ---

func TestIsIncapacitatedRaw_Stunned(t *testing.T) {
	raw := mustMarshal([]CombatCondition{{Condition: "stunned"}})
	assert.True(t, IsIncapacitatedRaw(raw))
}

func TestIsIncapacitatedRaw_Normal(t *testing.T) {
	raw := mustMarshal([]CombatCondition{{Condition: "poisoned"}})
	assert.False(t, IsIncapacitatedRaw(raw))
}

func TestIsIncapacitatedRaw_EmptyJSON(t *testing.T) {
	assert.False(t, IsIncapacitatedRaw(nil))
}

func TestCanActRaw_Paralyzed(t *testing.T) {
	raw := mustMarshal([]CombatCondition{{Condition: "paralyzed"}})
	canAct, reason := CanActRaw(raw)
	assert.False(t, canAct)
	assert.Contains(t, reason, "paralyzed")
}

func TestCanActRaw_Normal(t *testing.T) {
	raw := mustMarshal([]CombatCondition{{Condition: "prone"}})
	canAct, _ := CanActRaw(raw)
	assert.True(t, canAct)
}

func TestIsIncapacitatedRaw_InvalidJSON(t *testing.T) {
	assert.False(t, IsIncapacitatedRaw(json.RawMessage(`invalid`)))
}

func TestCanActRaw_InvalidJSON(t *testing.T) {
	canAct, _ := CanActRaw(json.RawMessage(`invalid`))
	assert.True(t, canAct) // gracefully returns true if can't parse
}

// --- Edge case tests for abilityLabel ---

func TestCheckSaveConditionEffects_AllAbilities(t *testing.T) {
	abilities := []string{"str", "dex", "con", "int", "wis", "cha"}
	for _, ab := range abilities {
		conds := []CombatCondition{{Condition: "paralyzed"}}
		autoFail, _, _ := CheckSaveConditionEffects(conds, ab)
		if ab == "str" || ab == "dex" {
			assert.True(t, autoFail, "paralyzed should auto-fail %s", ab)
		} else {
			assert.False(t, autoFail, "paralyzed should not auto-fail %s", ab)
		}
	}
}

func TestCheckSaveConditionEffects_UnknownAbility(t *testing.T) {
	conds := []CombatCondition{{Condition: "paralyzed"}}
	autoFail, mode, _ := CheckSaveConditionEffects(conds, "luck")
	assert.False(t, autoFail)
	assert.Equal(t, dice.Normal, mode)
}

func TestAbilityLabel(t *testing.T) {
	tests := []struct{ input, want string }{
		{"str", "STR"}, {"dex", "DEX"}, {"con", "CON"},
		{"int", "INT"}, {"wis", "WIS"}, {"cha", "CHA"},
		{"unknown", "unknown"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, abilityLabel(tc.input))
	}
}

// --- Edge case: gridDistance negative deltas ---

func TestGridDistance_AllDirections(t *testing.T) {
	// Test that distance works in all directions
	assert.Equal(t, 3, gridDistance(0, 0, 3, 0))
	assert.Equal(t, 3, gridDistance(3, 0, 0, 0))
	assert.Equal(t, 3, gridDistance(0, 0, 0, 3))
	assert.Equal(t, 3, gridDistance(0, 3, 0, 0))
	assert.Equal(t, 3, gridDistance(0, 0, 3, 3))
	assert.Equal(t, 0, gridDistance(2, 2, 2, 2))
}

// --- TDD Cycle 7b: StandFromProneCost ---

func TestStandFromProneCost_Normal(t *testing.T) {
	assert.Equal(t, 15, StandFromProneCost(30))
}

func TestStandFromProneCost_OddSpeed(t *testing.T) {
	// 25 / 2 = 12 (rounded down)
	assert.Equal(t, 12, StandFromProneCost(25))
}

func TestStandFromProneCost_ZeroSpeed(t *testing.T) {
	assert.Equal(t, 0, StandFromProneCost(0))
}

func TestStandFromProneCost_SmallSpeed(t *testing.T) {
	assert.Equal(t, 5, StandFromProneCost(10))
}

// --- TDD Cycle 8: EffectiveSpeed applied in ResolveTurnResources ---

func TestResolveTurnResources_GrappledZeroSpeed(t *testing.T) {
	charID := uuid.New()
	ms := &mockStore{
		getCharacterFn: func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
			return refdata.Character{
				ID:               charID,
				SpeedFt:          30,
				ProficiencyBonus: 2,
				AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10}`),
				Classes:          json.RawMessage(`[{"class":"Fighter","level":1}]`),
			}, nil
		},
		getClassFn: func(ctx context.Context, id string) (refdata.Class, error) {
			return refdata.Class{
				AttacksPerAction: json.RawMessage(`{"1":1}`),
			}, nil
		},
	}
	svc := NewService(ms)
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		Conditions:  mustMarshal([]CombatCondition{{Condition: "grappled"}}),
	}

	speed, _, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), speed)
}

func TestResolveTurnResources_NormalSpeedNoConditions(t *testing.T) {
	charID := uuid.New()
	ms := &mockStore{
		getCharacterFn: func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
			return refdata.Character{
				ID:               charID,
				SpeedFt:          30,
				ProficiencyBonus: 2,
				AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10}`),
				Classes:          json.RawMessage(`[{"class":"Fighter","level":1}]`),
			}, nil
		},
		getClassFn: func(ctx context.Context, id string) (refdata.Class, error) {
			return refdata.Class{
				AttacksPerAction: json.RawMessage(`{"1":1}`),
			}, nil
		},
	}
	svc := NewService(ms)
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		Conditions:  json.RawMessage(`[]`),
	}

	speed, _, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(30), speed)
}

func TestResolveTurnResources_RestrainedZeroSpeed(t *testing.T) {
	charID := uuid.New()
	ms := &mockStore{
		getCharacterFn: func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
			return refdata.Character{
				ID:               charID,
				SpeedFt:          30,
				ProficiencyBonus: 2,
				AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10}`),
				Classes:          json.RawMessage(`[{"class":"Fighter","level":1}]`),
			}, nil
		},
		getClassFn: func(ctx context.Context, id string) (refdata.Class, error) {
			return refdata.Class{
				AttacksPerAction: json.RawMessage(`{"1":1}`),
			}, nil
		},
	}
	svc := NewService(ms)
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		Conditions:  mustMarshal([]CombatCondition{{Condition: "restrained"}}),
	}

	speed, _, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), speed)
}

func TestResolveTurnResources_NPCGrappledZeroSpeed(t *testing.T) {
	svc := NewService(&mockStore{})
	combatant := refdata.Combatant{
		IsNpc:      true,
		Conditions: mustMarshal([]CombatCondition{{Condition: "grappled"}}),
	}

	speed, _, err := svc.ResolveTurnResources(context.Background(), combatant)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), speed)
}

// --- TDD Cycle 9: Auto-skip incapacitated turns in AdvanceTurn ---

func TestAdvanceTurn_SkipsIncapacitatedCombatant(t *testing.T) {
	encounterID := uuid.New()
	stunnedID := uuid.New()
	activeID := uuid.New()

	stunned := refdata.Combatant{
		ID:              stunnedID,
		DisplayName:     "Stunned Fighter",
		Conditions:      mustMarshal([]CombatCondition{{Condition: "stunned"}}),
		IsAlive:         true,
		InitiativeOrder: 1,
		IsNpc:           true,
	}
	active := refdata.Combatant{
		ID:              activeID,
		DisplayName:     "Rogue",
		Conditions:      json.RawMessage(`[]`),
		IsAlive:         true,
		InitiativeOrder: 2,
		IsNpc:           true,
	}

	ms := defaultMockStore()
	ms.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:          id,
			Status:      "active",
			RoundNumber: 2,
		}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{stunned, active}, nil
	}
	ms.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return nil, nil // no turns taken yet
	}

	var skippedTurnCreated bool
	ms.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		if arg.CombatantID == stunnedID && arg.Status == "skipped" {
			skippedTurnCreated = true
		}
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber, Status: arg.Status}, nil
	}
	ms.skipTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: id, Status: "skipped"}, nil
	}
	ms.updateEncounterCurrentTurnFn = func(ctx context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID}, nil
	}

	svc := NewService(ms)
	info, err := svc.AdvanceTurn(context.Background(), encounterID)
	assert.NoError(t, err)
	assert.True(t, skippedTurnCreated, "stunned combatant's turn should be created as skipped")
	assert.Equal(t, activeID, info.CombatantID, "should advance to the active (non-incapacitated) combatant")
}

func TestAdvanceTurn_ParalyzedCombatantSkipped(t *testing.T) {
	encounterID := uuid.New()
	paralyzedID := uuid.New()
	activeID := uuid.New()

	paralyzed := refdata.Combatant{
		ID:              paralyzedID,
		DisplayName:     "Paralyzed Wizard",
		Conditions:      mustMarshal([]CombatCondition{{Condition: "paralyzed"}}),
		IsAlive:         true,
		InitiativeOrder: 1,
		IsNpc:           true,
	}
	active := refdata.Combatant{
		ID:              activeID,
		DisplayName:     "Rogue",
		Conditions:      json.RawMessage(`[]`),
		IsAlive:         true,
		InitiativeOrder: 2,
		IsNpc:           true,
	}

	ms := defaultMockStore()
	ms.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:          id,
			Status:      "active",
			RoundNumber: 2,
		}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{paralyzed, active}, nil
	}
	ms.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return nil, nil
	}
	ms.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber, Status: arg.Status}, nil
	}
	ms.skipTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: id, Status: "skipped"}, nil
	}
	ms.updateEncounterCurrentTurnFn = func(ctx context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID}, nil
	}

	svc := NewService(ms)
	info, err := svc.AdvanceTurn(context.Background(), encounterID)
	assert.NoError(t, err)
	assert.Equal(t, activeID, info.CombatantID)
}

func TestSkipIncapacitatedTurn_CreateTurnError(t *testing.T) {
	encounterID := uuid.New()
	stunnedID := uuid.New()

	stunned := refdata.Combatant{
		ID:              stunnedID,
		DisplayName:     "Stunned Fighter",
		Conditions:      mustMarshal([]CombatCondition{{Condition: "stunned"}}),
		IsAlive:         true,
		InitiativeOrder: 1,
		IsNpc:           true,
	}

	ms := defaultMockStore()
	ms.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 2}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{stunned}, nil
	}
	ms.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return nil, nil
	}
	ms.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.AdvanceTurn(context.Background(), encounterID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating skipped turn for incapacitated")
}

func TestSkipIncapacitatedTurn_SkipTurnError(t *testing.T) {
	encounterID := uuid.New()
	stunnedID := uuid.New()

	stunned := refdata.Combatant{
		ID:              stunnedID,
		DisplayName:     "Stunned Fighter",
		Conditions:      mustMarshal([]CombatCondition{{Condition: "stunned"}}),
		IsAlive:         true,
		InitiativeOrder: 1,
		IsNpc:           true,
	}

	ms := defaultMockStore()
	ms.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 2}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{stunned}, nil
	}
	ms.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return nil, nil
	}
	ms.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, Status: "skipped"}, nil
	}
	ms.skipTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("skip db error")
	}

	svc := NewService(ms)
	_, err := svc.AdvanceTurn(context.Background(), encounterID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "skipping incapacitated turn")
}
