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
	listEncountersByCampaignIDFn  func(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error)
	listCombatantsByEncounterIDFn func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	getMapByIDFn                  func(ctx context.Context, id uuid.UUID) (refdata.Map, error)
	listEncounterZonesByEncounterIDFn func(ctx context.Context, encounterID uuid.UUID) ([]refdata.EncounterZone, error)
	updateCombatantHPFn           func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error)
	updateCombatantConditionsFn   func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error)
	getActiveTurnByEncounterIDFn  func(ctx context.Context, encounterID uuid.UUID) (refdata.Turn, error)
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

// --- TDD Cycle 10: GET /api/combat/workspace returns active encounters ---

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
					ID:          encounterID,
					CampaignID:  campaignID,
					Name:        "Goblin Ambush",
					Status:      "active",
					RoundNumber: 3,
					MapID:       uuid.NullUUID{UUID: mapID, Valid: true},
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

	h := NewWorkspaceHandler(store)
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

// --- TDD Cycle 11: GET /api/combat/workspace missing campaign_id ---

func TestWorkspaceHandler_GetWorkspace_MissingCampaignID(t *testing.T) {
	store := &mockWorkspaceStore{}
	h := NewWorkspaceHandler(store)
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
	h := NewWorkspaceHandler(store)
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

	h := NewWorkspaceHandler(store)
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

	store := &mockWorkspaceStore{
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			assert.Equal(t, combatantID, arg.ID)
			assert.Equal(t, int32(15), arg.HpCurrent)
			assert.Equal(t, int32(0), arg.TempHp)
			assert.True(t, arg.IsAlive)
			return refdata.Combatant{
				ID:        combatantID,
				HpCurrent: 15,
				TempHp:    0,
				IsAlive:   true,
				Conditions: json.RawMessage(`[]`),
			}, nil
		},
	}

	h := NewWorkspaceHandler(store)
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

	store := &mockWorkspaceStore{
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			assert.Equal(t, combatantID, arg.ID)
			return refdata.Combatant{
				ID:         combatantID,
				Conditions: json.RawMessage(`["Blinded","Prone"]`),
			}, nil
		},
	}

	h := NewWorkspaceHandler(store)
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
	h := NewWorkspaceHandler(store)
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

	h := NewWorkspaceHandler(store)
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
	h := NewWorkspaceHandler(store)
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

	store := &mockWorkspaceStore{
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("db error")
		},
	}

	h := NewWorkspaceHandler(store)
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
	h := NewWorkspaceHandler(store)
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
	h := NewWorkspaceHandler(store)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := `{"conditions":["Blinded"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/combat/"+encounterID.String()+"/combatants/not-a-uuid/conditions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWorkspaceHandler_UpdateCombatantConditions_StoreError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := &mockWorkspaceStore{
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("db error")
		},
	}

	h := NewWorkspaceHandler(store)
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

	h := NewWorkspaceHandler(store)
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

	h := NewWorkspaceHandler(store)
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

	h := NewWorkspaceHandler(store)
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

	h := NewWorkspaceHandler(store)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/workspace?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
