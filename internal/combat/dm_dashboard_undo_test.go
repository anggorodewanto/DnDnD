package combat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// fakeCombatLogPoster records PostCorrection calls.
type fakeCombatLogPoster struct {
	mu    sync.Mutex
	calls []fakeCorrectionCall
}

type fakeCorrectionCall struct {
	EncounterID uuid.UUID
	Message     string
}

func (f *fakeCombatLogPoster) PostCorrection(ctx context.Context, encounterID uuid.UUID, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeCorrectionCall{EncounterID: encounterID, Message: message})
}

func (f *fakeCombatLogPoster) Calls() []fakeCorrectionCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeCorrectionCall{}, f.calls...)
}

// newDMDashboardRouterWithPoster builds a router with a fake combat log poster.
func newDMDashboardRouterWithPoster(store Store, poster CombatLogPoster) http.Handler {
	svc := NewService(store)
	handler := NewDMDashboardHandlerWithDeps(svc, nil, poster)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	return r
}

// --- TDD Cycle 1: Undo Last Action - 404 when nothing to undo ---

func TestUndoLastAction_NoActiveTurn(t *testing.T) {
	encounterID := uuid.New()

	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, errNoActiveTurn{}
		},
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/undo-last-action", strings.NewReader(`{"reason":"oops"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// errNoActiveTurn is a placeholder error type used in tests.
type errNoActiveTurn struct{}

func (errNoActiveTurn) Error() string { return "no active turn" }

// --- TDD Cycle 2: Undo Last Action - 404 when no action_log rows for current turn ---

func TestUndoLastAction_NoActionsThisTurn(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		listActionLogByTurnIDFn: func(ctx context.Context, tid uuid.UUID) ([]refdata.ActionLog, error) {
			return nil, nil
		},
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/undo-last-action", strings.NewReader(`{"reason":"oops"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- TDD Cycle 3: Undo Last Action - restores HP from before_state ---

func TestUndoLastAction_RestoresHP(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	actorID := uuid.New()

	beforeState := json.RawMessage(`{"hp_current":20,"temp_hp":3,"is_alive":true}`)

	logs := []refdata.ActionLog{
		{
			ID:          uuid.New(),
			TurnID:      turnID,
			EncounterID: encounterID,
			ActionType:  "damage",
			ActorID:     actorID,
			BeforeState: beforeState,
		},
	}

	var updatedHP int32
	var updatedTemp int32
	var createdLogs []refdata.CreateActionLogParams

	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		listActionLogByTurnIDFn: func(ctx context.Context, tid uuid.UUID) ([]refdata.ActionLog, error) {
			return logs, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Goblin", HpCurrent: 5, TempHp: 0, IsAlive: true}, nil
		},
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			updatedHP = arg.HpCurrent
			updatedTemp = arg.TempHp
			return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, TempHp: arg.TempHp, IsAlive: arg.IsAlive}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			createdLogs = append(createdLogs, arg)
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	poster := &fakeCombatLogPoster{}
	r := newDMDashboardRouterWithPoster(store, poster)

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/undo-last-action", strings.NewReader(`{"reason":"missed resistance"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int32(20), updatedHP)
	assert.Equal(t, int32(3), updatedTemp)

	require.Len(t, createdLogs, 1)
	assert.Equal(t, "dm_override_undo", createdLogs[0].ActionType)
	assert.Equal(t, actorID, createdLogs[0].ActorID)

	calls := poster.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, encounterID, calls[0].EncounterID)
	assert.Contains(t, calls[0].Message, "DM Correction")
	assert.Contains(t, calls[0].Message, "Goblin")
	assert.Contains(t, calls[0].Message, "missed resistance")
}

// --- TDD Cycle 4: Undo Last Action - skips its own dm_override_undo entries (chained undo) ---

func TestUndoLastAction_SkipsPreviousUndoEntries(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	actorID := uuid.New()

	beforeState := json.RawMessage(`{"hp_current":15,"temp_hp":0,"is_alive":true}`)

	logs := []refdata.ActionLog{
		// older damage action
		{
			ID:          uuid.New(),
			TurnID:      turnID,
			EncounterID: encounterID,
			ActionType:  "damage",
			ActorID:     actorID,
			BeforeState: beforeState,
		},
		// most recent is itself a previous undo
		{
			ID:          uuid.New(),
			TurnID:      turnID,
			EncounterID: encounterID,
			ActionType:  "dm_override_undo",
			ActorID:     actorID,
			BeforeState: json.RawMessage(`{}`),
		},
	}

	var hpCalled bool
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		listActionLogByTurnIDFn: func(ctx context.Context, tid uuid.UUID) ([]refdata.ActionLog, error) {
			return logs, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Goblin", HpCurrent: 5, IsAlive: true}, nil
		},
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			hpCalled = true
			return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	poster := &fakeCombatLogPoster{}
	r := newDMDashboardRouterWithPoster(store, poster)

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/undo-last-action", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hpCalled, "expected HP to be restored from underlying damage action")
}

// --- TDD Cycle 5: Undo Last Action - restores position from before_state ---

func TestUndoLastAction_RestoresPosition(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	actorID := uuid.New()

	beforeState := json.RawMessage(`{"position_col":"B","position_row":4,"altitude_ft":0}`)

	logs := []refdata.ActionLog{
		{
			ID:          uuid.New(),
			TurnID:      turnID,
			EncounterID: encounterID,
			ActionType:  "move",
			ActorID:     actorID,
			BeforeState: beforeState,
		},
	}

	var movedCol string
	var movedRow int32
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		listActionLogByTurnIDFn: func(ctx context.Context, tid uuid.UUID) ([]refdata.ActionLog, error) {
			return logs, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Mage", PositionCol: "F", PositionRow: 9}, nil
		},
		updateCombatantPositionFn: func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
			movedCol = arg.PositionCol
			movedRow = arg.PositionRow
			return refdata.Combatant{ID: arg.ID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	poster := &fakeCombatLogPoster{}
	r := newDMDashboardRouterWithPoster(store, poster)

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/undo-last-action", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "B", movedCol)
	assert.Equal(t, int32(4), movedRow)
}

// --- TDD Cycle 6: Undo Last Action - restores conditions from before_state ---

func TestUndoLastAction_RestoresConditions(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	actorID := uuid.New()

	beforeState := json.RawMessage(`{"conditions":[{"condition":"prone"}]}`)

	logs := []refdata.ActionLog{
		{
			ID:          uuid.New(),
			TurnID:      turnID,
			EncounterID: encounterID,
			ActionType:  "condition_change",
			ActorID:     actorID,
			BeforeState: beforeState,
		},
	}

	var setConds json.RawMessage
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		listActionLogByTurnIDFn: func(ctx context.Context, tid uuid.UUID) ([]refdata.ActionLog, error) {
			return logs, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Knight", Conditions: json.RawMessage(`[{"condition":"stunned"}]`)}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			setConds = arg.Conditions
			return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	poster := &fakeCombatLogPoster{}
	r := newDMDashboardRouterWithPoster(store, poster)

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/undo-last-action", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, string(setConds), "prone")
}

// --- TDD Cycle 7: Override Combatant HP ---

func TestOverrideCombatantHP(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	var got refdata.UpdateCombatantHPParams
	var loggedAction refdata.CreateActionLogParams
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: combatantID, DisplayName: "Wizard", HpCurrent: 10, TempHp: 0, IsAlive: true}, nil
		},
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			got = arg
			return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, TempHp: arg.TempHp, IsAlive: arg.IsAlive}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			loggedAction = arg
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	poster := &fakeCombatLogPoster{}
	r := newDMDashboardRouterWithPoster(store, poster)

	body := `{"hp_current":7,"temp_hp":2,"reason":"manual fix"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/hp", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int32(7), got.HpCurrent)
	assert.Equal(t, int32(2), got.TempHp)
	assert.True(t, got.IsAlive)

	assert.Equal(t, "dm_override", loggedAction.ActionType)
	assert.Equal(t, "manual fix", loggedAction.Description.String)

	calls := poster.Calls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Message, "Wizard")
	assert.Contains(t, calls[0].Message, "HP")
}

// --- TDD Cycle 8: Override Combatant Position ---

func TestOverrideCombatantPosition(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	var got refdata.UpdateCombatantPositionParams
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Rogue", PositionCol: "A", PositionRow: 1}, nil
		},
		updateCombatantPositionFn: func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
			got = arg
			return refdata.Combatant{ID: arg.ID}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	poster := &fakeCombatLogPoster{}
	r := newDMDashboardRouterWithPoster(store, poster)

	body := `{"position_col":"D","position_row":7,"altitude_ft":5,"reason":"tile correction"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/position", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "D", got.PositionCol)
	assert.Equal(t, int32(7), got.PositionRow)
	assert.Equal(t, int32(5), got.AltitudeFt)

	calls := poster.Calls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Message, "position")
}

// --- TDD Cycle 9: Override Combatant Conditions ---

func TestOverrideCombatantConditions(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	var got refdata.UpdateCombatantConditionsParams
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Cleric", Conditions: json.RawMessage(`[]`)}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			got = arg
			return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	poster := &fakeCombatLogPoster{}
	r := newDMDashboardRouterWithPoster(store, poster)

	body := `{"conditions":[{"condition":"blessed"}],"reason":"forgot bless"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/conditions", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, string(got.Conditions), "blessed")

	calls := poster.Calls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Message, "conditions")
}

// --- TDD Cycle 10: Override Combatant Initiative ---

func TestOverrideCombatantInitiative(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	var got refdata.UpdateCombatantInitiativeParams
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Fighter", InitiativeRoll: 12, InitiativeOrder: 3}, nil
		},
		updateCombatantInitiativeFn: func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
			got = arg
			return refdata.Combatant{ID: arg.ID, InitiativeRoll: arg.InitiativeRoll, InitiativeOrder: arg.InitiativeOrder}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	poster := &fakeCombatLogPoster{}
	r := newDMDashboardRouterWithPoster(store, poster)

	body := `{"initiative_roll":18,"initiative_order":1,"reason":"miscounted"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/initiative", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int32(18), got.InitiativeRoll)
	assert.Equal(t, int32(1), got.InitiativeOrder)

	calls := poster.Calls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Message, "initiative")
}

// --- TDD Cycle 11: Override Character Spell Slots ---

func TestOverrideCharacterSpellSlots(t *testing.T) {
	encounterID := uuid.New()
	characterID := uuid.New()
	turnID := uuid.New()

	var got refdata.UpdateCharacterSpellSlotsParams
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		getCharacterFn: func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
			return refdata.Character{ID: id, Name: "Gandalf"}, nil
		},
		updateCharacterSpellSlotsFn: func(ctx context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
			got = arg
			return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	poster := &fakeCombatLogPoster{}
	r := newDMDashboardRouterWithPoster(store, poster)

	body := `{"spell_slots":{"1":{"max":4,"used":1}},"reason":"slot accounting fix"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/character/"+characterID.String()+"/spell-slots", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, got.SpellSlots.Valid)
	assert.Contains(t, string(got.SpellSlots.RawMessage), `"max":4`)

	calls := poster.Calls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Message, "Gandalf")
	assert.Contains(t, calls[0].Message, "spell slots")
}

// --- TDD Cycle 12: invalid encounter ID, invalid combatant ID, invalid body ---

func TestUndoLastAction_InvalidEncounterID(t *testing.T) {
	r := newDMDashboardRouterWithPoster(&mockStore{}, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/not-uuid/undo-last-action", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOverrideCombatantHP_InvalidCombatantID(t *testing.T) {
	encounterID := uuid.New()
	r := newDMDashboardRouterWithPoster(&mockStore{}, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/not-uuid/hp", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOverrideCombatantHP_InvalidBody(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	r := newDMDashboardRouterWithPoster(&mockStore{}, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/hp", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Additional error path coverage ---

func TestOverrideCharacterSpellSlots_InvalidEncounterID(t *testing.T) {
	r := newDMDashboardRouterWithPoster(&mockStore{}, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/not-uuid/override/character/"+uuid.New().String()+"/spell-slots", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOverrideCharacterSpellSlots_InvalidCharacterID(t *testing.T) {
	encID := uuid.New()
	r := newDMDashboardRouterWithPoster(&mockStore{}, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encID.String()+"/override/character/not-uuid/spell-slots", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOverrideCharacterSpellSlots_InvalidBody(t *testing.T) {
	encID := uuid.New()
	charID := uuid.New()
	r := newDMDashboardRouterWithPoster(&mockStore{}, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encID.String()+"/override/character/"+charID.String()+"/spell-slots", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOverrideCharacterSpellSlots_NoActiveTurn(t *testing.T) {
	encID := uuid.New()
	charID := uuid.New()
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, errNoActiveTurn{}
		},
	}
	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encID.String()+"/override/character/"+charID.String()+"/spell-slots", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestOverrideCombatantHP_NoActiveTurn(t *testing.T) {
	encID := uuid.New()
	combID := uuid.New()
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, errNoActiveTurn{}
		},
	}
	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encID.String()+"/override/combatant/"+combID.String()+"/hp", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUndoLastAction_NoPosterAndNoLockSucceeds(t *testing.T) {
	// Cover the no-poster code path of postCorrection.
	encounterID := uuid.New()
	turnID := uuid.New()
	actorID := uuid.New()
	logs := []refdata.ActionLog{{
		ID: uuid.New(), TurnID: turnID, EncounterID: encounterID,
		ActionType: "damage", ActorID: actorID,
		BeforeState: json.RawMessage(`{"hp_current":10}`),
	}}

	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		listActionLogByTurnIDFn: func(ctx context.Context, tid uuid.UUID) ([]refdata.ActionLog, error) {
			return logs, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Goblin"}, nil
		},
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{}, nil
		},
	}
	svc := NewService(store)
	handler := NewDMDashboardHandlerWithDeps(svc, nil, nil) // nil poster
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/undo-last-action", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- TDD Cycle 13: Undo unknown action_type returns error ---

func TestUndoLastAction_UnknownActionType(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	actorID := uuid.New()

	logs := []refdata.ActionLog{
		{
			ID: uuid.New(), TurnID: turnID, EncounterID: encounterID,
			ActionType: "weird_unknown_type", ActorID: actorID,
			BeforeState: json.RawMessage(`{}`),
		},
	}

	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		listActionLogByTurnIDFn: func(ctx context.Context, tid uuid.UUID) ([]refdata.ActionLog, error) {
			return logs, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Mob"}, nil
		},
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/undo-last-action", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
