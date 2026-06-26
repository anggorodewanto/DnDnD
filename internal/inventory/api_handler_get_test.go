package inventory

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

func TestHandleGetInventory(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()

	items := []character.InventoryItem{
		{ItemID: "torch", Name: "Torch", Quantity: 3, Type: "other"},
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}
	itemsJSON, _ := json.Marshal(items)

	store.chars[charID] = refdata.Character{
		ID:        charID,
		Name:      "Aria",
		Gold:      120,
		Inventory: pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
	}

	handler := NewAPIHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/inventory?character_id="+charID.String(), nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInventory(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp GetInventoryResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, charID.String(), resp.CharacterID)
	assert.Equal(t, "Aria", resp.Name)
	assert.Equal(t, 120, resp.Gold)
	require.Len(t, resp.Items, 2)
	assert.Equal(t, "Torch", resp.Items[0].Name)
	assert.Equal(t, 3, resp.Items[0].Quantity)
}

func TestHandleGetInventory_EmptyInventory(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()
	store.chars[charID] = refdata.Character{ID: charID, Name: "Broke", Gold: 0}

	handler := NewAPIHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/inventory?character_id="+charID.String(), nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInventory(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp GetInventoryResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Gold)
	// Empty inventory serializes as an empty array, never null, so the UI can
	// iterate without a guard.
	assert.NotNil(t, resp.Items)
	assert.Len(t, resp.Items, 0)
}

func TestHandleGetInventory_BadCharacterID(t *testing.T) {
	handler := NewAPIHandler(newMockStore())

	req := httptest.NewRequest(http.MethodGet, "/api/inventory?character_id=not-a-uuid", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInventory(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleGetInventory_MissingCharacterID(t *testing.T) {
	handler := NewAPIHandler(newMockStore())

	req := httptest.NewRequest(http.MethodGet, "/api/inventory", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInventory(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleGetInventory_NotFound(t *testing.T) {
	handler := NewAPIHandler(newMockStore())

	req := httptest.NewRequest(http.MethodGet, "/api/inventory?character_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInventory(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
