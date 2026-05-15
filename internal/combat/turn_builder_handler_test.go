package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

// --- TDD Cycle 11: GET /api/combat/{encounterID}/enemy-turn/{combatantID}/plan ---

func TestGetEnemyTurnPlan_Success(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	pcID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == npcID {
				return refdata.Combatant{
					ID:            npcID,
					EncounterID:   encounterID,
					DisplayName:   "Goblin",
					PositionCol:   "C",
					PositionRow:   3,
					IsNpc:         true,
					IsAlive:       true,
					HpCurrent:     10,
					CreatureRefID: sql.NullString{String: "goblin", Valid: true},
				}, nil
			}
			return refdata.Combatant{}, sql.ErrNoRows
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID:            "goblin",
				Name:          "Goblin",
				Size:          "Small",
				Speed:         json.RawMessage(`{"walk":30}`),
				Attacks:       json.RawMessage(`[{"name":"Scimitar","to_hit":4,"damage":"1d6+2","damage_type":"slashing","reach_ft":5}]`),
				AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`),
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{
					ID:          npcID,
					DisplayName: "Goblin",
					PositionCol: "C",
					PositionRow: 3,
					IsNpc:       true,
					IsAlive:     true,
					HpCurrent:   10,
				},
				{
					ID:          pcID,
					DisplayName: "Aragorn",
					PositionCol: "C",
					PositionRow: 5,
					IsNpc:       false,
					IsAlive:     true,
					HpCurrent:   45,
					Ac:          16,
				},
			}, nil
		},
		listActiveReactionDeclarationsByEncounterFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
			return nil, nil
		},
	}

	svc := NewService(store)
	roller := dice.NewRoller(func(max int) int { return 10 })
	handler := NewHandler(svc, roller)

	r := chi.NewRouter()
	handler.RegisterEnemyTurnRoutes(r)

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/enemy-turn/"+npcID.String()+"/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp enemyTurnPlanResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, npcID.String(), resp.CombatantID)
	assert.Equal(t, "Goblin", resp.DisplayName)
	assert.GreaterOrEqual(t, len(resp.Steps), 1) // at least an attack step
}

func TestGetEnemyTurnPlan_NotNPC(t *testing.T) {
	encounterID := uuid.New()
	pcID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          pcID,
				DisplayName: "Aragorn",
				IsNpc:       false,
			}, nil
		},
	}

	svc := NewService(store)
	roller := dice.NewRoller(nil)
	handler := NewHandler(svc, roller)

	r := chi.NewRouter()
	handler.RegisterEnemyTurnRoutes(r)

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/enemy-turn/"+pcID.String()+"/plan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- TDD Cycle 12: POST /api/combat/{encounterID}/enemy-turn ---

func TestExecuteEnemyTurn_Success(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	hpUpdated := false
	actionLogCreated := false
	turnActionsUpdated := false

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == npcID {
				return refdata.Combatant{
					ID:          npcID,
					DisplayName: "Goblin",
					IsNpc:       true,
					IsAlive:     true,
				}, nil
			}
			return refdata.Combatant{
				ID:        targetID,
				DisplayName: "Aragorn",
				IsNpc:     false,
				IsAlive:   true,
				HpCurrent: 45,
				Ac:        16,
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:          turnID,
				EncounterID: eid,
				CombatantID: npcID,
			}, nil
		},
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			hpUpdated = true
			return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			actionLogCreated = true
			assert.Equal(t, "enemy_turn", arg.ActionType)
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
		updateTurnActionsFn: func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			turnActionsUpdated = true
			return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
		},
	}

	svc := NewService(store)
	// Deterministic roller: d20=15, damage=4+3 (7 total)
	roller := newDeterministicRoller(15, 4, 3)
	handler := NewHandler(svc, roller)

	r := chi.NewRouter()
	handler.RegisterEnemyTurnRoutes(r)

	body := `{
		"combatant_id": "` + npcID.String() + `",
		"steps": [
			{
				"type": "attack",
				"attack": {
					"weapon_name": "Scimitar",
					"to_hit": 4,
					"damage_dice": "1d6+2",
					"damage_type": "slashing",
					"reach_ft": 5,
					"target_id": "` + targetID.String() + `",
					"target_name": "Aragorn"
				}
			}
		]
	}`

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/enemy-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp executeEnemyTurnResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.CombatLog)

	assert.True(t, hpUpdated, "HP should be updated")
	assert.True(t, actionLogCreated, "Action log should be created")
	assert.True(t, turnActionsUpdated, "Turn actions should be updated")
}

// --- TDD Cycle 13: indexToColLabel ---

func TestIndexToColLabel(t *testing.T) {
	assert.Equal(t, "A", indexToColLabel(0))
	assert.Equal(t, "B", indexToColLabel(1))
	assert.Equal(t, "Z", indexToColLabel(25))
	assert.Equal(t, "AA", indexToColLabel(26))
}

// --- TDD Cycle 14: EnemyTurnNotifier called after execute ---

type mockEnemyTurnNotifier struct {
	called      bool
	encounterID uuid.UUID
	combatLog   string
	done        chan struct{}
}

func newMockEnemyTurnNotifier() *mockEnemyTurnNotifier {
	return &mockEnemyTurnNotifier{done: make(chan struct{})}
}

func (m *mockEnemyTurnNotifier) NotifyEnemyTurnExecuted(ctx context.Context, encounterID uuid.UUID, combatLog string) {
	m.called = true
	m.encounterID = encounterID
	m.combatLog = combatLog
	close(m.done)
}

func TestExecuteEnemyTurn_NotifiesOnSuccess(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == npcID {
				return refdata.Combatant{
					ID:          npcID,
					DisplayName: "Goblin",
					IsNpc:       true,
					IsAlive:     true,
				}, nil
			}
			return refdata.Combatant{
				ID:          targetID,
				DisplayName: "Aragorn",
				IsNpc:       false,
				IsAlive:     true,
				HpCurrent:   45,
				Ac:          16,
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: eid, CombatantID: npcID}, nil
		},
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
		updateTurnActionsFn: func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
		},
	}

	svc := NewService(store)
	roller := newDeterministicRoller(15, 4, 3)
	handler := NewHandler(svc, roller)

	notifier := newMockEnemyTurnNotifier()
	handler.SetEnemyTurnNotifier(notifier)

	r := chi.NewRouter()
	handler.RegisterEnemyTurnRoutes(r)

	body := `{
		"combatant_id": "` + npcID.String() + `",
		"steps": [
			{
				"type": "attack",
				"attack": {
					"weapon_name": "Scimitar",
					"to_hit": 4,
					"damage_dice": "1d6+2",
					"damage_type": "slashing",
					"reach_ft": 5,
					"target_id": "` + targetID.String() + `",
					"target_name": "Aragorn"
				}
			}
		]
	}`

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/enemy-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Wait for async goroutine to complete
	select {
	case <-notifier.done:
	case <-time.After(2 * time.Second):
		t.Fatal("notifier was not called within timeout")
	}

	assert.True(t, notifier.called, "notifier should be called after successful execution")
	assert.Equal(t, encounterID, notifier.encounterID)
	assert.NotEmpty(t, notifier.combatLog)
}

func TestExecuteEnemyTurn_NoNotifierNoPanic(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          npcID,
				DisplayName: "Goblin",
				IsNpc:       true,
				IsAlive:     true,
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: eid, CombatantID: npcID}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
		updateTurnActionsFn: func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{ID: arg.ID}, nil
		},
	}

	svc := NewService(store)
	roller := newDeterministicRoller()
	handler := NewHandler(svc, roller)
	// No notifier set — should not panic

	r := chi.NewRouter()
	handler.RegisterEnemyTurnRoutes(r)

	body := `{"combatant_id": "` + npcID.String() + `", "steps": []}`

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/enemy-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// --- F-11: ExecuteEnemyTurn publishes WebSocket snapshot ---

func TestExecuteEnemyTurn_F11_PublishesSnapshot(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          npcID,
				EncounterID: encounterID,
				DisplayName: "Goblin",
				IsNpc:       true,
				IsAlive:     true,
			}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: eid, CombatantID: npcID}, nil
		},
		updateCombatantPositionFn: func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
		updateTurnActionsFn: func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
		},
	}

	pub := &fakePublisher{}
	svc := NewService(store)
	svc.SetPublisher(pub)

	plan := TurnPlan{
		CombatantID: npcID,
		Steps: []TurnStep{
			{
				Type: StepTypeMovement,
				Movement: &MovementStep{
					Path: []pathfinding.Point{{Col: 3, Row: 4}},
				},
			},
		},
	}

	roller := dice.NewRoller(nil)
	_, err := svc.ExecuteEnemyTurn(context.Background(), encounterID, plan, roller)
	require.NoError(t, err)

	require.Equal(t, []uuid.UUID{encounterID}, pub.calls(),
		"ExecuteEnemyTurn must publish a WebSocket snapshot after mutations")
}

// --- F-12: GenerateEnemyTurnPlan uses encounter's actual map ---

func TestGenerateEnemyTurnPlan_F12_UsesEncounterMap(t *testing.T) {
	encounterID := uuid.New()
	mapID := uuid.New()
	npcID := uuid.New()
	pcID := uuid.New()

	// Build a 10x8 map with a wall blocking the direct path.
	// The tiled JSON has a terrain layer (10x8 open) and a walls layer with one wall.
	tiledJSON := json.RawMessage(`{
		"width": 10,
		"height": 8,
		"tilewidth": 48,
		"tileheight": 48,
		"tilesets": [],
		"layers": [
			{
				"name": "terrain",
				"type": "tilelayer",
				"width": 10,
				"height": 8,
				"data": [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
			},
			{
				"name": "walls",
				"type": "objectgroup",
				"objects": [
					{"x": 144, "y": 0, "width": 0, "height": 384}
				]
			}
		]
	}`)

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:            npcID,
				EncounterID:   encounterID,
				DisplayName:   "Goblin",
				PositionCol:   "A",
				PositionRow:   1,
				IsNpc:         true,
				IsAlive:       true,
				HpCurrent:     10,
				CreatureRefID: sql.NullString{String: "goblin", Valid: true},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID:            "goblin",
				Name:          "Goblin",
				Size:          "Small",
				Speed:         json.RawMessage(`{"walk":30}`),
				Attacks:       json.RawMessage(`[{"name":"Scimitar","to_hit":4,"damage":"1d6+2","damage_type":"slashing","reach_ft":5}]`),
				AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`),
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: npcID, DisplayName: "Goblin", PositionCol: "A", PositionRow: 1, IsNpc: true, IsAlive: true, HpCurrent: 10},
				{ID: pcID, DisplayName: "Aragorn", PositionCol: "E", PositionRow: 1, IsNpc: false, IsAlive: true, HpCurrent: 45, Ac: 16},
			}, nil
		},
		listActiveReactionDeclarationsByEncounterFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
			return nil, nil
		},
		getEncounterFn: func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:     encounterID,
				MapID:  uuid.NullUUID{UUID: mapID, Valid: true},
				Status: "active",
			}, nil
		},
		getMapByIDUncheckedFn: func(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
			assert.Equal(t, mapID, id, "should load the encounter's map")
			return refdata.Map{
				ID:            mapID,
				WidthSquares:  10,
				HeightSquares: 8,
				TiledJson:     tiledJSON,
			}, nil
		},
	}

	svc := NewService(store)
	plan, err := svc.GenerateEnemyTurnPlan(context.Background(), encounterID, npcID)
	require.NoError(t, err)
	require.NotNil(t, plan)

	// The plan should have a movement step. With a 10x8 map and a wall,
	// the path should respect the map dimensions (not default 20x20).
	// Verify the plan was generated (movement + attack).
	assert.GreaterOrEqual(t, len(plan.Steps), 1)

	// Verify the movement step exists and its path stays within 10x8 bounds.
	if len(plan.Steps) > 0 && plan.Steps[0].Type == StepTypeMovement && plan.Steps[0].Movement != nil {
		for _, pt := range plan.Steps[0].Movement.Path {
			assert.Less(t, pt.Col, 10, "path col must be within map width 10")
			assert.Less(t, pt.Row, 8, "path row must be within map height 8")
		}
	}
}

func TestGenerateEnemyTurnPlan_F12_FallsBackToDefaultGrid(t *testing.T) {
	encounterID := uuid.New()
	npcID := uuid.New()
	pcID := uuid.New()

	store := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:            npcID,
				EncounterID:   encounterID,
				DisplayName:   "Goblin",
				PositionCol:   "C",
				PositionRow:   3,
				IsNpc:         true,
				IsAlive:       true,
				HpCurrent:     10,
				CreatureRefID: sql.NullString{String: "goblin", Valid: true},
			}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{
				ID:            "goblin",
				Name:          "Goblin",
				Size:          "Small",
				Speed:         json.RawMessage(`{"walk":30}`),
				Attacks:       json.RawMessage(`[{"name":"Scimitar","to_hit":4,"damage":"1d6+2","damage_type":"slashing","reach_ft":5}]`),
				AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`),
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: npcID, DisplayName: "Goblin", PositionCol: "C", PositionRow: 3, IsNpc: true, IsAlive: true, HpCurrent: 10},
				{ID: pcID, DisplayName: "Aragorn", PositionCol: "C", PositionRow: 5, IsNpc: false, IsAlive: true, HpCurrent: 45, Ac: 16},
			}, nil
		},
		listActiveReactionDeclarationsByEncounterFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
			return nil, nil
		},
		// GetEncounter returns an encounter with no map
		getEncounterFn: func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encounterID, Status: "active"}, nil
		},
	}

	svc := NewService(store)
	plan, err := svc.GenerateEnemyTurnPlan(context.Background(), encounterID, npcID)
	require.NoError(t, err)
	require.NotNil(t, plan)
	// Should still produce a valid plan using the 20x20 fallback
	assert.GreaterOrEqual(t, len(plan.Steps), 1)
}
