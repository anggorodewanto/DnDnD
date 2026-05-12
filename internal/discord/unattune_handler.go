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

// UnattuneHandler handles the /unattune slash command.
type UnattuneHandler struct {
	session         Session
	campaignProv    InventoryCampaignProvider
	characterLookup InventoryCharacterLookup
	store           AttuneCharacterStore
	cardUpdater     CardUpdater // SR-007
}

// SetCardUpdater wires the SR-007 character-card refresh callback fired
// after a successful /unattune write.
func (h *UnattuneHandler) SetCardUpdater(u CardUpdater) {
	h.cardUpdater = u
}

// NewUnattuneHandler creates a new UnattuneHandler.
func NewUnattuneHandler(
	session Session,
	campaignProv InventoryCampaignProvider,
	characterLookup InventoryCharacterLookup,
	store AttuneCharacterStore,
) *UnattuneHandler {
	return &UnattuneHandler{
		session:         session,
		campaignProv:    campaignProv,
		characterLookup: characterLookup,
		store:           store,
	}
}

// Handle processes the /unattune command interaction.
func (h *UnattuneHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	itemID := ""
	for _, opt := range data.Options {
		if opt.Name == "item" {
			itemID = opt.StringValue()
		}
	}
	if itemID == "" {
		respondEphemeral(h.session, interaction, "Please specify an item to unattune from.")
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

	slots, err := character.ParseAttunementSlots(char.AttunementSlots.RawMessage, char.AttunementSlots.Valid)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to read attunement data. Please contact the DM.")
		return
	}

	result, err := inventory.Unattune(inventory.UnattuneInput{
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

	// SR-007: refresh #character-cards after the unattune write.
	notifyCardUpdate(ctx, h.cardUpdater, char.ID)

	respondEphemeral(h.session, interaction, result.Message)
}
