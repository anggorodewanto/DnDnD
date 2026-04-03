package inventory

import (
	"bytes"
	"context"
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

// mockCharacterStore implements CharacterStore for tests.
type mockCharacterStore struct {
	chars          map[uuid.UUID]refdata.Character
	lastInvUpdate  map[uuid.UUID]json.RawMessage
	lastGoldUpdate map[uuid.UUID]int32
	invUpdateErr   error // injected error for UpdateCharacterInventory
	failOnSecond   bool  // fail on second call to UpdateCharacterInventory
	invCallCount   int
}

func newMockStore() *mockCharacterStore {
	return &mockCharacterStore{
		chars:          make(map[uuid.UUID]refdata.Character),
		lastInvUpdate:  make(map[uuid.UUID]json.RawMessage),
		lastGoldUpdate: make(map[uuid.UUID]int32),
	}
}

func (m *mockCharacterStore) GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
	c, ok := m.chars[id]
	if !ok {
		return refdata.Character{}, assert.AnError
	}
	return c, nil
}

func (m *mockCharacterStore) UpdateCharacterInventory(ctx context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error) {
	m.invCallCount++
	if m.failOnSecond && m.invCallCount == 2 {
		return refdata.Character{}, assert.AnError
	}
	if m.invUpdateErr != nil {
		return refdata.Character{}, m.invUpdateErr
	}
	m.lastInvUpdate[arg.ID] = arg.Inventory.RawMessage
	return refdata.Character{}, nil
}

func (m *mockCharacterStore) UpdateTwoCharacterInventories(ctx context.Context, id1 uuid.UUID, inv1 pqtype.NullRawMessage, id2 uuid.UUID, inv2 pqtype.NullRawMessage) error {
	m.lastInvUpdate[id1] = inv1.RawMessage
	m.lastInvUpdate[id2] = inv2.RawMessage
	return nil
}

func (m *mockCharacterStore) UpdateCharacterGold(ctx context.Context, arg refdata.UpdateCharacterGoldParams) (refdata.Character, error) {
	m.lastGoldUpdate[arg.ID] = arg.Gold
	return refdata.Character{}, nil
}

func (m *mockCharacterStore) UpdateCharacterInventoryAndGold(ctx context.Context, arg refdata.UpdateCharacterInventoryAndGoldParams) (refdata.Character, error) {
	m.lastInvUpdate[arg.ID] = arg.Inventory.RawMessage
	m.lastGoldUpdate[arg.ID] = arg.Gold
	return refdata.Character{}, nil
}

func TestHandleAddItem(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()

	existingItems := []character.InventoryItem{
		{ItemID: "torch", Name: "Torch", Quantity: 3, Type: "other"},
	}
	existingJSON, _ := json.Marshal(existingItems)

	store.chars[charID] = refdata.Character{
		ID:   charID,
		Name: "Aria",
		Inventory: pqtype.NullRawMessage{RawMessage: existingJSON, Valid: true},
	}

	handler := NewAPIHandler(store)

	body, _ := json.Marshal(AddItemRequest{
		CharacterID: charID.String(),
		Item: character.InventoryItem{
			ItemID:   "healing-potion",
			Name:     "Healing Potion",
			Quantity: 2,
			Type:     "consumable",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/add", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAddItem(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var updatedItems []character.InventoryItem
	require.NoError(t, json.Unmarshal(store.lastInvUpdate[charID], &updatedItems))
	assert.Equal(t, 2, len(updatedItems))
	assert.Equal(t, "healing-potion", updatedItems[1].ItemID)
	assert.Equal(t, 2, updatedItems[1].Quantity)
}

func TestHandleAddItem_ExistingItem(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()

	existingItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 1, Type: "consumable"},
	}
	existingJSON, _ := json.Marshal(existingItems)

	store.chars[charID] = refdata.Character{
		ID:   charID,
		Inventory: pqtype.NullRawMessage{RawMessage: existingJSON, Valid: true},
	}

	handler := NewAPIHandler(store)

	body, _ := json.Marshal(AddItemRequest{
		CharacterID: charID.String(),
		Item: character.InventoryItem{
			ItemID:   "healing-potion",
			Name:     "Healing Potion",
			Quantity: 2,
			Type:     "consumable",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/add", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAddItem(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var updatedItems []character.InventoryItem
	require.NoError(t, json.Unmarshal(store.lastInvUpdate[charID], &updatedItems))
	assert.Equal(t, 1, len(updatedItems))
	assert.Equal(t, 3, updatedItems[0].Quantity) // 1 + 2
}

func TestHandleRemoveItem(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()

	existingItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 3, Type: "consumable"},
	}
	existingJSON, _ := json.Marshal(existingItems)

	store.chars[charID] = refdata.Character{
		ID:   charID,
		Inventory: pqtype.NullRawMessage{RawMessage: existingJSON, Valid: true},
	}

	handler := NewAPIHandler(store)

	body, _ := json.Marshal(RemoveItemRequest{
		CharacterID: charID.String(),
		ItemID:      "healing-potion",
		Quantity:    2,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/remove", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleRemoveItem(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var updatedItems []character.InventoryItem
	require.NoError(t, json.Unmarshal(store.lastInvUpdate[charID], &updatedItems))
	assert.Equal(t, 1, updatedItems[0].Quantity)
}

func TestHandleRemoveItem_AllQuantity(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()

	existingItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 1, Type: "consumable"},
	}
	existingJSON, _ := json.Marshal(existingItems)

	store.chars[charID] = refdata.Character{
		ID:   charID,
		Inventory: pqtype.NullRawMessage{RawMessage: existingJSON, Valid: true},
	}

	handler := NewAPIHandler(store)

	body, _ := json.Marshal(RemoveItemRequest{
		CharacterID: charID.String(),
		ItemID:      "healing-potion",
		Quantity:    1,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/remove", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleRemoveItem(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var updatedItems []character.InventoryItem
	require.NoError(t, json.Unmarshal(store.lastInvUpdate[charID], &updatedItems))
	assert.Empty(t, updatedItems)
}

func TestHandleTransferItem(t *testing.T) {
	fromID := uuid.New()
	toID := uuid.New()
	store := newMockStore()

	fromItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 3, Type: "consumable"},
	}
	fromJSON, _ := json.Marshal(fromItems)

	toItems := []character.InventoryItem{}
	toJSON, _ := json.Marshal(toItems)

	store.chars[fromID] = refdata.Character{ID: fromID, Name: "Aria", Inventory: pqtype.NullRawMessage{RawMessage: fromJSON, Valid: true}}
	store.chars[toID] = refdata.Character{ID: toID, Name: "Gorak", Inventory: pqtype.NullRawMessage{RawMessage: toJSON, Valid: true}}

	handler := NewAPIHandler(store)

	body, _ := json.Marshal(TransferItemRequest{
		FromCharacterID: fromID.String(),
		ToCharacterID:   toID.String(),
		ItemID:          "healing-potion",
		Quantity:        2,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/transfer", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleTransferItem(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var updatedFrom []character.InventoryItem
	require.NoError(t, json.Unmarshal(store.lastInvUpdate[fromID], &updatedFrom))
	assert.Equal(t, 1, updatedFrom[0].Quantity) // 3 - 2

	var updatedTo []character.InventoryItem
	require.NoError(t, json.Unmarshal(store.lastInvUpdate[toID], &updatedTo))
	assert.Equal(t, 2, updatedTo[0].Quantity) // 0 + 2
}

func TestHandleSetGold(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()
	store.chars[charID] = refdata.Character{ID: charID}

	handler := NewAPIHandler(store)

	body, _ := json.Marshal(SetGoldRequest{
		CharacterID: charID.String(),
		Gold:        150,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/gold", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleSetGold(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int32(150), store.lastGoldUpdate[charID])
}

func TestHandleSetGold_Negative(t *testing.T) {
	handler := NewAPIHandler(newMockStore())

	body, _ := json.Marshal(SetGoldRequest{
		CharacterID: uuid.New().String(),
		Gold:        -10,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/gold", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleSetGold(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleRemoveItem_NotEnough(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()

	existingItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 1, Type: "consumable"},
	}
	existingJSON, _ := json.Marshal(existingItems)

	store.chars[charID] = refdata.Character{
		ID:   charID,
		Inventory: pqtype.NullRawMessage{RawMessage: existingJSON, Valid: true},
	}

	handler := NewAPIHandler(store)

	body, _ := json.Marshal(RemoveItemRequest{
		CharacterID: charID.String(),
		ItemID:      "healing-potion",
		Quantity:    5,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/remove", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleRemoveItem(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleAddItem_InvalidCharacterID(t *testing.T) {
	handler := NewAPIHandler(newMockStore())

	body, _ := json.Marshal(AddItemRequest{
		CharacterID: "not-a-uuid",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/add", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAddItem(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleAddItem_CharacterNotFound(t *testing.T) {
	handler := NewAPIHandler(newMockStore())

	body, _ := json.Marshal(AddItemRequest{
		CharacterID: uuid.New().String(),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/add", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAddItem(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleAddItem_InvalidBody(t *testing.T) {
	handler := NewAPIHandler(newMockStore())

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/add", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()
	handler.HandleAddItem(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleRemoveItem_InvalidBody(t *testing.T) {
	handler := NewAPIHandler(newMockStore())

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/remove", bytes.NewReader([]byte("bad")))
	rec := httptest.NewRecorder()
	handler.HandleRemoveItem(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleRemoveItem_InvalidCharacterID(t *testing.T) {
	handler := NewAPIHandler(newMockStore())

	body, _ := json.Marshal(RemoveItemRequest{CharacterID: "bad"})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/remove", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleRemoveItem(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleRemoveItem_CharNotFound(t *testing.T) {
	handler := NewAPIHandler(newMockStore())

	body, _ := json.Marshal(RemoveItemRequest{CharacterID: uuid.New().String(), ItemID: "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/remove", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleRemoveItem(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleTransferItem_InvalidBody(t *testing.T) {
	handler := NewAPIHandler(newMockStore())

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/transfer", bytes.NewReader([]byte("bad")))
	rec := httptest.NewRecorder()
	handler.HandleTransferItem(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleTransferItem_InvalidFromID(t *testing.T) {
	handler := NewAPIHandler(newMockStore())

	body, _ := json.Marshal(TransferItemRequest{FromCharacterID: "bad", ToCharacterID: uuid.New().String()})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/transfer", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleTransferItem(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleTransferItem_InvalidToID(t *testing.T) {
	handler := NewAPIHandler(newMockStore())

	body, _ := json.Marshal(TransferItemRequest{FromCharacterID: uuid.New().String(), ToCharacterID: "bad"})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/transfer", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleTransferItem(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleTransferItem_FromNotFound(t *testing.T) {
	handler := NewAPIHandler(newMockStore())

	body, _ := json.Marshal(TransferItemRequest{
		FromCharacterID: uuid.New().String(),
		ToCharacterID:   uuid.New().String(),
		ItemID:          "x",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/transfer", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleTransferItem(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleTransferItem_ToNotFound(t *testing.T) {
	fromID := uuid.New()
	store := newMockStore()
	store.chars[fromID] = refdata.Character{ID: fromID, Name: "Aria"}

	handler := NewAPIHandler(store)

	body, _ := json.Marshal(TransferItemRequest{
		FromCharacterID: fromID.String(),
		ToCharacterID:   uuid.New().String(),
		ItemID:          "x",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/transfer", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleTransferItem(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleTransferItem_ItemNotInSource(t *testing.T) {
	fromID := uuid.New()
	toID := uuid.New()
	store := newMockStore()
	store.chars[fromID] = refdata.Character{ID: fromID, Name: "Aria"}
	store.chars[toID] = refdata.Character{ID: toID, Name: "Gorak"}

	handler := NewAPIHandler(store)

	body, _ := json.Marshal(TransferItemRequest{
		FromCharacterID: fromID.String(),
		ToCharacterID:   toID.String(),
		ItemID:          "healing-potion",
		Quantity:        1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/transfer", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleTransferItem(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleSetGold_InvalidBody(t *testing.T) {
	handler := NewAPIHandler(newMockStore())

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/gold", bytes.NewReader([]byte("bad")))
	rec := httptest.NewRecorder()
	handler.HandleSetGold(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleSetGold_InvalidCharacterID(t *testing.T) {
	handler := NewAPIHandler(newMockStore())

	body, _ := json.Marshal(SetGoldRequest{CharacterID: "bad", Gold: 10})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/gold", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleSetGold(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleRemoveItem_ItemNotFound(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()
	store.chars[charID] = refdata.Character{ID: charID}

	handler := NewAPIHandler(store)

	body, _ := json.Marshal(RemoveItemRequest{
		CharacterID: charID.String(),
		ItemID:      "nonexistent",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/remove", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleRemoveItem(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleTransferItem_Atomic(t *testing.T) {
	// Transfer should use UpdateTwoCharacterInventories for atomic write
	fromID := uuid.New()
	toID := uuid.New()
	store := newMockStore()

	fromItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 3, Type: "consumable"},
	}
	fromJSON, _ := json.Marshal(fromItems)
	toJSON, _ := json.Marshal([]character.InventoryItem{})

	store.chars[fromID] = refdata.Character{ID: fromID, Name: "Aria", Inventory: pqtype.NullRawMessage{RawMessage: fromJSON, Valid: true}}
	store.chars[toID] = refdata.Character{ID: toID, Name: "Gorak", Inventory: pqtype.NullRawMessage{RawMessage: toJSON, Valid: true}}

	handler := NewAPIHandler(store)

	body, _ := json.Marshal(TransferItemRequest{
		FromCharacterID: fromID.String(),
		ToCharacterID:   toID.String(),
		ItemID:          "healing-potion",
		Quantity:        1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/transfer", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleTransferItem(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// Both inventories should be updated atomically
	assert.Contains(t, store.lastInvUpdate, fromID)
	assert.Contains(t, store.lastInvUpdate, toID)
}

func TestHandleAddItem_CombatLog(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()
	store.chars[charID] = refdata.Character{ID: charID, Name: "Aria"}

	var logged string
	handler := NewAPIHandler(store)
	handler.SetCombatLogFunc(func(msg string) { logged = msg })

	body, _ := json.Marshal(AddItemRequest{
		CharacterID: charID.String(),
		Item: character.InventoryItem{
			ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/add", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAddItem(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, logged, "Healing Potion")
	assert.Contains(t, logged, "Aria")
}

func TestHandleRemoveItem_CombatLog(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()

	existingItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 3, Type: "consumable"},
	}
	existingJSON, _ := json.Marshal(existingItems)
	store.chars[charID] = refdata.Character{ID: charID, Name: "Aria", Inventory: pqtype.NullRawMessage{RawMessage: existingJSON, Valid: true}}

	var logged string
	handler := NewAPIHandler(store)
	handler.SetCombatLogFunc(func(msg string) { logged = msg })

	body, _ := json.Marshal(RemoveItemRequest{
		CharacterID: charID.String(),
		ItemID:      "healing-potion",
		Quantity:    1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/remove", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleRemoveItem(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, logged, "Healing Potion")
	assert.Contains(t, logged, "Aria")
}

func TestHandleTransferItem_CombatLog(t *testing.T) {
	fromID := uuid.New()
	toID := uuid.New()
	store := newMockStore()

	fromItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 3, Type: "consumable"},
	}
	fromJSON, _ := json.Marshal(fromItems)
	toJSON, _ := json.Marshal([]character.InventoryItem{})

	store.chars[fromID] = refdata.Character{ID: fromID, Name: "Aria", Inventory: pqtype.NullRawMessage{RawMessage: fromJSON, Valid: true}}
	store.chars[toID] = refdata.Character{ID: toID, Name: "Gorak", Inventory: pqtype.NullRawMessage{RawMessage: toJSON, Valid: true}}

	var logged string
	handler := NewAPIHandler(store)
	handler.SetCombatLogFunc(func(msg string) { logged = msg })

	body, _ := json.Marshal(TransferItemRequest{
		FromCharacterID: fromID.String(),
		ToCharacterID:   toID.String(),
		ItemID:          "healing-potion",
		Quantity:        2,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/transfer", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleTransferItem(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, logged, "Healing Potion")
	assert.Contains(t, logged, "Aria")
	assert.Contains(t, logged, "Gorak")
}

func TestHandleIdentifyItem_SetToTrue(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()

	unidentified := false
	existingItems := []character.InventoryItem{
		{ItemID: "mystery-ring", Name: "Ring of Invisibility", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &unidentified},
	}
	existingJSON, _ := json.Marshal(existingItems)
	store.chars[charID] = refdata.Character{ID: charID, Name: "Aria", Inventory: pqtype.NullRawMessage{RawMessage: existingJSON, Valid: true}}

	handler := NewAPIHandler(store)

	body, _ := json.Marshal(IdentifyItemRequest{
		CharacterID: charID.String(),
		ItemID:      "mystery-ring",
		Identified:  true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/identify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleIdentifyItem(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var updatedItems []character.InventoryItem
	require.NoError(t, json.Unmarshal(store.lastInvUpdate[charID], &updatedItems))
	require.NotNil(t, updatedItems[0].Identified)
	assert.True(t, *updatedItems[0].Identified)
}

func TestHandleIdentifyItem_SetToFalse(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()

	existingItems := []character.InventoryItem{
		{ItemID: "ring", Name: "Ring of Protection", Quantity: 1, Type: "magic_item", IsMagic: true},
	}
	existingJSON, _ := json.Marshal(existingItems)
	store.chars[charID] = refdata.Character{ID: charID, Name: "Aria", Inventory: pqtype.NullRawMessage{RawMessage: existingJSON, Valid: true}}

	handler := NewAPIHandler(store)

	body, _ := json.Marshal(IdentifyItemRequest{
		CharacterID: charID.String(),
		ItemID:      "ring",
		Identified:  false,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/identify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleIdentifyItem(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var updatedItems []character.InventoryItem
	require.NoError(t, json.Unmarshal(store.lastInvUpdate[charID], &updatedItems))
	require.NotNil(t, updatedItems[0].Identified)
	assert.False(t, *updatedItems[0].Identified)
}

func TestHandleIdentifyItem_InvalidBody(t *testing.T) {
	handler := NewAPIHandler(newMockStore())
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/identify", bytes.NewReader([]byte("bad")))
	rec := httptest.NewRecorder()
	handler.HandleIdentifyItem(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleIdentifyItem_InvalidCharacterID(t *testing.T) {
	handler := NewAPIHandler(newMockStore())
	body, _ := json.Marshal(IdentifyItemRequest{CharacterID: "bad", ItemID: "x", Identified: true})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/identify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleIdentifyItem(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleIdentifyItem_CharacterNotFound(t *testing.T) {
	handler := NewAPIHandler(newMockStore())
	body, _ := json.Marshal(IdentifyItemRequest{CharacterID: uuid.New().String(), ItemID: "x", Identified: true})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/identify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleIdentifyItem(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleIdentifyItem_NotMagic(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()
	existingItems := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}
	existingJSON, _ := json.Marshal(existingItems)
	store.chars[charID] = refdata.Character{ID: charID, Name: "Aria", Inventory: pqtype.NullRawMessage{RawMessage: existingJSON, Valid: true}}

	handler := NewAPIHandler(store)
	body, _ := json.Marshal(IdentifyItemRequest{CharacterID: charID.String(), ItemID: "longsword", Identified: false})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/identify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleIdentifyItem(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleIdentifyItem_CombatLog(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()

	unidentified := false
	existingItems := []character.InventoryItem{
		{ItemID: "mystery-ring", Name: "Ring of Invisibility", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &unidentified},
	}
	existingJSON, _ := json.Marshal(existingItems)
	store.chars[charID] = refdata.Character{ID: charID, Name: "Aria", Inventory: pqtype.NullRawMessage{RawMessage: existingJSON, Valid: true}}

	var logged string
	handler := NewAPIHandler(store)
	handler.SetCombatLogFunc(func(msg string) { logged = msg })

	body, _ := json.Marshal(IdentifyItemRequest{CharacterID: charID.String(), ItemID: "mystery-ring", Identified: true})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/identify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleIdentifyItem(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, logged, "Ring of Invisibility")
	assert.Contains(t, logged, "Aria")
}

func TestHandleIdentifyItem_PersistError(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()

	unidentified := false
	existingItems := []character.InventoryItem{
		{ItemID: "ring", Name: "Ring", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &unidentified},
	}
	existingJSON, _ := json.Marshal(existingItems)
	store.chars[charID] = refdata.Character{ID: charID, Name: "Aria", Inventory: pqtype.NullRawMessage{RawMessage: existingJSON, Valid: true}}
	store.invUpdateErr = assert.AnError

	handler := NewAPIHandler(store)
	body, _ := json.Marshal(IdentifyItemRequest{CharacterID: charID.String(), ItemID: "ring", Identified: true})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/identify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleIdentifyItem(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleSetGold_CombatLog(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()
	store.chars[charID] = refdata.Character{ID: charID, Name: "Aria", Gold: 50}

	var logged string
	handler := NewAPIHandler(store)
	handler.SetCombatLogFunc(func(msg string) { logged = msg })

	body, _ := json.Marshal(SetGoldRequest{
		CharacterID: charID.String(),
		Gold:        150,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/gold", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleSetGold(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, logged, "Aria")
	assert.Contains(t, logged, "150")
}

