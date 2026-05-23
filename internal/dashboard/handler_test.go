package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/dndnd/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contextWithUser creates a context with the discord user ID set via auth package.
func contextWithUser(ctx context.Context, userID string) context.Context {
	return auth.ContextWithDiscordUserID(ctx, userID)
}

func TestDashboardHandler_RedirectsToSvelteHome(t *testing.T) {
	h := NewHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user123"))
	rec := httptest.NewRecorder()

	h.ServeDashboard(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/dashboard/app/#home", rec.Header().Get("Location"))
}

func TestDashboardHandler_RequiresAuth(t *testing.T) {
	h := NewHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	// No user in context.
	rec := httptest.NewRecorder()

	h.ServeDashboard(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestNewHandler_SetsHub(t *testing.T) {
	hub := NewHub()
	h := NewHandler(nil, hub)
	assert.Equal(t, hub, h.hub)
}

// stubCampaignLookup satisfies CampaignLookup for tests without touching a DB.
type stubCampaignLookup struct {
	id     string
	status string
	err    error
	seen   string
}

func (s *stubCampaignLookup) LookupActiveCampaign(_ context.Context, dmUserID string) (string, string, error) {
	s.seen = dmUserID
	return s.id, s.status, s.err
}

func TestDashboardHandler_SetCampaignLookup_Nil_ClearsLookup(t *testing.T) {
	h := NewHandler(nil, nil)
	h.SetCampaignLookup(&stubCampaignLookup{id: "x", status: "active"})
	h.SetCampaignLookup(nil)

	id, status := h.lookupCampaign(context.Background(), "user-1")
	assert.Empty(t, id)
	assert.Empty(t, status)
}

func TestHandler_LookupCampaign_PropagatesLookupResult(t *testing.T) {
	h := NewHandler(nil, nil)
	lookup := &stubCampaignLookup{id: "abc", status: "active"}
	h.SetCampaignLookup(lookup)

	id, status := h.lookupCampaign(context.Background(), "user-7")
	assert.Equal(t, "abc", id)
	assert.Equal(t, "active", status)
	assert.Equal(t, "user-7", lookup.seen)
}

func TestHandler_LookupCampaign_ErrorDegradesToEmpty(t *testing.T) {
	h := NewHandler(nil, nil)
	h.SetCampaignLookup(&stubCampaignLookup{err: errLookupFailed})

	id, status := h.lookupCampaign(context.Background(), "user-7")
	assert.Empty(t, id)
	assert.Empty(t, status)
}

var errLookupFailed = &lookupErr{msg: "lookup failed"}

type lookupErr struct{ msg string }

func (e *lookupErr) Error() string { return e.msg }
