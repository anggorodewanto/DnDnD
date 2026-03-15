package discord

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- Mock types for RecapHandler ---

type mockRecapSession struct {
	lastResponse *discordgo.InteractionResponse
}

func (m *mockRecapSession) InteractionRespond(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
	m.lastResponse = resp
	return nil
}
func (m *mockRecapSession) InteractionResponseEdit(*discordgo.Interaction, *discordgo.WebhookEdit) (*discordgo.Message, error) {
	return nil, nil
}
func (m *mockRecapSession) UserChannelCreate(string) (*discordgo.Channel, error) { return nil, nil }
func (m *mockRecapSession) ChannelMessageSend(string, string) (*discordgo.Message, error) {
	return nil, nil
}
func (m *mockRecapSession) ChannelMessageSendComplex(string, *discordgo.MessageSend) (*discordgo.Message, error) {
	return nil, nil
}
func (m *mockRecapSession) ApplicationCommandBulkOverwrite(string, string, []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
	return nil, nil
}
func (m *mockRecapSession) ApplicationCommands(string, string) ([]*discordgo.ApplicationCommand, error) {
	return nil, nil
}
func (m *mockRecapSession) ApplicationCommandDelete(string, string, string) error { return nil }
func (m *mockRecapSession) GuildChannels(string) ([]*discordgo.Channel, error)   { return nil, nil }
func (m *mockRecapSession) GuildChannelCreateComplex(string, discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
	return nil, nil
}
func (m *mockRecapSession) ChannelMessageEdit(string, string, string) (*discordgo.Message, error) {
	return nil, nil
}
func (m *mockRecapSession) GetState() *discordgo.State { return nil }

type mockRecapService struct {
	getEncounter                    func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	listActionLogWithRounds         func(ctx context.Context, encounterID uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error)
	getMostRecentCompletedEncounter func(ctx context.Context, campaignID uuid.UUID) (refdata.Encounter, error)
	getLastCompletedTurnByCombatant func(ctx context.Context, encounterID, combatantID uuid.UUID) (refdata.Turn, error)
}

func (m *mockRecapService) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	return m.getEncounter(ctx, id)
}
func (m *mockRecapService) ListActionLogWithRounds(ctx context.Context, encounterID uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
	return m.listActionLogWithRounds(ctx, encounterID)
}
func (m *mockRecapService) GetMostRecentCompletedEncounter(ctx context.Context, campaignID uuid.UUID) (refdata.Encounter, error) {
	return m.getMostRecentCompletedEncounter(ctx, campaignID)
}
func (m *mockRecapService) GetLastCompletedTurnByCombatant(ctx context.Context, encounterID, combatantID uuid.UUID) (refdata.Turn, error) {
	return m.getLastCompletedTurnByCombatant(ctx, encounterID, combatantID)
}

type mockRecapEncounterProvider struct {
	getActiveEncounterID func(ctx context.Context, guildID string) (uuid.UUID, error)
}

func (m *mockRecapEncounterProvider) GetActiveEncounterID(ctx context.Context, guildID string) (uuid.UUID, error) {
	return m.getActiveEncounterID(ctx, guildID)
}

type mockRecapPlayerLookup struct {
	getCombatantIDByDiscordUser func(ctx context.Context, encounterID uuid.UUID, discordUserID string) (uuid.UUID, error)
}

func (m *mockRecapPlayerLookup) GetCombatantIDByDiscordUser(ctx context.Context, encounterID uuid.UUID, discordUserID string) (uuid.UUID, error) {
	return m.getCombatantIDByDiscordUser(ctx, encounterID, discordUserID)
}

type mockRecapCampaignProvider struct {
	getCampaignByGuildID func(ctx context.Context, guildID string) (refdata.Campaign, error)
}

func (m *mockRecapCampaignProvider) GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error) {
	return m.getCampaignByGuildID(ctx, guildID)
}

// --- Helper to build recap interaction ---

func makeRecapInteraction(rounds ...int64) *discordgo.Interaction {
	var opts []*discordgo.ApplicationCommandInteractionDataOption
	if len(rounds) > 0 {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name:  "rounds",
			Value: float64(rounds[0]),
			Type:  discordgo.ApplicationCommandOptionInteger,
		})
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "recap",
			Options: opts,
		},
	}
}

// --- Tests ---

func TestRecapHandler_WithRoundsArg_ActiveEncounter(t *testing.T) {
	encounterID := uuid.New()
	sess := &mockRecapSession{}

	svc := &mockRecapService{
		getEncounter: func(_ context.Context, id uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encounterID, Status: "active", RoundNumber: 5}, nil
		},
		listActionLogWithRounds: func(_ context.Context, _ uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
			return []refdata.ListActionLogWithRoundsRow{
				{RoundNumber: 3, Description: sql.NullString{String: "Goblin attacked", Valid: true}},
				{RoundNumber: 4, Description: sql.NullString{String: "Thorn attacked", Valid: true}},
				{RoundNumber: 5, Description: sql.NullString{String: "Aria cast Fireball", Valid: true}},
			}, nil
		},
	}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		},
	}

	handler := NewRecapHandler(sess, svc, encProv, nil)
	handler.Handle(makeRecapInteraction(2))

	require.NotNil(t, sess.lastResponse)
	content := sess.lastResponse.Data.Content
	assert.Contains(t, content, "Thorn attacked")
	assert.Contains(t, content, "Aria cast Fireball")
	// Round 3 should be excluded since we asked for last 2 rounds (4 and 5)
	assert.NotContains(t, content, "Goblin attacked")
	assert.Equal(t, discordgo.MessageFlagsEphemeral, sess.lastResponse.Data.Flags)
}

func TestRecapHandler_NoArgs_ActiveEncounter_SinceLastTurn(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	sess := &mockRecapSession{}

	svc := &mockRecapService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encounterID, Status: "active", RoundNumber: 5}, nil
		},
		listActionLogWithRounds: func(_ context.Context, _ uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
			return []refdata.ListActionLogWithRoundsRow{
				{RoundNumber: 2, Description: sql.NullString{String: "old action", Valid: true}, CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
				{RoundNumber: 3, Description: sql.NullString{String: "Goblin attacked", Valid: true}, CreatedAt: time.Date(2025, 1, 1, 0, 1, 0, 0, time.UTC)},
				{RoundNumber: 4, Description: sql.NullString{String: "Thorn attacked", Valid: true}, CreatedAt: time.Date(2025, 1, 1, 0, 2, 0, 0, time.UTC)},
			}, nil
		},
		getLastCompletedTurnByCombatant: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:          uuid.New(),
				CombatantID: combatantID,
				RoundNumber: 2,
				CompletedAt: sql.NullTime{Time: time.Date(2025, 1, 1, 0, 0, 30, 0, time.UTC), Valid: true},
			}, nil
		},
	}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		},
	}

	playerLookup := &mockRecapPlayerLookup{
		getCombatantIDByDiscordUser: func(_ context.Context, _ uuid.UUID, _ string) (uuid.UUID, error) {
			return combatantID, nil
		},
	}

	handler := NewRecapHandler(sess, svc, encProv, playerLookup)
	handler.Handle(makeRecapInteraction())

	require.NotNil(t, sess.lastResponse)
	content := sess.lastResponse.Data.Content
	// Should show entries after the player's last turn completed_at
	assert.Contains(t, content, "Goblin attacked")
	assert.Contains(t, content, "Thorn attacked")
	assert.NotContains(t, content, "old action")
	assert.Contains(t, content, "since your last turn")
}

func TestRecapHandler_NoActiveEncounter_FallsBackToCompleted(t *testing.T) {
	campaignID := uuid.New()
	completedEncID := uuid.New()
	sess := &mockRecapSession{}

	svc := &mockRecapService{
		getEncounter: func(_ context.Context, id uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: completedEncID, Status: "completed", RoundNumber: 3, CampaignID: campaignID}, nil
		},
		getMostRecentCompletedEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: completedEncID, Status: "completed", RoundNumber: 3, CampaignID: campaignID}, nil
		},
		listActionLogWithRounds: func(_ context.Context, _ uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
			return []refdata.ListActionLogWithRoundsRow{
				{RoundNumber: 1, Description: sql.NullString{String: "Round 1 action", Valid: true}},
				{RoundNumber: 2, Description: sql.NullString{String: "Round 2 action", Valid: true}},
				{RoundNumber: 3, Description: sql.NullString{String: "Round 3 action", Valid: true}},
			}, nil
		},
	}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.Nil, errors.New("no active encounter")
		},
	}

	campProv := &mockRecapCampaignProvider{
		getCampaignByGuildID: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		},
	}

	handler := NewRecapHandler(sess, svc, encProv, nil)
	handler.SetCampaignProvider(campProv)
	handler.Handle(makeRecapInteraction())

	require.NotNil(t, sess.lastResponse)
	content := sess.lastResponse.Data.Content
	assert.Contains(t, content, "Round 1 action")
	assert.Contains(t, content, "Round 3 action")
}

func TestRecapHandler_NoEncounterAtAll(t *testing.T) {
	sess := &mockRecapSession{}

	svc := &mockRecapService{
		getMostRecentCompletedEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{}, errors.New("not found")
		},
	}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.Nil, errors.New("no active encounter")
		},
	}

	campProv := &mockRecapCampaignProvider{
		getCampaignByGuildID: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{}, errors.New("not found")
		},
	}

	handler := NewRecapHandler(sess, svc, encProv, nil)
	handler.SetCampaignProvider(campProv)
	handler.Handle(makeRecapInteraction())

	require.NotNil(t, sess.lastResponse)
	content := sess.lastResponse.Data.Content
	assert.Contains(t, content, "No encounter found")
}

func TestRecapHandler_EmptyLogs(t *testing.T) {
	encounterID := uuid.New()
	sess := &mockRecapSession{}

	svc := &mockRecapService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encounterID, Status: "active", RoundNumber: 3}, nil
		},
		listActionLogWithRounds: func(_ context.Context, _ uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
			return nil, nil
		},
	}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		},
	}

	handler := NewRecapHandler(sess, svc, encProv, nil)
	handler.Handle(makeRecapInteraction(2))

	require.NotNil(t, sess.lastResponse)
	content := sess.lastResponse.Data.Content
	assert.Contains(t, content, "No combat activity to recap.")
}

func TestRecapHandler_WiredViaRouter(t *testing.T) {
	encounterID := uuid.New()
	sess := &mockRecapSession{}

	svc := &mockRecapService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encounterID, Status: "active", RoundNumber: 2}, nil
		},
		listActionLogWithRounds: func(_ context.Context, _ uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
			return []refdata.ListActionLogWithRoundsRow{
				{RoundNumber: 1, Description: sql.NullString{String: "test action", Valid: true}},
			}, nil
		},
	}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		},
	}

	handler := NewRecapHandler(sess, svc, encProv, nil)

	bot := &Bot{session: sess}
	router := NewCommandRouter(bot, nil)
	router.SetRecapHandler(handler)

	interaction := makeRecapInteraction(1)
	router.Handle(interaction)

	require.NotNil(t, sess.lastResponse)
	content := sess.lastResponse.Data.Content
	assert.True(t, strings.Contains(content, "test action"))
}

func TestRecapHandler_ListLogsError(t *testing.T) {
	encounterID := uuid.New()
	sess := &mockRecapSession{}

	svc := &mockRecapService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encounterID, Status: "active"}, nil
		},
		listActionLogWithRounds: func(_ context.Context, _ uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
			return nil, errors.New("db error")
		},
	}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		},
	}

	handler := NewRecapHandler(sess, svc, encProv, nil)
	handler.Handle(makeRecapInteraction())

	require.NotNil(t, sess.lastResponse)
	assert.Contains(t, sess.lastResponse.Data.Content, "Failed to retrieve combat logs.")
}

func TestRecapHandler_NoArgs_NoPlayerLookup_ShowsAll(t *testing.T) {
	encounterID := uuid.New()
	sess := &mockRecapSession{}

	svc := &mockRecapService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encounterID, Status: "active", RoundNumber: 3}, nil
		},
		listActionLogWithRounds: func(_ context.Context, _ uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
			return []refdata.ListActionLogWithRoundsRow{
				{RoundNumber: 1, Description: sql.NullString{String: "action1", Valid: true}},
				{RoundNumber: 2, Description: sql.NullString{String: "action2", Valid: true}},
			}, nil
		},
	}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		},
	}

	// No player lookup — should show all logs
	handler := NewRecapHandler(sess, svc, encProv, nil)
	handler.Handle(makeRecapInteraction())

	require.NotNil(t, sess.lastResponse)
	content := sess.lastResponse.Data.Content
	assert.Contains(t, content, "action1")
	assert.Contains(t, content, "action2")
}

func TestRecapHandler_NoArgs_PlayerNotInEncounter_ShowsAll(t *testing.T) {
	encounterID := uuid.New()
	sess := &mockRecapSession{}

	svc := &mockRecapService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encounterID, Status: "active", RoundNumber: 3}, nil
		},
		listActionLogWithRounds: func(_ context.Context, _ uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
			return []refdata.ListActionLogWithRoundsRow{
				{RoundNumber: 1, Description: sql.NullString{String: "action1", Valid: true}},
			}, nil
		},
	}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		},
	}

	playerLookup := &mockRecapPlayerLookup{
		getCombatantIDByDiscordUser: func(_ context.Context, _ uuid.UUID, _ string) (uuid.UUID, error) {
			return uuid.Nil, errors.New("not found")
		},
	}

	handler := NewRecapHandler(sess, svc, encProv, playerLookup)
	handler.Handle(makeRecapInteraction())

	require.NotNil(t, sess.lastResponse)
	content := sess.lastResponse.Data.Content
	assert.Contains(t, content, "action1")
}

func TestRecapHandler_NoArgs_NoPreviousTurn_ShowsAll(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	sess := &mockRecapSession{}

	svc := &mockRecapService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encounterID, Status: "active", RoundNumber: 3}, nil
		},
		listActionLogWithRounds: func(_ context.Context, _ uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
			return []refdata.ListActionLogWithRoundsRow{
				{RoundNumber: 1, Description: sql.NullString{String: "action1", Valid: true}},
			}, nil
		},
		getLastCompletedTurnByCombatant: func(_ context.Context, _, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, errors.New("no previous turn")
		},
	}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		},
	}

	playerLookup := &mockRecapPlayerLookup{
		getCombatantIDByDiscordUser: func(_ context.Context, _ uuid.UUID, _ string) (uuid.UUID, error) {
			return combatantID, nil
		},
	}

	handler := NewRecapHandler(sess, svc, encProv, playerLookup)
	handler.Handle(makeRecapInteraction())

	require.NotNil(t, sess.lastResponse)
	content := sess.lastResponse.Data.Content
	assert.Contains(t, content, "action1")
}

func TestRecapHandler_NoCampaignProvider_NoActiveEncounter(t *testing.T) {
	sess := &mockRecapSession{}

	svc := &mockRecapService{}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.Nil, errors.New("no active encounter")
		},
	}

	// No campaign provider set
	handler := NewRecapHandler(sess, svc, encProv, nil)
	handler.Handle(makeRecapInteraction())

	require.NotNil(t, sess.lastResponse)
	assert.Contains(t, sess.lastResponse.Data.Content, "No encounter found")
}

func TestRecapHandler_ActiveEncounterID_GetEncounterFails_FallsBackToCompleted(t *testing.T) {
	campaignID := uuid.New()
	completedEncID := uuid.New()
	sess := &mockRecapSession{}

	svc := &mockRecapService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			// Active encounter exists but GetEncounter fails — then fallback uses completed
			return refdata.Encounter{}, errors.New("db error")
		},
		getMostRecentCompletedEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: completedEncID, Status: "completed", RoundNumber: 2}, nil
		},
		listActionLogWithRounds: func(_ context.Context, _ uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
			return []refdata.ListActionLogWithRoundsRow{
				{RoundNumber: 1, Description: sql.NullString{String: "fallback action", Valid: true}},
			}, nil
		},
	}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.New(), nil // returns ID, but GetEncounter will fail
		},
	}

	campProv := &mockRecapCampaignProvider{
		getCampaignByGuildID: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		},
	}

	handler := NewRecapHandler(sess, svc, encProv, nil)
	handler.SetCampaignProvider(campProv)
	handler.Handle(makeRecapInteraction())

	require.NotNil(t, sess.lastResponse)
	assert.Contains(t, sess.lastResponse.Data.Content, "fallback action")
}

func TestRecapHandler_NoArgs_LastTurnCompletedAtNull_ShowsAll(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	sess := &mockRecapSession{}

	svc := &mockRecapService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encounterID, Status: "active", RoundNumber: 3}, nil
		},
		listActionLogWithRounds: func(_ context.Context, _ uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
			return []refdata.ListActionLogWithRoundsRow{
				{RoundNumber: 1, Description: sql.NullString{String: "action1", Valid: true}},
				{RoundNumber: 2, Description: sql.NullString{String: "action2", Valid: true}},
			}, nil
		},
		getLastCompletedTurnByCombatant: func(_ context.Context, _, _ uuid.UUID) (refdata.Turn, error) {
			// CompletedAt is NOT valid (null) — all logs should be included
			return refdata.Turn{
				ID:          uuid.New(),
				CombatantID: combatantID,
				CompletedAt: sql.NullTime{Valid: false},
			}, nil
		},
	}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		},
	}

	playerLookup := &mockRecapPlayerLookup{
		getCombatantIDByDiscordUser: func(_ context.Context, _ uuid.UUID, _ string) (uuid.UUID, error) {
			return combatantID, nil
		},
	}

	handler := NewRecapHandler(sess, svc, encProv, playerLookup)
	handler.Handle(makeRecapInteraction())

	require.NotNil(t, sess.lastResponse)
	content := sess.lastResponse.Data.Content
	assert.Contains(t, content, "action1")
	assert.Contains(t, content, "action2")
	assert.Contains(t, content, "since your last turn")
}

func TestRecapHandler_CampaignFound_NoCompletedEncounter(t *testing.T) {
	campaignID := uuid.New()
	sess := &mockRecapSession{}

	svc := &mockRecapService{
		getMostRecentCompletedEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{}, errors.New("no completed encounters")
		},
	}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.Nil, errors.New("no active")
		},
	}

	campProv := &mockRecapCampaignProvider{
		getCampaignByGuildID: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		},
	}

	handler := NewRecapHandler(sess, svc, encProv, nil)
	handler.SetCampaignProvider(campProv)
	handler.Handle(makeRecapInteraction())

	require.NotNil(t, sess.lastResponse)
	assert.Contains(t, sess.lastResponse.Data.Content, "No encounter found")
}

func TestRecapHandler_ActiveEncounterID_GetEncounterFails_NoCampaignProvider(t *testing.T) {
	sess := &mockRecapSession{}

	svc := &mockRecapService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{}, errors.New("db error")
		},
	}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.New(), nil
		},
	}

	handler := NewRecapHandler(sess, svc, encProv, nil)
	handler.Handle(makeRecapInteraction())

	require.NotNil(t, sess.lastResponse)
	assert.Contains(t, sess.lastResponse.Data.Content, "No encounter found")
}

func TestRecapHandler_NoArgs_NilMember(t *testing.T) {
	encounterID := uuid.New()
	sess := &mockRecapSession{}

	svc := &mockRecapService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encounterID, Status: "active", RoundNumber: 3}, nil
		},
		listActionLogWithRounds: func(_ context.Context, _ uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
			return []refdata.ListActionLogWithRoundsRow{
				{RoundNumber: 1, Description: sql.NullString{String: "action1", Valid: true}},
			}, nil
		},
	}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		},
	}

	playerLookup := &mockRecapPlayerLookup{
		getCombatantIDByDiscordUser: func(_ context.Context, _ uuid.UUID, userID string) (uuid.UUID, error) {
			// Called with empty userID since Member is nil
			return uuid.Nil, errors.New("not found")
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  nil, // nil member
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "recap",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
	}

	handler := NewRecapHandler(sess, svc, encProv, playerLookup)
	handler.Handle(interaction)

	require.NotNil(t, sess.lastResponse)
	assert.Contains(t, sess.lastResponse.Data.Content, "action1")
}

func TestRecapHandler_Truncation(t *testing.T) {
	encounterID := uuid.New()
	sess := &mockRecapSession{}

	// Generate a very long log to trigger truncation
	var logs []refdata.ListActionLogWithRoundsRow
	for i := 0; i < 200; i++ {
		logs = append(logs, refdata.ListActionLogWithRoundsRow{
			RoundNumber: 1,
			Description: sql.NullString{String: "A very long action description that takes up space in the message", Valid: true},
		})
	}

	svc := &mockRecapService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encounterID, Status: "active", RoundNumber: 1}, nil
		},
		listActionLogWithRounds: func(_ context.Context, _ uuid.UUID) ([]refdata.ListActionLogWithRoundsRow, error) {
			return logs, nil
		},
	}

	encProv := &mockRecapEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		},
	}

	handler := NewRecapHandler(sess, svc, encProv, nil)
	handler.Handle(makeRecapInteraction(1))

	require.NotNil(t, sess.lastResponse)
	content := sess.lastResponse.Data.Content
	assert.True(t, len(content) <= 2000)
	assert.Contains(t, content, "... (truncated)")
}
