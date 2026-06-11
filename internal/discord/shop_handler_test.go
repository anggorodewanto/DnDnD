package discord

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/shops"
)

// mockShopBuyer is a stand-in for the shops.Service Buy method.
type mockShopBuyer struct {
	result        shops.BuyResult
	err           error
	gotShopItemID uuid.UUID
	gotCharID     uuid.UUID
}

func (m *mockShopBuyer) Buy(_ context.Context, shopItemID, characterID uuid.UUID) (shops.BuyResult, error) {
	m.gotShopItemID = shopItemID
	m.gotCharID = characterID
	return m.result, m.err
}

func makeShopBuyInteraction(guildID, userID, customID string) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: guildID,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: userID},
		},
		Data: discordgo.MessageComponentInteractionData{CustomID: customID},
	}
}

func TestParseShopBuyData_Valid(t *testing.T) {
	shopID := uuid.New()
	itemID := uuid.New()
	gotShop, gotItem, err := ParseShopBuyData(fmt.Sprintf("shop_buy:%s:%s", shopID, itemID))
	require.NoError(t, err)
	assert.Equal(t, shopID, gotShop)
	assert.Equal(t, itemID, gotItem)
}

func TestParseShopBuyData_WrongPartCount(t *testing.T) {
	_, _, err := ParseShopBuyData("shop_buy:only-one")
	assert.Error(t, err)
}

func TestParseShopBuyData_BadShopID(t *testing.T) {
	_, _, err := ParseShopBuyData(fmt.Sprintf("shop_buy:not-a-uuid:%s", uuid.New()))
	assert.Error(t, err)
}

func TestParseShopBuyData_BadItemID(t *testing.T) {
	_, _, err := ParseShopBuyData(fmt.Sprintf("shop_buy:%s:not-a-uuid", uuid.New()))
	assert.Error(t, err)
}

func TestShopHandler_NoCampaign(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewShopHandler(
		sess,
		&mockInventoryCampaignProvider{err: errors.New("not found")},
		&mockInventoryCharacterLookup{},
		&mockShopBuyer{},
	)

	i := makeShopBuyInteraction("guild1", "user1", fmt.Sprintf("shop_buy:%s:%s", uuid.New(), uuid.New()))
	handler.HandleShopBuy(i)

	assert.Contains(t, sess.lastResponse, "No campaign")
}

func TestShopHandler_NoCharacter(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	handler := NewShopHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{err: errors.New("not found")},
		&mockShopBuyer{},
	)

	i := makeShopBuyInteraction("guild1", "user1", fmt.Sprintf("shop_buy:%s:%s", uuid.New(), uuid.New()))
	handler.HandleShopBuy(i)

	assert.Contains(t, sess.lastResponse, "Could not find your character")
}

func TestShopHandler_BadCustomID(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()
	handler := NewShopHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{ID: charID, CampaignID: campID}},
		&mockShopBuyer{},
	)

	i := makeShopBuyInteraction("guild1", "user1", "shop_buy:garbage")
	handler.HandleShopBuy(i)

	assert.Contains(t, sess.lastResponse, "Invalid")
}

func TestShopHandler_Success(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()
	shopID := uuid.New()
	itemID := uuid.New()
	buyer := &mockShopBuyer{result: shops.BuyResult{
		ItemName:       "Longsword",
		PricePaid:      15,
		GoldRemaining:  85,
		StockRemaining: 2,
	}}
	handler := NewShopHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{ID: charID, CampaignID: campID}},
		buyer,
	)

	i := makeShopBuyInteraction("guild1", "user1", fmt.Sprintf("shop_buy:%s:%s", shopID, itemID))
	handler.HandleShopBuy(i)

	assert.Equal(t, itemID, buyer.gotShopItemID)
	assert.Equal(t, charID, buyer.gotCharID)
	assert.Contains(t, sess.lastResponse, "Longsword")
	assert.Contains(t, sess.lastResponse, "15 gp")
	assert.Contains(t, sess.lastResponse, "85")
	assert.Contains(t, sess.lastResponse, "2 left")
}

func TestShopHandler_InsufficientGold(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()
	buyer := &mockShopBuyer{err: shops.ErrInsufficientGold}
	handler := NewShopHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{ID: charID, CampaignID: campID}},
		buyer,
	)

	i := makeShopBuyInteraction("guild1", "user1", fmt.Sprintf("shop_buy:%s:%s", uuid.New(), uuid.New()))
	handler.HandleShopBuy(i)

	assert.Contains(t, sess.lastResponse, "enough gold")
}

func TestShopHandler_OutOfStock(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()
	buyer := &mockShopBuyer{err: shops.ErrOutOfStock}
	handler := NewShopHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{ID: charID, CampaignID: campID}},
		buyer,
	)

	i := makeShopBuyInteraction("guild1", "user1", fmt.Sprintf("shop_buy:%s:%s", uuid.New(), uuid.New()))
	handler.HandleShopBuy(i)

	assert.Contains(t, sess.lastResponse, "out of stock")
}

func TestShopHandler_ItemNotFound(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()
	buyer := &mockShopBuyer{err: shops.ErrShopItemNotFound}
	handler := NewShopHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{ID: charID, CampaignID: campID}},
		buyer,
	)

	i := makeShopBuyInteraction("guild1", "user1", fmt.Sprintf("shop_buy:%s:%s", uuid.New(), uuid.New()))
	handler.HandleShopBuy(i)

	assert.Contains(t, sess.lastResponse, "no longer available")
}

func TestShopHandler_WiredViaRouter(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()
	buyer := &mockShopBuyer{result: shops.BuyResult{
		ItemName: "Rope", PricePaid: 1, GoldRemaining: 99, StockRemaining: 4,
	}}
	handler := NewShopHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{ID: charID, CampaignID: campID}},
		buyer,
	)

	bot := &Bot{session: sess}
	router := NewCommandRouter(bot, nil)
	router.SetShopHandler(handler)

	i := makeShopBuyInteraction("G1", "user1", fmt.Sprintf("shop_buy:%s:%s", uuid.New(), uuid.New()))
	router.Handle(i)

	assert.Contains(t, sess.lastResponse, "Rope")
}

func TestShopHandler_GenericError(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()
	buyer := &mockShopBuyer{err: errors.New("db exploded")}
	handler := NewShopHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{ID: charID, CampaignID: campID}},
		buyer,
	)

	i := makeShopBuyInteraction("guild1", "user1", fmt.Sprintf("shop_buy:%s:%s", uuid.New(), uuid.New()))
	handler.HandleShopBuy(i)

	assert.Contains(t, sess.lastResponse, "Could not complete purchase")
}
