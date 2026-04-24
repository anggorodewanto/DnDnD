package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/errorlog"
)

func TestSidebarNav_IncludesErrors(t *testing.T) {
	found := false
	for _, n := range SidebarNav {
		if n.Path == "/dashboard/errors" {
			found = true
			break
		}
	}
	assert.True(t, found, "SidebarNav should include /dashboard/errors")
}

func TestDashboardHandler_ShowsErrorBadge24hCount(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	current := now
	clock := func() time.Time { return current }

	store := errorlog.NewMemoryStore(clock)
	// Two recent errors (within 24h) and one old (48h ago).
	current = now.Add(-48 * time.Hour)
	_ = store.Record(context.Background(), errorlog.Entry{Command: "cast"})
	current = now.Add(-1 * time.Hour)
	_ = store.Record(context.Background(), errorlog.Entry{Command: "attack"})
	current = now.Add(-30 * time.Minute)
	_ = store.Record(context.Background(), errorlog.Entry{Command: "move"})
	current = now

	h := NewHandler(nil, nil)
	h.SetErrorReader(store, clock)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()
	h.ServeDashboard(rec, req)

	body := rec.Body.String()
	// The sidebar entry for errors should include the 24h count.
	require.Contains(t, body, "Errors")
	require.Contains(t, body, "(2)", "sidebar should display 24h error count in parens")
}

func TestErrorsPage_ListsRecentErrors(t *testing.T) {
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
	r.Get("/dashboard/errors", handler.ServePage)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/errors", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "DB timeout on /cast by @alice")
	require.Contains(t, body, "PNG generation failed")
	// Most recent first -> render should appear before cast.
	rIdx := strings.Index(body, "PNG generation failed")
	cIdx := strings.Index(body, "DB timeout")
	require.True(t, rIdx < cIdx, "most recent error should appear first")
}

func TestErrorsPage_RequiresAuth(t *testing.T) {
	store := errorlog.NewMemoryStore(nil)
	handler := NewErrorsHandler(nil, store, time.Now)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/errors", nil)
	rec := httptest.NewRecorder()
	handler.ServePage(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestErrorsPage_EmptyState(t *testing.T) {
	store := errorlog.NewMemoryStore(nil)
	handler := NewErrorsHandler(nil, store, time.Now)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/errors", nil)
	req = req.WithContext(contextWithUser(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()
	handler.ServePage(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "No errors")
}

func TestRegisterErrorsRoutes_Mounted(t *testing.T) {
	store := errorlog.NewMemoryStore(nil)
	handler := NewErrorsHandler(nil, store, time.Now)

	r := chi.NewRouter()
	RegisterErrorsRoutes(r, handler, mockAuthMiddleware)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/errors", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, http.StatusOK, rec.Code)
}
