package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/inventory"
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

func TestRegisterInventoryAPI_IdentifyEndpoint(t *testing.T) {
	r := chi.NewRouter()
	// Use a nil store — we just need to verify the route is registered (will 400 on bad body)
	invHandler := &inventory.APIHandler{}
	RegisterInventoryAPI(r, invHandler, mockAuthMiddleware)

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/identify", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Should not be 404/405 — route exists. Will be 400 since body is incomplete.
	assert.NotEqual(t, http.StatusNotFound, rec.Code)
	assert.NotEqual(t, http.StatusMethodNotAllowed, rec.Code)
}

// TestRegisterExplorationRoutes_AppliesAuthMiddleware verifies Phase 110 it2:
// exploration dashboard routes are wrapped with authMiddleware (no public
// access) and return a non-404 when auth is provided.
func TestRegisterExplorationRoutes_AppliesAuthMiddleware(t *testing.T) {
	r := chi.NewRouter()
	authCalled := false
	auth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			authCalled = true
			next.ServeHTTP(w, req)
		})
	}
	h := &stubExplorationHandler{}
	RegisterExplorationRoutes(r, h, auth)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/exploration?campaign_id=abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.True(t, authCalled, "auth middleware must wrap exploration routes")
	assert.NotEqual(t, http.StatusNotFound, rec.Code)
	assert.True(t, h.pageCalled, "GET /dashboard/exploration should hit ServePage")
}

func TestRegisterExplorationRoutes_StartEndpointMounted(t *testing.T) {
	r := chi.NewRouter()
	auth := func(next http.Handler) http.Handler { return next }
	h := &stubExplorationHandler{}
	RegisterExplorationRoutes(r, h, auth)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/exploration/start", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusNotFound, rec.Code)
	assert.True(t, h.startCalled, "POST /dashboard/exploration/start should hit HandleStart")
}

// TestSidebarNav_IncludesExploration verifies the DM dashboard sidebar links
// to the exploration page (clarification Q4a: reachable from DM dashboard).
func TestSidebarNav_IncludesExploration(t *testing.T) {
	found := false
	for _, n := range SidebarNav {
		if n.Path == "/dashboard/exploration" {
			found = true
			break
		}
	}
	assert.True(t, found, "SidebarNav should include /dashboard/exploration")
}

type stubExplorationHandler struct {
	pageCalled  bool
	startCalled bool
}

func (s *stubExplorationHandler) ServePage(w http.ResponseWriter, r *http.Request) {
	s.pageCalled = true
	w.WriteHeader(http.StatusOK)
}
func (s *stubExplorationHandler) HandleStart(w http.ResponseWriter, r *http.Request) {
	s.startCalled = true
	w.WriteHeader(http.StatusOK)
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
