package discord

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/inventory"
	"github.com/ab/dndnd/internal/refdata"
)

// AttuneCharacterStore persists attunement slot updates.
type AttuneCharacterStore interface {
	UpdateCharacterAttunementSlots(ctx context.Context, arg refdata.UpdateCharacterAttunementSlotsParams) (refdata.Character, error)
}

// AttunePublisher refreshes the dashboard encounter snapshot for a character
// whose magic-item state just changed (H-104b / F-9). The handler depends on
// this minimal interface so it does not need to import internal/magicitem.
// Implementations (e.g. *magicitem.Service) must silently no-op when the
// character is not currently in an active encounter.
type AttunePublisher interface {
	PublishForCharacter(ctx context.Context, characterID uuid.UUID)
}

// AttuneHandler handles the /attune slash command.
type AttuneHandler struct {
	session         Session
	campaignProv    InventoryCampaignProvider
	characterLookup InventoryCharacterLookup
	store           AttuneCharacterStore
	publisher       AttunePublisher
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

// SetPublisher wires the optional dashboard publisher (F-9). A nil publisher
// is tolerated and disables fan-out — Handle simply skips the call.
func (h *AttuneHandler) SetPublisher(p AttunePublisher) {
	h.publisher = p
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

	var classes []character.ClassEntry
	if len(char.Classes) > 0 {
		_ = json.Unmarshal(char.Classes, &classes)
	}

	// Find the item to get its attunement restriction
	var attunementRestriction string
	for _, item := range items {
		if item.ItemID == itemID {
			attunementRestriction = item.AttunementRestriction
			break
		}
	}

	result, err := inventory.Attune(inventory.AttuneInput{
		Items:                 items,
		Slots:                 slots,
		ItemID:                itemID,
		Classes:               classes,
		AttunementRestriction: attunementRestriction,
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

	// F-9: fan out a fresh encounter snapshot if the character is currently
	// in combat (publisher silently no-ops otherwise).
	if h.publisher != nil {
		h.publisher.PublishForCharacter(ctx, char.ID)
	}

	respondEphemeral(h.session, interaction, result.Message)
}
