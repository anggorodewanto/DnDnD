package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
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
// Implementations resolve the active turn for the invoking character (when in
// combat) and persist the per-turn resource flags after a successful /use.
// med-35: /use of a potion deducts a bonus action; magic-item active abilities
// deduct an action. When no turn is active (out of combat), no cost is taken.
type UseCombatProvider interface {
	GetActiveTurnForCharacter(ctx context.Context, guildID string, charID uuid.UUID) (refdata.Turn, bool, error)
	UpdateTurnActions(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error)
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
	cardUpdater     CardUpdater // SR-007
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

// SetCardUpdater wires the SR-007 character-card refresh callback. Fires
// after a successful /use write (consumable or magic-item charge).
func (h *UseHandler) SetCardUpdater(u CardUpdater) {
	h.cardUpdater = u
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

	// med-35: when a turn is active, deduct the appropriate combat resource
	// before mutating inventory. Potions cost a bonus action; everything
	// else (DM-adjudicated consumables) costs an action. Out-of-combat
	// /use carries no cost.
	turn, inCombat, costErr := h.lookupActiveTurn(ctx, interaction.GuildID, char.ID)
	if costErr != nil {
		respondEphemeral(h.session, interaction, "Failed to check turn state. Please try again.")
		return
	}
	if inCombat {
		resource := combat.ResourceAction
		if inventory.IsPotion(itemID) {
			resource = combat.ResourceBonusAction
		}
		if err := combat.ValidateResource(turn, resource); err != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot use item: %v", err))
			return
		}
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

	// SR-007: refresh #character-cards after a successful /use write.
	notifyCardUpdate(ctx, h.cardUpdater, char.ID)

	// med-35: persist the turn-resource deduction. Best-effort: a save
	// failure is logged but does not undo the committed inventory change.
	if inCombat {
		resource := combat.ResourceAction
		if inventory.IsPotion(itemID) {
			resource = combat.ResourceBonusAction
		}
		h.spendTurnResource(ctx, turn, resource)
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
		h.postConsumableToDMQueue(ctx, interaction.GuildID, campaign.ID.String(), char.Name, usedItemName)
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
// rules via inventory.UseCharges. The default amount is 1 charge. med-35:
// when in combat, an action is deducted from the active turn before the
// charge is spent.
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

	turn, inCombat, costErr := h.lookupActiveTurn(ctx, interaction.GuildID, char.ID)
	if costErr != nil {
		respondEphemeral(h.session, interaction, "Failed to check turn state. Please try again.")
		return
	}
	if inCombat {
		if err := combat.ValidateResource(turn, combat.ResourceAction); err != nil {
			respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot use item: %v", err))
			return
		}
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

	// SR-007: refresh #character-cards after a magic-item charge write.
	notifyCardUpdate(ctx, h.cardUpdater, char.ID)

	if inCombat {
		h.spendTurnResource(ctx, turn, combat.ResourceAction)
	}

	respondEphemeral(h.session, interaction, result.Message)
}

// lookupActiveTurn returns the active turn for the invoking character when a
// combat provider is wired. inCombat is false when no provider is configured
// or when the character has no active turn (out-of-combat /use). Errors from
// the provider are surfaced so callers can short-circuit.
func (h *UseHandler) lookupActiveTurn(ctx context.Context, guildID string, charID uuid.UUID) (refdata.Turn, bool, error) {
	if h.combatProv == nil {
		return refdata.Turn{}, false, nil
	}
	turn, ok, err := h.combatProv.GetActiveTurnForCharacter(ctx, guildID, charID)
	if err != nil {
		return refdata.Turn{}, false, err
	}
	return turn, ok, nil
}

// spendTurnResource marks resource as used on turn and persists the change.
// Best-effort: persistence failures are swallowed so an uncommitted turn
// flag never undoes a committed inventory mutation.
func (h *UseHandler) spendTurnResource(ctx context.Context, turn refdata.Turn, resource combat.ResourceType) {
	updated, err := combat.UseResource(turn, resource)
	if err != nil {
		return
	}
	_, _ = h.combatProv.UpdateTurnActions(ctx, combat.TurnToUpdateParams(updated))
}

// postConsumableToDMQueue dispatches a consumable-without-effect notification
// either through the dmqueue Notifier (preferred) or the legacy dmQueueFunc
// fallback. Both paths produce content containing the player and item names.
// SR-002: CampaignID is required by PgStore.Insert.
func (h *UseHandler) postConsumableToDMQueue(ctx context.Context, guildID, campaignID, charName, itemName string) {
	if h.notifier != nil {
		_, _ = h.notifier.Post(ctx, dmqueue.Event{
			Kind:       dmqueue.KindConsumable,
			PlayerName: charName,
			Summary:    fmt.Sprintf("uses %s", itemName),
			GuildID:    guildID,
			CampaignID: campaignID,
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
