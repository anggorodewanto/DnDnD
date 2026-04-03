package shops_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/shops"
)

func chiCtx(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func setupAPITest(t *testing.T) (*refdata.Queries, *shops.Handler, refdata.Campaign) {
	t.Helper()
	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	svc := shops.NewService(q)
	handler := shops.NewHandler(svc)
	return q, handler, camp
}

// --- HandleCreateShop ---

func TestAPI_CreateShop_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	body, _ := json.Marshal(map[string]string{"name": "Ironforge Smithy", "description": "Best weapons"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var shop refdata.Shop
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&shop))
	assert.Equal(t, "Ironforge Smithy", shop.Name)
}

func TestAPI_CreateShop_BadCampaignID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _ := setupAPITest(t)

	body, _ := json.Marshal(map[string]string{"name": "Test"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"campaignID": "bad-uuid"})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_CreateShop_BadBody(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_CreateShop_EmptyName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	body, _ := json.Marshal(map[string]string{"name": ""})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- HandleGetShop ---

func TestAPI_GetShop_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	// Create a shop first
	body, _ := json.Marshal(map[string]string{"name": "Test Shop"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created refdata.Shop
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&created))

	// Get it
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": created.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandleGetShop(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAPI_GetShop_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": uuid.New().String()})
	rec := httptest.NewRecorder()
	handler.HandleGetShop(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPI_GetShop_BadID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": "bad-uuid"})
	rec := httptest.NewRecorder()
	handler.HandleGetShop(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- HandleListShops ---

func TestAPI_ListShops(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleListShops(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var list []refdata.Shop
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&list))
	assert.Empty(t, list)
}

func TestAPI_ListShops_BadCampaignID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _ := setupAPITest(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": "bad-uuid"})
	rec := httptest.NewRecorder()
	handler.HandleListShops(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- HandleUpdateShop ---

func TestAPI_UpdateShop_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	// Create shop
	body, _ := json.Marshal(map[string]string{"name": "Old Name"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created refdata.Shop
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&created))

	// Update
	body, _ = json.Marshal(map[string]string{"name": "New Name", "description": "Updated"})
	req = httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": created.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandleUpdateShop(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAPI_UpdateShop_BadID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	body, _ := json.Marshal(map[string]string{"name": "Test"})
	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": "bad"})
	rec := httptest.NewRecorder()
	handler.HandleUpdateShop(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_UpdateShop_BadBody(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("bad")))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": uuid.New().String()})
	rec := httptest.NewRecorder()
	handler.HandleUpdateShop(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_UpdateShop_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	body, _ := json.Marshal(map[string]string{"name": "Test"})
	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": uuid.New().String()})
	rec := httptest.NewRecorder()
	handler.HandleUpdateShop(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPI_UpdateShop_EmptyName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	// Create shop first
	createBody, _ := json.Marshal(map[string]string{"name": "Shop"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(createBody))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	var created refdata.Shop
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&created))

	// Update with empty name
	body, _ := json.Marshal(map[string]string{"name": ""})
	req = httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": created.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandleUpdateShop(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- HandleDeleteShop ---

func TestAPI_DeleteShop_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	// Create
	body, _ := json.Marshal(map[string]string{"name": "Delete Me"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	var created refdata.Shop
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&created))

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": created.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandleDeleteShop(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAPI_DeleteShop_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": uuid.New().String()})
	rec := httptest.NewRecorder()
	handler.HandleDeleteShop(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPI_DeleteShop_BadID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": "bad"})
	rec := httptest.NewRecorder()
	handler.HandleDeleteShop(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- HandleAddItem ---

func TestAPI_AddItem_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	// Create shop
	shopBody, _ := json.Marshal(map[string]string{"name": "Shop"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(shopBody))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	var shop refdata.Shop
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&shop))

	// Add item
	itemBody, _ := json.Marshal(shops.AddItemRequest{
		Name: "Potion", PriceGP: 50, Quantity: 10, Type: "consumable",
	})
	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(itemBody))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": shop.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandleAddItem(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestAPI_AddItem_BadShopID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	body, _ := json.Marshal(shops.AddItemRequest{Name: "Test"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": "bad"})
	rec := httptest.NewRecorder()
	handler.HandleAddItem(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_AddItem_BadBody(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("bad")))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": uuid.New().String()})
	rec := httptest.NewRecorder()
	handler.HandleAddItem(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_AddItem_EmptyName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	body, _ := json.Marshal(shops.AddItemRequest{Name: ""})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": uuid.New().String()})
	rec := httptest.NewRecorder()
	handler.HandleAddItem(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_AddItem_ShopNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	body, _ := json.Marshal(shops.AddItemRequest{Name: "Sword", Quantity: 1})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": uuid.New().String()})
	rec := httptest.NewRecorder()
	handler.HandleAddItem(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPI_AddItem_DefaultQuantityAndType(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	// Create shop
	shopBody, _ := json.Marshal(map[string]string{"name": "Shop"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(shopBody))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	var shop refdata.Shop
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&shop))

	// Add item with no quantity/type
	body, _ := json.Marshal(shops.AddItemRequest{Name: "Widget"})
	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": shop.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandleAddItem(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var item refdata.ShopItem
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&item))
	assert.Equal(t, int32(1), item.Quantity)
	assert.Equal(t, "other", item.Type)
}

// --- HandleRemoveItem ---

func TestAPI_RemoveItem_BadID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req = chiCtx(req, map[string]string{
		"campaignID": camp.ID.String(),
		"shopID":     uuid.New().String(),
		"itemID":     "bad",
	})
	rec := httptest.NewRecorder()
	handler.HandleRemoveItem(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- HandleUpdateItem ---

func TestAPI_UpdateItem_BadID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	body, _ := json.Marshal(map[string]interface{}{"name": "New", "price_gp": 10, "quantity": 1})
	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{
		"campaignID": camp.ID.String(),
		"shopID":     uuid.New().String(),
		"itemID":     "bad",
	})
	rec := httptest.NewRecorder()
	handler.HandleUpdateItem(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_UpdateItem_BadBody(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("bad")))
	req = chiCtx(req, map[string]string{
		"campaignID": camp.ID.String(),
		"shopID":     uuid.New().String(),
		"itemID":     uuid.New().String(),
	})
	rec := httptest.NewRecorder()
	handler.HandleUpdateItem(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- HandlePostToDiscord ---

func TestAPI_PostToDiscord_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	// Create shop
	shopBody, _ := json.Marshal(map[string]string{"name": "Test Shop"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(shopBody))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	var shop refdata.Shop
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&shop))

	var postedChannel, postedContent string
	handler.SetPostFunc(func(channelID, content string) {
		postedChannel = channelID
		postedContent = content
	})

	req = httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": shop.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandlePostToDiscord(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "chan-story", postedChannel)
	assert.Contains(t, postedContent, "Test Shop")
}

func TestAPI_PostToDiscord_BadCampaignID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _ := setupAPITest(t)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": "bad", "shopID": uuid.New().String()})
	rec := httptest.NewRecorder()
	handler.HandlePostToDiscord(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_PostToDiscord_BadShopID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": "bad"})
	rec := httptest.NewRecorder()
	handler.HandlePostToDiscord(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_PostToDiscord_ShopNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": uuid.New().String()})
	rec := httptest.NewRecorder()
	handler.HandlePostToDiscord(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPI_PostToDiscord_CampaignNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	// Create shop
	shopBody, _ := json.Marshal(map[string]string{"name": "Shop"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(shopBody))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	var shop refdata.Shop
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&shop))

	// Post with wrong campaign ID
	req = httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": uuid.New().String(), "shopID": shop.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandlePostToDiscord(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- HandleRemoveItem success ---

func TestAPI_RemoveItem_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	// Create shop
	shopBody, _ := json.Marshal(map[string]string{"name": "Shop"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(shopBody))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	var shop refdata.Shop
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&shop))

	// Add item
	itemBody, _ := json.Marshal(shops.AddItemRequest{Name: "Sword", Quantity: 1})
	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(itemBody))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": shop.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandleAddItem(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	var item refdata.ShopItem
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&item))

	// Remove item
	req = httptest.NewRequest(http.MethodDelete, "/", nil)
	req = chiCtx(req, map[string]string{
		"campaignID": camp.ID.String(),
		"shopID":     shop.ID.String(),
		"itemID":     item.ID.String(),
	})
	rec = httptest.NewRecorder()
	handler.HandleRemoveItem(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- HandleUpdateItem success ---

func TestAPI_UpdateItem_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, camp := setupAPITest(t)

	// Create shop
	shopBody, _ := json.Marshal(map[string]string{"name": "Shop"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(shopBody))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	var shop refdata.Shop
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&shop))

	// Add item
	itemBody, _ := json.Marshal(shops.AddItemRequest{Name: "Sword", PriceGP: 10, Quantity: 1})
	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(itemBody))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": shop.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandleAddItem(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	var item refdata.ShopItem
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&item))

	// Update item
	updateBody, _ := json.Marshal(map[string]interface{}{
		"name": "Longsword +1", "description": "Magical!", "price_gp": 500, "quantity": 1,
	})
	req = httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(updateBody))
	req = chiCtx(req, map[string]string{
		"campaignID": camp.ID.String(),
		"shopID":     shop.ID.String(),
		"itemID":     item.ID.String(),
	})
	rec = httptest.NewRecorder()
	handler.HandleUpdateItem(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var updated refdata.ShopItem
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&updated))
	assert.Equal(t, "Longsword +1", updated.Name)
	assert.Equal(t, int32(500), updated.PriceGp)
}

// --- HandleCreateShop internal error ---

func TestAPI_CreateShop_InternalError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _ := setupAPITest(t)

	// Provide a valid but non-existent campaign UUID - the FK constraint will fail
	body, _ := json.Marshal(map[string]string{"name": "Test"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"campaignID": uuid.New().String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- HandleListShops internal error ---

// HandleGetShop internal error (not ErrShopNotFound) is hard to trigger with real DB,
// but we covered the branch in mock tests.

// --- HandleDeleteShop internal error ---

// --- getTheStoryChannelID with invalid JSON settings ---

func TestAPI_PostToDiscord_NullSettings(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	// Create campaign with null settings
	camp, err := q.CreateCampaign(context.Background(), refdata.CreateCampaignParams{
		GuildID:  "guild-null-settings-" + uuid.New().String()[:8],
		DmUserID: "dm-1",
		Name:     "Null Settings Campaign",
		Settings: pqtype.NullRawMessage{Valid: false},
	})
	require.NoError(t, err)

	svc := shops.NewService(q)
	handler := shops.NewHandler(svc)

	// Create shop
	shopBody, _ := json.Marshal(map[string]string{"name": "Shop"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(shopBody))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	var shop refdata.Shop
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&shop))

	// Post with null settings
	req = httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": shop.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandlePostToDiscord(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_PostToDiscord_NoChannelConfigured(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	// Create campaign with no channel_ids
	camp, err := q.CreateCampaign(context.Background(), refdata.CreateCampaignParams{
		GuildID:  "guild-no-chan-" + uuid.New().String()[:8],
		DmUserID: "dm-1",
		Name:     "No Channels Campaign",
		Settings: pqtype.NullRawMessage{RawMessage: []byte(`{"channel_ids":{}}`), Valid: true},
	})
	require.NoError(t, err)

	svc := shops.NewService(q)
	handler := shops.NewHandler(svc)

	// Create shop
	shopBody, _ := json.Marshal(map[string]string{"name": "Shop"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(shopBody))
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateShop(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	var shop refdata.Shop
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&shop))

	// Post should fail
	req = httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String(), "shopID": shop.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandlePostToDiscord(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
