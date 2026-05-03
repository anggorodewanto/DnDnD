package main

import (
	"bytes"
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ab/dndnd/internal/discord"
	"github.com/ab/dndnd/internal/testutil"
	"github.com/ab/dndnd/internal/testutil/discordfake"
)

// TestRunWithOptions_InjectsDiscordSessionAndExposesRouter is the GREEN
// driver for the Phase 120 run() seam: it asserts that runWithOptions
// (a) accepts an externally-supplied discord.Session in lieu of buildDiscordSession,
// (b) invokes the supplied "router ready" callback exactly once after wiring
// is complete, and (c) the resulting router is non-nil so the e2e harness can
// route InteractionCreate events from the fake back into the production
// CommandRouter.
func TestRunWithOptions_InjectsDiscordSessionAndExposesRouter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	connStr := testutil.NewTestDBConnString(t)
	t.Setenv("DATABASE_URL", connStr)
	t.Setenv("DISCORD_BOT_TOKEN", "")

	fake := discordfake.New()
	addr := getFreePort(t)
	var logBuf bytes.Buffer

	var routerReadyCount atomic.Int32
	var capturedRouter *discord.CommandRouter

	errCh := make(chan error, 1)
	go func() {
		errCh <- runWithOptions(ctx, &logBuf, addr,
			withDiscordSession(fake),
			withCommandRouterReady(func(r *discord.CommandRouter) {
				routerReadyCount.Add(1)
				capturedRouter = r
			}),
		)
	}()

	// Wait for the HTTP server to come up so we know wiring finished.
	resp := waitForServer(t, addr)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /health, got %d", resp.StatusCode)
	}

	// Allow router-ready callback up to a generous window — wiring runs
	// after migrations + crash recovery so we cannot assume <50ms.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if routerReadyCount.Load() == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if got := routerReadyCount.Load(); got != 1 {
		t.Fatalf("expected router-ready callback to fire exactly once, got %d", got)
	}
	if capturedRouter == nil {
		t.Fatal("expected captured CommandRouter to be non-nil")
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("runWithOptions returned error: %v", err)
	}
}
