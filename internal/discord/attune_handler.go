package discord

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/inventory"
	"github.com/ab/dndnd/internal/refdata"
)

// AttuneCharacterStore persists attunement slot updates.
type AttuneCharacterStore interface {
	UpdateCharacterAttunementSlots(ctx context.Context, arg refdata.UpdateCharacterAttunementSlotsParams) (refdata.Character, error)
}

// AttuneHandler handles the /attune slash command.
type AttuneHandler struct {
	session         Session
	campaignProv    InventoryCampaignProvider
	characterLookup InventoryCharacterLookup
	store           AttuneCharacterStore
}

// NewAttuneHandler creates a new AttuneHandler.
func NewAttuneHandler(
	session Session,
	campaignProv InventoryCampaignProvider,
	characterLookup InventoryCharacterLookup,
	store AttuneCharacterStore,
) *AttuneHandler {
	return &AttuneHandler{
		session:         session,
		campaignProv:    campaignProv,
		characterLookup: characterLookup,
		store:           store,
	}
}

// Handle processes the /attune command interaction.
func (h *AttuneHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	itemID := ""
	for _, opt := range data.Options {
		if opt.Name == "item" {
			itemID = opt.StringValue()
		}
	}
	if itemID == "" {
		respondEphemeral(h.session, interaction, "Please specify an item to attune to.")
		return
	}

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

	slots, err := character.ParseAttunementSlots(char.AttunementSlots.RawMessage, char.AttunementSlots.Valid)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to read attunement data. Please contact the DM.")
		return
	}

	result, err := inventory.Attune(inventory.AttuneInput{
		Items:  items,
		Slots:  slots,
		ItemID: itemID,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("%v", err))
		return
	}

	slotsJSON, err := json.Marshal(result.UpdatedSlots)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to save attunement changes. Please try again.")
		return
	}

	if _, err := h.store.UpdateCharacterAttunementSlots(ctx, refdata.UpdateCharacterAttunementSlotsParams{
		ID:              char.ID,
		AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
	}); err != nil {
		respondEphemeral(h.session, interaction, "Failed to save attunement changes. Please try again.")
		return
	}

	respondEphemeral(h.session, interaction, result.Message)
}
