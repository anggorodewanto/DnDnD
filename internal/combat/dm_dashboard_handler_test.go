package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// newDMDashboardRouter creates a DMDashboardHandler with the given store and returns
// a ready-to-use Chi router with all routes registered.
func newDMDashboardRouter(store Store) http.Handler {
	svc := NewService(store)
	handler := NewDMDashboardHandler(svc)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	return r
}

// --- TDD Cycle 1: POST /api/combat/{encounterID}/advance-turn ---

func TestAdvanceTurn_Success(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getEncounterFn: func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:          encounterID,
				Status:      "active",
				RoundNumber: 1,
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: combatantID, DisplayName: "Fighter", InitiativeOrder: 1, IsAlive: true, Conditions: json.RawMessage(`[]`)},
			}, nil
		},
		listTurnsByEncounterAndRoundFn: func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
			return nil, nil
		},
		createTurnFn: func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"}, nil
		},
		updateEncounterCurrentTurnFn: func(ctx context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encounterID}, nil
		},
		updateEncounterRoundFn: func(ctx context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encounterID, RoundNumber: arg.RoundNumber}, nil
		},
		getCampaignByEncounterIDFn: func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{}, nil
		},
		getCharacterFn: func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
			return refdata.Character{SpeedFt: 30}, nil
		},
	}

	r := newDMDashboardRouter(store)

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/advance-turn", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp turnInfoResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, combatantID.String(), resp.CombatantID)
	assert.Equal(t, int32(1), resp.RoundNumber)
}

func TestAdvanceTurn_InvalidEncounterID(t *testing.T) {
	r := newDMDashboardRouter(&mockStore{})

	req := httptest.NewRequest("POST", "/api/combat/not-a-uuid/advance-turn", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- TDD Cycle 2: GET /api/combat/{encounterID}/pending-actions ---

func TestListPendingActions_Success(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	actionID := uuid.New()
	now := time.Now()

	store := &mockStore{
		listPendingActionsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.PendingAction, error) {
			return []refdata.PendingAction{
				{
					ID:          actionID,
					EncounterID: encounterID,
					CombatantID: combatantID,
					ActionText:  "I attack the goblin",
					Status:      "pending",
					CreatedAt:   now,
				},
			}, nil
		},
	}

	r := newDMDashboardRouter(store)

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/pending-actions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []pendingActionResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	require.Len(t, resp, 1)
	assert.Equal(t, actionID.String(), resp[0].ID)
	assert.Equal(t, "I attack the goblin", resp[0].ActionText)
	assert.Equal(t, "pending", resp[0].Status)
}

func TestListPendingActions_InvalidEncounterID(t *testing.T) {
	r := newDMDashboardRouter(&mockStore{})

	req := httptest.NewRequest("GET", "/api/combat/not-a-uuid/pending-actions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListPendingActions_Empty(t *testing.T) {
	encounterID := uuid.New()

	store := &mockStore{
		listPendingActionsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.PendingAction, error) {
			return nil, nil
		},
	}

	r := newDMDashboardRouter(store)

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/pending-actions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []pendingActionResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Empty(t, resp)
}

// --- TDD Cycle 3: POST /api/combat/{encounterID}/pending-actions/{actionID}/resolve ---

func TestResolvePendingAction_Success(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				CombatantID: combatantID,
				ActionText:  "I attack the goblin",
				Status:      "pending",
			}, nil
		},
		updatePendingActionStatusFn: func(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:     actionID,
				Status: "resolved",
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{}, nil
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"Hits for 8 damage","effects":[]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp resolvePendingActionResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "resolved", resp.Status)
	assert.Equal(t, "Hits for 8 damage", resp.Outcome)
}

func TestResolvePendingAction_NotPending(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				Status:      "resolved",
			}, nil
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"test"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestResolvePendingAction_WrongEncounter(t *testing.T) {
	encounterID := uuid.New()
	otherEncounterID := uuid.New()
	actionID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: otherEncounterID,
				Status:      "pending",
			}, nil
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"test"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResolvePendingAction_InvalidActionID(t *testing.T) {
	encounterID := uuid.New()

	r := newDMDashboardRouter(&mockStore{})

	body := `{"outcome":"test"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/not-a-uuid/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResolvePendingAction_InvalidBody(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()

	r := newDMDashboardRouter(&mockStore{})

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- TDD Cycle 4: Resolve with damage effect ---

func TestResolvePendingAction_WithDamageEffect(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	combatantID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	var updatedHP int32
	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				CombatantID: combatantID,
				Status:      "pending",
			}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:        targetID,
				HpMax:     20,
				HpCurrent: 20,
				TempHp:    0,
				IsAlive:   true,
			}, nil
		},
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			updatedHP = arg.HpCurrent
			return refdata.Combatant{ID: targetID, HpCurrent: arg.HpCurrent}, nil
		},
		updatePendingActionStatusFn: func(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
			return refdata.PendingAction{ID: actionID, Status: "resolved"}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{}, nil
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"8 slashing damage","effects":[{"type":"damage","target_id":"` + targetID.String() + `","value":{"amount":8}}]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int32(12), updatedHP)
}

// --- TDD Cycle 5: Resolve with condition_add effect ---

func TestResolvePendingAction_WithConditionAddEffect(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	combatantID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	var updatedConditions json.RawMessage
	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				CombatantID: combatantID,
				Status:      "pending",
			}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:         targetID,
				Conditions: json.RawMessage(`[]`),
			}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			updatedConditions = arg.Conditions
			return refdata.Combatant{ID: targetID, Conditions: arg.Conditions}, nil
		},
		updatePendingActionStatusFn: func(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
			return refdata.PendingAction{ID: actionID, Status: "resolved"}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{}, nil
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"Target is stunned","effects":[{"type":"condition_add","target_id":"` + targetID.String() + `","value":{"condition":"stunned"}}]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, string(updatedConditions), "stunned")
}

// --- TDD Cycle 6: Resolve with move effect ---

func TestResolvePendingAction_WithMoveEffect(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	combatantID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	var movedCol string
	var movedRow int32
	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				CombatantID: combatantID,
				Status:      "pending",
			}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: targetID, PositionCol: "A", PositionRow: 1}, nil
		},
		updateCombatantPositionFn: func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
			movedCol = arg.PositionCol
			movedRow = arg.PositionRow
			return refdata.Combatant{ID: targetID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow}, nil
		},
		updatePendingActionStatusFn: func(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
			return refdata.PendingAction{ID: actionID, Status: "resolved"}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{}, nil
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"Moved to C3","effects":[{"type":"move","target_id":"` + targetID.String() + `","value":{"col":"C","row":3}}]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "C", movedCol)
	assert.Equal(t, int32(3), movedRow)
}

// --- TDD Cycle 7: initiative_roll in workspace response ---

func TestWorkspaceResponse_IncludesInitiativeRoll(t *testing.T) {
	campaignID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := &mockWorkspaceStore{
		listEncountersByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{
				{ID: encounterID, CampaignID: campaignID, Name: "Test", Status: "active", RoundNumber: 1},
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{
					ID:              combatantID,
					ShortID:         "FI",
					DisplayName:     "Fighter",
					InitiativeRoll:  18,
					InitiativeOrder: 1,
					HpMax:           30,
					HpCurrent:       30,
					Ac:              16,
					IsAlive:         true,
					IsVisible:       true,
					Conditions:      json.RawMessage(`[]`),
				},
			}, nil
		},
		getMapByIDFn: func(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
			return refdata.Map{}, nil
		},
		listEncounterZonesByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
			return nil, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{CombatantID: combatantID}, nil
		},
	}

	handler := NewWorkspaceHandler(store, nil)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/combat/workspace?campaign_id="+campaignID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp workspaceResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	require.Len(t, resp.Encounters, 1)
	require.Len(t, resp.Encounters[0].Combatants, 1)
	assert.Equal(t, int32(18), resp.Encounters[0].Combatants[0].InitiativeRoll)
}

// --- TDD Cycle: AdvanceTurn error from service ---

func TestAdvanceTurn_ServiceError(t *testing.T) {
	encounterID := uuid.New()

	store := &mockStore{
		getEncounterFn: func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{}, errors.New("db error")
		},
	}

	r := newDMDashboardRouter(store)

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/advance-turn", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- TDD Cycle: ListPendingActions store error ---

func TestListPendingActions_StoreError(t *testing.T) {
	encounterID := uuid.New()

	store := &mockStore{
		listPendingActionsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.PendingAction, error) {
			return nil, errors.New("db error")
		},
	}

	r := newDMDashboardRouter(store)

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/pending-actions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- TDD Cycle: Resolve action not found ---

func TestResolvePendingAction_NotFound(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{}, errors.New("not found")
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"test"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- TDD Cycle: Resolve with invalid effects JSON ---

func TestResolvePendingAction_InvalidEffectsJSON(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				Status:      "pending",
			}, nil
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"test","effects":"not-an-array"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- TDD Cycle: Resolve with invalid target ID in effect ---

func TestResolvePendingAction_InvalidTargetID(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				Status:      "pending",
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: uuid.New()}, nil
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"test","effects":[{"type":"damage","target_id":"not-a-uuid","value":{"amount":5}}]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- TDD Cycle: Resolve no active turn returns 404 ---

func TestResolvePendingAction_NoActiveTurn(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	combatantID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				CombatantID: combatantID,
				Status:      "pending",
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, errors.New("no active turn")
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"Resolved without turn"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- TDD Cycle: Resolve with unknown effect type (ignored) ---

func TestResolvePendingAction_UnknownEffectType(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	combatantID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				CombatantID: combatantID,
				Status:      "pending",
			}, nil
		},
		updatePendingActionStatusFn: func(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
			return refdata.PendingAction{ID: actionID, Status: "resolved"}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{}, nil
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"test","effects":[{"type":"unknown_type","target_id":"` + targetID.String() + `","value":{}}]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// --- TDD Cycle: Damage absorbs temp HP ---

func TestResolvePendingAction_DamageAbsorbsTempHP(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	combatantID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	var updatedHP, updatedTempHP int32
	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				CombatantID: combatantID,
				Status:      "pending",
			}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:        targetID,
				HpMax:     20,
				HpCurrent: 20,
				TempHp:    5,
				IsAlive:   true,
			}, nil
		},
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			updatedHP = arg.HpCurrent
			updatedTempHP = arg.TempHp
			return refdata.Combatant{ID: targetID, HpCurrent: arg.HpCurrent, TempHp: arg.TempHp}, nil
		},
		updatePendingActionStatusFn: func(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
			return refdata.PendingAction{ID: actionID, Status: "resolved"}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{}, nil
		},
	}

	r := newDMDashboardRouter(store)

	// 8 damage: 5 absorbed by temp HP, 3 from HP (20->17)
	body := `{"outcome":"8 damage","effects":[{"type":"damage","target_id":"` + targetID.String() + `","value":{"amount":8}}]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int32(17), updatedHP)
	assert.Equal(t, int32(0), updatedTempHP)
}

// --- TDD Cycle: Resolve with invalid encounter ID in URL ---

func TestResolvePendingAction_InvalidEncounterID(t *testing.T) {
	r := newDMDashboardRouter(&mockStore{})

	actionID := uuid.New()
	body := `{"outcome":"test"}`
	req := httptest.NewRequest("POST", "/api/combat/not-a-uuid/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- TDD Cycle: Damage effect with getCombatant error ---

func TestResolvePendingAction_DamageEffectGetCombatantError(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	targetID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				Status:      "pending",
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: uuid.New()}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("not found")
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"test","effects":[{"type":"damage","target_id":"` + targetID.String() + `","value":{"amount":5}}]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- TDD Cycle: Condition add with getCombatant error ---

func TestResolvePendingAction_ConditionAddGetCombatantError(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	targetID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				Status:      "pending",
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: uuid.New()}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("not found")
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"test","effects":[{"type":"condition_add","target_id":"` + targetID.String() + `","value":{"condition":"stunned"}}]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- TDD Cycle: Move effect with getCombatant error ---

func TestResolvePendingAction_MoveEffectGetCombatantError(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	targetID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				Status:      "pending",
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: uuid.New()}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("not found")
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"test","effects":[{"type":"move","target_id":"` + targetID.String() + `","value":{"col":"B","row":3}}]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- TDD Cycle: Damage effect with invalid value JSON ---

func TestResolvePendingAction_DamageEffectInvalidValue(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	targetID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				Status:      "pending",
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: uuid.New()}, nil
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"test","effects":[{"type":"damage","target_id":"` + targetID.String() + `","value":"not-json-obj"}]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- TDD Cycle: Condition add with invalid value JSON ---

func TestResolvePendingAction_ConditionAddInvalidValue(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	targetID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				Status:      "pending",
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: uuid.New()}, nil
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"test","effects":[{"type":"condition_add","target_id":"` + targetID.String() + `","value":"bad"}]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- TDD Cycle: Move effect with invalid value JSON ---

func TestResolvePendingAction_MoveEffectInvalidValue(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	targetID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				Status:      "pending",
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: uuid.New()}, nil
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"test","effects":[{"type":"move","target_id":"` + targetID.String() + `","value":"bad"}]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- TDD Cycle: Resolve with update status error ---

func TestResolvePendingAction_UpdateStatusError(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				Status:      "pending",
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: uuid.New()}, nil
		},
		updatePendingActionStatusFn: func(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
			return refdata.PendingAction{}, errors.New("db error")
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"test"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- TDD Cycle: Condition remove with getCombatant error ---

func TestResolvePendingAction_ConditionRemoveGetCombatantError(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	targetID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				Status:      "pending",
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: uuid.New()}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("not found")
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"test","effects":[{"type":"condition_remove","target_id":"` + targetID.String() + `","value":{"condition":"stunned"}}]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- TDD Cycle: Condition remove with invalid value JSON ---

func TestResolvePendingAction_ConditionRemoveInvalidValue(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	targetID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				Status:      "pending",
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: uuid.New()}, nil
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"test","effects":[{"type":"condition_remove","target_id":"` + targetID.String() + `","value":"bad"}]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestResolvePendingAction_WithConditionRemoveEffect(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	combatantID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	var updatedConditions json.RawMessage
	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				CombatantID: combatantID,
				Status:      "pending",
			}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:         targetID,
				Conditions: json.RawMessage(`[{"condition":"stunned"}]`),
			}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			updatedConditions = arg.Conditions
			return refdata.Combatant{ID: targetID, Conditions: arg.Conditions}, nil
		},
		updatePendingActionStatusFn: func(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
			return refdata.PendingAction{ID: actionID, Status: "resolved"}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{}, nil
		},
	}

	r := newDMDashboardRouter(store)

	body := `{"outcome":"Stun removed","effects":[{"type":"condition_remove","target_id":"` + targetID.String() + `","value":{"condition":"stunned"}}]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, string(updatedConditions), "stunned")
}

// --- Phase 118 TDD Cycle 15: Voluntary concentration drop endpoint ---

func TestDropConcentration_Success(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	var (
		clearCalled   bool
		zoneCleanCalled bool
	)
	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          combatantID,
				EncounterID: encounterID,
				DisplayName: "Aria",
			}, nil
		},
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{
				ConcentrationSpellID:   sql.NullString{String: "bless", Valid: true},
				ConcentrationSpellName: sql.NullString{String: "Bless", Valid: true},
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return nil, nil
		},
		deleteConcentrationZonesByCombatantFn: func(ctx context.Context, id uuid.UUID) (int64, error) {
			zoneCleanCalled = true
			return 0, nil
		},
		clearCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) error {
			clearCalled = true
			return nil
		},
	}

	r := newDMDashboardRouter(store)
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/concentration/drop", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, clearCalled)
	assert.True(t, zoneCleanCalled)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	msg, _ := resp["consolidated_message"].(string)
	assert.Contains(t, msg, "💨")
	assert.Contains(t, msg, "Bless")
}

func TestDropConcentration_NotConcentrating(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, EncounterID: encounterID, DisplayName: "Aria"}, nil
		},
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{}, nil
		},
	}
	r := newDMDashboardRouter(store)
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/concentration/drop", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code, "dropping when not concentrating should fail")
}

func TestDropConcentration_InvalidIDs(t *testing.T) {
	store := &mockStore{}
	r := newDMDashboardRouter(store)

	// invalid encounter
	req := httptest.NewRequest("POST", "/api/combat/not-a-uuid/combatants/"+uuid.New().String()+"/concentration/drop", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// invalid combatant
	req = httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/combatants/not-a-uuid/concentration/drop", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- F-04: Action Resolver effects publish snapshot and record before/after state ---

func TestResolvePendingAction_F04_PublishesSnapshot(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				CombatantID: combatantID,
				Status:      "pending",
			}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          combatantID,
				EncounterID: encounterID,
				HpCurrent:   30,
				Conditions:  json.RawMessage(`[]`),
				PositionCol: "B",
				PositionRow: 3,
			}, nil
		},
		updatePendingActionStatusFn: func(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
			return refdata.PendingAction{ID: actionID, Status: "resolved"}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{}, nil
		},
	}

	pub := &fakePublisher{}
	svc := NewService(store)
	svc.SetPublisher(pub)
	handler := NewDMDashboardHandler(svc)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	body := `{"outcome":"Moved","effects":[]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	calls := pub.calls()
	require.Len(t, calls, 1, "expected exactly one snapshot publish")
	assert.Equal(t, encounterID, calls[0])
}

func TestResolvePendingAction_F04_BeforeAfterState(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	combatantID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	hpBefore := int32(30)
	hpAfter := int32(30) // condition effect doesn't change HP

	getCalls := 0
	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				CombatantID: combatantID,
				Status:      "pending",
			}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			getCalls++
			// First two calls are for before-state capture and condition effect lookup
			// After condition is applied, the third call (after-state) returns updated conditions
			if id == combatantID && getCalls <= 2 {
				return refdata.Combatant{
					ID:          combatantID,
					EncounterID: encounterID,
					HpCurrent:   hpBefore,
					Conditions:  json.RawMessage(`[]`),
					PositionCol: "A",
					PositionRow: 1,
				}, nil
			}
			// After effects applied - combatant now has condition
			return refdata.Combatant{
				ID:          combatantID,
				EncounterID: encounterID,
				HpCurrent:   hpAfter,
				Conditions:  json.RawMessage(`[{"condition":"poisoned"}]`),
				PositionCol: "A",
				PositionRow: 1,
			}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: targetID}, nil
		},
		updatePendingActionStatusFn: func(ctx context.Context, arg refdata.UpdatePendingActionStatusParams) (refdata.PendingAction, error) {
			return refdata.PendingAction{ID: actionID, Status: "resolved"}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			// Verify before/after state are populated
			assert.NotNil(t, arg.BeforeState, "BeforeState should be populated")
			assert.NotNil(t, arg.AfterState, "AfterState should be populated")

			var before, after resolverStateSnapshot
			require.NoError(t, json.Unmarshal(arg.BeforeState, &before))
			require.NoError(t, json.Unmarshal(arg.AfterState, &after))

			assert.Equal(t, hpBefore, before.HP)
			assert.JSONEq(t, `[]`, string(before.Conditions))
			assert.Equal(t, "A1", before.Position)

			assert.JSONEq(t, `[{"condition":"poisoned"}]`, string(after.Conditions))
			return refdata.ActionLog{}, nil
		},
	}

	svc := NewService(store)
	handler := NewDMDashboardHandler(svc)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	// Apply a condition_add effect targeting the combatant
	body := `{"outcome":"Poisoned","effects":[{"type":"condition_add","target_id":"` + combatantID.String() + `","value":{"condition":"poisoned"}}]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

// --- TDD Cycle: ResolvePendingAction acquires turn lock (I-H04) ---

func TestResolvePendingAction_AcquiresTurnLock(t *testing.T) {
	encounterID := uuid.New()
	actionID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getPendingActionFn: func(ctx context.Context, id uuid.UUID) (refdata.PendingAction, error) {
			return refdata.PendingAction{
				ID:          actionID,
				EncounterID: encounterID,
				CombatantID: combatantID,
				Status:      "pending",
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
	}

	// fakeTxBeginner always fails → if the handler acquires the lock, we get 500.
	r := newDMDashboardRouterWithDB(store, &fakeCombatLogPoster{}, fakeTxBeginner{})

	body := `{"outcome":"test","effects":[]}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-actions/"+actionID.String()+"/resolve", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// If the handler uses withTurnLock, the fake BeginTx error causes a 500.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
