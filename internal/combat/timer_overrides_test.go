package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

func TestSkipTurnNow(t *testing.T) {
	turnID := uuid.New()
	completedCalled := false

	store := defaultMockStore()
	store.completeTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		completedCalled = true
		assert.Equal(t, turnID, id)
		return refdata.Turn{ID: id, Status: "completed"}, nil
	}

	svc := NewService(store)
	err := svc.SkipTurnNow(context.Background(), turnID)
	require.NoError(t, err)
	assert.True(t, completedCalled)
}

func TestExtendTimer(t *testing.T) {
	turnID := uuid.New()
	now := time.Now()
	originalTimeout := now.Add(12 * time.Hour)

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID:        id,
			TimeoutAt: sql.NullTime{Time: originalTimeout, Valid: true},
		}, nil
	}
	var capturedTimeout time.Time
	store.updateTurnTimeoutFn = func(ctx context.Context, arg refdata.UpdateTurnTimeoutParams) (refdata.Turn, error) {
		capturedTimeout = arg.TimeoutAt.Time
		return refdata.Turn{ID: arg.ID, TimeoutAt: arg.TimeoutAt}, nil
	}

	svc := NewService(store)
	err := svc.ExtendTimer(context.Background(), turnID, 6)
	require.NoError(t, err)

	expected := originalTimeout.Add(6 * time.Hour)
	assert.WithinDuration(t, expected, capturedTimeout, time.Second)
}

func TestExtendTimer_NoTimeoutSet(t *testing.T) {
	turnID := uuid.New()

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID:        id,
			TimeoutAt: sql.NullTime{Valid: false},
		}, nil
	}

	svc := NewService(store)
	err := svc.ExtendTimer(context.Background(), turnID, 6)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no timeout set")
}

func TestPauseCombatTimers(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	clearCalled := false

	store := defaultMockStore()
	store.listActiveTurnsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Turn, error) {
		return []refdata.Turn{{ID: turnID, EncounterID: eid}}, nil
	}
	store.clearTurnTimeoutFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		clearCalled = true
		assert.Equal(t, turnID, id)
		return refdata.Turn{ID: id}, nil
	}

	svc := NewService(store)
	err := svc.PauseCombatTimers(context.Background(), encounterID)
	require.NoError(t, err)
	assert.True(t, clearCalled)
}

func TestResumeCombatTimers(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	setCalled := false

	store := defaultMockStore()
	store.listActiveTurnsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Turn, error) {
		return []refdata.Turn{{ID: turnID, EncounterID: eid}}, nil
	}
	store.setTurnTimeoutFn = func(ctx context.Context, arg refdata.SetTurnTimeoutParams) (refdata.Turn, error) {
		setCalled = true
		assert.Equal(t, turnID, arg.ID)
		assert.True(t, arg.TimeoutAt.Valid)
		return refdata.Turn{ID: arg.ID, TimeoutAt: arg.TimeoutAt}, nil
	}

	svc := NewService(store)
	err := svc.ResumeCombatTimers(context.Background(), encounterID, 24)
	require.NoError(t, err)
	assert.True(t, setCalled)
}

func TestSkipTurnNow_CompleteTurnError(t *testing.T) {
	turnID := uuid.New()
	store := defaultMockStore()
	store.completeTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, errors.New("db error")
	}

	svc := NewService(store)
	err := svc.SkipTurnNow(context.Background(), turnID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "completing turn for skip")
}

func TestExtendTimer_GetTurnError(t *testing.T) {
	turnID := uuid.New()
	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, errors.New("db error")
	}

	svc := NewService(store)
	err := svc.ExtendTimer(context.Background(), turnID, 6)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting turn")
}

func TestExtendTimer_UpdateTimeoutError(t *testing.T) {
	turnID := uuid.New()
	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID:        id,
			TimeoutAt: sql.NullTime{Time: time.Now().Add(12 * time.Hour), Valid: true},
		}, nil
	}
	store.updateTurnTimeoutFn = func(ctx context.Context, arg refdata.UpdateTurnTimeoutParams) (refdata.Turn, error) {
		return refdata.Turn{}, errors.New("db error")
	}

	svc := NewService(store)
	err := svc.ExtendTimer(context.Background(), turnID, 6)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn timeout")
}

func TestPauseCombatTimers_ListError(t *testing.T) {
	store := defaultMockStore()
	store.listActiveTurnsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Turn, error) {
		return nil, errors.New("db error")
	}

	svc := NewService(store)
	err := svc.PauseCombatTimers(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing active turns")
}

func TestPauseCombatTimers_ClearError(t *testing.T) {
	store := defaultMockStore()
	store.listActiveTurnsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Turn, error) {
		return []refdata.Turn{{ID: uuid.New()}}, nil
	}
	store.clearTurnTimeoutFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, errors.New("db error")
	}

	svc := NewService(store)
	err := svc.PauseCombatTimers(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "clearing timeout")
}

func TestResumeCombatTimers_ListError(t *testing.T) {
	store := defaultMockStore()
	store.listActiveTurnsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Turn, error) {
		return nil, errors.New("db error")
	}

	svc := NewService(store)
	err := svc.ResumeCombatTimers(context.Background(), uuid.New(), 24)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing active turns")
}

func TestResumeCombatTimers_SetError(t *testing.T) {
	store := defaultMockStore()
	store.listActiveTurnsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Turn, error) {
		return []refdata.Turn{{ID: uuid.New()}}, nil
	}
	store.setTurnTimeoutFn = func(ctx context.Context, arg refdata.SetTurnTimeoutParams) (refdata.Turn, error) {
		return refdata.Turn{}, errors.New("db error")
	}

	svc := NewService(store)
	err := svc.ResumeCombatTimers(context.Background(), uuid.New(), 24)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "setting timeout")
}

func TestPauseCombatTimers_MultipleActiveTurns(t *testing.T) {
	encounterID := uuid.New()
	turnID1 := uuid.New()
	turnID2 := uuid.New()
	cleared := make(map[uuid.UUID]bool)

	store := defaultMockStore()
	store.listActiveTurnsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID1, EncounterID: eid},
			{ID: turnID2, EncounterID: eid},
		}, nil
	}
	store.clearTurnTimeoutFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		cleared[id] = true
		return refdata.Turn{ID: id}, nil
	}

	svc := NewService(store)
	err := svc.PauseCombatTimers(context.Background(), encounterID)
	require.NoError(t, err)
	assert.True(t, cleared[turnID1])
	assert.True(t, cleared[turnID2])
}

// Suppress unused import warning
var _ = json.RawMessage{}
