package asset

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

func newTestRouter(h *Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/api/assets/upload", h.UploadAsset)
	r.Get("/api/assets/{id}", h.ServeAsset)
	return r
}

func TestHandler_ServeAsset_OK(t *testing.T) {
	assetID := uuid.New()
	db := &mockDBStore{
		getAssetFn: func(ctx context.Context, id uuid.UUID) (refdata.Asset, error) {
			return refdata.Asset{
				ID:          assetID,
				MimeType:    "image/png",
				StoragePath: "a/b/c",
				ByteSize:    9,
			}, nil
		},
	}
	fs := &mockFileStore{
		getFn: func(ctx context.Context, sp string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("imagedata")), nil
		},
	}
	svc := NewService(db, fs)
	h := NewHandler(svc)
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/assets/"+assetID.String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	assert.Equal(t, "imagedata", rec.Body.String())
}

func TestHandler_ServeAsset_InvalidUUID(t *testing.T) {
	svc := NewService(&mockDBStore{}, &mockFileStore{})
	h := NewHandler(svc)
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/assets/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ServeAsset_NotFound(t *testing.T) {
	db := &mockDBStore{
		getAssetFn: func(ctx context.Context, id uuid.UUID) (refdata.Asset, error) {
			return refdata.Asset{}, sql.ErrNoRows
		},
	}
	svc := NewService(db, &mockFileStore{})
	h := NewHandler(svc)
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/assets/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_ServeAsset_FileOpenError(t *testing.T) {
	assetID := uuid.New()
	db := &mockDBStore{
		getAssetFn: func(ctx context.Context, id uuid.UUID) (refdata.Asset, error) {
			return refdata.Asset{ID: assetID, StoragePath: "a/b/c"}, nil
		},
	}
	fs := &mockFileStore{
		getFn: func(ctx context.Context, sp string) (io.ReadCloser, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}
	svc := NewService(db, fs)
	h := NewHandler(svc)
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/assets/"+assetID.String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// File missing from disk -> 500
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_ServeAsset_ContentLength(t *testing.T) {
	assetID := uuid.New()
	db := &mockDBStore{
		getAssetFn: func(ctx context.Context, id uuid.UUID) (refdata.Asset, error) {
			return refdata.Asset{
				ID:          assetID,
				MimeType:    "application/json",
				StoragePath: "a/b/c",
				ByteSize:    42,
			}, nil
		},
	}
	fs := &mockFileStore{
		getFn: func(ctx context.Context, sp string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("{}")), nil
		},
	}
	svc := NewService(db, fs)
	h := NewHandler(svc)
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/assets/"+assetID.String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "42", rec.Header().Get("Content-Length"))
}

// --- TDD Cycle 1: POST /api/assets/upload with multipart form ---

func TestHandler_UploadAsset_Success(t *testing.T) {
	campaignID := uuid.New()
	assetID := uuid.New()

	db := &mockDBStore{
		createAssetFn: func(ctx context.Context, arg refdata.CreateAssetParams) (refdata.Asset, error) {
			return refdata.Asset{
				ID:           assetID,
				CampaignID:   arg.CampaignID,
				Type:         arg.Type,
				OriginalName: arg.OriginalName,
				MimeType:     arg.MimeType,
				ByteSize:     arg.ByteSize,
				StoragePath:  arg.StoragePath,
			}, nil
		},
	}
	fs := &mockFileStore{
		putFn: func(ctx context.Context, cid uuid.UUID, at AssetType, fn string, r io.Reader) (string, error) {
			return "camp/maps/abc", nil
		},
		urlFn: func(id uuid.UUID) string {
			return "/api/assets/" + id.String()
		},
	}
	svc := NewService(db, fs)
	h := NewHandler(svc)
	router := newTestRouter(h)

	// Build multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("campaign_id", campaignID.String())
	writer.WriteField("type", "map_background")
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="map.png"`)
	partHeader.Set("Content-Type", "image/png")
	part, _ := writer.CreatePart(partHeader)
	part.Write([]byte("fakepngdata"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, assetID.String(), resp["id"])
	assert.Contains(t, resp["url"].(string), assetID.String())
}

func TestHandler_UploadAsset_MissingCampaignID(t *testing.T) {
	svc := NewService(&mockDBStore{}, &mockFileStore{})
	h := NewHandler(svc)
	router := newTestRouter(h)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("type", "map_background")
	part, _ := writer.CreateFormFile("file", "map.png")
	part.Write([]byte("data"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_UploadAsset_MissingFile(t *testing.T) {
	svc := NewService(&mockDBStore{}, &mockFileStore{})
	h := NewHandler(svc)
	router := newTestRouter(h)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("campaign_id", uuid.New().String())
	writer.WriteField("type", "map_background")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_UploadAsset_InvalidType(t *testing.T) {
	svc := NewService(&mockDBStore{}, &mockFileStore{})
	h := NewHandler(svc)
	router := newTestRouter(h)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("campaign_id", uuid.New().String())
	writer.WriteField("type", "bogus_type")
	part, _ := writer.CreateFormFile("file", "map.png")
	part.Write([]byte("data"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid asset type")
}

func TestHandler_UploadAsset_InvalidMultipartForm(t *testing.T) {
	svc := NewService(&mockDBStore{}, &mockFileStore{})
	h := NewHandler(svc)
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_UploadAsset_ServiceError(t *testing.T) {
	db := &mockDBStore{
		createAssetFn: func(ctx context.Context, arg refdata.CreateAssetParams) (refdata.Asset, error) {
			return refdata.Asset{}, errors.New("db error")
		},
	}
	fs := &mockFileStore{
		putFn: func(ctx context.Context, cid uuid.UUID, at AssetType, fn string, r io.Reader) (string, error) {
			return "path", nil
		},
	}
	svc := NewService(db, fs)
	h := NewHandler(svc)
	router := newTestRouter(h)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("campaign_id", uuid.New().String())
	writer.WriteField("type", "map_background")
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="map.png"`)
	partHeader.Set("Content-Type", "image/png")
	part, _ := writer.CreatePart(partHeader)
	part.Write([]byte("data"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_RegisterRoutes(t *testing.T) {
	svc := NewService(&mockDBStore{}, &mockFileStore{})
	h := NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	// Test that upload route is registered
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("campaign_id", uuid.New().String())
	writer.WriteField("type", "map_background")
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="test.png"`)
	partHeader.Set("Content-Type", "image/png")
	part, _ := writer.CreatePart(partHeader)
	part.Write([]byte("data"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Should get 201 (upload works) or 400 (validation) but NOT 404/405
	assert.NotEqual(t, http.StatusNotFound, rec.Code)
	assert.NotEqual(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestHandler_UploadAsset_DisallowedMimeType(t *testing.T) {
	svc := NewService(&mockDBStore{}, &mockFileStore{})
	h := NewHandler(svc)
	router := newTestRouter(h)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("campaign_id", uuid.New().String())
	writer.WriteField("type", "map_background")
	// Create a part with text/html content type (disallowed)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="evil.html"`)
	partHeader.Set("Content-Type", "text/html")
	part, _ := writer.CreatePart(partHeader)
	part.Write([]byte("<script>alert('xss')</script>"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "mime type not allowed")
}

func TestHandler_UploadAsset_AllowedMimeType_ImagePNG(t *testing.T) {
	campaignID := uuid.New()
	assetID := uuid.New()

	db := &mockDBStore{
		createAssetFn: func(ctx context.Context, arg refdata.CreateAssetParams) (refdata.Asset, error) {
			return refdata.Asset{ID: assetID, CampaignID: arg.CampaignID}, nil
		},
	}
	fs := &mockFileStore{
		putFn: func(ctx context.Context, cid uuid.UUID, at AssetType, fn string, r io.Reader) (string, error) {
			return "path", nil
		},
		urlFn: func(id uuid.UUID) string { return "/api/assets/" + id.String() },
	}
	svc := NewService(db, fs)
	h := NewHandler(svc)
	router := newTestRouter(h)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("campaign_id", campaignID.String())
	writer.WriteField("type", "map_background")
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="map.png"`)
	partHeader.Set("Content-Type", "image/png")
	part, _ := writer.CreatePart(partHeader)
	part.Write([]byte("fakepng"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestHandler_UploadAsset_AllowedMimeType_JSONForTileset(t *testing.T) {
	campaignID := uuid.New()
	assetID := uuid.New()

	db := &mockDBStore{
		createAssetFn: func(ctx context.Context, arg refdata.CreateAssetParams) (refdata.Asset, error) {
			return refdata.Asset{ID: assetID, CampaignID: arg.CampaignID}, nil
		},
	}
	fs := &mockFileStore{
		putFn: func(ctx context.Context, cid uuid.UUID, at AssetType, fn string, r io.Reader) (string, error) {
			return "path", nil
		},
		urlFn: func(id uuid.UUID) string { return "/api/assets/" + id.String() },
	}
	svc := NewService(db, fs)
	h := NewHandler(svc)
	router := newTestRouter(h)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("campaign_id", campaignID.String())
	writer.WriteField("type", "tileset")
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="tiles.json"`)
	partHeader.Set("Content-Type", "application/json")
	part, _ := writer.CreatePart(partHeader)
	part.Write([]byte(`{"tiles":[]}`))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}
