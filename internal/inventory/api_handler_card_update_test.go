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

// recordingCardUpdater records OnCharacterUpdated calls so SR-007 tests can
// assert each DM inventory API mutation fires the persistent
// #character-cards refresh on success.
type recordingCardUpdater struct {
	calls []uuid.UUID
}

func (r *recordingCardUpdater) OnCharacterUpdated(ctx context.Context, characterID uuid.UUID) error {
	r.calls = append(r.calls, characterID)
	return nil
}

// SR-007: HandleAddItem MUST fire OnCharacterUpdated on a successful write so
// the DM inventory dashboard mutation lands on #character-cards too.
func TestAPIHandler_AddItem_FiresCardUpdater(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()

	existingJSON, _ := json.Marshal([]character.InventoryItem{})
	store.chars[charID] = refdata.Character{
		ID:        charID,
		Inventory: pqtype.NullRawMessage{RawMessage: existingJSON, Valid: true},
	}

	handler := NewAPIHandler(store)
	rec := &recordingCardUpdater{}
	handler.SetCardUpdater(rec)

	body, _ := json.Marshal(AddItemRequest{
		CharacterID: charID.String(),
		Item: character.InventoryItem{
			ItemID:   "healing-potion",
			Name:     "Healing Potion",
			Quantity: 1,
			Type:     "consumable",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/add", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleAddItem(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, rec.calls, 1, "DM /add success must fire OnCharacterUpdated exactly once")
	assert.Equal(t, charID, rec.calls[0])
}

// SR-007: HandleRemoveItem.
func TestAPIHandler_RemoveItem_FiresCardUpdater(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()

	existingJSON, _ := json.Marshal([]character.InventoryItem{
		{ItemID: "potion", Name: "Potion", Quantity: 3, Type: "consumable"},
	})
	store.chars[charID] = refdata.Character{
		ID:        charID,
		Inventory: pqtype.NullRawMessage{RawMessage: existingJSON, Valid: true},
	}

	handler := NewAPIHandler(store)
	rec := &recordingCardUpdater{}
	handler.SetCardUpdater(rec)

	body, _ := json.Marshal(RemoveItemRequest{
		CharacterID: charID.String(),
		ItemID:      "potion",
		Quantity:    1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/remove", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleRemoveItem(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, rec.calls, 1, "DM /remove success must fire OnCharacterUpdated")
	assert.Equal(t, charID, rec.calls[0])
}

// SR-007: HandleTransferItem fires for BOTH characters (one card refresh each).
func TestAPIHandler_TransferItem_FiresCardUpdaterForBoth(t *testing.T) {
	fromID := uuid.New()
	toID := uuid.New()
	store := newMockStore()

	fromJSON, _ := json.Marshal([]character.InventoryItem{
		{ItemID: "potion", Name: "Potion", Quantity: 2, Type: "consumable"},
	})
	toJSON, _ := json.Marshal([]character.InventoryItem{})
	store.chars[fromID] = refdata.Character{ID: fromID, Inventory: pqtype.NullRawMessage{RawMessage: fromJSON, Valid: true}}
	store.chars[toID] = refdata.Character{ID: toID, Inventory: pqtype.NullRawMessage{RawMessage: toJSON, Valid: true}}

	handler := NewAPIHandler(store)
	rec := &recordingCardUpdater{}
	handler.SetCardUpdater(rec)

	body, _ := json.Marshal(TransferItemRequest{
		FromCharacterID: fromID.String(),
		ToCharacterID:   toID.String(),
		ItemID:          "potion",
		Quantity:        1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/transfer", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleTransferItem(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, rec.calls, 2, "DM transfer success must fire OnCharacterUpdated for both parties")
	got := map[uuid.UUID]bool{rec.calls[0]: true, rec.calls[1]: true}
	assert.True(t, got[fromID])
	assert.True(t, got[toID])
}

// SR-007: HandleSetGold.
func TestAPIHandler_SetGold_FiresCardUpdater(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()
	store.chars[charID] = refdata.Character{ID: charID, Gold: 5}

	handler := NewAPIHandler(store)
	rec := &recordingCardUpdater{}
	handler.SetCardUpdater(rec)

	body, _ := json.Marshal(SetGoldRequest{CharacterID: charID.String(), Gold: 42})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/gold", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleSetGold(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, rec.calls, 1, "DM set-gold success must fire OnCharacterUpdated")
	assert.Equal(t, charID, rec.calls[0])
}

// SR-007: HandleIdentifyItem.
func TestAPIHandler_IdentifyItem_FiresCardUpdater(t *testing.T) {
	charID := uuid.New()
	store := newMockStore()

	existingJSON, _ := json.Marshal([]character.InventoryItem{
		{ItemID: "wand", Name: "Wand of Magic Missiles", Quantity: 1, Type: "magic_item", IsMagic: true},
	})
	store.chars[charID] = refdata.Character{ID: charID, Inventory: pqtype.NullRawMessage{RawMessage: existingJSON, Valid: true}}

	handler := NewAPIHandler(store)
	rec := &recordingCardUpdater{}
	handler.SetCardUpdater(rec)

	body, _ := json.Marshal(IdentifyItemRequest{
		CharacterID: charID.String(),
		ItemID:      "wand",
		Identified:  true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/inventory/identify", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleIdentifyItem(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, rec.calls, 1, "DM identify success must fire OnCharacterUpdated")
	assert.Equal(t, charID, rec.calls[0])
}
