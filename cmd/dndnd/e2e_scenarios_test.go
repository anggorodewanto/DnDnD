//go:build e2e
// +build e2e

// Phase 120 + 120a reference scenarios. Each test boots a fresh harness
// (in-process dndnd against a clean testcontainers Postgres + a fresh
// discordfake) and drives one player flow end-to-end through the production
// CommandRouter.
//
// Run with:  make e2e   (or  go test -tags e2e ./cmd/dndnd/ -count=1)
//
// Scenarios cover five distinct surfaces of the bot, picked so each one
// exercises a real, currently-implemented Phase 105 handler chain end-to-end:
//
//   - TestE2E_RegistrationScenario  — /register submit + DM-side approval
//   - TestE2E_MovementScenario      — /move <coord> + Confirm button
//   - TestE2E_LootScenario          — /loot pool + Claim button
//   - TestE2E_SaveScenario          — out-of-combat saving throw via /save
//   - TestE2E_RecapEmptyScenario    — /recap when no encounter has yet completed
package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/loot"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/testutil/discordfake"
)

// TestE2E_RegistrationScenario covers the spec-named /register flow:
// 1. DM has seeded a character placeholder ("Aria"). Player runs /register
//    name:"Aria" — the bot responds with a "Registration submitted" ephemeral
//    and persists a pending player_character row.
// 2. The harness then stands in for the DM dashboard and calls
//    registration.Service.Approve directly, then sends the approval DM.
//    Re-reading the row asserts the status flipped to "approved".
// 3. Assert the approval DM was sent to the player.
func TestE2E_RegistrationScenario(t *testing.T) {
	h := startE2EHarness(t)
	defer h.Stop()

	camp := h.SeedCampaign("registration-campaign")

	// Seed a DM-curated placeholder character so /register has something to
	// fuzzy-match. Don't link a player_character — that's what /register does.
	char := h.SeedCharacterOnly("Aria")

	playerID := "user-register"

	interactionID := h.PlayerCommand(playerID, "register", stringOpt("name", "Aria"))
	h.AssertEphemeralContains(interactionID, "Registration submitted")

	// Finding 22: assert that the DM queue channel received a notification
	// (verifies channel output, not just ephemeral responses).
	dmQueueEntry, err := h.fake.WaitFor(func(e discordfake.Entry) bool {
		return e.Kind == discordfake.KindChannelMessage &&
			e.ChannelID == "ch-dmqueue-"+h.guildID &&
			strings.Contains(e.Content, "Aria") &&
			strings.Contains(e.Content, "register")
	}, 5*time.Second)
	if err != nil {
		t.Fatalf("expected DM queue notification for registration: %v\nTranscript:\n%s", err, h.RenderTranscript())
	}
	if !strings.Contains(dmQueueEntry.Content, playerID) {
		t.Fatalf("DM queue notification should mention player ID %s; got: %s", playerID, dmQueueEntry.Content)
	}

	pcs, err := h.queries.ListPlayerCharactersByCampaign(context.Background(), camp.ID)
	if err != nil {
		t.Fatalf("ListPlayerCharactersByCampaign: %v", err)
	}
	var pending refdata.PlayerCharacter
	for _, pc := range pcs {
		if pc.DiscordUserID == playerID && pc.CharacterID == char.ID {
			pending = pc
			break
		}
	}
	if pending.ID == uuid.Nil {
		t.Fatalf("expected pending player_character for %s; got: %+v", playerID, pcs)
	}
	if pending.Status != "pending" {
		t.Fatalf("expected status=pending; got %q", pending.Status)
	}

	approved := h.SeedDMApproval(pending.ID)
	if approved.Status != "approved" {
		t.Fatalf("expected SeedDMApproval to flip status to approved; got %q", approved.Status)
	}

	// Re-read by ID to make sure the row is durably approved.
	got, err := h.queries.GetPlayerCharacter(context.Background(), pending.ID)
	if err != nil {
		t.Fatalf("GetPlayerCharacter: %v", err)
	}
	if got.Status != "approved" {
		t.Fatalf("expected DB status=approved after SeedDMApproval; got %q", got.Status)
	}

	// F-24: assert the approval DM was sent to the player.
	if _, err := h.fake.WaitFor(func(e discordfake.Entry) bool {
		return e.Kind == discordfake.KindDirectMessage &&
			strings.Contains(e.Content, "approved") &&
			strings.Contains(e.Content, "Aria")
	}, 5*time.Second); err != nil {
		t.Fatalf("expected approval DM to player: %v\nTranscript:\n%s", err, h.RenderTranscript())
	}
}

// TestE2E_MovementScenario covers /move <coord> end-to-end:
// 1. Player runs /move coordinate:B1. The bot responds with a confirmation
//    ephemeral that includes a "Confirm" button (custom_id move_confirm:...).
// 2. Harness extracts the custom_id and injects a button-click interaction.
// 3. The handler edits the original message to "Moved to B1." and persists the
//    new combatant position + decremented turn movement.
// 4. Assert #combat-log received the move line.
func TestE2E_MovementScenario(t *testing.T) {
	h := startE2EHarness(t)
	defer h.Stop()

	h.SeedCampaign("movement-campaign")
	playerID := "user-move"

	char, _ := h.SeedApprovedPlayer(playerID, "Mover")
	mp := h.SeedMap()
	encShell := h.SeedEncounterShell()
	h.AttachMapToEncounter(encShell.ID, mp.ID)

	comb := h.SeedCombatant(encShell.ID, char.ID, "Mover", "A", 1)
	_, turn := h.PromoteEncounterToActive(encShell.ID, comb.ID)

	confirmInteractionID := h.PlayerCommand(playerID, "move", stringOpt("coordinate", "B1"))
	// Body is "🏃 Move to B1 — 5ft, 25ft remaining after." — assert on the
	// stable destination label and require a Confirm button in Components.
	first := h.AssertEphemeralContains(confirmInteractionID, "B1")
	if len(first.Components) == 0 {
		t.Fatalf("expected /move response to include component buttons; got 0 rows\nFull transcript:\n%s", h.RenderTranscript())
	}

	confirmCustomID := findButtonCustomID(t, first.Components, "move_confirm:")

	clickInteractionID := uuid.NewString()
	h.fake.InjectInteraction(&discordgo.Interaction{
		ID:        clickInteractionID,
		ChannelID: "ch-cmd-" + h.guildID,
		GuildID:   h.guildID,
		Type:      discordgo.InteractionMessageComponent,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: playerID, Username: "player-" + playerID},
		},
		Data: discordgo.MessageComponentInteractionData{
			CustomID:      confirmCustomID,
			ComponentType: discordgo.ButtonComponent,
		},
	})

	if _, err := h.fake.WaitFor(func(e discordfake.Entry) bool {
		return e.Kind == discordfake.KindInteractionResponse &&
			e.InteractionID == clickInteractionID &&
			strings.Contains(e.Content, "Moved to B1")
	}, 5*time.Second); err != nil {
		t.Fatalf("expected confirm response to include 'Moved to B1': %v\nTranscript:\n%s", err, h.RenderTranscript())
	}

	updatedComb, err := h.queries.GetCombatant(context.Background(), comb.ID)
	if err != nil {
		t.Fatalf("GetCombatant after move: %v", err)
	}
	if updatedComb.PositionCol != "B" || updatedComb.PositionRow != 1 {
		t.Fatalf("expected combatant at B1 after move; got %s%d", updatedComb.PositionCol, updatedComb.PositionRow)
	}

	updatedTurn, err := h.queries.GetTurn(context.Background(), turn.ID)
	if err != nil {
		t.Fatalf("GetTurn after move: %v", err)
	}
	if updatedTurn.MovementRemainingFt >= 30 {
		t.Fatalf("expected movement_remaining_ft < 30 after move; got %d", updatedTurn.MovementRemainingFt)
	}

	// F-24: assert #combat-log received the move line.
	if _, err := h.fake.WaitFor(func(e discordfake.Entry) bool {
		return e.Kind == discordfake.KindChannelMessage &&
			e.ChannelID == "ch-combatlog-"+h.guildID &&
			strings.Contains(e.Content, "Mover") &&
			strings.Contains(e.Content, "moves to B1")
	}, 5*time.Second); err != nil {
		t.Fatalf("expected #combat-log move message: %v\nTranscript:\n%s", err, h.RenderTranscript())
	}
}

// TestE2E_LootScenario covers DM-places-loot → /loot → claim flow:
// 1. Harness completes an encounter and seeds a loot pool with one item.
// 2. Player runs /loot. The bot responds with an embed + a Claim button per
//    unclaimed item. Custom ID is loot_claim:<pool>:<item>:<char>.
// 3. Harness injects a click on that Claim button. The handler persists the
//    claim and updates characters.inventory JSONB.
// 4. Assert #the-story received the loot claim announcement.
func TestE2E_LootScenario(t *testing.T) {
	h := startE2EHarness(t)
	defer h.Stop()

	camp := h.SeedCampaign("loot-campaign")
	playerID := "user-loot"
	char, _ := h.SeedApprovedPlayer(playerID, "Looter")

	enc := h.SeedCompletedEncounter()

	lootSvc := loot.NewService(h.queries)
	pool, err := lootSvc.CreateLootPool(context.Background(), enc.ID)
	if err != nil {
		t.Fatalf("CreateLootPool: %v\ncamp=%s enc=%s", err, camp.ID, enc.ID)
	}
	if _, err := lootSvc.AddItem(context.Background(), pool.Pool.ID, refdata.CreateLootPoolItemParams{
		Name:     "Magic Sword",
		Quantity: 1,
		Type:     "weapon",
	}); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	lootInteractionID := h.PlayerCommand(playerID, "loot")
	first := h.WaitForInteractionResponse(lootInteractionID, 5*time.Second)
	if !first.Ephemeral {
		t.Fatalf("expected /loot response to be ephemeral; got non-ephemeral.\nFull transcript:\n%s", h.RenderTranscript())
	}
	if len(first.Embeds) == 0 {
		t.Fatalf("expected /loot response to include an embed; got 0\nTranscript:\n%s", h.RenderTranscript())
	}

	claimCustomID := findButtonCustomID(t, first.Components, "loot_claim:")

	clickInteractionID := uuid.NewString()
	h.fake.InjectInteraction(&discordgo.Interaction{
		ID:        clickInteractionID,
		ChannelID: "ch-cmd-" + h.guildID,
		GuildID:   h.guildID,
		Type:      discordgo.InteractionMessageComponent,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: playerID, Username: "player-" + playerID},
		},
		Data: discordgo.MessageComponentInteractionData{
			CustomID:      claimCustomID,
			ComponentType: discordgo.ButtonComponent,
		},
	})

	if _, err := h.fake.WaitFor(func(e discordfake.Entry) bool {
		return e.Kind == discordfake.KindInteractionResponse &&
			e.InteractionID == clickInteractionID &&
			strings.Contains(e.Content, "You claimed")
	}, 5*time.Second); err != nil {
		t.Fatalf("expected claim ephemeral 'You claimed': %v\nTranscript:\n%s", err, h.RenderTranscript())
	}

	updatedChar, err := h.queries.GetCharacter(context.Background(), char.ID)
	if err != nil {
		t.Fatalf("GetCharacter after claim: %v", err)
	}
	items, err := character.ParseInventoryItems(updatedChar.Inventory.RawMessage, updatedChar.Inventory.Valid)
	if err != nil {
		t.Fatalf("ParseInventoryItems: %v", err)
	}
	found := false
	for _, item := range items {
		if item.Name == "Magic Sword" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'Magic Sword' in inventory after claim; got %+v", items)
	}

	// F-24: assert #the-story received the loot claim announcement.
	if _, err := h.fake.WaitFor(func(e discordfake.Entry) bool {
		return e.Kind == discordfake.KindChannelMessage &&
			e.ChannelID == "ch-story-"+h.guildID &&
			strings.Contains(e.Content, "Magic Sword")
	}, 5*time.Second); err != nil {
		t.Fatalf("expected #the-story loot claim message: %v\nTranscript:\n%s", err, h.RenderTranscript())
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

// findButtonCustomID walks an action-row component tree and returns the
// custom_id of the first button whose ID begins with prefix. Fails the test
// on no match — scenarios use this to grab dynamic custom IDs (move_confirm,
// loot_claim) without re-implementing them.
func findButtonCustomID(t *testing.T, components []discordgo.MessageComponent, prefix string) string {
	t.Helper()
	for _, comp := range components {
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
				return btn.CustomID
			}
		}
	}
	t.Fatalf("no button with custom_id prefix %q found in components %+v", prefix, components)
	return ""
}
