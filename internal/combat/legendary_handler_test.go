package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// --- TDD Cycle 8: GET /api/combat/{encounterID}/legendary/{combatantID}/plan ---

func TestGetLegendaryActionPlan_Success(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:            npcID,
				EncounterID:   encounterID,
				DisplayName:   "Adult Red Dragon",
				IsNpc:         true,
				IsAlive:       true,
				CreatureRefID: sql.NullString{String: "adult-red-dragon", Valid: true},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID:   "adult-red-dragon",
				Name: "Adult Red Dragon",
				Abilities: pqtype.NullRawMessage{Valid: true, RawMessage: json.RawMessage(`[
					{"name":"Legendary Actions","description":"The dragon can take 3 legendary actions."},
					{"name":"Detect","description":"The dragon makes a Perception check."},
					{"name":"Tail Attack","description":"The dragon makes a tail attack."},
					{"name":"Wing Attack (Costs 2 Actions)","description":"Wings beat."}
				]`)},
			}, nil
		},
	}

	svc := NewService(store)
	roller := dice.NewRoller(nil)
	handler := NewHandler(svc, roller)

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/legendary/"+npcID.String()+"/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp legendaryActionPlanResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "Adult Red Dragon", resp.CreatureName)
	assert.Equal(t, 3, resp.BudgetTotal)
	assert.Equal(t, 3, resp.BudgetRemaining)
	require.Len(t, resp.AvailableActions, 3)
}

func TestGetLegendaryActionPlan_NoLegendaryActions(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:            npcID,
				DisplayName:   "Goblin",
				IsNpc:         true,
				CreatureRefID: sql.NullString{String: "goblin", Valid: true},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{ID: "goblin", Name: "Goblin"}, nil
		},
	}

	svc := NewService(store)
	roller := dice.NewRoller(nil)
	handler := NewHandler(svc, roller)

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/legendary/"+npcID.String()+"/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetLegendaryActionPlan_WithBudgetQuery(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:            npcID,
				DisplayName:   "Dragon",
				IsNpc:         true,
				CreatureRefID: sql.NullString{String: "dragon", Valid: true},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID: "dragon", Name: "Dragon",
				Abilities: pqtype.NullRawMessage{Valid: true, RawMessage: json.RawMessage(`[
					{"name":"Legendary Actions","description":"The dragon can take 3 legendary actions."},
					{"name":"Detect","description":"Check."},
					{"name":"Wing Attack (Costs 2 Actions)","description":"Wings."}
				]`)},
			}, nil
		},
	}

	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/legendary/"+npcID.String()+"/plan?budget_remaining=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp legendaryActionPlanResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, 1, resp.BudgetRemaining)
	// Detect (cost 1) affordable, Wing Attack (cost 2) not
	assert.True(t, resp.AvailableActions[0].Affordable)
	assert.False(t, resp.AvailableActions[1].Affordable)
}

// --- TDD Cycle 9: POST /api/combat/{encounterID}/legendary ---

func TestExecuteLegendaryAction_Success(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	turnID := uuid.New()

	actionLogged := false
	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:            npcID,
				DisplayName:   "Adult Red Dragon",
				IsNpc:         true,
				IsAlive:       true,
				CreatureRefID: sql.NullString{String: "adult-red-dragon", Valid: true},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID:   "adult-red-dragon",
				Name: "Adult Red Dragon",
				Abilities: pqtype.NullRawMessage{Valid: true, RawMessage: json.RawMessage(`[
					{"name":"Legendary Actions","description":"The dragon can take 3 legendary actions."},
					{"name":"Detect","description":"The dragon makes a Perception check."},
					{"name":"Tail Attack","description":"The dragon makes a tail attack."},
					{"name":"Wing Attack (Costs 2 Actions)","description":"Wings beat."}
				]`)},
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: eid, CombatantID: uuid.New()}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			actionLogged = true
			assert.Equal(t, "legendary_action", arg.ActionType)
			assert.Equal(t, uuid.NullUUID{UUID: npcID, Valid: true}, arg.ActorID)
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	body := `{
		"combatant_id": "` + npcID.String() + `",
		"action_name": "Tail Attack",
		"budget_remaining": 3
	}`

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/legendary", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp executeLegendaryActionResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, 2, resp.BudgetRemaining)
	assert.Contains(t, resp.CombatLog, "Tail Attack")

	assert.True(t, actionLogged)
}

func TestExecuteLegendaryAction_InsufficientBudget(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:            npcID,
				DisplayName:   "Dragon",
				IsNpc:         true,
				CreatureRefID: sql.NullString{String: "dragon", Valid: true},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID: "dragon", Name: "Dragon",
				Abilities: pqtype.NullRawMessage{Valid: true, RawMessage: json.RawMessage(`[
					{"name":"Legendary Actions","description":"The dragon can take 3 legendary actions."},
					{"name":"Wing Attack (Costs 2 Actions)","description":"Wings beat."}
				]`)},
			}, nil
		},
	}

	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	body := `{
		"combatant_id": "` + npcID.String() + `",
		"action_name": "Wing Attack",
		"budget_remaining": 1
	}`

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/legendary", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExecuteLegendaryAction_UnknownAction(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID: npcID, DisplayName: "Dragon", IsNpc: true,
				CreatureRefID: sql.NullString{String: "dragon", Valid: true},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID: "dragon", Name: "Dragon",
				Abilities: pqtype.NullRawMessage{Valid: true, RawMessage: json.RawMessage(`[
					{"name":"Legendary Actions","description":"The dragon can take 3 legendary actions."},
					{"name":"Detect","description":"Check."}
				]`)},
			}, nil
		},
	}

	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))
	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	body := `{"combatant_id": "` + npcID.String() + `", "action_name": "Nonexistent", "budget_remaining": 3}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/legendary", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- TDD Cycle 10: GET /api/combat/{encounterID}/lair-action/plan ---

func TestGetLairActionPlan_Success(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()

	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{
					ID:            npcID,
					DisplayName:   "Adult Red Dragon",
					IsNpc:         true,
					IsAlive:       true,
					CreatureRefID: sql.NullString{String: "adult-red-dragon", Valid: true},
				},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID:   "adult-red-dragon",
				Name: "Adult Red Dragon",
				Abilities: pqtype.NullRawMessage{Valid: true, RawMessage: json.RawMessage(`[
					{"name":"Lair Actions","description":"On initiative count 20..."},
					{"name":"Magma Eruption","description":"Magma erupts."},
					{"name":"Tremor","description":"A tremor shakes the lair."}
				]`)},
			}, nil
		},
	}

	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/lair-action/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp lairActionPlanResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "Adult Red Dragon", resp.CreatureName)
	require.Len(t, resp.AvailableActions, 2)
}

func TestGetLairActionPlan_NoLairCreature(t *testing.T) {
	encounterID := uuid.New()

	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: uuid.New(), DisplayName: "Goblin", IsNpc: true, IsAlive: true, CreatureRefID: sql.NullString{String: "goblin", Valid: true}},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{ID: "goblin", Name: "Goblin"}, nil
		},
	}

	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))
	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/lair-action/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- TDD Cycle 11: POST /api/combat/{encounterID}/lair-action ---

func TestExecuteLairAction_Success(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	turnID := uuid.New()

	actionLogged := false
	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{
					ID:            npcID,
					DisplayName:   "Adult Red Dragon",
					IsNpc:         true,
					IsAlive:       true,
					CreatureRefID: sql.NullString{String: "adult-red-dragon", Valid: true},
				},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID:   "adult-red-dragon",
				Name: "Adult Red Dragon",
				Abilities: pqtype.NullRawMessage{Valid: true, RawMessage: json.RawMessage(`[
					{"name":"Lair Actions","description":"On initiative count 20..."},
					{"name":"Magma Eruption","description":"Magma erupts."},
					{"name":"Tremor","description":"A tremor shakes the lair."}
				]`)},
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: eid}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			actionLogged = true
			assert.Equal(t, "lair_action", arg.ActionType)
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	body := `{"action_name": "Magma Eruption", "last_used_action": ""}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/lair-action", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp executeLairActionResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.True(t, resp.Success)
	assert.Contains(t, resp.CombatLog, "Magma Eruption")
	assert.Equal(t, "Magma Eruption", resp.LastUsedAction)
	assert.True(t, actionLogged)
}

func TestExecuteLairAction_ConsecutiveRepeat(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()

	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: npcID, DisplayName: "Dragon", IsNpc: true, IsAlive: true, CreatureRefID: sql.NullString{String: "dragon", Valid: true}},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID: "dragon", Name: "Dragon",
				Abilities: pqtype.NullRawMessage{Valid: true, RawMessage: json.RawMessage(`[
					{"name":"Lair Actions","description":"On initiative count 20..."},
					{"name":"Magma Eruption","description":"Magma erupts."},
					{"name":"Tremor","description":"A tremor shakes the lair."}
				]`)},
			}, nil
		},
	}

	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))
	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	body := `{"action_name": "Magma Eruption", "last_used_action": "Magma Eruption"}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/lair-action", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExecuteLairAction_UnknownAction(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()

	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: npcID, DisplayName: "Dragon", IsNpc: true, IsAlive: true, CreatureRefID: sql.NullString{String: "dragon", Valid: true}},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID: "dragon", Name: "Dragon",
				Abilities: pqtype.NullRawMessage{Valid: true, RawMessage: json.RawMessage(`[
					{"name":"Lair Actions","description":"On init 20..."},
					{"name":"Tremor","description":"shakes."}
				]`)},
			}, nil
		},
	}

	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))
	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	body := `{"action_name": "Nonexistent", "last_used_action": ""}`
	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/lair-action", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- TDD Cycle 14: GET /api/combat/{encounterID}/turn-queue ---

func TestGetTurnQueue_WithLegendaryAndLair(t *testing.T) {
	encounterID := uuid.New()
	dragonID := uuid.New()
	pcID := uuid.New()

	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: pcID, DisplayName: "Aragorn", InitiativeRoll: 18, InitiativeOrder: 1, IsNpc: false, IsAlive: true},
				{ID: dragonID, DisplayName: "Adult Red Dragon", InitiativeRoll: 15, InitiativeOrder: 2, IsNpc: true, IsAlive: true, CreatureRefID: sql.NullString{String: "adult-red-dragon", Valid: true}},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID:   "adult-red-dragon",
				Name: "Adult Red Dragon",
				Abilities: pqtype.NullRawMessage{Valid: true, RawMessage: json.RawMessage(`[
					{"name":"Legendary Actions","description":"The dragon can take 3 legendary actions."},
					{"name":"Detect","description":"Check."},
					{"name":"Lair Actions","description":"On initiative count 20..."},
					{"name":"Magma","description":"Magma erupts."}
				]`)},
			}, nil
		},
	}

	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))
	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/turn-queue", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp turnQueueResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.GreaterOrEqual(t, len(resp.Entries), 3)

	assert.Equal(t, TurnQueueLairAction, resp.Entries[0].Type)
	assert.Equal(t, int32(20), resp.Entries[0].Initiative)
}

// --- Error-path tests for GetLegendaryActionPlan ---

func TestGetLegendaryActionPlan_InvalidCombatantID(t *testing.T) {
	svc := NewService(&mockStore{})
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+uuid.New().String()+"/legendary/not-a-uuid/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid combatant ID")
}

func TestGetLegendaryActionPlan_CombatantNotFound(t *testing.T) {
	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, fmt.Errorf("not found")
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+uuid.New().String()+"/legendary/"+uuid.New().String()+"/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "combatant not found")
}

func TestGetLegendaryActionPlan_NoCreatureRef(t *testing.T) {
	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:            id,
				DisplayName:   "Goblin",
				CreatureRefID: sql.NullString{Valid: false},
			}, nil
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+uuid.New().String()+"/legendary/"+uuid.New().String()+"/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "combatant has no creature reference")
}

func TestGetLegendaryActionPlan_CreatureNotFound(t *testing.T) {
	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:            id,
				DisplayName:   "Dragon",
				CreatureRefID: sql.NullString{String: "dragon", Valid: true},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{}, fmt.Errorf("not found")
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+uuid.New().String()+"/legendary/"+uuid.New().String()+"/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "creature not found")
}

// --- Error-path tests for ExecuteLegendaryAction ---

func TestExecuteLegendaryAction_InvalidEncounterID(t *testing.T) {
	svc := NewService(&mockStore{})
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	body := `{"combatant_id": "` + uuid.New().String() + `", "action_name": "Detect", "budget_remaining": 3}`
	req := httptest.NewRequest("POST", "/api/combat/not-a-uuid/legendary", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid encounter ID")
}

func TestExecuteLegendaryAction_InvalidJSON(t *testing.T) {
	svc := NewService(&mockStore{})
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/legendary", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid JSON body")
}

func TestExecuteLegendaryAction_InvalidCombatantIDInBody(t *testing.T) {
	svc := NewService(&mockStore{})
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	body := `{"combatant_id": "not-a-uuid", "action_name": "Detect", "budget_remaining": 3}`
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/legendary", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid combatant_id")
}

func TestExecuteLegendaryAction_CombatantNotFound(t *testing.T) {
	npcID := uuid.New()
	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, fmt.Errorf("not found")
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	body := `{"combatant_id": "` + npcID.String() + `", "action_name": "Detect", "budget_remaining": 3}`
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/legendary", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "combatant not found")
}

func TestExecuteLegendaryAction_NoCreatureRef(t *testing.T) {
	npcID := uuid.New()
	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:            npcID,
				DisplayName:   "Goblin",
				CreatureRefID: sql.NullString{Valid: false},
			}, nil
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	body := `{"combatant_id": "` + npcID.String() + `", "action_name": "Detect", "budget_remaining": 3}`
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/legendary", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "combatant has no creature reference")
}

func TestExecuteLegendaryAction_CreatureNotFound(t *testing.T) {
	npcID := uuid.New()
	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:            npcID,
				DisplayName:   "Dragon",
				CreatureRefID: sql.NullString{String: "dragon", Valid: true},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{}, fmt.Errorf("not found")
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	body := `{"combatant_id": "` + npcID.String() + `", "action_name": "Detect", "budget_remaining": 3}`
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/legendary", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "creature not found")
}

func TestExecuteLegendaryAction_NoLegendaryActions(t *testing.T) {
	npcID := uuid.New()
	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:            npcID,
				DisplayName:   "Goblin",
				CreatureRefID: sql.NullString{String: "goblin", Valid: true},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{ID: "goblin", Name: "Goblin"}, nil
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	body := `{"combatant_id": "` + npcID.String() + `", "action_name": "Detect", "budget_remaining": 3}`
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/legendary", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "creature has no legendary actions")
}

// --- Error-path tests for GetLairActionPlan ---

func TestGetLairActionPlan_InvalidEncounterID(t *testing.T) {
	svc := NewService(&mockStore{})
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/not-a-uuid/lair-action/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// chi won't match: 404/405
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestGetLairActionPlan_ListCombatantsError(t *testing.T) {
	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+uuid.New().String()+"/lair-action/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetLairActionPlan_NoLairActions(t *testing.T) {
	npcID := uuid.New()
	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: npcID, DisplayName: "Dragon", IsNpc: true, IsAlive: true, CreatureRefID: sql.NullString{String: "dragon", Valid: true}},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID: "dragon", Name: "Dragon",
				Abilities: pqtype.NullRawMessage{Valid: true, RawMessage: json.RawMessage(`[
					{"name":"Legendary Actions","description":"The dragon can take 3 legendary actions."},
					{"name":"Detect","description":"Check."}
				]`)},
			}, nil
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+uuid.New().String()+"/lair-action/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetLairActionPlan_WithLastUsedQuery(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()

	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: npcID, DisplayName: "Dragon", IsNpc: true, IsAlive: true, CreatureRefID: sql.NullString{String: "dragon", Valid: true}},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID: "dragon", Name: "Dragon",
				Abilities: pqtype.NullRawMessage{Valid: true, RawMessage: json.RawMessage(`[
					{"name":"Lair Actions","description":"On init 20..."},
					{"name":"Magma Eruption","description":"erupts."},
					{"name":"Tremor","description":"shakes."}
				]`)},
			}, nil
		},
	}

	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))
	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/lair-action/plan?last_used=Magma+Eruption", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp lairActionPlanResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp.AvailableActions, 1)
	assert.Equal(t, "Tremor", resp.AvailableActions[0].Name)
	require.Len(t, resp.DisabledActions, 1)
	assert.Equal(t, "Magma Eruption", resp.DisabledActions[0].Name)
}

// --- Error-path tests for ExecuteLairAction ---

func TestExecuteLairAction_InvalidEncounterID(t *testing.T) {
	svc := NewService(&mockStore{})
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("POST", "/api/combat/not-a-uuid/lair-action", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestExecuteLairAction_InvalidJSON(t *testing.T) {
	npcID := uuid.New()
	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: npcID, DisplayName: "Dragon", IsNpc: true, IsAlive: true, CreatureRefID: sql.NullString{String: "dragon", Valid: true}},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID: "dragon", Name: "Dragon",
				Abilities: pqtype.NullRawMessage{Valid: true, RawMessage: json.RawMessage(`[
					{"name":"Lair Actions","description":"On init 20..."},
					{"name":"Tremor","description":"shakes."}
				]`)},
			}, nil
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/lair-action", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid JSON body")
}

func TestExecuteLairAction_FindLairCreatureError(t *testing.T) {
	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	body := `{"action_name": "Tremor", "last_used_action": ""}`
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/lair-action", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestExecuteLairAction_NoLairActions(t *testing.T) {
	npcID := uuid.New()
	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: npcID, DisplayName: "Dragon", IsNpc: true, IsAlive: true, CreatureRefID: sql.NullString{String: "dragon", Valid: true}},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{ID: "dragon", Name: "Dragon"}, nil
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	body := `{"action_name": "Tremor", "last_used_action": ""}`
	req := httptest.NewRequest("POST", "/api/combat/"+uuid.New().String()+"/lair-action", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- Error-path tests for GetTurnQueue ---

func TestGetTurnQueue_InvalidEncounterID(t *testing.T) {
	svc := NewService(&mockStore{})
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/not-a-uuid/turn-queue", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestGetTurnQueue_ListCombatantsError(t *testing.T) {
	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+uuid.New().String()+"/turn-queue", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to list combatants")
}

func TestGetTurnQueue_GetCreatureError(t *testing.T) {
	npcID := uuid.New()
	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: npcID, DisplayName: "Dragon", InitiativeRoll: 15, IsNpc: true, IsAlive: true, CreatureRefID: sql.NullString{String: "dragon", Valid: true}},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{}, fmt.Errorf("not found")
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+uuid.New().String()+"/turn-queue", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should still succeed but without legendary/lair entries (creature error is skipped)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp turnQueueResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Len(t, resp.Entries, 1) // only the combatant, no legendary/lair
}

// --- findLairCreature error-path tests ---

func TestFindLairCreature_NpcWithoutCreatureRef(t *testing.T) {
	encounterID := uuid.New()
	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: uuid.New(), DisplayName: "Spirit", IsNpc: true, IsAlive: true, CreatureRefID: sql.NullString{Valid: false}},
			}, nil
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/lair-action/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestFindLairCreature_DeadNpc(t *testing.T) {
	encounterID := uuid.New()
	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: uuid.New(), DisplayName: "Dragon", IsNpc: true, IsAlive: false, CreatureRefID: sql.NullString{String: "dragon", Valid: true}},
			}, nil
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/lair-action/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestFindLairCreature_PlayerCombatant(t *testing.T) {
	encounterID := uuid.New()
	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: uuid.New(), DisplayName: "Aragorn", IsNpc: false, IsAlive: true},
			}, nil
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/lair-action/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestFindLairCreature_GetCreatureError(t *testing.T) {
	encounterID := uuid.New()
	store := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: uuid.New(), DisplayName: "Dragon", IsNpc: true, IsAlive: true, CreatureRefID: sql.NullString{String: "dragon", Valid: true}},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{}, fmt.Errorf("db error")
		},
	}
	svc := NewService(store)
	handler := NewHandler(svc, dice.NewRoller(nil))

	r := chi.NewRouter()
	r.Route("/api/combat", func(r chi.Router) { handler.RegisterLegendaryRoutes(r) })

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/lair-action/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
