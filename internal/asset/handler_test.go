package asset

import (
	"context"
	"database/sql"
	"io"
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

func newTestRouter(h *Handler) *chi.Mux {
	r := chi.NewRouter()
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
