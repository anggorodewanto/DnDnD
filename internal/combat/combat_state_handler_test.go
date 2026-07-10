package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// APP-8: GET /api/combat/{enc}/state returns the tracker snapshot (round, the
// current-turn combatant, and each combatant's initiative/hp/active flag) the
// DM previously derived from Postgres by hand.

func getCombatState(t *testing.T, store *mockStore, encID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	r := newDMDashboardRouter(store)
	req := httptest.NewRequest(http.MethodGet, "/api/combat/"+encID.String()+"/state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCombatState_ReturnsTrackerState(t *testing.T) {
	encID := uuid.New()
	turnID := uuid.New()
	first := uuid.New()
	second := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID: id, Status: "active", RoundNumber: 2,
			CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: first, ShortID: "F1", DisplayName: "Forge", InitiativeRoll: 19, InitiativeOrder: 1, HpMax: 30, HpCurrent: 22, Ac: 16, IsNpc: false, IsAlive: true, PositionCol: "D", PositionRow: 5},
			{ID: second, ShortID: "GOB", DisplayName: "Goblin", InitiativeRoll: 5, InitiativeOrder: 2, HpMax: 7, HpCurrent: 7, Ac: 15, IsNpc: true, IsAlive: true},
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encID, CombatantID: first, RoundNumber: 2, Status: "active"}, nil
	}

	w := getCombatState(t, store, encID)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp combatStateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, encID.String(), resp.EncounterID)
	assert.Equal(t, "active", resp.Status)
	assert.Equal(t, int32(2), resp.RoundNumber)
	require.NotNil(t, resp.CurrentTurnID)
	assert.Equal(t, turnID.String(), *resp.CurrentTurnID)
	require.NotNil(t, resp.CurrentCombatantID)
	assert.Equal(t, first.String(), *resp.CurrentCombatantID)

	require.Len(t, resp.Combatants, 2)
	// ListCombatantsByEncounterID orders by initiative_order ASC.
	assert.Equal(t, first.String(), resp.Combatants[0].ID)
	assert.Equal(t, int32(1), resp.Combatants[0].InitiativeOrder)
	assert.Equal(t, int32(22), resp.Combatants[0].HpCurrent)
	assert.Equal(t, int32(30), resp.Combatants[0].HpMax)
	assert.True(t, resp.Combatants[0].IsActive, "the order-1 combatant is the active turn")
	assert.False(t, resp.Combatants[1].IsActive)
}

func TestCombatState_NoActiveTurn(t *testing.T) {
	encID := uuid.New()
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "preparing", RoundNumber: 0}, nil // CurrentTurnID invalid
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), ShortID: "F1", DisplayName: "Forge", IsAlive: true},
		}, nil
	}
	// A missing (invalid) CurrentTurnID must short-circuit the turn read.
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
		t.Fatalf("must not read the active turn when CurrentTurnID is invalid")
		return refdata.Turn{}, nil
	}

	w := getCombatState(t, store, encID)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp combatStateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Nil(t, resp.CurrentTurnID)
	assert.Nil(t, resp.CurrentCombatantID)
	require.Len(t, resp.Combatants, 1)
	assert.False(t, resp.Combatants[0].IsActive)
}

func TestCombatState_EncounterNotFound(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{}, sql.ErrNoRows
	}
	w := getCombatState(t, store, uuid.New())
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCombatState_ListCombatantsError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active"}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return nil, sql.ErrConnDone
	}
	w := getCombatState(t, store, uuid.New())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// A momentarily-inconsistent active turn (read error) must not fail the whole
// snapshot: the endpoint returns 200 with no active combatant.
func TestCombatState_ActiveTurnReadErrorIsBestEffort(t *testing.T) {
	turnID := uuid.New()
	only := uuid.New()
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 1, CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true}}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{{ID: only, ShortID: "F1", DisplayName: "Forge", IsAlive: true}}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, sql.ErrConnDone
	}

	w := getCombatState(t, store, uuid.New())
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp combatStateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.CurrentTurnID, "the turn pointer still reflects the encounter")
	assert.Nil(t, resp.CurrentCombatantID, "no active combatant when the turn read failed")
	require.Len(t, resp.Combatants, 1)
	assert.False(t, resp.Combatants[0].IsActive)
}

func TestCombatState_InvalidEncounterID(t *testing.T) {
	r := newDMDashboardRouter(defaultMockStore())
	req := httptest.NewRequest(http.MethodGet, "/api/combat/not-a-uuid/state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
