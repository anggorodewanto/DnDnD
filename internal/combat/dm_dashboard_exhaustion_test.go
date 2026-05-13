package combat

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- SR-019: DM dashboard exhaustion mutation endpoint ---

// TestOverrideCombatantExhaustion_Set verifies that POSTing an absolute
// exhaustion_level (with delta=0) writes that value through the store, logs
// a dm_override entry, and posts a combat-log correction.
func TestOverrideCombatantExhaustion_Set(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	var got refdata.UpdateCombatantConditionsParams
	var logged refdata.CreateActionLogParams
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Aragorn", ExhaustionLevel: 0}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			got = arg
			return refdata.Combatant{ID: arg.ID, ExhaustionLevel: arg.ExhaustionLevel}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			logged = arg
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	poster := &fakeCombatLogPoster{}
	r := newDMDashboardRouterWithPoster(store, poster)

	body := `{"exhaustion_level":3,"reason":"forced march"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/exhaustion", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int32(3), got.ExhaustionLevel)
	assert.Equal(t, "dm_override", logged.ActionType)
	assert.Equal(t, "forced march", logged.Description.String)

	calls := poster.Calls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Message, "Aragorn")
	assert.Contains(t, calls[0].Message, "exhaustion")
}

// TestOverrideCombatantExhaustion_Increment covers the forced-march hook:
// delta=+1 on a combatant currently at level 2 raises them to level 3.
func TestOverrideCombatantExhaustion_Increment(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	var got refdata.UpdateCombatantConditionsParams
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Boromir", ExhaustionLevel: 2}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			got = arg
			return refdata.Combatant{ID: arg.ID, ExhaustionLevel: arg.ExhaustionLevel}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	poster := &fakeCombatLogPoster{}
	r := newDMDashboardRouterWithPoster(store, poster)

	body := `{"delta":1,"reason":"forced march"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/exhaustion", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int32(3), got.ExhaustionLevel)
}

// TestOverrideCombatantExhaustion_DecrementFloor verifies that delta clamps
// at 0 (5e RAW: exhaustion cannot go negative).
func TestOverrideCombatantExhaustion_DecrementFloor(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	var got refdata.UpdateCombatantConditionsParams
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Frodo", ExhaustionLevel: 1}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			got = arg
			return refdata.Combatant{ID: arg.ID, ExhaustionLevel: arg.ExhaustionLevel}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	poster := &fakeCombatLogPoster{}
	r := newDMDashboardRouterWithPoster(store, poster)

	body := `{"delta":-3,"reason":"greater restoration"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/exhaustion", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int32(0), got.ExhaustionLevel)
}

// TestOverrideCombatantExhaustion_ClampToMax verifies that delta clamps at 6
// (exhaustion level 6 = death; higher is meaningless).
func TestOverrideCombatantExhaustion_ClampToMax(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	var got refdata.UpdateCombatantConditionsParams
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Gollum", ExhaustionLevel: 4}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			got = arg
			return refdata.Combatant{ID: arg.ID, ExhaustionLevel: arg.ExhaustionLevel}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	poster := &fakeCombatLogPoster{}
	r := newDMDashboardRouterWithPoster(store, poster)

	body := `{"delta":5,"reason":"overflow"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/exhaustion", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int32(6), got.ExhaustionLevel)
}

// TestOverrideCombatantExhaustion_InvalidBody covers malformed JSON.
func TestOverrideCombatantExhaustion_InvalidBody(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: uuid.New()}, nil
		},
	}
	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/exhaustion", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestOverrideCombatantExhaustion_NoActiveTurn returns 404 when the
// encounter has no active turn (consistent with the other override
// endpoints).
func TestOverrideCombatantExhaustion_NoActiveTurn(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, errNoActiveTurn{}
		},
	}
	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	body := `{"exhaustion_level":1}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/exhaustion", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestOverrideCombatantExhaustion_GetCombatantError returns 500 when the
// store cannot read the target combatant.
func TestOverrideCombatantExhaustion_GetCombatantError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("db down")
		},
	}
	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	body := `{"exhaustion_level":1}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/exhaustion", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestOverrideCombatantExhaustion_UpdateError returns 500 when the persist
// step fails.
func TestOverrideCombatantExhaustion_UpdateError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Sam", ExhaustionLevel: 1}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("update failed")
		},
	}
	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	body := `{"exhaustion_level":2}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/exhaustion", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
