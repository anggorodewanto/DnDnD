package discord

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/combat"
)

type mockSummonCommandService struct {
	commandCreatureFn func(ctx context.Context, input combat.CommandCreatureInput) (combat.CommandCreatureResult, error)
}

func (m *mockSummonCommandService) CommandCreature(ctx context.Context, input combat.CommandCreatureInput) (combat.CommandCreatureResult, error) {
	if m.commandCreatureFn != nil {
		return m.commandCreatureFn(ctx, input)
	}
	return combat.CommandCreatureResult{}, errors.New("not configured")
}

type mockSummonEncounterProvider struct {
	encounterID uuid.UUID
	err         error
}

func (m *mockSummonEncounterProvider) ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error) {
	return m.encounterID, m.err
}

type mockSummonPlayerLookup struct {
	combatantID   uuid.UUID
	combatantName string
	err           error
}

func (m *mockSummonPlayerLookup) GetCombatantIDByDiscordUser(ctx context.Context, encounterID uuid.UUID, discordUserID string) (uuid.UUID, string, error) {
	return m.combatantID, m.combatantName, m.err
}

func buildCommandInteraction(creatureID, action, target string) *discordgo.Interaction {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "creature_id", Type: discordgo.ApplicationCommandOptionString, Value: creatureID},
		{Name: "action", Type: discordgo.ApplicationCommandOptionString, Value: action},
	}
	if target != "" {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name: "target", Type: discordgo.ApplicationCommandOptionString, Value: target,
		})
	}
	return &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "command",
			Options: opts,
		},
		GuildID: "guild-1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user-1"}},
	}
}

func TestSummonCommandHandler_Success(t *testing.T) {
	summonerID := uuid.New()
	encounterID := uuid.New()

	var respondedContent string
	mock := newTestMock()
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Data != nil {
			respondedContent = resp.Data.Content
		}
		return nil
	}

	svc := &mockSummonCommandService{
		commandCreatureFn: func(ctx context.Context, input combat.CommandCreatureInput) (combat.CommandCreatureResult, error) {
			assert.Equal(t, encounterID, input.EncounterID)
			assert.Equal(t, summonerID, input.SummonerID)
			assert.Equal(t, "Aria", input.SummonerName)
			assert.Equal(t, "FAM", input.CreatureShortID)
			assert.Equal(t, "help", input.Action)
			assert.Equal(t, []string{"G1"}, input.Args)
			return combat.CommandCreatureResult{
				Action:    "help",
				CombatLog: "Aria's Owl (FAM) uses Help on G1",
			}, nil
		},
	}

	handler := NewSummonCommandHandler(mock, svc)
	handler.SetEncounterProvider(&mockSummonEncounterProvider{encounterID: encounterID})
	handler.SetPlayerLookup(&mockSummonPlayerLookup{combatantID: summonerID, combatantName: "Aria"})

	handler.Handle(buildCommandInteraction("FAM", "help", "G1"))
	assert.Contains(t, respondedContent, "Aria's Owl (FAM) uses Help on G1")
}

func TestSummonCommandHandler_NoEncounter(t *testing.T) {
	var respondedContent string
	mock := newTestMock()
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Data != nil {
			respondedContent = resp.Data.Content
		}
		return nil
	}

	handler := NewSummonCommandHandler(mock, &mockSummonCommandService{})
	handler.SetEncounterProvider(&mockSummonEncounterProvider{err: errors.New("none")})
	handler.SetPlayerLookup(&mockSummonPlayerLookup{})

	handler.Handle(buildCommandInteraction("FAM", "done", ""))
	assert.Contains(t, respondedContent, "No active encounter")
}

func TestSummonCommandHandler_ServiceError(t *testing.T) {
	var respondedContent string
	mock := newTestMock()
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Data != nil {
			respondedContent = resp.Data.Content
		}
		return nil
	}

	svc := &mockSummonCommandService{
		commandCreatureFn: func(ctx context.Context, input combat.CommandCreatureInput) (combat.CommandCreatureResult, error) {
			return combat.CommandCreatureResult{}, combat.ErrNotSummoner
		},
	}

	handler := NewSummonCommandHandler(mock, svc)
	handler.SetEncounterProvider(&mockSummonEncounterProvider{encounterID: uuid.New()})
	handler.SetPlayerLookup(&mockSummonPlayerLookup{combatantID: uuid.New(), combatantName: "Aria"})

	handler.Handle(buildCommandInteraction("FAM", "dismiss", ""))
	assert.Contains(t, respondedContent, "Command failed")
}

func TestSummonCommandHandler_NotConfigured(t *testing.T) {
	var respondedContent string
	mock := newTestMock()
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Data != nil {
			respondedContent = resp.Data.Content
		}
		return nil
	}

	handler := &SummonCommandHandler{session: mock}
	handler.Handle(buildCommandInteraction("FAM", "done", ""))
	assert.Contains(t, respondedContent, "not fully configured")
}

func TestSummonCommandHandler_MissingArgs(t *testing.T) {
	var respondedContent string
	mock := newTestMock()
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Data != nil {
			respondedContent = resp.Data.Content
		}
		return nil
	}

	handler := NewSummonCommandHandler(mock, &mockSummonCommandService{})
	handler.SetEncounterProvider(&mockSummonEncounterProvider{encounterID: uuid.New()})
	handler.SetPlayerLookup(&mockSummonPlayerLookup{combatantID: uuid.New(), combatantName: "Aria"})

	// Empty creature_id
	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "command",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
		GuildID: "guild-1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user-1"}},
	}
	handler.Handle(interaction)
	assert.Contains(t, respondedContent, "Usage")
}

func TestSummonCommandHandler_PlayerNotFound(t *testing.T) {
	var respondedContent string
	mock := newTestMock()
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Data != nil {
			respondedContent = resp.Data.Content
		}
		return nil
	}

	handler := NewSummonCommandHandler(mock, &mockSummonCommandService{})
	handler.SetEncounterProvider(&mockSummonEncounterProvider{encounterID: uuid.New()})
	handler.SetPlayerLookup(&mockSummonPlayerLookup{err: errors.New("not found")})

	handler.Handle(buildCommandInteraction("FAM", "done", ""))
	assert.Contains(t, respondedContent, "Could not find your combatant")
}
