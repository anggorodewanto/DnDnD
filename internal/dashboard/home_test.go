package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/auth"
)

// homeStubLookup satisfies CampaignLookup with deterministic data.
type homeStubLookup struct {
	id     string
	status string
	err    error
	seen   string
}

func (s *homeStubLookup) LookupActiveCampaign(_ context.Context, dmUserID string) (string, string, error) {
	s.seen = dmUserID
	return s.id, s.status, s.err
}

type homeStubApprovals struct {
	count int
	err   error
}

func (s *homeStubApprovals) CountPendingApprovals(_ context.Context, _ uuid.UUID) (int, error) {
	return s.count, s.err
}

type homeStubDMQueue struct {
	count int
	err   error
}

func (s *homeStubDMQueue) CountPendingDMQueue(_ context.Context, _ uuid.UUID) (int, error) {
	return s.count, s.err
}

type homeStubEncounters struct {
	active []string
	saved  []string
	err    error
}

func (s *homeStubEncounters) ListActiveEncounterNames(_ context.Context, _ uuid.UUID) ([]string, error) {
	return s.active, s.err
}

func (s *homeStubEncounters) ListSavedEncounterNames(_ context.Context, _ uuid.UUID) ([]string, error) {
	return s.saved, s.err
}

func TestHomeHandler_ReturnsFullPayload(t *testing.T) {
	cid := "11111111-1111-1111-1111-111111111111"
	dash := NewHandler(nil, nil)
	dash.SetCampaignLookup(&homeStubLookup{id: cid, status: "active"})
	dash.SetCounters(&homeStubApprovals{count: 4}, &homeStubDMQueue{count: 7})
	dash.SetEncounterLister(&homeStubEncounters{
		active: []string{"Battle at the Bridge"},
		saved:  []string{"Boss Fight", "Ambush"},
	})

	h := NewHomeHandler(nil, dash)
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/home", nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var got HomeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, cid, got.CampaignID)
	assert.Equal(t, "active", got.CampaignStatus)
	assert.Equal(t, 7, got.DMQueueCount)
	assert.Equal(t, 4, got.PendingApprovals)
	assert.Equal(t, []string{"Battle at the Bridge"}, got.ActiveEncounters)
	assert.Equal(t, []string{"Boss Fight", "Ambush"}, got.SavedEncounters)
}

func TestHomeHandler_RequiresAuth(t *testing.T) {
	h := NewHomeHandler(nil, NewHandler(nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/home", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHomeHandler_NoCampaign_ReturnsEmptyArraysAndZeros(t *testing.T) {
	dash := NewHandler(nil, nil)
	// No campaign lookup configured — campaign id stays empty.
	h := NewHomeHandler(nil, dash)
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/home", nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got HomeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Empty(t, got.CampaignID)
	assert.Empty(t, got.CampaignStatus)
	assert.Equal(t, 0, got.DMQueueCount)
	assert.Equal(t, 0, got.PendingApprovals)
	assert.NotNil(t, got.ActiveEncounters, "active_encounters must be a JSON array, not null")
	assert.NotNil(t, got.SavedEncounters, "saved_encounters must be a JSON array, not null")
	assert.Len(t, got.ActiveEncounters, 0)
	assert.Len(t, got.SavedEncounters, 0)
}

func TestHomeHandler_LookupErrors_DegradeGracefully(t *testing.T) {
	cid := "11111111-1111-1111-1111-111111111111"
	dash := NewHandler(nil, nil)
	dash.SetCampaignLookup(&homeStubLookup{id: cid, status: "active"})
	dash.SetCounters(
		&homeStubApprovals{err: errors.New("boom")},
		&homeStubDMQueue{err: errors.New("boom")},
	)
	dash.SetEncounterLister(&homeStubEncounters{err: errors.New("boom")})

	h := NewHomeHandler(nil, dash)
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/home", nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got HomeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, cid, got.CampaignID)
	assert.Equal(t, "active", got.CampaignStatus)
	assert.Equal(t, 0, got.DMQueueCount)
	assert.Equal(t, 0, got.PendingApprovals)
	assert.Len(t, got.ActiveEncounters, 0)
	assert.Len(t, got.SavedEncounters, 0)
}

func TestHomeHandler_PausedCampaign_StatusPropagates(t *testing.T) {
	cid := "22222222-2222-2222-2222-222222222222"
	dash := NewHandler(nil, nil)
	dash.SetCampaignLookup(&homeStubLookup{id: cid, status: "paused"})

	h := NewHomeHandler(nil, dash)
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/home", nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got HomeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "paused", got.CampaignStatus)
}

func TestHomeHandler_PassesAuthenticatedUserToLookup(t *testing.T) {
	dash := NewHandler(nil, nil)
	lookup := &homeStubLookup{id: "11111111-1111-1111-1111-111111111111", status: "active"}
	dash.SetCampaignLookup(lookup)

	h := NewHomeHandler(nil, dash)
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/home", nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-99"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, "dm-99", lookup.seen)
}

func TestRegisterHomeRoute_AppliesAuthMiddleware(t *testing.T) {
	called := false
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			ctx := auth.ContextWithDiscordUserID(r.Context(), "mw-user")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	r := chi.NewRouter()
	RegisterHomeRoute(r, NewHomeHandler(nil, NewHandler(nil, nil)), mw)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/home", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, called, "auth middleware must wrap /api/dashboard/home")
}
