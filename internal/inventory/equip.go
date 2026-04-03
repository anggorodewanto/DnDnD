package inventory

import (
	"fmt"

	"github.com/ab/dndnd/internal/character"
)

// EquipInput holds parameters for equipping an item.
type EquipInput struct {
	Items           []character.InventoryItem
	ItemID          string
	OffHand         bool
	Armor           bool
	AttunementSlots []character.AttunementSlot
}

// EquipResult holds the result of an equip operation.
type EquipResult struct {
	UpdatedItems []character.InventoryItem
	EquipSlot    string
	Message      string
	Warning      string
}

// Equip equips an item from the character's inventory.
func Equip(input EquipInput) (EquipResult, error) {
	idx := findItemIndex(input.Items, input.ItemID)
	if idx == -1 {
		return EquipResult{}, fmt.Errorf("item %q not found in inventory", input.ItemID)
	}

	item := input.Items[idx]
	if item.Equipped {
		return EquipResult{}, fmt.Errorf("%q is already equipped", item.Name)
	}

	slot := "main_hand"
	if input.OffHand {
		slot = "off_hand"
	}
	if input.Armor {
		slot = "armor"
	}

	updated := make([]character.InventoryItem, len(input.Items))
	copy(updated, input.Items)
	updated[idx].Equipped = true
	updated[idx].EquipSlot = slot

	var warning string
	if item.RequiresAttunement && !isAttuned(input.AttunementSlots, input.ItemID) {
		warning = fmt.Sprintf("⚠️ This item requires attunement. Use `/attune %s` during a short rest to activate its properties.", item.Name)
	}

	return EquipResult{
		UpdatedItems: updated,
		EquipSlot:    slot,
		Message:      fmt.Sprintf("🛡️ Equipped **%s** (%s).", item.Name, slot),
		Warning:      warning,
	}, nil
}

// isAttuned checks if an item is in the attunement slots.
func isAttuned(slots []character.AttunementSlot, itemID string) bool {
	for _, s := range slots {
		if s.ItemID == itemID {
			return true
		}
	}
	return false
}
