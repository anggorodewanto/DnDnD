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
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

var ctx = context.Background()

// TDD Cycle 1: ChannelDivinityMaxUses

func TestChannelDivinityMaxUses_Cleric(t *testing.T) {
	tests := []struct {
		level    int
		expected int
	}{
		{1, 0}, // below level 2 — not available
		{2, 1},
		{5, 1},
		{6, 2},
		{17, 2},
		{18, 3},
		{20, 3},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, ChannelDivinityMaxUses("Cleric", tt.level), "Cleric level %d", tt.level)
	}
}

func TestChannelDivinityMaxUses_Paladin(t *testing.T) {
	tests := []struct {
		level    int
		expected int
	}{
		{2, 0}, // below level 3 — not available
		{3, 1},
		{14, 1},
		{15, 1}, // PHB p.85: Paladin never gains a second CD use
		{20, 1},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, ChannelDivinityMaxUses("Paladin", tt.level), "Paladin level %d", tt.level)
	}
}

func TestChannelDivinityMaxUses_OtherClass(t *testing.T) {
	assert.Equal(t, 0, ChannelDivinityMaxUses("Fighter", 10))
}

// TDD Cycle 3: ValidateChannelDivinity

func TestValidateChannelDivinity(t *testing.T) {
	t.Run("not cleric or paladin", func(t *testing.T) {
		err := ValidateChannelDivinity("Fighter", 10, 2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Cleric or Paladin")
	})

	t.Run("cleric below level 2", func(t *testing.T) {
		err := ValidateChannelDivinity("Cleric", 1, 2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "level 2+")
	})

	t.Run("paladin below level 3", func(t *testing.T) {
		err := ValidateChannelDivinity("Paladin", 2, 2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "level 3+")
	})

	t.Run("no uses remaining", func(t *testing.T) {
		err := ValidateChannelDivinity("Cleric", 5, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no Channel Divinity uses remaining")
	})

	t.Run("valid cleric", func(t *testing.T) {
		err := ValidateChannelDivinity("Cleric", 2, 1)
		assert.NoError(t, err)
	})

	t.Run("valid paladin", func(t *testing.T) {
		err := ValidateChannelDivinity("Paladin", 3, 1)
		assert.NoError(t, err)
	})
}

// TDD Cycle 4: TurnUndead service method

func newChannelDivinityMockStore(char refdata.Character, combatants []refdata.Combatant, creatures map[string]refdata.Creature) *mockStore {
	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
		return combatants, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		if c, ok := creatures[id]; ok {
			return c, nil
		}
		return refdata.Creature{}, fmt.Errorf("creature %q not found", id)
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return char, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		for _, c := range combatants {
			if c.ID == id {
				return c, nil
			}
		}
		return refdata.Combatant{}, fmt.Errorf("combatant not found")
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		for _, c := range combatants {
			if c.ID == arg.ID {
				c.Conditions = arg.Conditions
				return c, nil
			}
		}
		return refdata.Combatant{}, nil
	}
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		for _, c := range combatants {
			if c.ID == arg.ID {
				c.HpCurrent = arg.HpCurrent
				return c, nil
			}
		}
		return refdata.Combatant{}, nil
	}
	ms.createActionLogFn = func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
		return refdata.ActionLog{}, nil
	}
	return ms
}

func TestTurnUndead_FailedSave_AppliesTurnedCondition(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	skeletonCombatantID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	cleric := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	skeleton := refdata.Combatant{
		ID:            skeletonCombatantID,
		EncounterID:   encounterID,
		CreatureRefID: sql.NullString{String: "skeleton", Valid: true},
		DisplayName:   "Skeleton #1",
		PositionCol:   "A",
		PositionRow:   3, // 10ft away (within 30ft)
		HpCurrent:     13,
		HpMax:         13,
		IsNpc:         true,
		IsAlive:       true,
		Conditions:    json.RawMessage(`[]`),
	}

	creatures := map[string]refdata.Creature{
		"skeleton": {
			ID:            "skeleton",
			Type:          "undead",
			Cr:            "1/4",
			AbilityScores: json.RawMessage(`{"str":10,"dex":14,"con":15,"int":6,"wis":8,"cha":5}`),
		},
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{cleric, skeleton}, creatures)
	svc := NewService(ms)

	turn := refdata.Turn{
		ID:          turnID,
		EncounterID: encounterID,
		CombatantID: clericCombatantID,
	}

	// Skeleton WIS save: -1 modifier. Roll 8 → total 7. DC = 8 + 2 + 3 = 13. Fail.
	roller := dice.NewRoller(func(max int) int { return 8 })

	result, err := svc.TurnUndead(ctx, TurnUndeadCommand{
		Cleric:       cleric,
		Turn:         turn,
		CurrentRound: 1,
	}, roller)
	require.NoError(t, err)
	require.Equal(t, 1, len(result.Targets))
	assert.False(t, result.Targets[0].SaveSucceeded)
	assert.True(t, result.Targets[0].Turned)
	assert.False(t, result.Targets[0].Destroyed)
	assert.Contains(t, result.CombatLog, "Turned")
}

func TestTurnUndead_PassedSave_NoEffect(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	skeletonCombatantID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	cleric := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	skeleton := refdata.Combatant{
		ID:            skeletonCombatantID,
		EncounterID:   encounterID,
		CreatureRefID: sql.NullString{String: "skeleton", Valid: true},
		DisplayName:   "Skeleton #1",
		PositionCol:   "A",
		PositionRow:   3,
		HpCurrent:     13,
		HpMax:         13,
		IsNpc:         true,
		IsAlive:       true,
		Conditions:    json.RawMessage(`[]`),
	}

	creatures := map[string]refdata.Creature{
		"skeleton": {
			ID:            "skeleton",
			Type:          "undead",
			Cr:            "1/4",
			AbilityScores: json.RawMessage(`{"str":10,"dex":14,"con":15,"int":6,"wis":8,"cha":5}`),
		},
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{cleric, skeleton}, creatures)
	svc := NewService(ms)

	turn := refdata.Turn{
		ID:          turnID,
		EncounterID: encounterID,
		CombatantID: clericCombatantID,
	}

	// Skeleton WIS save: -1 modifier. Roll 18 → total 17. DC = 13. Pass.
	roller := dice.NewRoller(func(max int) int { return 18 })

	result, err := svc.TurnUndead(ctx, TurnUndeadCommand{
		Cleric:       cleric,
		Turn:         turn,
		CurrentRound: 1,
	}, roller)
	require.NoError(t, err)
	require.Equal(t, 1, len(result.Targets))
	assert.True(t, result.Targets[0].SaveSucceeded)
	assert.False(t, result.Targets[0].Turned)
	assert.Contains(t, result.CombatLog, "Resists")
}

// TDD Cycle 5: Destroy Undead (Cleric 5+)

func TestTurnUndead_DestroyUndead_BelowCRThreshold(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	skeletonCombatantID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":5}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 3,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	cleric := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	skeleton := refdata.Combatant{
		ID:            skeletonCombatantID,
		EncounterID:   encounterID,
		CreatureRefID: sql.NullString{String: "skeleton", Valid: true},
		DisplayName:   "Skeleton #1",
		PositionCol:   "A",
		PositionRow:   3,
		HpCurrent:     13,
		HpMax:         13,
		IsNpc:         true,
		IsAlive:       true,
		Conditions:    json.RawMessage(`[]`),
	}

	// CR 1/4 = 0.25 <= 0.5 (destroy threshold at level 5)
	creatures := map[string]refdata.Creature{
		"skeleton": {
			ID:            "skeleton",
			Type:          "undead",
			Cr:            "1/4",
			AbilityScores: json.RawMessage(`{"str":10,"dex":14,"con":15,"int":6,"wis":8,"cha":5}`),
		},
	}

	var hpUpdateCalled bool
	ms := newChannelDivinityMockStore(char, []refdata.Combatant{cleric, skeleton}, creatures)
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpUpdateCalled = true
		assert.Equal(t, int32(0), arg.HpCurrent)
		assert.False(t, arg.IsAlive)
		return refdata.Combatant{ID: arg.ID, HpCurrent: 0, IsAlive: false}, nil
	}
	svc := NewService(ms)

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: clericCombatantID}

	// Skeleton fails WIS save: roll 5, -1 mod = 4 vs DC=8+3+3=14. Fail.
	roller := dice.NewRoller(func(max int) int { return 5 })

	result, err := svc.TurnUndead(ctx, TurnUndeadCommand{
		Cleric:       cleric,
		Turn:         turn,
		CurrentRound: 1,
	}, roller)
	require.NoError(t, err)
	require.Equal(t, 1, len(result.Targets))
	assert.True(t, result.Targets[0].Destroyed)
	assert.False(t, result.Targets[0].Turned)
	assert.True(t, hpUpdateCalled)
	assert.Contains(t, result.CombatLog, "destroyed")
}

// TDD Cycle 6: PreserveLife

func TestPreserveLife_DistributesHP(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	ally1ID := uuid.New()
	ally2ID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":5}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 3,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	cleric := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	// Ally1: 10/40 HP, within 30ft. Half max = 20. Can heal up to 10 (20-10).
	ally1 := refdata.Combatant{
		ID:          ally1ID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		DisplayName: "Ally1",
		PositionCol: "A",
		PositionRow: 3,
		HpCurrent:   10,
		HpMax:       40,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}

	// Ally2: 5/30 HP, within 30ft. Half max = 15. Can heal up to 10 (15-5).
	ally2 := refdata.Combatant{
		ID:          ally2ID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		DisplayName: "Ally2",
		PositionCol: "A",
		PositionRow: 5,
		HpCurrent:   5,
		HpMax:       30,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}

	combatants := []refdata.Combatant{cleric, ally1, ally2}
	ms := newChannelDivinityMockStore(char, combatants, nil)

	hpUpdates := make(map[uuid.UUID]int32)
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpUpdates[arg.ID] = arg.HpCurrent
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
	}
	svc := NewService(ms)

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: clericCombatantID}

	// Budget = 5 * 5 = 25. Heal ally1 by 8, ally2 by 10 = 18 total (within budget)
	result, err := svc.PreserveLife(ctx, PreserveLifeCommand{
		Cleric: cleric,
		Turn:   turn,
		TargetHealing: map[string]int32{
			ally1ID.String(): 8,
			ally2ID.String(): 10,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, len(result.HealedTargets))
	assert.Contains(t, result.CombatLog, "Preserve Life")

	// Check HP updates
	assert.Equal(t, int32(18), hpUpdates[ally1ID]) // 10 + 8
	assert.Equal(t, int32(15), hpUpdates[ally2ID]) // 5 + 10
}

func TestPreserveLife_ExceedsBudget(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	allyID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":2}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	cleric := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	ally := refdata.Combatant{
		ID:          allyID,
		EncounterID: encounterID,
		DisplayName: "Ally",
		PositionCol: "A",
		PositionRow: 3,
		HpCurrent:   5,
		HpMax:       40,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{cleric, ally}, nil)
	svc := NewService(ms)

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: clericCombatantID}

	// Budget = 5 * 2 = 10. Request 15 healing — exceeds budget.
	_, err := svc.PreserveLife(ctx, PreserveLifeCommand{
		Cleric: cleric,
		Turn:   turn,
		TargetHealing: map[string]int32{
			allyID.String(): 15,
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds budget")
}

func TestPreserveLife_CapsAtHalfMaxHP(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	allyID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":5}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 3,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	cleric := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	// Ally: 18/20 HP. Half max = 10. Already above half max, so can't heal.
	ally := refdata.Combatant{
		ID:          allyID,
		EncounterID: encounterID,
		DisplayName: "Ally",
		PositionCol: "A",
		PositionRow: 3,
		HpCurrent:   18,
		HpMax:       20,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{cleric, ally}, nil)
	svc := NewService(ms)

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: clericCombatantID}

	// Attempt to heal ally by 5, but ally is already at 18 (above half max 10). Cap at 0.
	_, err := svc.PreserveLife(ctx, PreserveLifeCommand{
		Cleric: cleric,
		Turn:   turn,
		TargetHealing: map[string]int32{
			allyID.String(): 5,
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "above half max")
}

// TDD Cycle 7: DMQueue routing for narrative Channel Divinity options

func TestChannelDivinityDMQueue(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	cleric := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{cleric}, nil)
	svc := NewService(ms)
	svc.SetDMNotifier(&fakeDMNotifier{})

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: clericCombatantID}

	result, err := svc.ChannelDivinityDMQueue(ctx, ChannelDivinityDMQueueCommand{
		Caster:     cleric,
		Turn:       turn,
		OptionName: "Knowledge of the Ages",
		ClassName:  "Cleric",
	})
	require.NoError(t, err)
	assert.Contains(t, result.CombatLog, "Knowledge of the Ages")
	assert.Contains(t, result.CombatLog, "#dm-queue")
	assert.Equal(t, "Knowledge of the Ages", result.OptionName)
}

// SR-059: Narrative Channel Divinity options must insert a dm_queue_items row.
func TestChannelDivinityDMQueue_PostsDMQueueItem(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	cleric := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{cleric}, nil)
	notifier := &fakeDMNotifier{postID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}
	svc := NewService(ms)
	svc.SetDMNotifier(notifier)

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: clericCombatantID}

	result, err := svc.ChannelDivinityDMQueue(ctx, ChannelDivinityDMQueueCommand{
		Caster:     cleric,
		Turn:       turn,
		OptionName: "Knowledge of the Ages",
		ClassName:  "Cleric",
		GuildID:    "guild-1",
		CampaignID: "campaign-1",
	})
	require.NoError(t, err)

	// Verify a dm_queue_items row was posted via the notifier.
	require.Len(t, notifier.posts, 1)
	posted := notifier.posts[0]
	assert.Equal(t, dmqueue.KindChannelDivinity, posted.Kind)
	assert.Equal(t, "Thorn", posted.PlayerName)
	assert.Equal(t, "Knowledge of the Ages", posted.Summary)
	assert.Equal(t, "guild-1", posted.GuildID)
	assert.Equal(t, "campaign-1", posted.CampaignID)

	// The returned DMQueueItemID must match the notifier's response.
	assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", result.DMQueueItemID)
}

// TDD Cycle 8: SacredWeapon and VowOfEnmity

func TestSacredWeapon(t *testing.T) {
	paladinID := uuid.New()
	paladinCombatantID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()

	char := refdata.Character{
		ID:               paladinID,
		Classes:          json.RawMessage(`[{"class":"Paladin","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":16}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	paladin := refdata.Combatant{
		ID:          paladinCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: paladinID, Valid: true},
		DisplayName: "Oath",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{paladin}, nil)
	svc := NewService(ms)

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: paladinCombatantID}

	result, err := svc.SacredWeapon(ctx, SacredWeaponCommand{
		Paladin:      paladin,
		Turn:         turn,
		CurrentRound: 1,
	})
	require.NoError(t, err)
	assert.Contains(t, result.CombatLog, "Sacred Weapon")
	assert.Equal(t, 3, result.CHAModifier)
	assert.Equal(t, 0, result.UsesLeft)
}

func TestSacredWeapon_CHAModClampedToMinimum1(t *testing.T) {
	paladinID := uuid.New()
	paladinCombatantID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()

	char := refdata.Character{
		ID:               paladinID,
		Classes:          json.RawMessage(`[{"class":"Paladin","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":8}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	paladin := refdata.Combatant{
		ID:          paladinCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: paladinID, Valid: true},
		DisplayName: "Oath",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{paladin}, nil)
	svc := NewService(ms)

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: paladinCombatantID}

	result, err := svc.SacredWeapon(ctx, SacredWeaponCommand{
		Paladin:      paladin,
		Turn:         turn,
		CurrentRound: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.CHAModifier) // CHA 8 = -1 modifier, clamped to 1
	assert.Equal(t, 0, result.UsesLeft)
}

func TestSacredWeapon_GetCharacterError(t *testing.T) {
	paladinCombatantID := uuid.New()

	paladin := refdata.Combatant{
		ID:          paladinCombatantID,
		CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		DisplayName: "Oath",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)

	_, err := svc.SacredWeapon(ctx, SacredWeaponCommand{
		Paladin: paladin, Turn: refdata.Turn{}, CurrentRound: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

func TestSacredWeapon_ParseFeatureUsesError(t *testing.T) {
	paladinID := uuid.New()
	paladinCombatantID := uuid.New()

	char := refdata.Character{
		ID:          paladinID,
		Classes:     json.RawMessage(`[{"class":"Paladin","level":3}]`),
		FeatureUses: pqtype.NullRawMessage{RawMessage: json.RawMessage(`{invalid`), Valid: true},
	}

	paladin := refdata.Combatant{
		ID:          paladinCombatantID,
		CharacterID: uuid.NullUUID{UUID: paladinID, Valid: true},
		DisplayName: "Oath",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	svc := NewService(ms)

	_, err := svc.SacredWeapon(ctx, SacredWeaponCommand{
		Paladin: paladin, Turn: refdata.Turn{}, CurrentRound: 1,
	})
	assert.Error(t, err)
}

func TestSacredWeapon_ValidateChannelDivinityError(t *testing.T) {
	paladinID := uuid.New()
	paladinCombatantID := uuid.New()

	char := refdata.Character{
		ID:               paladinID,
		Classes:          json.RawMessage(`[{"class":"Paladin","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":16}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":0,"max":0,"recharge":"short"}}`), Valid: true},
	}

	paladin := refdata.Combatant{
		ID:          paladinCombatantID,
		CharacterID: uuid.NullUUID{UUID: paladinID, Valid: true},
		DisplayName: "Oath",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	svc := NewService(ms)

	_, err := svc.SacredWeapon(ctx, SacredWeaponCommand{
		Paladin: paladin, Turn: refdata.Turn{}, CurrentRound: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no Channel Divinity uses remaining")
}

func TestSacredWeapon_ParseAbilityScoresError(t *testing.T) {
	paladinID := uuid.New()
	paladinCombatantID := uuid.New()

	char := refdata.Character{
		ID:               paladinID,
		Classes:          json.RawMessage(`[{"class":"Paladin","level":3}]`),
		AbilityScores:    json.RawMessage(`{invalid`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	paladin := refdata.Combatant{
		ID:          paladinCombatantID,
		CharacterID: uuid.NullUUID{UUID: paladinID, Valid: true},
		DisplayName: "Oath",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	svc := NewService(ms)

	_, err := svc.SacredWeapon(ctx, SacredWeaponCommand{
		Paladin: paladin, Turn: refdata.Turn{}, CurrentRound: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing ability scores")
}

func TestSacredWeapon_DeductFeatureUseError(t *testing.T) {
	paladinID := uuid.New()
	paladinCombatantID := uuid.New()
	encounterID := uuid.New()

	char := refdata.Character{
		ID:               paladinID,
		Classes:          json.RawMessage(`[{"class":"Paladin","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":16}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	paladin := refdata.Combatant{
		ID:          paladinCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: paladinID, Valid: true},
		DisplayName: "Oath",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{paladin}, nil)
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)

	_, err := svc.SacredWeapon(ctx, SacredWeaponCommand{
		Paladin: paladin, Turn: refdata.Turn{EncounterID: encounterID, CombatantID: paladinCombatantID}, CurrentRound: 1,
	})
	assert.Error(t, err)
}

func TestSacredWeapon_UpdateTurnActionsError(t *testing.T) {
	paladinID := uuid.New()
	paladinCombatantID := uuid.New()
	encounterID := uuid.New()

	char := refdata.Character{
		ID:               paladinID,
		Classes:          json.RawMessage(`[{"class":"Paladin","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":16}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	paladin := refdata.Combatant{
		ID:          paladinCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: paladinID, Valid: true},
		DisplayName: "Oath",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{paladin}, nil)
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)

	_, err := svc.SacredWeapon(ctx, SacredWeaponCommand{
		Paladin: paladin, Turn: refdata.Turn{EncounterID: encounterID, CombatantID: paladinCombatantID}, CurrentRound: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

func TestSacredWeapon_ApplyConditionError(t *testing.T) {
	paladinID := uuid.New()
	paladinCombatantID := uuid.New()
	encounterID := uuid.New()

	char := refdata.Character{
		ID:               paladinID,
		Classes:          json.RawMessage(`[{"class":"Paladin","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":16}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	paladin := refdata.Combatant{
		ID:          paladinCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: paladinID, Valid: true},
		DisplayName: "Oath",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{paladin}, nil)
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)

	_, err := svc.SacredWeapon(ctx, SacredWeaponCommand{
		Paladin: paladin, Turn: refdata.Turn{EncounterID: encounterID, CombatantID: paladinCombatantID}, CurrentRound: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "applying sacred_weapon condition")
}

func TestSacredWeapon_ActionAlreadyUsed(t *testing.T) {
	paladin := refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		DisplayName: "Oath",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	svc := NewService(ms)

	_, err := svc.SacredWeapon(ctx, SacredWeaponCommand{
		Paladin: paladin, Turn: refdata.Turn{ActionUsed: true}, CurrentRound: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource already spent")
}

func TestVowOfEnmity(t *testing.T) {
	paladinID := uuid.New()
	paladinCombatantID := uuid.New()
	targetID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()

	char := refdata.Character{
		ID:               paladinID,
		Classes:          json.RawMessage(`[{"class":"Paladin","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":16}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	paladin := refdata.Combatant{
		ID:          paladinCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: paladinID, Valid: true},
		DisplayName: "Oath",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	target := refdata.Combatant{
		ID:          targetID,
		EncounterID: encounterID,
		DisplayName: "Fiend",
		PositionCol: "B",
		PositionRow: 1, // within 10ft
		HpCurrent:   50,
		HpMax:       50,
		IsNpc:       true,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{paladin, target}, nil)
	svc := NewService(ms)

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: paladinCombatantID}

	result, err := svc.VowOfEnmity(ctx, VowOfEnmityCommand{
		Paladin:      paladin,
		Target:       target,
		Turn:         turn,
		CurrentRound: 1,
	})
	require.NoError(t, err)
	assert.Contains(t, result.CombatLog, "Vow of Enmity")
	assert.Contains(t, result.CombatLog, "Fiend")
	assert.Equal(t, 0, result.UsesLeft)
}

func TestVowOfEnmity_GetCharacterError(t *testing.T) {
	paladin := refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		DisplayName: "Oath",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "Fiend",
		PositionCol: "B",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)

	_, err := svc.VowOfEnmity(ctx, VowOfEnmityCommand{
		Paladin: paladin, Target: target, Turn: refdata.Turn{}, CurrentRound: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

func TestVowOfEnmity_ParseFeatureUsesError(t *testing.T) {
	paladinID := uuid.New()
	char := refdata.Character{
		ID:          paladinID,
		Classes:     json.RawMessage(`[{"class":"Paladin","level":3}]`),
		FeatureUses: pqtype.NullRawMessage{RawMessage: json.RawMessage(`{invalid`), Valid: true},
	}
	paladin := refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: paladinID, Valid: true},
		DisplayName: "Oath",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "Fiend",
		PositionCol: "B",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	svc := NewService(ms)

	_, err := svc.VowOfEnmity(ctx, VowOfEnmityCommand{
		Paladin: paladin, Target: target, Turn: refdata.Turn{}, CurrentRound: 1,
	})
	assert.Error(t, err)
}

func TestVowOfEnmity_ValidateChannelDivinityError(t *testing.T) {
	paladinID := uuid.New()
	char := refdata.Character{
		ID:               paladinID,
		Classes:          json.RawMessage(`[{"class":"Paladin","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":16}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":0,"max":0,"recharge":"short"}}`), Valid: true},
	}
	paladin := refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: paladinID, Valid: true},
		DisplayName: "Oath",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "Fiend",
		PositionCol: "B",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	svc := NewService(ms)

	_, err := svc.VowOfEnmity(ctx, VowOfEnmityCommand{
		Paladin: paladin, Target: target, Turn: refdata.Turn{}, CurrentRound: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no Channel Divinity uses remaining")
}

func TestVowOfEnmity_DeductFeatureUseError(t *testing.T) {
	paladinID := uuid.New()
	paladinCombatantID := uuid.New()
	encounterID := uuid.New()
	targetID := uuid.New()

	char := refdata.Character{
		ID:               paladinID,
		Classes:          json.RawMessage(`[{"class":"Paladin","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":16}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}
	paladin := refdata.Combatant{
		ID:          paladinCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: paladinID, Valid: true},
		DisplayName: "Oath",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          targetID,
		EncounterID: encounterID,
		DisplayName: "Fiend",
		PositionCol: "B",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{paladin, target}, nil)
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)

	_, err := svc.VowOfEnmity(ctx, VowOfEnmityCommand{
		Paladin: paladin, Target: target, Turn: refdata.Turn{EncounterID: encounterID, CombatantID: paladinCombatantID}, CurrentRound: 1,
	})
	assert.Error(t, err)
}

func TestVowOfEnmity_UpdateTurnActionsError(t *testing.T) {
	paladinID := uuid.New()
	paladinCombatantID := uuid.New()
	encounterID := uuid.New()
	targetID := uuid.New()

	char := refdata.Character{
		ID:               paladinID,
		Classes:          json.RawMessage(`[{"class":"Paladin","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":16}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}
	paladin := refdata.Combatant{
		ID:          paladinCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: paladinID, Valid: true},
		DisplayName: "Oath",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          targetID,
		EncounterID: encounterID,
		DisplayName: "Fiend",
		PositionCol: "B",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{paladin, target}, nil)
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)

	_, err := svc.VowOfEnmity(ctx, VowOfEnmityCommand{
		Paladin: paladin, Target: target, Turn: refdata.Turn{EncounterID: encounterID, CombatantID: paladinCombatantID}, CurrentRound: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

func TestVowOfEnmity_ApplyConditionError(t *testing.T) {
	paladinID := uuid.New()
	paladinCombatantID := uuid.New()
	encounterID := uuid.New()
	targetID := uuid.New()

	char := refdata.Character{
		ID:               paladinID,
		Classes:          json.RawMessage(`[{"class":"Paladin","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":16}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}
	paladin := refdata.Combatant{
		ID:          paladinCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: paladinID, Valid: true},
		DisplayName: "Oath",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          targetID,
		EncounterID: encounterID,
		DisplayName: "Fiend",
		PositionCol: "B",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{paladin, target}, nil)
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)

	_, err := svc.VowOfEnmity(ctx, VowOfEnmityCommand{
		Paladin: paladin, Target: target, Turn: refdata.Turn{EncounterID: encounterID, CombatantID: paladinCombatantID}, CurrentRound: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "applying vow_of_enmity condition")
}

func TestVowOfEnmity_ActionAlreadyUsed(t *testing.T) {
	paladin := refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		DisplayName: "Oath",
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "Fiend",
		PositionCol: "B",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	svc := NewService(ms)

	_, err := svc.VowOfEnmity(ctx, VowOfEnmityCommand{
		Paladin: paladin, Target: target, Turn: refdata.Turn{ActionUsed: true}, CurrentRound: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource already spent")
}

func TestVowOfEnmity_OutOfRange(t *testing.T) {
	paladinID := uuid.New()
	paladinCombatantID := uuid.New()
	targetID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()

	char := refdata.Character{
		ID:               paladinID,
		Classes:          json.RawMessage(`[{"class":"Paladin","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":16}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	paladin := refdata.Combatant{
		ID:          paladinCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: paladinID, Valid: true},
		DisplayName: "Oath",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	target := refdata.Combatant{
		ID:          targetID,
		EncounterID: encounterID,
		DisplayName: "Fiend",
		PositionCol: "A",
		PositionRow: 10, // far away (45ft)
		HpCurrent:   50,
		IsNpc:       true,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{paladin, target}, nil)
	svc := NewService(ms)

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: paladinCombatantID}

	_, err := svc.VowOfEnmity(ctx, VowOfEnmityCommand{
		Paladin:      paladin,
		Target:       target,
		Turn:         turn,
		CurrentRound: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

// TDD Cycle 9: Coverage — SpellSaveDC, edge cases

func TestSpellSaveDC(t *testing.T) {
	// DC = 8 + profBonus + abilityMod
	assert.Equal(t, 13, SpellSaveDC(2, 16)) // 8 + 2 + 3
	assert.Equal(t, 14, SpellSaveDC(3, 16)) // 8 + 3 + 3
	assert.Equal(t, 16, SpellSaveDC(4, 18)) // 8 + 4 + 4
	assert.Equal(t, 10, SpellSaveDC(2, 10)) // 8 + 2 + 0
}

func TestTurnUndead_ActionAlreadyUsed(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	encounterID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	cleric := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{cleric}, nil)
	svc := NewService(ms)

	turn := refdata.Turn{
		EncounterID: encounterID,
		CombatantID: clericCombatantID,
		ActionUsed:  true, // already used
	}

	roller := dice.NewRoller(func(max int) int { return 10 })
	_, err := svc.TurnUndead(ctx, TurnUndeadCommand{
		Cleric: cleric, Turn: turn, CurrentRound: 1,
	}, roller)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource already spent")
}

func TestTurnUndead_NotACharacter(t *testing.T) {
	npc := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "NPC",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	svc := NewService(ms)

	roller := dice.NewRoller(func(max int) int { return 10 })
	_, err := svc.TurnUndead(ctx, TurnUndeadCommand{
		Cleric: npc, Turn: refdata.Turn{}, CurrentRound: 1,
	}, roller)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "character (not NPC)")
}

func TestTurnUndead_NoUndeadInRange(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	encounterID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	cleric := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	// Humanoid, not undead
	goblin := refdata.Combatant{
		ID:            uuid.New(),
		EncounterID:   encounterID,
		CreatureRefID: sql.NullString{String: "goblin", Valid: true},
		DisplayName:   "Goblin",
		PositionCol:   "A",
		PositionRow:   3,
		HpCurrent:     7,
		IsNpc:         true,
		IsAlive:       true,
		Conditions:    json.RawMessage(`[]`),
	}

	creatures := map[string]refdata.Creature{
		"goblin": {
			ID:            "goblin",
			Type:          "humanoid",
			Cr:            "1/4",
			AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`),
		},
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{cleric, goblin}, creatures)
	svc := NewService(ms)

	roller := dice.NewRoller(func(max int) int { return 10 })
	result, err := svc.TurnUndead(ctx, TurnUndeadCommand{
		Cleric: cleric, Turn: refdata.Turn{EncounterID: encounterID, CombatantID: clericCombatantID}, CurrentRound: 1,
	}, roller)
	require.NoError(t, err)
	assert.Empty(t, result.Targets)
}

func TestTurnUndead_UndeadOutOfRange(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	encounterID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	cleric := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	// Far away skeleton (row 20 = 95ft away)
	skeleton := refdata.Combatant{
		ID:            uuid.New(),
		EncounterID:   encounterID,
		CreatureRefID: sql.NullString{String: "skeleton-far", Valid: true},
		DisplayName:   "Far Skeleton",
		PositionCol:   "A",
		PositionRow:   20,
		HpCurrent:     13,
		IsNpc:         true,
		IsAlive:       true,
		Conditions:    json.RawMessage(`[]`),
	}

	creatures := map[string]refdata.Creature{
		"skeleton-far": {
			ID:            "skeleton-far",
			Type:          "undead",
			Cr:            "1/4",
			AbilityScores: json.RawMessage(`{"str":10,"dex":14,"con":15,"int":6,"wis":8,"cha":5}`),
		},
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{cleric, skeleton}, creatures)
	svc := NewService(ms)

	roller := dice.NewRoller(func(max int) int { return 5 })
	result, err := svc.TurnUndead(ctx, TurnUndeadCommand{
		Cleric: cleric, Turn: refdata.Turn{EncounterID: encounterID, CombatantID: clericCombatantID}, CurrentRound: 1,
	}, roller)
	require.NoError(t, err)
	assert.Empty(t, result.Targets)
}

func TestTurnUndead_WisSaveWithCreatureSavingThrows(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	skeletonCombatantID := uuid.New()
	encounterID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	cleric := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	skeleton := refdata.Combatant{
		ID:            skeletonCombatantID,
		EncounterID:   encounterID,
		CreatureRefID: sql.NullString{String: "wight", Valid: true},
		DisplayName:   "Wight",
		PositionCol:   "A",
		PositionRow:   3,
		HpCurrent:     45,
		IsNpc:         true,
		IsAlive:       true,
		Conditions:    json.RawMessage(`[]`),
	}

	// Wight with explicit WIS saving throw bonus of +3
	creatures := map[string]refdata.Creature{
		"wight": {
			ID:            "wight",
			Type:          "undead",
			Cr:            "3",
			AbilityScores: json.RawMessage(`{"str":15,"dex":14,"con":16,"int":10,"wis":13,"cha":15}`),
			SavingThrows:  pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"wis":3}`), Valid: true},
		},
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{cleric, skeleton}, creatures)
	svc := NewService(ms)

	// Roll 12 + 3 (explicit WIS save) = 15 >= DC 13. Pass.
	roller := dice.NewRoller(func(max int) int { return 12 })
	result, err := svc.TurnUndead(ctx, TurnUndeadCommand{
		Cleric: cleric, Turn: refdata.Turn{EncounterID: encounterID, CombatantID: clericCombatantID}, CurrentRound: 1,
	}, roller)
	require.NoError(t, err)
	require.Equal(t, 1, len(result.Targets))
	assert.True(t, result.Targets[0].SaveSucceeded)
	assert.Equal(t, 3, result.Targets[0].SaveBonus)
}

func TestPreserveLife_NotACharacter(t *testing.T) {
	npc := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "NPC",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	svc := NewService(ms)

	_, err := svc.PreserveLife(ctx, PreserveLifeCommand{
		Cleric: npc, Turn: refdata.Turn{}, TargetHealing: map[string]int32{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "character (not NPC)")
}

func TestPreserveLife_ActionAlreadyUsed(t *testing.T) {
	cleric := refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		DisplayName: "Thorn",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	svc := NewService(ms)

	_, err := svc.PreserveLife(ctx, PreserveLifeCommand{
		Cleric: cleric, Turn: refdata.Turn{ActionUsed: true}, TargetHealing: map[string]int32{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource already spent")
}

func TestPreserveLife_GetCharacterError(t *testing.T) {
	cleric := refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		DisplayName: "Thorn",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)

	_, err := svc.PreserveLife(ctx, PreserveLifeCommand{
		Cleric: cleric, Turn: refdata.Turn{}, TargetHealing: map[string]int32{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

func TestPreserveLife_ParseFeatureUsesError(t *testing.T) {
	clericID := uuid.New()
	char := refdata.Character{
		ID:          clericID,
		Classes:     json.RawMessage(`[{"class":"Cleric","level":5}]`),
		FeatureUses: pqtype.NullRawMessage{RawMessage: json.RawMessage(`{invalid`), Valid: true},
	}
	cleric := refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	svc := NewService(ms)

	_, err := svc.PreserveLife(ctx, PreserveLifeCommand{
		Cleric: cleric, Turn: refdata.Turn{}, TargetHealing: map[string]int32{},
	})
	assert.Error(t, err)
}

func TestPreserveLife_ValidateChannelDivinityError(t *testing.T) {
	clericID := uuid.New()
	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":5}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 3,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":0,"max":0,"recharge":"short"}}`), Valid: true},
	}
	cleric := refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	svc := NewService(ms)

	_, err := svc.PreserveLife(ctx, PreserveLifeCommand{
		Cleric: cleric, Turn: refdata.Turn{}, TargetHealing: map[string]int32{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no Channel Divinity uses remaining")
}

func TestPreserveLife_DeductFeatureUseError(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	encounterID := uuid.New()
	allyID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":5}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 3,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}
	cleric := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}
	ally := refdata.Combatant{
		ID:          allyID,
		EncounterID: encounterID,
		DisplayName: "Ally",
		PositionCol: "A",
		PositionRow: 3,
		HpCurrent:   5,
		HpMax:       30,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{cleric, ally}, nil)
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)

	_, err := svc.PreserveLife(ctx, PreserveLifeCommand{
		Cleric: cleric, Turn: refdata.Turn{EncounterID: encounterID, CombatantID: clericCombatantID},
		TargetHealing: map[string]int32{allyID.String(): 5},
	})
	assert.Error(t, err)
}

func TestPreserveLife_UpdateTurnActionsError(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	encounterID := uuid.New()
	allyID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":5}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 3,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}
	cleric := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}
	ally := refdata.Combatant{
		ID:          allyID,
		EncounterID: encounterID,
		DisplayName: "Ally",
		PositionCol: "A",
		PositionRow: 3,
		HpCurrent:   5,
		HpMax:       30,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{cleric, ally}, nil)
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)

	_, err := svc.PreserveLife(ctx, PreserveLifeCommand{
		Cleric: cleric, Turn: refdata.Turn{EncounterID: encounterID, CombatantID: clericCombatantID},
		TargetHealing: map[string]int32{allyID.String(): 5},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

func TestSacredWeapon_NotACharacter(t *testing.T) {
	npc := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "NPC",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	svc := NewService(ms)

	_, err := svc.SacredWeapon(ctx, SacredWeaponCommand{
		Paladin: npc, Turn: refdata.Turn{}, CurrentRound: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "character (not NPC)")
}

func TestVowOfEnmity_NotACharacter(t *testing.T) {
	npc := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "NPC",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	svc := NewService(ms)

	_, err := svc.VowOfEnmity(ctx, VowOfEnmityCommand{
		Paladin: npc, Target: refdata.Combatant{}, Turn: refdata.Turn{}, CurrentRound: 1,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "character (not NPC)")
}

func TestChannelDivinityDMQueue_NilNotifier_ReturnsError(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	encounterID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	caster := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{caster}, nil)
	svc := NewService(ms)
	// Deliberately do NOT call svc.SetDMNotifier — dmNotifier is nil.

	turn := refdata.Turn{EncounterID: encounterID, CombatantID: clericCombatantID}

	_, err := svc.ChannelDivinityDMQueue(ctx, ChannelDivinityDMQueueCommand{
		Caster:     caster,
		Turn:       turn,
		OptionName: "Knowledge of the Ages",
		ClassName:  "Cleric",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no DM notifier")
}

func TestChannelDivinityDMQueue_ActionAlreadyUsed(t *testing.T) {
	caster := refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		DisplayName: "Thorn",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	svc := NewService(ms)
	svc.SetDMNotifier(&fakeDMNotifier{})

	_, err := svc.ChannelDivinityDMQueue(ctx, ChannelDivinityDMQueueCommand{
		Caster: caster, Turn: refdata.Turn{ActionUsed: true}, OptionName: "test", ClassName: "Cleric",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource already spent")
}

func TestChannelDivinityDMQueue_GetCharacterError(t *testing.T) {
	caster := refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		DisplayName: "Thorn",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)
	svc.SetDMNotifier(&fakeDMNotifier{})

	_, err := svc.ChannelDivinityDMQueue(ctx, ChannelDivinityDMQueueCommand{
		Caster: caster, Turn: refdata.Turn{}, OptionName: "test", ClassName: "Cleric",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

func TestChannelDivinityDMQueue_ParseFeatureUsesError(t *testing.T) {
	clericID := uuid.New()
	char := refdata.Character{
		ID:          clericID,
		Classes:     json.RawMessage(`[{"class":"Cleric","level":3}]`),
		FeatureUses: pqtype.NullRawMessage{RawMessage: json.RawMessage(`{invalid`), Valid: true},
	}
	caster := refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	svc := NewService(ms)
	svc.SetDMNotifier(&fakeDMNotifier{})

	_, err := svc.ChannelDivinityDMQueue(ctx, ChannelDivinityDMQueueCommand{
		Caster: caster, Turn: refdata.Turn{}, OptionName: "test", ClassName: "Cleric",
	})
	assert.Error(t, err)
}

func TestChannelDivinityDMQueue_ValidateChannelDivinityError(t *testing.T) {
	clericID := uuid.New()
	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":0,"max":0,"recharge":"short"}}`), Valid: true},
	}
	caster := refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	svc := NewService(ms)
	svc.SetDMNotifier(&fakeDMNotifier{})

	_, err := svc.ChannelDivinityDMQueue(ctx, ChannelDivinityDMQueueCommand{
		Caster: caster, Turn: refdata.Turn{}, OptionName: "test", ClassName: "Cleric",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no Channel Divinity uses remaining")
}

func TestChannelDivinityDMQueue_DeductFeatureUseError(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	encounterID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}
	caster := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{caster}, nil)
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)
	svc.SetDMNotifier(&fakeDMNotifier{})

	_, err := svc.ChannelDivinityDMQueue(ctx, ChannelDivinityDMQueueCommand{
		Caster: caster, Turn: refdata.Turn{EncounterID: encounterID, CombatantID: clericCombatantID}, OptionName: "test", ClassName: "Cleric",
	})
	assert.Error(t, err)
}

func TestChannelDivinityDMQueue_UpdateTurnActionsError(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	encounterID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":3}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}
	caster := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{caster}, nil)
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)
	svc.SetDMNotifier(&fakeDMNotifier{})

	_, err := svc.ChannelDivinityDMQueue(ctx, ChannelDivinityDMQueueCommand{
		Caster: caster, Turn: refdata.Turn{EncounterID: encounterID, CombatantID: clericCombatantID}, OptionName: "test", ClassName: "Cleric",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

func TestChannelDivinityDMQueue_NotACharacter(t *testing.T) {
	npc := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "NPC",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	svc := NewService(ms)
	svc.SetDMNotifier(&fakeDMNotifier{})

	_, err := svc.ChannelDivinityDMQueue(ctx, ChannelDivinityDMQueueCommand{
		Caster: npc, Turn: refdata.Turn{}, OptionName: "test", ClassName: "Cleric",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "character (not NPC)")
}

func TestTurnUndead_DestroyUndead_HigherCRNotDestroyed(t *testing.T) {
	clericID := uuid.New()
	clericCombatantID := uuid.New()
	encounterID := uuid.New()

	char := refdata.Character{
		ID:               clericID,
		Classes:          json.RawMessage(`[{"class":"Cleric","level":5}]`),
		AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
		ProficiencyBonus: 3,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
	}

	cleric := refdata.Combatant{
		ID:          clericCombatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
		DisplayName: "Thorn",
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  json.RawMessage(`[]`),
	}

	// CR 1 wight — above 0.5 threshold for level 5, so turned not destroyed
	wight := refdata.Combatant{
		ID:            uuid.New(),
		EncounterID:   encounterID,
		CreatureRefID: sql.NullString{String: "wight-cr1", Valid: true},
		DisplayName:   "Wight",
		PositionCol:   "A",
		PositionRow:   3,
		HpCurrent:     45,
		IsNpc:         true,
		IsAlive:       true,
		Conditions:    json.RawMessage(`[]`),
	}

	creatures := map[string]refdata.Creature{
		"wight-cr1": {
			ID:            "wight-cr1",
			Type:          "undead",
			Cr:            "1",
			AbilityScores: json.RawMessage(`{"str":15,"dex":14,"con":16,"int":10,"wis":8,"cha":15}`),
		},
	}

	ms := newChannelDivinityMockStore(char, []refdata.Combatant{cleric, wight}, creatures)
	svc := NewService(ms)

	// Roll 3, WIS -1 = 2 vs DC 14. Fail, but CR 1 > 0.5 threshold.
	roller := dice.NewRoller(func(max int) int { return 3 })
	result, err := svc.TurnUndead(ctx, TurnUndeadCommand{
		Cleric: cleric, Turn: refdata.Turn{EncounterID: encounterID, CombatantID: clericCombatantID}, CurrentRound: 1,
	}, roller)
	require.NoError(t, err)
	require.Equal(t, 1, len(result.Targets))
	assert.True(t, result.Targets[0].Turned)
	assert.False(t, result.Targets[0].Destroyed)
}

// TDD Cycle 10: resolveTargetWisSave edge cases

func TestResolveTargetWisSave_PCTarget(t *testing.T) {
	charID := uuid.New()
	targetCombatantID := uuid.New()

	target := refdata.Combatant{
		ID:          targetCombatantID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Ally",
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:            charID,
			AbilityScores: json.RawMessage(`{"str":10,"dex":10,"con":10,"int":10,"wis":14,"cha":10}`),
		}, nil
	}
	svc := NewService(ms)

	bonus, err := svc.resolveTargetWisSave(ctx, target)
	require.NoError(t, err)
	assert.Equal(t, 2, bonus) // WIS 14 → +2
}

func TestResolveTargetWisSave_NPCNoCreatureRef(t *testing.T) {
	target := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "NPC",
	}

	ms := defaultMockStore()
	svc := NewService(ms)

	bonus, err := svc.resolveTargetWisSave(ctx, target)
	require.NoError(t, err)
	assert.Equal(t, 0, bonus) // default
}

// TDD Cycle 2: DestroyUndeadCRThreshold

func TestDestroyUndeadCRThreshold(t *testing.T) {
	tests := []struct {
		level    int
		expected float64
		active   bool
	}{
		{4, 0, false},  // below 5 — not available
		{5, 0.5, true}, // CR 1/2 or lower
		{7, 0.5, true},
		{8, 1.0, true},
		{10, 1.0, true},
		{11, 2.0, true},
		{13, 2.0, true},
		{14, 3.0, true},
		{16, 3.0, true},
		{17, 4.0, true},
		{20, 4.0, true},
	}
	for _, tt := range tests {
		threshold, active := DestroyUndeadCRThreshold(tt.level)
		assert.Equal(t, tt.active, active, "level %d active", tt.level)
		if active {
			assert.Equal(t, tt.expected, threshold, "level %d threshold", tt.level)
		}
	}
}
