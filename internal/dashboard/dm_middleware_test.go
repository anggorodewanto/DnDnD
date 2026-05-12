package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/auth"
)

// stubDMVerifier implements DMVerifier for tests. isDM is the canned answer
// when discordUserID matches dmUserID; err short-circuits ahead of either
// path so we can prove the middleware degrades to 403 on lookup failures.
type stubDMVerifier struct {
	dmUserID string
	err      error
	calls    int
}

func (s *stubDMVerifier) IsDM(_ context.Context, discordUserID string) (bool, error) {
	s.calls++
	if s.err != nil {
		return false, s.err
	}
	return discordUserID != "" && discordUserID == s.dmUserID, nil
}

// passthroughHandler is the downstream handler used by the middleware tests.
// It records whether it was called and returns 200. The middleware test
// asserts called==false on the reject path.
type passthroughHandler struct {
	called bool
}

func (p *passthroughHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	p.called = true
	w.WriteHeader(http.StatusOK)
}

func TestRequireDM_AllowsDM(t *testing.T) {
	verifier := &stubDMVerifier{dmUserID: "dm-1"}
	next := &passthroughHandler{}
	mw := RequireDM(verifier)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, next.called, "downstream handler must run when user is DM")
	assert.Equal(t, 1, verifier.calls)
}

func TestRequireDM_RejectsNonDM(t *testing.T) {
	verifier := &stubDMVerifier{dmUserID: "dm-1"}
	next := &passthroughHandler{}
	mw := RequireDM(verifier)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/queue/abc/resolve", nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "player-7"))
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.False(t, next.called, "downstream handler must NOT run when user is not DM")

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "forbidden: DM only", body["error"])
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestRequireDM_RejectsMissingUserContext(t *testing.T) {
	verifier := &stubDMVerifier{dmUserID: "dm-1"}
	next := &passthroughHandler{}
	mw := RequireDM(verifier)

	// No auth.ContextWithDiscordUserID applied — simulates a misconfigured
	// router that mounted RequireDM without SessionMiddleware in front.
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.False(t, next.called)
	assert.Equal(t, 0, verifier.calls, "verifier must NOT be called when no user id is in context")
}

func TestRequireDM_RejectsOnVerifierError(t *testing.T) {
	verifier := &stubDMVerifier{err: errors.New("db down")}
	next := &passthroughHandler{}
	mw := RequireDM(verifier)

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/add", nil)
	req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), "dm-1"))
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.False(t, next.called)
}

func TestRequireDM_NilVerifierFallsThrough(t *testing.T) {
	// A nil verifier disables the gate (matches the existing passthrough
	// pattern used when DISCORD_CLIENT_ID is not set). This keeps the local
	// dev path working without OAuth: SessionMiddleware is passthrough, so
	// RequireDM is also passthrough.
	next := &passthroughHandler{}
	mw := RequireDM(nil)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, next.called)
}
