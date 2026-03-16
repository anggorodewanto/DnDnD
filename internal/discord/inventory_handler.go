package discord

import (
	"context"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/inventory"
	"github.com/ab/dndnd/internal/refdata"
)

// InventoryCampaignProvider provides the campaign for a guild.
type InventoryCampaignProvider interface {
	GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error)
}

// InventoryCharacterLookup resolves a Discord user to their character.
type InventoryCharacterLookup interface {
	GetCharacterByCampaignAndDiscord(ctx context.Context, campaignID uuid.UUID, discordUserID string) (refdata.Character, error)
}

// InventoryHandler handles the /inventory slash command.
type InventoryHandler struct {
	session         Session
	campaignProv    InventoryCampaignProvider
	characterLookup InventoryCharacterLookup
}

// NewInventoryHandler creates a new InventoryHandler.
func NewInventoryHandler(
	session Session,
	campaignProv InventoryCampaignProvider,
	characterLookup InventoryCharacterLookup,
) *InventoryHandler {
	return &InventoryHandler{
		session:         session,
		campaignProv:    campaignProv,
		characterLookup: characterLookup,
	}
}

// Handle processes the /inventory command interaction.
func (h *InventoryHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	campaign, err := h.campaignProv.GetCampaignByGuildID(ctx, interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	userID := discordUserID(interaction)
	char, err := h.characterLookup.GetCharacterByCampaignAndDiscord(ctx, campaign.ID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not find your character. Use `/register` first.")
		return
	}

	items, err := character.ParseInventoryItems(char.Inventory.RawMessage, char.Inventory.Valid)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to read inventory. Please contact the DM.")
		return
	}

	attunement, err := character.ParseAttunementSlots(char.AttunementSlots.RawMessage, char.AttunementSlots.Valid)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to read attunement data. Please contact the DM.")
		return
	}

	msg := inventory.FormatInventory(char.Name, char.Gold, items, attunement)
	respondEphemeral(h.session, interaction, msg)
}
