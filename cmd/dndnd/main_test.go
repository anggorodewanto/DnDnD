package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/ab/dndnd/internal/testutil"
)

// getFreePort asks the OS for a free port and returns it as a "host:port" string.
func getFreePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

// waitForServer polls until the health endpoint responds or the test fails.
func waitForServer(t *testing.T, addr string) *http.Response {
	t.Helper()
	for range 30 {
		resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
		if err == nil {
			return resp
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("server did not start in time")
	return nil
}

func TestRun_ServerStartsAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var logBuf bytes.Buffer

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, &logBuf, ":0")
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-errCh
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	// Verify structured JSON logs were produced
	if logBuf.Len() == 0 {
		t.Fatal("expected log output, got none")
	}

	// Check that at least one log line is valid JSON
	var entry map[string]any
	decoder := json.NewDecoder(&logBuf)
	if err := decoder.Decode(&entry); err != nil {
		t.Fatalf("log output is not valid JSON: %v", err)
	}
}

func TestRun_ListenError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logBuf bytes.Buffer

	// Use an invalid address to trigger a listen error
	err := run(ctx, &logBuf, ":-1")
	if err == nil {
		t.Fatal("expected error for invalid address, got nil")
	}
}

func TestRun_HealthEndpointFunctional(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := getFreePort(t)
	var logBuf bytes.Buffer

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, &logBuf, addr)
	}()

	resp := waitForServer(t, addr)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}

	cancel()
	err := <-errCh
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

func TestRun_InvalidDatabaseURL(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logBuf bytes.Buffer

	// Set an invalid DATABASE_URL — run() should return an error
	t.Setenv("DATABASE_URL", "postgres://invalid:5432/nonexistent?connect_timeout=1")

	err := run(ctx, &logBuf, ":0")
	if err == nil {
		t.Fatal("expected error for invalid DATABASE_URL, got nil")
	}
}

func TestRun_NoDatabaseURL(t *testing.T) {
	// When DATABASE_URL is not set, server should start without database
	ctx, cancel := context.WithCancel(context.Background())

	var logBuf bytes.Buffer

	t.Setenv("DATABASE_URL", "")

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, &logBuf, ":0")
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-errCh
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

func TestRun_WithDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	connStr := testutil.NewTestDBConnString(t)
	t.Setenv("DATABASE_URL", connStr)

	addr := getFreePort(t)
	var logBuf bytes.Buffer

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, &logBuf, addr)
	}()

	resp := waitForServer(t, addr)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	cancel()
	err := <-errCh
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	// Verify log mentions database connected
	if !bytes.Contains(logBuf.Bytes(), []byte("database connected and migrated")) {
		t.Fatal("expected log to mention database connected and migrated")
	}
}

func TestRun_CombatAPIRoutesRegistered(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	connStr := testutil.NewTestDBConnString(t)
	t.Setenv("DATABASE_URL", connStr)

	addr := getFreePort(t)
	var logBuf bytes.Buffer

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, &logBuf, addr)
	}()

	waitForServer(t, addr)

	// GET /api/combat/characters without campaign_id should return 400,
	// NOT 404 — proving the combat handler is mounted on the router.
	resp, err := http.Get(fmt.Sprintf("http://%s/api/combat/characters", addr))
	if err != nil {
		t.Fatalf("failed to reach /api/combat/characters: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Fatal("/api/combat/characters returned 404; combat handler is not wired into the server")
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 from /api/combat/characters without campaign_id, got %d", resp.StatusCode)
	}

	cancel()
	<-errCh
}

// TestRun_LevelUpAPIRoutesRegistered proves Phase 104c wired
// levelup.Handler onto the router. POSTing an empty body must return 400
// (the handler's own validation error) rather than 404 (unrouted).
func TestRun_LevelUpAPIRoutesRegistered(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	connStr := testutil.NewTestDBConnString(t)
	t.Setenv("DATABASE_URL", connStr)

	addr := getFreePort(t)
	var logBuf bytes.Buffer

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, &logBuf, addr)
	}()

	waitForServer(t, addr)

	// Empty body — handler rejects with 400. A 404 here would mean the
	// route is not mounted.
	resp, err := http.Post(fmt.Sprintf("http://%s/api/levelup/", addr), "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("failed to reach /api/levelup/: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Fatal("/api/levelup/ returned 404; levelup handler is not wired into the server")
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 from /api/levelup/ with empty body, got %d", resp.StatusCode)
	}

	cancel()
	<-errCh
}

func TestRun_MapAPIRoutesRegistered(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	connStr := testutil.NewTestDBConnString(t)
	t.Setenv("DATABASE_URL", connStr)

	addr := getFreePort(t)
	var logBuf bytes.Buffer

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, &logBuf, addr)
	}()

	waitForServer(t, addr)

	// GET /api/maps without campaign_id should return 400 (Bad Request),
	// NOT 404 — proving the route is registered.
	resp, err := http.Get(fmt.Sprintf("http://%s/api/maps", addr))
	if err != nil {
		t.Fatalf("failed to reach /api/maps: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Fatal("/api/maps returned 404; map API routes are not wired into the server")
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 from /api/maps without campaign_id, got %d", resp.StatusCode)
	}

	cancel()
	<-errCh
}

func TestBuildDiscordSession_EmptyTokenReturnsNil(t *testing.T) {
	sess, dg, err := buildDiscordSession("")
	if err != nil {
		t.Fatalf("expected nil error for empty token, got %v", err)
	}
	if sess != nil || dg != nil {
		t.Fatalf("expected (nil, nil) for empty token, got (%v, %v)", sess, dg)
	}
}

func TestBuildDiscordSession_NonEmptyTokenReturnsSession(t *testing.T) {
	// discordgo.New is lenient and does not hit the network, so we can
	// deterministically assert that a non-empty token yields a constructed
	// (but not yet opened) session pair. This replaces an earlier test that
	// used t.Skip to paper over network-dependent Open() calls.
	sess, dg, err := buildDiscordSession("Bot fake-token-for-unit-tests")
	if err != nil {
		t.Fatalf("unexpected error from buildDiscordSession: %v", err)
	}
	if sess == nil {
		t.Fatal("expected non-nil Session wrapper")
	}
	if dg == nil {
		t.Fatal("expected non-nil *discordgo.Session")
	}
}

func TestRun_DiscordTokenUnset_DoesNotError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var logBuf bytes.Buffer

	t.Setenv("DATABASE_URL", "")
	t.Setenv("DISCORD_BOT_TOKEN", "")

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, &logBuf, ":0")
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

func TestRun_DefaultAddr(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	addr := getFreePort(t)
	var logBuf bytes.Buffer

	// Set ADDR env so run() picks it up as default
	t.Setenv("ADDR", addr)

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, &logBuf, "")
	}()

	resp := waitForServer(t, addr)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	cancel()
	err := <-errCh
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}
