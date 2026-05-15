package inventory

import (
	"fmt"

	"github.com/ab/dndnd/internal/character"
)

// UseChargesInput holds parameters for using charges from a magic item.
type UseChargesInput struct {
	Items         []character.InventoryItem
	Attunement    []character.AttunementSlot
	ItemID        string
	Amount        int
	DestroyOnZero bool // if true and charges reach 0, roll d20 — on 1, item is destroyed
}

// UseChargesResult holds the result of using charges.
type UseChargesResult struct {
	UpdatedItems []character.InventoryItem
	ItemName     string
	ChargesUsed  int
	ChargesLeft  int
	Destroyed    bool
	Message      string
}

// UseCharges deducts charges from a magic item in the inventory.
// Validates the item exists, is magic, has charges, attunement is met, and sufficient charges remain.
// If DestroyOnZero is set and charges reach 0, rolls a d20 — on a 1 the item is destroyed.
func (s *Service) UseCharges(input UseChargesInput) (UseChargesResult, error) {
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

	result := UseChargesResult{
		UpdatedItems: updated,
		ItemName:     item.Name,
		ChargesUsed:  input.Amount,
		ChargesLeft:  updated[idx].Charges,
		Message:      fmt.Sprintf("⚡ Used %d charges from **%s** (%d/%d remaining)", input.Amount, item.Name, updated[idx].Charges, item.MaxCharges),
	}

	// Destroy-on-zero check: when last charge is spent, roll d20. On a 1, item is destroyed.
	if input.DestroyOnZero && updated[idx].Charges == 0 {
		roll, err := s.roller.Roll("1d20")
		if err != nil {
			return UseChargesResult{}, fmt.Errorf("rolling destroy check for %s: %w", item.Name, err)
		}
		if roll.Total == 1 {
			result.Destroyed = true
			result.Message = fmt.Sprintf("⚡ Used %d charges from **%s** — 💥 rolled a 1 on d20, the item crumbles to dust!", input.Amount, item.Name)
		}
	}

	return result, nil
}

// UseCharges is a package-level convenience that calls UseCharges without destroy-on-zero logic.
// Deprecated: prefer calling (*Service).UseCharges directly.
func UseCharges(input UseChargesInput) (UseChargesResult, error) {
	svc := NewService(nil)
	return svc.UseCharges(input)
}
