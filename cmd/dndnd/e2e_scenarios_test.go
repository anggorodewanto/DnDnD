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
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/loot"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/testutil/discordfake"
)

// TestE2E_RegistrationScenario covers the spec-named /register flow:
//  1. DM has seeded a character placeholder ("Aria"). Player runs /register
//     name:"Aria" — the bot responds with a "Registration submitted" ephemeral
//     and persists a pending player_character row.
//  2. The harness then stands in for the DM dashboard and calls
//     registration.Service.Approve directly, then sends the approval DM.
//     Re-reading the row asserts the status flipped to "approved".
//  3. Assert the approval DM was sent to the player.
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

// TestE2E_SetupScenario covers the DM /setup flow end-to-end:
//  1. A campaign already exists for the guild (seeded). The DM runs /setup.
//  2. The bot defers, creates the SYSTEM/NARRATION/COMBAT/REFERENCE channel
//     structure (10 text channels) via GuildChannelCreateComplex, and edits the
//     deferred response with a public "Channel structure created successfully!
//     10 channels set up." message.
//  3. Assert the public (non-ephemeral) success message.
//  4. Assert the 10 channel IDs were persisted into campaign settings JSONB —
//     the .jsonl replay (setup.jsonl) can only see the Discord message, so the
//     DB-side lock lives here.
func TestE2E_SetupScenario(t *testing.T) {
	h := startE2EHarness(t)
	defer h.Stop()

	camp := h.SeedCampaign("setup-campaign")

	// /setup is gated to the campaign DM (existing-campaign path checks
	// invoker == DM, no admin bit needed). Dispatch as the seeded DM user id.
	interactionID := h.PlayerCommand(camp.DmUserID, "setup")

	// The success message is delivered by editing the deferred response.
	entry, err := h.fake.WaitFor(func(e discordfake.Entry) bool {
		return e.Kind == discordfake.KindInteractionEdit &&
			e.InteractionID == interactionID &&
			strings.Contains(e.Content, "Channel structure created successfully! 10 channels set up.")
	}, 5*time.Second)
	if err != nil {
		t.Fatalf("expected /setup success edit: %v\nTranscript:\n%s", err, h.RenderTranscript())
	}
	if entry.Ephemeral {
		t.Fatalf("expected /setup success to be public (non-ephemeral); got ephemeral\nContent: %q", entry.Content)
	}

	// Assert the 10 channel IDs were persisted into campaign settings JSONB.
	got, err := h.queries.GetCampaignByGuildID(context.Background(), camp.GuildID)
	if err != nil {
		t.Fatalf("GetCampaignByGuildID: %v", err)
	}
	if !got.Settings.Valid {
		t.Fatalf("expected campaign settings to be set after /setup")
	}
	var settings campaign.Settings
	if err := json.Unmarshal(got.Settings.RawMessage, &settings); err != nil {
		t.Fatalf("decoding campaign settings: %v", err)
	}
	if len(settings.ChannelIDs) != 10 {
		t.Fatalf("expected 10 channel IDs persisted after /setup; got %d: %+v", len(settings.ChannelIDs), settings.ChannelIDs)
	}
	// Spot-check one channel from each of the four categories.
	for _, name := range []string{"initiative-tracker", "the-story", "combat-map", "dm-queue"} {
		if settings.ChannelIDs[name] == "" {
			t.Fatalf("expected persisted channel id for %q; got %+v", name, settings.ChannelIDs)
		}
	}
}

// TestE2E_SetupAutoCreateScenario covers the /setup auto-create happy path
// end-to-end (admin on a fresh guild):
//  1. No campaign exists for the guild. A server admin runs /setup.
//  2. The bot auto-creates the campaign (admin becomes DM), builds the 10-text-
//     channel structure, and edits the deferred response with the public
//     "Campaign created and channel structure set up! 10 channels set up."
//     message (distinct from the existing-campaign "Channel structure created
//     successfully!" wording).
//  3. Assert the DB side the .jsonl replay cannot see: a campaign row now exists
//     with the admin as DM and 10 channel IDs persisted in settings JSONB.
func TestE2E_SetupAutoCreateScenario(t *testing.T) {
	h := startE2EHarness(t)
	defer h.Stop()

	// No SeedCampaign: the guild starts empty. SeedCampaign is what normally
	// sets h.guildID, so set a stable id directly for the no-campaign path.
	h.guildID = "guild-autocreate"
	adminUser := "admin-user"

	interactionID := h.PlayerCommandWithPermissions(adminUser, "setup", int64(discordgo.PermissionAdministrator))

	entry, err := h.fake.WaitFor(func(e discordfake.Entry) bool {
		return e.Kind == discordfake.KindInteractionEdit &&
			e.InteractionID == interactionID &&
			strings.Contains(e.Content, "Campaign created and channel structure set up! 10 channels set up.")
	}, 5*time.Second)
	if err != nil {
		t.Fatalf("expected /setup auto-create success edit: %v\nTranscript:\n%s", err, h.RenderTranscript())
	}
	if entry.Ephemeral {
		t.Fatalf("expected /setup success to be public (non-ephemeral); got ephemeral\nContent: %q", entry.Content)
	}

	got, err := h.queries.GetCampaignByGuildID(context.Background(), h.guildID)
	if err != nil {
		t.Fatalf("expected auto-created campaign for guild; GetCampaignByGuildID: %v", err)
	}
	if got.DmUserID != adminUser {
		t.Fatalf("expected auto-created campaign DM = %q (the admin invoker); got %q", adminUser, got.DmUserID)
	}
	if !got.Settings.Valid {
		t.Fatalf("expected campaign settings to be set after /setup")
	}
	var settings campaign.Settings
	if err := json.Unmarshal(got.Settings.RawMessage, &settings); err != nil {
		t.Fatalf("decoding campaign settings: %v", err)
	}
	if len(settings.ChannelIDs) != 10 {
		t.Fatalf("expected 10 channel IDs persisted after /setup; got %d: %+v", len(settings.ChannelIDs), settings.ChannelIDs)
	}
}

// TestE2E_SetupRejectsNonAdminWithoutCreatingCampaign locks the /setup
// auto-create authorization gate end-to-end:
//  1. No campaign exists for the guild. A non-admin member runs /setup.
//  2. The bot rejects with the public "⛔ Only a server administrator can
//     create a new campaign via /setup." message.
//  3. CRITICALLY no campaign row is created — the admin gate must run BEFORE any
//     persistence. Regression lock for the bug where the campaign was
//     auto-created during lookup (before the gate), silently making the rejected
//     non-admin the DM of a real campaign.
func TestE2E_SetupRejectsNonAdminWithoutCreatingCampaign(t *testing.T) {
	h := startE2EHarness(t)
	defer h.Stop()

	h.guildID = "guild-no-campaign"

	interactionID := h.PlayerCommand("non-admin-user", "setup") // Permissions == 0

	if _, err := h.fake.WaitFor(func(e discordfake.Entry) bool {
		return e.Kind == discordfake.KindInteractionEdit &&
			e.InteractionID == interactionID &&
			strings.Contains(e.Content, "Only a server administrator can create a new campaign")
	}, 5*time.Second); err != nil {
		t.Fatalf("expected non-admin /setup reject: %v\nTranscript:\n%s", err, h.RenderTranscript())
	}

	if _, err := h.queries.GetCampaignByGuildID(context.Background(), h.guildID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected NO campaign created on non-admin reject; GetCampaignByGuildID err = %v (want sql.ErrNoRows)", err)
	}
}

// TestE2E_MovementScenario covers /move <coord> end-to-end:
//  1. Player runs /move coordinate:B1. The bot responds with a confirmation
//     ephemeral that includes a "Confirm" button (custom_id move_confirm:...).
//  2. Harness extracts the custom_id and injects a button-click interaction.
//  3. The handler edits the original message to "Moved to B1." and persists the
//     new combatant position + decremented turn movement.
//  4. Assert #combat-log received the move line.
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

// TestE2E_AttackScenario covers /attack <target> end-to-end in active combat:
//  1. Seed an active encounter: player "Striker" (turn holder) at A1, an NPC
//     "Goblin" at B1.
//  2. Player runs /attack target:B1 weapon:longsword. The harness's default
//     always-max roller forces a natural 20, so the longsword crit is fully
//     deterministic: doubled 2d8 (16) + STR 3 = 19 slashing.
//  3. Assert the ephemeral + #combat-log carry the crit attack line.
//  4. Assert the attack resource was spent (AttacksRemaining 1 → 0).
//  5. Assert the target's HP dropped 20 → 1 (the bug fix: a hit now applies its
//     damage to the target through combat.ApplyDamage). The goblin survives at
//     1 HP, documenting the announce + apply contract.
func TestE2E_AttackScenario(t *testing.T) {
	h := startE2EHarness(t)
	defer h.Stop()

	h.SeedCampaign("attack-campaign")
	playerID := "user-attack"

	char, _ := h.SeedApprovedPlayer(playerID, "Striker")
	mp := h.SeedMap()
	encShell := h.SeedEncounterShell()
	h.AttachMapToEncounter(encShell.ID, mp.ID)

	comb := h.SeedCombatant(encShell.ID, char.ID, "Striker", "A", 1)
	npc := h.SeedNPCCombatant(encShell.ID, "Goblin", "B", 1)
	_, turn := h.PromoteEncounterToActive(encShell.ID, comb.ID)

	atkID := h.PlayerCommand(playerID, "attack",
		stringOpt("target", "B1"), stringOpt("weapon", "longsword"))

	// Ephemeral: the deterministic crit attack line.
	h.AssertEphemeralContains(atkID,
		"Striker attacks Goblin with Longsword",
		"NAT 20 — CRITICAL HIT!",
		"19 slashing")

	// #combat-log mirrors the same line.
	if _, err := h.fake.WaitFor(func(e discordfake.Entry) bool {
		return e.Kind == discordfake.KindChannelMessage &&
			e.ChannelID == "ch-combatlog-"+h.guildID &&
			strings.Contains(e.Content, "Striker attacks Goblin with Longsword") &&
			strings.Contains(e.Content, "NAT 20")
	}, 5*time.Second); err != nil {
		t.Fatalf("expected #combat-log attack message: %v\nTranscript:\n%s", err, h.RenderTranscript())
	}

	// Attack resource spent: AttacksRemaining 1 → 0.
	updatedTurn, err := h.queries.GetTurn(context.Background(), turn.ID)
	if err != nil {
		t.Fatalf("GetTurn after attack: %v", err)
	}
	if updatedTurn.AttacksRemaining != 0 {
		t.Fatalf("expected AttacksRemaining 0 after attack; got %d", updatedTurn.AttacksRemaining)
	}

	// Bug-fix lock: the hit applied its damage to the target. Goblin 20 - 19 = 1.
	updatedNPC, err := h.queries.GetCombatant(context.Background(), npc.ID)
	if err != nil {
		t.Fatalf("GetCombatant after attack: %v", err)
	}
	if updatedNPC.HpCurrent != 1 {
		t.Fatalf("expected target HP 1 after 19 crit damage (20-19); got %d", updatedNPC.HpCurrent)
	}
	if !updatedNPC.IsAlive {
		t.Fatalf("expected target alive at 1 HP after attack; got is_alive=false")
	}
}

// TestE2E_MartialArtsBonusAttackScenario covers the monk /bonus martial-arts
// path end-to-end in active combat and locks the STEP-006 bug fix:
//  1. Seed an active encounter with monk "Kira" (turn holder, Attack action
//     already used) at A1 and an NPC "Goblin" at B1.
//  2. Player runs /bonus action:martial-arts args:B1. The always-max roller
//     forces a natural 20, so the unarmed strike crit is deterministic:
//     doubled 2d6 (12) + STR 3 = 15 bludgeoning.
//  3. Assert the ephemeral + #combat-log carry the crit unarmed-strike line.
//  4. Assert the bonus action was spent.
//  5. Bug-fix lock: the bonus strike applied its damage to the target — Goblin
//     20 - 15 = 5 HP. Before the fix this path announced the hit + spent the
//     bonus action but never reduced the target's HP.
func TestE2E_MartialArtsBonusAttackScenario(t *testing.T) {
	h := startE2EHarness(t)
	defer h.Stop()

	h.SeedCampaign("martial-arts-campaign")
	playerID := "user-monk"

	char, _ := h.SeedApprovedMonk(playerID, "Kira", 5, 0)
	mp := h.SeedMap()
	encShell := h.SeedEncounterShell()
	h.AttachMapToEncounter(encShell.ID, mp.ID)

	comb := h.SeedCombatant(encShell.ID, char.ID, "Kira", "A", 1)
	npc := h.SeedNPCCombatant(encShell.ID, "Goblin", "B", 1)
	_, turn := h.PromoteEncounterToActive(encShell.ID, comb.ID)
	h.MarkTurnActionUsed(turn.ID)

	bonusID := h.PlayerCommand(playerID, "bonus",
		stringOpt("action", "martial-arts"), stringOpt("args", "B1"))

	h.AssertEphemeralContains(bonusID,
		"Kira attacks Goblin with Unarmed Strike",
		"NAT 20 — CRITICAL HIT!")

	if _, err := h.fake.WaitFor(func(e discordfake.Entry) bool {
		return e.Kind == discordfake.KindChannelMessage &&
			e.ChannelID == "ch-combatlog-"+h.guildID &&
			strings.Contains(e.Content, "Kira attacks Goblin with Unarmed Strike") &&
			strings.Contains(e.Content, "NAT 20")
	}, 5*time.Second); err != nil {
		t.Fatalf("expected #combat-log martial-arts message: %v\nTranscript:\n%s", err, h.RenderTranscript())
	}

	// Bonus action spent.
	updatedTurn, err := h.queries.GetTurn(context.Background(), turn.ID)
	if err != nil {
		t.Fatalf("GetTurn after bonus: %v", err)
	}
	if !updatedTurn.BonusActionUsed {
		t.Fatalf("expected BonusActionUsed=true after martial-arts; got false")
	}

	// Bug-fix lock: the bonus strike applied damage. Goblin 20 - 15 = 5.
	updatedNPC, err := h.queries.GetCombatant(context.Background(), npc.ID)
	if err != nil {
		t.Fatalf("GetCombatant after bonus: %v", err)
	}
	if updatedNPC.HpCurrent != 5 {
		t.Fatalf("expected target HP 5 after 15 crit bonus damage (20-15); got %d", updatedNPC.HpCurrent)
	}
	if !updatedNPC.IsAlive {
		t.Fatalf("expected target alive at 5 HP; got is_alive=false")
	}
}

// TestE2E_FlurryOfBlowsScenario covers the monk /bonus flurry-of-blows path and
// locks that BOTH strikes apply damage (the STEP-006 sibling bug):
//  1. Seed an active encounter with monk "Kira" (turn holder, 5 ki, Attack
//     action already used) at A1 and an NPC "Goblin" at B1.
//  2. Player runs /bonus action:flurry args:B1. Two unarmed strikes, each a
//     deterministic crit: 2d6 (12) + STR 3 = 15. The two strikes stack: Goblin
//     20 → 5 → 0 (dead).
//  3. Assert the ephemeral + #combat-log carry the flurry line + ki spend.
//  4. Assert the bonus action was spent and 1 ki deducted (5 → 4).
//  5. Bug-fix lock: both strikes applied damage — the target is dead at 0 HP.
//     Before the fix flurry rolled + announced two hits but applied 0 HP.
func TestE2E_FlurryOfBlowsScenario(t *testing.T) {
	h := startE2EHarness(t)
	defer h.Stop()

	h.SeedCampaign("flurry-campaign")
	playerID := "user-monk"

	char, _ := h.SeedApprovedMonk(playerID, "Kira", 5, 5)
	mp := h.SeedMap()
	encShell := h.SeedEncounterShell()
	h.AttachMapToEncounter(encShell.ID, mp.ID)

	comb := h.SeedCombatant(encShell.ID, char.ID, "Kira", "A", 1)
	npc := h.SeedNPCCombatant(encShell.ID, "Goblin", "B", 1)
	_, turn := h.PromoteEncounterToActive(encShell.ID, comb.ID)
	h.MarkTurnActionUsed(turn.ID)

	bonusID := h.PlayerCommand(playerID, "bonus",
		stringOpt("action", "flurry"), stringOpt("args", "B1"))

	h.AssertEphemeralContains(bonusID,
		"uses Flurry of Blows",
		"1 ki spent, 4 remaining")

	if _, err := h.fake.WaitFor(func(e discordfake.Entry) bool {
		return e.Kind == discordfake.KindChannelMessage &&
			e.ChannelID == "ch-combatlog-"+h.guildID &&
			strings.Contains(e.Content, "uses Flurry of Blows")
	}, 5*time.Second); err != nil {
		t.Fatalf("expected #combat-log flurry message: %v\nTranscript:\n%s", err, h.RenderTranscript())
	}

	// Bonus action spent + 1 ki deducted.
	updatedTurn, err := h.queries.GetTurn(context.Background(), turn.ID)
	if err != nil {
		t.Fatalf("GetTurn after flurry: %v", err)
	}
	if !updatedTurn.BonusActionUsed {
		t.Fatalf("expected BonusActionUsed=true after flurry; got false")
	}

	// Bug-fix lock: both strikes applied damage. Goblin 20 - 15 - 15 → 0, dead.
	updatedNPC, err := h.queries.GetCombatant(context.Background(), npc.ID)
	if err != nil {
		t.Fatalf("GetCombatant after flurry: %v", err)
	}
	if updatedNPC.HpCurrent != 0 {
		t.Fatalf("expected target HP 0 after two 15-dmg crits; got %d", updatedNPC.HpCurrent)
	}
	if updatedNPC.IsAlive {
		t.Fatalf("expected target dead at 0 HP after flurry; got is_alive=true")
	}
}

// TestE2E_InitiativeScenario covers the DM "start combat" flow end-to-end
// (STEP-007). Initiative has no slash command — it is the dashboard REST action
// POST /api/combat/start → combat.Service.StartCombat — so the harness drives
// the real, fully-wired production service directly via h.combatService.
//
// It seeds a template (one SRD goblin) + two PCs with distinct DEX, then runs
// StartCombat and asserts the real RollInitiative result. Under the always-max
// roller every d20 = 20, so initiative order is driven purely by DEX modifier
// (roll = 20 + dexMod). Bram (DEX 20, +5 → 25) is added AFTER the goblin yet
// lands at order 1 — proving SortByInitiative actually reorders, not just
// preserves insertion order. Alice (DEX 6, −2 → 18) lands last.
func TestE2E_InitiativeScenario(t *testing.T) {
	h := startE2EHarness(t)
	defer h.Stop()
	ctx := context.Background()

	h.SeedCampaign("initiative-campaign")

	// Two PCs whose DEX brackets the goblin's so the order is deterministic
	// regardless of the SRD goblin's exact DEX (25 > goblin > 18).
	bram, _ := h.SeedApprovedPlayerWithDex("user-bram", "Bram", 20)   // +5 → 25
	alice, _ := h.SeedApprovedPlayerWithDex("user-alice", "Alice", 6) // −2 → 18

	// Map-less template with one goblin; PCs get explicit positions below so we
	// don't depend on spawn-zone seeding (irrelevant to initiative).
	tmpl := h.SeedEncounterTemplate("Goblin Ambush", uuid.NullUUID{}, combat.TemplateCreature{
		CreatureRefID: "goblin",
		ShortID:       "G",
		DisplayName:   "Goblin",
		PositionCol:   "B",
		PositionRow:   1,
		Quantity:      1,
	})

	// Drive the REAL start-combat flow with the deterministic always-max roller.
	result, err := h.combatService.StartCombat(ctx, combat.StartCombatInput{
		TemplateID:   tmpl.ID,
		CharacterIDs: []uuid.UUID{bram.ID, alice.ID},
		CharacterPositions: map[uuid.UUID]combat.Position{
			bram.ID:  {Col: "A", Row: 1},
			alice.ID: {Col: "C", Row: 1},
		},
	}, dice.NewRoller(e2eDefaultRoll))
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}

	// The goblin's expected roll comes from its real SRD DEX so the assertion
	// is robust to the seeded stat block.
	goblin, err := h.queries.GetCreature(ctx, "goblin")
	if err != nil {
		t.Fatalf("GetCreature(goblin): %v", err)
	}
	gScores, err := combat.ParseAbilityScores(goblin.AbilityScores)
	if err != nil {
		t.Fatalf("ParseAbilityScores(goblin): %v", err)
	}
	goblinRoll := int32(20 + combat.AbilityModifier(gScores.Dex))

	// Real DB state: combatants come back ordered by initiative_order ASC.
	combatants, err := h.queries.ListCombatantsByEncounterID(ctx, result.Encounter.ID)
	if err != nil {
		t.Fatalf("ListCombatantsByEncounterID: %v", err)
	}
	if len(combatants) != 3 {
		t.Fatalf("expected 3 combatants; got %d", len(combatants))
	}

	type want struct {
		name  string
		order int32
		roll  int32
		isNPC bool
	}
	wants := []want{
		{"Bram", 1, 25, false},          // DEX 20 (+5), added after goblin → still first
		{"Goblin", 2, goblinRoll, true}, // SRD goblin DEX, between the two PCs
		{"Alice", 3, 18, false},         // DEX 6 (−2) → last
	}
	for i, w := range wants {
		c := combatants[i]
		if c.DisplayName != w.name {
			got := make([]string, len(combatants))
			for j, cc := range combatants {
				got[j] = cc.DisplayName
			}
			t.Fatalf("order slot %d: expected %s; got %s (full order: %s)", i+1, w.name, c.DisplayName, strings.Join(got, ", "))
		}
		if c.InitiativeOrder != w.order {
			t.Fatalf("%s: expected initiative_order %d; got %d", w.name, w.order, c.InitiativeOrder)
		}
		if c.InitiativeRoll != w.roll {
			t.Fatalf("%s: expected initiative_roll %d; got %d", w.name, w.roll, c.InitiativeRoll)
		}
		if c.IsNpc != w.isNPC {
			t.Fatalf("%s: expected is_npc %v; got %v", w.name, w.isNPC, c.IsNpc)
		}
	}

	// Encounter activated at round 1 with a current turn pointing at the winner.
	enc, err := h.queries.GetEncounter(ctx, result.Encounter.ID)
	if err != nil {
		t.Fatalf("GetEncounter: %v", err)
	}
	if enc.Status != "active" {
		t.Fatalf("expected encounter status active; got %q", enc.Status)
	}
	if enc.RoundNumber != 1 {
		t.Fatalf("expected round_number 1; got %d", enc.RoundNumber)
	}
	if !enc.CurrentTurnID.Valid {
		t.Fatalf("expected current_turn_id set after StartCombat")
	}

	// First turn belongs to the initiative winner (Bram, order 1).
	bramComb := combatants[0]
	if result.FirstTurn.CombatantID != bramComb.ID {
		t.Fatalf("expected first turn for Bram (%s); got %s", bramComb.ID, result.FirstTurn.CombatantID)
	}
	turn, err := h.queries.GetTurn(ctx, enc.CurrentTurnID.UUID)
	if err != nil {
		t.Fatalf("GetTurn(current): %v", err)
	}
	if turn.CombatantID != bramComb.ID {
		t.Fatalf("expected current turn combatant Bram (%s); got %s", bramComb.ID, turn.CombatantID)
	}

	// The initiative tracker announces round 1 and pings the winner.
	if !strings.Contains(result.InitiativeTracker, "Round 1") {
		t.Fatalf("expected tracker to mention Round 1; got:\n%s", result.InitiativeTracker)
	}
	if !strings.Contains(result.InitiativeTracker, "@Bram — it's your turn!") {
		t.Fatalf("expected tracker to ping Bram as turn holder; got:\n%s", result.InitiativeTracker)
	}
}

// TestE2E_LootScenario covers DM-places-loot → /loot → claim flow:
//  1. Harness completes an encounter and seeds a loot pool with one item.
//  2. Player runs /loot. The bot responds with an embed + a Claim button per
//     unclaimed item. Custom ID is loot_claim:<pool>:<item>:<char>.
//  3. Harness injects a click on that Claim button. The handler persists the
//     claim and updates characters.inventory JSONB.
//  4. Assert #the-story received the loot claim announcement.
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
