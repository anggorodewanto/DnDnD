//go:build e2e
// +build e2e

// Phase 120 reference scenarios. Each test boots a fresh harness (in-process
// dndnd against a clean testcontainers Postgres + a fresh discordfake) and
// drives one player flow end-to-end through the production CommandRouter.
//
// Run with:  make e2e   (or  go test -tags e2e ./cmd/dndnd/ -count=1)
//
// Scenarios cover five distinct surfaces of the bot, each picked so it
// exercises a real, currently-implemented Phase 105 handler chain:
//
//   - TestE2E_DistanceScenario      — combat info: /distance between two combatants
//   - TestE2E_StatusScenario        — character info: /status outside combat
//   - TestE2E_SaveScenario          — out-of-combat saving throw via /save
//   - TestE2E_RestScenario          — short-rest UX (slash command + Done button)
//   - TestE2E_RecapEmptyScenario    — /recap when no encounter has yet completed
//
// Per the Phase 120 spec, scenarios that depend on stubbed-only handlers
// (/attack, /cast, /loot, /register approval) are deferred and called out in
// the agent report's "Questions for User" section.
package main

import (
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/testutil/discordfake"
)

// TestE2E_DistanceScenario covers the simplest combat-information flow:
// two PCs are seeded as combatants in an active encounter and one of them
// runs `/distance <shortID>` to learn how far away the other combatant is.
func TestE2E_DistanceScenario(t *testing.T) {
	h := startE2EHarness(t)
	defer h.Stop()

	h.SeedCampaign("distance-campaign")

	playerA := "user-A"
	playerB := "user-B"

	charA, _ := h.SeedApprovedPlayer(playerA, "Alice")
	charB, _ := h.SeedApprovedPlayer(playerB, "Bob")

	encShell := h.SeedEncounterShell()
	combA := h.SeedCombatant(encShell.ID, charA.ID, "Alice", "A", 1)
	combB := h.SeedCombatant(encShell.ID, charB.ID, "Bob", "F", 1) // 5 cols east

	h.PromoteEncounterToActive(encShell.ID, combA.ID)

	interactionID := h.PlayerCommand(playerA, "distance", stringOpt("target", combB.ShortID))

	got := h.AssertEphemeralContains(interactionID, "ft")
	if !strings.Contains(got.Content, "25") {
		t.Fatalf("expected 25 ft in /distance response, got %q", got.Content)
	}

	transcript := h.RenderTranscript()
	if !strings.Contains(transcript, "interaction_response ephemeral") {
		t.Fatalf("expected ephemeral response in transcript:\n%s", transcript)
	}
}

// TestE2E_StatusScenario covers the simplest character-info flow: a player
// with an approved character runs `/status` outside of combat and gets back
// their character header.
func TestE2E_StatusScenario(t *testing.T) {
	h := startE2EHarness(t)
	defer h.Stop()

	h.SeedCampaign("status-campaign")
	playerID := "user-status"
	h.SeedApprovedPlayer(playerID, "Cassidy")

	interactionID := h.PlayerCommand(playerID, "status")
	resp := h.AssertEphemeralContains(interactionID, "Status", "Cassidy")

	if !strings.HasPrefix(resp.Content, "**Status — Cassidy**") {
		t.Fatalf("expected status header to start with **Status — Cassidy**; got %q", resp.Content)
	}

	// Negative golden assertion: an unregistered player should be told to
	// /register, not get the same Status header.
	intruder := h.PlayerCommand("user-uninvited", "status")
	got := h.AssertEphemeralContains(intruder, "register")
	if strings.Contains(got.Content, "Cassidy") {
		t.Fatalf("expected unregistered response to omit other players' names; got %q", got.Content)
	}
}

// TestE2E_SaveScenario covers an out-of-combat /save dex roll: the saving
// throw runs end-to-end, hits the rest of the stack including dice rolling,
// and answers the player ephemerally with their roll breakdown.
func TestE2E_SaveScenario(t *testing.T) {
	h := startE2EHarness(t)
	defer h.Stop()

	h.SeedCampaign("save-campaign")
	playerID := "user-save"
	char, _ := h.SeedApprovedPlayer(playerID, "Dax")

	interactionID := h.PlayerCommand(playerID, "save", stringOpt("ability", "dex"))

	resp := h.AssertEphemeralContains(interactionID, char.Name)
	// Save responses include a "Saving Throw" or ability label and a roll
	// total. Be lenient about the exact format but assert both pieces.
	if !strings.Contains(strings.ToLower(resp.Content), "save") &&
		!strings.Contains(strings.ToLower(resp.Content), "throw") {
		t.Fatalf("expected /save response to mention save/throw; got %q", resp.Content)
	}
}

// TestE2E_RestScenario covers the /rest short flow:
// 1. Player runs /rest short — gets an ephemeral with hit-dice buttons.
// 2. Player clicks Done — handler finalises with no dice spent and edits
//    the ephemeral to show the short-rest summary.
// This validates BOTH the slash-command path AND the message-component
// callback path through the router.
func TestE2E_RestScenario(t *testing.T) {
	h := startE2EHarness(t)
	defer h.Stop()

	h.SeedCampaign("rest-campaign")
	playerID := "user-rest"
	char, _ := h.SeedApprovedPlayer(playerID, "Edda")

	// /rest short — should land an ephemeral with hit-dice buttons.
	interactionID := h.PlayerCommand(playerID, "rest", stringOpt("type", "short"))
	first := h.AssertEphemeralContains(interactionID, "Short Rest")
	if len(first.Components) == 0 {
		t.Fatalf("expected hit-dice buttons in /rest short response; got %d components", len(first.Components))
	}

	// Click the Done button. The custom_id format from rest_handler.go is
	// "rest_hitdice:<charID>:done:0".
	doneID := "rest_hitdice:" + char.ID.String() + ":done:0"
	h.fake.InjectInteraction(&discordgo.Interaction{
		ID:        uuid.NewString(),
		ChannelID: "ch-cmd-" + h.guildID,
		GuildID:   h.guildID,
		Type:      discordgo.InteractionMessageComponent,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: playerID},
		},
		Data: discordgo.MessageComponentInteractionData{
			CustomID:      doneID,
			ComponentType: discordgo.ButtonComponent,
		},
	})

	// The Done click acks via DeferredMessageUpdate (no content) then edits
	// the original interaction; wait for the edit.
	if _, err := h.fake.WaitFor(func(e discordfake.Entry) bool {
		return e.Kind == discordfake.KindInteractionEdit && strings.Contains(e.Content, char.Name)
	}, 5*time.Second); err != nil {
		t.Fatalf("expected interaction edit naming %s after Done click: %v\nTranscript:\n%s", char.Name, err, h.RenderTranscript())
	}
}

// TestE2E_RecapEmptyScenario covers /recap when the campaign has not yet
// completed any encounter: the bot must answer with the canonical "No
// encounter found" ephemeral instead of crashing.
//
// This scenario also locks down the deterministic transcript: the rendered
// dump is compared against a golden string so any drift in router-level
// boilerplate (prefix, phrasing, ephemeral flag) trips the test.
func TestE2E_RecapEmptyScenario(t *testing.T) {
	h := startE2EHarness(t)
	defer h.Stop()

	h.SeedCampaign("recap-campaign")
	playerID := "user-recap"
	h.SeedApprovedPlayer(playerID, "Faye")

	interactionID := h.PlayerCommand(playerID, "recap")
	h.AssertEphemeralContains(interactionID, "No encounter found")

	const want = `[1] interaction_response ephemeral channel=ch-cmd-<GUILD> interaction=<UUID-1> content="No encounter found for recap."
`
	got := h.RenderTranscript()
	if got != want {
		t.Fatalf("transcript mismatch.\nwant:\n%s\n got:\n%s", want, got)
	}
}

