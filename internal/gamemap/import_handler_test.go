package gamemap

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- TDD Cycle 15: POST /api/maps/import success ---

func TestHandler_ImportMap_Success(t *testing.T) {
	campaignID := uuid.New()
	store := successStore(campaignID)
	_, r := newTestRouter(store)

	body := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"name":        "Imported Map",
		"tmj":         json.RawMessage(validTmj(15, 12)),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/maps/import", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp importMapResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Imported Map", resp.Map.Name)
	assert.Equal(t, 15, resp.Map.Width)
	assert.Equal(t, 12, resp.Map.Height)
	assert.NotNil(t, resp.Skipped)
	assert.Empty(t, resp.Skipped)
}

// --- TDD Cycle 16: POST /api/maps/import returns skipped features ---

func TestHandler_ImportMap_ReturnsSkippedFeatures(t *testing.T) {
	campaignID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))

	tmj := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[
			{"type":"tilelayer","name":"terrain","width":10,"height":10,"data":[1]},
			{"type":"imagelayer","name":"bg","image":"bg.png"}
		]
	}`)
	body := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"name":        "Has Image",
		"tmj":         tmj,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/maps/import", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var resp importMapResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Skipped, 1)
	assert.Equal(t, SkippedImageLayer, resp.Skipped[0].Feature)
}

// --- TDD Cycle 17: POST /api/maps/import hard rejection ---

func TestHandler_ImportMap_HardRejection(t *testing.T) {
	campaignID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))

	cases := []struct {
		name string
		tmj  string
		want string
	}{
		{
			"infinite",
			`{"orientation":"orthogonal","width":10,"height":10,"tilewidth":48,"tileheight":48,"infinite":true,"layers":[]}`,
			"infinite",
		},
		{
			"isometric",
			`{"orientation":"isometric","width":10,"height":10,"tilewidth":48,"tileheight":48,"infinite":false,"layers":[]}`,
			"orthogonal",
		},
		{
			"too_large",
			`{"orientation":"orthogonal","width":250,"height":10,"tilewidth":48,"tileheight":48,"infinite":false,"layers":[]}`,
			"hard limit",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]interface{}{
				"campaign_id": campaignID.String(),
				"name":        "X",
				"tmj":         json.RawMessage(tc.tmj),
			}
			b, _ := json.Marshal(body)
			req := httptest.NewRequest(http.MethodPost, "/api/maps/import", bytes.NewReader(b))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.want)
		})
	}
}

// --- TDD Cycle 18: POST /api/maps/import invalid JSON body ---

func TestHandler_ImportMap_InvalidBody(t *testing.T) {
	_, r := newTestRouter(&mockStore{})
	req := httptest.NewRequest(http.MethodPost, "/api/maps/import", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 19: POST /api/maps/import invalid campaign_id ---

func TestHandler_ImportMap_InvalidCampaignID(t *testing.T) {
	_, r := newTestRouter(&mockStore{})
	body := map[string]interface{}{
		"campaign_id": "not-a-uuid",
		"name":        "X",
		"tmj":         json.RawMessage(validTmj(10, 10)),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/maps/import", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ImportMap_MissingTmj(t *testing.T) {
	campaignID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))
	body := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"name":        "X",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/maps/import", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "tmj")
}

func TestHandler_ImportMap_InvalidBackgroundImageID(t *testing.T) {
	campaignID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))
	body := map[string]interface{}{
		"campaign_id":         campaignID.String(),
		"name":                "X",
		"tmj":                 json.RawMessage(validTmj(10, 10)),
		"background_image_id": "not-a-uuid",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/maps/import", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "background_image_id")
}

// --- TDD Cycle 22: ImportMap surfaces store errors as 500 ---

func TestHandler_ImportMap_StoreError(t *testing.T) {
	campaignID := uuid.New()
	store := &mockStore{
		createMapFn: func(_ context.Context, _ refdata.CreateMapParams) (refdata.Map, error) {
			return refdata.Map{}, assert.AnError
		},
	}
	_, r := newTestRouter(store)
	body := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"name":        "X",
		"tmj":         json.RawMessage(validTmj(10, 10)),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/maps/import", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle 23: ImportMap rejects validation failures (e.g. empty name) ---

func TestHandler_ImportMap_EmptyName(t *testing.T) {
	campaignID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))
	body := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"name":        "",
		"tmj":         json.RawMessage(validTmj(10, 10)),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/maps/import", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "name must not be empty")
}

func TestHandler_ImportMap_InvalidTmjPayload(t *testing.T) {
	campaignID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))
	// Outer body parses, but `tmj` is a valid JSON value the importer can't accept (a string, not a map).
	body := map[string]interface{}{
		"campaign_id": campaignID.String(),
		"name":        "X",
		"tmj":         json.RawMessage(`"not a tmj"`),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/maps/import", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid Tiled JSON")
}

func TestHandler_ImportMap_WithBackgroundImageID(t *testing.T) {
	campaignID := uuid.New()
	bgID := uuid.New()
	var capturedBgID uuid.NullUUID
	store := &mockStore{
		createMapFn: func(_ context.Context, arg refdata.CreateMapParams) (refdata.Map, error) {
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
		"name":                "X",
		"tmj":                 json.RawMessage(validTmj(10, 10)),
		"background_image_id": bgID.String(),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/maps/import", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	assert.True(t, capturedBgID.Valid)
	assert.Equal(t, bgID, capturedBgID.UUID)

	var resp importMapResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.Map.BackgroundImageID)
	assert.Equal(t, bgID.String(), *resp.Map.BackgroundImageID)
}
