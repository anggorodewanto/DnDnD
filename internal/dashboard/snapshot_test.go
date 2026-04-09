package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

type fakeSnapshotStore struct {
	enc        refdata.Encounter
	encErr     error
	combatants []refdata.Combatant
	combErr    error
	turn       refdata.Turn
	turnErr    error
}

func (f *fakeSnapshotStore) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	return f.enc, f.encErr
}

func (f *fakeSnapshotStore) ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return f.combatants, f.combErr
}

func (f *fakeSnapshotStore) GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	return f.turn, f.turnErr
}

func TestBuildEncounterSnapshot_HappyPath(t *testing.T) {
	encID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	turnID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	combID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	store := &fakeSnapshotStore{
		enc: refdata.Encounter{
			ID:            encID,
			Name:          "The Cave",
			Status:        "active",
			RoundNumber:   3,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
		},
		combatants: []refdata.Combatant{
			{ID: combID, EncounterID: encID, DisplayName: "Grog", HpCurrent: 10, HpMax: 15},
		},
		turn: refdata.Turn{ID: turnID, EncounterID: encID, CombatantID: combID, RoundNumber: 3, Status: "active"},
	}

	now := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)
	builder := NewSnapshotBuilder(store, func() time.Time { return now })

	snap, err := builder.Build(context.Background(), encID)
	require.NoError(t, err)

	assert.Equal(t, "encounter_snapshot", snap.Type)
	assert.Equal(t, encID.String(), snap.EncounterID)
	assert.Equal(t, "The Cave", snap.Encounter.Name)
	require.Len(t, snap.Combatants, 1)
	assert.Equal(t, "Grog", snap.Combatants[0].DisplayName)
	require.NotNil(t, snap.CurrentTurn)
	assert.Equal(t, turnID, snap.CurrentTurn.ID)
	assert.Equal(t, now, snap.ServerTime)
}

func TestBuildEncounterSnapshot_NoCurrentTurn(t *testing.T) {
	encID := uuid.New()
	store := &fakeSnapshotStore{
		enc: refdata.Encounter{
			ID:          encID,
			Name:        "Idle",
			Status:      "setup",
			RoundNumber: 0,
		},
		combatants: []refdata.Combatant{},
	}
	builder := NewSnapshotBuilder(store, time.Now)
	snap, err := builder.Build(context.Background(), encID)
	require.NoError(t, err)
	assert.Nil(t, snap.CurrentTurn)
	assert.Equal(t, 0, len(snap.Combatants))
}

func TestBuildEncounterSnapshot_GetEncounterError(t *testing.T) {
	store := &fakeSnapshotStore{encErr: errors.New("db down")}
	builder := NewSnapshotBuilder(store, time.Now)
	_, err := builder.Build(context.Background(), uuid.New())
	assert.Error(t, err)
}

func TestBuildEncounterSnapshot_ListCombatantsError(t *testing.T) {
	store := &fakeSnapshotStore{
		enc:     refdata.Encounter{ID: uuid.New()},
		combErr: errors.New("db down"),
	}
	builder := NewSnapshotBuilder(store, time.Now)
	_, err := builder.Build(context.Background(), uuid.New())
	assert.Error(t, err)
}

func TestBuildEncounterSnapshot_GetTurnError(t *testing.T) {
	turnID := uuid.New()
	store := &fakeSnapshotStore{
		enc: refdata.Encounter{
			ID:            uuid.New(),
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
		},
		turnErr: errors.New("turn missing"),
	}
	builder := NewSnapshotBuilder(store, time.Now)
	_, err := builder.Build(context.Background(), uuid.New())
	assert.Error(t, err)
}

func TestBuildEncounterSnapshot_DefaultClock(t *testing.T) {
	store := &fakeSnapshotStore{enc: refdata.Encounter{ID: uuid.New()}}
	builder := NewSnapshotBuilder(store, nil)
	snap, err := builder.Build(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.False(t, snap.ServerTime.IsZero())
}

func TestEncounterSnapshot_JSONMarshal(t *testing.T) {
	encID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	store := &fakeSnapshotStore{
		enc:        refdata.Encounter{ID: encID, Name: "X"},
		combatants: []refdata.Combatant{},
	}
	builder := NewSnapshotBuilder(store, time.Now)
	snap, err := builder.Build(context.Background(), encID)
	require.NoError(t, err)
	data, err := json.Marshal(snap)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"type":"encounter_snapshot"`)
	assert.Contains(t, string(data), encID.String())
}

func TestPublisher_PublishEncounterSnapshot(t *testing.T) {
	encID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	store := &fakeSnapshotStore{
		enc:        refdata.Encounter{ID: encID, Name: "Alpha"},
		combatants: []refdata.Combatant{},
	}

	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	ch := make(chan []byte, 1)
	hub.Register <- &Client{UserID: "dm", EncounterID: encID.String(), Send: ch}
	time.Sleep(10 * time.Millisecond)

	pub := NewPublisher(hub, NewSnapshotBuilder(store, time.Now))
	err := pub.PublishEncounterSnapshot(context.Background(), encID)
	require.NoError(t, err)

	select {
	case msg := <-ch:
		assert.Contains(t, string(msg), `"type":"encounter_snapshot"`)
		assert.Contains(t, string(msg), encID.String())
	case <-time.After(time.Second):
		t.Fatal("expected encounter snapshot to be pushed")
	}
}

func TestPublisher_PublishEncounterSnapshot_BuildError(t *testing.T) {
	store := &fakeSnapshotStore{encErr: errors.New("db down")}
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()
	pub := NewPublisher(hub, NewSnapshotBuilder(store, time.Now))
	err := pub.PublishEncounterSnapshot(context.Background(), uuid.New())
	assert.Error(t, err)
}

func TestPublisher_NilHub_NoPanic(t *testing.T) {
	store := &fakeSnapshotStore{
		enc:        refdata.Encounter{ID: uuid.New()},
		combatants: []refdata.Combatant{},
	}
	pub := NewPublisher(nil, NewSnapshotBuilder(store, time.Now))
	assert.NotPanics(t, func() {
		_ = pub.PublishEncounterSnapshot(context.Background(), uuid.New())
	})
}

