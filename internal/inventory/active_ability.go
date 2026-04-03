package inventory

import (
	"fmt"

	"github.com/ab/dndnd/internal/character"
)

// UseChargesInput holds parameters for using charges from a magic item.
type UseChargesInput struct {
	Items      []character.InventoryItem
	Attunement []character.AttunementSlot
	ItemID     string
	Amount     int
}

// UseChargesResult holds the result of using charges.
type UseChargesResult struct {
	UpdatedItems   []character.InventoryItem
	ItemName       string
	ChargesUsed    int
	ChargesLeft    int
	Message        string
}

// UseCharges deducts charges from a magic item in the inventory.
// Validates the item exists, is magic, has charges, attunement is met, and sufficient charges remain.
func UseCharges(input UseChargesInput) (UseChargesResult, error) {
	idx := findItemIndex(input.Items, input.ItemID)
	if idx == -1 {
		return UseChargesResult{}, fmt.Errorf("item %q not found in inventory", input.ItemID)
	}

	item := input.Items[idx]
	if !item.IsMagic {
		return UseChargesResult{}, fmt.Errorf("%q is not a magic item", item.Name)
	}

	if item.MaxCharges == 0 {
		return UseChargesResult{}, fmt.Errorf("%q has no charges", item.Name)
	}

	if item.RequiresAttunement && !isAttuned(input.Attunement, input.ItemID) {
		return UseChargesResult{}, fmt.Errorf("%q requires attunement to use its abilities", item.Name)
	}

	if input.Amount > item.Charges {
		return UseChargesResult{}, fmt.Errorf("insufficient charges on %q: need %d, have %d", item.Name, input.Amount, item.Charges)
	}

	updated := make([]character.InventoryItem, len(input.Items))
	copy(updated, input.Items)
	updated[idx].Charges -= input.Amount

	return UseChargesResult{
		UpdatedItems: updated,
		ItemName:     item.Name,
		ChargesUsed:  input.Amount,
		ChargesLeft:  updated[idx].Charges,
		Message:      fmt.Sprintf("⚡ Used %d charges from **%s** (%d/%d remaining)", input.Amount, item.Name, updated[idx].Charges, item.MaxCharges),
	}, nil
}
