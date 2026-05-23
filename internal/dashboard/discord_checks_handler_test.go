package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/discordcheck"
)

func TestDiscordChecksHandler_RequiresAuth(t *testing.T) {
	holder := &atomic.Pointer[discordcheck.Report]{}
	h := NewDiscordChecksHandler(nil, holder)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/discord-checks", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDiscordChecksHandler_NoReport_ReturnsDisabled(t *testing.T) {
	holder := &atomic.Pointer[discordcheck.Report]{}
	h := NewDiscordChecksHandler(nil, holder)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/discord-checks", nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, true, got["disabled"])
	assert.NotNil(t, got["results"])
	results, ok := got["results"].([]any)
	require.True(t, ok, "results must be a JSON array")
	assert.Len(t, results, 0)
}

func TestDiscordChecksHandler_ReturnsLatestReport(t *testing.T) {
	holder := &atomic.Pointer[discordcheck.Report]{}
	now := time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC)
	report := discordcheck.Report{
		RanAt: now,
		Results: []discordcheck.Result{
			{Name: "token-identity", OK: true, Detail: "bot DnDnD"},
			{Name: "guild-membership-g-1", OK: false, Detail: "bot is not a member of guild g-1"},
		},
	}
	holder.Store(&report)

	h := NewDiscordChecksHandler(nil, holder)
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/discord-checks", nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var got discordcheck.Report
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Results, 2)
	assert.Equal(t, "token-identity", got.Results[0].Name)
	assert.True(t, got.Results[0].OK)
	assert.Equal(t, "guild-membership-g-1", got.Results[1].Name)
	assert.False(t, got.Results[1].OK)
	assert.WithinDuration(t, now, got.RanAt, time.Second)
}

func TestRegisterDiscordChecksRoute_AppliesAuthMiddleware(t *testing.T) {
	called := false
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			ctx := auth.ContextWithDiscordUserID(r.Context(), "mw-user")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	holder := &atomic.Pointer[discordcheck.Report]{}
	r := chi.NewRouter()
	RegisterDiscordChecksRoute(r, NewDiscordChecksHandler(nil, holder), mw)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/discord-checks", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, called, "auth middleware must wrap /api/dashboard/discord-checks")
}

func TestDiscordChecksHandler_NilHolder_TreatsAsDisabled(t *testing.T) {
	h := NewDiscordChecksHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/discord-checks", nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, true, got["disabled"])
}
