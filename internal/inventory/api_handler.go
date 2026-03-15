package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// CharacterStore defines the database operations for inventory management.
type CharacterStore interface {
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	UpdateCharacterInventory(ctx context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error)
	UpdateCharacterGold(ctx context.Context, arg refdata.UpdateCharacterGoldParams) (refdata.Character, error)
	UpdateCharacterInventoryAndGold(ctx context.Context, arg refdata.UpdateCharacterInventoryAndGoldParams) (refdata.Character, error)
}

// APIHandler handles DM inventory management HTTP endpoints.
type APIHandler struct {
	store CharacterStore
}

// NewAPIHandler creates a new APIHandler.
func NewAPIHandler(store CharacterStore) *APIHandler {
	return &APIHandler{store: store}
}

// AddItemRequest is the request body for adding an item to a character.
type AddItemRequest struct {
	CharacterID string               `json:"character_id"`
	Item        character.InventoryItem `json:"item"`
}

// RemoveItemRequest is the request body for removing an item from a character.
type RemoveItemRequest struct {
	CharacterID string `json:"character_id"`
	ItemID      string `json:"item_id"`
	Quantity    int    `json:"quantity"`
}

// TransferItemRequest is the request body for transferring an item between characters.
type TransferItemRequest struct {
	FromCharacterID string `json:"from_character_id"`
	ToCharacterID   string `json:"to_character_id"`
	ItemID          string `json:"item_id"`
	Quantity        int    `json:"quantity"`
}

// SetGoldRequest is the request body for setting a character's gold.
type SetGoldRequest struct {
	CharacterID string `json:"character_id"`
	Gold        int    `json:"gold"`
}

// HandleAddItem handles POST /api/inventory/add.
func (h *APIHandler) HandleAddItem(w http.ResponseWriter, r *http.Request) {
	var req AddItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	charID, err := uuid.Parse(req.CharacterID)
	if err != nil {
		http.Error(w, "invalid character_id", http.StatusBadRequest)
		return
	}

	char, err := h.store.GetCharacter(r.Context(), charID)
	if err != nil {
		http.Error(w, "character not found", http.StatusNotFound)
		return
	}

	items := parseInventoryItems(char)

	// Check if item already exists
	found := false
	for i := range items {
		if items[i].ItemID == req.Item.ItemID {
			items[i].Quantity += req.Item.Quantity
			found = true
			break
		}
	}
	if !found {
		items = append(items, req.Item)
	}

	if err := h.persistInventory(r.Context(), charID, items); err != nil {
		http.Error(w, "failed to update inventory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleRemoveItem handles POST /api/inventory/remove.
func (h *APIHandler) HandleRemoveItem(w http.ResponseWriter, r *http.Request) {
	var req RemoveItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	charID, err := uuid.Parse(req.CharacterID)
	if err != nil {
		http.Error(w, "invalid character_id", http.StatusBadRequest)
		return
	}

	char, err := h.store.GetCharacter(r.Context(), charID)
	if err != nil {
		http.Error(w, "character not found", http.StatusNotFound)
		return
	}

	items := parseInventoryItems(char)
	qty := req.Quantity
	if qty <= 0 {
		qty = 1
	}

	updated, err := removeItem(items, req.ItemID, qty)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.persistInventory(r.Context(), charID, updated); err != nil {
		http.Error(w, "failed to update inventory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleTransferItem handles POST /api/inventory/transfer.
func (h *APIHandler) HandleTransferItem(w http.ResponseWriter, r *http.Request) {
	var req TransferItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	fromID, err := uuid.Parse(req.FromCharacterID)
	if err != nil {
		http.Error(w, "invalid from_character_id", http.StatusBadRequest)
		return
	}
	toID, err := uuid.Parse(req.ToCharacterID)
	if err != nil {
		http.Error(w, "invalid to_character_id", http.StatusBadRequest)
		return
	}

	qty := req.Quantity
	if qty <= 0 {
		qty = 1
	}

	fromChar, err := h.store.GetCharacter(r.Context(), fromID)
	if err != nil {
		http.Error(w, "source character not found", http.StatusNotFound)
		return
	}
	toChar, err := h.store.GetCharacter(r.Context(), toID)
	if err != nil {
		http.Error(w, "target character not found", http.StatusNotFound)
		return
	}

	fromItems := parseInventoryItems(fromChar)
	toItems := parseInventoryItems(toChar)

	// Transfer qty units
	for i := 0; i < qty; i++ {
		result, giveErr := GiveItem(GiveInput{
			GiverItems:    fromItems,
			ReceiverItems: toItems,
			ItemID:        req.ItemID,
			GiverName:     fromChar.Name,
			ReceiverName:  toChar.Name,
		})
		if giveErr != nil {
			http.Error(w, giveErr.Error(), http.StatusBadRequest)
			return
		}
		fromItems = result.UpdatedGiverItems
		toItems = result.UpdatedReceiverItems
	}

	if err := h.persistInventory(r.Context(), fromID, fromItems); err != nil {
		http.Error(w, "failed to update source inventory", http.StatusInternalServerError)
		return
	}
	if err := h.persistInventory(r.Context(), toID, toItems); err != nil {
		http.Error(w, "failed to update target inventory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleSetGold handles POST /api/inventory/gold.
func (h *APIHandler) HandleSetGold(w http.ResponseWriter, r *http.Request) {
	var req SetGoldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	charID, err := uuid.Parse(req.CharacterID)
	if err != nil {
		http.Error(w, "invalid character_id", http.StatusBadRequest)
		return
	}

	if req.Gold < 0 {
		http.Error(w, "gold cannot be negative", http.StatusBadRequest)
		return
	}

	_, err = h.store.UpdateCharacterGold(r.Context(), refdata.UpdateCharacterGoldParams{
		ID:   charID,
		Gold: int32(req.Gold),
	})
	if err != nil {
		http.Error(w, "failed to update gold", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// parseInventoryItems extracts inventory items from a character.
func parseInventoryItems(char refdata.Character) []character.InventoryItem {
	var items []character.InventoryItem
	if char.Inventory.Valid {
		_ = json.Unmarshal(char.Inventory.RawMessage, &items)
	}
	return items
}

// removeItem removes qty of the item from the list.
func removeItem(items []character.InventoryItem, itemID string, qty int) ([]character.InventoryItem, error) {
	for i := range items {
		if items[i].ItemID != itemID {
			continue
		}
		if items[i].Quantity < qty {
			return nil, fmt.Errorf("not enough %q: have %d, removing %d", items[i].Name, items[i].Quantity, qty)
		}
		items[i].Quantity -= qty
		if items[i].Quantity <= 0 {
			return append(items[:i], items[i+1:]...), nil
		}
		return items, nil
	}
	return nil, fmt.Errorf("item %q not found", itemID)
}

// persistInventory saves the updated inventory to the database.
func (h *APIHandler) persistInventory(ctx context.Context, charID uuid.UUID, items []character.InventoryItem) error {
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	_, err = h.store.UpdateCharacterInventory(ctx, refdata.UpdateCharacterInventoryParams{
		ID:        charID,
		Inventory: pqtype.NullRawMessage{RawMessage: data, Valid: true},
	})
	return err
}
