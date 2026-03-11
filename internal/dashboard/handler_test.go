package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contextWithUser creates a context with the discord user ID set via auth package.
func contextWithUser(ctx context.Context, userID string) context.Context {
	return auth.ContextWithDiscordUserID(ctx, userID)
}

func TestDashboardHandler_ReturnsHTML(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user123"))
	rec := httptest.NewRecorder()

	h.ServeDashboard(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
}

func TestDashboardHandler_SidebarContainsNavEntries(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user123"))
	rec := httptest.NewRecorder()

	h.ServeDashboard(rec, req)

	body := rec.Body.String()
	expectedEntries := []string{
		"Campaign Home",
		"Character Approval",
		"Encounter Builder",
		"Stat Block Library",
		"Asset Library",
		"Map Editor",
		"Character Overview",
	}
	for _, entry := range expectedEntries {
		assert.True(t, strings.Contains(body, entry), "sidebar should contain %q", entry)
	}
}

func TestDashboardHandler_RequiresAuth(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	// No user in context
	rec := httptest.NewRecorder()

	h.ServeDashboard(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_SetHub_GetHub(t *testing.T) {
	h := NewHandler(nil)
	hub := NewHub()

	h.SetHub(hub)
	assert.Equal(t, hub, h.GetHub())
}

func TestDashboardHandler_CampaignHome_ShowsPlaceholders(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user123"))
	rec := httptest.NewRecorder()

	h.ServeDashboard(rec, req)

	body := rec.Body.String()
	require.Contains(t, body, "dm-queue")
	require.Contains(t, body, "Pending")
	require.Contains(t, body, "New Encounter")
	require.Contains(t, body, "Narrate")
	require.Contains(t, body, "Pause Campaign")
	require.Contains(t, body, "Saved Encounters")
	require.Contains(t, body, "Active Encounters")
}
