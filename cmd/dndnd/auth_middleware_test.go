package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/testutil"
)

func TestPassthroughMiddleware_InjectsDevUserAndForwardsRequest(t *testing.T) {
	t.Setenv("DEV_DISCORD_USER_ID", "")

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		userID, ok := auth.DiscordUserIDFromContext(r.Context())
		require.True(t, ok)
		assert.Equal(t, "local-dev", userID)
		w.WriteHeader(http.StatusTeapot)
	})

	wrapped := passthroughMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	require.True(t, called, "passthroughMiddleware must call the inner handler")
	assert.Equal(t, http.StatusTeapot, w.Code)
}

func TestPassthroughMiddleware_UsesConfiguredDevUser(t *testing.T) {
	t.Setenv("DEV_DISCORD_USER_ID", "12345")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.DiscordUserIDFromContext(r.Context())
		require.True(t, ok)
		assert.Equal(t, "12345", userID)
		w.WriteHeader(http.StatusTeapot)
	})

	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
	w := httptest.NewRecorder()
	passthroughMiddleware(next).ServeHTTP(w, req)

	assert.Equal(t, http.StatusTeapot, w.Code)
}

func TestBuildAuthMiddleware_FallsBackWithoutEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		clientID string
		secret   string
	}{
		{"both missing", "", ""},
		{"client ID missing", "", "some-secret"},
		{"client secret missing", "some-id", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DISCORD_CLIENT_ID", tt.clientID)
			t.Setenv("DISCORD_CLIENT_SECRET", tt.secret)

			var logBuf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

			bundle, err := buildAuth(nil, logger)
			require.NoError(t, err)
			mw := bundle.middleware

			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusTeapot)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			mw(next).ServeHTTP(w, req)

			require.True(t, called, "fallback middleware must call the inner handler")
			assert.Equal(t, http.StatusTeapot, w.Code)
			assert.Contains(t, logBuf.String(), "DISCORD_CLIENT_ID")
		})
	}
}

// T08 / finding 6·e: a set-but-invalid TOKEN_ENCRYPTION_KEY silently
// downgrades OAuth tokens to plaintext-at-rest. When the operator has declared
// production intent via COOKIE_SECURE=true, that is a security regression, so
// buildAuth must refuse to boot rather than log-and-continue.
func TestBuildAuth_RefusesBootOnInvalidKeyWhenCookieSecure(t *testing.T) {
	t.Setenv("DISCORD_CLIENT_ID", "test-client-id")
	t.Setenv("DISCORD_CLIENT_SECRET", "test-client-secret")
	t.Setenv("COOKIE_SECURE", "true")
	// openssl rand -hex 32 yields 64 chars — the canonical wrong-length key.
	t.Setenv("TOKEN_ENCRYPTION_KEY", strings.Repeat("a", 64))

	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	_, err := buildAuth(nil, logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "TOKEN_ENCRYPTION_KEY")
}

// Outside production (COOKIE_SECURE not "true") the same bad key keeps the soft
// path: log an error and continue with plaintext tokens so a fat-fingered key
// never blocks a local playtest.
func TestBuildAuth_InvalidKeyWithoutCookieSecure_SoftFallback(t *testing.T) {
	t.Setenv("DISCORD_CLIENT_ID", "test-client-id")
	t.Setenv("DISCORD_CLIENT_SECRET", "test-client-secret")
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("TOKEN_ENCRYPTION_KEY", strings.Repeat("a", 64))

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	bundle, err := buildAuth(nil, logger)

	require.NoError(t, err)
	require.NotNil(t, bundle.middleware)
	assert.Contains(t, logBuf.String(), "invalid TOKEN_ENCRYPTION_KEY")
}

func TestBuildAuthMiddleware_RejectsWithoutCookie(t *testing.T) {
	t.Setenv("DISCORD_CLIENT_ID", "test-client-id")
	t.Setenv("DISCORD_CLIENT_SECRET", "test-client-secret")

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	bundle, err := buildAuth(nil, logger)
	require.NoError(t, err)
	mw := bundle.middleware

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, req)

	assert.False(t, called, "real middleware must NOT call next without a session cookie")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRun_AuthProtectedRoutesRejectUnauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	connStr := testutil.NewTestDBConnString(t)
	t.Setenv("DATABASE_URL", connStr)
	t.Setenv("DISCORD_CLIENT_ID", "test-client-id")
	t.Setenv("DISCORD_CLIENT_SECRET", "test-client-secret")

	addr := getFreePort(t)
	var logBuf bytes.Buffer

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, &logBuf, addr)
	}()

	waitForServer(t, addr)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"dm queue", http.MethodGet, "/dashboard/queue/some-item", ""},
		{"inventory API", http.MethodPost, "/api/inventory/add", "{}"},
		// SR-001 / F-2: every DM-mutation route group below should
		// require auth. Walking one path per group is enough — the
		// gating is shared (mountDMOnlyAPIs).
		{"map import", http.MethodPost, "/api/maps/import", "{}"},
		{"statblocks list", http.MethodGet, "/api/statblocks/", ""},
		{"homebrew create", http.MethodPost, "/api/homebrew/creatures", "{}"},
		{"campaign pause", http.MethodPost, "/api/campaigns/00000000-0000-0000-0000-000000000001/pause", "{}"},
		{"narration post", http.MethodPost, "/api/narration/post", "{}"},
		{"narration templates list", http.MethodGet, "/api/narration/templates/", ""},
		{"character overview", http.MethodGet, "/api/character-overview?campaign_id=00000000-0000-0000-0000-000000000001", ""},
		{"message player", http.MethodPost, "/api/message-player/", "{}"},
		{"combat start", http.MethodPost, "/api/combat/start", "{}"},
		{"combat advance-turn", http.MethodPost, "/api/combat/00000000-0000-0000-0000-000000000001/advance-turn", "{}"},
		{"combat workspace", http.MethodGet, "/api/combat/workspace?campaign_id=00000000-0000-0000-0000-000000000001", ""},
		// SR-063: levelup routes gated by dmAuthMw.
		{"levelup post", http.MethodPost, "/api/levelup/", "{}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("http://%s%s", addr, tt.path)

			var resp *http.Response
			var err error
			if tt.method == http.MethodPost {
				resp, err = http.Post(url, "application/json", bytes.NewReader([]byte(tt.body)))
			} else {
				resp, err = http.Get(url)
			}
			if err != nil {
				t.Fatalf("failed to reach %s: %v", tt.path, err)
			}
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}

	cancel()
	<-errCh
}
