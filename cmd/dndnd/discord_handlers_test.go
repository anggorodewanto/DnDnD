package main

import (
	"context"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/database"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/testutil"
)

// testSession is a minimal discord.Session implementation used by the
// buildDiscordHandlers wiring tests. It records the arguments of the calls
// exercised by the enemy-turn-notifier smoke test and no-ops everything else.
type testSession struct {
	sendFunc func(channelID, content string) (*discordgo.Message, error)
}

func (t *testSession) UserChannelCreate(recipientID string) (*discordgo.Channel, error) {
	return &discordgo.Channel{}, nil
}
func (t *testSession) ChannelMessageSend(channelID, content string) (*discordgo.Message, error) {
	if t.sendFunc != nil {
		return t.sendFunc(channelID, content)
	}
	return &discordgo.Message{}, nil
}
func (t *testSession) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
	return &discordgo.Message{}, nil
}
func (t *testSession) ApplicationCommandBulkOverwrite(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
	return nil, nil
}
func (t *testSession) ApplicationCommands(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
	return nil, nil
}
func (t *testSession) ApplicationCommandDelete(appID, guildID, cmdID string) error { return nil }
func (t *testSession) GuildChannels(guildID string) ([]*discordgo.Channel, error)  { return nil, nil }
func (t *testSession) GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
	return &discordgo.Channel{}, nil
}
func (t *testSession) InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
	return nil
}
func (t *testSession) InteractionResponseEdit(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit) (*discordgo.Message, error) {
	return &discordgo.Message{}, nil
}
func (t *testSession) ChannelMessageEdit(channelID, messageID, content string) (*discordgo.Message, error) {
	return &discordgo.Message{}, nil
}
func (t *testSession) GetState() *discordgo.State { return nil }

// TestBuildDiscordHandlers_ConstructsAllPhase105Handlers ensures the Phase 105b
// wiring helper constructs every handler with an *EncounterProvider dependency
// and injects a resolver into each, so no handler is left on the tests-only
// wiring path in production.
func TestBuildDiscordHandlers_ConstructsAllPhase105Handlers(t *testing.T) {
	session := &testSession{}
	deps := discordHandlerDeps{
		session:        session,
		queries:        nil, // not invoked in constructor path
		combatService:  combat.NewService(nil),
		roller:         dice.NewRoller(nil),
		resolver:       &stubUserEncounterResolver{},
	}
	result := buildDiscordHandlers(deps)

	require.NotNil(t, result.move, "move handler must be constructed")
	require.NotNil(t, result.fly, "fly handler must be constructed")
	require.NotNil(t, result.distance, "distance handler must be constructed")
	require.NotNil(t, result.done, "done handler must be constructed")
	require.NotNil(t, result.check, "check handler must be constructed")
	require.NotNil(t, result.save, "save handler must be constructed")
	require.NotNil(t, result.rest, "rest handler must be constructed")
	require.NotNil(t, result.summon, "summon command handler must be constructed")
	require.NotNil(t, result.recap, "recap handler must be constructed")
}

// TestBuildDiscordHandlers_EnemyTurnNotifierHasEncounterLookup ensures the
// Phase 105b wiring sets SetEncounterLookup on the DiscordEnemyTurnNotifier so
// combat log messages posted to shared channels get the "⚔️ <name> — Round N"
// label instead of the empty fallback.
func TestBuildDiscordHandlers_EnemyTurnNotifierHasEncounterLookup(t *testing.T) {
	var sentContent string
	session := &testSession{
		sendFunc: func(channelID, content string) (*discordgo.Message, error) {
			sentContent = content
			return &discordgo.Message{}, nil
		},
	}

	encID := uuid.New()
	combatSvc := &stubEnemyTurnCombatService{
		enc: refdata.Encounter{
			ID:          encID,
			Name:        "Cavern Skirmish",
			RoundNumber: 7,
		},
	}

	deps := discordHandlerDeps{
		session:       session,
		combatService: nil,
		roller:        dice.NewRoller(nil),
		resolver:      &stubUserEncounterResolver{},
		campaignSettings: &stubCampaignSettingsProvider{
			channels: map[string]string{"combat-log": "ch-cl", "combat-map": "ch-cm"},
		},
		enemyTurnEncounterLookup: combatSvc,
	}
	result := buildDiscordHandlers(deps)

	require.NotNil(t, result.enemyTurnNotifier, "enemy turn notifier must be constructed")
	result.enemyTurnNotifier.NotifyEnemyTurnExecuted(context.Background(), encID, "Goblin attacks!")

	if !strings.Contains(sentContent, "Cavern Skirmish") {
		t.Errorf("expected combat log to include encounter display name, got %q", sentContent)
	}
	if !strings.Contains(sentContent, "Round 7") {
		t.Errorf("expected combat log to include round number, got %q", sentContent)
	}
}

type stubUserEncounterResolver struct{}

func (s *stubUserEncounterResolver) ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error) {
	return uuid.Nil, nil
}

type stubCampaignSettingsProvider struct {
	channels map[string]string
}

func (s *stubCampaignSettingsProvider) GetChannelIDs(ctx context.Context, encounterID uuid.UUID) (map[string]string, error) {
	return s.channels, nil
}

type stubEnemyTurnCombatService struct {
	enc refdata.Encounter
}

func (s *stubEnemyTurnCombatService) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	return s.enc, nil
}

// TestBuildDiscordHandlers_Integration exercises buildDiscordHandlers with a
// real Postgres-backed refdata.Queries and a real combat.Service so the
// production wiring path is end-to-end covered.
func TestBuildDiscordHandlers_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)
	require.NoError(t, database.MigrateUp(db, dbfs.Migrations))

	queries := refdata.New(db)
	combatStore := combat.NewStoreAdapter(queries)
	combatSvc := combat.NewService(combatStore)

	deps := discordHandlerDeps{
		session:                  &testSession{},
		queries:                  queries,
		combatService:            combatSvc,
		roller:                   dice.NewRoller(nil),
		resolver:                 newDiscordUserEncounterResolver(queries),
		enemyTurnEncounterLookup: combatSvc,
	}
	result := buildDiscordHandlers(deps)

	assert.NotNil(t, result.move)
	assert.NotNil(t, result.fly)
	assert.NotNil(t, result.distance)
	assert.NotNil(t, result.done)
	assert.NotNil(t, result.check)
	assert.NotNil(t, result.save)
	assert.NotNil(t, result.rest)
	assert.NotNil(t, result.summon)
	assert.NotNil(t, result.recap)
	assert.NotNil(t, result.enemyTurnNotifier)
}
