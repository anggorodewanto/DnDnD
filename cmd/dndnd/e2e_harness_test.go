//go:build e2e
// +build e2e

// This file builds only under the `e2e` tag (`go test -tags e2e ./cmd/dndnd/...`).
// It implements the Phase 120 black-box harness: a goroutine-safe wrapper that
// boots the production run() against a testcontainers Postgres and a
// discordfake.Fake session, exposes Discord-style scenario helpers, and
// returns a deterministic transcript so each scenario can assert on the
// exact bot output the players would see.
//
// The harness lives in the cmd/dndnd package (not internal/e2etest) because
// runWithOptions is unexported. Keeping the seam unexported preserves the
// production API surface while still letting the harness drive every wiring
// path that real `dndnd` startup goes through.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/database"
	"github.com/ab/dndnd/internal/discord"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/registration"
	"github.com/ab/dndnd/internal/testutil"
	"github.com/ab/dndnd/internal/testutil/discordfake"
)

// e2eHarness boots one in-process dndnd binary against a fresh testcontainers
// Postgres + a fresh discordfake.Fake. The same harness instance is reused
// across scenarios in a single test run via TestMain.
type e2eHarness struct {
	t          *testing.T
	db         *sql.DB
	queries    *refdata.Queries
	fake       *discordfake.Fake
	cancel     context.CancelFunc
	doneCh     chan error
	guildID    string
	campaignID uuid.UUID
}

// startE2EHarness constructs and boots a fresh harness. Callers must defer
// h.Stop(). Failure is reported via t.Fatalf.
func startE2EHarness(t *testing.T) *e2eHarness {
	t.Helper()

	connStr := testutil.NewTestDBConnString(t)

	// Open a side-channel DB connection so scenarios can seed and assert on
	// state without going through the bot.
	db, err := database.Connect(connStr)
	if err != nil {
		t.Fatalf("e2e harness: connect to database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	t.Setenv("DATABASE_URL", connStr)
	t.Setenv("DISCORD_BOT_TOKEN", "")

	addr := getFreePort(t)

	fake := discordfake.New()

	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan error, 1)
	routerReady := make(chan struct{})

	go func() {
		doneCh <- runWithOptions(ctx, io.Discard, addr,
			withDiscordSession(fake),
			withCommandRouterReady(func(r *discord.CommandRouter) {
				// Hook the router into the fake so InjectInteraction calls
				// hit the production handler chain.
				fake.SetInteractionHandler(r.Handle)
				close(routerReady)
			}),
		)
	}()

	// Wait for HTTP server to come up so we know wiring finished.
	waitForHTTP(t, addr)

	select {
	case <-routerReady:
	case <-time.After(5 * time.Second):
		cancel()
		<-doneCh
		t.Fatal("e2e harness: command router not ready within 5s")
	}

	// queries is the same *refdata.Queries the bot is using internally.
	// Constructed against the side-channel connection so seeding helpers
	// share the same schema view.
	return &e2eHarness{
		t:       t,
		db:      db,
		queries: refdata.New(db),
		fake:    fake,
		cancel:  cancel,
		doneCh:  doneCh,
	}
}

// Stop signals the in-process server to shut down and waits for it.
func (h *e2eHarness) Stop() {
	h.cancel()
	select {
	case <-h.doneCh:
	case <-time.After(10 * time.Second):
		h.t.Fatal("e2e harness: server did not shut down within 10s")
	}
}

// SeedCampaign creates a campaign + map + DM user in the harness DB and
// stores the IDs on the harness for later helper calls. Idempotent per
// harness instance.
func (h *e2eHarness) SeedCampaign(name string) refdata.Campaign {
	h.t.Helper()
	camp := testutil.NewTestCampaign(h.t, h.queries, "g")
	h.guildID = camp.GuildID
	h.campaignID = camp.ID
	// Register a #the-story channel so any narration/announcer paths that
	// look it up find it.
	h.fake.AddGuildChannel(h.guildID, &discordgo.Channel{ID: "ch-story-" + h.guildID, Name: "the-story"})
	h.fake.AddGuildChannel(h.guildID, &discordgo.Channel{ID: "ch-yourturn-" + h.guildID, Name: "your-turn"})
	h.fake.AddGuildChannel(h.guildID, &discordgo.Channel{ID: "ch-dmqueue-" + h.guildID, Name: "dm-queue"})
	return camp
}

// SeedApprovedPlayer creates a character + an approved player_character row
// linking it to discordUserID. Returns both records.
func (h *e2eHarness) SeedApprovedPlayer(discordUserID, charName string) (refdata.Character, refdata.PlayerCharacter) {
	h.t.Helper()
	char := testutil.NewTestCharacter(h.t, h.queries, h.campaignID, charName, 3)
	pc := testutil.NewTestPlayerCharacter(h.t, h.queries, h.campaignID, char.ID, discordUserID)
	return char, pc
}

// SeedCharacterOnly creates a DM-curated character with no player_character
// row. /register can then claim it by name. Used by the registration scenario
// to drive the real fuzzy-match flow.
func (h *e2eHarness) SeedCharacterOnly(charName string) refdata.Character {
	h.t.Helper()
	return testutil.NewTestCharacter(h.t, h.queries, h.campaignID, charName, 3)
}

// SeedDMApproval bypasses the dashboard and approves a pending player
// character via registration.Service.Approve directly. Per Phase 120a
// clarification (1) the welcome-DM is not asserted; the harness's job is to
// flip status to "approved" and return the updated row so the scenario can
// re-read it.
func (h *e2eHarness) SeedDMApproval(playerCharID uuid.UUID) refdata.PlayerCharacter {
	h.t.Helper()
	svc := registration.NewService(h.queries)
	pc, err := svc.Approve(context.Background(), playerCharID)
	if err != nil {
		h.t.Fatalf("SeedDMApproval(%s): %v", playerCharID, err)
	}
	return *pc
}

// SeedMap inserts a 10x10 Tiled map with one short wall on the campaign and
// returns the persisted row. The Tiled JSON is the minimum shape that
// renderer.ParseTiledJSON accepts with a "terrain" tilelayer + a "walls"
// objectgroup so the production /move pathfinder can build a Grid.
func (h *e2eHarness) SeedMap() refdata.Map {
	h.t.Helper()
	const w, hght = 10, 10
	terrain := make([]int, w*hght)
	tiledJSON, err := json.Marshal(map[string]any{
		"version":      "1.10",
		"tiledversion": "1.10.0",
		"type":         "map",
		"orientation":  "orthogonal",
		"width":        w,
		"height":       hght,
		"tilewidth":    48,
		"tileheight":   48,
		"layers": []map[string]any{
			{
				"name":   "terrain",
				"type":   "tilelayer",
				"width":  w,
				"height": hght,
				"data":   terrain,
			},
			{
				"name": "walls",
				"type": "objectgroup",
				"objects": []map[string]any{
					{
						"x":      48.0 * 5,
						"y":      48.0 * 5,
						"width":  48.0,
						"height": 0.0,
					},
				},
			},
		},
	})
	if err != nil {
		h.t.Fatalf("SeedMap: marshal tiled json: %v", err)
	}
	mp, err := h.queries.CreateMap(context.Background(), refdata.CreateMapParams{
		CampaignID:    h.campaignID,
		Name:          "Test Map",
		WidthSquares:  int32(w),
		HeightSquares: int32(hght),
		TiledJson:     tiledJSON,
	})
	if err != nil {
		h.t.Fatalf("SeedMap: CreateMap: %v", err)
	}
	return mp
}

// AttachMapToEncounter wires an existing map to an encounter so the /move
// handler's mapProvider lookup succeeds. The migrations leave map_id
// nullable, so the harness uses raw SQL instead of a sqlc-named UPDATE.
func (h *e2eHarness) AttachMapToEncounter(encounterID, mapID uuid.UUID) {
	h.t.Helper()
	if _, err := h.db.Exec("UPDATE encounters SET map_id=$1 WHERE id=$2", mapID, encounterID); err != nil {
		h.t.Fatalf("AttachMapToEncounter: %v", err)
	}
}

// SeedCompletedEncounter creates an encounter directly in status='completed'
// so the loot service's "must be completed" precondition is satisfied. The
// encounter has no combatants — production /loot consults the pool, not the
// combatant list.
func (h *e2eHarness) SeedCompletedEncounter() refdata.Encounter {
	h.t.Helper()
	enc, err := h.queries.CreateEncounter(context.Background(), refdata.CreateEncounterParams{
		CampaignID:  h.campaignID,
		Name:        "Completed Encounter",
		Status:      "completed",
		RoundNumber: 0,
	})
	if err != nil {
		h.t.Fatalf("SeedCompletedEncounter: CreateEncounter: %v", err)
	}
	return enc
}

// SeedEncounterShell creates a "preparing" encounter with no current turn.
// Combatants and the active turn are layered in via SeedCombatant +
// PromoteEncounterToActive once their character ids are known.
func (h *e2eHarness) SeedEncounterShell() refdata.Encounter {
	h.t.Helper()
	return testutil.NewTestEncounter(h.t, h.queries, h.campaignID)
}

// PromoteEncounterToActive flips an encounter to status='active' at round 1
// and creates a Turn pointing at turnHolderID, wiring it as current_turn_id.
// Returns the updated encounter.
func (h *e2eHarness) PromoteEncounterToActive(encounterID, turnHolderID uuid.UUID) (refdata.Encounter, refdata.Turn) {
	h.t.Helper()
	if _, err := h.db.Exec("UPDATE encounters SET status='active', round_number=1 WHERE id=$1", encounterID); err != nil {
		h.t.Fatalf("PromoteEncounterToActive: update status: %v", err)
	}
	turn, err := h.queries.CreateTurn(context.Background(), refdata.CreateTurnParams{
		EncounterID:         encounterID,
		CombatantID:         turnHolderID,
		RoundNumber:         1,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	if err != nil {
		h.t.Fatalf("PromoteEncounterToActive: create turn: %v", err)
	}
	if _, err := h.db.Exec("UPDATE encounters SET current_turn_id=$1 WHERE id=$2", turn.ID, encounterID); err != nil {
		h.t.Fatalf("PromoteEncounterToActive: set current_turn_id: %v", err)
	}
	enc, err := h.queries.GetEncounter(context.Background(), encounterID)
	if err != nil {
		h.t.Fatalf("PromoteEncounterToActive: re-read: %v", err)
	}
	return enc, turn
}

// SeedCombatant creates a combatant in the given encounter for the supplied
// character at the requested grid position.
func (h *e2eHarness) SeedCombatant(encounterID, characterID uuid.UUID, displayName, posCol string, posRow int32) refdata.Combatant {
	h.t.Helper()
	short := "c-" + uuid.NewString()[:8]
	comb, err := h.queries.CreateCombatant(context.Background(), refdata.CreateCombatantParams{
		EncounterID:     encounterID,
		CharacterID:     uuid.NullUUID{UUID: characterID, Valid: true},
		ShortID:         short,
		DisplayName:     displayName,
		InitiativeRoll:  10,
		InitiativeOrder: 1,
		PositionCol:     posCol,
		PositionRow:     posRow,
		AltitudeFt:      0,
		HpMax:           20,
		HpCurrent:       20,
		TempHp:          0,
		Ac:              15,
		Conditions:      []byte(`[]`),
		ExhaustionLevel: 0,
		IsVisible:       true,
		IsAlive:         true,
		IsNpc:           false,
	})
	if err != nil {
		h.t.Fatalf("seed combatant: %v", err)
	}
	return comb
}

// PlayerCommand builds a /name slash-command interaction for the supplied
// Discord user with the given string options and delivers it through the
// fake to the production CommandRouter. Returns the assigned interaction ID
// so the scenario can later assert on it via Transcript filtering.
func (h *e2eHarness) PlayerCommand(discordUserID, name string, opts ...slashOpt) string {
	h.t.Helper()
	interactionID := uuid.NewString()
	options := make([]*discordgo.ApplicationCommandInteractionDataOption, 0, len(opts))
	for _, o := range opts {
		options = append(options, &discordgo.ApplicationCommandInteractionDataOption{
			Name:  o.name,
			Type:  discordgo.ApplicationCommandOptionString,
			Value: o.value,
		})
	}
	interaction := &discordgo.Interaction{
		ID:        interactionID,
		ChannelID: "ch-cmd-" + h.guildID,
		GuildID:   h.guildID,
		Type:      discordgo.InteractionApplicationCommand,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: discordUserID, Username: "player-" + discordUserID},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    name,
			Options: options,
		},
	}
	h.fake.InjectInteraction(interaction)
	return interactionID
}

// slashOpt is a tiny wrapper for typed string options used by PlayerCommand.
type slashOpt struct {
	name  string
	value string
}

// stringOpt is a constructor for a string slash-command option.
func stringOpt(name, value string) slashOpt {
	return slashOpt{name: name, value: value}
}

// WaitForInteractionResponse blocks until an interaction response with the
// given InteractionID arrives in the transcript or the timeout expires.
func (h *e2eHarness) WaitForInteractionResponse(interactionID string, timeout time.Duration) discordfake.Entry {
	h.t.Helper()
	entry, err := h.fake.WaitFor(func(e discordfake.Entry) bool {
		return e.Kind == discordfake.KindInteractionResponse && e.InteractionID == interactionID
	}, timeout)
	if err != nil {
		h.t.Fatalf("WaitForInteractionResponse(%s): %v\nFull transcript:\n%s", interactionID, err, h.RenderTranscript())
	}
	return entry
}

// AssertEphemeralContains fails the test unless the harness has seen at
// least one ephemeral interaction response that contains every supplied
// substring.
func (h *e2eHarness) AssertEphemeralContains(interactionID string, substrs ...string) discordfake.Entry {
	h.t.Helper()
	entry := h.WaitForInteractionResponse(interactionID, 5*time.Second)
	if !entry.Ephemeral {
		h.t.Fatalf("expected ephemeral response, got non-ephemeral.\nContent: %q", entry.Content)
	}
	for _, sub := range substrs {
		if !strings.Contains(entry.Content, sub) {
			h.t.Fatalf("expected response to contain %q; got %q\nFull transcript:\n%s", sub, entry.Content, h.RenderTranscript())
		}
	}
	return entry
}

// RenderTranscript returns a stable, redacted multi-line dump of every
// recorded transcript entry in order. UUIDs and channel-id suffixes are
// replaced with stable placeholders so the result can be compared against
// in-test golden strings without per-run flake.
func (h *e2eHarness) RenderTranscript() string {
	entries := h.fake.Transcript()
	var b strings.Builder
	uuidMap := map[string]string{}
	for i, e := range entries {
		fmt.Fprintf(&b, "[%d] %s", i+1, e.Kind)
		if e.Ephemeral {
			b.WriteString(" ephemeral")
		}
		if e.ChannelID != "" {
			fmt.Fprintf(&b, " channel=%s", redactChannel(e.ChannelID, h.guildID))
		}
		if e.InteractionID != "" {
			fmt.Fprintf(&b, " interaction=%s", placeholderUUID(e.InteractionID, uuidMap))
		}
		if e.MessageID != "" {
			fmt.Fprintf(&b, " message=%s", redactChannel(e.MessageID, h.guildID))
		}
		content := redactUUIDs(e.Content, uuidMap)
		fmt.Fprintf(&b, " content=%q", oneLine(content))
		if len(e.Embeds) > 0 {
			fmt.Fprintf(&b, " embeds=%d", len(e.Embeds))
		}
		if len(e.Components) > 0 {
			fmt.Fprintf(&b, " components=%d", len(e.Components))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// oneLine collapses newlines to "\n" literals and trims leading/trailing
// whitespace so multi-line messages render on one transcript line.
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	return strings.TrimSpace(s)
}

// redactChannel replaces the trailing per-guild suffix with <GUILD> so
// transcripts compare equal across runs.
func redactChannel(id, guild string) string {
	if guild == "" {
		return id
	}
	return strings.ReplaceAll(id, guild, "<GUILD>")
}

// placeholderUUID returns a deterministic <UUID-N> placeholder for an
// observed UUID, registering new ones on demand.
func placeholderUUID(s string, m map[string]string) string {
	if v, ok := m[s]; ok {
		return v
	}
	v := fmt.Sprintf("<UUID-%d>", len(m)+1)
	m[s] = v
	return v
}

// redactUUIDs scans content for any UUID-shaped substring and replaces it
// with a deterministic <UUID-N> placeholder so downstream golden compares
// stay stable across runs.
func redactUUIDs(s string, m map[string]string) string {
	// Cheap scanner: 36-char chunks with dashes at the canonical positions.
	out := s
	for i := 0; i+36 <= len(out); i++ {
		chunk := out[i : i+36]
		if isUUID(chunk) {
			ph := placeholderUUID(chunk, m)
			out = out[:i] + ph + out[i+36:]
			i += len(ph) - 1
		}
	}
	return out
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, ch := range s {
		switch i {
		case 8, 13, 18, 23:
			if ch != '-' {
				return false
			}
		default:
			if !isHex(byte(ch)) {
				return false
			}
		}
	}
	return true
}

func isHex(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

// waitForHTTP polls /health until it answers 200 or 5s passes.
func waitForHTTP(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	url := fmt.Sprintf("http://%s/health", addr)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("e2e harness: server did not answer /health within 5s at %s", addr)
}
