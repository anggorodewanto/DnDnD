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

// GiveCharacterStore persists inventory updates for both giver and receiver.
type GiveCharacterStore interface {
	UpdateCharacterInventory(ctx context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error)
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

	var giverItems []character.InventoryItem
	if giver.Inventory.Valid {
		_ = json.Unmarshal(giver.Inventory.RawMessage, &giverItems)
	}

	// Resolve target
	receiver, err := h.targetResolver.ResolveTarget(ctx, campaign.ID, targetStr)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Could not find target %q.", targetStr))
		return
	}

	var receiverItems []character.InventoryItem
	if receiver.Inventory.Valid {
		_ = json.Unmarshal(receiver.Inventory.RawMessage, &receiverItems)
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

	// Persist both inventories
	giverInvJSON, _ := json.Marshal(result.UpdatedGiverItems)
	receiverInvJSON, _ := json.Marshal(result.UpdatedReceiverItems)

	if _, err := h.store.UpdateCharacterInventory(ctx, refdata.UpdateCharacterInventoryParams{
		ID:        giver.ID,
		Inventory: pqtype.NullRawMessage{RawMessage: giverInvJSON, Valid: true},
	}); err != nil {
		respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
		return
	}
	if _, err := h.store.UpdateCharacterInventory(ctx, refdata.UpdateCharacterInventoryParams{
		ID:        receiver.ID,
		Inventory: pqtype.NullRawMessage{RawMessage: receiverInvJSON, Valid: true},
	}); err != nil {
		respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
		return
	}

	respondEphemeral(h.session, interaction, result.Message)
}
