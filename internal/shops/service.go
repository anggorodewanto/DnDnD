package shops

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/inventory"
	"github.com/ab/dndnd/internal/refdata"
)

// Errors returned by the shop service.
var (
	ErrShopNotFound     = errors.New("shop not found")
	ErrNameRequired     = errors.New("shop name is required")
	ErrShopItemNotFound = errors.New("shop item not found")
	ErrOutOfStock       = errors.New("shop item is out of stock")
	ErrInsufficientGold = errors.New("insufficient gold")
)

// Store defines the database operations needed by the shop service.
type Store interface {
	CreateShop(ctx context.Context, arg refdata.CreateShopParams) (refdata.Shop, error)
	GetShop(ctx context.Context, id uuid.UUID) (refdata.Shop, error)
	ListShopsByCampaign(ctx context.Context, campaignID uuid.UUID) ([]refdata.Shop, error)
	UpdateShop(ctx context.Context, arg refdata.UpdateShopParams) (refdata.Shop, error)
	DeleteShop(ctx context.Context, id uuid.UUID) error
	CreateShopItem(ctx context.Context, arg refdata.CreateShopItemParams) (refdata.ShopItem, error)
	GetShopItem(ctx context.Context, id uuid.UUID) (refdata.ShopItem, error)
	ListShopItems(ctx context.Context, shopID uuid.UUID) ([]refdata.ShopItem, error)
	UpdateShopItem(ctx context.Context, arg refdata.UpdateShopItemParams) (refdata.ShopItem, error)
	DeleteShopItem(ctx context.Context, id uuid.UUID) error
	DeleteShopItemsByShop(ctx context.Context, shopID uuid.UUID) error
	GetCampaignByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	UpdateCharacterInventoryAndGold(ctx context.Context, arg refdata.UpdateCharacterInventoryAndGoldParams) (refdata.Character, error)
}

// BuyResult summarizes a successful purchase.
type BuyResult struct {
	ItemName       string
	PricePaid      int32
	GoldRemaining  int32
	StockRemaining int32
}

// ShopResult holds a shop and its items.
type ShopResult struct {
	Shop  refdata.Shop       `json:"shop"`
	Items []refdata.ShopItem `json:"items"`
}

// Service manages shop operations.
type Service struct {
	store Store
}

// NewService creates a new shop Service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// CreateShop creates a new shop template for a campaign.
func (s *Service) CreateShop(ctx context.Context, campaignID uuid.UUID, name, description string) (refdata.Shop, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return refdata.Shop{}, ErrNameRequired
	}
	return s.store.CreateShop(ctx, refdata.CreateShopParams{
		CampaignID:  campaignID,
		Name:        name,
		Description: description,
	})
}

// GetShop returns a shop with all its items.
func (s *Service) GetShop(ctx context.Context, shopID uuid.UUID) (ShopResult, error) {
	shop, err := s.store.GetShop(ctx, shopID)
	if err != nil {
		return ShopResult{}, ErrShopNotFound
	}
	items, err := s.store.ListShopItems(ctx, shopID)
	if err != nil {
		return ShopResult{}, fmt.Errorf("listing shop items: %w", err)
	}
	return ShopResult{Shop: shop, Items: items}, nil
}

// ListShops returns all shops for a campaign.
func (s *Service) ListShops(ctx context.Context, campaignID uuid.UUID) ([]refdata.Shop, error) {
	return s.store.ListShopsByCampaign(ctx, campaignID)
}

// UpdateShop updates a shop's name and description.
func (s *Service) UpdateShop(ctx context.Context, shopID uuid.UUID, name, description string) (refdata.Shop, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return refdata.Shop{}, ErrNameRequired
	}
	if _, err := s.store.GetShop(ctx, shopID); err != nil {
		return refdata.Shop{}, ErrShopNotFound
	}
	return s.store.UpdateShop(ctx, refdata.UpdateShopParams{
		ID:          shopID,
		Name:        name,
		Description: description,
	})
}

// DeleteShop deletes a shop and all its items.
func (s *Service) DeleteShop(ctx context.Context, shopID uuid.UUID) error {
	if _, err := s.store.GetShop(ctx, shopID); err != nil {
		return ErrShopNotFound
	}
	return s.store.DeleteShop(ctx, shopID)
}

// AddItem adds an item to a shop.
func (s *Service) AddItem(ctx context.Context, shopID uuid.UUID, item refdata.CreateShopItemParams) (refdata.ShopItem, error) {
	if _, err := s.store.GetShop(ctx, shopID); err != nil {
		return refdata.ShopItem{}, ErrShopNotFound
	}
	item.ShopID = shopID
	return s.store.CreateShopItem(ctx, item)
}

// UpdateItem updates a shop item's details.
func (s *Service) UpdateItem(ctx context.Context, arg refdata.UpdateShopItemParams) (refdata.ShopItem, error) {
	return s.store.UpdateShopItem(ctx, arg)
}

// RemoveItem removes an item from a shop.
func (s *Service) RemoveItem(ctx context.Context, itemID uuid.UUID) error {
	return s.store.DeleteShopItem(ctx, itemID)
}

// GetCampaign returns a campaign by ID (used by the handler for Discord posting).
func (s *Service) GetCampaign(ctx context.Context, campaignID uuid.UUID) (refdata.Campaign, error) {
	return s.store.GetCampaignByID(ctx, campaignID)
}

// Buy purchases one unit of a shop item for a character: it deducts the price
// from the character's gold, adds the item to their inventory, and decrements
// the shop's stock. The gold deduction and inventory grant are persisted
// atomically; the stock decrement follows. Mirrors loot.ClaimItem's inventory
// grant pattern.
func (s *Service) Buy(ctx context.Context, shopItemID uuid.UUID, characterID uuid.UUID) (BuyResult, error) {
	item, err := s.store.GetShopItem(ctx, shopItemID)
	if err != nil {
		return BuyResult{}, ErrShopItemNotFound
	}
	if item.Quantity <= 0 {
		return BuyResult{}, ErrOutOfStock
	}

	char, err := s.store.GetCharacter(ctx, characterID)
	if err != nil {
		return BuyResult{}, fmt.Errorf("getting character: %w", err)
	}
	if char.Gold < item.PriceGp {
		return BuyResult{}, ErrInsufficientGold
	}

	newGold := char.Gold - item.PriceGp

	items, err := character.ParseInventoryItems(char.Inventory.RawMessage, char.Inventory.Valid)
	if err != nil {
		return BuyResult{}, fmt.Errorf("parsing inventory: %w", err)
	}

	itemIDStr := item.ItemID
	if itemIDStr == "" {
		itemIDStr = "custom-" + shopItemID.String()
	}
	bought := character.InventoryItem{
		ItemID:   itemIDStr,
		Name:     item.Name,
		Quantity: 1,
		Type:     item.Type,
	}
	items = inventory.AddItemQuantity(items, bought, 1)

	invJSON, err := character.MarshalInventory(items)
	if err != nil {
		return BuyResult{}, fmt.Errorf("marshaling inventory: %w", err)
	}

	if _, err := s.store.UpdateCharacterInventoryAndGold(ctx, refdata.UpdateCharacterInventoryAndGoldParams{
		ID:        characterID,
		Inventory: pqtype.NullRawMessage{RawMessage: invJSON, Valid: true},
		Gold:      newGold,
	}); err != nil {
		return BuyResult{}, fmt.Errorf("updating character: %w", err)
	}

	// Stock decrement is best-effort: the buyer has already been charged and
	// granted the item atomically above, so the purchase is committed. A
	// stock-update failure must NOT surface as an error here — returning one
	// would prompt a retry that double-charges the buyer. Worst case the shop
	// oversells by one unit until the next successful update.
	newStock := item.Quantity - 1
	_, _ = s.store.UpdateShopItem(ctx, refdata.UpdateShopItemParams{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		PriceGp:     item.PriceGp,
		Quantity:    newStock,
	})

	return BuyResult{
		ItemName:       item.Name,
		PricePaid:      item.PriceGp,
		GoldRemaining:  newGold,
		StockRemaining: newStock,
	}, nil
}

// FormatShopAnnouncement formats a shop as a Discord message.
func FormatShopAnnouncement(shop refdata.Shop, items []refdata.ShopItem) string {
	if len(items) == 0 {
		return fmt.Sprintf("**%s**\n_No items available._", shop.Name)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "**%s**", shop.Name)
	if shop.Description != "" {
		fmt.Fprintf(&b, "\n_%s_", shop.Description)
	}
	b.WriteString("\n\n")

	for _, item := range items {
		line := fmt.Sprintf("- **%s**", item.Name)
		if item.PriceGp > 0 {
			line += fmt.Sprintf(" — %d gp", item.PriceGp)
		}
		if item.Description != "" {
			line += fmt.Sprintf(" _%s_", item.Description)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}
