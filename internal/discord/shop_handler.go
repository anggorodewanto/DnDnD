package discord

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/shops"
)

// ShopBuyer purchases one unit of a shop item for a character, deducting gold
// and granting the item. Satisfied by *shops.Service.
type ShopBuyer interface {
	Buy(ctx context.Context, shopItemID, characterID uuid.UUID) (shops.BuyResult, error)
}

// ShopHandler handles the shop_buy button component callbacks.
type ShopHandler struct {
	session         Session
	campaignProv    InventoryCampaignProvider
	characterLookup InventoryCharacterLookup
	buyer           ShopBuyer
}

// NewShopHandler creates a new ShopHandler.
func NewShopHandler(
	session Session,
	campaignProv InventoryCampaignProvider,
	characterLookup InventoryCharacterLookup,
	buyer ShopBuyer,
) *ShopHandler {
	return &ShopHandler{
		session:         session,
		campaignProv:    campaignProv,
		characterLookup: characterLookup,
		buyer:           buyer,
	}
}

// ParseShopBuyData parses a "shop_buy:shopID:shopItemID" custom ID.
func ParseShopBuyData(customID string) (shopID, shopItemID uuid.UUID, err error) {
	parts := strings.SplitN(customID, ":", 3)
	if len(parts) != 3 {
		return uuid.Nil, uuid.Nil, fmt.Errorf("expected 3 parts, got %d", len(parts))
	}
	shopID, err = uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid shop ID: %w", err)
	}
	shopItemID, err = uuid.Parse(parts[2])
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid shop item ID: %w", err)
	}
	return shopID, shopItemID, nil
}

// HandleShopBuy processes a shop Buy button click: it resolves the clicker's
// character, purchases the item, and responds ephemerally with the result.
func (h *ShopHandler) HandleShopBuy(interaction *discordgo.Interaction) {
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

	customID := componentCustomID(interaction)
	_, shopItemID, err := ParseShopBuyData(customID)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Invalid shop button data: %v", err))
		return
	}

	result, err := h.buyer.Buy(ctx, shopItemID, char.ID)
	if err != nil {
		respondEphemeral(h.session, interaction, shopBuyErrorMessage(err))
		return
	}

	respondEphemeral(h.session, interaction, fmt.Sprintf(
		"\U0001f6d2 Bought **%s** for %d gp. Gold left: %d. (%d left in stock)",
		result.ItemName, result.PricePaid, result.GoldRemaining, result.StockRemaining,
	))
}

// shopBuyErrorMessage maps a Buy error to a friendly ephemeral message.
func shopBuyErrorMessage(err error) string {
	if errors.Is(err, shops.ErrInsufficientGold) {
		return "\U0001f4b8 You don't have enough gold for that."
	}
	if errors.Is(err, shops.ErrOutOfStock) {
		return "\U0001f4e6 That item is out of stock."
	}
	if errors.Is(err, shops.ErrShopItemNotFound) {
		return "❌ That item is no longer available."
	}
	return "Could not complete purchase. Please try again."
}

// componentCustomID extracts the custom ID from a message-component interaction.
func componentCustomID(interaction *discordgo.Interaction) string {
	data, ok := interaction.Data.(discordgo.MessageComponentInteractionData)
	if !ok {
		return ""
	}
	return data.CustomID
}
