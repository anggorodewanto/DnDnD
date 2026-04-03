package shops_test

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

var errDB = errors.New("db error")

// mockStore implements shops.Store for unit testing error branches.
type mockStore struct {
	createShopFn          func(ctx context.Context, arg refdata.CreateShopParams) (refdata.Shop, error)
	getShopFn             func(ctx context.Context, id uuid.UUID) (refdata.Shop, error)
	listShopsByCampaignFn func(ctx context.Context, campaignID uuid.UUID) ([]refdata.Shop, error)
	updateShopFn          func(ctx context.Context, arg refdata.UpdateShopParams) (refdata.Shop, error)
	deleteShopFn          func(ctx context.Context, id uuid.UUID) error
	createShopItemFn      func(ctx context.Context, arg refdata.CreateShopItemParams) (refdata.ShopItem, error)
	listShopItemsFn       func(ctx context.Context, shopID uuid.UUID) ([]refdata.ShopItem, error)
	updateShopItemFn      func(ctx context.Context, arg refdata.UpdateShopItemParams) (refdata.ShopItem, error)
	deleteShopItemFn      func(ctx context.Context, id uuid.UUID) error
	deleteShopItemsByShopFn func(ctx context.Context, shopID uuid.UUID) error
	getCampaignByIDFn     func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)
}

func (m *mockStore) CreateShop(ctx context.Context, arg refdata.CreateShopParams) (refdata.Shop, error) {
	return m.createShopFn(ctx, arg)
}
func (m *mockStore) GetShop(ctx context.Context, id uuid.UUID) (refdata.Shop, error) {
	return m.getShopFn(ctx, id)
}
func (m *mockStore) ListShopsByCampaign(ctx context.Context, campaignID uuid.UUID) ([]refdata.Shop, error) {
	return m.listShopsByCampaignFn(ctx, campaignID)
}
func (m *mockStore) UpdateShop(ctx context.Context, arg refdata.UpdateShopParams) (refdata.Shop, error) {
	return m.updateShopFn(ctx, arg)
}
func (m *mockStore) DeleteShop(ctx context.Context, id uuid.UUID) error {
	return m.deleteShopFn(ctx, id)
}
func (m *mockStore) CreateShopItem(ctx context.Context, arg refdata.CreateShopItemParams) (refdata.ShopItem, error) {
	return m.createShopItemFn(ctx, arg)
}
func (m *mockStore) ListShopItems(ctx context.Context, shopID uuid.UUID) ([]refdata.ShopItem, error) {
	return m.listShopItemsFn(ctx, shopID)
}
func (m *mockStore) UpdateShopItem(ctx context.Context, arg refdata.UpdateShopItemParams) (refdata.ShopItem, error) {
	return m.updateShopItemFn(ctx, arg)
}
func (m *mockStore) DeleteShopItem(ctx context.Context, id uuid.UUID) error {
	return m.deleteShopItemFn(ctx, id)
}
func (m *mockStore) DeleteShopItemsByShop(ctx context.Context, shopID uuid.UUID) error {
	return m.deleteShopItemsByShopFn(ctx, shopID)
}
func (m *mockStore) GetCampaignByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
	return m.getCampaignByIDFn(ctx, id)
}
