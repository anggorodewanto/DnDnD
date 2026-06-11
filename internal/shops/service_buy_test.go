package shops_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/shops"
	"github.com/ab/dndnd/internal/testutil"
)

// createCharWithGold creates a character with the given gold and inventory,
// mirroring the loot package's createCharWithInventory helper.
func createCharWithGold(t *testing.T, q *refdata.Queries, campID uuid.UUID, name string, gold int32, items []character.InventoryItem) refdata.Character {
	t.Helper()
	char := testutil.NewTestCharacter(t, q, campID, name, 5)
	invJSON, _ := json.Marshal(items)
	updated, err := q.UpdateCharacterInventoryAndGold(context.Background(), refdata.UpdateCharacterInventoryAndGoldParams{
		ID:        char.ID,
		Inventory: pqtype.NullRawMessage{RawMessage: invJSON, Valid: true},
		Gold:      gold,
	})
	require.NoError(t, err)
	return updated
}

// --- TDD: Buy ---

func TestBuy_ShopItemNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	char := createCharWithGold(t, q, camp.ID, "Buyer", 100, nil)

	svc := shops.NewService(q)
	_, err := svc.Buy(context.Background(), uuid.New(), char.ID)
	assert.ErrorIs(t, err, shops.ErrShopItemNotFound)
}

func TestBuy_OutOfStock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	char := createCharWithGold(t, q, camp.ID, "Buyer", 100, nil)

	svc := shops.NewService(q)
	shop, err := svc.CreateShop(context.Background(), camp.ID, "Shop", "")
	require.NoError(t, err)
	item, err := svc.AddItem(context.Background(), shop.ID, refdata.CreateShopItemParams{
		Name: "Sold Out", PriceGp: 5, Quantity: 0, Type: "other",
	})
	require.NoError(t, err)

	_, err = svc.Buy(context.Background(), item.ID, char.ID)
	assert.ErrorIs(t, err, shops.ErrOutOfStock)
}

func TestBuy_InsufficientGold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	char := createCharWithGold(t, q, camp.ID, "Pauper", 5, nil)

	svc := shops.NewService(q)
	shop, err := svc.CreateShop(context.Background(), camp.ID, "Shop", "")
	require.NoError(t, err)
	item, err := svc.AddItem(context.Background(), shop.ID, refdata.CreateShopItemParams{
		Name: "Expensive", PriceGp: 50, Quantity: 3, Type: "other",
	})
	require.NoError(t, err)

	_, err = svc.Buy(context.Background(), item.ID, char.ID)
	assert.ErrorIs(t, err, shops.ErrInsufficientGold)
}

func TestBuy_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	char := createCharWithGold(t, q, camp.ID, "Buyer", 100, nil)

	svc := shops.NewService(q)
	shop, err := svc.CreateShop(context.Background(), camp.ID, "Shop", "")
	require.NoError(t, err)
	item, err := svc.AddItem(context.Background(), shop.ID, refdata.CreateShopItemParams{
		ItemID: "longsword", Name: "Longsword", PriceGp: 15, Quantity: 3, Type: "weapon",
	})
	require.NoError(t, err)

	result, err := svc.Buy(context.Background(), item.ID, char.ID)
	require.NoError(t, err)
	assert.Equal(t, "Longsword", result.ItemName)
	assert.Equal(t, int32(15), result.PricePaid)
	assert.Equal(t, int32(85), result.GoldRemaining)
	assert.Equal(t, int32(2), result.StockRemaining)

	// Gold deducted and item added to inventory in the DB.
	got, err := q.GetCharacter(context.Background(), char.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(85), got.Gold)
	items, err := character.ParseInventoryItems(got.Inventory.RawMessage, got.Inventory.Valid)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "longsword", items[0].ItemID)
	assert.Equal(t, 1, items[0].Quantity)

	// Shop stock decremented.
	gotItem, err := q.GetShopItem(context.Background(), item.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(2), gotItem.Quantity)
}

func TestBuy_ExactGold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	char := createCharWithGold(t, q, camp.ID, "Buyer", 15, nil)

	svc := shops.NewService(q)
	shop, err := svc.CreateShop(context.Background(), camp.ID, "Shop", "")
	require.NoError(t, err)
	item, err := svc.AddItem(context.Background(), shop.ID, refdata.CreateShopItemParams{
		ItemID: "rope", Name: "Rope", PriceGp: 15, Quantity: 1, Type: "other",
	})
	require.NoError(t, err)

	result, err := svc.Buy(context.Background(), item.ID, char.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), result.GoldRemaining)
	assert.Equal(t, int32(0), result.StockRemaining)
}

func TestBuy_StacksExistingInventoryItem(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	char := createCharWithGold(t, q, camp.ID, "Buyer", 100, []character.InventoryItem{
		{ItemID: "arrow", Name: "Arrow", Quantity: 5, Type: "ammunition"},
	})

	svc := shops.NewService(q)
	shop, err := svc.CreateShop(context.Background(), camp.ID, "Shop", "")
	require.NoError(t, err)
	item, err := svc.AddItem(context.Background(), shop.ID, refdata.CreateShopItemParams{
		ItemID: "arrow", Name: "Arrow", PriceGp: 1, Quantity: 10, Type: "ammunition",
	})
	require.NoError(t, err)

	_, err = svc.Buy(context.Background(), item.ID, char.ID)
	require.NoError(t, err)

	got, err := q.GetCharacter(context.Background(), char.ID)
	require.NoError(t, err)
	items, err := character.ParseInventoryItems(got.Inventory.RawMessage, got.Inventory.Valid)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, 6, items[0].Quantity)
}

// --- Mock error-branch tests ---

func TestBuy_GetCharacterError(t *testing.T) {
	itemID := uuid.New()
	store := &mockStore{
		getShopItemFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			return refdata.ShopItem{ID: itemID, Name: "Sword", PriceGp: 10, Quantity: 2}, nil
		},
		getCharacterFn: func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
			return refdata.Character{}, errDB
		},
	}
	svc := shops.NewService(store)
	_, err := svc.Buy(context.Background(), itemID, uuid.New())
	assert.Error(t, err)
}

func TestBuy_PersistError(t *testing.T) {
	itemID := uuid.New()
	store := &mockStore{
		getShopItemFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			return refdata.ShopItem{ID: itemID, Name: "Sword", PriceGp: 10, Quantity: 2}, nil
		},
		getCharacterFn: func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
			return refdata.Character{ID: uuid.New(), Gold: 100}, nil
		},
		updateCharacterInventoryAndGoldFn: func(_ context.Context, _ refdata.UpdateCharacterInventoryAndGoldParams) (refdata.Character, error) {
			return refdata.Character{}, errDB
		},
	}
	svc := shops.NewService(store)
	_, err := svc.Buy(context.Background(), itemID, uuid.New())
	assert.Error(t, err)
}

// TestBuy_StockDecrementError verifies the purchase is committed even when the
// best-effort stock decrement fails: the buyer was already charged and granted
// the item atomically, so Buy must return success (not an error that would
// prompt a double-charging retry).
func TestBuy_StockDecrementError(t *testing.T) {
	itemID := uuid.New()
	store := &mockStore{
		getShopItemFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			return refdata.ShopItem{ID: itemID, Name: "Sword", PriceGp: 10, Quantity: 2}, nil
		},
		getCharacterFn: func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
			return refdata.Character{ID: uuid.New(), Gold: 100}, nil
		},
		updateCharacterInventoryAndGoldFn: func(_ context.Context, _ refdata.UpdateCharacterInventoryAndGoldParams) (refdata.Character, error) {
			return refdata.Character{Gold: 90}, nil
		},
		updateShopItemFn: func(_ context.Context, _ refdata.UpdateShopItemParams) (refdata.ShopItem, error) {
			return refdata.ShopItem{}, errDB
		},
	}
	svc := shops.NewService(store)
	result, err := svc.Buy(context.Background(), itemID, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, int32(10), result.PricePaid)
	assert.Equal(t, int32(90), result.GoldRemaining)
	assert.Equal(t, int32(1), result.StockRemaining)
}
