package shops_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/shops"
	"github.com/ab/dndnd/internal/testutil"
)

func setupTestDB(t *testing.T) (*sql.DB, *refdata.Queries) {
	t.Helper()
	db := sharedDB.AcquireDB(t)
	return db, refdata.New(db)
}

func createCampaign(t *testing.T, q *refdata.Queries) refdata.Campaign {
	t.Helper()
	camp := testutil.NewTestCampaign(t, q, "guild-shops-svc")
	// Handler tests posting to Discord need a channel_ids.the-story setting.
	updated, err := q.UpdateCampaignSettings(context.Background(), refdata.UpdateCampaignSettingsParams{
		ID:       camp.ID,
		Settings: pqtype.NullRawMessage{RawMessage: []byte(`{"channel_ids":{"the-story":"chan-story"}}`), Valid: true},
	})
	require.NoError(t, err)
	return updated
}

// --- TDD Cycle 1: CreateShop validation ---

func TestCreateShop_EmptyName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)

	svc := shops.NewService(q)
	_, err := svc.CreateShop(context.Background(), camp.ID, "", "")
	assert.ErrorIs(t, err, shops.ErrNameRequired)
}

func TestCreateShop_WhitespaceOnlyName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)

	svc := shops.NewService(q)
	_, err := svc.CreateShop(context.Background(), camp.ID, "   ", "")
	assert.ErrorIs(t, err, shops.ErrNameRequired)
}

func TestCreateShop_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)

	svc := shops.NewService(q)
	shop, err := svc.CreateShop(context.Background(), camp.ID, "Ironforge Smithy", "Best weapons in town")
	require.NoError(t, err)
	assert.Equal(t, "Ironforge Smithy", shop.Name)
	assert.Equal(t, "Best weapons in town", shop.Description)
	assert.Equal(t, camp.ID, shop.CampaignID)
}

// --- TDD Cycle 2: GetShop ---

func TestGetShop_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	svc := shops.NewService(q)

	_, err := svc.GetShop(context.Background(), uuid.New())
	assert.ErrorIs(t, err, shops.ErrShopNotFound)
}

func TestGetShop_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)

	svc := shops.NewService(q)
	shop, err := svc.CreateShop(context.Background(), camp.ID, "Ye Olde Potion Shoppe", "Potions galore")
	require.NoError(t, err)

	result, err := svc.GetShop(context.Background(), shop.ID)
	require.NoError(t, err)
	assert.Equal(t, shop.ID, result.Shop.ID)
	assert.Equal(t, "Ye Olde Potion Shoppe", result.Shop.Name)
	assert.Empty(t, result.Items)
}

// --- TDD Cycle 3: ListShops ---

func TestListShops_Empty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)

	svc := shops.NewService(q)
	list, err := svc.ListShops(context.Background(), camp.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestListShops_Multiple(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)

	svc := shops.NewService(q)
	_, err := svc.CreateShop(context.Background(), camp.ID, "Shop A", "")
	require.NoError(t, err)
	_, err = svc.CreateShop(context.Background(), camp.ID, "Shop B", "")
	require.NoError(t, err)

	list, err := svc.ListShops(context.Background(), camp.ID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

// --- TDD Cycle 4: UpdateShop ---

func TestUpdateShop_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	svc := shops.NewService(q)

	_, err := svc.UpdateShop(context.Background(), uuid.New(), "New Name", "")
	assert.ErrorIs(t, err, shops.ErrShopNotFound)
}

func TestUpdateShop_EmptyName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	svc := shops.NewService(q)

	_, err := svc.UpdateShop(context.Background(), uuid.New(), "", "")
	assert.ErrorIs(t, err, shops.ErrNameRequired)
}

func TestUpdateShop_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)

	svc := shops.NewService(q)
	shop, err := svc.CreateShop(context.Background(), camp.ID, "Old Name", "Old desc")
	require.NoError(t, err)

	updated, err := svc.UpdateShop(context.Background(), shop.ID, "New Name", "New desc")
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
	assert.Equal(t, "New desc", updated.Description)
}

// --- TDD Cycle 5: DeleteShop ---

func TestDeleteShop_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	svc := shops.NewService(q)

	err := svc.DeleteShop(context.Background(), uuid.New())
	assert.ErrorIs(t, err, shops.ErrShopNotFound)
}

func TestDeleteShop_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)

	svc := shops.NewService(q)
	shop, err := svc.CreateShop(context.Background(), camp.ID, "Delete Me", "")
	require.NoError(t, err)

	err = svc.DeleteShop(context.Background(), shop.ID)
	require.NoError(t, err)

	_, err = svc.GetShop(context.Background(), shop.ID)
	assert.ErrorIs(t, err, shops.ErrShopNotFound)
}

// --- TDD Cycle 6: AddItem ---

func TestAddItem_ShopNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	svc := shops.NewService(q)

	_, err := svc.AddItem(context.Background(), uuid.New(), refdata.CreateShopItemParams{
		Name: "Sword", PriceGp: 10, Quantity: 1, Type: "weapon",
	})
	assert.ErrorIs(t, err, shops.ErrShopNotFound)
}

func TestAddItem_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)

	svc := shops.NewService(q)
	shop, err := svc.CreateShop(context.Background(), camp.ID, "Item Shop", "")
	require.NoError(t, err)

	item, err := svc.AddItem(context.Background(), shop.ID, refdata.CreateShopItemParams{
		ItemID:      "longsword",
		Name:        "Longsword",
		Description: "A fine blade",
		PriceGp:     15,
		Quantity:    5,
		Type:        "weapon",
	})
	require.NoError(t, err)
	assert.Equal(t, "Longsword", item.Name)
	assert.Equal(t, int32(15), item.PriceGp)
	assert.Equal(t, int32(5), item.Quantity)

	// Verify item shows in GetShop
	result, err := svc.GetShop(context.Background(), shop.ID)
	require.NoError(t, err)
	assert.Len(t, result.Items, 1)
}

// --- TDD Cycle 7: RemoveItem ---

func TestRemoveItem_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)

	svc := shops.NewService(q)
	shop, err := svc.CreateShop(context.Background(), camp.ID, "Item Shop", "")
	require.NoError(t, err)

	item, err := svc.AddItem(context.Background(), shop.ID, refdata.CreateShopItemParams{
		Name: "Rope", PriceGp: 1, Quantity: 10, Type: "other",
	})
	require.NoError(t, err)

	err = svc.RemoveItem(context.Background(), item.ID)
	require.NoError(t, err)

	result, err := svc.GetShop(context.Background(), shop.ID)
	require.NoError(t, err)
	assert.Empty(t, result.Items)
}

// --- TDD Cycle 8: FormatShopAnnouncement ---

func TestFormatShopAnnouncement_WithItems(t *testing.T) {
	shop := refdata.Shop{Name: "Ironforge Smithy", Description: "Finest weapons in the realm"}
	items := []refdata.ShopItem{
		{Name: "Longsword", PriceGp: 15, Description: "A fine blade"},
		{Name: "Shield", PriceGp: 10, Description: ""},
		{Name: "Free Sample", PriceGp: 0, Description: "Try it!"},
	}

	msg := shops.FormatShopAnnouncement(shop, items)
	assert.Contains(t, msg, "**Ironforge Smithy**")
	assert.Contains(t, msg, "_Finest weapons in the realm_")
	assert.Contains(t, msg, "**Longsword** — 15 gp")
	assert.Contains(t, msg, "_A fine blade_")
	assert.Contains(t, msg, "**Shield** — 10 gp")
	assert.Contains(t, msg, "**Free Sample**")
	assert.NotContains(t, msg, "— 0 gp") // free items should not show price
}

func TestFormatShopAnnouncement_Empty(t *testing.T) {
	shop := refdata.Shop{Name: "Empty Shop"}
	msg := shops.FormatShopAnnouncement(shop, nil)
	assert.Contains(t, msg, "**Empty Shop**")
	assert.Contains(t, msg, "No items available")
}

func TestFormatShopAnnouncement_NoDescription(t *testing.T) {
	shop := refdata.Shop{Name: "Basic Shop"}
	items := []refdata.ShopItem{
		{Name: "Potion", PriceGp: 50},
	}
	msg := shops.FormatShopAnnouncement(shop, items)
	assert.Contains(t, msg, "**Basic Shop**")
	assert.NotContains(t, msg, "_\n") // no description line
	assert.Contains(t, msg, "**Potion** — 50 gp")
}

// --- TDD Cycle 9: Full lifecycle ---

func TestShopLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)

	svc := shops.NewService(q)
	ctx := context.Background()

	// Create shop
	shop, err := svc.CreateShop(ctx, camp.ID, "Ye Olde Shoppe", "Your one-stop shop")
	require.NoError(t, err)

	// Add items
	_, err = svc.AddItem(ctx, shop.ID, refdata.CreateShopItemParams{
		Name: "Healing Potion", PriceGp: 50, Quantity: 10, Type: "consumable",
	})
	require.NoError(t, err)

	sword, err := svc.AddItem(ctx, shop.ID, refdata.CreateShopItemParams{
		ItemID: "longsword", Name: "Longsword", PriceGp: 15, Quantity: 3, Type: "weapon",
	})
	require.NoError(t, err)

	// Get shop with items
	result, err := svc.GetShop(ctx, shop.ID)
	require.NoError(t, err)
	assert.Len(t, result.Items, 2)

	// Update item
	_, err = svc.UpdateItem(ctx, refdata.UpdateShopItemParams{
		ID: sword.ID, Name: "Longsword +1", Description: "Magical!", PriceGp: 500, Quantity: 1,
	})
	require.NoError(t, err)

	// Format announcement
	result, err = svc.GetShop(ctx, shop.ID)
	require.NoError(t, err)
	msg := shops.FormatShopAnnouncement(result.Shop, result.Items)
	assert.Contains(t, msg, "Ye Olde Shoppe")
	assert.Contains(t, msg, "Healing Potion")

	// Update shop name
	_, err = svc.UpdateShop(ctx, shop.ID, "Renamed Shop", "New desc")
	require.NoError(t, err)

	// Delete shop
	err = svc.DeleteShop(ctx, shop.ID)
	require.NoError(t, err)

	// Confirm deleted
	_, err = svc.GetShop(ctx, shop.ID)
	assert.ErrorIs(t, err, shops.ErrShopNotFound)

	// Items should be cascade-deleted (list from campaign)
	shops_list, err := svc.ListShops(ctx, camp.ID)
	require.NoError(t, err)
	assert.Empty(t, shops_list)
}

// --- Mock tests for error branches ---

func TestCreateShop_StoreError(t *testing.T) {
	store := &mockStore{
		createShopFn: func(_ context.Context, _ refdata.CreateShopParams) (refdata.Shop, error) {
			return refdata.Shop{}, errDB
		},
	}
	svc := shops.NewService(store)
	_, err := svc.CreateShop(context.Background(), uuid.New(), "Test", "")
	assert.Error(t, err)
}

func TestGetShop_ListItemsError(t *testing.T) {
	shopID := uuid.New()
	store := &mockStore{
		getShopFn: func(_ context.Context, _ uuid.UUID) (refdata.Shop, error) {
			return refdata.Shop{ID: shopID}, nil
		},
		listShopItemsFn: func(_ context.Context, _ uuid.UUID) ([]refdata.ShopItem, error) {
			return nil, errDB
		},
	}
	svc := shops.NewService(store)
	_, err := svc.GetShop(context.Background(), shopID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "listing shop items")
}

func TestUpdateShop_StoreUpdateError(t *testing.T) {
	shopID := uuid.New()
	store := &mockStore{
		getShopFn: func(_ context.Context, _ uuid.UUID) (refdata.Shop, error) {
			return refdata.Shop{ID: shopID}, nil
		},
		updateShopFn: func(_ context.Context, _ refdata.UpdateShopParams) (refdata.Shop, error) {
			return refdata.Shop{}, errDB
		},
	}
	svc := shops.NewService(store)
	_, err := svc.UpdateShop(context.Background(), shopID, "New", "")
	assert.Error(t, err)
}
