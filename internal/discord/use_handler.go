package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/inventory"
	"github.com/ab/dndnd/internal/refdata"
)

// UseCharacterStore persists inventory/HP updates after using an item.
type UseCharacterStore interface {
	UpdateCharacterInventoryAndHP(ctx context.Context, arg refdata.UpdateCharacterInventoryAndHPParams) (refdata.Character, error)
	UpdateCharacterInventory(ctx context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error)
}

// UseCombatProvider provides combat state for action-cost validation.
type UseCombatProvider interface {
	GetActiveTurnForCharacter(ctx context.Context, guildID string, charID uuid.UUID) (refdata.Turn, bool, error)
}

// UseHandler handles the /use slash command.
type UseHandler struct {
	session         Session
	campaignProv    InventoryCampaignProvider
	characterLookup InventoryCharacterLookup
	store           UseCharacterStore
	invService      *inventory.Service
	combatProv      UseCombatProvider
	dmQueueFunc     func(guildID string) string
	notifier        dmqueue.Notifier
}

// NewUseHandler creates a new UseHandler.
func NewUseHandler(
	session Session,
	campaignProv InventoryCampaignProvider,
	characterLookup InventoryCharacterLookup,
	store UseCharacterStore,
	randFn dice.RandSource,
	combatProv UseCombatProvider,
) *UseHandler {
	return &UseHandler{
		session:         session,
		campaignProv:    campaignProv,
		characterLookup: characterLookup,
		store:           store,
		invService:      inventory.NewService(randFn),
		combatProv:      combatProv,
	}
}

// SetDMQueueFunc sets the function that resolves a guild ID to a #dm-queue channel ID.
func (h *UseHandler) SetDMQueueFunc(fn func(guildID string) string) {
	h.dmQueueFunc = fn
}

// SetNotifier wires the dm-queue Notifier. When set, consumable-without-effect
// posts route through the unified dmqueue framework instead of the legacy
// dmQueueFunc path.
func (h *UseHandler) SetNotifier(n dmqueue.Notifier) {
	h.notifier = n
}

// Handle processes the /use command interaction.
func (h *UseHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	itemID := ""
	for _, opt := range data.Options {
		if opt.Name == "item" {
			itemID = opt.StringValue()
		}
	}
	if itemID == "" {
		respondEphemeral(h.session, interaction, "Please specify an item to use.")
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

	// Magic items with charges short-circuit to the active-ability path.
	// Consumables fall through to UseConsumable below.
	if itemHasActiveCharges(items, itemID) {
		h.handleMagicItemCharge(ctx, interaction, char, items, itemID)
		return
	}

	result, err := h.invService.UseConsumable(inventory.UseInput{
		Items:     items,
		ItemID:    itemID,
		HPCurrent: int(char.HpCurrent),
		HPMax:     int(char.HpMax),
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot use item: %v", err))
		return
	}

	// Persist changes
	invJSON, err := character.MarshalInventory(result.UpdatedItems)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
		return
	}
	invMsg := pqtype.NullRawMessage{RawMessage: invJSON, Valid: true}

	if result.HealingDone > 0 {
		if _, err := h.store.UpdateCharacterInventoryAndHP(ctx, refdata.UpdateCharacterInventoryAndHPParams{
			ID:        char.ID,
			Inventory: invMsg,
			HpCurrent: int32(result.HPAfter),
		}); err != nil {
			respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
			return
		}
	} else {
		if _, err := h.store.UpdateCharacterInventory(ctx, refdata.UpdateCharacterInventoryParams{
			ID:        char.ID,
			Inventory: invMsg,
		}); err != nil {
			respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
			return
		}
	}

	// Post to #dm-queue if item requires DM adjudication.
	if result.DMQueueRequired {
		usedItemName := itemID
		for _, it := range items {
			if it.ItemID == itemID {
				usedItemName = it.Name
				break
			}
		}
		h.postConsumableToDMQueue(ctx, interaction.GuildID, char.Name, usedItemName)
	}

	respondEphemeral(h.session, interaction, result.Message)
}

// itemHasActiveCharges reports whether the inventory item with itemID is a
// magic item with non-zero MaxCharges (i.e. an active-ability target).
func itemHasActiveCharges(items []character.InventoryItem, itemID string) bool {
	for _, it := range items {
		if it.ItemID != itemID {
			continue
		}
		return it.IsMagic && it.MaxCharges > 0
	}
	return false
}

// handleMagicItemCharge consumes one charge from the named magic item and
// persists the updated inventory. It enforces attunement and charge-balance
// rules via inventory.UseCharges. The default amount is 1 charge.
func (h *UseHandler) handleMagicItemCharge(
	ctx context.Context,
	interaction *discordgo.Interaction,
	char refdata.Character,
	items []character.InventoryItem,
	itemID string,
) {
	attunement, err := character.ParseAttunementSlots(char.AttunementSlots.RawMessage, char.AttunementSlots.Valid)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to read attunement data. Please contact the DM.")
		return
	}

	result, err := inventory.UseCharges(inventory.UseChargesInput{
		Items:      items,
		Attunement: attunement,
		ItemID:     itemID,
		Amount:     1,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot use item: %v", err))
		return
	}

	invJSON, err := character.MarshalInventory(result.UpdatedItems)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
		return
	}

	if _, err := h.store.UpdateCharacterInventory(ctx, refdata.UpdateCharacterInventoryParams{
		ID:        char.ID,
		Inventory: pqtype.NullRawMessage{RawMessage: invJSON, Valid: true},
	}); err != nil {
		respondEphemeral(h.session, interaction, "Failed to save inventory changes. Please try again.")
		return
	}

	respondEphemeral(h.session, interaction, result.Message)
}

// postConsumableToDMQueue dispatches a consumable-without-effect notification
// either through the dmqueue Notifier (preferred) or the legacy dmQueueFunc
// fallback. Both paths produce content containing the player and item names.
func (h *UseHandler) postConsumableToDMQueue(ctx context.Context, guildID, charName, itemName string) {
	if h.notifier != nil {
		_, _ = h.notifier.Post(ctx, dmqueue.Event{
			Kind:       dmqueue.KindConsumable,
			PlayerName: charName,
			Summary:    fmt.Sprintf("uses %s", itemName),
			GuildID:    guildID,
		})
		return
	}
	if h.dmQueueFunc == nil {
		return
	}
	channelID := h.dmQueueFunc(guildID)
	if channelID == "" {
		return
	}
	event := dmqueue.Event{
		Kind:        dmqueue.KindConsumable,
		PlayerName:  charName,
		Summary:     fmt.Sprintf("uses %s", itemName),
		ResolvePath: "#", // legacy path has no dashboard item ID
	}
	_, _ = h.session.ChannelMessageSend(channelID, dmqueue.FormatEvent(event))
}
