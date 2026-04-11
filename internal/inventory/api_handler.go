package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// EncounterPublisher fans out a fresh encounter snapshot over the dashboard
// WebSocket hub whenever an inventory or gold mutation touches a character
// currently in an active encounter. It is injected (optionally) into the
// APIHandler so tests can use a fake and the package stays decoupled from
// the concrete dashboard.Publisher.
type EncounterPublisher interface {
	PublishEncounterSnapshot(ctx context.Context, encounterID uuid.UUID) error
}

// EncounterLookup resolves the active encounter (if any) that currently
// contains the given character. Returns (encID, true, nil) when the character
// is a combatant in an active encounter; (uuid.Nil, false, nil) when not in
// combat; or a non-nil error on store failure.
type EncounterLookup interface {
	ActiveEncounterIDForCharacter(ctx context.Context, characterID uuid.UUID) (uuid.UUID, bool, error)
}

// CharacterStore defines the database operations for inventory management.
type CharacterStore interface {
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	UpdateCharacterInventory(ctx context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error)
	UpdateCharacterGold(ctx context.Context, arg refdata.UpdateCharacterGoldParams) (refdata.Character, error)
	UpdateCharacterInventoryAndGold(ctx context.Context, arg refdata.UpdateCharacterInventoryAndGoldParams) (refdata.Character, error)
	UpdateTwoCharacterInventories(ctx context.Context, id1 uuid.UUID, inv1 pqtype.NullRawMessage, id2 uuid.UUID, inv2 pqtype.NullRawMessage) error
}

// APIHandler handles DM inventory management HTTP endpoints.
type APIHandler struct {
	store       CharacterStore
	combatLogFn func(msg string)
	publisher   EncounterPublisher
	lookup      EncounterLookup
}

// NewAPIHandler creates a new APIHandler.
func NewAPIHandler(store CharacterStore) *APIHandler {
	return &APIHandler{store: store}
}

// SetCombatLogFunc sets the callback for posting messages to #combat-log.
func (h *APIHandler) SetCombatLogFunc(fn func(msg string)) {
	h.combatLogFn = fn
}

// SetPublisher wires the optional dashboard publisher and encounter lookup.
// A nil publisher is tolerated. Publish errors are logged but never surfaced
// to callers so a dashboard hiccup cannot undo a committed DB mutation.
func (h *APIHandler) SetPublisher(p EncounterPublisher, lookup EncounterLookup) {
	h.publisher = p
	h.lookup = lookup
}

// publishForCharacter looks up the character's active encounter (if any) and
// fires the publisher. Silently no-ops when the character is not in combat,
// when the publisher is unset, or when the lookup/publish fails.
func (h *APIHandler) publishForCharacter(ctx context.Context, charID uuid.UUID) {
	if h.publisher == nil || h.lookup == nil {
		return
	}
	encID, ok, err := h.lookup.ActiveEncounterIDForCharacter(ctx, charID)
	if err != nil {
		log.Printf("inventory: active encounter lookup failed for %s: %v", charID, err)
		return
	}
	if !ok {
		return
	}
	if err := h.publisher.PublishEncounterSnapshot(ctx, encID); err != nil {
		log.Printf("inventory: encounter snapshot publish failed for %s: %v", encID, err)
	}
}

// logCombat posts a message to #combat-log if the callback is configured.
func (h *APIHandler) logCombat(msg string) {
	if h.combatLogFn != nil {
		h.combatLogFn(msg)
	}
}

// AddItemRequest is the request body for adding an item to a character.
type AddItemRequest struct {
	CharacterID string                 `json:"character_id"`
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

// IdentifyItemRequest is the request body for identifying/hiding a magic item.
type IdentifyItemRequest struct {
	CharacterID string `json:"character_id"`
	ItemID      string `json:"item_id"`
	Identified  bool   `json:"identified"`
}

// HandleIdentifyItem handles POST /api/inventory/identify.
// DM can set identified = true (reveal) or false (hide) on a magic item.
func (h *APIHandler) HandleIdentifyItem(w http.ResponseWriter, r *http.Request) {
	var req IdentifyItemRequest
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

	result, err := SetItemIdentified(SetIdentifiedInput{
		Items:      items,
		ItemID:     req.ItemID,
		Identified: req.Identified,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.persistInventory(r.Context(), charID, result.UpdatedItems); err != nil {
		http.Error(w, "failed to update inventory", http.StatusInternalServerError)
		return
	}

	h.publishForCharacter(r.Context(), charID)
	h.logCombat(fmt.Sprintf("🔍 DM updated identification of **%s** for **%s** (identified: %v).", result.ItemName, char.Name, req.Identified))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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

	idx := findItemIndex(items, req.Item.ItemID)
	if idx >= 0 {
		items[idx].Quantity += req.Item.Quantity
	} else {
		items = append(items, req.Item)
	}

	if err := h.persistInventory(r.Context(), charID, items); err != nil {
		http.Error(w, "failed to update inventory", http.StatusInternalServerError)
		return
	}

	h.publishForCharacter(r.Context(), charID)
	h.logCombat(fmt.Sprintf("📦 DM added **%s** ×%d to **%s**'s inventory.", req.Item.Name, req.Item.Quantity, char.Name))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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

	updated, itemName, err := removeItem(items, req.ItemID, qty)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.persistInventory(r.Context(), charID, updated); err != nil {
		http.Error(w, "failed to update inventory", http.StatusInternalServerError)
		return
	}

	h.publishForCharacter(r.Context(), charID)
	h.logCombat(fmt.Sprintf("📦 DM removed **%s** ×%d from **%s**'s inventory.", itemName, qty, char.Name))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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

	// Find item to transfer
	idx := findItemIndex(fromItems, req.ItemID)
	if idx == -1 {
		http.Error(w, fmt.Sprintf("item %q not found", req.ItemID), http.StatusBadRequest)
		return
	}
	item := fromItems[idx]
	if item.Quantity < qty {
		http.Error(w, fmt.Sprintf("not enough %q: have %d, need %d", item.Name, item.Quantity, qty), http.StatusBadRequest)
		return
	}

	fromItems = decrementItem(fromItems, idx, qty)
	toItems = AddItemQuantity(toItems, item, qty)

	// Persist both inventories atomically
	fromInvJSON, err := character.MarshalInventory(fromItems)
	if err != nil {
		http.Error(w, "failed to marshal inventory", http.StatusInternalServerError)
		return
	}
	toInvJSON, err := character.MarshalInventory(toItems)
	if err != nil {
		http.Error(w, "failed to marshal inventory", http.StatusInternalServerError)
		return
	}
	fromInvMsg := pqtype.NullRawMessage{RawMessage: fromInvJSON, Valid: true}
	toInvMsg := pqtype.NullRawMessage{RawMessage: toInvJSON, Valid: true}

	if err := h.store.UpdateTwoCharacterInventories(r.Context(), fromID, fromInvMsg, toID, toInvMsg); err != nil {
		http.Error(w, "failed to update inventories", http.StatusInternalServerError)
		return
	}

	h.publishForCharacter(r.Context(), fromID)
	h.publishForCharacter(r.Context(), toID)
	h.logCombat(fmt.Sprintf("📦 DM transferred **%s** ×%d from **%s** to **%s**.", item.Name, qty, fromChar.Name, toChar.Name))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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

	// Get character for logging
	char, err := h.store.GetCharacter(r.Context(), charID)
	if err != nil {
		http.Error(w, "character not found", http.StatusNotFound)
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

	h.publishForCharacter(r.Context(), charID)
	h.logCombat(fmt.Sprintf("💰 DM set **%s**'s gold to **%d** gp (was %d gp).", char.Name, req.Gold, char.Gold))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// parseInventoryItems extracts inventory items from a character.
func parseInventoryItems(char refdata.Character) []character.InventoryItem {
	var items []character.InventoryItem
	if char.Inventory.Valid {
		_ = json.Unmarshal(char.Inventory.RawMessage, &items)
	}
	return items
}

// removeItem removes qty of the item from the list, returning updated items and removed item name.
func removeItem(items []character.InventoryItem, itemID string, qty int) ([]character.InventoryItem, string, error) {
	idx := findItemIndex(items, itemID)
	if idx == -1 {
		return nil, "", fmt.Errorf("item %q not found", itemID)
	}
	if items[idx].Quantity < qty {
		return nil, "", fmt.Errorf("not enough %q: have %d, removing %d", items[idx].Name, items[idx].Quantity, qty)
	}
	name := items[idx].Name
	return decrementItem(items, idx, qty), name, nil
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
