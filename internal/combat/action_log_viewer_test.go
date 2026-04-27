package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- Helpers ---

func mkLogRow(id uuid.UUID, actionType string, actorID uuid.UUID, round int32, createdAt time.Time) refdata.ListActionLogWithRoundsRow {
	return refdata.ListActionLogWithRoundsRow{
		ID:          id,
		TurnID:      uuid.New(),
		EncounterID: uuid.New(),
		ActionType:  actionType,
		ActorID:     actorID,
		BeforeState: json.RawMessage(`{}`),
		AfterState:  json.RawMessage(`{}`),
		RoundNumber: round,
		CreatedAt:   createdAt,
	}
}

// --- TDD Cycle 1: Service returns all logs enriched, newest-first by default ---

func TestService_ListActionLogForViewer_DefaultNewestFirst(t *testing.T) {
	encounterID := uuid.New()
	actorA := uuid.New()
	actorB := uuid.New()
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()
	t1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 1, 10, 1, 0, 0, time.UTC)
	t3 := time.Date(2026, 1, 1, 10, 2, 0, 0, time.UTC)

	store := defaultMockStore()
	store.listActionLogWithRoundsFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
		assert.Equal(t, encounterID, eid)
		return []refdata.ListActionLogWithRoundsRow{
			mkLogRow(id1, "attack", actorA, 1, t1),
			mkLogRow(id2, "damage", actorB, 1, t2),
			mkLogRow(id3, "dm_override", actorA, 2, t3),
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: actorA, ShortID: "AR", DisplayName: "Aragorn"},
			{ID: actorB, ShortID: "GB", DisplayName: "Goblin"},
		}, nil
	}

	svc := NewService(store)
	entries, err := svc.ListActionLogForViewer(context.Background(), encounterID, ActionLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 3)
	// newest-first default
	assert.Equal(t, id3, entries[0].ID)
	assert.Equal(t, id2, entries[1].ID)
	assert.Equal(t, id1, entries[2].ID)
	// enrichment
	assert.Equal(t, "Aragorn", entries[0].ActorDisplayName)
	assert.Equal(t, "dm_override", entries[0].ActionType)
	assert.True(t, entries[0].IsOverride)
}

// --- TDD Cycle 2: Filter by action type, actor, round, turn + sort asc ---

func TestService_ListActionLogForViewer_Filters(t *testing.T) {
	encounterID := uuid.New()
	actorA := uuid.New()
	actorB := uuid.New()
	targetA := uuid.New()
	turnX := uuid.New()
	turnY := uuid.New()
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()
	id4 := uuid.New()
	t1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 1, 10, 1, 0, 0, time.UTC)
	t3 := time.Date(2026, 1, 1, 10, 2, 0, 0, time.UTC)
	t4 := time.Date(2026, 1, 1, 10, 3, 0, 0, time.UTC)

	rows := []refdata.ListActionLogWithRoundsRow{
		{
			ID: id1, TurnID: turnX, EncounterID: encounterID,
			ActionType: "attack", ActorID: actorA,
			TargetID:    uuid.NullUUID{UUID: targetA, Valid: true},
			BeforeState: json.RawMessage(`{"hp":10}`),
			AfterState:  json.RawMessage(`{"hp":5}`),
			RoundNumber: 1, CreatedAt: t1,
		},
		{
			ID: id2, TurnID: turnX, EncounterID: encounterID,
			ActionType: "damage", ActorID: actorB,
			BeforeState: json.RawMessage(`{}`),
			AfterState:  json.RawMessage(`{}`),
			RoundNumber: 1, CreatedAt: t2,
		},
		{
			ID: id3, TurnID: turnY, EncounterID: encounterID,
			ActionType: "cast", ActorID: actorA,
			BeforeState: json.RawMessage(`{}`),
			AfterState:  json.RawMessage(`{}`),
			RoundNumber: 2, CreatedAt: t3,
		},
		{
			ID: id4, TurnID: turnY, EncounterID: encounterID,
			ActionType: "dm_override", ActorID: actorB,
			TargetID:    uuid.NullUUID{UUID: targetA, Valid: true},
			BeforeState: json.RawMessage(`{}`),
			AfterState:  json.RawMessage(`{}`),
			RoundNumber: 2, CreatedAt: t4,
		},
	}

	store := defaultMockStore()
	store.listActionLogWithRoundsFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
		return rows, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: actorA, ShortID: "AR", DisplayName: "Aragorn"},
			{ID: actorB, ShortID: "GB", DisplayName: "Goblin"},
			{ID: targetA, ShortID: "OR", DisplayName: "Orc"},
		}, nil
	}
	svc := NewService(store)

	// Filter by action type (multi)
	got, err := svc.ListActionLogForViewer(context.Background(), encounterID, ActionLogFilter{
		ActionTypes: []string{"attack", "dm_override"},
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, id4, got[0].ID) // newest first
	assert.Equal(t, id1, got[1].ID)
	// target enrichment
	assert.NotNil(t, got[0].TargetID)
	assert.Equal(t, "Orc", got[0].TargetDisplayName)
	assert.Equal(t, "OR", got[0].TargetShortID)

	// Filter by actor
	got, err = svc.ListActionLogForViewer(context.Background(), encounterID, ActionLogFilter{ActorID: actorA})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, id3, got[0].ID)
	assert.Equal(t, id1, got[1].ID)

	// Filter by target
	got, err = svc.ListActionLogForViewer(context.Background(), encounterID, ActionLogFilter{TargetID: targetA})
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Filter by round
	got, err = svc.ListActionLogForViewer(context.Background(), encounterID, ActionLogFilter{Round: 2})
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Filter by turn
	got, err = svc.ListActionLogForViewer(context.Background(), encounterID, ActionLogFilter{TurnID: turnX})
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Sort asc
	got, err = svc.ListActionLogForViewer(context.Background(), encounterID, ActionLogFilter{SortAsc: true})
	require.NoError(t, err)
	require.Len(t, got, 4)
	assert.Equal(t, id1, got[0].ID)
	assert.Equal(t, id4, got[3].ID)

	// Combined filters: no matches
	got, err = svc.ListActionLogForViewer(context.Background(), encounterID, ActionLogFilter{
		ActionTypes: []string{"attack"},
		ActorID:     actorB,
	})
	require.NoError(t, err)
	assert.Empty(t, got)

	// ActionTypes with empty string is ignored
	got, err = svc.ListActionLogForViewer(context.Background(), encounterID, ActionLogFilter{
		ActionTypes: []string{""},
	})
	require.NoError(t, err)
	assert.Empty(t, got)
}

// --- TDD Cycle 2b: Description and dice_rolls passthrough ---

func TestService_ListActionLogForViewer_DescriptionAndDice(t *testing.T) {
	encounterID := uuid.New()
	actorA := uuid.New()

	store := defaultMockStore()
	store.listActionLogWithRoundsFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
		return []refdata.ListActionLogWithRoundsRow{
			{
				ID:          uuid.New(),
				TurnID:      uuid.New(),
				EncounterID: encounterID,
				ActionType:  "attack",
				ActorID:     actorA,
				Description: sql.NullString{String: "hits orc for 7", Valid: true},
				BeforeState: json.RawMessage(`{"hp":10}`),
				AfterState:  json.RawMessage(`{"hp":3}`),
				DiceRolls:   pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"d20":[17]}`), Valid: true},
				RoundNumber: 1, CreatedAt: time.Now(),
			},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{{ID: actorA, ShortID: "AR", DisplayName: "Aragorn"}}, nil
	}
	svc := NewService(store)
	got, err := svc.ListActionLogForViewer(context.Background(), encounterID, ActionLogFilter{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "hits orc for 7", got[0].Description)
	assert.JSONEq(t, `{"d20":[17]}`, string(got[0].DiceRolls))
	assert.JSONEq(t, `{"hp":10}`, string(got[0].BeforeState))
	assert.JSONEq(t, `{"hp":3}`, string(got[0].AfterState))
}

// --- TDD Cycle 3: Error propagation ---

func TestService_ListActionLogForViewer_StoreErrors(t *testing.T) {
	encounterID := uuid.New()

	t.Run("list action log error", func(t *testing.T) {
		store := defaultMockStore()
		store.listActionLogWithRoundsFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
			return nil, assert.AnError
		}
		svc := NewService(store)
		_, err := svc.ListActionLogForViewer(context.Background(), encounterID, ActionLogFilter{})
		require.Error(t, err)
	})

	t.Run("list combatants error", func(t *testing.T) {
		store := defaultMockStore()
		store.listActionLogWithRoundsFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
			return []refdata.ListActionLogWithRoundsRow{}, nil
		}
		store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return nil, assert.AnError
		}
		svc := NewService(store)
		_, err := svc.ListActionLogForViewer(context.Background(), encounterID, ActionLogFilter{})
		require.Error(t, err)
	})
}
