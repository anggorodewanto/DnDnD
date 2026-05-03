//go:build e2e
// +build e2e

// Phase 121.3 transcript replay bridge: drives the Phase 120 e2e harness
// from a JSON-lines transcript captured by cmd/playtest-player. Each
// dispatch entry is fed into the production CommandRouter via
// h.PlayerCommand; each observed entry is matched (after normalization)
// against the next outbound transcript entry the harness records.
//
// Round-trip test:    record a synthesized session against the harness,
//                     replay through a fresh harness, assert green.
// Drift test:         mutate one expected line, assert replay fails loudly.
// File-driven test:   `make playtest-replay TRANSCRIPT=path` reads a
//                     transcript file off disk and replays it. Default is
//                     internal/playtest/testdata/sample.jsonl.
package main

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ab/dndnd/internal/playtest"
	"github.com/ab/dndnd/internal/testutil/discordfake"
)

// harnessDispatcher routes a ParsedCommand into the e2e harness's
// PlayerCommand seam. The player ID is fixed at construction time so a
// transcript can be re-targeted at a different player without rewriting.
type harnessDispatcher struct {
	h         *e2eHarness
	playerID  string
	lastIntID string
}

func (d *harnessDispatcher) Dispatch(cmd playtest.ParsedCommand) error {
	opts := make([]slashOpt, 0, len(cmd.NamedArgs))
	for k, v := range cmd.NamedArgs {
		opts = append(opts, stringOpt(k, v))
	}
	d.lastIntID = d.h.PlayerCommand(d.playerID, cmd.Name, opts...)
	return nil
}

// harnessObserver consumes the harness's transcript in order. A cursor
// marks the next un-consumed entry so each Wait returns the next
// outbound message rather than re-matching earlier ones.
type harnessObserver struct {
	mu     sync.Mutex
	fake   *discordfake.Fake
	cursor int
}

func (o *harnessObserver) Wait(timeout time.Duration) (string, error) {
	o.mu.Lock()
	cursor := o.cursor
	o.mu.Unlock()
	idx := -1
	entry, err := o.fake.WaitFor(func(e discordfake.Entry) bool {
		// WaitFor scans from the start each call; track the index by
		// counting matches against the cursor target.
		idx++
		return idx == cursor
	}, timeout)
	if err != nil {
		return "", err
	}
	o.mu.Lock()
	o.cursor = cursor + 1
	o.mu.Unlock()
	return entry.Content, nil
}

// TestE2E_ReplayRoundtrip records a synthesized session against the
// harness, then replays the same transcript through a fresh harness
// and asserts every line matches.
func TestE2E_ReplayRoundtrip(t *testing.T) {
	transcript := captureRecapTranscript(t)

	h := startE2EHarness(t)
	defer h.Stop()
	h.SeedCampaign("replay-roundtrip-campaign")
	playerID := "user-replay"
	h.SeedApprovedPlayer(playerID, "Echo")

	disp := &harnessDispatcher{h: h, playerID: playerID}
	obs := &harnessObserver{fake: h.fake}

	if err := playtest.Replay(transcript, disp, obs, playtest.ReplayOptions{
		WaitTimeout: 5 * time.Second,
	}); err != nil {
		t.Fatalf("Replay: %v\nTranscript dump:\n%s", err, h.RenderTranscript())
	}
}

// TestE2E_ReplayDetectsDrift mutates one expected observation in the
// transcript and asserts Replay surfaces a drift error rather than
// silently accepting the divergence.
func TestE2E_ReplayDetectsDrift(t *testing.T) {
	transcript := captureRecapTranscript(t)

	// Find the observed entry and mutate its content to something the
	// real bot will never produce.
	mutated := false
	for i, e := range transcript {
		if e.Direction == playtest.DirectionObserved {
			transcript[i].Content = "WRONG: this string is not in the bot's reply"
			mutated = true
			break
		}
	}
	if !mutated {
		t.Fatalf("expected at least one observed entry in synthesized transcript")
	}

	h := startE2EHarness(t)
	defer h.Stop()
	h.SeedCampaign("replay-drift-campaign")
	playerID := "user-drift"
	h.SeedApprovedPlayer(playerID, "Faye")

	disp := &harnessDispatcher{h: h, playerID: playerID}
	obs := &harnessObserver{fake: h.fake}

	err := playtest.Replay(transcript, disp, obs, playtest.ReplayOptions{
		WaitTimeout: 3 * time.Second,
	})
	if err == nil {
		t.Fatalf("expected drift error; got nil")
	}
	if !strings.Contains(err.Error(), "drift") {
		t.Fatalf("expected drift error; got %v", err)
	}
}

// TestE2E_ReplayFromFile drives the harness from a JSON-lines
// transcript on disk. PLAYTEST_TRANSCRIPT overrides the default path so
// `make playtest-replay TRANSCRIPT=path/to/file.jsonl` plays an
// arbitrary recording.
func TestE2E_ReplayFromFile(t *testing.T) {
	path := os.Getenv("PLAYTEST_TRANSCRIPT")
	if path == "" {
		path = "../../internal/playtest/testdata/sample.jsonl"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transcript %s: %v", path, err)
	}
	entries, err := playtest.LoadTranscript(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("LoadTranscript: %v", err)
	}

	h := startE2EHarness(t)
	defer h.Stop()
	h.SeedCampaign("replay-file-campaign")
	playerID := "user-file"
	h.SeedApprovedPlayer(playerID, "Gale")

	disp := &harnessDispatcher{h: h, playerID: playerID}
	obs := &harnessObserver{fake: h.fake}

	if err := playtest.Replay(entries, disp, obs, playtest.ReplayOptions{
		WaitTimeout: 5 * time.Second,
	}); err != nil {
		t.Fatalf("Replay (%s): %v\nTranscript dump:\n%s", path, err, h.RenderTranscript())
	}
}

// captureRecapTranscript boots a one-shot harness, runs /recap on an
// empty campaign, and returns the resulting JSON-lines transcript as
// the structured form. /recap is chosen because it has no preconditions
// beyond an approved player and produces a single deterministic line.
func captureRecapTranscript(t *testing.T) []playtest.TranscriptEntry {
	t.Helper()

	h := startE2EHarness(t)
	defer h.Stop()
	h.SeedCampaign("replay-capture-campaign")
	playerID := "user-capture"
	h.SeedApprovedPlayer(playerID, "Capture")

	intID := h.PlayerCommand(playerID, "recap")
	resp := h.AssertEphemeralContains(intID, "No encounter found")

	cmd, err := playtest.Parse("/recap")
	if err != nil {
		t.Fatalf("parse /recap: %v", err)
	}
	return []playtest.TranscriptEntry{
		{Direction: playtest.DirectionDispatch, Command: playtest.Format(cmd)},
		{Direction: playtest.DirectionObserved, Content: resp.Content},
	}
}
