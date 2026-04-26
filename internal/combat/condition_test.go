package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- TDD Cycle 1: CombatCondition new fields and AddCondition ---

func TestAddCondition_ToEmptyArray(t *testing.T) {
	cond := CombatCondition{
		Condition:         "frightened",
		DurationRounds:    3,
		StartedRound:      1,
		SourceCombatantID: "abc-123",
		ExpiresOn:         "start_of_turn",
	}
	result, err := AddCondition(json.RawMessage(`[]`), cond)
	require.NoError(t, err)

	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(result, &conds))
	require.Len(t, conds, 1)
	assert.Equal(t, "frightened", conds[0].Condition)
	assert.Equal(t, 3, conds[0].DurationRounds)
	assert.Equal(t, 1, conds[0].StartedRound)
	assert.Equal(t, "abc-123", conds[0].SourceCombatantID)
	assert.Equal(t, "start_of_turn", conds[0].ExpiresOn)
}

func TestAddCondition_ToNilRawMessage(t *testing.T) {
	cond := CombatCondition{Condition: "prone"}
	result, err := AddCondition(nil, cond)
	require.NoError(t, err)

	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(result, &conds))
	require.Len(t, conds, 1)
	assert.Equal(t, "prone", conds[0].Condition)
}

func TestAddCondition_ToExistingArray(t *testing.T) {
	existing := json.RawMessage(`[{"condition":"surprised","duration_rounds":1,"started_round":0}]`)
	cond := CombatCondition{Condition: "grappled", ExpiresOn: "end_of_turn"}
	result, err := AddCondition(existing, cond)
	require.NoError(t, err)

	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(result, &conds))
	require.Len(t, conds, 2)
	assert.Equal(t, "surprised", conds[0].Condition)
	assert.Equal(t, "grappled", conds[1].Condition)
}

func TestAddCondition_InvalidJSON(t *testing.T) {
	_, err := AddCondition(json.RawMessage(`invalid`), CombatCondition{Condition: "prone"})
	require.Error(t, err)
}

// --- TDD Cycle 2: RemoveCondition ---

func TestRemoveCondition_RemovesExisting(t *testing.T) {
	existing := json.RawMessage(`[{"condition":"frightened","duration_rounds":3},{"condition":"prone"}]`)
	result, err := RemoveCondition(existing, "frightened")
	require.NoError(t, err)

	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(result, &conds))
	require.Len(t, conds, 1)
	assert.Equal(t, "prone", conds[0].Condition)
}

func TestRemoveCondition_NotFound(t *testing.T) {
	existing := json.RawMessage(`[{"condition":"prone"}]`)
	result, err := RemoveCondition(existing, "grappled")
	require.NoError(t, err)

	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(result, &conds))
	require.Len(t, conds, 1)
}

func TestRemoveCondition_EmptyArray(t *testing.T) {
	result, err := RemoveCondition(json.RawMessage(`[]`), "prone")
	require.NoError(t, err)

	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(result, &conds))
	require.Len(t, conds, 0)
}

func TestRemoveCondition_InvalidJSON(t *testing.T) {
	_, err := RemoveCondition(json.RawMessage(`invalid`), "prone")
	require.Error(t, err)
}

// --- TDD Cycle 3: HasCondition ---

func TestHasCondition_Found(t *testing.T) {
	raw := json.RawMessage(`[{"condition":"prone"},{"condition":"grappled"}]`)
	assert.True(t, HasCondition(raw, "prone"))
	assert.True(t, HasCondition(raw, "grappled"))
}

func TestHasCondition_NotFound(t *testing.T) {
	raw := json.RawMessage(`[{"condition":"prone"}]`)
	assert.False(t, HasCondition(raw, "grappled"))
}

func TestHasCondition_EmptyAndNil(t *testing.T) {
	assert.False(t, HasCondition(nil, "prone"))
	assert.False(t, HasCondition(json.RawMessage(`[]`), "prone"))
	assert.False(t, HasCondition(json.RawMessage(`invalid`), "prone"))
}

// --- TDD Cycle 4: GetCondition ---

func TestGetCondition_Found(t *testing.T) {
	raw := json.RawMessage(`[{"condition":"frightened","duration_rounds":3,"started_round":1,"source_combatant_id":"src-1","expires_on":"end_of_turn"}]`)
	cond, ok := GetCondition(raw, "frightened")
	require.True(t, ok)
	assert.Equal(t, "frightened", cond.Condition)
	assert.Equal(t, 3, cond.DurationRounds)
	assert.Equal(t, "src-1", cond.SourceCombatantID)
	assert.Equal(t, "end_of_turn", cond.ExpiresOn)
}

func TestGetCondition_NotFound(t *testing.T) {
	raw := json.RawMessage(`[{"condition":"prone"}]`)
	_, ok := GetCondition(raw, "grappled")
	assert.False(t, ok)
}

func TestGetCondition_EmptyAndNil(t *testing.T) {
	_, ok := GetCondition(nil, "prone")
	assert.False(t, ok)
}

// --- TDD Cycle 5: ListConditions ---

func TestListConditions_Multiple(t *testing.T) {
	raw := json.RawMessage(`[{"condition":"prone"},{"condition":"grappled"}]`)
	conds, err := ListConditions(raw)
	require.NoError(t, err)
	require.Len(t, conds, 2)
	assert.Equal(t, "prone", conds[0].Condition)
	assert.Equal(t, "grappled", conds[1].Condition)
}

func TestListConditions_Empty(t *testing.T) {
	conds, err := ListConditions(json.RawMessage(`[]`))
	require.NoError(t, err)
	assert.Len(t, conds, 0)
}

func TestListConditions_Nil(t *testing.T) {
	conds, err := ListConditions(nil)
	require.NoError(t, err)
	assert.Nil(t, conds)
}

func TestListConditions_InvalidJSON(t *testing.T) {
	_, err := ListConditions(json.RawMessage(`invalid`))
	require.Error(t, err)
}

// --- TDD Cycle 6: CheckExpiredConditions ---

func TestCheckExpiredConditions_ExpiresAtStartOfTurn(t *testing.T) {
	sourceID := "source-uuid"
	// Condition started at round 1, lasts 2 rounds, expires at start of source's turn
	conds := json.RawMessage(`[{"condition":"frightened","duration_rounds":2,"started_round":1,"source_combatant_id":"source-uuid","expires_on":"start_of_turn"}]`)

	// Round 3: 1 + 2 = 3, currentRound >= 3, should expire
	updated, expired, err := CheckExpiredConditions(conds, 3, sourceID, "start_of_turn")
	require.NoError(t, err)
	require.Len(t, expired, 1)
	assert.Contains(t, expired[0].Message, "frightened")
	assert.Contains(t, expired[0].Message, "expired")

	var remaining []CombatCondition
	require.NoError(t, json.Unmarshal(updated, &remaining))
	assert.Len(t, remaining, 0)
}

func TestCheckExpiredConditions_NotYetExpired(t *testing.T) {
	sourceID := "source-uuid"
	conds := json.RawMessage(`[{"condition":"frightened","duration_rounds":2,"started_round":1,"source_combatant_id":"source-uuid","expires_on":"start_of_turn"}]`)

	// Round 2: 1 + 2 = 3, currentRound 2 < 3, should NOT expire
	updated, expired, err := CheckExpiredConditions(conds, 2, sourceID, "start_of_turn")
	require.NoError(t, err)
	assert.Len(t, expired, 0)

	var remaining []CombatCondition
	require.NoError(t, json.Unmarshal(updated, &remaining))
	assert.Len(t, remaining, 1)
}

func TestCheckExpiredConditions_WrongSource(t *testing.T) {
	conds := json.RawMessage(`[{"condition":"frightened","duration_rounds":2,"started_round":1,"source_combatant_id":"source-uuid","expires_on":"start_of_turn"}]`)

	// Different source combatant — should NOT expire even though round matches
	updated, expired, err := CheckExpiredConditions(conds, 3, "other-uuid", "start_of_turn")
	require.NoError(t, err)
	assert.Len(t, expired, 0)

	var remaining []CombatCondition
	require.NoError(t, json.Unmarshal(updated, &remaining))
	assert.Len(t, remaining, 1)
}

func TestCheckExpiredConditions_WrongTiming(t *testing.T) {
	conds := json.RawMessage(`[{"condition":"frightened","duration_rounds":2,"started_round":1,"source_combatant_id":"source-uuid","expires_on":"start_of_turn"}]`)

	// Correct source, correct round, but wrong timing (end_of_turn instead of start_of_turn)
	updated, expired, err := CheckExpiredConditions(conds, 3, "source-uuid", "end_of_turn")
	require.NoError(t, err)
	assert.Len(t, expired, 0)

	var remaining []CombatCondition
	require.NoError(t, json.Unmarshal(updated, &remaining))
	assert.Len(t, remaining, 1)
}

func TestCheckExpiredConditions_IndefiniteNotExpired(t *testing.T) {
	// duration_rounds = 0 means indefinite — should never auto-expire
	conds := json.RawMessage(`[{"condition":"grappled","duration_rounds":0,"started_round":1,"source_combatant_id":"source-uuid","expires_on":"start_of_turn"}]`)

	updated, expired, err := CheckExpiredConditions(conds, 100, "source-uuid", "start_of_turn")
	require.NoError(t, err)
	assert.Len(t, expired, 0)

	var remaining []CombatCondition
	require.NoError(t, json.Unmarshal(updated, &remaining))
	assert.Len(t, remaining, 1)
}

func TestCheckExpiredConditions_EndOfTurnTiming(t *testing.T) {
	sourceID := "source-uuid"
	conds := json.RawMessage(`[{"condition":"hold_person","duration_rounds":3,"started_round":2,"source_combatant_id":"source-uuid","expires_on":"end_of_turn"}]`)

	// Round 5: 2 + 3 = 5, expires at end of turn
	updated, expired, err := CheckExpiredConditions(conds, 5, sourceID, "end_of_turn")
	require.NoError(t, err)
	assert.Len(t, expired, 1)

	var remaining []CombatCondition
	require.NoError(t, json.Unmarshal(updated, &remaining))
	assert.Len(t, remaining, 0)
}

func TestCheckExpiredConditions_MixedConditions(t *testing.T) {
	sourceID := "source-uuid"
	// Two conditions: one should expire, one should not
	conds := json.RawMessage(`[
		{"condition":"frightened","duration_rounds":2,"started_round":1,"source_combatant_id":"source-uuid","expires_on":"start_of_turn"},
		{"condition":"grappled","duration_rounds":0,"started_round":1,"source_combatant_id":"source-uuid"},
		{"condition":"charmed","duration_rounds":3,"started_round":1,"source_combatant_id":"other-uuid","expires_on":"start_of_turn"}
	]`)

	updated, expired, err := CheckExpiredConditions(conds, 3, sourceID, "start_of_turn")
	require.NoError(t, err)
	assert.Len(t, expired, 1) // only frightened expires

	var remaining []CombatCondition
	require.NoError(t, json.Unmarshal(updated, &remaining))
	assert.Len(t, remaining, 2) // grappled and charmed remain
}

func TestCheckExpiredConditions_DefaultTimingStartOfTurn(t *testing.T) {
	sourceID := "source-uuid"
	// No expires_on field — should default to "start_of_turn"
	conds := json.RawMessage(`[{"condition":"blinded","duration_rounds":1,"started_round":1,"source_combatant_id":"source-uuid"}]`)

	updated, expired, err := CheckExpiredConditions(conds, 2, sourceID, "start_of_turn")
	require.NoError(t, err)
	assert.Len(t, expired, 1)

	var remaining []CombatCondition
	require.NoError(t, json.Unmarshal(updated, &remaining))
	assert.Len(t, remaining, 0)
}

func TestCheckExpiredConditions_InvalidJSON(t *testing.T) {
	_, _, err := CheckExpiredConditions(json.RawMessage(`invalid`), 1, "src", "start_of_turn")
	require.Error(t, err)
}

// --- TDD Cycle 7: ApplyCondition service method ---

func conditionMockStore(combatant refdata.Combatant) *mockStore {
	ms := defaultMockStore()
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return combatant, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		c := combatant
		c.Conditions = arg.Conditions
		return c, nil
	}
	return ms
}

func TestApplyCondition_TimedCondition(t *testing.T) {
	combatantID := uuid.New()
	sourceID := uuid.New()
	combatant := refdata.Combatant{
		ID:          combatantID,
		DisplayName: "Goblin",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := conditionMockStore(combatant)
	svc := NewService(ms)

	cond := CombatCondition{
		Condition:         "frightened",
		DurationRounds:    3,
		StartedRound:      1,
		SourceCombatantID: sourceID.String(),
		ExpiresOn:         "start_of_turn",
	}

	updated, msgs, err := svc.ApplyCondition(context.Background(), combatantID, cond)
	require.NoError(t, err)
	assert.True(t, HasCondition(updated.Conditions, "frightened"))
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], "frightened")
	assert.Contains(t, msgs[0], "Goblin")
	assert.Contains(t, msgs[0], "3 rounds")
}

func TestApplyCondition_IndefiniteCondition(t *testing.T) {
	combatantID := uuid.New()
	combatant := refdata.Combatant{
		ID:          combatantID,
		DisplayName: "Fighter",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := conditionMockStore(combatant)
	svc := NewService(ms)

	cond := CombatCondition{
		Condition:      "prone",
		DurationRounds: 0,
	}

	updated, msgs, err := svc.ApplyCondition(context.Background(), combatantID, cond)
	require.NoError(t, err)
	assert.True(t, HasCondition(updated.Conditions, "prone"))
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], "indefinite")
}

// --- TDD Cycle 8: RemoveConditionFromCombatant ---

func TestRemoveConditionFromCombatant_NotFound(t *testing.T) {
	combatantID := uuid.New()
	combatant := refdata.Combatant{
		ID:          combatantID,
		DisplayName: "Fighter",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := conditionMockStore(combatant)
	svc := NewService(ms)

	updated, msgs, err := svc.RemoveConditionFromCombatant(context.Background(), combatantID, "prone")
	require.NoError(t, err)
	assert.False(t, HasCondition(updated.Conditions, "prone"))
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], "removed")
}

func TestRemoveConditionFromCombatant(t *testing.T) {
	combatantID := uuid.New()
	combatant := refdata.Combatant{
		ID:          combatantID,
		DisplayName: "Fighter",
		Conditions:  json.RawMessage(`[{"condition":"prone"}]`),
	}

	ms := conditionMockStore(combatant)
	svc := NewService(ms)

	updated, msgs, err := svc.RemoveConditionFromCombatant(context.Background(), combatantID, "prone")
	require.NoError(t, err)
	assert.False(t, HasCondition(updated.Conditions, "prone"))
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], "prone")
	assert.Contains(t, msgs[0], "removed")
	assert.Contains(t, msgs[0], "Fighter")
}

// --- TDD Cycle 9: ProcessTurnStart ---

func TestProcessTurnStart_ExpiresConditionsOnOtherCombatants(t *testing.T) {
	sourceID := uuid.New()
	targetID := uuid.New()

	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin",
		Conditions: json.RawMessage(fmt.Sprintf(
			`[{"condition":"frightened","duration_rounds":2,"started_round":1,"source_combatant_id":"%s","expires_on":"start_of_turn"}]`,
			sourceID.String(),
		)),
	}
	source := refdata.Combatant{
		ID:          sourceID,
		DisplayName: "Fighter",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{source, target}, nil
	}
	var updatedConditions json.RawMessage
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		updatedConditions = arg.Conditions
		c := target
		c.Conditions = arg.Conditions
		return c, nil
	}

	svc := NewService(ms)
	encounterID := uuid.New()

	msgs, err := svc.ProcessTurnStart(context.Background(), encounterID, source, 3)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], "frightened")
	assert.Contains(t, msgs[0], "expired")
	assert.Contains(t, msgs[0], "Goblin")

	// Verify the condition was removed
	var remaining []CombatCondition
	require.NoError(t, json.Unmarshal(updatedConditions, &remaining))
	assert.Len(t, remaining, 0)
}

func TestProcessTurnStart_NoExpiredConditions(t *testing.T) {
	sourceID := uuid.New()
	source := refdata.Combatant{
		ID:          sourceID,
		DisplayName: "Fighter",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{source}, nil
	}

	svc := NewService(ms)
	msgs, err := svc.ProcessTurnStart(context.Background(), uuid.New(), source, 1)
	require.NoError(t, err)
	assert.Len(t, msgs, 0)
}

// --- TDD Cycle 10: ProcessTurnEnd ---

func TestProcessTurnEnd_ExpiresEndOfTurnConditions(t *testing.T) {
	sourceID := uuid.New()
	targetID := uuid.New()

	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin",
		Conditions: json.RawMessage(fmt.Sprintf(
			`[{"condition":"hold_person","duration_rounds":3,"started_round":2,"source_combatant_id":"%s","expires_on":"end_of_turn"}]`,
			sourceID.String(),
		)),
	}
	source := refdata.Combatant{
		ID:          sourceID,
		DisplayName: "Wizard",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{source, target}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		c := target
		c.Conditions = arg.Conditions
		return c, nil
	}

	svc := NewService(ms)
	msgs, err := svc.ProcessTurnEnd(context.Background(), uuid.New(), sourceID, 5)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], "hold_person")
	assert.Contains(t, msgs[0], "expired")
}

func TestProcessTurnEnd_DoesNotExpireStartOfTurnConditions(t *testing.T) {
	sourceID := uuid.New()
	targetID := uuid.New()

	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin",
		Conditions: json.RawMessage(fmt.Sprintf(
			`[{"condition":"frightened","duration_rounds":2,"started_round":1,"source_combatant_id":"%s","expires_on":"start_of_turn"}]`,
			sourceID.String(),
		)),
	}

	ms := defaultMockStore()
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{target}, nil
	}

	svc := NewService(ms)
	msgs, err := svc.ProcessTurnEnd(context.Background(), uuid.New(), sourceID, 3)
	require.NoError(t, err)
	assert.Len(t, msgs, 0) // start_of_turn conditions don't expire at end of turn
}

// --- TDD Cycle 11: AdvanceTurn integrates ProcessTurnEnd ---

func TestAdvanceTurn_ProcessesTurnEndExpiration(t *testing.T) {
	encounterID := uuid.New()
	sourceID := uuid.New() // current turn combatant whose turn is ending
	targetID := uuid.New()
	turnID := uuid.New()

	// Target has a condition placed by source that should expire at end of source's turn
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin",
		Conditions: json.RawMessage(fmt.Sprintf(
			`[{"condition":"hold_person","duration_rounds":2,"started_round":1,"source_combatant_id":"%s","expires_on":"end_of_turn"}]`,
			sourceID.String(),
		)),
		IsAlive:         true,
		InitiativeOrder: 2,
		IsNpc:           true,
	}
	source := refdata.Combatant{
		ID:              sourceID,
		DisplayName:     "Wizard",
		Conditions:      json.RawMessage(`[]`),
		IsAlive:         true,
		InitiativeOrder: 1,
		CharacterID:     uuid.NullUUID{UUID: uuid.New(), Valid: true},
	}

	ms := defaultMockStore()
	ms.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:            id,
			Status:        "active",
			RoundNumber:   3,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
		}, nil
	}
	ms.completeTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: id, CombatantID: sourceID, Status: "completed"}, nil
	}
	ms.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, CombatantID: sourceID}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{source, target}, nil
	}
	ms.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{{CombatantID: sourceID}}, nil
	}
	var conditionsUpdated bool
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		conditionsUpdated = true
		c := target
		c.Conditions = arg.Conditions
		return c, nil
	}
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{SpeedFt: 30, Classes: json.RawMessage(`[]`)}, nil
	}
	ms.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber}, nil
	}
	ms.updateEncounterCurrentTurnFn = func(ctx context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID}, nil
	}

	svc := NewService(ms)
	info, err := svc.AdvanceTurn(context.Background(), encounterID)
	require.NoError(t, err)
	assert.Equal(t, targetID, info.CombatantID)
	assert.True(t, conditionsUpdated, "conditions should have been updated via ProcessTurnEnd")
}

// --- TDD Cycle 12: createActiveTurn integrates ProcessTurnStart ---

func TestAdvanceTurn_ProcessesTurnStartExpiration(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New() // combatant whose turn is starting
	targetID := uuid.New()

	// Target has condition placed by combatant that expires at start of combatant's turn
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin",
		Conditions: json.RawMessage(fmt.Sprintf(
			`[{"condition":"frightened","duration_rounds":2,"started_round":1,"source_combatant_id":"%s","expires_on":"start_of_turn"}]`,
			combatantID.String(),
		)),
		IsAlive:         true,
		InitiativeOrder: 2,
		IsNpc:           true,
	}
	combatant := refdata.Combatant{
		ID:              combatantID,
		DisplayName:     "Fighter",
		Conditions:      json.RawMessage(`[]`),
		IsAlive:         true,
		InitiativeOrder: 1,
		CharacterID:     uuid.NullUUID{UUID: uuid.New(), Valid: true},
	}

	ms := defaultMockStore()
	ms.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 3}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{combatant, target}, nil
	}
	ms.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil // no turns yet this round
	}
	var conditionsUpdated bool
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		conditionsUpdated = true
		c := target
		c.Conditions = arg.Conditions
		return c, nil
	}
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{SpeedFt: 30, Classes: json.RawMessage(`[]`)}, nil
	}
	ms.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber}, nil
	}
	ms.updateEncounterCurrentTurnFn = func(ctx context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID}, nil
	}

	svc := NewService(ms)
	info, err := svc.AdvanceTurn(context.Background(), encounterID)
	require.NoError(t, err)
	assert.Equal(t, combatantID, info.CombatantID)
	assert.True(t, conditionsUpdated, "conditions should have been updated via ProcessTurnStart")
}

// --- TDD Cycle 13: CheckExpiredConditions returns source_combatant_id ---

func TestCheckExpiredConditions_ReturnsSourceCombatantID(t *testing.T) {
	sourceID := "source-uuid"
	conds := json.RawMessage(`[{"condition":"frightened","duration_rounds":2,"started_round":1,"source_combatant_id":"source-uuid","expires_on":"start_of_turn"}]`)

	updated, expired, err := CheckExpiredConditions(conds, 3, sourceID, "start_of_turn")
	require.NoError(t, err)
	require.Len(t, expired, 1)
	assert.Equal(t, "frightened", expired[0].Condition)
	assert.Equal(t, "source-uuid", expired[0].SourceCombatantID)
	assert.Contains(t, expired[0].Message, "frightened")
	assert.Contains(t, expired[0].Message, "expired")

	var remaining []CombatCondition
	require.NoError(t, json.Unmarshal(updated, &remaining))
	assert.Len(t, remaining, 0)
}

// --- TDD Cycle 14: processExpiredConditions includes source name in message ---

func TestProcessTurnStart_ExpirationMessageIncludesSourceName(t *testing.T) {
	sourceID := uuid.New()
	targetID := uuid.New()

	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin",
		Conditions: json.RawMessage(fmt.Sprintf(
			`[{"condition":"frightened","duration_rounds":2,"started_round":1,"source_combatant_id":"%s","expires_on":"start_of_turn"}]`,
			sourceID.String(),
		)),
	}
	source := refdata.Combatant{
		ID:          sourceID,
		DisplayName: "Fighter",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{source, target}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		c := target
		c.Conditions = arg.Conditions
		return c, nil
	}

	svc := NewService(ms)
	encounterID := uuid.New()

	msgs, err := svc.ProcessTurnStart(context.Background(), encounterID, source, 3)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "⏱️ frightened on Goblin has expired (placed by Fighter).", msgs[0])
}

// --- TDD Cycle 15: processExpiredConditions persists to action_log ---

func TestProcessTurnStart_PersistsToActionLog(t *testing.T) {
	sourceID := uuid.New()
	targetID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()

	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin",
		Conditions: json.RawMessage(fmt.Sprintf(
			`[{"condition":"frightened","duration_rounds":2,"started_round":1,"source_combatant_id":"%s","expires_on":"start_of_turn"}]`,
			sourceID.String(),
		)),
	}
	source := refdata.Combatant{
		ID:          sourceID,
		DisplayName: "Fighter",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{source, target}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		c := target
		c.Conditions = arg.Conditions
		return c, nil
	}
	var loggedEntries []refdata.CreateActionLogParams
	ms.createActionLogFn = func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
		loggedEntries = append(loggedEntries, arg)
		return refdata.ActionLog{ID: uuid.New()}, nil
	}

	svc := NewService(ms)

	msgs, err := svc.ProcessTurnStartWithLog(context.Background(), encounterID, source, 3, turnID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Len(t, loggedEntries, 1)
	assert.Equal(t, uuid.NullUUID{UUID: encounterID, Valid: true}, loggedEntries[0].EncounterID)
	assert.Equal(t, uuid.NullUUID{UUID: turnID, Valid: true}, loggedEntries[0].TurnID)
	assert.Equal(t, "condition_expired", loggedEntries[0].ActionType)
	assert.Contains(t, loggedEntries[0].Description.String, "frightened")
}

func TestProcessTurnEndWithLog_PersistsToActionLog(t *testing.T) {
	sourceID := uuid.New()
	targetID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()

	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin",
		Conditions: json.RawMessage(fmt.Sprintf(
			`[{"condition":"hold_person","duration_rounds":3,"started_round":2,"source_combatant_id":"%s","expires_on":"end_of_turn"}]`,
			sourceID.String(),
		)),
	}
	source := refdata.Combatant{
		ID:          sourceID,
		DisplayName: "Wizard",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := defaultMockStore()
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{source, target}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		c := target
		c.Conditions = arg.Conditions
		return c, nil
	}
	var loggedEntries []refdata.CreateActionLogParams
	ms.createActionLogFn = func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
		loggedEntries = append(loggedEntries, arg)
		return refdata.ActionLog{ID: uuid.New()}, nil
	}

	svc := NewService(ms)

	msgs, err := svc.ProcessTurnEndWithLog(context.Background(), encounterID, sourceID, 5, turnID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Len(t, loggedEntries, 1)
	assert.Equal(t, "condition_expired", loggedEntries[0].ActionType)
}

// --- TDD Cycle 16: ApplyCondition persists to action_log when encounterID/turnID provided ---

func TestApplyConditionWithLog_PersistsToActionLog(t *testing.T) {
	combatantID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()
	combatant := refdata.Combatant{
		ID:          combatantID,
		DisplayName: "Goblin",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := conditionMockStore(combatant)
	var loggedEntries []refdata.CreateActionLogParams
	ms.createActionLogFn = func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
		loggedEntries = append(loggedEntries, arg)
		return refdata.ActionLog{ID: uuid.New()}, nil
	}
	svc := NewService(ms)

	cond := CombatCondition{
		Condition:      "prone",
		DurationRounds: 0,
	}

	_, msgs, err := svc.ApplyConditionWithLog(context.Background(), combatantID, cond, encounterID, turnID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Len(t, loggedEntries, 1)
	assert.Equal(t, "condition_applied", loggedEntries[0].ActionType)
	assert.Equal(t, uuid.NullUUID{UUID: encounterID, Valid: true}, loggedEntries[0].EncounterID)
	assert.Equal(t, uuid.NullUUID{UUID: turnID, Valid: true}, loggedEntries[0].TurnID)
}

// --- TDD Cycle 17: RemoveConditionFromCombatant persists to action_log ---

func TestRemoveConditionWithLog_PersistsToActionLog(t *testing.T) {
	combatantID := uuid.New()
	encounterID := uuid.New()
	turnID := uuid.New()
	combatant := refdata.Combatant{
		ID:          combatantID,
		DisplayName: "Fighter",
		Conditions:  json.RawMessage(`[{"condition":"prone"}]`),
	}

	ms := conditionMockStore(combatant)
	var loggedEntries []refdata.CreateActionLogParams
	ms.createActionLogFn = func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
		loggedEntries = append(loggedEntries, arg)
		return refdata.ActionLog{ID: uuid.New()}, nil
	}
	svc := NewService(ms)

	_, msgs, err := svc.RemoveConditionWithLog(context.Background(), combatantID, "prone", encounterID, turnID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Len(t, loggedEntries, 1)
	assert.Equal(t, "condition_removed", loggedEntries[0].ActionType)
}

// --- Backward compatibility: SurprisedCondition with new fields ---

func TestSurprisedCondition_BackwardCompatible(t *testing.T) {
	sc := SurprisedCondition()
	assert.Equal(t, "surprised", sc.Condition)
	assert.Equal(t, 1, sc.DurationRounds)
	assert.Equal(t, 0, sc.StartedRound)
	assert.Equal(t, "", sc.SourceCombatantID) // empty is fine
	assert.Equal(t, "", sc.ExpiresOn)          // empty defaults to start_of_turn

	// JSON round-trip should work with old-format data
	raw := json.RawMessage(`[{"condition":"surprised","duration_rounds":1,"started_round":0}]`)
	assert.True(t, HasCondition(raw, "surprised"))
	assert.True(t, IsSurprised(raw))
}

// --- Edge cases ---

func TestApplyCondition_GetCombatantError(t *testing.T) {
	ms := defaultMockStore()
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("not found")
	}
	svc := NewService(ms)
	_, _, err := svc.ApplyCondition(context.Background(), uuid.New(), CombatCondition{Condition: "prone"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting combatant")
}

func TestApplyCondition_UpdateError(t *testing.T) {
	combatantID := uuid.New()
	ms := defaultMockStore()
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, Conditions: json.RawMessage(`[]`)}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)
	_, _, err := svc.ApplyCondition(context.Background(), combatantID, CombatCondition{Condition: "prone"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating conditions")
}

func TestRemoveConditionFromCombatant_GetError(t *testing.T) {
	ms := defaultMockStore()
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("not found")
	}
	svc := NewService(ms)
	_, _, err := svc.RemoveConditionFromCombatant(context.Background(), uuid.New(), "prone")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting combatant")
}

func TestRemoveConditionFromCombatant_UpdateError(t *testing.T) {
	combatantID := uuid.New()
	ms := defaultMockStore()
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, Conditions: json.RawMessage(`[{"condition":"prone"}]`)}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)
	_, _, err := svc.RemoveConditionFromCombatant(context.Background(), combatantID, "prone")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating conditions")
}

func TestProcessTurnStart_ListError(t *testing.T) {
	ms := defaultMockStore()
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return nil, fmt.Errorf("db error")
	}
	svc := NewService(ms)
	_, err := svc.ProcessTurnStart(context.Background(), uuid.New(), refdata.Combatant{ID: uuid.New()}, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing combatants")
}

func TestProcessTurnEnd_UpdateError(t *testing.T) {
	sourceID := uuid.New()
	targetID := uuid.New()
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin",
		Conditions: json.RawMessage(fmt.Sprintf(
			`[{"condition":"frightened","duration_rounds":1,"started_round":1,"source_combatant_id":"%s","expires_on":"end_of_turn"}]`,
			sourceID.String(),
		)),
	}
	ms := defaultMockStore()
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{target}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}
	svc := NewService(ms)
	_, err := svc.ProcessTurnEnd(context.Background(), uuid.New(), sourceID, 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating conditions")
}

func TestApplyCondition_WithSourceAndEndOfTurn(t *testing.T) {
	combatantID := uuid.New()
	sourceID := uuid.New()
	combatant := refdata.Combatant{
		ID:          combatantID,
		DisplayName: "Goblin",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := conditionMockStore(combatant)
	svc := NewService(ms)

	cond := CombatCondition{
		Condition:         "hold_person",
		DurationRounds:    3,
		StartedRound:      1,
		SourceCombatantID: sourceID.String(),
		ExpiresOn:         "end_of_turn",
	}

	_, msgs, err := svc.ApplyCondition(context.Background(), combatantID, cond)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], "end")
	assert.Contains(t, msgs[0], "source")
}

func TestApplyCondition_WithoutSource(t *testing.T) {
	combatantID := uuid.New()
	combatant := refdata.Combatant{
		ID:          combatantID,
		DisplayName: "Goblin",
		Conditions:  json.RawMessage(`[]`),
	}

	ms := conditionMockStore(combatant)
	svc := NewService(ms)

	cond := CombatCondition{
		Condition:      "blinded",
		DurationRounds: 2,
		StartedRound:   1,
		ExpiresOn:      "start_of_turn",
	}

	_, msgs, err := svc.ApplyCondition(context.Background(), combatantID, cond)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], "blinded")
	assert.Contains(t, msgs[0], "2 rounds")
	assert.NotContains(t, msgs[0], "source") // no source
}
