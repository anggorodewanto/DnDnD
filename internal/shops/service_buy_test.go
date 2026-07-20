package shops_test

import (
	"context"
	"database/sql"
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

	// The reservation taken before the (failed) charge must be given back.
	gotItem, err := q.GetShopItem(context.Background(), item.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(3), gotItem.Quantity, "a failed purchase must not consume stock")
}

// TestBuy_LastUnitRejectedByDB exercises the real `quantity > 0` guard: once
// the shelf is empty the reservation statement matches no row.
func TestBuy_LastUnitRejectedByDB(t *testing.T) {
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
		ItemID: "potion", Name: "Last Potion", PriceGp: 10, Quantity: 1, Type: "other",
	})
	require.NoError(t, err)

	first, err := svc.Buy(context.Background(), item.ID, char.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), first.StockRemaining)

	_, err = svc.Buy(context.Background(), item.ID, char.ID)
	assert.ErrorIs(t, err, shops.ErrOutOfStock)

	// Charged exactly once, and stock never went negative.
	got, err := q.GetCharacter(context.Background(), char.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(90), got.Gold)
	gotItem, err := q.GetShopItem(context.Background(), item.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), gotItem.Quantity)
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

// raceStore is a stateful mock whose stock and gold writes enforce the same
// guards as the real SQL (quantity > 0, gold >= price) while GetShopItem /
// GetCharacter keep serving the PRE-purchase snapshot. That models two buyers
// (or two clicks on the still-live shop_buy button) reading before either
// write lands: a Buy that trusts its own read will oversell, an atomic one
// cannot.
type raceStore struct {
	staleItem refdata.ShopItem
	staleChar refdata.Character

	stock int32
	gold  int32

	goldWrites   int
	restoreCalls int
	reserveErr   error
	goldErr      error
}

func (r *raceStore) store() *mockStore {
	return &mockStore{
		getShopItemFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			return r.staleItem, nil
		},
		getCharacterFn: func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
			return r.staleChar, nil
		},
		reserveShopItemStockFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			if r.reserveErr != nil {
				return refdata.ShopItem{}, r.reserveErr
			}
			if r.stock <= 0 {
				return refdata.ShopItem{}, sql.ErrNoRows
			}
			r.stock--
			reserved := r.staleItem
			reserved.Quantity = r.stock
			return reserved, nil
		},
		restoreShopItemStockFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			r.restoreCalls++
			r.stock++
			restored := r.staleItem
			restored.Quantity = r.stock
			return restored, nil
		},
		deductCharacterGoldFn: func(_ context.Context, arg refdata.DeductCharacterGoldAndSetInventoryParams) (refdata.Character, error) {
			if r.goldErr != nil {
				return refdata.Character{}, r.goldErr
			}
			if r.gold < arg.PriceGp {
				return refdata.Character{}, sql.ErrNoRows
			}
			r.goldWrites++
			r.gold -= arg.PriceGp
			charged := r.staleChar
			charged.Gold = r.gold
			charged.Inventory = arg.Inventory
			return charged, nil
		},
	}
}

// TestBuy_LastUnitCannotBeOversold is the concurrency regression: both buyers
// read quantity 1, so only the atomic reservation can reject the second.
func TestBuy_LastUnitCannotBeOversold(t *testing.T) {
	itemID := uuid.New()
	race := &raceStore{
		staleItem: refdata.ShopItem{ID: itemID, ItemID: "potion", Name: "Last Potion", PriceGp: 10, Quantity: 1, Type: "other"},
		staleChar: refdata.Character{ID: uuid.New(), Gold: 100},
		stock:     1,
		gold:      100,
	}
	svc := shops.NewService(race.store())

	first, err := svc.Buy(context.Background(), itemID, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, int32(0), first.StockRemaining)

	_, err = svc.Buy(context.Background(), itemID, uuid.New())
	assert.ErrorIs(t, err, shops.ErrOutOfStock)
	assert.Equal(t, 1, race.goldWrites, "sold-out buy must not charge gold or write inventory")
	assert.Equal(t, int32(0), race.stock, "stock must not go negative")
}

// TestBuy_InsufficientGoldRestoresStock covers the second half of the race: the
// reservation succeeds but the buyer cannot pay, so the unit goes back on the
// shelf instead of vanishing.
func TestBuy_InsufficientGoldRestoresStock(t *testing.T) {
	itemID := uuid.New()
	race := &raceStore{
		staleItem: refdata.ShopItem{ID: itemID, ItemID: "potion", Name: "Potion", PriceGp: 10, Quantity: 2, Type: "other"},
		staleChar: refdata.Character{ID: uuid.New(), Gold: 10},
		stock:     2,
		gold:      10,
	}
	svc := shops.NewService(race.store())

	_, err := svc.Buy(context.Background(), itemID, uuid.New())
	require.NoError(t, err)

	_, err = svc.Buy(context.Background(), itemID, uuid.New())
	assert.ErrorIs(t, err, shops.ErrInsufficientGold)
	assert.Equal(t, 1, race.restoreCalls, "unaffordable buy must return its reserved unit")
	assert.Equal(t, int32(1), race.stock)
	assert.Equal(t, 1, race.goldWrites)
}

// TestBuy_GoldWriteErrorRestoresStock verifies a DB failure on the charge does
// not silently destroy the reserved unit.
func TestBuy_GoldWriteErrorRestoresStock(t *testing.T) {
	itemID := uuid.New()
	race := &raceStore{
		staleItem: refdata.ShopItem{ID: itemID, Name: "Sword", PriceGp: 10, Quantity: 2, Type: "weapon"},
		staleChar: refdata.Character{ID: uuid.New(), Gold: 100},
		stock:     2,
		gold:      100,
		goldErr:   errDB,
	}
	svc := shops.NewService(race.store())

	_, err := svc.Buy(context.Background(), itemID, uuid.New())
	assert.Error(t, err)
	assert.NotErrorIs(t, err, shops.ErrInsufficientGold)
	assert.Equal(t, 1, race.restoreCalls)
	assert.Equal(t, int32(2), race.stock)
}

// TestBuy_ResultUsesReturnedRows pins BuyResult to the values the atomic
// statements returned, not to Go-side arithmetic over the stale reads.
func TestBuy_ResultUsesReturnedRows(t *testing.T) {
	itemID := uuid.New()
	store := &mockStore{
		getShopItemFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			return refdata.ShopItem{ID: itemID, ItemID: "sword", Name: "Sword", PriceGp: 10, Quantity: 99}, nil
		},
		reserveShopItemStockFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			// Another buyer moved stock between the read and the reservation.
			return refdata.ShopItem{ID: itemID, ItemID: "sword", Name: "Sword", PriceGp: 10, Quantity: 7}, nil
		},
		getCharacterFn: func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
			return refdata.Character{ID: uuid.New(), Gold: 500}, nil
		},
		deductCharacterGoldFn: func(_ context.Context, _ refdata.DeductCharacterGoldAndSetInventoryParams) (refdata.Character, error) {
			// ...and spent some of their gold elsewhere too.
			return refdata.Character{Gold: 42}, nil
		},
	}
	svc := shops.NewService(store)

	result, err := svc.Buy(context.Background(), itemID, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "Sword", result.ItemName)
	assert.Equal(t, int32(10), result.PricePaid)
	assert.Equal(t, int32(42), result.GoldRemaining)
	assert.Equal(t, int32(7), result.StockRemaining)
}

func TestBuy_GetCharacterError(t *testing.T) {
	itemID := uuid.New()
	restored := 0
	store := &mockStore{
		getShopItemFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			return refdata.ShopItem{ID: itemID, Name: "Sword", PriceGp: 10, Quantity: 2}, nil
		},
		reserveShopItemStockFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			return refdata.ShopItem{ID: itemID, Name: "Sword", PriceGp: 10, Quantity: 1}, nil
		},
		restoreShopItemStockFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			restored++
			return refdata.ShopItem{Quantity: 2}, nil
		},
		getCharacterFn: func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
			return refdata.Character{}, errDB
		},
	}
	svc := shops.NewService(store)
	_, err := svc.Buy(context.Background(), itemID, uuid.New())
	assert.Error(t, err)
	assert.Equal(t, 1, restored, "a buy that aborts after reserving must restore stock")
}

func TestBuy_PersistError(t *testing.T) {
	itemID := uuid.New()
	store := &mockStore{
		getShopItemFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			return refdata.ShopItem{ID: itemID, Name: "Sword", PriceGp: 10, Quantity: 2}, nil
		},
		reserveShopItemStockFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			return refdata.ShopItem{ID: itemID, Name: "Sword", PriceGp: 10, Quantity: 1}, nil
		},
		restoreShopItemStockFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			return refdata.ShopItem{Quantity: 2}, nil
		},
		getCharacterFn: func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
			return refdata.Character{ID: uuid.New(), Gold: 100}, nil
		},
		deductCharacterGoldFn: func(_ context.Context, _ refdata.DeductCharacterGoldAndSetInventoryParams) (refdata.Character, error) {
			return refdata.Character{}, errDB
		},
	}
	svc := shops.NewService(store)
	_, err := svc.Buy(context.Background(), itemID, uuid.New())
	assert.Error(t, err)
}

// TestBuy_ReserveStockError verifies a failed reservation IS a real error:
// nothing was charged, so there is no committed purchase to protect and the
// caller must see the failure rather than a phantom success.
func TestBuy_ReserveStockError(t *testing.T) {
	itemID := uuid.New()
	race := &raceStore{
		staleItem:  refdata.ShopItem{ID: itemID, Name: "Sword", PriceGp: 10, Quantity: 2},
		staleChar:  refdata.Character{ID: uuid.New(), Gold: 100},
		stock:      2,
		gold:       100,
		reserveErr: errDB,
	}
	svc := shops.NewService(race.store())

	_, err := svc.Buy(context.Background(), itemID, uuid.New())
	assert.Error(t, err)
	assert.Equal(t, 0, race.goldWrites, "a failed reservation must not charge the buyer")
	assert.Equal(t, 0, race.restoreCalls, "nothing was reserved, so nothing to restore")
}

// TestBuy_RestoreFailureStillReportsOriginalError verifies the best-effort
// restore cannot mask the reason the purchase failed.
func TestBuy_RestoreFailureStillReportsOriginalError(t *testing.T) {
	itemID := uuid.New()
	store := &mockStore{
		getShopItemFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			return refdata.ShopItem{ID: itemID, Name: "Sword", PriceGp: 10, Quantity: 2}, nil
		},
		reserveShopItemStockFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			return refdata.ShopItem{ID: itemID, Name: "Sword", PriceGp: 10, Quantity: 1}, nil
		},
		restoreShopItemStockFn: func(_ context.Context, _ uuid.UUID) (refdata.ShopItem, error) {
			return refdata.ShopItem{}, errDB
		},
		getCharacterFn: func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
			return refdata.Character{ID: uuid.New(), Gold: 1}, nil
		},
		deductCharacterGoldFn: func(_ context.Context, _ refdata.DeductCharacterGoldAndSetInventoryParams) (refdata.Character, error) {
			return refdata.Character{}, sql.ErrNoRows
		},
	}
	svc := shops.NewService(store)
	_, err := svc.Buy(context.Background(), itemID, uuid.New())
	assert.ErrorIs(t, err, shops.ErrInsufficientGold)
}
