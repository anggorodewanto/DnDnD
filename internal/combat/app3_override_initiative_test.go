package combat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// APP-3: the initiative-override endpoint uses pointer fields (omit = leave
// unchanged, not silently write 0) and rejects a duplicate initiative_order.

// overrideInitFixture wires a mock store for a single target combatant plus a
// controllable combatant list, and captures the params written to
// UpdateCombatantInitiative.
func overrideInitFixture(t *testing.T, target refdata.Combatant, others []refdata.Combatant) (*mockStore, *refdata.UpdateCombatantInitiativeParams) {
	t.Helper()
	turnID := uuid.New()
	captured := &refdata.UpdateCombatantInitiativeParams{}
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return target, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return append([]refdata.Combatant{target}, others...), nil
		},
		updateCombatantInitiativeFn: func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
			*captured = arg
			return refdata.Combatant{ID: arg.ID, InitiativeRoll: arg.InitiativeRoll, InitiativeOrder: arg.InitiativeOrder}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}
	return store, captured
}

func postOverrideInit(t *testing.T, store *mockStore, combatantID uuid.UUID, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	path := "/api/combat/" + uuid.New().String() + "/override/combatant/" + combatantID.String() + "/initiative"
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// Omitting initiative_order must preserve the combatant's current order rather
// than writing 0 (which jumps it to the front).
func TestOverrideInitiative_OmitOrderLeavesUnchanged(t *testing.T) {
	combatantID := uuid.New()
	target := refdata.Combatant{ID: combatantID, DisplayName: "Fighter", InitiativeRoll: 12, InitiativeOrder: 3}
	store, got := overrideInitFixture(t, target, nil)

	w := postOverrideInit(t, store, combatantID, `{"initiative_roll":18,"reason":"miscounted"}`)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int32(18), got.InitiativeRoll, "roll updated")
	assert.Equal(t, int32(3), got.InitiativeOrder, "order left unchanged, not zeroed")
}

// Omitting both fields is a no-op write of the current values.
func TestOverrideInitiative_OmitBothNoop(t *testing.T) {
	combatantID := uuid.New()
	target := refdata.Combatant{ID: combatantID, DisplayName: "Fighter", InitiativeRoll: 12, InitiativeOrder: 3}
	store, got := overrideInitFixture(t, target, nil)

	w := postOverrideInit(t, store, combatantID, `{"reason":"noop"}`)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int32(12), got.InitiativeRoll)
	assert.Equal(t, int32(3), got.InitiativeOrder)
}

// Setting initiative_order to a value already held by another combatant is a
// 409 conflict, with no write.
func TestOverrideInitiative_DuplicateOrderConflict(t *testing.T) {
	combatantID := uuid.New()
	target := refdata.Combatant{ID: combatantID, DisplayName: "Fighter", InitiativeRoll: 12, InitiativeOrder: 3}
	other := refdata.Combatant{ID: uuid.New(), DisplayName: "Rogue", InitiativeOrder: 2}
	store, got := overrideInitFixture(t, target, []refdata.Combatant{other})

	w := postOverrideInit(t, store, combatantID, `{"initiative_order":2,"reason":"bad"}`)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, int32(0), got.InitiativeOrder, "no write on conflict")
}

// A non-conflicting order change succeeds and writes the requested order.
func TestOverrideInitiative_OrderChangeNoConflict(t *testing.T) {
	combatantID := uuid.New()
	target := refdata.Combatant{ID: combatantID, DisplayName: "Fighter", InitiativeRoll: 12, InitiativeOrder: 3}
	others := []refdata.Combatant{
		{ID: uuid.New(), InitiativeOrder: 1},
		{ID: uuid.New(), InitiativeOrder: 2},
	}
	store, got := overrideInitFixture(t, target, others)

	w := postOverrideInit(t, store, combatantID, `{"initiative_roll":9,"initiative_order":5,"reason":"reseat"}`)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int32(9), got.InitiativeRoll)
	assert.Equal(t, int32(5), got.InitiativeOrder)
}
