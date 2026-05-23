package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/errorlog"
)

// decodeErrorsResponse decodes the JSON body produced by GET /api/errors.
func decodeErrorsResponse(t *testing.T, rec *httptest.ResponseRecorder) errorsListResponse {
	t.Helper()
	var got errorsListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	return got
}

func TestErrorsJSON_ListsRecentErrorsMostRecentFirst(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	current := now
	clock := func() time.Time { return current }
	store := errorlog.NewMemoryStore(clock)

	current = now.Add(-10 * time.Minute)
	_ = store.Record(context.Background(), errorlog.Entry{
		Command: "cast",
		UserID:  "alice",
		Summary: "DB timeout on /cast by @alice",
	})
	current = now.Add(-5 * time.Minute)
	_ = store.Record(context.Background(), errorlog.Entry{
		Command: "render",
		UserID:  "",
		Summary: "PNG generation failed for encounter X",
	})
	current = now

	handler := NewErrorsHandler(nil, store, clock)
	r := chi.NewRouter()
	r.Get("/api/errors", handler.ServeJSON)

	req := httptest.NewRequest(http.MethodGet, "/api/errors", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	got := decodeErrorsResponse(t, rec)
	require.Len(t, got.Errors, 2)
	// Most recent first -> render before cast.
	assert.Equal(t, "render", got.Errors[0].Command)
	assert.Equal(t, "PNG generation failed for encounter X", got.Errors[0].Summary)
	assert.Equal(t, "cast", got.Errors[1].Command)
	assert.Equal(t, "alice", got.Errors[1].UserID)
	assert.Equal(t, now.Add(-5*time.Minute).UTC().Format(time.RFC3339), got.Errors[0].Timestamp)
}

func TestErrorsJSON_FiltersOlderThan24h(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	current := now
	clock := func() time.Time { return current }
	store := errorlog.NewMemoryStore(clock)

	// 48h ago should be excluded.
	current = now.Add(-48 * time.Hour)
	_ = store.Record(context.Background(), errorlog.Entry{Command: "old", Summary: "old failure"})
	// 1h ago should be included.
	current = now.Add(-1 * time.Hour)
	_ = store.Record(context.Background(), errorlog.Entry{Command: "fresh", Summary: "fresh failure"})
	current = now

	handler := NewErrorsHandler(nil, store, clock)
	req := httptest.NewRequest(http.MethodGet, "/api/errors", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()
	handler.ServeJSON(rec, req)

	got := decodeErrorsResponse(t, rec)
	require.Len(t, got.Errors, 1)
	assert.Equal(t, "fresh", got.Errors[0].Command)
}

func TestErrorsJSON_EmptyState(t *testing.T) {
	store := errorlog.NewMemoryStore(nil)
	handler := NewErrorsHandler(nil, store, time.Now)

	req := httptest.NewRequest(http.MethodGet, "/api/errors", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()
	handler.ServeJSON(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	got := decodeErrorsResponse(t, rec)
	assert.NotNil(t, got.Errors, "errors field must be a non-nil array even when empty")
	assert.Len(t, got.Errors, 0)
}

func TestErrorsJSON_NilReaderReturnsEmpty(t *testing.T) {
	handler := NewErrorsHandler(nil, nil, time.Now)
	req := httptest.NewRequest(http.MethodGet, "/api/errors", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()
	handler.ServeJSON(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	got := decodeErrorsResponse(t, rec)
	assert.Len(t, got.Errors, 0)
}

func TestErrorsJSON_RequiresAuth(t *testing.T) {
	store := errorlog.NewMemoryStore(nil)
	handler := NewErrorsHandler(nil, store, time.Now)

	req := httptest.NewRequest(http.MethodGet, "/api/errors", nil)
	rec := httptest.NewRecorder()
	handler.ServeJSON(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRegisterErrorsRoutes_MountsJSONEndpoint(t *testing.T) {
	store := errorlog.NewMemoryStore(nil)
	handler := NewErrorsHandler(nil, store, time.Now)

	r := chi.NewRouter()
	RegisterErrorsRoutes(r, handler, mockAuthMiddleware)

	req := httptest.NewRequest(http.MethodGet, "/api/errors", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

// TestMountErrorsRoutes_MountsJSONEndpoint verifies the convenience wrapper
// mounts the JSON endpoint under the supplied router.
func TestMountErrorsRoutes_MountsJSONEndpoint(t *testing.T) {
	store := errorlog.NewMemoryStore(nil)
	r := chi.NewRouter()
	dash := NewHandler(nil, nil)

	MountErrorsRoutes(r, dash, store, time.Now, mockAuthMiddleware)

	req := httptest.NewRequest(http.MethodGet, "/api/errors", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMountErrorsRoutes_NilClockDefaultsToTimeNow(t *testing.T) {
	store := errorlog.NewMemoryStore(nil)
	r := chi.NewRouter()

	// Passing a nil clock must not panic and must still mount the route.
	MountErrorsRoutes(r, nil, store, nil, mockAuthMiddleware)

	req := httptest.NewRequest(http.MethodGet, "/api/errors", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}
