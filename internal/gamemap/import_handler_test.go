package gamemap

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// importPart is one named file part for a multipart import request.
type importPart struct {
	field    string // form field name: "tmj" or "images"
	filename string
	mime     string // optional explicit Content-Type
	content  []byte
}

// newImportRequest builds a multipart/form-data POST /api/maps/import request
// carrying the given text fields and file parts.
func newImportRequest(fields map[string]string, parts ...importPart) *http.Request {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	for _, p := range parts {
		fw, _ := mw.CreatePart(newPartHeader(p.field, p.filename, p.mime))
		_, _ = fw.Write(p.content)
	}
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/maps/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// newReimportRequest builds a multipart/form-data PUT /api/maps/{id}/import
// request carrying the given text fields and file parts.
func newReimportRequest(id string, fields map[string]string, parts ...importPart) *http.Request {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	for _, p := range parts {
		fw, _ := mw.CreatePart(newPartHeader(p.field, p.filename, p.mime))
		_, _ = fw.Write(p.content)
	}
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPut, "/api/maps/"+id+"/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// newPartHeader builds a MIME header for a form-data file part.
func newPartHeader(field, filename, mime string) map[string][]string {
	cd := `form-data; name="` + field + `"; filename="` + filename + `"`
	h := map[string][]string{"Content-Disposition": {cd}}
	if mime != "" {
		h["Content-Type"] = []string{mime}
	}
	return h
}

// tmjPart wraps raw .tmj bytes as a file part.
func tmjPart(raw []byte) importPart {
	return importPart{field: "tmj", filename: "map.tmj", mime: "application/json", content: raw}
}

// --- POST /api/maps/import success (abstract map, no images) ---

func TestHandler_ImportMap_Success(t *testing.T) {
	campaignID := uuid.New()
	store := successStore(campaignID)
	_, r := newTestRouter(store)

	req := newImportRequest(
		map[string]string{"campaign_id": campaignID.String(), "name": "Imported Map"},
		tmjPart(validTmj(15, 12)),
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp importMapResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Imported Map", resp.Map.Name)
	assert.Equal(t, 15, resp.Map.Width)
	assert.Equal(t, 12, resp.Map.Height)
	assert.NotNil(t, resp.Skipped)
	assert.Empty(t, resp.Skipped)
}

// --- POST /api/maps/import returns skipped features ---

func TestHandler_ImportMap_ReturnsSkippedFeatures(t *testing.T) {
	campaignID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))

	tmj := []byte(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[
			{"type":"tilelayer","name":"terrain","width":10,"height":10,"data":[1]}
		],
		"tilesets":[
			{"firstgid":1,"name":"t","wangsets":[{"name":"corners","type":"corner"}]}
		]
	}`)
	req := newImportRequest(
		map[string]string{"campaign_id": campaignID.String(), "name": "Has WangSet"},
		tmjPart(tmj),
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var resp importMapResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Skipped, 1)
	assert.Equal(t, SkippedWangSet, resp.Skipped[0].Feature)
}

// --- POST /api/maps/import hard rejection ---

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
			req := newImportRequest(
				map[string]string{"campaign_id": campaignID.String(), "name": "X"},
				tmjPart([]byte(tc.tmj)),
			)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.want)
		})
	}
}

// --- POST /api/maps/import not multipart -> 400 ---

func TestHandler_ImportMap_NotMultipart(t *testing.T) {
	_, r := newTestRouter(&mockStore{})
	req := httptest.NewRequest(http.MethodPost, "/api/maps/import", bytes.NewReader([]byte("not multipart")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- POST /api/maps/import invalid campaign_id ---

func TestHandler_ImportMap_InvalidCampaignID(t *testing.T) {
	_, r := newTestRouter(&mockStore{})
	req := newImportRequest(
		map[string]string{"campaign_id": "not-a-uuid", "name": "X"},
		tmjPart(validTmj(10, 10)),
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "campaign_id")
}

// --- POST /api/maps/import missing tmj file ---

func TestHandler_ImportMap_MissingTmj(t *testing.T) {
	campaignID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))
	req := newImportRequest(map[string]string{"campaign_id": campaignID.String(), "name": "X"})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "tmj")
}

// --- POST /api/maps/import empty tmj file ---

func TestHandler_ImportMap_EmptyTmj(t *testing.T) {
	campaignID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))
	req := newImportRequest(
		map[string]string{"campaign_id": campaignID.String(), "name": "X"},
		tmjPart([]byte{}),
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "tmj")
}

// --- ImportMap surfaces store errors as 500 ---

func TestHandler_ImportMap_StoreError(t *testing.T) {
	campaignID := uuid.New()
	store := &mockStore{
		createMapFn: func(_ context.Context, _ refdata.CreateMapParams) (refdata.Map, error) {
			return refdata.Map{}, assert.AnError
		},
	}
	_, r := newTestRouter(store)
	req := newImportRequest(
		map[string]string{"campaign_id": campaignID.String(), "name": "X"},
		tmjPart(validTmj(10, 10)),
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- ImportMap rejects validation failures (empty name) ---

func TestHandler_ImportMap_EmptyName(t *testing.T) {
	campaignID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))
	req := newImportRequest(
		map[string]string{"campaign_id": campaignID.String(), "name": ""},
		tmjPart(validTmj(10, 10)),
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "name must not be empty")
}

// --- ImportMap rejects a tmj payload the importer can't accept ---

func TestHandler_ImportMap_InvalidTmjPayload(t *testing.T) {
	campaignID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))
	req := newImportRequest(
		map[string]string{"campaign_id": campaignID.String(), "name": "X"},
		tmjPart([]byte(`"not a tmj"`)),
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid Tiled JSON")
}

// --- Multipart import of a map with an embedded tileset image succeeds and
// rewrites the stored tileset image to an asset URL ---

func TestHandler_ImportMap_WithTilesetImage(t *testing.T) {
	campaignID := uuid.New()
	var captured json.RawMessage
	svc := NewService(captureStore(campaignID, &captured)).SetImageUploader(&fakeUploader{})
	h := NewHandler(svc)
	_, r := newRouterFromHandler(h)

	req := newImportRequest(
		map[string]string{"campaign_id": campaignID.String(), "name": "Tileset Map"},
		tmjPart(tmjWithTileset("dungeon.png")),
		importPart{field: "images", filename: "dungeon.png", mime: "image/png", content: []byte("PNGDATA")},
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(captured, &parsed))
	tilesets := parsed["tilesets"].([]any)
	ts := tilesets[0].(map[string]any)
	assert.Equal(t, "/api/assets/dungeon.png", ts["image"])
}

// --- Multipart import detects the MIME type when the part omits Content-Type ---

func TestHandler_ImportMap_TilesetImage_SniffsMime(t *testing.T) {
	campaignID := uuid.New()
	var captured json.RawMessage
	up := &fakeUploader{}
	svc := NewService(captureStore(campaignID, &captured)).SetImageUploader(up)
	h := NewHandler(svc)
	_, r := newRouterFromHandler(h)

	// PNG magic bytes so http.DetectContentType returns image/png.
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	req := newImportRequest(
		map[string]string{"campaign_id": campaignID.String(), "name": "Sniff Map"},
		tmjPart(tmjWithTileset("dungeon.png")),
		importPart{field: "images", filename: "dungeon.png", content: pngMagic},
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, up.calls, 1)
	assert.Equal(t, "image/png", up.calls[0].mimeType)
}

// --- Multipart import with a missing image returns 400 ---

func TestHandler_ImportMap_MissingImage(t *testing.T) {
	campaignID := uuid.New()
	svc := NewService(successStore(campaignID)).SetImageUploader(&fakeUploader{})
	h := NewHandler(svc)
	_, r := newRouterFromHandler(h)

	req := newImportRequest(
		map[string]string{"campaign_id": campaignID.String(), "name": "Missing Img"},
		tmjPart(tmjWithTileset("dungeon.png")),
		// no images part
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "dungeon.png")
}

// --- PUT /api/maps/{id}/import overwrites an existing map in place ---

func TestHandler_ReimportMap_Success(t *testing.T) {
	campaignID := uuid.New()
	mapID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))

	req := newReimportRequest(
		mapID.String(),
		map[string]string{"campaign_id": campaignID.String(), "name": "Reimported Map"},
		tmjPart(validTmj(15, 12)),
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp importMapResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, mapID.String(), resp.Map.ID, "same map ID is preserved")
	assert.Equal(t, "Reimported Map", resp.Map.Name)
	assert.Equal(t, 15, resp.Map.Width)
	assert.Equal(t, 12, resp.Map.Height)
	assert.NotNil(t, resp.Skipped)
	assert.Empty(t, resp.Skipped)
}

// --- PUT /api/maps/{id}/import invalid map id -> 400 ---

func TestHandler_ReimportMap_InvalidMapID(t *testing.T) {
	campaignID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))

	req := newReimportRequest(
		"not-a-uuid",
		map[string]string{"campaign_id": campaignID.String(), "name": "X"},
		tmjPart(validTmj(10, 10)),
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "map id")
}

// --- PUT /api/maps/{id}/import missing tmj file -> 400 ---

func TestHandler_ReimportMap_MissingTmj(t *testing.T) {
	campaignID := uuid.New()
	mapID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))

	req := newReimportRequest(
		mapID.String(),
		map[string]string{"campaign_id": campaignID.String(), "name": "X"},
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "tmj")
}

// --- PUT /api/maps/{id}/import for a missing map -> 404 ---

func TestHandler_ReimportMap_NotFound(t *testing.T) {
	campaignID := uuid.New()
	store := successStore(campaignID)
	store.getMapByIDFn = func(ctx context.Context, arg refdata.GetMapByIDParams) (refdata.Map, error) {
		return refdata.Map{}, errNotFound
	}
	_, r := newTestRouter(store)

	req := newReimportRequest(
		uuid.New().String(),
		map[string]string{"campaign_id": campaignID.String(), "name": "X"},
		tmjPart(validTmj(10, 10)),
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not found")
}

// --- PUT /api/maps/{id}/import with a bad campaign_id -> 400 ---

func TestHandler_ReimportMap_InvalidCampaignID(t *testing.T) {
	campaignID := uuid.New()
	_, r := newTestRouter(successStore(campaignID))

	req := newReimportRequest(
		uuid.New().String(),
		map[string]string{"campaign_id": "not-a-uuid", "name": "X"},
		tmjPart(validTmj(10, 10)),
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "campaign_id")
}
