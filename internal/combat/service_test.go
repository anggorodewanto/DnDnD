package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// mockStore implements Store for unit tests.
type mockStore struct {
	createEncounterFn             func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error)
	getEncounterFn                func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	listEncountersByCampaignIDFn  func(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error)
	updateEncounterStatusFn       func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error)
	updateEncounterCurrentTurnFn  func(ctx context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error)
	updateEncounterRoundFn        func(ctx context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error)
	deleteEncounterFn             func(ctx context.Context, id uuid.UUID) error
	createCombatantFn             func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error)
	getCombatantFn                func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
	listCombatantsByEncounterIDFn func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	updateCombatantHPFn           func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error)
	updateCombatantConditionsFn   func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error)
	updateCombatantPositionFn     func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error)
	deleteCombatantFn             func(ctx context.Context, id uuid.UUID) error
	createTurnFn                  func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error)
	getTurnFn                     func(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	getActiveTurnByEncounterIDFn  func(ctx context.Context, encounterID uuid.UUID) (refdata.Turn, error)
	completeTurnFn                func(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	createActionLogFn             func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error)
	listActionLogByEncounterIDFn  func(ctx context.Context, encounterID uuid.UUID) ([]refdata.ActionLog, error)
	listActionLogByTurnIDFn       func(ctx context.Context, turnID uuid.UUID) ([]refdata.ActionLog, error)
	getEncounterTemplateFn        func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error)
	getCreatureFn                 func(ctx context.Context, id string) (refdata.Creature, error)
}

func (m *mockStore) CreateEncounter(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
	return m.createEncounterFn(ctx, arg)
}
func (m *mockStore) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	return m.getEncounterFn(ctx, id)
}
func (m *mockStore) ListEncountersByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error) {
	return m.listEncountersByCampaignIDFn(ctx, campaignID)
}
func (m *mockStore) UpdateEncounterStatus(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
	return m.updateEncounterStatusFn(ctx, arg)
}
func (m *mockStore) UpdateEncounterCurrentTurn(ctx context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error) {
	return m.updateEncounterCurrentTurnFn(ctx, arg)
}
func (m *mockStore) UpdateEncounterRound(ctx context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error) {
	return m.updateEncounterRoundFn(ctx, arg)
}
func (m *mockStore) DeleteEncounter(ctx context.Context, id uuid.UUID) error {
	return m.deleteEncounterFn(ctx, id)
}
func (m *mockStore) CreateCombatant(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
	return m.createCombatantFn(ctx, arg)
}
func (m *mockStore) GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
	return m.getCombatantFn(ctx, id)
}
func (m *mockStore) ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return m.listCombatantsByEncounterIDFn(ctx, encounterID)
}
func (m *mockStore) UpdateCombatantHP(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
	return m.updateCombatantHPFn(ctx, arg)
}
func (m *mockStore) UpdateCombatantConditions(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
	return m.updateCombatantConditionsFn(ctx, arg)
}
func (m *mockStore) UpdateCombatantPosition(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
	return m.updateCombatantPositionFn(ctx, arg)
}
func (m *mockStore) DeleteCombatant(ctx context.Context, id uuid.UUID) error {
	return m.deleteCombatantFn(ctx, id)
}
func (m *mockStore) CreateTurn(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
	return m.createTurnFn(ctx, arg)
}
func (m *mockStore) GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	return m.getTurnFn(ctx, id)
}
func (m *mockStore) GetActiveTurnByEncounterID(ctx context.Context, encounterID uuid.UUID) (refdata.Turn, error) {
	return m.getActiveTurnByEncounterIDFn(ctx, encounterID)
}
func (m *mockStore) CompleteTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	return m.completeTurnFn(ctx, id)
}
func (m *mockStore) CreateActionLog(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
	return m.createActionLogFn(ctx, arg)
}
func (m *mockStore) ListActionLogByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.ActionLog, error) {
	return m.listActionLogByEncounterIDFn(ctx, encounterID)
}
func (m *mockStore) ListActionLogByTurnID(ctx context.Context, turnID uuid.UUID) ([]refdata.ActionLog, error) {
	return m.listActionLogByTurnIDFn(ctx, turnID)
}
func (m *mockStore) GetEncounterTemplate(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
	return m.getEncounterTemplateFn(ctx, id)
}
func (m *mockStore) GetCreature(ctx context.Context, id string) (refdata.Creature, error) {
	return m.getCreatureFn(ctx, id)
}

func defaultMockStore() *mockStore {
	encounterID := uuid.New()
	combatantID := uuid.New()
	return &mockStore{
		createEncounterFn: func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:         encounterID,
				CampaignID: arg.CampaignID,
				Name:       arg.Name,
				Status:     arg.Status,
			}, nil
		},
		getEncounterFn: func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: id, Name: "Test", Status: "preparing"}, nil
		},
		listEncountersByCampaignIDFn: func(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{}, nil
		},
		updateEncounterStatusFn: func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
			return refdata.Encounter{ID: arg.ID, Status: arg.Status}, nil
		},
		updateEncounterCurrentTurnFn: func(ctx context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error) {
			return refdata.Encounter{ID: arg.ID, CurrentTurnID: arg.CurrentTurnID}, nil
		},
		updateEncounterRoundFn: func(ctx context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error) {
			return refdata.Encounter{ID: arg.ID, RoundNumber: arg.RoundNumber}, nil
		},
		deleteEncounterFn: func(ctx context.Context, id uuid.UUID) error { return nil },
		createCombatantFn: func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          combatantID,
				EncounterID: arg.EncounterID,
				ShortID:     arg.ShortID,
				DisplayName: arg.DisplayName,
				HpMax:       arg.HpMax,
				HpCurrent:   arg.HpCurrent,
				Ac:          arg.Ac,
				IsAlive:     true,
				IsVisible:   true,
				Conditions:  json.RawMessage(`[]`),
			}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Test", Conditions: json.RawMessage(`[]`)}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{}, nil
		},
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, Conditions: json.RawMessage(`[]`)}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
		},
		updateCombatantPositionFn: func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow, Conditions: json.RawMessage(`[]`)}, nil
		},
		deleteCombatantFn: func(ctx context.Context, id uuid.UUID) error { return nil },
		createTurnFn: func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
			return refdata.Turn{ID: uuid.New(), EncounterID: arg.EncounterID, Status: arg.Status}, nil
		},
		getTurnFn: func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: id, Status: "active"}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, encounterID uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: uuid.New(), EncounterID: encounterID, Status: "active"}, nil
		},
		completeTurnFn: func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: id, Status: "completed"}, nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New(), EncounterID: arg.EncounterID}, nil
		},
		listActionLogByEncounterIDFn: func(ctx context.Context, encounterID uuid.UUID) ([]refdata.ActionLog, error) {
			return []refdata.ActionLog{}, nil
		},
		listActionLogByTurnIDFn: func(ctx context.Context, turnID uuid.UUID) ([]refdata.ActionLog, error) {
			return []refdata.ActionLog{}, nil
		},
		getEncounterTemplateFn: func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{}, sql.ErrNoRows
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			return refdata.Creature{}, sql.ErrNoRows
		},
	}
}

// --- TDD Cycle 6: NewService returns non-nil ---

func TestNewCombatService(t *testing.T) {
	svc := NewService(defaultMockStore())
	assert.NotNil(t, svc)
}

// --- TDD Cycle 7: CreateEncounter validates name ---

func TestService_CreateEncounter_RejectsEmptyName(t *testing.T) {
	svc := NewService(defaultMockStore())
	_, err := svc.CreateEncounter(context.Background(), CreateEncounterInput{
		CampaignID: uuid.New(),
		Name:       "",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name must not be empty")
}

// --- TDD Cycle 8: CreateEncounter success ---

func TestService_CreateEncounter_Success(t *testing.T) {
	svc := NewService(defaultMockStore())
	enc, err := svc.CreateEncounter(context.Background(), CreateEncounterInput{
		CampaignID: uuid.New(),
		Name:       "Goblin Ambush",
	})
	require.NoError(t, err)
	assert.Equal(t, "Goblin Ambush", enc.Name)
	assert.Equal(t, "preparing", enc.Status)
}

// --- TDD Cycle 9: CreateEncounter store error ---

func TestService_CreateEncounter_StoreError(t *testing.T) {
	store := defaultMockStore()
	store.createEncounterFn = func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
		return refdata.Encounter{}, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.CreateEncounter(context.Background(), CreateEncounterInput{
		CampaignID: uuid.New(),
		Name:       "Test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating encounter")
}

// --- TDD Cycle 10: GetEncounter ---

func TestService_GetEncounter(t *testing.T) {
	id := uuid.New()
	svc := NewService(defaultMockStore())
	enc, err := svc.GetEncounter(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, enc.ID)
}

// --- TDD Cycle 11: ListEncountersByCampaign ---

func TestService_ListEncounters(t *testing.T) {
	svc := NewService(defaultMockStore())
	list, err := svc.ListEncountersByCampaignID(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotNil(t, list)
}

// --- TDD Cycle 12: UpdateEncounterStatus ---

func TestService_UpdateEncounterStatus(t *testing.T) {
	svc := NewService(defaultMockStore())
	enc, err := svc.UpdateEncounterStatus(context.Background(), uuid.New(), "active")
	require.NoError(t, err)
	assert.Equal(t, "active", enc.Status)
}

func TestService_UpdateEncounterStatus_InvalidStatus(t *testing.T) {
	svc := NewService(defaultMockStore())
	_, err := svc.UpdateEncounterStatus(context.Background(), uuid.New(), "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

// --- TDD Cycle 13: AddCombatant ---

func TestService_AddCombatant(t *testing.T) {
	svc := NewService(defaultMockStore())
	encID := uuid.New()
	params := CombatantParams{
		CreatureRefID: "goblin",
		ShortID:       "G1",
		DisplayName:   "Goblin 1",
		HPMax:         7,
		HPCurrent:     7,
		AC:            15,
		SpeedFt:       30,
		PositionCol:   "A",
		PositionRow:   1,
		IsNPC:         true,
		IsAlive:       true,
		IsVisible:     true,
	}
	c, err := svc.AddCombatant(context.Background(), encID, params)
	require.NoError(t, err)
	assert.Equal(t, "G1", c.ShortID)
}

func TestService_AddCombatant_StoreError(t *testing.T) {
	store := defaultMockStore()
	store.createCombatantFn = func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("db error")
	}
	svc := NewService(store)
	_, err := svc.AddCombatant(context.Background(), uuid.New(), CombatantParams{
		ShortID:     "G1",
		DisplayName: "Goblin",
		HPMax:       7,
		HPCurrent:   7,
		AC:          15,
		IsAlive:     true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating combatant")
}

// --- TDD Cycle 14: CreateEncounterFromTemplate ---

func TestService_CreateEncounterFromTemplate_Success(t *testing.T) {
	templateID := uuid.New()
	campaignID := uuid.New()
	mapID := uuid.New()

	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID:         templateID,
			CampaignID: campaignID,
			MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
			Name:       "Goblin Ambush",
			DisplayName: sql.NullString{String: "The Dark Forest", Valid: true},
			Creatures: json.RawMessage(`[
				{"creature_ref_id":"goblin","short_id":"G1","display_name":"Goblin 1","position_col":"A","position_row":1,"quantity":1}
			]`),
		}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:        "goblin",
			Name:      "Goblin",
			Ac:        15,
			HpAverage: 7,
			Speed:     json.RawMessage(`{"walk":30}`),
		}, nil
	}

	var createdCombatants []refdata.CreateCombatantParams
	store.createCombatantFn = func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
		createdCombatants = append(createdCombatants, arg)
		return refdata.Combatant{
			ID:          uuid.New(),
			EncounterID: arg.EncounterID,
			ShortID:     arg.ShortID,
			DisplayName: arg.DisplayName,
			HpMax:       arg.HpMax,
			HpCurrent:   arg.HpCurrent,
			Ac:          arg.Ac,
			IsAlive:     true,
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}

	svc := NewService(store)
	enc, combatants, err := svc.CreateEncounterFromTemplate(context.Background(), templateID)
	require.NoError(t, err)
	assert.Equal(t, "Goblin Ambush", enc.Name)
	assert.Equal(t, "preparing", enc.Status)
	require.Len(t, combatants, 1)
	assert.Equal(t, "G1", createdCombatants[0].ShortID)
	assert.Equal(t, int32(7), createdCombatants[0].HpMax)
	assert.Equal(t, int32(15), createdCombatants[0].Ac)
}

func TestService_CreateEncounterFromTemplate_TemplateNotFound(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{}, sql.ErrNoRows
	}
	svc := NewService(store)
	_, _, err := svc.CreateEncounterFromTemplate(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting encounter template")
}

func TestService_CreateEncounterFromTemplate_CreatureNotFound(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID:         uuid.New(),
			CampaignID: uuid.New(),
			Name:       "Test",
			Creatures:  json.RawMessage(`[{"creature_ref_id":"missing","short_id":"M1","display_name":"Missing","position_col":"A","position_row":1,"quantity":1}]`),
		}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{}, sql.ErrNoRows
	}
	svc := NewService(store)
	_, _, err := svc.CreateEncounterFromTemplate(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting creature")
}

func TestService_CreateEncounterFromTemplate_MultipleQuantity(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID:         uuid.New(),
			CampaignID: uuid.New(),
			Name:       "Test",
			Creatures:  json.RawMessage(`[{"creature_ref_id":"goblin","short_id":"G","display_name":"Goblin","position_col":"A","position_row":1,"quantity":3}]`),
		}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "goblin", Ac: 15, HpAverage: 7, Speed: json.RawMessage(`{"walk":30}`)}, nil
	}
	var count int
	store.createCombatantFn = func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
		count++
		return refdata.Combatant{
			ID:          uuid.New(),
			EncounterID: arg.EncounterID,
			ShortID:     arg.ShortID,
			DisplayName: arg.DisplayName,
			HpMax:       arg.HpMax,
			Ac:          arg.Ac,
			IsAlive:     true,
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}

	svc := NewService(store)
	_, combatants, err := svc.CreateEncounterFromTemplate(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Len(t, combatants, 3)
	assert.Equal(t, 3, count)
}

// --- TDD Cycle 15: DeleteEncounter ---

func TestService_DeleteEncounter(t *testing.T) {
	svc := NewService(defaultMockStore())
	err := svc.DeleteEncounter(context.Background(), uuid.New())
	require.NoError(t, err)
}

// --- TDD Cycle 16: GetCombatant ---

func TestService_GetCombatant(t *testing.T) {
	id := uuid.New()
	svc := NewService(defaultMockStore())
	c, err := svc.GetCombatant(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, c.ID)
}

// --- TDD Cycle 17: ListCombatants ---

func TestService_ListCombatants(t *testing.T) {
	svc := NewService(defaultMockStore())
	list, err := svc.ListCombatantsByEncounterID(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotNil(t, list)
}

// --- TDD Cycle 18: UpdateCombatantHP ---

func TestService_UpdateCombatantHP(t *testing.T) {
	svc := NewService(defaultMockStore())
	c, err := svc.UpdateCombatantHP(context.Background(), uuid.New(), 10, 0, true)
	require.NoError(t, err)
	assert.Equal(t, int32(10), c.HpCurrent)
}

// --- TDD Cycle 19: DeleteCombatant ---

func TestService_DeleteCombatant(t *testing.T) {
	svc := NewService(defaultMockStore())
	err := svc.DeleteCombatant(context.Background(), uuid.New())
	require.NoError(t, err)
}

// --- TDD Cycle 20: CreateEncounterFromTemplate with invalid creatures JSON ---

func TestService_CreateEncounterFromTemplate_InvalidCreaturesJSON(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID:         uuid.New(),
			CampaignID: uuid.New(),
			Name:       "Bad",
			Creatures:  json.RawMessage(`invalid`),
		}, nil
	}
	svc := NewService(store)
	_, _, err := svc.CreateEncounterFromTemplate(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing template creatures")
}

// --- TDD Cycle 21: ListActionLog ---

func TestService_ListActionLogByEncounterID(t *testing.T) {
	svc := NewService(defaultMockStore())
	logs, err := svc.ListActionLogByEncounterID(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotNil(t, logs)
}

func TestService_ListActionLogByTurnID(t *testing.T) {
	svc := NewService(defaultMockStore())
	logs, err := svc.ListActionLogByTurnID(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotNil(t, logs)
}

// --- TDD Cycle 22: CreateActionLog ---

func TestService_CreateActionLog(t *testing.T) {
	svc := NewService(defaultMockStore())
	log, err := svc.CreateActionLog(context.Background(), CreateActionLogInput{
		TurnID:      uuid.New(),
		EncounterID: uuid.New(),
		ActionType:  "attack",
		ActorID:     uuid.New(),
		BeforeState: json.RawMessage(`{}`),
		AfterState:  json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, log.ID)
}

func TestService_CreateActionLog_EmptyActionType(t *testing.T) {
	svc := NewService(defaultMockStore())
	_, err := svc.CreateActionLog(context.Background(), CreateActionLogInput{
		TurnID:      uuid.New(),
		EncounterID: uuid.New(),
		ActionType:  "",
		ActorID:     uuid.New(),
		BeforeState: json.RawMessage(`{}`),
		AfterState:  json.RawMessage(`{}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "action_type must not be empty")
}

// --- TDD Cycle 23: UpdateCombatantPosition ---

func TestService_UpdateCombatantPosition(t *testing.T) {
	svc := NewService(defaultMockStore())
	c, err := svc.UpdateCombatantPosition(context.Background(), uuid.New(), "C", 5, 0)
	require.NoError(t, err)
	assert.Equal(t, "C", c.PositionCol)
	assert.Equal(t, int32(5), c.PositionRow)
}

// --- TDD Cycle 24: UpdateCombatantConditions ---

func TestService_UpdateCombatantConditions(t *testing.T) {
	svc := NewService(defaultMockStore())
	conds := json.RawMessage(`["poisoned"]`)
	c, err := svc.UpdateCombatantConditions(context.Background(), uuid.New(), conds, 1)
	require.NoError(t, err)
	assert.Equal(t, conds, c.Conditions)
}

// --- TDD Cycle 25: CreateEncounterFromTemplate encounter create error ---

func TestService_CreateEncounterFromTemplate_CreateEncounterError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID:         uuid.New(),
			CampaignID: uuid.New(),
			Name:       "Test",
			Creatures:  json.RawMessage(`[]`),
		}, nil
	}
	store.createEncounterFn = func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
		return refdata.Encounter{}, errors.New("db error")
	}
	svc := NewService(store)
	_, _, err := svc.CreateEncounterFromTemplate(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating encounter")
}

// --- TDD Cycle 26: CreateEncounterFromTemplate combatant create error ---

func TestService_CreateEncounterFromTemplate_CombatantCreateError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID:         uuid.New(),
			CampaignID: uuid.New(),
			Name:       "Test",
			Creatures:  json.RawMessage(`[{"creature_ref_id":"goblin","short_id":"G1","display_name":"Goblin","position_col":"A","position_row":1,"quantity":1}]`),
		}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "goblin", Ac: 15, HpAverage: 7, Speed: json.RawMessage(`{"walk":30}`)}, nil
	}
	store.createCombatantFn = func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, errors.New("db error")
	}
	svc := NewService(store)
	_, _, err := svc.CreateEncounterFromTemplate(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating combatant")
}

// --- TDD Cycle 27: AddCombatant with character ID ---

func TestService_AddCombatant_WithCharacterID(t *testing.T) {
	charID := uuid.New()
	svc := NewService(defaultMockStore())
	c, err := svc.AddCombatant(context.Background(), uuid.New(), CombatantParams{
		CharacterID: charID.String(),
		ShortID:     "AR",
		DisplayName: "Aragorn",
		HPMax:       45,
		HPCurrent:   45,
		AC:          18,
		IsAlive:     true,
		IsVisible:   true,
		DeathSaves:  json.RawMessage(`{"successes":0,"failures":0}`),
		PositionCol: "D",
		PositionRow: 5,
	})
	require.NoError(t, err)
	assert.Equal(t, "AR", c.ShortID)
}

func TestService_AddCombatant_InvalidCharacterID(t *testing.T) {
	svc := NewService(defaultMockStore())
	_, err := svc.AddCombatant(context.Background(), uuid.New(), CombatantParams{
		CharacterID: "not-a-uuid",
		ShortID:     "AR",
		DisplayName: "Aragorn",
		HPMax:       45,
		HPCurrent:   45,
		AC:          18,
		IsAlive:     true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing character_id")
}

// --- TDD Cycle 28: CreateActionLog with description and dice rolls ---

func TestService_CreateActionLog_WithOptionalFields(t *testing.T) {
	svc := NewService(defaultMockStore())
	log, err := svc.CreateActionLog(context.Background(), CreateActionLogInput{
		TurnID:      uuid.New(),
		EncounterID: uuid.New(),
		ActionType:  "attack",
		ActorID:     uuid.New(),
		TargetID:    uuid.NullUUID{UUID: uuid.New(), Valid: true},
		Description: "Goblin attacks Aragorn with a scimitar",
		BeforeState: json.RawMessage(`{"hp":45}`),
		AfterState:  json.RawMessage(`{"hp":39}`),
		DiceRolls:   json.RawMessage(`[{"type":"d20","result":15}]`),
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, log.ID)
}

// Use unused imports to satisfy compiler
var _ = pqtype.NullRawMessage{}
