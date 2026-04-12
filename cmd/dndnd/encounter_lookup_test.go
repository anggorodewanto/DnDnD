package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/testutil"
)

// TestPassthroughMiddleware_ForwardsRequestUnchanged verifies the no-op
// middleware used as a fallback when OAuth env vars are not set.
func TestPassthroughMiddleware_ForwardsRequestUnchanged(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	})

	wrapped := passthroughMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	require.True(t, called, "passthroughMiddleware must call the inner handler")
	assert.Equal(t, http.StatusTeapot, w.Code)
}

// TestBuildAuthMiddleware_FallsBackWithoutEnvVars verifies that when
// DISCORD_CLIENT_ID or DISCORD_CLIENT_SECRET are not set, buildAuthMiddleware
// returns a passthrough middleware (no-op) and logs a warning.
func TestBuildAuthMiddleware_FallsBackWithoutEnvVars(t *testing.T) {
	t.Setenv("DISCORD_CLIENT_ID", "")
	t.Setenv("DISCORD_CLIENT_SECRET", "")

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	mw := buildAuthMiddleware(nil, logger)

	// The returned middleware should be a passthrough — it must call next
	// without blocking.
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
}

// TestBuildAuthMiddleware_RejectsWithoutCookie verifies that when OAuth env
// vars are set and a db is provided, the returned middleware rejects requests
// without a valid session cookie with 401.
func TestBuildAuthMiddleware_RejectsWithoutCookie(t *testing.T) {
	t.Setenv("DISCORD_CLIENT_ID", "test-client-id")
	t.Setenv("DISCORD_CLIENT_SECRET", "test-client-secret")

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	// We pass a nil db here — NewSessionStore accepts *sql.DB and won't
	// error on construction. The middleware will try to read a cookie first
	// and reject before hitting the DB.
	mw := buildAuthMiddleware(nil, logger)

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

// TestBuildAuthMiddleware_MissingClientIDOnly verifies fallback when only
// DISCORD_CLIENT_ID is missing but DISCORD_CLIENT_SECRET is set.
func TestBuildAuthMiddleware_MissingClientIDOnly(t *testing.T) {
	t.Setenv("DISCORD_CLIENT_ID", "")
	t.Setenv("DISCORD_CLIENT_SECRET", "some-secret")

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	mw := buildAuthMiddleware(nil, logger)

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
}

// TestBuildAuthMiddleware_MissingClientSecretOnly verifies fallback when only
// DISCORD_CLIENT_SECRET is missing but DISCORD_CLIENT_ID is set.
func TestBuildAuthMiddleware_MissingClientSecretOnly(t *testing.T) {
	t.Setenv("DISCORD_CLIENT_ID", "some-id")
	t.Setenv("DISCORD_CLIENT_SECRET", "")

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	mw := buildAuthMiddleware(nil, logger)

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
}

// TestRun_DMQueueRouteRejectsUnauthenticated verifies the integration: when
// OAuth env vars are set and the server is running with a database, the
// /dashboard/queue/{id} route rejects unauthenticated requests with 401.
func TestRun_DMQueueRouteRejectsUnauthenticated(t *testing.T) {
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

	// GET /dashboard/queue/some-item without session cookie — must get 401,
	// NOT 200 or 404.
	resp, err := http.Get(fmt.Sprintf("http://%s/dashboard/queue/some-item", addr))
	if err != nil {
		t.Fatalf("failed to reach /dashboard/queue/some-item: %v", err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	cancel()
	<-errCh
}

// TestRun_InventoryAPIRejectsUnauthenticated verifies that /api/inventory
// routes reject unauthenticated requests with 401 when OAuth is configured.
func TestRun_InventoryAPIRejectsUnauthenticated(t *testing.T) {
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

	resp, err := http.Post(fmt.Sprintf("http://%s/api/inventory/add", addr), "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("failed to reach /api/inventory/add: %v", err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	cancel()
	<-errCh
}
