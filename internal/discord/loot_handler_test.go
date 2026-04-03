package discord

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/refdata"
)

// mockLootEncounterProvider returns a fixed encounter.
type mockLootEncounterProvider struct {
	encounter refdata.Encounter
	err       error
}

func (m *mockLootEncounterProvider) GetMostRecentCompletedEncounter(ctx context.Context, campaignID uuid.UUID) (refdata.Encounter, error) {
	return m.encounter, m.err
}

func makeLootInteraction(guildID, userID string) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: guildID,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: userID},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "loot",
		},
	}
}

func TestLootHandler_NoCampaign(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewLootHandler(
		sess,
		&mockInventoryCampaignProvider{err: errors.New("not found")},
		&mockInventoryCharacterLookup{},
		&mockLootEncounterProvider{},
		nil,
	)

	interaction := makeLootInteraction("guild1", "user1")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "No campaign")
}

func TestLootHandler_NoCharacter(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	handler := NewLootHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{err: errors.New("not found")},
		&mockLootEncounterProvider{},
		nil,
	)

	interaction := makeLootInteraction("guild1", "user1")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Could not find your character")
}

func TestLootHandler_NoEncounter(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()
	handler := NewLootHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{ID: charID, CampaignID: campID, Name: "Aria"}},
		&mockLootEncounterProvider{err: errors.New("no encounter")},
		nil,
	)

	interaction := makeLootInteraction("guild1", "user1")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "No completed encounter")
}
