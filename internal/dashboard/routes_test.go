package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAuthMiddleware injects a fake user ID for testing.
func mockAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(contextWithUser(r.Context(), "dm-user-test"))
		next.ServeHTTP(w, r)
	})
}

// setupRoutesTest creates a chi router with dashboard routes registered and a running hub.
// The caller must call the returned cleanup function (typically via defer).
func setupRoutesTest(t *testing.T) chi.Router {
	t.Helper()
	hub := NewHub()
	go hub.Run()
	t.Cleanup(hub.Stop)

	h := NewHandler(nil, hub)
	r := chi.NewRouter()
	RegisterRoutes(r, h, mockAuthMiddleware)
	return r
}

func TestRegisterRoutes_DashboardEndpoint(t *testing.T) {
	r := setupRoutesTest(t)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Campaign Home")
}

func TestRegisterRoutes_SvelteAppStub(t *testing.T) {
	r := setupRoutesTest(t)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/app/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "DnDnD Map Editor")
}

func TestRegisterRoutes_WebSocketEndpoint(t *testing.T) {
	r := setupRoutesTest(t)

	// Test that the websocket endpoint exists (will fail upgrade but route is found)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/ws", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusNotFound, rec.Code)
}

func TestRegisterRoutes_DashboardIncludesWSScript(t *testing.T) {
	r := setupRoutesTest(t)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	body := rec.Body.String()
	require.Contains(t, body, "WebSocket")
	require.Contains(t, body, "/dashboard/ws")
}

func TestSvelteAppStub_ContainsMountPoint(t *testing.T) {
	r := setupRoutesTest(t)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/app/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.True(t, strings.Contains(body, "id=\"app\"") || strings.Contains(body, `id="svelte-app"`),
		"SPA stub should have a mount point element")
}
