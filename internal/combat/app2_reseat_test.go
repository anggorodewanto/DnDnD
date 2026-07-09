package combat

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// APP-2: re-seat the active turn without a raw DB write.

// reseatFixture wires a mock store where the active turn is seated on `wrongID`
// and `targetID` (a living NPC in the encounter) is the intended first actor.
func reseatFixture(t *testing.T) (*mockStore, uuid.UUID, uuid.UUID, uuid.UUID, *refdata.ReseatTurnParams) {
	t.Helper()
	encounterID := uuid.New()
	turnID := uuid.New()
	wrongID := uuid.New()
	targetID := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1, CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true}}, nil
	}
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: wrongID, RoundNumber: 1, Status: "active"}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: targetID, EncounterID: encounterID, DisplayName: "Forge", IsAlive: true, IsNpc: true,
			CreatureRefID: sql.NullString{String: "follower", Valid: true},
			Conditions:    json.RawMessage(`[]`),
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		// Only the currently-active (wrongly-seated) row exists this round.
		return []refdata.Turn{{ID: turnID, EncounterID: encounterID, CombatantID: wrongID, RoundNumber: 1, Status: "active"}}, nil
	}
	captured := &refdata.ReseatTurnParams{}
	store.reseatTurnFn = func(ctx context.Context, arg refdata.ReseatTurnParams) (refdata.Turn, error) {
		*captured = arg
		return refdata.Turn{ID: arg.ID, EncounterID: encounterID, CombatantID: arg.CombatantID, RoundNumber: 1, Status: "active", MovementRemainingFt: arg.MovementRemainingFt, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	return store, encounterID, turnID, targetID, captured
}

func TestService_ReseatActiveTurn_Success(t *testing.T) {
	ctx := context.Background()
	store, encounterID, turnID, targetID, captured := reseatFixture(t)

	svc := NewService(store)
	info, err := svc.ReseatActiveTurn(ctx, encounterID, targetID)
	require.NoError(t, err)

	assert.Equal(t, targetID, info.CombatantID, "the target is now the active combatant")
	assert.Equal(t, turnID, captured.ID, "the existing active turn row is reassigned in place")
	assert.Equal(t, targetID, captured.CombatantID)
	assert.Equal(t, int32(30), captured.MovementRemainingFt, "movement reset to the target's speed")
	assert.Equal(t, int32(1), captured.AttacksRemaining)
}

func TestService_ReseatActiveTurn_NoActiveTurn(t *testing.T) {
	ctx := context.Background()
	store, encounterID, _, targetID, _ := reseatFixture(t)
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", CurrentTurnID: uuid.NullUUID{}}, nil
	}
	_, err := NewService(store).ReseatActiveTurn(ctx, encounterID, targetID)
	assert.ErrorIs(t, err, ErrNoActiveTurn)
}

func TestService_ReseatActiveTurn_AlreadyActive(t *testing.T) {
	ctx := context.Background()
	store, encounterID, turnID, targetID, _ := reseatFixture(t)
	// Current turn is already seated on the target.
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: targetID, RoundNumber: 1, Status: "active"}, nil
	}
	_, err := NewService(store).ReseatActiveTurn(ctx, encounterID, targetID)
	assert.ErrorIs(t, err, ErrAlreadyActiveTurn)
}

func TestService_ReseatActiveTurn_CurrentTurnActed(t *testing.T) {
	ctx := context.Background()
	store, encounterID, turnID, targetID, _ := reseatFixture(t)
	wrongID := uuid.New()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: wrongID, RoundNumber: 1, Status: "active", ActionUsed: true}, nil
	}
	_, err := NewService(store).ReseatActiveTurn(ctx, encounterID, targetID)
	assert.ErrorIs(t, err, ErrTurnAlreadyActed)
}

func TestService_ReseatActiveTurn_TargetAlreadyActed(t *testing.T) {
	ctx := context.Background()
	store, encounterID, turnID, targetID, _ := reseatFixture(t)
	wrongID := uuid.New()
	// The target already has a completed turn row this round → double-act guard.
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: wrongID, RoundNumber: 1, Status: "active"},
			{ID: uuid.New(), EncounterID: encounterID, CombatantID: targetID, RoundNumber: 1, Status: "completed"},
		}, nil
	}
	_, err := NewService(store).ReseatActiveTurn(ctx, encounterID, targetID)
	assert.ErrorIs(t, err, ErrCombatantAlreadyActed)
}

func TestService_ReseatActiveTurn_TargetDead(t *testing.T) {
	ctx := context.Background()
	store, encounterID, _, targetID, _ := reseatFixture(t)
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: targetID, EncounterID: encounterID, IsAlive: false, Conditions: json.RawMessage(`[]`)}, nil
	}
	_, err := NewService(store).ReseatActiveTurn(ctx, encounterID, targetID)
	assert.ErrorIs(t, err, ErrCombatantNotAlive)
}

func TestService_ReseatActiveTurn_TargetSharesCasterTurn(t *testing.T) {
	ctx := context.Background()
	store, encounterID, _, targetID, _ := reseatFixture(t)
	// A summoned creature (SummonerID set, initiative_order 0) acts on its
	// summoner's turn and cannot be seated on its own.
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: targetID, EncounterID: encounterID, IsAlive: true, InitiativeOrder: 0,
			SummonerID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
			Conditions: json.RawMessage(`[]`),
		}, nil
	}
	_, err := NewService(store).ReseatActiveTurn(ctx, encounterID, targetID)
	assert.ErrorIs(t, err, ErrCombatantSharesCasterTurn)
}

func TestService_ReseatActiveTurn_TargetInOtherEncounter(t *testing.T) {
	ctx := context.Background()
	store, encounterID, _, targetID, _ := reseatFixture(t)
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: targetID, EncounterID: uuid.New(), IsAlive: true, Conditions: json.RawMessage(`[]`)}, nil
	}
	_, err := NewService(store).ReseatActiveTurn(ctx, encounterID, targetID)
	assert.ErrorIs(t, err, ErrCombatantNotInEncounter)
}

// --- Handler ---

func newReseatRouter(store Store) http.Handler {
	svc := NewService(store)
	h := NewDMDashboardHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

func postSetActiveTurn(t *testing.T, r http.Handler, encounterID string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+encounterID+"/set-active-turn", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestSetActiveTurn_Success(t *testing.T) {
	store, encounterID, _, targetID, _ := reseatFixture(t)
	rec := postSetActiveTurn(t, newReseatRouter(store), encounterID.String(), map[string]any{"combatant_id": targetID.String()})

	require.Equal(t, http.StatusOK, rec.Code)
	var resp turnInfoResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, targetID.String(), resp.CombatantID)
}

func TestSetActiveTurn_InvalidEncounterID(t *testing.T) {
	rec := postSetActiveTurn(t, newReseatRouter(defaultMockStore()), "not-a-uuid", map[string]any{"combatant_id": uuid.New().String()})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSetActiveTurn_InvalidBody(t *testing.T) {
	_, encounterID, _, _, _ := reseatFixture(t)
	rec := postSetActiveTurn(t, newReseatRouter(defaultMockStore()), encounterID.String(), map[string]any{"combatant_id": "not-a-uuid"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSetActiveTurn_Conflict(t *testing.T) {
	store, encounterID, turnID, targetID, _ := reseatFixture(t)
	// Current turn already acted → guard 409.
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: uuid.New(), RoundNumber: 1, Status: "active", ActionUsed: true}, nil
	}
	rec := postSetActiveTurn(t, newReseatRouter(store), encounterID.String(), map[string]any{"combatant_id": targetID.String()})
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestSetActiveTurn_TargetNotFound(t *testing.T) {
	store, encounterID, _, targetID, _ := reseatFixture(t)
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, sql.ErrNoRows
	}
	rec := postSetActiveTurn(t, newReseatRouter(store), encounterID.String(), map[string]any{"combatant_id": targetID.String()})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
