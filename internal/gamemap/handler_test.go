package gamemap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/refdata"
)

// newTestRouter creates a chi router with map API routes registered and returns it along with the handler.
func newTestRouter(store Store) (*Handler, chi.Router) {
	svc := NewService(store)
	h := NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return h, r
}

// --- TDD Cycle 1: POST /api/maps creates a map ---

func TestHandler_CreateMap_Success(t *testing.T) {
	campaignID := uuid.New()
	store := successStore(campaignID)
	_, r := newTestRouter(store)

	body := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"name":        "Test Map",
		"width":       10,
		"height":      10,
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/maps", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp mapResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Test Map", resp.Name)
	assert.Equal(t, 10, resp.Width)
	assert.Equal(t, 10, resp.Height)
	assert.NotEmpty(t, resp.ID)
	// Should have generated default tiled JSON
	assert.NotEmpty(t, resp.TiledJSON)
}

// --- TDD Cycle 2: POST /api/maps with invalid body returns 400 ---

func TestHandler_CreateMap_InvalidJSON(t *testing.T) {
	_, r := newTestRouter(&mockStore{})

	req := httptest.NewRequest(http.MethodPost, "/api/maps", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 3: POST /api/maps with invalid campaign_id ---

func TestHandler_CreateMap_InvalidCampaignID(t *testing.T) {
	_, r := newTestRouter(&mockStore{})

	body := map[string]interface{}{
		"campaign_id": "not-a-uuid",
		"name":        "Test Map",
		"width":       10,
		"height":      10,
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/maps", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 4: POST /api/maps with validation error (bad dimensions) ---

func TestHandler_CreateMap_ValidationError(t *testing.T) {
	campaignID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))

	body := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"name":        "Test Map",
		"width":       201,
		"height":      10,
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/maps", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "exceeds hard limit")
}

// --- TDD Cycle 5: POST /api/maps with custom tiled_json ---

func TestHandler_CreateMap_CustomTiledJSON(t *testing.T) {
	campaignID := uuid.New()
	var capturedTiledJSON json.RawMessage
	store := &mockStore{
		createMapFn: func(ctx context.Context, arg refdata.CreateMapParams) (refdata.Map, error) {
			capturedTiledJSON = arg.TiledJson
			return refdata.Map{
				ID:            uuid.New(),
				CampaignID:    arg.CampaignID,
				Name:          arg.Name,
				WidthSquares:  arg.WidthSquares,
				HeightSquares: arg.HeightSquares,
				TiledJson:     arg.TiledJson,
			}, nil
		},
	}
	_, r := newTestRouter(store)

	customJSON := json.RawMessage(`{"width":10,"height":10,"layers":[]}`)
	body := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"name":        "Custom Map",
		"width":       10,
		"height":      10,
		"tiled_json":  customJSON,
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/maps", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	// Verify custom JSON was passed through (not default)
	var parsed map[string]interface{}
	err := json.Unmarshal(capturedTiledJSON, &parsed)
	require.NoError(t, err)
	layers, ok := parsed["layers"].([]interface{})
	require.True(t, ok)
	assert.Empty(t, layers)
}

// --- TDD Cycle 6: GET /api/maps/{id} returns a map ---

func TestHandler_GetMap_Success(t *testing.T) {
	mapID := uuid.New()
	campaignID := uuid.New()
	store := &mockStore{
		getMapByIDFn: func(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
			return refdata.Map{
				ID:            mapID,
				CampaignID:    campaignID,
				Name:          "Found Map",
				WidthSquares:  20,
				HeightSquares: 15,
				TiledJson:     minimalTiledJSON(),
			}, nil
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/maps/"+mapID.String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp mapResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, mapID.String(), resp.ID)
	assert.Equal(t, "Found Map", resp.Name)
	assert.Equal(t, 20, resp.Width)
	assert.Equal(t, 15, resp.Height)
}

// --- TDD Cycle 7: GET /api/maps/{id} with invalid UUID ---

func TestHandler_GetMap_InvalidID(t *testing.T) {
	_, r := newTestRouter(&mockStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/maps/not-a-uuid", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 8: GET /api/maps/{id} not found ---

func TestHandler_GetMap_NotFound(t *testing.T) {
	store := &mockStore{
		getMapByIDFn: func(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
			return refdata.Map{}, errors.New("not found")
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/maps/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- TDD Cycle 9: GET /api/maps/{id} internal error ---

func TestHandler_GetMap_InternalError(t *testing.T) {
	store := &mockStore{
		getMapByIDFn: func(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
			return refdata.Map{}, errors.New("database connection lost")
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/maps/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle 10: GET /api/maps?campaign_id=X returns list ---

func TestHandler_ListMaps_Success(t *testing.T) {
	campaignID := uuid.New()
	store := &mockStore{
		listMapsByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Map, error) {
			return []refdata.Map{
				{ID: uuid.New(), CampaignID: cid, Name: "Map 1", WidthSquares: 10, HeightSquares: 10, TiledJson: minimalTiledJSON()},
				{ID: uuid.New(), CampaignID: cid, Name: "Map 2", WidthSquares: 20, HeightSquares: 20, TiledJson: minimalTiledJSON()},
			}, nil
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/maps?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []mapResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "Map 1", resp[0].Name)
	assert.Equal(t, "Map 2", resp[1].Name)
}

// --- TDD Cycle 11: GET /api/maps without campaign_id ---

func TestHandler_ListMaps_MissingCampaignID(t *testing.T) {
	_, r := newTestRouter(&mockStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/maps", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "campaign_id")
}

// --- TDD Cycle 12: GET /api/maps with invalid campaign_id ---

func TestHandler_ListMaps_InvalidCampaignID(t *testing.T) {
	_, r := newTestRouter(&mockStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/maps?campaign_id=not-uuid", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 13: GET /api/maps empty list ---

func TestHandler_ListMaps_EmptyList(t *testing.T) {
	campaignID := uuid.New()
	store := &mockStore{
		listMapsByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Map, error) {
			return []refdata.Map{}, nil
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/maps?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []mapResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Empty(t, resp)
}

// --- TDD Cycle 14: PUT /api/maps/{id} updates a map ---

func TestHandler_UpdateMap_Success(t *testing.T) {
	campaignID := uuid.New()
	mapID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))

	body := map[string]interface{}{
		"name":       "Updated Map",
		"width":      15,
		"height":     15,
		"tiled_json": minimalTiledJSON(),
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/maps/"+mapID.String(), bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp mapResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, mapID.String(), resp.ID)
	assert.Equal(t, "Updated Map", resp.Name)
}

// --- TDD Cycle 15: PUT /api/maps/{id} invalid ID ---

func TestHandler_UpdateMap_InvalidID(t *testing.T) {
	_, r := newTestRouter(&mockStore{})

	req := httptest.NewRequest(http.MethodPut, "/api/maps/bad-id", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 16: PUT /api/maps/{id} invalid body ---

func TestHandler_UpdateMap_InvalidJSON(t *testing.T) {
	_, r := newTestRouter(&mockStore{})
	mapID := uuid.New()

	req := httptest.NewRequest(http.MethodPut, "/api/maps/"+mapID.String(), bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 17: PUT /api/maps/{id} validation error ---

func TestHandler_UpdateMap_ValidationError(t *testing.T) {
	campaignID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))
	mapID := uuid.New()

	body := map[string]interface{}{
		"name":       "",
		"width":      10,
		"height":     10,
		"tiled_json": minimalTiledJSON(),
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/maps/"+mapID.String(), bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "name must not be empty")
}

// --- TDD Cycle 18: DELETE /api/maps/{id} ---

func TestHandler_DeleteMap_Success(t *testing.T) {
	store := &mockStore{
		deleteMapFn: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodDelete, "/api/maps/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

// --- TDD Cycle 19: DELETE /api/maps/{id} invalid ID ---

func TestHandler_DeleteMap_InvalidID(t *testing.T) {
	_, r := newTestRouter(&mockStore{})

	req := httptest.NewRequest(http.MethodDelete, "/api/maps/bad-id", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 20: DELETE /api/maps/{id} store error ---

func TestHandler_DeleteMap_StoreError(t *testing.T) {
	store := &mockStore{
		deleteMapFn: func(ctx context.Context, id uuid.UUID) error {
			return errors.New("db error")
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodDelete, "/api/maps/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle 21: POST /api/maps generates default Tiled JSON with correct structure ---

func TestHandler_CreateMap_DefaultTiledJSON_Structure(t *testing.T) {
	campaignID := uuid.New()
	var capturedTiledJSON json.RawMessage
	store := &mockStore{
		createMapFn: func(ctx context.Context, arg refdata.CreateMapParams) (refdata.Map, error) {
			capturedTiledJSON = arg.TiledJson
			return refdata.Map{
				ID:            uuid.New(),
				CampaignID:    arg.CampaignID,
				Name:          arg.Name,
				WidthSquares:  arg.WidthSquares,
				HeightSquares: arg.HeightSquares,
				TiledJson:     arg.TiledJson,
			}, nil
		},
	}
	_, r := newTestRouter(store)

	body := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"name":        "Auto Map",
		"width":       5,
		"height":      3,
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/maps", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var parsed map[string]interface{}
	err := json.Unmarshal(capturedTiledJSON, &parsed)
	require.NoError(t, err)

	// Check structure
	assert.Equal(t, float64(5), parsed["width"])
	assert.Equal(t, float64(3), parsed["height"])
	assert.Equal(t, float64(48), parsed["tilewidth"])
	assert.Equal(t, "orthogonal", parsed["orientation"])

	// Check layers — SR-030 raised the default from 2 (terrain+walls) to 5
	// (terrain, walls, lighting, elevation, spawn_zones) to match the Svelte
	// editor's generateBlankMap. Detailed shape is asserted in
	// TestGenerateDefaultTiledJSON_FiveLayers; here we only verify the handler
	// forwards the canonical 5-layer JSON unchanged.
	layers, ok := parsed["layers"].([]interface{})
	require.True(t, ok)
	require.Len(t, layers, 5)

	// Terrain layer
	terrainLayer := layers[0].(map[string]interface{})
	assert.Equal(t, "terrain", terrainLayer["name"])
	assert.Equal(t, "tilelayer", terrainLayer["type"])
	data := terrainLayer["data"].([]interface{})
	assert.Len(t, data, 15) // 5*3
	for _, v := range data {
		assert.Equal(t, float64(1), v) // all open ground
	}

	// Walls layer
	wallsLayer := layers[1].(map[string]interface{})
	assert.Equal(t, "walls", wallsLayer["name"])
	assert.Equal(t, "objectgroup", wallsLayer["type"])
	objects := wallsLayer["objects"].([]interface{})
	assert.Empty(t, objects)

	// Check tilesets — terrain + lighting (firstgid 7) after SR-030
	tilesets := parsed["tilesets"].([]interface{})
	require.Len(t, tilesets, 2)
	tileset := tilesets[0].(map[string]interface{})
	assert.Equal(t, float64(1), tileset["firstgid"])
	tiles := tileset["tiles"].([]interface{})
	assert.Len(t, tiles, 5)
}

// --- TDD Cycle 22: POST /api/maps store error ---

func TestHandler_CreateMap_StoreError(t *testing.T) {
	campaignID := uuid.New()
	store := &mockStore{
		createMapFn: func(ctx context.Context, arg refdata.CreateMapParams) (refdata.Map, error) {
			return refdata.Map{}, errors.New("db error")
		},
	}
	_, r := newTestRouter(store)

	body := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"name":        "Test",
		"width":       10,
		"height":      10,
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/maps", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle 23: GET /api/maps list store error ---

func TestHandler_ListMaps_StoreError(t *testing.T) {
	campaignID := uuid.New()
	store := &mockStore{
		listMapsByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Map, error) {
			return nil, errors.New("db error")
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/maps?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle 24: PUT /api/maps/{id} store error ---

func TestHandler_UpdateMap_StoreError(t *testing.T) {
	store := &mockStore{
		updateMapFn: func(ctx context.Context, arg refdata.UpdateMapParams) (refdata.Map, error) {
			return refdata.Map{}, errors.New("db error")
		},
	}
	_, r := newTestRouter(store)
	mapID := uuid.New()

	body := map[string]interface{}{
		"name":       "Updated",
		"width":      10,
		"height":     10,
		"tiled_json": minimalTiledJSON(),
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/maps/"+mapID.String(), bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle 25: generateDefaultTiledJSON unit test ---

func TestGenerateDefaultTiledJSON(t *testing.T) {
	result := generateDefaultTiledJSON(3, 2, 48)

	var parsed map[string]interface{}
	err := json.Unmarshal(result, &parsed)
	require.NoError(t, err)

	assert.Equal(t, float64(3), parsed["width"])
	assert.Equal(t, float64(2), parsed["height"])
	assert.Equal(t, float64(48), parsed["tilewidth"])
	assert.Equal(t, float64(48), parsed["tileheight"])
	assert.Equal(t, "orthogonal", parsed["orientation"])
	assert.Equal(t, "right-down", parsed["renderorder"])

	layers := parsed["layers"].([]interface{})
	terrainLayer := layers[0].(map[string]interface{})
	data := terrainLayer["data"].([]interface{})
	assert.Len(t, data, 6) // 3*2
}

// --- TDD Cycle 26: generateDefaultTiledJSON large map uses 32px tiles ---

func TestGenerateDefaultTiledJSON_LargeTileSize(t *testing.T) {
	result := generateDefaultTiledJSON(10, 10, 32)

	var parsed map[string]interface{}
	err := json.Unmarshal(result, &parsed)
	require.NoError(t, err)

	assert.Equal(t, float64(32), parsed["tilewidth"])
	assert.Equal(t, float64(32), parsed["tileheight"])
}

// --- TDD Cycle 27: contains helper ---

func TestContainsAny_Helper(t *testing.T) {
	assert.True(t, containsAny("hello world", "world"))
	assert.True(t, containsAny("must be positive", "must be positive"))
	assert.False(t, containsAny("hello", "world"))
	assert.False(t, containsAny("", "world"))
	// strings.Contains returns true for empty substring (stdlib behavior)
	assert.True(t, containsAny("hello", ""))
}

// --- TDD Cycle 28: handleServiceError maps errors correctly ---

func TestHandleServiceError_ValidationErrors(t *testing.T) {
	cases := []struct {
		err    string
		status int
	}{
		{"dimensions must be positive (got 0x10)", http.StatusBadRequest},
		{"dimensions 201x100 exceeds hard limit of 200x200", http.StatusBadRequest},
		{"name must not be empty", http.StatusBadRequest},
		{"tiled_json must not be empty", http.StatusBadRequest},
		{"creating map: db error", http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.err, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handleServiceError(rec, fmt.Errorf("%s", tc.err))
			assert.Equal(t, tc.status, rec.Code)
		})
	}
}

// --- TDD Cycle 29: RegisterRoutes mounts all endpoints ---

func TestHandler_RegisterRoutes_AllEndpoints(t *testing.T) {
	campaignID := uuid.New()
	mapID := uuid.New()
	store := &mockStore{
		createMapFn: func(ctx context.Context, arg refdata.CreateMapParams) (refdata.Map, error) {
			return refdata.Map{ID: uuid.New(), CampaignID: arg.CampaignID, Name: arg.Name, WidthSquares: arg.WidthSquares, HeightSquares: arg.HeightSquares, TiledJson: arg.TiledJson}, nil
		},
		getMapByIDFn: func(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
			return refdata.Map{ID: id, Name: "Test", TiledJson: minimalTiledJSON()}, nil
		},
		listMapsByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Map, error) {
			return []refdata.Map{}, nil
		},
		updateMapFn: func(ctx context.Context, arg refdata.UpdateMapParams) (refdata.Map, error) {
			return refdata.Map{ID: arg.ID, Name: arg.Name, WidthSquares: arg.WidthSquares, HeightSquares: arg.HeightSquares, TiledJson: arg.TiledJson}, nil
		},
		deleteMapFn: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}
	_, r := newTestRouter(store)

	// POST
	body := map[string]interface{}{"campaign_id": campaignID.String(), "name": "Test", "width": 10, "height": 10}
	b, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/maps", bytes.NewReader(b)))
	assert.Equal(t, http.StatusCreated, rec.Code)

	// GET by ID
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/maps/"+mapID.String(), nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	// GET list
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/maps?campaign_id="+campaignID.String(), nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	// PUT
	updateBody := map[string]interface{}{"name": "Updated", "width": 10, "height": 10, "tiled_json": minimalTiledJSON()}
	ub, _ := json.Marshal(updateBody)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/maps/"+mapID.String(), bytes.NewReader(ub)))
	assert.Equal(t, http.StatusOK, rec.Code)

	// DELETE
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/maps/"+mapID.String(), nil))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

// --- TDD Cycle 30: NewHandler returns non-nil handler ---

func TestNewHandler_ReturnsHandler(t *testing.T) {
	svc := NewService(&mockStore{})
	h := NewHandler(svc)
	assert.NotNil(t, h)
	assert.Equal(t, svc, h.svc)
}

// --- TDD Cycle 31: mapResponse includes background_image_id ---

func TestHandler_GetMap_IncludesBackgroundImageID(t *testing.T) {
	mapID := uuid.New()
	bgID := uuid.New()
	store := &mockStore{
		getMapByIDFn: func(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
			return refdata.Map{
				ID:                mapID,
				Name:              "BG Map",
				WidthSquares:      10,
				HeightSquares:     10,
				TiledJson:         minimalTiledJSON(),
				BackgroundImageID: uuid.NullUUID{UUID: bgID, Valid: true},
			}, nil
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/maps/"+mapID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, bgID.String(), resp["background_image_id"])
}

func TestHandler_GetMap_NullBackgroundImageID(t *testing.T) {
	mapID := uuid.New()
	store := &mockStore{
		getMapByIDFn: func(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
			return refdata.Map{
				ID:            mapID,
				Name:          "No BG",
				WidthSquares:  10,
				HeightSquares: 10,
				TiledJson:     minimalTiledJSON(),
			}, nil
		},
	}
	_, r := newTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/maps/"+mapID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	// Null should be omitted or null
	bgVal, exists := resp["background_image_id"]
	if exists {
		assert.Nil(t, bgVal)
	}
}

// --- TDD Cycle 32: CreateMap passes background_image_id ---

func TestHandler_CreateMap_WithBackgroundImageID(t *testing.T) {
	campaignID := uuid.New()
	bgID := uuid.New()
	var capturedBgID uuid.NullUUID
	store := &mockStore{
		createMapFn: func(ctx context.Context, arg refdata.CreateMapParams) (refdata.Map, error) {
			capturedBgID = arg.BackgroundImageID
			return refdata.Map{
				ID:                uuid.New(),
				CampaignID:        arg.CampaignID,
				Name:              arg.Name,
				WidthSquares:      arg.WidthSquares,
				HeightSquares:     arg.HeightSquares,
				TiledJson:         arg.TiledJson,
				BackgroundImageID: arg.BackgroundImageID,
			}, nil
		},
	}
	_, r := newTestRouter(store)

	body := map[string]interface{}{
		"campaign_id":         campaignID.String(),
		"name":                "BG Map",
		"width":               10,
		"height":              10,
		"background_image_id": bgID.String(),
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/maps", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.True(t, capturedBgID.Valid)
	assert.Equal(t, bgID, capturedBgID.UUID)
}

func TestHandler_CreateMap_InvalidBackgroundImageID(t *testing.T) {
	campaignID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))

	body := map[string]interface{}{
		"campaign_id":         campaignID.String(),
		"name":                "Bad BG",
		"width":               10,
		"height":              10,
		"background_image_id": "not-a-uuid",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/maps", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid background_image_id")
}

// --- TDD Cycle 33: UpdateMap passes background_image_id ---

func TestHandler_UpdateMap_WithBackgroundImageID(t *testing.T) {
	mapID := uuid.New()
	bgID := uuid.New()
	campaignID := uuid.New()
	var capturedBgID uuid.NullUUID
	store := &mockStore{
		updateMapFn: func(ctx context.Context, arg refdata.UpdateMapParams) (refdata.Map, error) {
			capturedBgID = arg.BackgroundImageID
			return refdata.Map{
				ID:                arg.ID,
				CampaignID:        campaignID,
				Name:              arg.Name,
				WidthSquares:      arg.WidthSquares,
				HeightSquares:     arg.HeightSquares,
				TiledJson:         arg.TiledJson,
				BackgroundImageID: arg.BackgroundImageID,
			}, nil
		},
	}
	_, r := newTestRouter(store)

	body := map[string]interface{}{
		"name":                "Updated BG",
		"width":               10,
		"height":              10,
		"tiled_json":          minimalTiledJSON(),
		"background_image_id": bgID.String(),
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/maps/"+mapID.String(), bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, capturedBgID.Valid)
	assert.Equal(t, bgID, capturedBgID.UUID)
}

func TestHandler_UpdateMap_InvalidBackgroundImageID(t *testing.T) {
	_, r := newTestRouter(successStore(uuid.New()))
	mapID := uuid.New()

	body := map[string]interface{}{
		"name":                "Bad BG Update",
		"width":               10,
		"height":              10,
		"tiled_json":          minimalTiledJSON(),
		"background_image_id": "not-a-uuid",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/maps/"+mapID.String(), bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid background_image_id")
}

// --- SR-030: default tiled JSON must carry all five editor layers ---

// TestGenerateDefaultTiledJSON_FiveLayers locks in the canonical layer schema
// emitted by the Svelte editor's generateBlankMap (dashboard/svelte/src/lib/mapdata.js):
// terrain, walls, lighting, elevation, spawn_zones — in that order, with the
// new three layers well-formed but empty (zero-filled data / empty objects).
// Also asserts the lighting tileset entry is present so renderer.ParseTiledJSON
// can resolve lighting GIDs via the tileset path (SR-029 buildLightingGIDMap).
func TestGenerateDefaultTiledJSON_FiveLayers(t *testing.T) {
	const w, h, ts = 4, 3, 48
	raw := generateDefaultTiledJSON(w, h, ts)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &parsed))

	layers, ok := parsed["layers"].([]interface{})
	require.True(t, ok, "layers must be an array")
	require.Len(t, layers, 5, "default map must have 5 layers like the editor")

	expectedOrder := []struct {
		name      string
		layerType string
		tile      bool // true => tilelayer, false => objectgroup
	}{
		{"terrain", "tilelayer", true},
		{"walls", "objectgroup", false},
		{"lighting", "tilelayer", true},
		{"elevation", "tilelayer", true},
		{"spawn_zones", "objectgroup", false},
	}
	tileCount := w * h
	for i, want := range expectedOrder {
		layer := layers[i].(map[string]interface{})
		assert.Equal(t, want.name, layer["name"], "layer[%d] name", i)
		assert.Equal(t, want.layerType, layer["type"], "layer[%d] type", i)
		assert.Equal(t, true, layer["visible"], "layer[%d] visible", i)
		if !want.tile {
			objects, ok := layer["objects"].([]interface{})
			require.True(t, ok, "layer[%d] objects must be array", i)
			assert.Empty(t, objects, "layer[%d] objects should be empty by default", i)
			continue
		}
		assert.Equal(t, float64(w), layer["width"], "layer[%d] width", i)
		assert.Equal(t, float64(h), layer["height"], "layer[%d] height", i)
		data, ok := layer["data"].([]interface{})
		require.True(t, ok, "layer[%d] data must be array", i)
		require.Len(t, data, tileCount, "layer[%d] data length", i)
		// terrain defaults to open_ground (GID 1); lighting & elevation default to 0.
		wantFill := 0.0
		if want.name == "terrain" {
			wantFill = 1.0
		}
		for j, cell := range data {
			assert.Equal(t, wantFill, cell, "layer[%d=%s] data[%d]", i, want.name, j)
		}
	}

	// Lighting tileset must be present so the renderer can resolve lighting GIDs
	// via the tileset path (firstgid + tile.id) — mirrors mapdata.js LIGHTING_TYPES.
	tilesets, ok := parsed["tilesets"].([]interface{})
	require.True(t, ok)
	require.Len(t, tilesets, 2, "expect terrain + lighting tilesets")
	lighting := tilesets[1].(map[string]interface{})
	assert.Equal(t, "lighting", lighting["name"])
	assert.Equal(t, float64(7), lighting["firstgid"], "lighting firstgid follows the editor (terrain firstgid 1 + tilecount 6)")
	tiles := lighting["tiles"].([]interface{})
	gotTypes := make([]string, 0, len(tiles))
	for _, tl := range tiles {
		gotTypes = append(gotTypes, tl.(map[string]interface{})["type"].(string))
	}
	assert.Equal(t, []string{"dim_light", "darkness", "magical_darkness", "fog", "light_obscurement"}, gotTypes)
}

// TestGenerateDefaultTiledJSON_RoundTrip pushes the default JSON through the
// SR-029 renderer parser and asserts a clean parse: dimensions match, terrain
// is fully open ground, walls/magical-darkness/spawn-zones are empty, and the
// elevation grid is zero-filled with length width*height.
func TestGenerateDefaultTiledJSON_RoundTrip(t *testing.T) {
	const w, h, ts = 5, 4, 48
	raw := generateDefaultTiledJSON(w, h, ts)

	md, err := renderer.ParseTiledJSON(raw, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, md)

	assert.Equal(t, w, md.Width)
	assert.Equal(t, h, md.Height)
	assert.Equal(t, ts, md.TileSize)

	require.Len(t, md.TerrainGrid, w*h)
	for i, terr := range md.TerrainGrid {
		assert.Equal(t, renderer.TerrainOpenGround, terr, "terrain[%d]", i)
	}

	assert.Empty(t, md.Walls, "default walls layer is empty")
	assert.Empty(t, md.MagicalDarknessTiles, "default lighting layer has no magical darkness")
	assert.Empty(t, md.SpawnZones, "default spawn_zones layer is empty")

	require.Len(t, md.ElevationByTile, w*h, "elevation grid must be width*height")
	for i, e := range md.ElevationByTile {
		assert.Equal(t, 0, e, "elevation[%d]", i)
	}
}
