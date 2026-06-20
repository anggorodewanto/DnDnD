//go:build e2e
// +build e2e

// Phase 121.3 transcript replay bridge: drives the Phase 120 e2e harness
// from a JSON-lines transcript captured by cmd/playtest-player. Each
// dispatch entry is fed into the production CommandRouter via
// h.PlayerCommand; each observed entry is matched (after normalization)
// against the next outbound transcript entry the harness records.
//
// Round-trip test:    record a synthesized session against the harness,
//
//	replay through a fresh harness, assert green.
//
// Drift test:         mutate one expected line, assert replay fails loudly.
// File-driven test:   `make playtest-replay TRANSCRIPT=path` reads a
//
//	transcript file off disk and replays it. Default is
//	internal/playtest/testdata/sample.jsonl.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ab/dndnd/internal/playtest"
	"github.com/ab/dndnd/internal/testutil/discordfake"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

// harnessDispatcher routes a ParsedCommand into the e2e harness's
// PlayerCommand seam. The player ID is fixed at construction time so a
// transcript can be re-targeted at a different player without rewriting.
type harnessDispatcher struct {
	h           *e2eHarness
	playerID    string
	permissions int64 // Member.Permissions bitmask for every dispatched interaction
}

func (d *harnessDispatcher) Dispatch(cmd playtest.ParsedCommand) error {
	opts := make([]slashOpt, 0, len(cmd.NamedArgs))
	for k, v := range cmd.NamedArgs {
		opts = append(opts, stringOpt(k, v))
	}
	d.h.PlayerCommandWithPermissions(d.playerID, cmd.Name, d.permissions, opts...)
	return nil
}

// harnessObserver consumes the harness's transcript in order. cursor
// marks the next un-consumed entry; Replay calls Wait sequentially so
// no synchronization is needed.
type harnessObserver struct {
	fake   *discordfake.Fake
	cursor int
}

func (o *harnessObserver) Wait(timeout time.Duration) (string, error) {
	// WaitFor scans from the start each call; count matches until we
	// hit the cursor target so each Wait returns the next un-consumed
	// entry rather than re-matching the first one.
	target := o.cursor
	idx := -1
	entry, err := o.fake.WaitFor(func(discordfake.Entry) bool {
		idx++
		return idx == target
	}, timeout)
	if err != nil {
		return "", err
	}
	o.cursor++
	return entry.Content, nil
}

// harnessClicker resolves a button by CustomID prefix from the most recent
// outbound message that carries one, then clicks it as the dispatcher player.
// It satisfies playtest.Clicker.
type harnessClicker struct {
	h        *e2eHarness
	playerID string
}

func (c *harnessClicker) Click(selector string) error {
	customID, ok := latestButtonCustomID(c.h.fake.Transcript(), selector)
	if !ok {
		return fmt.Errorf("no button with custom_id prefix %q in transcript", selector)
	}
	c.h.ClickButton(c.playerID, customID)
	return nil
}

// latestButtonCustomID scans entries newest-first and returns the CustomID of
// the first button whose CustomID has the given prefix.
func latestButtonCustomID(entries []discordfake.Entry, prefix string) (string, bool) {
	for i := len(entries) - 1; i >= 0; i-- {
		for _, comp := range entries[i].Components {
			row, ok := comp.(discordgo.ActionsRow)
			if !ok {
				continue
			}
			for _, child := range row.Components {
				btn, ok := child.(discordgo.Button)
				if !ok {
					continue
				}
				if strings.HasPrefix(btn.CustomID, prefix) {
					return btn.CustomID, true
				}
			}
		}
	}
	return "", false
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

// replayPreconditions declares the seed state a transcript needs before
// replay. It is loaded from a sidecar `<transcript>.preconditions.json`;
// when no sidecar exists TestE2E_ReplayFromFile applies the legacy default
// (a campaign plus one approved player "Gale") so existing transcripts keep
// working without a manifest.
type replayPreconditions struct {
	Campaign string `json:"campaign"`
	Player   string `json:"player"` // dispatcher player ID
	// DispatchAsDM re-targets the dispatcher at the seeded campaign's DM user
	// id instead of Player. Needed for DM/admin commands like /setup whose
	// permission gate checks invoker == campaign DM, since the DM id is random
	// per seed (testutil.NewTestCampaign) and unknowable when authoring the
	// transcript. Overrides Player when true.
	DispatchAsDM bool `json:"dispatchAsDM"`
	// DispatchAsAdmin sets the Administrator permission bit on the dispatched
	// interaction's Member.Permissions. Needed for /setup's auto-create path,
	// which gates new-campaign creation on a server-admin bit
	// (setupInvokerIsAdmin). Orthogonal to DispatchAsDM/Player: it sets
	// permissions, not invoker identity.
	DispatchAsAdmin       bool                 `json:"dispatchAsAdmin"`
	ApprovedPlayers       []approvedPlayerSeed `json:"approvedPlayers"`
	PlaceholderCharacters []string             `json:"placeholderCharacters"`
	Encounter             *encounterSeed       `json:"encounter,omitempty"`
}

type approvedPlayerSeed struct {
	DiscordUserID string `json:"discordUserId"`
	CharacterName string `json:"characterName"`
}

// encounterSeed declares an active combat encounter: an optional map plus
// one or more combatants placed on the grid. Exactly one combatant must be
// the turn holder (the active turn points at it). Combatant.Player must name
// a discord user listed in ApprovedPlayers so its character ID resolves.
type encounterSeed struct {
	WithMap    bool            `json:"withMap"`
	Combatants []combatantSeed `json:"combatants"`
}

type combatantSeed struct {
	Player      string `json:"player"` // discord user ID owning this combatant's character
	DisplayName string `json:"displayName"`
	Col         string `json:"col"`
	Row         int32  `json:"row"`
	TurnHolder  bool   `json:"turnHolder"`
}

// preconditionsPath maps a transcript path to its sidecar manifest:
// `foo.jsonl` -> `foo.preconditions.json`.
func preconditionsPath(transcript string) string {
	return strings.TrimSuffix(transcript, ".jsonl") + ".preconditions.json"
}

// loadPreconditions reads the sidecar manifest if present. It returns
// (nil, nil) when no sidecar exists so the caller applies the default seed.
func loadPreconditions(transcript string) (*replayPreconditions, error) {
	data, err := os.ReadFile(preconditionsPath(transcript))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pc replayPreconditions
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil, err
	}
	return &pc, nil
}

// applyPreconditions seeds the harness from a manifest and returns the
// dispatcher's player ID. With no manifest it applies the legacy default:
// a campaign plus one approved player "Gale", dispatching as "user-file".
func applyPreconditions(t *testing.T, h *e2eHarness, pc *replayPreconditions) (string, int64) {
	t.Helper()
	if pc == nil {
		h.SeedCampaign("replay-file-campaign")
		h.SeedApprovedPlayer("user-file", "Gale")
		return "user-file", 0
	}
	var permissions int64
	if pc.DispatchAsAdmin {
		permissions = int64(discordgo.PermissionAdministrator)
	}
	dispatcher := pc.Player
	if pc.Campaign != "" {
		camp := h.SeedCampaign(pc.Campaign)
		if pc.DispatchAsDM {
			dispatcher = camp.DmUserID
		}
	}
	charByPlayer := make(map[string]uuid.UUID)
	for _, ap := range pc.ApprovedPlayers {
		char, _ := h.SeedApprovedPlayer(ap.DiscordUserID, ap.CharacterName)
		charByPlayer[ap.DiscordUserID] = char.ID
	}
	for _, name := range pc.PlaceholderCharacters {
		h.SeedCharacterOnly(name)
	}
	if pc.Encounter != nil {
		applyEncounter(t, h, pc.Encounter, charByPlayer)
	}
	if dispatcher == "" {
		return "user-file", permissions
	}
	return dispatcher, permissions
}

// applyEncounter seeds an active encounter from the manifest: a shell, an
// optional map, the placed combatants, then promotion to active with the
// turn pointing at the declared turn holder.
func applyEncounter(t *testing.T, h *e2eHarness, enc *encounterSeed, charByPlayer map[string]uuid.UUID) {
	t.Helper()
	encShell := h.SeedEncounterShell()
	if enc.WithMap {
		mp := h.SeedMap()
		h.AttachMapToEncounter(encShell.ID, mp.ID)
	}
	var turnHolder uuid.UUID
	for _, cs := range enc.Combatants {
		charID, ok := charByPlayer[cs.Player]
		if !ok {
			t.Fatalf("preconditions: combatant references unseeded player %q (add it to approvedPlayers)", cs.Player)
		}
		comb := h.SeedCombatant(encShell.ID, charID, cs.DisplayName, cs.Col, cs.Row)
		if cs.TurnHolder {
			turnHolder = comb.ID
		}
	}
	if turnHolder == uuid.Nil {
		t.Fatalf("preconditions: encounter has no combatant marked turnHolder")
	}
	h.PromoteEncounterToActive(encShell.ID, turnHolder)
}

// TestE2E_ReplayFromFile drives the harness from a JSON-lines
// transcript on disk. PLAYTEST_TRANSCRIPT overrides the default path so
// `make playtest-replay TRANSCRIPT=path/to/file.jsonl` plays an
// arbitrary recording. A sidecar `<transcript>.preconditions.json`, if
// present, declares the seed state the transcript needs.
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

	pc, err := loadPreconditions(path)
	if err != nil {
		t.Fatalf("load preconditions for %s: %v", path, err)
	}

	h := startE2EHarness(t)
	defer h.Stop()

	playerID, permissions := applyPreconditions(t, h, pc)

	disp := &harnessDispatcher{h: h, playerID: playerID, permissions: permissions}
	obs := &harnessObserver{fake: h.fake}
	clk := &harnessClicker{h: h, playerID: playerID}

	if err := playtest.Replay(entries, disp, obs, playtest.ReplayOptions{
		WaitTimeout: 5 * time.Second,
		Clicker:     clk,
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
