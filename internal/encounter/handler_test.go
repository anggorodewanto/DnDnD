package encounter

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

func newTestRouter(store Store) (*Handler, chi.Router) {
	svc := NewService(store)
	h := NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return h, r
}

// --- TDD Cycle 12: POST /api/encounters creates an encounter ---

func TestHandler_Create_Success(t *testing.T) {
	_, r := newTestRouter(successStore())

	body := map[string]interface{}{
		"campaign_id":  uuid.New().String(),
		"name":         "Goblin Ambush",
		"display_name": "The Dark Forest",
		"creatures":    []map[string]interface{}{{"creature_ref_id": "goblin", "short_id": "G1", "quantity": 3}},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/encounters", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp encounterResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Goblin Ambush", resp.Name)
	assert.NotNil(t, resp.DisplayName)
	assert.Equal(t, "The Dark Forest", *resp.DisplayName)
	assert.NotEmpty(t, resp.ID)
}

// --- TDD Cycle 13: POST /api/encounters with invalid JSON ---

func TestHandler_Create_InvalidJSON(t *testing.T) {
	_, r := newTestRouter(successStore())

	req := httptest.NewRequest(http.MethodPost, "/api/encounters", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 14: POST /api/encounters with invalid campaign_id ---

func TestHandler_Create_InvalidCampaignID(t *testing.T) {
	_, r := newTestRouter(successStore())

	body := map[string]interface{}{
		"campaign_id": "not-uuid",
		"name":        "Test",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/encounters", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 15: POST /api/encounters with empty name ---

func TestHandler_Create_EmptyName(t *testing.T) {
	_, r := newTestRouter(successStore())

	body := map[string]interface{}{
		"campaign_id": uuid.New().String(),
		"name":        "",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/encounters", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "name must not be empty")
}

// --- TDD Cycle 16: POST /api/encounters with map_id ---

func TestHandler_Create_WithMapID(t *testing.T) {
	var capturedMapID uuid.NullUUID
	store := &mockStore{
		createFn: func(ctx context.Context, arg refdata.CreateEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			capturedMapID = arg.MapID
			return refdata.EncounterTemplate{
				ID:        uuid.New(),
				Name:      arg.Name,
				MapID:     arg.MapID,
				Creatures: arg.Creatures,
			}, nil
		},
	}
	_, r := newTestRouter(store)

	mapID := uuid.New()
	body := map[string]interface{}{
		"campaign_id": uuid.New().String(),
		"name":        "With Map",
		"map_id":      mapID.String(),
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/encounters", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.True(t, capturedMapID.Valid)
	assert.Equal(t, mapID, capturedMapID.UUID)
}

// --- TDD Cycle 17: POST /api/encounters with invalid map_id ---

func TestHandler_Create_InvalidMapID(t *testing.T) {
	_, r := newTestRouter(successStore())

	body := map[string]interface{}{
		"campaign_id": uuid.New().String(),
		"name":        "Test",
		"map_id":      "not-uuid",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/encounters", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid map_id")
}

// --- TDD Cycle 18: POST /api/encounters store error ---

func TestHandler_Create_StoreError(t *testing.T) {
	store := &mockStore{
		createFn: func(ctx context.Context, arg refdata.CreateEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{}, errors.New("db error")
		},
	}
	_, r := newTestRouter(store)

	body := map[string]interface{}{
		"campaign_id": uuid.New().String(),
		"name":        "Test",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/encounters", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle 19: GET /api/encounters/{id} ---

func TestHandler_Get_Success(t *testing.T) {
	id := uuid.New()
	campaignID := uuid.New()
	_, r := newTestRouter(successStore())

	req := httptest.NewRequest(http.MethodGet, "/api/encounters/"+id.String()+"?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp encounterResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, id.String(), resp.ID)
}

func TestHandler_Get_InvalidID(t *testing.T) {
	_, r := newTestRouter(successStore())

	req := httptest.NewRequest(http.MethodGet, "/api/encounters/not-uuid?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Get_NotFound(t *testing.T) {
	store := &mockStore{
		getFn: func(ctx context.Context, arg refdata.GetEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{}, sql.ErrNoRows
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/encounters/"+uuid.New().String()+"?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_Get_InternalError(t *testing.T) {
	store := &mockStore{
		getFn: func(ctx context.Context, arg refdata.GetEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{}, errors.New("db error")
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/encounters/"+uuid.New().String()+"?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle 20: GET /api/encounters?campaign_id=X ---

func TestHandler_List_Success(t *testing.T) {
	campaignID := uuid.New()
	store := &mockStore{
		listFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.EncounterTemplate, error) {
			return []refdata.EncounterTemplate{
				{ID: uuid.New(), CampaignID: cid, Name: "Enc 1", Creatures: json.RawMessage(`[]`)},
				{ID: uuid.New(), CampaignID: cid, Name: "Enc 2", Creatures: json.RawMessage(`[]`)},
			}, nil
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/encounters?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []encounterResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestHandler_List_MissingCampaignID(t *testing.T) {
	_, r := newTestRouter(successStore())

	req := httptest.NewRequest(http.MethodGet, "/api/encounters", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_List_InvalidCampaignID(t *testing.T) {
	_, r := newTestRouter(successStore())

	req := httptest.NewRequest(http.MethodGet, "/api/encounters?campaign_id=not-uuid", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_List_Empty(t *testing.T) {
	_, r := newTestRouter(successStore())

	req := httptest.NewRequest(http.MethodGet, "/api/encounters?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []encounterResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Empty(t, resp)
}

func TestHandler_List_StoreError(t *testing.T) {
	store := &mockStore{
		listFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.EncounterTemplate, error) {
			return nil, errors.New("db error")
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/encounters?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle 21: PUT /api/encounters/{id} ---

func TestHandler_Update_Success(t *testing.T) {
	_, r := newTestRouter(successStore())
	id := uuid.New()

	body := map[string]interface{}{
		"name":      "Updated Encounter",
		"creatures": []map[string]interface{}{{"creature_ref_id": "ogre", "short_id": "O1", "quantity": 1}},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/encounters/"+id.String()+"?campaign_id="+uuid.New().String(), bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp encounterResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Updated Encounter", resp.Name)
}

func TestHandler_Update_InvalidID(t *testing.T) {
	_, r := newTestRouter(successStore())

	req := httptest.NewRequest(http.MethodPut, "/api/encounters/bad-id?campaign_id="+uuid.New().String(), bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Update_InvalidJSON(t *testing.T) {
	_, r := newTestRouter(successStore())

	req := httptest.NewRequest(http.MethodPut, "/api/encounters/"+uuid.New().String()+"?campaign_id="+uuid.New().String(), bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Update_EmptyName(t *testing.T) {
	_, r := newTestRouter(successStore())

	body := map[string]interface{}{"name": ""}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/encounters/"+uuid.New().String()+"?campaign_id="+uuid.New().String(), bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Update_StoreError(t *testing.T) {
	store := &mockStore{
		updateFn: func(ctx context.Context, arg refdata.UpdateEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{}, errors.New("db error")
		},
	}
	_, r := newTestRouter(store)

	body := map[string]interface{}{"name": "Test", "creatures": json.RawMessage(`[]`)}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/encounters/"+uuid.New().String()+"?campaign_id="+uuid.New().String(), bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_Update_InvalidMapID(t *testing.T) {
	_, r := newTestRouter(successStore())

	body := map[string]interface{}{
		"name":   "Test",
		"map_id": "not-uuid",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/encounters/"+uuid.New().String()+"?campaign_id="+uuid.New().String(), bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 22: DELETE /api/encounters/{id} ---

func TestHandler_Delete_Success(t *testing.T) {
	_, r := newTestRouter(successStore())

	req := httptest.NewRequest(http.MethodDelete, "/api/encounters/"+uuid.New().String()+"?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestHandler_Delete_InvalidID(t *testing.T) {
	_, r := newTestRouter(successStore())

	req := httptest.NewRequest(http.MethodDelete, "/api/encounters/bad-id?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Delete_StoreError(t *testing.T) {
	store := &mockStore{
		deleteFn: func(ctx context.Context, arg refdata.DeleteEncounterTemplateParams) error {
			return errors.New("db error")
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodDelete, "/api/encounters/"+uuid.New().String()+"?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle 23: POST /api/encounters/{id}/duplicate ---

func TestHandler_Duplicate_Success(t *testing.T) {
	store := successStore()
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/api/encounters/"+uuid.New().String()+"/duplicate?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp encounterResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp.Name, "(copy)")
}

func TestHandler_Duplicate_InvalidID(t *testing.T) {
	_, r := newTestRouter(successStore())

	req := httptest.NewRequest(http.MethodPost, "/api/encounters/bad-id/duplicate?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Duplicate_NotFound(t *testing.T) {
	store := &mockStore{
		getFn: func(ctx context.Context, arg refdata.GetEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{}, sql.ErrNoRows
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/api/encounters/"+uuid.New().String()+"/duplicate?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle 24: NewHandler and RegisterRoutes ---

func TestNewHandler(t *testing.T) {
	svc := NewService(successStore())
	h := NewHandler(svc)
	assert.NotNil(t, h)
	assert.Equal(t, svc, h.svc)
}

// --- TDD Cycle 25: Response serialization with map_id and display_name ---

func TestHandler_Get_ResponseIncludesMapID(t *testing.T) {
	mapID := uuid.New()
	store := &mockStore{
		getFn: func(ctx context.Context, arg refdata.GetEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{
				ID:          arg.ID,
				Name:        "With Map",
				MapID:       uuid.NullUUID{UUID: mapID, Valid: true},
				DisplayName: sql.NullString{String: "Player Name", Valid: true},
				Creatures:   json.RawMessage(`[]`),
			}, nil
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/encounters/"+uuid.New().String()+"?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, mapID.String(), resp["map_id"])
	assert.Equal(t, "Player Name", resp["display_name"])
}

func TestHandler_Get_NullMapID(t *testing.T) {
	store := &mockStore{
		getFn: func(ctx context.Context, arg refdata.GetEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{
				ID:        arg.ID,
				Name:      "No Map",
				Creatures: json.RawMessage(`[]`),
			}, nil
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/encounters/"+uuid.New().String()+"?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Nil(t, resp["map_id"])
	assert.Nil(t, resp["display_name"])
}

// --- TDD Cycle 26: GET /api/creatures ---

func TestHandler_ListCreatures_Success(t *testing.T) {
	store := &mockStore{
		listCreaturesFn: func(ctx context.Context) ([]refdata.Creature, error) {
			return []refdata.Creature{
				{ID: "goblin", Name: "Goblin", Cr: "1/4", Size: "Small", Type: "humanoid", Ac: 15, HpAverage: 7},
				{ID: "ogre", Name: "Ogre", Cr: "2", Size: "Large", Type: "giant", Ac: 11, HpAverage: 59},
			}, nil
		},
	}
	// Need full store for newTestRouter
	store.createFn = successStore().createFn
	store.getFn = successStore().getFn
	store.listFn = successStore().listFn
	store.updateFn = successStore().updateFn
	store.deleteFn = successStore().deleteFn

	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/creatures", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "Goblin", resp[0]["name"])
	assert.Equal(t, "1/4", resp[0]["cr"])
}

func TestHandler_ListCreatures_StoreError(t *testing.T) {
	store := &mockStore{
		listCreaturesFn: func(ctx context.Context) ([]refdata.Creature, error) {
			return nil, errors.New("db error")
		},
	}
	store.createFn = successStore().createFn
	store.getFn = successStore().getFn
	store.listFn = successStore().listFn
	store.updateFn = successStore().updateFn
	store.deleteFn = successStore().deleteFn

	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/creatures", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle 27: encounterResponse includes creature_count and created_at ---

func TestHandler_List_IncludesCreatureCount(t *testing.T) {
	store := &mockStore{
		listFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.EncounterTemplate, error) {
			return []refdata.EncounterTemplate{
				{
					ID:        uuid.New(),
					Name:      "Multi Creature",
					Creatures: json.RawMessage(`[{"creature_ref_id":"goblin","short_id":"G1","quantity":3},{"creature_ref_id":"ogre","short_id":"O1","quantity":1}]`),
				},
			}, nil
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/encounters?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Len(t, resp, 1)
	assert.Equal(t, float64(2), resp[0]["creature_count"])
}

// --- F-02: Campaign-scoped access control ---

// TestHandler_GetEncounter_WrongCampaignID proves that an encounter template
// belonging to campaign A cannot be retrieved when campaign B's ID is supplied.
func TestHandler_GetEncounter_WrongCampaignID(t *testing.T) {
	templateID := uuid.New()
	attackerCampaign := uuid.New()
	store := &mockStore{
		getFn: func(ctx context.Context, arg refdata.GetEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			// Simulate DB: wrong campaign returns no rows
			return refdata.EncounterTemplate{}, sql.ErrNoRows
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/encounters/"+templateID.String()+"?campaign_id="+attackerCampaign.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestHandler_UpdateEncounter_WrongCampaignID proves that updating an encounter
// template with wrong campaign_id fails.
func TestHandler_UpdateEncounter_WrongCampaignID(t *testing.T) {
	templateID := uuid.New()
	attackerCampaign := uuid.New()
	store := &mockStore{
		updateFn: func(ctx context.Context, arg refdata.UpdateEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{}, errors.New("updating encounter template: no rows")
		},
	}
	_, r := newTestRouter(store)

	body := map[string]interface{}{"name": "Hacked", "creatures": json.RawMessage(`[]`)}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/encounters/"+templateID.String()+"?campaign_id="+attackerCampaign.String(), bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestHandler_DeleteEncounter_WrongCampaignID proves that deleting an encounter
// template with wrong campaign_id fails.
func TestHandler_DeleteEncounter_WrongCampaignID(t *testing.T) {
	templateID := uuid.New()
	attackerCampaign := uuid.New()
	store := &mockStore{
		deleteFn: func(ctx context.Context, arg refdata.DeleteEncounterTemplateParams) error {
			return errors.New("not found")
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodDelete, "/api/encounters/"+templateID.String()+"?campaign_id="+attackerCampaign.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestHandler_DuplicateEncounter_WrongCampaignID proves that duplicating an
// encounter template with wrong campaign_id fails.
func TestHandler_DuplicateEncounter_WrongCampaignID(t *testing.T) {
	templateID := uuid.New()
	attackerCampaign := uuid.New()
	store := &mockStore{
		getFn: func(ctx context.Context, arg refdata.GetEncounterTemplateParams) (refdata.EncounterTemplate, error) {
			return refdata.EncounterTemplate{}, sql.ErrNoRows
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/api/encounters/"+templateID.String()+"/duplicate?campaign_id="+attackerCampaign.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestHandler_GetEncounter_MissingCampaignID proves that object routes reject
// requests without campaign_id.
func TestHandler_GetEncounter_MissingCampaignID(t *testing.T) {
	_, r := newTestRouter(successStore())

	req := httptest.NewRequest(http.MethodGet, "/api/encounters/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "campaign_id")
}
