package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/auth"
)

type meStubResolver struct {
	id     string
	status string
	err    error
	seen   string
}

func (s *meStubResolver) LookupActiveCampaign(_ context.Context, dmUserID string) (string, string, error) {
	s.seen = dmUserID
	return s.id, s.status, s.err
}

func TestMeHandler_ReturnsCampaignAndUser(t *testing.T) {
	resolver := &meStubResolver{id: "11111111-1111-1111-1111-111111111111", status: "active"}
	h := NewMeHandler(nil, resolver)

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "user-99"))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	var got MeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "user-99", got.DiscordUserID)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", got.CampaignID)
	assert.Equal(t, "active", got.Status)
	assert.Equal(t, "user-99", resolver.seen, "resolver receives the authenticated discord user id")
}

func TestMeHandler_RequiresAuth(t *testing.T) {
	h := NewMeHandler(nil, &meStubResolver{})

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMeHandler_NilResolver_ReturnsEmptyCampaignID(t *testing.T) {
	h := NewMeHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "user-1"))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got MeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "user-1", got.DiscordUserID)
	assert.Empty(t, got.CampaignID)
}

func TestMeHandler_ResolverError_DegradesToEmptyCampaignID(t *testing.T) {
	h := NewMeHandler(nil, &meStubResolver{err: errors.New("db down")})

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "user-1"))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got MeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "user-1", got.DiscordUserID)
	assert.Empty(t, got.CampaignID, "errors degrade to empty campaign id, still 200 OK")
}

func TestRegisterMeRoute_BehindAuthMiddleware(t *testing.T) {
	called := false
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			ctx := auth.ContextWithDiscordUserID(r.Context(), "user-from-mw")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
	r := chi.NewRouter()
	RegisterMeRoute(r, NewMeHandler(nil, &meStubResolver{id: "abc"}), mw)

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, called, "auth middleware must run on /api/me")
}
