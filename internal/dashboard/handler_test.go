package dashboard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contextWithUser creates a context with the discord user ID set via auth package.
func contextWithUser(ctx context.Context, userID string) context.Context {
	return auth.ContextWithDiscordUserID(ctx, userID)
}

func TestDashboardHandler_ReturnsHTML(t *testing.T) {
	h := NewHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user123"))
	rec := httptest.NewRecorder()

	h.ServeDashboard(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
}

func TestDashboardHandler_SidebarContainsNavEntries(t *testing.T) {
	h := NewHandler(nil, nil)
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
		assert.Contains(t, body, entry, "sidebar should contain %q", entry)
	}
}

func TestDashboardHandler_RequiresAuth(t *testing.T) {
	h := NewHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	// No user in context
	rec := httptest.NewRecorder()

	h.ServeDashboard(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestNewHandler_SetsHub(t *testing.T) {
	hub := NewHub()
	h := NewHandler(nil, hub)
	assert.Equal(t, hub, h.hub)
}

func TestDashboardHandler_CampaignHome_ShowsPlaceholders(t *testing.T) {
	h := NewHandler(nil, nil)
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

// --- Phase 115 dashboard wiring ---

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

func TestDashboardHandler_Phase115_PauseButton_WhenCampaignActive(t *testing.T) {
	h := NewHandler(nil, nil)
	h.SetCampaignLookup(&stubCampaignLookup{id: "11111111-1111-1111-1111-111111111111", status: "active"})

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user123"))
	rec := httptest.NewRecorder()
	h.ServeDashboard(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, "data-campaign-id=\"11111111-1111-1111-1111-111111111111\"", "button must carry the campaign id")
	assert.Contains(t, body, ">Pause Campaign<", "active campaign shows Pause label")
	assert.Contains(t, body, "/api/campaigns/", "JS handler must hit the pause/resume endpoint")
	assert.Contains(t, body, "btn-pause", "pause button must be present")
}

func TestDashboardHandler_Phase115_ResumeButton_WhenCampaignPaused(t *testing.T) {
	h := NewHandler(nil, nil)
	h.SetCampaignLookup(&stubCampaignLookup{id: "22222222-2222-2222-2222-222222222222", status: "paused"})

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user123"))
	rec := httptest.NewRecorder()
	h.ServeDashboard(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, "data-campaign-id=\"22222222-2222-2222-2222-222222222222\"")
	assert.Contains(t, body, ">Resume Campaign<", "paused campaign shows Resume label")
}

func TestDashboardHandler_Phase115_NoLookup_ButtonStillRenders(t *testing.T) {
	h := NewHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user123"))
	rec := httptest.NewRecorder()
	h.ServeDashboard(rec, req)

	body := rec.Body.String()
	// Without a lookup configured the button still appears (disabled-looking,
	// no data-campaign-id) so existing tests and pre-Phase-115 deploys keep
	// working.
	assert.Contains(t, body, "Pause Campaign")
}

func TestDashboardHandler_Phase115_LookupError_DegradesGracefully(t *testing.T) {
	h := NewHandler(nil, nil)
	h.SetCampaignLookup(&stubCampaignLookup{err: errLookupFailed})

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user123"))
	rec := httptest.NewRecorder()
	h.ServeDashboard(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "lookup error must not break the dashboard render")
	body := rec.Body.String()
	assert.Contains(t, body, "Pause Campaign", "falls back to default label")
	assert.NotContains(t, body, "data-campaign-id=\"11111111", "no campaign id rendered on error")
}

func TestCampaignHomeData_PauseButtonLabel(t *testing.T) {
	assert.Equal(t, "Pause Campaign", CampaignHomeData{CampaignStatus: "active"}.PauseButtonLabel())
	assert.Equal(t, "Resume Campaign", CampaignHomeData{CampaignStatus: "paused"}.PauseButtonLabel())
	assert.Equal(t, "Pause Campaign", CampaignHomeData{CampaignStatus: ""}.PauseButtonLabel())
}

func TestDashboardHandler_SetCampaignLookup_Nil_ClearsLookup(t *testing.T) {
	h := NewHandler(nil, nil)
	h.SetCampaignLookup(&stubCampaignLookup{id: "x", status: "active"})
	h.SetCampaignLookup(nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user123"))
	rec := httptest.NewRecorder()
	h.ServeDashboard(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

var errLookupFailed = &lookupErr{msg: "lookup failed"}

type lookupErr struct{ msg string }

func (e *lookupErr) Error() string { return e.msg }

// --- med-40 / Phase 15: Campaign Home live counts ---

type stubApprovalsCounter struct {
	count int
	err   error
	seen  uuid.UUID
}

func (s *stubApprovalsCounter) CountPendingApprovals(_ context.Context, campaignID uuid.UUID) (int, error) {
	s.seen = campaignID
	return s.count, s.err
}

type stubDMQueueCounter struct {
	count int
	err   error
	seen  uuid.UUID
}

func (s *stubDMQueueCounter) CountPendingDMQueue(_ context.Context, campaignID uuid.UUID) (int, error) {
	s.seen = campaignID
	return s.count, s.err
}

func TestDashboardHandler_CampaignHome_RendersLiveCounts(t *testing.T) {
	cid := "11111111-1111-1111-1111-111111111111"
	h := NewHandler(nil, nil)
	h.SetCampaignLookup(&stubCampaignLookup{id: cid, status: "active"})
	approvals := &stubApprovalsCounter{count: 4}
	dmQueue := &stubDMQueueCounter{count: 7}
	h.SetCounters(approvals, dmQueue)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user123"))
	rec := httptest.NewRecorder()
	h.ServeDashboard(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, ">7<", "DM queue count must render")
	assert.Contains(t, body, ">4<", "pending approvals count must render")
	assert.Equal(t, cid, approvals.seen.String(), "approvals counter receives the active campaign id")
	assert.Equal(t, cid, dmQueue.seen.String(), "dm-queue counter receives the active campaign id")
}

func TestDashboardHandler_CampaignHome_CounterErrors_DegradeToZero(t *testing.T) {
	cid := "11111111-1111-1111-1111-111111111111"
	h := NewHandler(nil, nil)
	h.SetCampaignLookup(&stubCampaignLookup{id: cid, status: "active"})
	h.SetCounters(
		&stubApprovalsCounter{err: errors.New("boom")},
		&stubDMQueueCounter{err: errors.New("boom")},
	)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user123"))
	rec := httptest.NewRecorder()
	h.ServeDashboard(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "counter errors must not break the render")
	body := rec.Body.String()
	assert.Contains(t, body, ">0<", "counts fall back to 0 on error")
}

func TestDashboardHandler_CampaignHome_NoCampaign_KeepsZeroCounts(t *testing.T) {
	h := NewHandler(nil, nil)
	approvals := &stubApprovalsCounter{count: 99}
	dmQueue := &stubDMQueueCounter{count: 99}
	h.SetCounters(approvals, dmQueue)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = req.WithContext(contextWithUser(req.Context(), "user123"))
	rec := httptest.NewRecorder()
	h.ServeDashboard(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, uuid.Nil, approvals.seen, "no lookup id => approvals counter never invoked")
	assert.Equal(t, uuid.Nil, dmQueue.seen, "no lookup id => dm-queue counter never invoked")
}
