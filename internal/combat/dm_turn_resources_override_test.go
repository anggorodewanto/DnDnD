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

// --- DM override: turn action economy (turn-resources) ---

// turnResourcesPath builds the endpoint URL under test.
func turnResourcesPath(encounterID, combatantID uuid.UUID) string {
	return "/api/combat/" + encounterID.String() + "/override/combatant/" + combatantID.String() + "/turn-resources"
}

// postTurnResources drives one request through the DM dashboard router and
// returns the recorder so each test asserts only what it cares about.
func postTurnResources(r http.Handler, encounterID, combatantID uuid.UUID, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", turnResourcesPath(encounterID, combatantID), strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// activeTurnStore returns a mockStore whose active turn belongs to combatantID
// and carries the given spent-economy state.
func activeTurnStore(turn refdata.Turn, name string) *mockStore {
	return &mockStore{
		getActiveTurnByEncounterIDFn: func(context.Context, uuid.UUID) (refdata.Turn, error) {
			return turn, nil
		},
		getCombatantFn: func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: name}, nil
		},
		updateTurnActionsFn: func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{ID: arg.ID}, nil
		},
		createActionLogFn: func(context.Context, refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}
}

// TestOverrideCombatantTurnResources_ClearsSpentEconomy is the happy path: the
// DM hands back a fully spent action economy in one call and every requested
// field is persisted through UpdateTurnActions.
func TestOverrideCombatantTurnResources_ClearsSpentEconomy(t *testing.T) {
	encounterID, combatantID, turnID := uuid.New(), uuid.New(), uuid.New()

	turn := refdata.Turn{
		ID: turnID, EncounterID: encounterID, CombatantID: combatantID, Status: "active",
		ActionUsed: true, BonusActionUsed: true, ReactionUsed: true,
		MovementRemainingFt: 0, AttacksRemaining: 0,
	}
	store := activeTurnStore(turn, "Windreth")

	var got refdata.UpdateTurnActionsParams
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		got = arg
		return refdata.Turn{ID: arg.ID}, nil
	}
	var logged refdata.CreateActionLogParams
	store.createActionLogFn = func(_ context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
		logged = arg
		return refdata.ActionLog{ID: uuid.New()}, nil
	}

	poster := &fakeCombatLogPoster{}
	r := newDMDashboardRouterWithPoster(store, poster)

	body := `{"action_used":false,"bonus_action_used":false,"reaction_used":false,` +
		`"movement_remaining_ft":30,"attacks_remaining":1,"reason":"mis-adjudicated grapple"}`
	w := postTurnResources(r, encounterID, combatantID, body)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, turnID, got.ID)
	assert.False(t, got.ActionUsed)
	assert.False(t, got.BonusActionUsed)
	assert.False(t, got.ReactionUsed)
	assert.Equal(t, int32(30), got.MovementRemainingFt)
	assert.Equal(t, int32(1), got.AttacksRemaining)

	assert.Equal(t, "dm_override", logged.ActionType)
	assert.Equal(t, combatantID, logged.ActorID)
	assert.Equal(t, turnID, logged.TurnID)
	// The audit must name every changed resource with its before→after values.
	assert.Contains(t, logged.Description.String, "action_used true→false")
	assert.Contains(t, logged.Description.String, "bonus_action_used true→false")
	assert.Contains(t, logged.Description.String, "reaction_used true→false")
	assert.Contains(t, logged.Description.String, "movement_remaining_ft 0→30")
	assert.Contains(t, logged.Description.String, "attacks_remaining 0→1")
	assert.Contains(t, logged.Description.String, "mis-adjudicated grapple")
	assert.Contains(t, string(logged.BeforeState), `"action_used":true`)
	assert.Contains(t, string(logged.AfterState), `"action_used":false`)

	calls := poster.Calls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Message, "Windreth")
	assert.Contains(t, calls[0].Message, "movement_remaining_ft 0→30")
}

// TestOverrideCombatantTurnResources_PartialUpdateLeavesOthers verifies the
// pointer fields: an omitted field carries the turn's current value through
// untouched rather than being zeroed.
func TestOverrideCombatantTurnResources_PartialUpdateLeavesOthers(t *testing.T) {
	encounterID, combatantID, turnID := uuid.New(), uuid.New(), uuid.New()

	turn := refdata.Turn{
		ID: turnID, EncounterID: encounterID, CombatantID: combatantID, Status: "active",
		ActionUsed: true, BonusActionUsed: true, ReactionUsed: true,
		MovementRemainingFt: 15, AttacksRemaining: 2,
	}
	store := activeTurnStore(turn, "Sabinnet")

	var got refdata.UpdateTurnActionsParams
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		got = arg
		return refdata.Turn{ID: arg.ID}, nil
	}
	var logged refdata.CreateActionLogParams
	store.createActionLogFn = func(_ context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
		logged = arg
		return refdata.ActionLog{ID: uuid.New()}, nil
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	w := postTurnResources(r, encounterID, combatantID, `{"reaction_used":false,"reason":"shield never resolved"}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.False(t, got.ReactionUsed)
	assert.True(t, got.ActionUsed, "omitted action_used must not be zeroed")
	assert.True(t, got.BonusActionUsed, "omitted bonus_action_used must not be zeroed")
	assert.Equal(t, int32(15), got.MovementRemainingFt, "omitted movement must be carried through")
	assert.Equal(t, int32(2), got.AttacksRemaining, "omitted attacks must be carried through")

	// Only the changed resource is named in the audit description.
	assert.Contains(t, logged.Description.String, "reaction_used true→false")
	assert.NotContains(t, logged.Description.String, "bonus_action_used")
	assert.NotContains(t, logged.Description.String, "movement_remaining_ft")
	assert.NotContains(t, logged.Description.String, "attacks_remaining")
}

// TestOverrideCombatantTurnResources_ExplicitTrueIsHonoured checks that setting
// a resource to spent (not just refunding it) works — a DM correcting a missed
// reaction charge.
func TestOverrideCombatantTurnResources_ExplicitTrueIsHonoured(t *testing.T) {
	encounterID, combatantID, turnID := uuid.New(), uuid.New(), uuid.New()

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, Status: "active"}
	store := activeTurnStore(turn, "Windreth")

	var got refdata.UpdateTurnActionsParams
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		got = arg
		return refdata.Turn{ID: arg.ID}, nil
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	w := postTurnResources(r, encounterID, combatantID, `{"reaction_used":true,"reason":"hellish rebuke charged late"}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.True(t, got.ReactionUsed)
}

// TestOverrideCombatantTurnResources_ZeroValuesAreHonoured guards the pointer
// contract for the numeric fields: an explicit 0 must persist as 0, not be
// mistaken for "omitted".
func TestOverrideCombatantTurnResources_ZeroValuesAreHonoured(t *testing.T) {
	encounterID, combatantID, turnID := uuid.New(), uuid.New(), uuid.New()

	turn := refdata.Turn{
		ID: turnID, EncounterID: encounterID, CombatantID: combatantID, Status: "active",
		MovementRemainingFt: 30, AttacksRemaining: 2,
	}
	store := activeTurnStore(turn, "Windreth")

	var got refdata.UpdateTurnActionsParams
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		got = arg
		return refdata.Turn{ID: arg.ID}, nil
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	w := postTurnResources(r, encounterID, combatantID, `{"movement_remaining_ft":0,"attacks_remaining":0,"reason":"restrained"}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, int32(0), got.MovementRemainingFt)
	assert.Equal(t, int32(0), got.AttacksRemaining)
}

// TestOverrideCombatantTurnResources_ReassertingCurrentStateAuditsNoChange:
// re-asserting the state a turn already has is idempotent, not an error, and
// the audit says so plainly rather than listing a phantom diff.
func TestOverrideCombatantTurnResources_ReassertingCurrentStateAuditsNoChange(t *testing.T) {
	encounterID, combatantID, turnID := uuid.New(), uuid.New(), uuid.New()

	turn := refdata.Turn{
		ID: turnID, EncounterID: encounterID, CombatantID: combatantID, Status: "active",
		ActionUsed: true,
	}
	store := activeTurnStore(turn, "Windreth")

	var logged refdata.CreateActionLogParams
	store.createActionLogFn = func(_ context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
		logged = arg
		return refdata.ActionLog{ID: uuid.New()}, nil
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	w := postTurnResources(r, encounterID, combatantID, `{"action_used":true,"reason":"confirming"}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, logged.Description.String, "no change")
}

// TestOverrideCombatantTurnResources_NoFieldsRejected rejects a well-formed body
// that asks for no change at all.
func TestOverrideCombatantTurnResources_NoFieldsRejected(t *testing.T) {
	encounterID, combatantID := uuid.New(), uuid.New()
	store := activeTurnStore(refdata.Turn{CombatantID: combatantID}, "Windreth")

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	w := postTurnResources(r, encounterID, combatantID, `{"reason":"nothing"}`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "at least one")
}

// TestOverrideCombatantTurnResources_MissingReasonRejected: a DM correction to
// the action economy must be auditable, so reason is mandatory.
func TestOverrideCombatantTurnResources_MissingReasonRejected(t *testing.T) {
	encounterID, combatantID := uuid.New(), uuid.New()
	store := activeTurnStore(refdata.Turn{CombatantID: combatantID}, "Windreth")

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	for _, body := range []string{`{"action_used":false}`, `{"action_used":false,"reason":"   "}`} {
		w := postTurnResources(r, encounterID, combatantID, body)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
		assert.Contains(t, w.Body.String(), "reason")
	}
}

// TestOverrideCombatantTurnResources_NegativeValuesRejected covers both numeric
// range checks.
func TestOverrideCombatantTurnResources_NegativeValuesRejected(t *testing.T) {
	encounterID, combatantID := uuid.New(), uuid.New()
	store := activeTurnStore(refdata.Turn{CombatantID: combatantID}, "Windreth")
	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	cases := map[string]string{
		"movement": `{"movement_remaining_ft":-5,"reason":"typo"}`,
		"attacks":  `{"attacks_remaining":-1,"reason":"typo"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			w := postTurnResources(r, encounterID, combatantID, body)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "negative")
		})
	}
}

// TestOverrideCombatantTurnResources_InvalidBody covers malformed JSON.
func TestOverrideCombatantTurnResources_InvalidBody(t *testing.T) {
	encounterID, combatantID := uuid.New(), uuid.New()
	store := activeTurnStore(refdata.Turn{CombatantID: combatantID}, "Windreth")
	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	w := postTurnResources(r, encounterID, combatantID, "not json")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestOverrideCombatantTurnResources_InvalidCombatantID rejects an unparseable
// path parameter before any store call.
func TestOverrideCombatantTurnResources_InvalidCombatantID(t *testing.T) {
	encounterID := uuid.New()
	store := activeTurnStore(refdata.Turn{}, "Windreth")
	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	req := httptest.NewRequest("POST",
		"/api/combat/"+encounterID.String()+"/override/combatant/not-a-uuid/turn-resources",
		strings.NewReader(`{"action_used":false,"reason":"x"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestOverrideCombatantTurnResources_NotActiveCombatant409 is the core safety
// rule: a completed turn's flags are mechanically meaningless (the next turn
// re-initialises them), so the endpoint must refuse rather than silently
// "succeed" against a combatant who is not currently acting.
func TestOverrideCombatantTurnResources_NotActiveCombatant409(t *testing.T) {
	encounterID, combatantID, otherID := uuid.New(), uuid.New(), uuid.New()

	updated := false
	store := activeTurnStore(refdata.Turn{
		ID: uuid.New(), EncounterID: encounterID, CombatantID: otherID, Status: "active",
	}, "Windreth")
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		updated = true
		return refdata.Turn{ID: arg.ID}, nil
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	w := postTurnResources(r, encounterID, combatantID, `{"action_used":false,"reason":"late undo"}`)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.False(t, updated, "must not mutate a turn row that is not the combatant's own active turn")
}

// TestOverrideCombatantTurnResources_NoActiveTurn409 covers an encounter with no
// active turn at all — there is no action economy to correct.
func TestOverrideCombatantTurnResources_NoActiveTurn409(t *testing.T) {
	encounterID, combatantID := uuid.New(), uuid.New()

	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(context.Context, uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, errNoActiveTurn{}
		},
	}
	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	w := postTurnResources(r, encounterID, combatantID, `{"action_used":false,"reason":"late undo"}`)

	assert.Equal(t, http.StatusConflict, w.Code)
}

// TestOverrideCombatantTurnResources_GetCombatantError500 — a real DB failure
// reading the target is a server fault, not a bad request.
func TestOverrideCombatantTurnResources_GetCombatantError500(t *testing.T) {
	encounterID, combatantID := uuid.New(), uuid.New()

	store := activeTurnStore(refdata.Turn{
		ID: uuid.New(), EncounterID: encounterID, CombatantID: combatantID, Status: "active",
	}, "Windreth")
	store.getCombatantFn = func(context.Context, uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("db down")
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	w := postTurnResources(r, encounterID, combatantID, `{"action_used":false,"reason":"x"}`)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestOverrideCombatantTurnResources_UpdateError500 — the persist step failing
// is likewise a 500.
func TestOverrideCombatantTurnResources_UpdateError500(t *testing.T) {
	encounterID, combatantID := uuid.New(), uuid.New()

	store := activeTurnStore(refdata.Turn{
		ID: uuid.New(), EncounterID: encounterID, CombatantID: combatantID, Status: "active",
	}, "Windreth")
	store.updateTurnActionsFn = func(context.Context, refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, errors.New("update failed")
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	w := postTurnResources(r, encounterID, combatantID, `{"action_used":false,"reason":"x"}`)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
