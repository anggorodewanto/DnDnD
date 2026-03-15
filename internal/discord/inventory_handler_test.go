package discord

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// mockInventorySession captures the ephemeral response.
type mockInventorySession struct {
	lastResponse string
}

func (m *mockInventorySession) UserChannelCreate(recipientID string) (*discordgo.Channel, error) {
	return nil, nil
}
func (m *mockInventorySession) ChannelMessageSend(channelID, content string) (*discordgo.Message, error) {
	return nil, nil
}
func (m *mockInventorySession) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
	return nil, nil
}
func (m *mockInventorySession) ApplicationCommandBulkOverwrite(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
	return nil, nil
}
func (m *mockInventorySession) ApplicationCommands(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
	return nil, nil
}
func (m *mockInventorySession) ApplicationCommandDelete(appID, guildID, cmdID string) error {
	return nil
}
func (m *mockInventorySession) GuildChannels(guildID string) ([]*discordgo.Channel, error) {
	return nil, nil
}
func (m *mockInventorySession) GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
	return nil, nil
}
func (m *mockInventorySession) InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
	if resp.Data != nil {
		m.lastResponse = resp.Data.Content
	}
	return nil
}
func (m *mockInventorySession) InteractionResponseEdit(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit) (*discordgo.Message, error) {
	return nil, nil
}
func (m *mockInventorySession) ChannelMessageEdit(channelID, messageID, content string) (*discordgo.Message, error) {
	return nil, nil
}
func (m *mockInventorySession) GetState() *discordgo.State {
	return nil
}

// mockInventoryCampaignProvider returns a fixed campaign.
type mockInventoryCampaignProvider struct {
	campaign refdata.Campaign
	err      error
}

func (m *mockInventoryCampaignProvider) GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error) {
	return m.campaign, m.err
}

// mockInventoryCharacterLookup returns a fixed character.
type mockInventoryCharacterLookup struct {
	char refdata.Character
	err  error
}

func (m *mockInventoryCharacterLookup) GetCharacterByCampaignAndDiscord(ctx context.Context, campaignID uuid.UUID, discordUserID string) (refdata.Character, error) {
	return m.char, m.err
}

func makeInventoryInteraction(guildID, userID string) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: guildID,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: userID},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "inventory",
		},
	}
}

func TestInventoryHandler_ShowsInventory(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Equipped: true, Type: "weapon"},
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	itemsJSON, _ := json.Marshal(items)

	attunement := []character.AttunementSlot{}
	attunementJSON, _ := json.Marshal(attunement)

	handler := NewInventoryHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              uuid.New(),
			CampaignID:      campID,
			Name:            "Aria",
			Gold:            23,
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: attunementJSON, Valid: true},
		}},
	)

	interaction := makeInventoryInteraction("guild1", "user1")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Aria's Inventory (23 gp)")
	assert.Contains(t, sess.lastResponse, "Longsword")
	assert.Contains(t, sess.lastResponse, "Healing Potion")
}

func TestInventoryHandler_NoCampaign(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewInventoryHandler(sess,
		&mockInventoryCampaignProvider{err: assert.AnError},
		&mockInventoryCharacterLookup{},
	)

	interaction := makeInventoryInteraction("guild1", "user1")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "No campaign")
}

func TestInventoryHandler_NoCharacter(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	handler := NewInventoryHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{err: assert.AnError},
	)

	interaction := makeInventoryInteraction("guild1", "user1")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Could not find your character")
}

func TestInventoryHandler_EmptyInventory(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	handler := NewInventoryHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         uuid.New(),
			CampaignID: campID,
			Name:       "Gorak",
			Gold:       0,
		}},
	)

	interaction := makeInventoryInteraction("guild1", "user1")
	handler.Handle(interaction)

	require.Contains(t, sess.lastResponse, "Gorak's Inventory (0 gp)")
	assert.Contains(t, sess.lastResponse, "empty")
}
