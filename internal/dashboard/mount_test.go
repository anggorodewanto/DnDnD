package dashboard

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestMountDashboard_IntegratesWithRouter(t *testing.T) {
	r := chi.NewRouter()
	hub := NewHub()
	go hub.Run()
	t.Cleanup(hub.Stop)

	MountDashboard(r, newTestLogger(), hub, mockAuthMiddleware)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Campaign Home")
}

func TestMountDashboard_AppRoute(t *testing.T) {
	r := chi.NewRouter()
	hub := NewHub()
	go hub.Run()
	t.Cleanup(hub.Stop)

	MountDashboard(r, newTestLogger(), hub, mockAuthMiddleware)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/app/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "DnDnD Map Editor")
}
