package discord

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
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

	var items []character.InventoryItem
	if char.Inventory.Valid {
		_ = json.Unmarshal(char.Inventory.RawMessage, &items)
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
	invJSON, _ := json.Marshal(result.UpdatedItems)
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

	// Post to #dm-queue if item requires DM adjudication
	if result.DMQueueRequired && h.dmQueueFunc != nil {
		channelID := h.dmQueueFunc(interaction.GuildID)
		if channelID != "" {
			// Find the item name from original items
			usedItemName := itemID
			for _, it := range items {
				if it.ItemID == itemID {
					usedItemName = it.Name
					break
				}
			}
			dmMsg := fmt.Sprintf("🧪 **%s** used **%s** — needs DM adjudication.", char.Name, usedItemName)
			_, _ = h.session.ChannelMessageSend(channelID, dmMsg)
		}
	}

	respondEphemeral(h.session, interaction, result.Message)
}
