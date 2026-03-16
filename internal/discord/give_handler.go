package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/inventory"
	"github.com/ab/dndnd/internal/refdata"
)

// GiveCharacterStore persists inventory updates for both giver and receiver atomically.
type GiveCharacterStore interface {
	UpdateTwoCharacterInventories(ctx context.Context, id1 uuid.UUID, inv1 pqtype.NullRawMessage, id2 uuid.UUID, inv2 pqtype.NullRawMessage) error
}

// GiveTargetResolver resolves a name or ID to a character in the campaign.
type GiveTargetResolver interface {
	ResolveTarget(ctx context.Context, campaignID uuid.UUID, nameOrID string) (refdata.Character, error)
}

// GiveCombatProvider provides combat state for adjacency/resource checks.
type GiveCombatProvider interface {
	GetCombatantsForGuild(ctx context.Context, guildID string) ([]refdata.Combatant, bool, error)
}

// GiveHandler handles the /give slash command.
type GiveHandler struct {
	session         Session
	campaignProv    InventoryCampaignProvider
	characterLookup InventoryCharacterLookup
	targetResolver  GiveTargetResolver
	store           GiveCharacterStore
	combatProv      GiveCombatProvider
}

// NewGiveHandler creates a new GiveHandler.
func NewGiveHandler(
	session Session,
	campaignProv InventoryCampaignProvider,
	characterLookup InventoryCharacterLookup,
	targetResolver GiveTargetResolver,
	store GiveCharacterStore,
	combatProv GiveCombatProvider,
) *GiveHandler {
	return &GiveHandler{
		session:         session,
		campaignProv:    campaignProv,
		characterLookup: characterLookup,
		targetResolver:  targetResolver,
		store:           store,
		combatProv:      combatProv,
	}
}

// Handle processes the /give command interaction.
func (h *GiveHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	var itemID, targetStr string
	for _, opt := range data.Options {
		switch opt.Name {
		case "item":
			itemID = opt.StringValue()
		case "target":
			targetStr = opt.StringValue()
		}
	}

	campaign, err := h.campaignProv.GetCampaignByGuildID(ctx, interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	userID := discordUserID(interaction)
	giver, err := h.characterLookup.GetCharacterByCampaignAndDiscord(ctx, campaign.ID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not find your character. Use `/register` first.")
		return
	}

	giverItems, err := character.ParseInventoryItems(giver.Inventory.RawMessage, giver.Inventory.Valid)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to read inventory. Please contact the DM.")
		return
	}

	// Resolve target
	receiver, err := h.targetResolver.ResolveTarget(ctx, campaign.ID, targetStr)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Could not find target %q.", targetStr))
		return
	}

	receiverItems, err := character.ParseInventoryItems(receiver.Inventory.RawMessage, receiver.Inventory.Valid)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to read target inventory. Please contact the DM.")
		return
	}

	result, err := inventory.GiveItem(inventory.GiveInput{
		GiverItems:    giverItems,
		ReceiverItems: receiverItems,
		ItemID:        itemID,
		GiverName:     giver.Name,
		ReceiverName:  receiver.Name,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot give item: %v", err))
		return
	}

	// Persist both inventories atomically
	giverInvJSON, err := character.MarshalInventory(result.UpdatedGiverItems)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
		return
	}
	receiverInvJSON, err := character.MarshalInventory(result.UpdatedReceiverItems)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
		return
	}

	if err := h.store.UpdateTwoCharacterInventories(ctx, giver.ID,
		pqtype.NullRawMessage{RawMessage: giverInvJSON, Valid: true},
		receiver.ID,
		pqtype.NullRawMessage{RawMessage: receiverInvJSON, Valid: true},
	); err != nil {
		respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
		return
	}

	respondEphemeral(h.session, interaction, result.Message)
}
