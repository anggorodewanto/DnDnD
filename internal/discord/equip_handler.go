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

// EquipCharacterStore persists inventory updates for the equip command.
type EquipCharacterStore interface {
	UpdateCharacterInventory(ctx context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error)
}

// EquipHandler handles the /equip slash command.
type EquipHandler struct {
	session         Session
	campaignProv    InventoryCampaignProvider
	characterLookup InventoryCharacterLookup
	store           EquipCharacterStore
}

// NewEquipHandler creates a new EquipHandler.
func NewEquipHandler(
	session Session,
	campaignProv InventoryCampaignProvider,
	characterLookup InventoryCharacterLookup,
	store EquipCharacterStore,
) *EquipHandler {
	return &EquipHandler{
		session:         session,
		campaignProv:    campaignProv,
		characterLookup: characterLookup,
		store:           store,
	}
}

// Handle processes the /equip command interaction.
func (h *EquipHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	var itemID string
	var offhand, armor bool
	for _, opt := range data.Options {
		switch opt.Name {
		case "item":
			itemID = opt.StringValue()
		case "offhand":
			offhand = opt.BoolValue()
		case "armor":
			armor = opt.BoolValue()
		}
	}
	if itemID == "" {
		respondEphemeral(h.session, interaction, "Please specify an item to equip.")
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

	result, err := inventory.Equip(inventory.EquipInput{
		Items:           items,
		ItemID:          itemID,
		OffHand:         offhand,
		Armor:           armor,
		AttunementSlots: slots,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("%v", err))
		return
	}

	invJSON, err := json.Marshal(result.UpdatedItems)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to save equipment changes. Please try again.")
		return
	}

	if _, err := h.store.UpdateCharacterInventory(ctx, refdata.UpdateCharacterInventoryParams{
		ID:        char.ID,
		Inventory: pqtype.NullRawMessage{RawMessage: invJSON, Valid: true},
	}); err != nil {
		respondEphemeral(h.session, interaction, "Failed to save equipment changes. Please try again.")
		return
	}

	msg := result.Message
	if result.Warning != "" {
		msg += "\n" + result.Warning
	}
	respondEphemeral(h.session, interaction, msg)
}
