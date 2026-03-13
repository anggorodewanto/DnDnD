package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// newTestRoller creates a dice.Roller that always returns the specified value.
func newTestRoller(value int) *dice.Roller {
	return dice.NewRoller(func(max int) int { return value })
}

// --- TDD Cycle 1: AbilityModifier ---

func TestAbilityModifier(t *testing.T) {
	tests := []struct {
		score    int
		expected int
	}{
		{10, 0},
		{11, 0},
		{12, 1},
		{14, 2},
		{8, -1},
		{7, -2},
		{1, -5},
		{20, 5},
		{9, -1},
	}
	for _, tc := range tests {
		got := AbilityModifier(tc.score)
		assert.Equal(t, tc.expected, got, "AbilityModifier(%d)", tc.score)
	}
}

// --- TDD Cycle 2: ParseAbilityScores ---

func TestParseAbilityScores(t *testing.T) {
	raw := json.RawMessage(`{"str":10,"dex":14,"con":12,"int":8,"wis":13,"cha":15}`)
	scores, err := ParseAbilityScores(raw)
	require.NoError(t, err)
	assert.Equal(t, 10, scores.Str)
	assert.Equal(t, 14, scores.Dex)
	assert.Equal(t, 12, scores.Con)
	assert.Equal(t, 8, scores.Int)
	assert.Equal(t, 13, scores.Wis)
	assert.Equal(t, 15, scores.Cha)
}

func TestParseAbilityScores_InvalidJSON(t *testing.T) {
	_, err := ParseAbilityScores(json.RawMessage(`invalid`))
	require.Error(t, err)
}

func TestParseAbilityScores_EmptyJSON(t *testing.T) {
	scores, err := ParseAbilityScores(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, 0, scores.Dex)
}

// --- TDD Cycle 3: SortByInitiative ---

func TestSortByInitiative_BasicOrdering(t *testing.T) {
	entries := []InitiativeEntry{
		{DisplayName: "Goblin", Roll: 10, DexMod: 1},
		{DisplayName: "Aria", Roll: 18, DexMod: 3},
		{DisplayName: "Barbarian", Roll: 14, DexMod: 0},
	}
	SortByInitiative(entries)
	assert.Equal(t, "Aria", entries[0].DisplayName)
	assert.Equal(t, "Barbarian", entries[1].DisplayName)
	assert.Equal(t, "Goblin", entries[2].DisplayName)
}

func TestSortByInitiative_TieDexBreaker(t *testing.T) {
	entries := []InitiativeEntry{
		{DisplayName: "Slow", Roll: 15, DexMod: 1},
		{DisplayName: "Fast", Roll: 15, DexMod: 3},
	}
	SortByInitiative(entries)
	assert.Equal(t, "Fast", entries[0].DisplayName)
	assert.Equal(t, "Slow", entries[1].DisplayName)
}

func TestSortByInitiative_TieAlphabetical(t *testing.T) {
	entries := []InitiativeEntry{
		{DisplayName: "Zara", Roll: 15, DexMod: 2},
		{DisplayName: "Aria", Roll: 15, DexMod: 2},
	}
	SortByInitiative(entries)
	assert.Equal(t, "Aria", entries[0].DisplayName)
	assert.Equal(t, "Zara", entries[1].DisplayName)
}

// --- TDD Cycle 4: Condition helpers ---

func TestIsSurprised_True(t *testing.T) {
	conds := json.RawMessage(`[{"condition":"surprised","duration_rounds":1,"started_round":0}]`)
	assert.True(t, IsSurprised(conds))
}

func TestIsSurprised_False_Empty(t *testing.T) {
	assert.False(t, IsSurprised(json.RawMessage(`[]`)))
}

func TestIsSurprised_False_OtherCondition(t *testing.T) {
	conds := json.RawMessage(`[{"condition":"poisoned","duration_rounds":10,"started_round":1}]`)
	assert.False(t, IsSurprised(conds))
}

func TestIsSurprised_False_Nil(t *testing.T) {
	assert.False(t, IsSurprised(nil))
}

func TestSurprisedConditionJSON(t *testing.T) {
	cond := SurprisedCondition()
	assert.Equal(t, "surprised", cond.Condition)
	assert.Equal(t, 1, cond.DurationRounds)
	assert.Equal(t, 0, cond.StartedRound)
}

func TestAddSurprisedCondition(t *testing.T) {
	existing := json.RawMessage(`[]`)
	result, err := AddSurprisedCondition(existing)
	require.NoError(t, err)
	assert.True(t, IsSurprised(result))
}

func TestAddSurprisedCondition_PreservesExisting(t *testing.T) {
	existing := json.RawMessage(`[{"condition":"poisoned","duration_rounds":10,"started_round":1}]`)
	result, err := AddSurprisedCondition(existing)
	require.NoError(t, err)
	assert.True(t, IsSurprised(result))
	assert.Contains(t, string(result), "poisoned")
}

func TestRemoveSurprisedCondition(t *testing.T) {
	conds := json.RawMessage(`[{"condition":"surprised","duration_rounds":1,"started_round":0},{"condition":"poisoned","duration_rounds":10,"started_round":1}]`)
	result, err := RemoveSurprisedCondition(conds)
	require.NoError(t, err)
	assert.False(t, IsSurprised(result))
	assert.Contains(t, string(result), "poisoned")
}

// --- TDD Cycle 5: FormatInitiativeTracker ---

func TestFormatInitiativeTracker_Basic(t *testing.T) {
	enc := refdata.Encounter{
		DisplayName: sql.NullString{String: "Rooftop Ambush", Valid: true},
		RoundNumber: 3,
	}
	ariaID := uuid.New()
	combatants := []refdata.Combatant{
		{ID: ariaID, DisplayName: "Aria", InitiativeRoll: 18, InitiativeOrder: 1, IsNpc: false, HpCurrent: 30, HpMax: 45},
		{ID: uuid.New(), DisplayName: "Goblin 1", InitiativeRoll: 10, InitiativeOrder: 2, IsNpc: true, HpCurrent: 7, HpMax: 7},
	}
	result := FormatInitiativeTracker(enc, combatants, ariaID)
	assert.Contains(t, result, "Rooftop Ambush")
	assert.Contains(t, result, "Round 3")
	assert.Contains(t, result, "Aria")
}

func TestFormatInitiativeTracker_NameFallback(t *testing.T) {
	enc := refdata.Encounter{
		Name:        "goblin-ambush",
		DisplayName: sql.NullString{Valid: false},
		RoundNumber: 1,
	}
	id := uuid.New()
	combatants := []refdata.Combatant{
		{ID: id, DisplayName: "Aria", InitiativeRoll: 18, InitiativeOrder: 1, IsNpc: false, HpCurrent: 30, HpMax: 45},
	}
	result := FormatInitiativeTracker(enc, combatants, id)
	assert.Contains(t, result, "goblin-ambush")
}

func TestFormatInitiativeTracker_CurrentTurnHighlighted(t *testing.T) {
	enc := refdata.Encounter{
		DisplayName: sql.NullString{String: "Test", Valid: true},
		RoundNumber: 1,
	}
	ariaID := uuid.New()
	goblinID := uuid.New()
	combatants := []refdata.Combatant{
		{ID: ariaID, DisplayName: "Aria", InitiativeRoll: 18, InitiativeOrder: 1, IsNpc: false, HpCurrent: 30, HpMax: 45},
		{ID: goblinID, DisplayName: "Goblin", InitiativeRoll: 10, InitiativeOrder: 2, IsNpc: true, HpCurrent: 7, HpMax: 7},
	}
	result := FormatInitiativeTracker(enc, combatants, ariaID)
	// Current turn combatant should have the bell marker
	assert.Contains(t, result, "\U0001f514")
}

// --- TDD Cycle 6: RollInitiative ---

func TestService_RollInitiative_Success(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	charID := uuid.New()

	combatant1 := refdata.Combatant{
		ID:            uuid.New(),
		EncounterID:   encounterID,
		DisplayName:   "Aria",
		CharacterID:   uuid.NullUUID{UUID: charID, Valid: true},
		CreatureRefID: sql.NullString{},
		IsNpc:         false,
		Conditions:    json.RawMessage(`[]`),
	}
	combatant2 := refdata.Combatant{
		ID:            uuid.New(),
		EncounterID:   encounterID,
		DisplayName:   "Goblin",
		CharacterID:   uuid.NullUUID{},
		CreatureRefID: sql.NullString{String: "goblin", Valid: true},
		IsNpc:         true,
		Conditions:    json.RawMessage(`[]`),
	}

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "preparing", RoundNumber: 0}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{combatant1, combatant2}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{ID: charID, AbilityScores: json.RawMessage(`{"str":10,"dex":16,"con":12,"int":8,"wis":13,"cha":15}`)}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "goblin", AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`)}, nil
	}

	var updatedInitiatives []refdata.UpdateCombatantInitiativeParams
	store.updateCombatantInitiativeFn = func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
		updatedInitiatives = append(updatedInitiatives, arg)
		return refdata.Combatant{ID: arg.ID, InitiativeRoll: arg.InitiativeRoll, InitiativeOrder: arg.InitiativeOrder}, nil
	}
	store.updateEncounterRoundFn = func(ctx context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID, RoundNumber: arg.RoundNumber}, nil
	}
	store.updateEncounterStatusFn = func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID, Status: arg.Status}, nil
	}

	// Use a deterministic roller that always returns 10
	roller := newTestRoller(10)

	svc := NewService(store)
	combatants, err := svc.RollInitiative(ctx, encounterID, roller)
	require.NoError(t, err)
	require.Len(t, combatants, 2)
	// Both roll 10 on d20. Aria has DEX 16 (+3), total=13. Goblin has DEX 14 (+2), total=12.
	// Aria should be first (higher total).
	require.Len(t, updatedInitiatives, 2)
	assert.Equal(t, int32(1), updatedInitiatives[0].InitiativeOrder)
	assert.Equal(t, int32(2), updatedInitiatives[1].InitiativeOrder)
}

func TestService_RollInitiative_TiebreakByDex(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()

	// Two creatures with same roll but different DEX
	combatant1 := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Slow",
		CreatureRefID: sql.NullString{String: "slow", Valid: true},
		IsNpc: true, Conditions: json.RawMessage(`[]`),
	}
	combatant2 := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Fast",
		CreatureRefID: sql.NullString{String: "fast", Valid: true},
		IsNpc: true, Conditions: json.RawMessage(`[]`),
	}

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "preparing"}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{combatant1, combatant2}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		if id == "slow" {
			return refdata.Creature{ID: "slow", AbilityScores: json.RawMessage(`{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10}`)}, nil
		}
		return refdata.Creature{ID: "fast", AbilityScores: json.RawMessage(`{"str":10,"dex":16,"con":10,"int":10,"wis":10,"cha":10}`)}, nil
	}

	var order []uuid.UUID
	store.updateCombatantInitiativeFn = func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
		order = append(order, arg.ID)
		return refdata.Combatant{ID: arg.ID, InitiativeRoll: arg.InitiativeRoll, InitiativeOrder: arg.InitiativeOrder}, nil
	}

	// Both roll 10 on d20. Slow gets +0 DEX (total 10), Fast gets +3 DEX (total 13).
	// Fast should be first due to higher total.
	svc := NewService(store)
	_, err := svc.RollInitiative(ctx, encounterID, newTestRoller(10))
	require.NoError(t, err)
	// Fast (higher total) should be first
	assert.Equal(t, combatant2.ID, order[0], "Fast should be first")
	assert.Equal(t, combatant1.ID, order[1], "Slow should be second")
}

func TestService_RollInitiative_TiebreakAlphabetical(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()

	combatant1 := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Zara",
		CreatureRefID: sql.NullString{String: "elf", Valid: true},
		IsNpc: true, Conditions: json.RawMessage(`[]`),
	}
	combatant2 := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Aria",
		CreatureRefID: sql.NullString{String: "elf", Valid: true},
		IsNpc: true, Conditions: json.RawMessage(`[]`),
	}

	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{combatant1, combatant2}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "elf", AbilityScores: json.RawMessage(`{"str":10,"dex":14,"con":10,"int":10,"wis":10,"cha":10}`)}, nil
	}

	var order []uuid.UUID
	store.updateCombatantInitiativeFn = func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
		order = append(order, arg.ID)
		return refdata.Combatant{ID: arg.ID, InitiativeRoll: arg.InitiativeRoll, InitiativeOrder: arg.InitiativeOrder}, nil
	}

	svc := NewService(store)
	_, err := svc.RollInitiative(ctx, encounterID, newTestRoller(10))
	require.NoError(t, err)
	// Same roll, same DEX: alphabetical wins. Aria < Zara
	assert.Equal(t, combatant2.ID, order[0], "Aria should be first")
	assert.Equal(t, combatant1.ID, order[1], "Zara should be second")
}

func TestService_RollInitiative_NoCombatants(t *testing.T) {
	ctx := context.Background()
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "preparing"}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{}, nil
	}
	svc := NewService(store)
	_, err := svc.RollInitiative(ctx, uuid.New(), newTestRoller(10))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no combatants")
}

// --- TDD Cycle 7: MarkSurprised ---

func TestService_MarkSurprised_Success(t *testing.T) {
	ctx := context.Background()
	combatantID := uuid.New()
	store := defaultMockStore()
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, Conditions: json.RawMessage(`[]`), ExhaustionLevel: 0}, nil
	}
	var updatedConds json.RawMessage
	store.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		updatedConds = arg.Conditions
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}
	svc := NewService(store)
	err := svc.MarkSurprised(ctx, combatantID)
	require.NoError(t, err)
	assert.True(t, IsSurprised(updatedConds))
}

func TestService_MarkSurprised_GetCombatantError(t *testing.T) {
	ctx := context.Background()
	store := defaultMockStore()
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("db error")
	}
	svc := NewService(store)
	err := svc.MarkSurprised(ctx, uuid.New())
	require.Error(t, err)
}

// --- TDD Cycle 9: RollInitiative error paths ---

func TestService_RollInitiative_ListError(t *testing.T) {
	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return nil, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.RollInitiative(context.Background(), uuid.New(), newTestRoller(10))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing combatants")
}

func TestService_RollInitiative_GetCharacterError(t *testing.T) {
	store := defaultMockStore()
	charID := uuid.New()
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "PC", Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, errors.New("not found")
	}
	svc := NewService(store)
	_, err := svc.RollInitiative(context.Background(), uuid.New(), newTestRoller(10))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

func TestService_RollInitiative_UpdateInitiativeError(t *testing.T) {
	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), CreatureRefID: sql.NullString{String: "goblin", Valid: true}, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{AbilityScores: json.RawMessage(`{"str":10,"dex":14,"con":10,"int":10,"wis":10,"cha":10}`)}, nil
	}
	store.updateCombatantInitiativeFn = func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.RollInitiative(context.Background(), uuid.New(), newTestRoller(10))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating initiative")
}

func TestService_RollInitiative_SetRoundError(t *testing.T) {
	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), CreatureRefID: sql.NullString{String: "goblin", Valid: true}, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{AbilityScores: json.RawMessage(`{"str":10,"dex":14,"con":10,"int":10,"wis":10,"cha":10}`)}, nil
	}
	store.updateEncounterRoundFn = func(ctx context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error) {
		return refdata.Encounter{}, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.RollInitiative(context.Background(), uuid.New(), newTestRoller(10))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "setting round to 1")
}

func TestService_RollInitiative_SetStatusError(t *testing.T) {
	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), CreatureRefID: sql.NullString{String: "goblin", Valid: true}, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{AbilityScores: json.RawMessage(`{"str":10,"dex":14,"con":10,"int":10,"wis":10,"cha":10}`)}, nil
	}
	store.updateEncounterStatusFn = func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
		return refdata.Encounter{}, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.RollInitiative(context.Background(), uuid.New(), newTestRoller(10))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "setting status to active")
}

// --- TDD Cycle 10: AdvanceTurn error paths ---

func TestService_AdvanceTurn_GetEncounterError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{}, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.AdvanceTurn(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting encounter")
}

func TestService_AdvanceTurn_NoCombatants(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{}, nil
	}
	svc := NewService(store)
	_, err := svc.AdvanceTurn(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no combatants")
}

func TestService_AdvanceTurn_CompletesCurrentTurn(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	activeTurnID := uuid.New()
	combatantID := uuid.New()
	nextCombatantID := uuid.New()

	store := defaultMockStore()
	var completedID uuid.UUID
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:            id,
			Status:        "active",
			RoundNumber:   1,
			CurrentTurnID: uuid.NullUUID{UUID: activeTurnID, Valid: true},
		}, nil
	}
	store.completeTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		completedID = id
		return refdata.Turn{ID: id, Status: "completed"}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: combatantID, InitiativeOrder: 1, DisplayName: "A", Conditions: json.RawMessage(`[]`), IsAlive: true},
			{ID: nextCombatantID, InitiativeOrder: 2, DisplayName: "B", Conditions: json.RawMessage(`[]`), IsAlive: true},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{CombatantID: combatantID, Status: "completed"},
		}, nil
	}
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, Status: arg.Status}, nil
	}

	svc := NewService(store)
	turnInfo, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)
	assert.Equal(t, activeTurnID, completedID)
	assert.Equal(t, nextCombatantID, turnInfo.CombatantID)
}

func TestService_AdvanceTurn_SkipsDeadCombatants(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	deadID := uuid.New()
	aliveID := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: deadID, InitiativeOrder: 1, DisplayName: "Dead", Conditions: json.RawMessage(`[]`), IsAlive: false},
			{ID: aliveID, InitiativeOrder: 2, DisplayName: "Alive", Conditions: json.RawMessage(`[]`), IsAlive: true},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, Status: arg.Status}, nil
	}

	svc := NewService(store)
	turnInfo, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)
	assert.Equal(t, aliveID, turnInfo.CombatantID)
}

// --- TDD Cycle 11: getDexModifier edge cases ---

func TestService_getDexModifier_NeitherCharacterNorCreature(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)
	combatant := refdata.Combatant{
		CharacterID:   uuid.NullUUID{},
		CreatureRefID: sql.NullString{},
	}
	mod, err := svc.getDexModifier(context.Background(), combatant)
	require.NoError(t, err)
	assert.Equal(t, 0, mod)
}

func TestService_getDexModifier_BadCreatureAbilityScores(t *testing.T) {
	store := defaultMockStore()
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{AbilityScores: json.RawMessage(`invalid`)}, nil
	}
	svc := NewService(store)
	combatant := refdata.Combatant{
		CreatureRefID: sql.NullString{String: "bad", Valid: true},
	}
	_, err := svc.getDexModifier(context.Background(), combatant)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing creature ability scores")
}

func TestService_AdvanceTurn_AllSurprisedAdvancesRound(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	id1 := uuid.New()
	id2 := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: id1, InitiativeOrder: 1, DisplayName: "S1", Conditions: json.RawMessage(`[{"condition":"surprised","duration_rounds":1,"started_round":0}]`), IsAlive: true},
			{ID: id2, InitiativeOrder: 2, DisplayName: "S2", Conditions: json.RawMessage(`[{"condition":"surprised","duration_rounds":1,"started_round":0}]`), IsAlive: true},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber, Status: arg.Status}, nil
	}
	store.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}
	var newRound int32
	store.updateEncounterRoundFn = func(ctx context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error) {
		newRound = arg.RoundNumber
		return refdata.Encounter{ID: arg.ID, RoundNumber: arg.RoundNumber}, nil
	}

	svc := NewService(store)
	turnInfo, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)
	// Should advance to round 2
	assert.Equal(t, int32(2), newRound)
	assert.Equal(t, id1, turnInfo.CombatantID)
}

func TestService_AdvanceTurn_CompleteTurnError(t *testing.T) {
	store := defaultMockStore()
	turnID := uuid.New()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1, CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true}}, nil
	}
	store.completeTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.AdvanceTurn(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "completing current turn")
}

func TestService_AdvanceTurn_ListTurnsError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), InitiativeOrder: 1, DisplayName: "A", Conditions: json.RawMessage(`[]`), IsAlive: true},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return nil, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.AdvanceTurn(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing turns")
}

func TestService_AdvanceTurn_CreateTurnError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), InitiativeOrder: 1, DisplayName: "A", Conditions: json.RawMessage(`[]`), IsAlive: true},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{}, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.AdvanceTurn(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating turn")
}

func TestService_AdvanceTurn_UpdateCurrentTurnError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), InitiativeOrder: 1, DisplayName: "A", Conditions: json.RawMessage(`[]`), IsAlive: true},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, Status: arg.Status}, nil
	}
	store.updateEncounterCurrentTurnFn = func(ctx context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error) {
		return refdata.Encounter{}, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.AdvanceTurn(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating current turn")
}

func TestService_AdvanceTurn_SkipSurprisedCreateTurnError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), InitiativeOrder: 1, DisplayName: "S", Conditions: json.RawMessage(`[{"condition":"surprised","duration_rounds":1,"started_round":0}]`), IsAlive: true},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{}, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.AdvanceTurn(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating skipped turn")
}

func TestService_AdvanceTurn_SkipSurprisedSkipTurnError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), InitiativeOrder: 1, DisplayName: "S", Conditions: json.RawMessage(`[{"condition":"surprised","duration_rounds":1,"started_round":0}]`), IsAlive: true},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New()}, nil
	}
	store.skipTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.AdvanceTurn(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "skipping surprised turn")
}

func TestService_AdvanceTurn_SkipSurprisedUpdateConditionsError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), InitiativeOrder: 1, DisplayName: "S", Conditions: json.RawMessage(`[{"condition":"surprised","duration_rounds":1,"started_round":0}]`), IsAlive: true},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New()}, nil
	}
	store.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.AdvanceTurn(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating conditions after surprise skip")
}

func TestService_AdvanceTurn_ListCombatantsError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return nil, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.AdvanceTurn(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing combatants")
}

func TestService_AdvanceTurn_RoundAdvanceError(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	c1ID := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: c1ID, InitiativeOrder: 1, DisplayName: "A", Conditions: json.RawMessage(`[]`), IsAlive: true},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{{CombatantID: c1ID, Status: "completed"}}, nil
	}
	store.updateEncounterRoundFn = func(ctx context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error) {
		return refdata.Encounter{}, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.AdvanceTurn(ctx, encounterID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "advancing round")
}

func TestIsSurprised_InvalidJSON(t *testing.T) {
	assert.False(t, IsSurprised(json.RawMessage(`invalid`)))
}

func TestAddSurprisedCondition_InvalidJSON(t *testing.T) {
	_, err := AddSurprisedCondition(json.RawMessage(`invalid`))
	require.Error(t, err)
}

func TestRemoveSurprisedCondition_InvalidJSON(t *testing.T) {
	_, err := RemoveSurprisedCondition(json.RawMessage(`invalid`))
	require.Error(t, err)
}

func TestRemoveSurprisedCondition_Empty(t *testing.T) {
	result, err := RemoveSurprisedCondition(json.RawMessage(`[]`))
	require.NoError(t, err)
	assert.Equal(t, "[]", string(result))
}

func TestService_MarkSurprised_UpdateConditionsError(t *testing.T) {
	store := defaultMockStore()
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, Conditions: json.RawMessage(`[]`)}, nil
	}
	store.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("db error")
	}
	svc := NewService(store)
	err := svc.MarkSurprised(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating conditions")
}

func TestFormatInitiativeTracker_NoPCHP(t *testing.T) {
	enc := refdata.Encounter{
		DisplayName: sql.NullString{String: "Test", Valid: true},
		RoundNumber: 1,
	}
	gobID := uuid.New()
	combatants := []refdata.Combatant{
		{ID: gobID, DisplayName: "Goblin", InitiativeOrder: 1, IsNpc: true, HpCurrent: 7, HpMax: 7},
	}
	// NPC not the current turn — should not show HP
	result := FormatInitiativeTracker(enc, combatants, uuid.New())
	assert.NotContains(t, result, "HP")
}

func TestService_getDexModifier_BadCharacterAbilityScores(t *testing.T) {
	store := defaultMockStore()
	charID := uuid.New()
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{AbilityScores: json.RawMessage(`invalid`)}, nil
	}
	svc := NewService(store)
	combatant := refdata.Combatant{
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
	}
	_, err := svc.getDexModifier(context.Background(), combatant)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing character ability scores")
}

// --- TDD Cycle 8: AdvanceTurn ---

func TestService_AdvanceTurn_FirstTurn(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	combatant1ID := uuid.New()
	combatant2ID := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:          id,
			Status:      "active",
			RoundNumber: 1,
			CurrentTurnID: uuid.NullUUID{},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: combatant1ID, InitiativeOrder: 1, DisplayName: "Aria", Conditions: json.RawMessage(`[]`), IsAlive: true},
			{ID: combatant2ID, InitiativeOrder: 2, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`), IsAlive: true},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}

	var createdTurn refdata.CreateTurnParams
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		createdTurn = arg
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber, Status: arg.Status}, nil
	}

	svc := NewService(store)
	turnInfo, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)
	assert.Equal(t, combatant1ID, createdTurn.CombatantID)
	assert.Equal(t, "active", createdTurn.Status)
	assert.False(t, turnInfo.Skipped)
}

func TestService_AdvanceTurn_SkipsSurprisedInRound1(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	surprisedID := uuid.New()
	normalID := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:          id,
			Status:      "active",
			RoundNumber: 1,
			CurrentTurnID: uuid.NullUUID{},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: surprisedID, InitiativeOrder: 1, DisplayName: "Surprised", Conditions: json.RawMessage(`[{"condition":"surprised","duration_rounds":1,"started_round":0}]`), IsAlive: true},
			{ID: normalID, InitiativeOrder: 2, DisplayName: "Normal", Conditions: json.RawMessage(`[]`), IsAlive: true},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}

	var createdTurns []refdata.CreateTurnParams
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		createdTurns = append(createdTurns, arg)
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber, Status: arg.Status}, nil
	}
	store.skipTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: id, Status: "skipped"}, nil
	}
	// After skipping surprised, the surprised condition should be removed
	store.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(store)
	turnInfo, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)
	// The first combatant is surprised, so it should be skipped and the next non-surprised combatant's turn returned
	assert.Equal(t, normalID, turnInfo.CombatantID)
	assert.False(t, turnInfo.Skipped)
}

func TestService_AdvanceTurn_RoundAdvancement(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	combatant1ID := uuid.New()
	combatant2ID := uuid.New()
	activeTurnID := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:            id,
			Status:        "active",
			RoundNumber:   1,
			CurrentTurnID: uuid.NullUUID{UUID: activeTurnID, Valid: true},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: combatant1ID, InitiativeOrder: 1, DisplayName: "Aria", Conditions: json.RawMessage(`[]`), IsAlive: true},
			{ID: combatant2ID, InitiativeOrder: 2, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`), IsAlive: true},
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: activeTurnID, CombatantID: combatant2ID, RoundNumber: 1, Status: "active"}, nil
	}
	store.completeTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: id, Status: "completed"}, nil
	}
	// Both combatants have gone in round 1
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		if arg.RoundNumber == 1 {
			return []refdata.Turn{
				{CombatantID: combatant1ID, Status: "completed"},
				{CombatantID: combatant2ID, Status: "completed"},
			}, nil
		}
		return []refdata.Turn{}, nil
	}

	var newRound int32
	store.updateEncounterRoundFn = func(ctx context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error) {
		newRound = arg.RoundNumber
		return refdata.Encounter{ID: arg.ID, RoundNumber: arg.RoundNumber}, nil
	}
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber, Status: arg.Status}, nil
	}

	svc := NewService(store)
	turnInfo, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)
	// Should advance to round 2 and start with first combatant
	assert.Equal(t, int32(2), newRound)
	assert.Equal(t, combatant1ID, turnInfo.CombatantID)
}

// --- TDD Cycle 70: FormatCompletedInitiativeTracker ---

func TestFormatCompletedInitiativeTracker_Basic(t *testing.T) {
	enc := refdata.Encounter{
		DisplayName: sql.NullString{String: "Rooftop Ambush", Valid: true},
		RoundNumber: 5,
	}
	combatants := []refdata.Combatant{
		{ID: uuid.New(), DisplayName: "Aria", InitiativeOrder: 1, IsNpc: false, HpCurrent: 30, HpMax: 45},
		{ID: uuid.New(), DisplayName: "Goblin 1", InitiativeOrder: 2, IsNpc: true, HpCurrent: 0, HpMax: 7},
	}

	result := FormatCompletedInitiativeTracker(enc, combatants)

	assert.Contains(t, result, "Rooftop Ambush")
	assert.Contains(t, result, "Round 5")
	assert.Contains(t, result, "Aria")
	assert.Contains(t, result, "Goblin 1")
	assert.Contains(t, result, "--- Combat Complete ---")
	// Should NOT contain the bell indicator (no active turn)
	assert.NotContains(t, result, "\U0001f514")
}

func TestFormatCompletedInitiativeTracker_NameFallback(t *testing.T) {
	enc := refdata.Encounter{
		Name:        "goblin-ambush",
		DisplayName: sql.NullString{Valid: false},
		RoundNumber: 2,
	}
	combatants := []refdata.Combatant{
		{ID: uuid.New(), DisplayName: "Aria", InitiativeOrder: 1, IsNpc: false, HpCurrent: 30, HpMax: 45},
	}

	result := FormatCompletedInitiativeTracker(enc, combatants)

	assert.Contains(t, result, "goblin-ambush")
	assert.Contains(t, result, "--- Combat Complete ---")
}

func TestFormatCompletedInitiativeTracker_PCShowsHP(t *testing.T) {
	enc := refdata.Encounter{
		DisplayName: sql.NullString{String: "Test", Valid: true},
		RoundNumber: 1,
	}
	combatants := []refdata.Combatant{
		{ID: uuid.New(), DisplayName: "Aria", InitiativeOrder: 1, IsNpc: false, HpCurrent: 30, HpMax: 45},
	}

	result := FormatCompletedInitiativeTracker(enc, combatants)

	assert.Contains(t, result, "30/45 HP")
}

func TestFormatCompletedInitiativeTracker_NPCNoHP(t *testing.T) {
	enc := refdata.Encounter{
		DisplayName: sql.NullString{String: "Test", Valid: true},
		RoundNumber: 1,
	}
	combatants := []refdata.Combatant{
		{ID: uuid.New(), DisplayName: "Goblin", InitiativeOrder: 1, IsNpc: true, HpCurrent: 7, HpMax: 7},
	}

	result := FormatCompletedInitiativeTracker(enc, combatants)

	assert.NotContains(t, result, "HP")
}

// --- TDD Cycle: createActiveTurn resolves turn resources ---

func TestService_AdvanceTurn_SetsResourcesForPC(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	charID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:            id,
			Status:        "active",
			RoundNumber:   1,
			CurrentTurnID: uuid.NullUUID{},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{
				ID:              combatantID,
				InitiativeOrder: 1,
				DisplayName:     "Aria",
				Conditions:      json.RawMessage(`[]`),
				IsAlive:         true,
				IsNpc:           false,
				CharacterID:     uuid.NullUUID{UUID: charID, Valid: true},
			},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:      charID,
			SpeedFt: 35,
			Classes: json.RawMessage(`[{"class":"fighter","level":5}]`),
		}, nil
	}
	store.getClassFn = func(ctx context.Context, id string) (refdata.Class, error) {
		return refdata.Class{
			ID:               "fighter",
			AttacksPerAction: json.RawMessage(`{"1":1,"5":2}`),
		}, nil
	}

	var createdParams refdata.CreateTurnParams
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		createdParams = arg
		return refdata.Turn{
			ID:                  uuid.New(),
			CombatantID:         arg.CombatantID,
			RoundNumber:         arg.RoundNumber,
			Status:              arg.Status,
			MovementRemainingFt: arg.MovementRemainingFt,
			AttacksRemaining:    arg.AttacksRemaining,
		}, nil
	}

	svc := NewService(store)
	_, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)

	assert.Equal(t, int32(35), createdParams.MovementRemainingFt, "movement should be character speed")
	assert.Equal(t, int32(2), createdParams.AttacksRemaining, "fighter level 5 gets 2 attacks")
}

func TestService_AdvanceTurn_SetsResourcesForNPC(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:            id,
			Status:        "active",
			RoundNumber:   1,
			CurrentTurnID: uuid.NullUUID{},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{
				ID:              combatantID,
				InitiativeOrder: 1,
				DisplayName:     "Goblin",
				Conditions:      json.RawMessage(`[]`),
				IsAlive:         true,
				IsNpc:           true,
			},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}

	var createdParams refdata.CreateTurnParams
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		createdParams = arg
		return refdata.Turn{
			ID:                  uuid.New(),
			CombatantID:         arg.CombatantID,
			RoundNumber:         arg.RoundNumber,
			Status:              arg.Status,
			MovementRemainingFt: arg.MovementRemainingFt,
			AttacksRemaining:    arg.AttacksRemaining,
		}, nil
	}

	svc := NewService(store)
	_, err := svc.AdvanceTurn(ctx, encounterID)
	require.NoError(t, err)

	assert.Equal(t, int32(30), createdParams.MovementRemainingFt, "NPC defaults to 30ft")
	assert.Equal(t, int32(1), createdParams.AttacksRemaining, "NPC defaults to 1 attack")
}

func TestService_AdvanceTurn_ResolveTurnResourcesError(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	charID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID:            id,
			Status:        "active",
			RoundNumber:   1,
			CurrentTurnID: uuid.NullUUID{},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{
				ID:              combatantID,
				InitiativeOrder: 1,
				DisplayName:     "Aria",
				Conditions:      json.RawMessage(`[]`),
				IsAlive:         true,
				IsNpc:           false,
				CharacterID:     uuid.NullUUID{UUID: charID, Valid: true},
			},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, errors.New("db connection error")
	}

	svc := NewService(store)
	_, err := svc.AdvanceTurn(ctx, encounterID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolving turn resources")
}
