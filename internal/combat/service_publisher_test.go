package combat

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// fakePublisher records PublishEncounterSnapshot calls so tests can verify
// that combat.Service invokes the publisher after DB-committing mutations.
type fakePublisher struct {
	mu        sync.Mutex
	published []uuid.UUID
	err       error
}

func (f *fakePublisher) PublishEncounterSnapshot(_ context.Context, encounterID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published = append(f.published, encounterID)
	return f.err
}

func (f *fakePublisher) calls() []uuid.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]uuid.UUID, len(f.published))
	copy(out, f.published)
	return out
}

func TestService_UpdateEncounterStatus_PublishesSnapshot(t *testing.T) {
	encID := uuid.New()
	store := &mockStore{
		updateEncounterStatusFn: func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
			return refdata.Encounter{ID: arg.ID, Status: arg.Status}, nil
		},
	}
	pub := &fakePublisher{}
	svc := NewService(store)
	svc.SetPublisher(pub)

	_, err := svc.UpdateEncounterStatus(context.Background(), encID, "active")
	require.NoError(t, err)

	require.Equal(t, []uuid.UUID{encID}, pub.calls())
}

func TestService_UpdateEncounterStatus_InvalidStatus_NoPublish(t *testing.T) {
	pub := &fakePublisher{}
	svc := NewService(&mockStore{})
	svc.SetPublisher(pub)

	_, err := svc.UpdateEncounterStatus(context.Background(), uuid.New(), "bogus")
	require.Error(t, err)
	assert.Empty(t, pub.calls(), "publish should not fire when validation fails")
}

func TestService_UpdateEncounterStatus_NilPublisherTolerated(t *testing.T) {
	encID := uuid.New()
	store := &mockStore{
		updateEncounterStatusFn: func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
			return refdata.Encounter{ID: arg.ID, Status: arg.Status}, nil
		},
	}
	svc := NewService(store)
	// Intentionally do NOT SetPublisher.

	_, err := svc.UpdateEncounterStatus(context.Background(), encID, "active")
	require.NoError(t, err)
}

func TestService_UpdateEncounterStatus_PublishFailureDoesNotBreakMutation(t *testing.T) {
	encID := uuid.New()
	store := &mockStore{
		updateEncounterStatusFn: func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
			return refdata.Encounter{ID: arg.ID, Status: arg.Status}, nil
		},
	}
	pub := &fakePublisher{err: errors.New("broadcast dropped")}
	svc := NewService(store)
	svc.SetPublisher(pub)

	enc, err := svc.UpdateEncounterStatus(context.Background(), encID, "active")
	require.NoError(t, err, "publish failure must not surface to caller")
	assert.Equal(t, "active", enc.Status)
	assert.Len(t, pub.calls(), 1)
}

func TestService_UpdateEncounterStatus_StoreError_NoPublish(t *testing.T) {
	store := &mockStore{
		updateEncounterStatusFn: func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
			return refdata.Encounter{}, errors.New("db down")
		},
	}
	pub := &fakePublisher{}
	svc := NewService(store)
	svc.SetPublisher(pub)

	_, err := svc.UpdateEncounterStatus(context.Background(), uuid.New(), "active")
	require.Error(t, err)
	assert.Empty(t, pub.calls())
}

func TestService_UpdateCombatantHP_StoreError_NoPublish(t *testing.T) {
	store := &mockStore{
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("db down")
		},
	}
	pub := &fakePublisher{}
	svc := NewService(store)
	svc.SetPublisher(pub)

	_, err := svc.UpdateCombatantHP(context.Background(), uuid.New(), 1, 0, true)
	require.Error(t, err)
	assert.Empty(t, pub.calls())
}

func TestService_UpdateCombatantConditions_StoreError_NoPublish(t *testing.T) {
	store := &mockStore{
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("db down")
		},
	}
	pub := &fakePublisher{}
	svc := NewService(store)
	svc.SetPublisher(pub)

	_, err := svc.UpdateCombatantConditions(context.Background(), uuid.New(), []byte(`[]`), 0)
	require.Error(t, err)
	assert.Empty(t, pub.calls())
}

func TestService_UpdateCombatantPosition_StoreError_NoPublish(t *testing.T) {
	store := &mockStore{
		updateCombatantPositionFn: func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("db down")
		},
	}
	pub := &fakePublisher{}
	svc := NewService(store)
	svc.SetPublisher(pub)

	_, err := svc.UpdateCombatantPosition(context.Background(), uuid.New(), "A", 1, 0)
	require.Error(t, err)
	assert.Empty(t, pub.calls())
}

func TestService_UpdateCombatantHP_PublishesSnapshot(t *testing.T) {
	encID := uuid.New()
	combatantID := uuid.New()
	store := &mockStore{
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, EncounterID: encID, HpCurrent: arg.HpCurrent}, nil
		},
	}
	pub := &fakePublisher{}
	svc := NewService(store)
	svc.SetPublisher(pub)

	_, err := svc.UpdateCombatantHP(context.Background(), combatantID, 10, 0, true)
	require.NoError(t, err)

	require.Equal(t, []uuid.UUID{encID}, pub.calls())
}

func TestService_UpdateCombatantConditions_PublishesSnapshot(t *testing.T) {
	encID := uuid.New()
	combatantID := uuid.New()
	store := &mockStore{
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, EncounterID: encID, Conditions: arg.Conditions}, nil
		},
	}
	pub := &fakePublisher{}
	svc := NewService(store)
	svc.SetPublisher(pub)

	_, err := svc.UpdateCombatantConditions(context.Background(), combatantID, []byte(`[]`), 0)
	require.NoError(t, err)

	require.Equal(t, []uuid.UUID{encID}, pub.calls())
}

func TestService_EndCombat_PublishesSnapshot(t *testing.T) {
	encID := uuid.New()
	store := &mockStore{
		getEncounterFn: func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: id, Status: "active", RoundNumber: 3}, nil
		},
		deleteEncounterZonesByEncounterIDFn: func(ctx context.Context, encounterID uuid.UUID) error { return nil },
		deleteReactionDeclarationsByEncounterFn: func(ctx context.Context, encounterID uuid.UUID) error { return nil },
		updateEncounterStatusFn: func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
			return refdata.Encounter{ID: arg.ID, Status: arg.Status, RoundNumber: 3}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{}, nil
		},
	}
	pub := &fakePublisher{}
	svc := NewService(store)
	svc.SetPublisher(pub)

	_, err := svc.EndCombat(context.Background(), encID)
	require.NoError(t, err)

	// EndCombat triggers UpdateEncounterStatus internally, so at least one
	// publish is expected. Use ">=1" to avoid over-specifying exact count.
	calls := pub.calls()
	require.NotEmpty(t, calls)
	assert.Equal(t, encID, calls[len(calls)-1])
}

func TestService_UpdateCombatantPosition_PublishesSnapshot(t *testing.T) {
	encID := uuid.New()
	combatantID := uuid.New()
	store := &mockStore{
		updateCombatantPositionFn: func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, EncounterID: encID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow}, nil
		},
	}
	pub := &fakePublisher{}
	svc := NewService(store)
	svc.SetPublisher(pub)

	_, err := svc.UpdateCombatantPosition(context.Background(), combatantID, "B", 2, 0)
	require.NoError(t, err)

	require.Equal(t, []uuid.UUID{encID}, pub.calls())
}
