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

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// mockWorkspaceStore implements WorkspaceStore for tests.
type mockWorkspaceStore struct {
	listEncountersByCampaignIDFn      func(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error)
	listCombatantsByEncounterIDFn     func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	getMapByIDFn                      func(ctx context.Context, id uuid.UUID) (refdata.Map, error)
	listEncounterZonesByEncounterIDFn func(ctx context.Context, encounterID uuid.UUID) ([]refdata.EncounterZone, error)
	updateCombatantHPFn               func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error)
	updateCombatantConditionsFn       func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error)
	getActiveTurnByEncounterIDFn      func(ctx context.Context, encounterID uuid.UUID) (refdata.Turn, error)
	getCombatantByIDFn                func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
	updateCombatantPositionFn         func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error)
	deleteCombatantFn                 func(ctx context.Context, id uuid.UUID) error
	getCharacterFn                    func(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	getCreatureFn                     func(ctx context.Context, id string) (refdata.Creature, error)
	countPendingDMQueueByCampaignFn   func(ctx context.Context, campaignID uuid.UUID) (int64, error)
	countPendingDMQueueByEncounterFn  func(ctx context.Context, encounterID uuid.UUID) (int64, error)
}

// mockWorkspaceSvc implements WorkspaceCombatService for tests.
type mockWorkspaceSvc struct {
	updateCombatantHPFn         func(ctx context.Context, id uuid.UUID, hpCurrent, tempHP int32, isAlive bool) (refdata.Combatant, error)
	updateCombatantConditionsFn func(ctx context.Context, id uuid.UUID, conditions json.RawMessage, exhaustion int32) (refdata.Combatant, error)
	updateCombatantPositionFn   func(ctx context.Context, id uuid.UUID, col string, row, altitude int32) (refdata.Combatant, error)
	getCombatantFn              func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
}

func (m *mockWorkspaceSvc) UpdateCombatantHP(ctx context.Context, id uuid.UUID, hpCurrent, tempHP int32, isAlive bool) (refdata.Combatant, error) {
	return m.updateCombatantHPFn(ctx, id, hpCurrent, tempHP, isAlive)
}
func (m *mockWorkspaceSvc) UpdateCombatantConditions(ctx context.Context, id uuid.UUID, conditions json.RawMessage, exhaustion int32) (refdata.Combatant, error) {
	return m.updateCombatantConditionsFn(ctx, id, conditions, exhaustion)
}
func (m *mockWorkspaceSvc) UpdateCombatantPosition(ctx context.Context, id uuid.UUID, col string, row, altitude int32) (refdata.Combatant, error) {
	return m.updateCombatantPositionFn(ctx, id, col, row, altitude)
}
func (m *mockWorkspaceSvc) GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
	return m.getCombatantFn(ctx, id)
}

func (m *mockWorkspaceStore) ListEncountersByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error) {
	return m.listEncountersByCampaignIDFn(ctx, campaignID)
}
func (m *mockWorkspaceStore) ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return m.listCombatantsByEncounterIDFn(ctx, encounterID)
}
func (m *mockWorkspaceStore) GetMapByID(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
	return m.getMapByIDFn(ctx, id)
}
func (m *mockWorkspaceStore) ListEncounterZonesByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.EncounterZone, error) {
	return m.listEncounterZonesByEncounterIDFn(ctx, encounterID)
}
func (m *mockWorkspaceStore) UpdateCombatantHP(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
	return m.updateCombatantHPFn(ctx, arg)
}
func (m *mockWorkspaceStore) UpdateCombatantConditions(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
	return m.updateCombatantConditionsFn(ctx, arg)
}
func (m *mockWorkspaceStore) GetActiveTurnByEncounterID(ctx context.Context, encounterID uuid.UUID) (refdata.Turn, error) {
	return m.getActiveTurnByEncounterIDFn(ctx, encounterID)
}
func (m *mockWorkspaceStore) GetCombatantByID(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
	return m.getCombatantByIDFn(ctx, id)
}
func (m *mockWorkspaceStore) UpdateCombatantPosition(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
	return m.updateCombatantPositionFn(ctx, arg)
}
func (m *mockWorkspaceStore) DeleteCombatant(ctx context.Context, id uuid.UUID) error {
	return m.deleteCombatantFn(ctx, id)
}
func (m *mockWorkspaceStore) GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
	if m.getCharacterFn == nil {
		return refdata.Character{}, sql.ErrNoRows
	}
	return m.getCharacterFn(ctx, id)
}
func (m *mockWorkspaceStore) GetCreature(ctx context.Context, id string) (refdata.Creature, error) {
	if m.getCreatureFn == nil {
		return refdata.Creature{}, sql.ErrNoRows
	}
	return m.getCreatureFn(ctx, id)
}
func (m *mockWorkspaceStore) CountPendingDMQueueItemsByCampaign(ctx context.Context, campaignID uuid.UUID) (int64, error) {
	if m.countPendingDMQueueByCampaignFn == nil {
		return 0, nil
	}
	return m.countPendingDMQueueByCampaignFn(ctx, campaignID)
}
func (m *mockWorkspaceStore) CountPendingDMQueueItemsByEncounter(ctx context.Context, encounterID uuid.UUID) (int64, error) {
	if m.countPendingDMQueueByEncounterFn == nil {
		return 0, nil
	}
	return m.countPendingDMQueueByEncounterFn(ctx, encounterID)
}

// --- TDD Cycle: parseCreatureWalkSpeed ---

func TestParseCreatureWalkSpeed(t *testing.T) {
	tests := []struct {
		name     string
		input    json.RawMessage
		expected int32
	}{
		{"walk speed present", json.RawMessage(`{"walk":30}`), 30},
		{"fly only defaults to 30", json.RawMessage(`{"fly":60}`), 30},
		{"walk 25", json.RawMessage(`{"walk":25,"fly":50}`), 25},
		{"empty JSON", json.RawMessage(`{}`), 30},
		{"nil input", nil, 30},
		{"invalid JSON", json.RawMessage(`not json`), 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseCreatureWalkSpeed(tt.input))
		})
	}
}

// --- TDD Cycle 10: GET /api/combat/workspace returns active encounters ---

func TestWorkspaceHandler_GetWorkspace_IncludesSpeedFt(t *testing.T) {
	campaignID := uuid.New()
	encounterID := uuid.New()
	mapID := uuid.New()
	pcCombatantID := uuid.New()
	npcCombatantID := uuid.New()
	charID := uuid.New()
	turnID := uuid.New()

	store := &mockWorkspaceStore{
		listEncountersByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{
				{
					ID: encounterID, CampaignID: campaignID, Name: "Speed Test",
					Status: "active", RoundNumber: 1,
					MapID:         uuid.NullUUID{UUID: mapID, Valid: true},
					CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
				},
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{
					ID: pcCombatantID, EncounterID: encounterID, ShortID: "PC1",
					DisplayName: "Fighter", HpMax: 30, HpCurrent: 30, Ac: 18,
					IsNpc: false, IsAlive: true, Conditions: json.RawMessage(`[]`),
					CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
				},
				{
					ID: npcCombatantID, EncounterID: encounterID, ShortID: "GO1",
					DisplayName: "Goblin", HpMax: 7, HpCurrent: 7, Ac: 15,
					IsNpc: true, IsAlive: true, Conditions: json.RawMessage(`[]`),
					CreatureRefID: sql.NullString{String: "goblin", Valid: true},
				},
			}, nil
		},
		getMapByIDFn: func(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
			return refdata.Map{ID: mapID, WidthSquares: 20, HeightSquares: 15, TiledJson: json.RawMessage(`{}`)}, nil
		},
		listEncounterZonesByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
			return []refdata.EncounterZone{}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, CombatantID: pcCombatantID}, nil
		},
		getCharacterFn: func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
			assert.Equal(t, charID, id)
			return refdata.Character{ID: charID, SpeedFt: 30}, nil
		},
		getCreatureFn: func(ctx context.Context, id string) (refdata.Creature, error) {
			assert.Equal(t, "goblin", id)
			return refdata.Creature{ID: "goblin", Speed: json.RawMessage(`{"walk":30}`)}, nil
		},
	}

	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/workspace?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp workspaceResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Encounters, 1)
	require.Len(t, resp.Encounters[0].Combatants, 2)

	// PC combatant should have speed_ft from character
	assert.Equal(t, int32(30), resp.Encounters[0].Combatants[0].SpeedFt)
	// NPC combatant should have speed_ft from creature walk speed
	assert.Equal(t, int32(30), resp.Encounters[0].Combatants[1].SpeedFt)
}

func TestWorkspaceHandler_GetWorkspace_Success(t *testing.T) {
	campaignID := uuid.New()
	encounterID := uuid.New()
	mapID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := &mockWorkspaceStore{
		listEncountersByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Encounter, error) {
			assert.Equal(t, campaignID, cid)
			return []refdata.Encounter{
				{
					ID:            encounterID,
					CampaignID:    campaignID,
					Name:          "Goblin Ambush",
					Status:        "active",
					RoundNumber:   3,
					MapID:         uuid.NullUUID{UUID: mapID, Valid: true},
					CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
				},
				{
					ID:         uuid.New(),
					CampaignID: campaignID,
					Name:       "Completed Fight",
					Status:     "completed",
				},
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{
					ID:          combatantID,
					EncounterID: encounterID,
					ShortID:     "GO1",
					DisplayName: "Goblin 1",
					HpMax:       7,
					HpCurrent:   5,
					TempHp:      0,
					Ac:          15,
					PositionCol: "D",
					PositionRow: 5,
					IsNpc:       true,
					IsAlive:     true,
					Conditions:  json.RawMessage(`["Prone"]`),
				},
			}, nil
		},
		getMapByIDFn: func(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
			return refdata.Map{
				ID:            mapID,
				WidthSquares:  20,
				HeightSquares: 15,
				TiledJson:     json.RawMessage(`{"width":20,"height":15}`),
			}, nil
		},
		listEncounterZonesByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
			return []refdata.EncounterZone{}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:          turnID,
				CombatantID: combatantID,
				RoundNumber: 3,
			}, nil
		},
	}

	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/workspace?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp workspaceResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Encounters, 1) // Only active encounters
	assert.Equal(t, "Goblin Ambush", resp.Encounters[0].Name)
	assert.Len(t, resp.Encounters[0].Combatants, 1)
	assert.Equal(t, "GO1", resp.Encounters[0].Combatants[0].ShortID)
	assert.NotNil(t, resp.Encounters[0].Map)
	assert.Equal(t, int32(20), resp.Encounters[0].Map.WidthSquares)
	assert.Equal(t, int32(3), resp.Encounters[0].RoundNumber)
	assert.Equal(t, combatantID.String(), resp.Encounters[0].ActiveTurnCombatantID)
}

// --- Phase 105: workspace response exposes display_name and active_turn_combatant_name ---

func TestWorkspaceHandler_GetWorkspace_IncludesDisplayNameAndTurnName(t *testing.T) {
	campaignID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := &mockWorkspaceStore{
		listEncountersByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{
				{
					ID: encounterID, CampaignID: campaignID,
					Name:          "Boss Fight",
					DisplayName:   sql.NullString{String: "The Shadows Stir", Valid: true},
					Status:        "active",
					RoundNumber:   2,
					CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
				},
				{
					ID: uuid.New(), CampaignID: campaignID,
					Name:          "Rooftop Ambush",
					Status:        "active",
					RoundNumber:   1,
					CurrentTurnID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
				},
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{
					ID: combatantID, EncounterID: eid,
					ShortID: "AR", DisplayName: "Aragorn",
					HpMax: 30, HpCurrent: 30, Ac: 18,
					IsAlive: true, Conditions: json.RawMessage(`[]`),
				},
			}, nil
		},
		listEncounterZonesByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
			return []refdata.EncounterZone{}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, CombatantID: combatantID}, nil
		},
	}

	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/workspace?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp workspaceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Encounters, 2)

	// First: display_name override wins, player-facing name is surfaced.
	assert.Equal(t, "The Shadows Stir", resp.Encounters[0].DisplayName)
	assert.Equal(t, "Aragorn", resp.Encounters[0].ActiveTurnCombatantName)

	// Second: no override, DisplayName falls back to internal name so the
	// DM always has something to render in the Encounter Overview bar.
	assert.Equal(t, "Rooftop Ambush", resp.Encounters[1].DisplayName)
}

// --- TDD Cycle 11: GET /api/combat/workspace missing campaign_id ---

func TestWorkspaceHandler_GetWorkspace_MissingCampaignID(t *testing.T) {
	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/workspace", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 12: GET /api/combat/workspace invalid campaign_id ---

func TestWorkspaceHandler_GetWorkspace_InvalidCampaignID(t *testing.T) {
	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/workspace?campaign_id=not-a-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 13: GET /api/combat/workspace with no active encounters ---

func TestWorkspaceHandler_GetWorkspace_NoActiveEncounters(t *testing.T) {
	campaignID := uuid.New()

	store := &mockWorkspaceStore{
		listEncountersByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{
				{ID: uuid.New(), Status: "completed"},
				{ID: uuid.New(), Status: "preparing"},
			}, nil
		},
	}

	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/workspace?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp workspaceResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Empty(t, resp.Encounters)
}

// --- TDD Cycle 14: PATCH combatant HP ---

func TestWorkspaceHandler_UpdateCombatantHP_Success(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	svc := &mockWorkspaceSvc{
		updateCombatantHPFn: func(ctx context.Context, id uuid.UUID, hpCurrent, tempHP int32, isAlive bool) (refdata.Combatant, error) {
			assert.Equal(t, combatantID, id)
			assert.Equal(t, int32(15), hpCurrent)
			assert.Equal(t, int32(0), tempHP)
			assert.True(t, isAlive)
			return refdata.Combatant{
				ID:         combatantID,
				HpCurrent:  15,
				TempHp:     0,
				IsAlive:    true,
				Conditions: json.RawMessage(`[]`),
			}, nil
		},
	}

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"hp_current":15,"temp_hp":0,"is_alive":true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/hp", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp workspaceCombatantResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, int32(15), resp.HpCurrent)
}

// --- TDD Cycle 15: PATCH combatant conditions ---

func TestWorkspaceHandler_UpdateCombatantConditions_Success(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	svc := &mockWorkspaceSvc{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: combatantID, ExhaustionLevel: 0}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, id uuid.UUID, conditions json.RawMessage, exhaustion int32) (refdata.Combatant, error) {
			assert.Equal(t, combatantID, id)
			return refdata.Combatant{
				ID:         combatantID,
				Conditions: json.RawMessage(`["Blinded","Prone"]`),
			}, nil
		},
	}

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"conditions":["Blinded","Prone"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/conditions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- TDD Cycle 16: PATCH combatant HP with invalid combatant ID ---

func TestWorkspaceHandler_UpdateCombatantHP_InvalidCombatantID(t *testing.T) {
	encounterID := uuid.New()

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"hp_current":15,"temp_hp":0,"is_alive":true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/not-a-uuid/hp", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 17: GET workspace encounter without map ---

func TestWorkspaceHandler_GetWorkspace_EncounterWithoutMap(t *testing.T) {
	campaignID := uuid.New()
	encounterID := uuid.New()

	store := &mockWorkspaceStore{
		listEncountersByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{
				{
					ID:          encounterID,
					CampaignID:  campaignID,
					Name:        "No Map Fight",
					Status:      "active",
					RoundNumber: 1,
					MapID:       uuid.NullUUID{Valid: false},
				},
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{}, nil
		},
		listEncounterZonesByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
			return []refdata.EncounterZone{}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, sql.ErrNoRows
		},
	}

	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/workspace?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp workspaceResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Encounters, 1)
	assert.Nil(t, resp.Encounters[0].Map)
}

// --- Edge case tests for coverage ---

func TestWorkspaceHandler_UpdateCombatantHP_InvalidJSON(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/hp", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWorkspaceHandler_UpdateCombatantHP_StoreError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	svc := &mockWorkspaceSvc{
		updateCombatantHPFn: func(ctx context.Context, id uuid.UUID, hpCurrent, tempHP int32, isAlive bool) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("db error")
		},
	}

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"hp_current":15,"temp_hp":0,"is_alive":true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/hp", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWorkspaceHandler_UpdateCombatantConditions_InvalidJSON(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/conditions", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWorkspaceHandler_UpdateCombatantConditions_InvalidCombatantID(t *testing.T) {
	encounterID := uuid.New()

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"conditions":["Blinded"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/not-a-uuid/conditions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 21: UpdateCombatantConditions preserves exhaustion level ---

func TestWorkspaceHandler_UpdateCombatantConditions_PreservesExhaustionLevel(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	svc := &mockWorkspaceSvc{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			assert.Equal(t, combatantID, id)
			return refdata.Combatant{
				ID:              combatantID,
				ExhaustionLevel: 3,
				Conditions:      json.RawMessage(`[]`),
			}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, id uuid.UUID, conditions json.RawMessage, exhaustion int32) (refdata.Combatant, error) {
			assert.Equal(t, combatantID, id)
			assert.Equal(t, int32(3), exhaustion, "exhaustion level must be preserved from existing combatant")
			return refdata.Combatant{
				ID:              combatantID,
				Conditions:      json.RawMessage(`["Blinded"]`),
				ExhaustionLevel: 3,
			}, nil
		},
	}

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"conditions":["Blinded"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/conditions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, float64(3), resp["exhaustion_level"])
}

func TestWorkspaceHandler_UpdateCombatantConditions_GetCombatantError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	svc := &mockWorkspaceSvc{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("not found")
		},
	}

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"conditions":["Blinded"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/conditions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWorkspaceHandler_UpdateCombatantConditions_StoreError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	svc := &mockWorkspaceSvc{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: combatantID, ExhaustionLevel: 0}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, id uuid.UUID, conditions json.RawMessage, exhaustion int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("db error")
		},
	}

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"conditions":["Blinded"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/conditions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWorkspaceHandler_GetWorkspace_ListEncountersError(t *testing.T) {
	campaignID := uuid.New()

	store := &mockWorkspaceStore{
		listEncountersByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Encounter, error) {
			return nil, errors.New("db error")
		},
	}

	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/workspace?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWorkspaceHandler_GetWorkspace_CombatantsError(t *testing.T) {
	campaignID := uuid.New()
	encounterID := uuid.New()

	store := &mockWorkspaceStore{
		listEncountersByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{
				{ID: encounterID, Status: "active", MapID: uuid.NullUUID{Valid: false}},
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return nil, errors.New("db error")
		},
	}

	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/workspace?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWorkspaceHandler_GetWorkspace_MapError(t *testing.T) {
	campaignID := uuid.New()
	encounterID := uuid.New()
	mapID := uuid.New()

	store := &mockWorkspaceStore{
		listEncountersByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{
				{ID: encounterID, Status: "active", MapID: uuid.NullUUID{UUID: mapID, Valid: true}},
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{}, nil
		},
		getMapByIDFn: func(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
			return refdata.Map{}, errors.New("connection error")
		},
	}

	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/workspace?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWorkspaceHandler_GetWorkspace_ZoneError(t *testing.T) {
	campaignID := uuid.New()
	encounterID := uuid.New()

	store := &mockWorkspaceStore{
		listEncountersByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{
				{ID: encounterID, Status: "active", MapID: uuid.NullUUID{Valid: false}},
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{}, nil
		},
		listEncounterZonesByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
			return nil, errors.New("db error")
		},
	}

	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/workspace?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle 22: PATCH combatant position ---

func TestWorkspaceHandler_UpdateCombatantPosition_Success(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	svc := &mockWorkspaceSvc{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			assert.Equal(t, combatantID, id)
			return refdata.Combatant{ID: combatantID, AltitudeFt: 0, PositionCol: "A", PositionRow: 1}, nil
		},
		updateCombatantPositionFn: func(ctx context.Context, id uuid.UUID, col string, row, altitude int32) (refdata.Combatant, error) {
			assert.Equal(t, combatantID, id)
			assert.Equal(t, "D", col)
			assert.Equal(t, int32(4), row)
			assert.Equal(t, int32(0), altitude)
			return refdata.Combatant{
				ID:          combatantID,
				PositionCol: "D",
				PositionRow: 4,
				Conditions:  json.RawMessage(`[]`),
			}, nil
		},
	}

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"position_col":"D","position_row":4}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/position", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp workspaceCombatantResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "D", resp.PositionCol)
	assert.Equal(t, int32(4), resp.PositionRow)
}

func TestWorkspaceHandler_UpdateCombatantPosition_InvalidJSON(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/position", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWorkspaceHandler_UpdateCombatantPosition_MissingCol(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"position_col":"","position_row":4}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/position", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWorkspaceHandler_UpdateCombatantPosition_InvalidCombatantID(t *testing.T) {
	encounterID := uuid.New()

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"position_col":"D","position_row":4}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/not-a-uuid/position", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWorkspaceHandler_UpdateCombatantPosition_CombatantNotFound(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	svc := &mockWorkspaceSvc{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, sql.ErrNoRows
		},
	}

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"position_col":"D","position_row":4}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/position", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWorkspaceHandler_UpdateCombatantPosition_StoreError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	svc := &mockWorkspaceSvc{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: combatantID, AltitudeFt: 0}, nil
		},
		updateCombatantPositionFn: func(ctx context.Context, id uuid.UUID, col string, row, altitude int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("db error")
		},
	}

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"position_col":"D","position_row":4}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/position", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle 23: DELETE combatant ---

func TestWorkspaceHandler_DeleteCombatant_Success(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := &mockWorkspaceStore{
		deleteCombatantFn: func(ctx context.Context, id uuid.UUID) error {
			assert.Equal(t, combatantID, id)
			return nil
		},
	}

	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodDelete, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestWorkspaceHandler_DeleteCombatant_InvalidCombatantID(t *testing.T) {
	encounterID := uuid.New()

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodDelete, "/api/combat/"+encounterID.String()+"/combatants/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWorkspaceHandler_DeleteCombatant_StoreError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := &mockWorkspaceStore{
		deleteCombatantFn: func(ctx context.Context, id uuid.UUID) error {
			return errors.New("db error")
		},
	}

	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodDelete, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- SR-032: pending_queue_count populated from dm_queue_items aggregation ---

// Phase 94a's Combat Workspace tab badges + Encounter Overview "queued" pill
// read `enc.pending_queue_count` (CombatManager.svelte:878,900). The field
// was previously absent from workspaceEncounterResponse, so the UI badge
// never lit up. These tests pin the field-presence + population contract.

func TestWorkspaceHandler_GetWorkspace_PopulatesPendingQueueCount(t *testing.T) {
	campaignID := uuid.New()
	encA := uuid.New()
	encB := uuid.New()
	turnA := uuid.New()
	turnB := uuid.New()
	combA := uuid.New()
	combB := uuid.New()

	store := &mockWorkspaceStore{
		listEncountersByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Encounter, error) {
			assert.Equal(t, campaignID, cid)
			return []refdata.Encounter{
				{ID: encA, CampaignID: campaignID, Name: "Goblin Fight", Status: "active", RoundNumber: 1, CurrentTurnID: uuid.NullUUID{UUID: turnA, Valid: true}},
				{ID: encB, CampaignID: campaignID, Name: "Rooftop Chase", Status: "active", RoundNumber: 2, CurrentTurnID: uuid.NullUUID{UUID: turnB, Valid: true}},
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			if eid == encA {
				return []refdata.Combatant{{ID: combA, EncounterID: encA, ShortID: "PC1", DisplayName: "Aragorn", HpMax: 30, HpCurrent: 30, IsAlive: true, Conditions: json.RawMessage(`[]`)}}, nil
			}
			return []refdata.Combatant{{ID: combB, EncounterID: encB, ShortID: "PC2", DisplayName: "Gimli", HpMax: 28, HpCurrent: 28, IsAlive: true, Conditions: json.RawMessage(`[]`)}}, nil
		},
		listEncounterZonesByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
			return []refdata.EncounterZone{}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			if eid == encA {
				return refdata.Turn{ID: turnA, CombatantID: combA}, nil
			}
			return refdata.Turn{ID: turnB, CombatantID: combB}, nil
		},
		countPendingDMQueueByEncounterFn: func(ctx context.Context, encounterID uuid.UUID) (int64, error) {
			return 2, nil
		},
	}

	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/workspace?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	// Decode through map[string]any so this test pins the exact JSON tag the
	// Svelte UI reads (`pending_queue_count`) rather than the Go field name.
	var raw struct {
		Encounters []map[string]any `json:"encounters"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	require.Len(t, raw.Encounters, 2)
	for i, enc := range raw.Encounters {
		val, ok := enc["pending_queue_count"]
		require.Truef(t, ok, "encounter %d missing pending_queue_count JSON field", i)
		assert.Equal(t, float64(2), val, "encounter %d pending_queue_count", i)
	}
}

func TestWorkspaceHandler_GetWorkspace_PendingQueueCount_PerEncounter(t *testing.T) {
	campaignID := uuid.New()
	encA := uuid.New()
	encB := uuid.New()
	combA := uuid.New()
	combB := uuid.New()

	store := &mockWorkspaceStore{
		listEncountersByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{
				{ID: encA, CampaignID: campaignID, Name: "Goblin Fight", Status: "active", RoundNumber: 1},
				{ID: encB, CampaignID: campaignID, Name: "Rooftop Chase", Status: "active", RoundNumber: 2},
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			if eid == encA {
				return []refdata.Combatant{{ID: combA, EncounterID: encA, ShortID: "PC1", DisplayName: "Aragorn", HpMax: 30, HpCurrent: 30, IsAlive: true, Conditions: json.RawMessage(`[]`)}}, nil
			}
			return []refdata.Combatant{{ID: combB, EncounterID: encB, ShortID: "PC2", DisplayName: "Gimli", HpMax: 28, HpCurrent: 28, IsAlive: true, Conditions: json.RawMessage(`[]`)}}, nil
		},
		listEncounterZonesByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
			return []refdata.EncounterZone{}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, sql.ErrNoRows
		},
		countPendingDMQueueByCampaignFn: func(ctx context.Context, cid uuid.UUID) (int64, error) {
			return 5, nil // campaign-wide total (should NOT be used per-encounter)
		},
		countPendingDMQueueByEncounterFn: func(ctx context.Context, encounterID uuid.UUID) (int64, error) {
			if encounterID == encA {
				return 3, nil
			}
			return 1, nil
		},
	}

	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/workspace?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp workspaceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Encounters, 2)

	// Each encounter must have its own count, NOT the campaign-wide total.
	assert.Equal(t, int32(3), resp.Encounters[0].PendingQueueCount, "encA should have 3 pending items")
	assert.Equal(t, int32(1), resp.Encounters[1].PendingQueueCount, "encB should have 1 pending item")
}

func TestWorkspaceHandler_GetWorkspace_PendingQueueCount_ZeroWhenNoPending(t *testing.T) {
	campaignID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := &mockWorkspaceStore{
		listEncountersByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{
				{ID: encounterID, CampaignID: campaignID, Name: "Quiet Camp", Status: "active", RoundNumber: 1},
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{{ID: combatantID, EncounterID: eid, ShortID: "PC", DisplayName: "Frodo", HpMax: 20, HpCurrent: 20, IsAlive: true, Conditions: json.RawMessage(`[]`)}}, nil
		},
		listEncounterZonesByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
			return []refdata.EncounterZone{}, nil
		},
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, sql.ErrNoRows
		},
		countPendingDMQueueByCampaignFn: func(ctx context.Context, cid uuid.UUID) (int64, error) {
			// The SQL `WHERE status='pending'` filter is what guarantees that
			// resolved/cancelled rows are excluded; from the handler's
			// perspective an empty queue returns 0 here.
			return 0, nil
		},
	}

	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/workspace?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp workspaceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Encounters, 1)
	assert.Equal(t, int32(0), resp.Encounters[0].PendingQueueCount)
}

func TestWorkspaceHandler_GetWorkspace_PendingQueueCount_StoreError(t *testing.T) {
	campaignID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := &mockWorkspaceStore{
		listEncountersByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Encounter, error) {
			return []refdata.Encounter{
				{ID: encounterID, CampaignID: campaignID, Name: "Anywhere", Status: "active"},
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{{ID: combatantID, EncounterID: eid, ShortID: "PC", DisplayName: "X", HpMax: 10, HpCurrent: 10, IsAlive: true, Conditions: json.RawMessage(`[]`)}}, nil
		},
		countPendingDMQueueByEncounterFn: func(ctx context.Context, eid uuid.UUID) (int64, error) {
			return 0, errors.New("db down")
		},
	}

	h := NewWorkspaceHandler(store, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/workspace?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- F-03: Workspace PATCH routes through service (lock + publish) ---

func TestWorkspaceHandler_F03_HPPatchRoutesViaService(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	var svcCalled bool
	svc := &mockWorkspaceSvc{
		updateCombatantHPFn: func(ctx context.Context, id uuid.UUID, hpCurrent, tempHP int32, isAlive bool) (refdata.Combatant, error) {
			svcCalled = true
			assert.Equal(t, combatantID, id)
			assert.Equal(t, int32(10), hpCurrent)
			assert.Equal(t, int32(5), tempHP)
			assert.True(t, isAlive)
			return refdata.Combatant{
				ID: combatantID, EncounterID: encounterID,
				HpCurrent: 10, TempHp: 5, IsAlive: true,
				Conditions: json.RawMessage(`[]`),
			}, nil
		},
	}

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"hp_current":10,"temp_hp":5,"is_alive":true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/hp", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, svcCalled, "HP PATCH must route through service (which publishes snapshot)")
}

func TestWorkspaceHandler_F03_ConditionsPatchRoutesViaService(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	var svcCalled bool
	svc := &mockWorkspaceSvc{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: combatantID, ExhaustionLevel: 2}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, id uuid.UUID, conditions json.RawMessage, exhaustion int32) (refdata.Combatant, error) {
			svcCalled = true
			assert.Equal(t, combatantID, id)
			assert.Equal(t, int32(2), exhaustion)
			return refdata.Combatant{
				ID: combatantID, EncounterID: encounterID,
				Conditions: conditions, ExhaustionLevel: 2,
			}, nil
		},
	}

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"conditions":["Stunned"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/conditions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, svcCalled, "Conditions PATCH must route through service (which publishes snapshot)")
}

func TestWorkspaceHandler_F03_PositionPatchRoutesViaService(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	var svcCalled bool
	svc := &mockWorkspaceSvc{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: combatantID, AltitudeFt: 10, PositionCol: "A", PositionRow: 1}, nil
		},
		updateCombatantPositionFn: func(ctx context.Context, id uuid.UUID, col string, row, altitude int32) (refdata.Combatant, error) {
			svcCalled = true
			assert.Equal(t, combatantID, id)
			assert.Equal(t, "E", col)
			assert.Equal(t, int32(7), row)
			assert.Equal(t, int32(10), altitude, "altitude must be preserved from existing combatant")
			return refdata.Combatant{
				ID: combatantID, EncounterID: encounterID,
				PositionCol: "E", PositionRow: 7, AltitudeFt: 10,
				Conditions: json.RawMessage(`[]`),
			}, nil
		},
	}

	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store, svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"position_col":"E","position_row":7}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/position", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, svcCalled, "Position PATCH must route through service (which publishes snapshot and runs silence-zone checks)")
}
