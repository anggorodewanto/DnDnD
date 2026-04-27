package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// errBeginTxFake is the sentinel error returned by fakeTxBeginner.
var errBeginTxFake = errors.New("fake BeginTx failure")

// fakeTxBeginner is a TxBeginner whose BeginTx always fails.
// Used to exercise the lock-acquire error branch of withTurnLock without a real DB.
type fakeTxBeginner struct{}

func (fakeTxBeginner) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return nil, errBeginTxFake
}

// newDMDashboardRouterWithDB builds a router with the given store, poster, and TxBeginner.
func newDMDashboardRouterWithDB(store Store, poster CombatLogPoster, db TxBeginner) http.Handler {
	svc := NewService(store)
	handler := NewDMDashboardHandlerWithDeps(svc, db, poster)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	return r
}

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
		ID:          uuid.New(),
		TurnID:      turnID,
		EncounterID: encounterID,
		ActionType:  "damage",
		ActorID:     actorID,
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
			ID:          uuid.New(),
			TurnID:      turnID,
			EncounterID: encounterID,
			ActionType:  "weird_unknown_type",
			ActorID:     actorID,
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

// --- applyUndo: zero ActorID returns 422 without calling GetCombatant ---
//
// Covers the early-return in applyUndo — an action_log row whose ActorID is
// uuid.Nil (the zero value) has no combatant to restore against, so applyUndo
// wraps errUnknownActionType and the HTTP handler maps that to 422. The test
// also asserts GetCombatant was never called. Phase 119: action_log.actor_id
// is NOT NULL again, but defensively we still guard against the Nil case.
func TestUndoLastAction_InvalidActorID(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()

	logs := []refdata.ActionLog{
		{
			ID:          uuid.New(),
			TurnID:      turnID,
			EncounterID: encounterID,
			ActionType:  "damage",
			ActorID:     uuid.Nil, // zero actor — the defensive branch under test
			BeforeState: json.RawMessage(`{"hp_current":10}`),
		},
	}

	getCombatantCalled := false
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		listActionLogByTurnIDFn: func(ctx context.Context, tid uuid.UUID) ([]refdata.ActionLog, error) {
			return logs, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			getCombatantCalled = true
			return refdata.Combatant{}, nil
		},
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/undo-last-action", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "undo target has no actor")
	assert.False(t, getCombatantCalled, "GetCombatant must not be called when ActorID is invalid")
}

// --- Iteration 2: error-path coverage to push dm_dashboard_undo.go above 90% ---

// errStore is a sentinel store error reused throughout the error-path tests.
var errStoreFake = errors.New("fake store failure")

// helper: build a turn-returning store with optional listLogs / extra setup.
func turnOnlyStore(turnID, encounterID uuid.UUID) *mockStore {
	return &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
	}
}

// --- withTurnLock: lock-acquire failure ---

func TestWithTurnLock_LockAcquireError_UndoReturns500(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	logs := []refdata.ActionLog{{
		ID:          uuid.New(),
		TurnID:      turnID,
		EncounterID: encounterID,
		ActionType:  "damage",
		ActorID:     uuid.New(),
		BeforeState: json.RawMessage(`{"hp_current":10}`),
	}}
	store := turnOnlyStore(turnID, encounterID)
	store.listActionLogByTurnIDFn = func(ctx context.Context, tid uuid.UUID) ([]refdata.ActionLog, error) {
		return logs, nil
	}

	r := newDMDashboardRouterWithDB(store, &fakeCombatLogPoster{}, fakeTxBeginner{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/undo-last-action", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestWithTurnLock_LockAcquireError_OverrideHPReturns500(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()
	store := turnOnlyStore(turnID, encounterID)

	r := newDMDashboardRouterWithDB(store, &fakeCombatLogPoster{}, fakeTxBeginner{})
	body := `{"hp_current":1,"temp_hp":0,"reason":"x"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/hp", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestWithTurnLock_LockAcquireError_SpellSlotsReturns500(t *testing.T) {
	encounterID := uuid.New()
	characterID := uuid.New()
	turnID := uuid.New()
	store := turnOnlyStore(turnID, encounterID)

	r := newDMDashboardRouterWithDB(store, &fakeCombatLogPoster{}, fakeTxBeginner{})
	body := `{"spell_slots":{"1":{"max":1,"used":0}},"reason":"x"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/character/"+characterID.String()+"/spell-slots", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- UndoLastAction: list action log failure ---

func TestUndoLastAction_ListActionLogError(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	store := turnOnlyStore(turnID, encounterID)
	store.listActionLogByTurnIDFn = func(ctx context.Context, tid uuid.UUID) ([]refdata.ActionLog, error) {
		return nil, errStoreFake
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/undo-last-action", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- applyUndo: malformed before_state JSON ---

func TestUndoLastAction_MalformedBeforeState(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	logs := []refdata.ActionLog{{
		ID:          uuid.New(),
		TurnID:      turnID,
		EncounterID: encounterID,
		ActionType:  "damage",
		ActorID:     uuid.New(),
		BeforeState: json.RawMessage(`{not json`),
	}}
	store := turnOnlyStore(turnID, encounterID)
	store.listActionLogByTurnIDFn = func(ctx context.Context, tid uuid.UUID) ([]refdata.ActionLog, error) {
		return logs, nil
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/undo-last-action", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- applyUndo: GetCombatant failure ---

func TestUndoLastAction_GetCombatantError(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	logs := []refdata.ActionLog{{
		ID:          uuid.New(),
		TurnID:      turnID,
		EncounterID: encounterID,
		ActionType:  "damage",
		ActorID:     uuid.New(),
		BeforeState: json.RawMessage(`{"hp_current":10}`),
	}}
	store := turnOnlyStore(turnID, encounterID)
	store.listActionLogByTurnIDFn = func(ctx context.Context, tid uuid.UUID) ([]refdata.ActionLog, error) {
		return logs, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, errStoreFake
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/undo-last-action", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- dispatchUndo: UpdateCombatantHP failure ---

func TestUndoLastAction_DispatchUpdateHPError(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	logs := []refdata.ActionLog{{
		ID:          uuid.New(),
		TurnID:      turnID,
		EncounterID: encounterID,
		ActionType:  "damage",
		ActorID:     uuid.New(),
		BeforeState: json.RawMessage(`{"hp_current":10}`),
	}}
	store := turnOnlyStore(turnID, encounterID)
	store.listActionLogByTurnIDFn = func(ctx context.Context, tid uuid.UUID) ([]refdata.ActionLog, error) {
		return logs, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, DisplayName: "x"}, nil
	}
	store.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errStoreFake
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/undo-last-action", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- dispatchUndo: UpdateCombatantPosition failure ---

func TestUndoLastAction_DispatchUpdatePositionError(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	logs := []refdata.ActionLog{{
		ID:          uuid.New(),
		TurnID:      turnID,
		EncounterID: encounterID,
		ActionType:  "move",
		ActorID:     uuid.New(),
		BeforeState: json.RawMessage(`{"position_col":"A","position_row":1}`),
	}}
	store := turnOnlyStore(turnID, encounterID)
	store.listActionLogByTurnIDFn = func(ctx context.Context, tid uuid.UUID) ([]refdata.ActionLog, error) {
		return logs, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, DisplayName: "x"}, nil
	}
	store.updateCombatantPositionFn = func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errStoreFake
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/undo-last-action", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- dispatchUndo: UpdateCombatantConditions failure ---

func TestUndoLastAction_DispatchUpdateConditionsError(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	logs := []refdata.ActionLog{{
		ID:          uuid.New(),
		TurnID:      turnID,
		EncounterID: encounterID,
		ActionType:  "condition_change",
		ActorID:     uuid.New(),
		BeforeState: json.RawMessage(`{"conditions":[{"condition":"prone"}]}`),
	}}
	store := turnOnlyStore(turnID, encounterID)
	store.listActionLogByTurnIDFn = func(ctx context.Context, tid uuid.UUID) ([]refdata.ActionLog, error) {
		return logs, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, DisplayName: "x"}, nil
	}
	store.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errStoreFake
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/undo-last-action", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- parseCombatantOverrideIDs: invalid encounter ID on combatant override route ---

func TestOverrideCombatantHP_InvalidEncounterID(t *testing.T) {
	r := newDMDashboardRouterWithPoster(&mockStore{}, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/not-uuid/override/combatant/"+uuid.New().String()+"/hp", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- OverrideCombatantHP: store error paths ---

func TestOverrideCombatantHP_GetCombatantError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()
	store := turnOnlyStore(turnID, encounterID)
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, errStoreFake
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/hp", strings.NewReader(`{"hp_current":1,"temp_hp":0}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestOverrideCombatantHP_UpdateError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()
	store := turnOnlyStore(turnID, encounterID)
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, DisplayName: "x"}, nil
	}
	store.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errStoreFake
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/hp", strings.NewReader(`{"hp_current":1,"temp_hp":0}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- OverrideCombatantPosition: store error paths ---

func TestOverrideCombatantPosition_GetCombatantError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()
	store := turnOnlyStore(turnID, encounterID)
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, errStoreFake
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/position", strings.NewReader(`{"position_col":"A","position_row":1,"altitude_ft":0}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestOverrideCombatantPosition_UpdateError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()
	store := turnOnlyStore(turnID, encounterID)
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, DisplayName: "x"}, nil
	}
	store.updateCombatantPositionFn = func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errStoreFake
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/position", strings.NewReader(`{"position_col":"A","position_row":1,"altitude_ft":0}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- OverrideCombatantConditions: store error paths + empty conditions branch ---

func TestOverrideCombatantConditions_GetCombatantError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()
	store := turnOnlyStore(turnID, encounterID)
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, errStoreFake
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/conditions", strings.NewReader(`{"conditions":[]}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestOverrideCombatantConditions_UpdateError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()
	store := turnOnlyStore(turnID, encounterID)
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, DisplayName: "x"}, nil
	}
	store.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errStoreFake
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/conditions", strings.NewReader(`{"conditions":[{"condition":"blessed"}]}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// Empty conditions array path: when req.Conditions is empty/missing the handler
// should default to `[]`. Verifies the conditions == 0 branch in OverrideCombatantConditions.
func TestOverrideCombatantConditions_EmptyDefaultsToArray(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	var passed json.RawMessage
	store := turnOnlyStore(turnID, encounterID)
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, DisplayName: "x", Conditions: json.RawMessage(`[]`)}, nil
	}
	store.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		passed = arg.Conditions
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}
	store.createActionLogFn = func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
		return refdata.ActionLog{}, nil
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	// Body without conditions field — req.Conditions will be empty.
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/conditions", strings.NewReader(`{"reason":"clear"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]", string(passed))
}

// --- OverrideCombatantInitiative: store error paths ---

func TestOverrideCombatantInitiative_GetCombatantError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()
	store := turnOnlyStore(turnID, encounterID)
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, errStoreFake
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/initiative", strings.NewReader(`{"initiative_roll":5,"initiative_order":1}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestOverrideCombatantInitiative_UpdateError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()
	store := turnOnlyStore(turnID, encounterID)
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, DisplayName: "x"}, nil
	}
	store.updateCombatantInitiativeFn = func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errStoreFake
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/initiative", strings.NewReader(`{"initiative_roll":5,"initiative_order":1}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- OverrideCharacterSpellSlots: store error paths and SpellSlots.Valid branch ---

func TestOverrideCharacterSpellSlots_GetCharacterError(t *testing.T) {
	encounterID := uuid.New()
	characterID := uuid.New()
	turnID := uuid.New()
	store := turnOnlyStore(turnID, encounterID)
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, errStoreFake
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/character/"+characterID.String()+"/spell-slots", strings.NewReader(`{"spell_slots":{"1":{"max":1,"used":0}}}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestOverrideCharacterSpellSlots_UpdateError(t *testing.T) {
	encounterID := uuid.New()
	characterID := uuid.New()
	turnID := uuid.New()
	store := turnOnlyStore(turnID, encounterID)
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{ID: id, Name: "x"}, nil
	}
	store.updateCharacterSpellSlotsFn = func(ctx context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{}, errStoreFake
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/character/"+characterID.String()+"/spell-slots", strings.NewReader(`{"spell_slots":{"1":{"max":1,"used":0}}}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// Covers the SpellSlots.Valid == true branch (existing happy-path test had an empty SpellSlots).
func TestOverrideCharacterSpellSlots_PreservesPreviousSlotsAsBefore(t *testing.T) {
	encounterID := uuid.New()
	characterID := uuid.New()
	turnID := uuid.New()
	prev := json.RawMessage(`{"1":{"max":2,"used":1}}`)

	var loggedBefore json.RawMessage
	store := turnOnlyStore(turnID, encounterID)
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID: id, Name: "x",
			SpellSlots: pqtype.NullRawMessage{Valid: true, RawMessage: prev},
		}, nil
	}
	store.updateCharacterSpellSlotsFn = func(ctx context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}
	store.createActionLogFn = func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
		loggedBefore = arg.BeforeState
		return refdata.ActionLog{}, nil
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/override/character/"+characterID.String()+"/spell-slots", strings.NewReader(`{"spell_slots":{"1":{"max":4,"used":0}}}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, string(prev), string(loggedBefore))
}
