package inventory

import (
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/character"
)

const maxAttunementSlots = 3

// AttuneInput holds parameters for attuning to a magic item.
type AttuneInput struct {
	Items                 []character.InventoryItem
	Slots                 []character.AttunementSlot
	ItemID                string
	Classes               []character.ClassEntry
	AttunementRestriction string // e.g. "paladin", "cleric" — empty means no restriction
}

// AttuneResult holds the result of an attunement operation.
type AttuneResult struct {
	UpdatedSlots []character.AttunementSlot
	Message      string
}

// Attune attunes a character to a magic item. Validates:
// - item is in inventory
// - item requires attunement
// - character is not already attuned to it
// - fewer than 3 attuned items
// - class restriction (if any)
func Attune(input AttuneInput) (AttuneResult, error) {
	if len(input.Slots) >= maxAttunementSlots {
		return AttuneResult{}, fmt.Errorf("❌ You already have %d attuned items. Use `/unattune [item]` to free a slot.", maxAttunementSlots)
	}

	idx := findItemIndex(input.Items, input.ItemID)
	if idx == -1 {
		return AttuneResult{}, fmt.Errorf("item %q not found in inventory", input.ItemID)
	}

	item := input.Items[idx]
	if !item.RequiresAttunement {
		return AttuneResult{}, fmt.Errorf("%q does not require attunement", item.Name)
	}

	if isAttuned(input.Slots, input.ItemID) {
		return AttuneResult{}, fmt.Errorf("already attuned to %q", item.Name)
	}

	if input.AttunementRestriction != "" && !meetsClassRestriction(input.Classes, input.AttunementRestriction) {
		return AttuneResult{}, fmt.Errorf("attunement restriction not met: requires %s", input.AttunementRestriction)
	}

	updated := make([]character.AttunementSlot, len(input.Slots))
	copy(updated, input.Slots)
	updated = append(updated, character.AttunementSlot{
		ItemID: item.ItemID,
		Name:   item.Name,
	})

	return AttuneResult{
		UpdatedSlots: updated,
		Message:      fmt.Sprintf("✨ Attuned to **%s**. (%d/%d attunement slots)", item.Name, len(updated), maxAttunementSlots),
	}, nil
}

// UnattuneInput holds parameters for ending attunement.
type UnattuneInput struct {
	Slots  []character.AttunementSlot
	ItemID string
}

// UnattuneResult holds the result of ending attunement.
type UnattuneResult struct {
	UpdatedSlots []character.AttunementSlot
	ItemName     string
	Message      string
}

// Unattune ends attunement with a magic item.
func Unattune(input UnattuneInput) (UnattuneResult, error) {
	idx := -1
	for i, slot := range input.Slots {
		if slot.ItemID == input.ItemID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return UnattuneResult{}, fmt.Errorf("not attuned to item %q", input.ItemID)
	}

	name := input.Slots[idx].Name
	updated := make([]character.AttunementSlot, 0, len(input.Slots)-1)
	updated = append(updated, input.Slots[:idx]...)
	updated = append(updated, input.Slots[idx+1:]...)

	return UnattuneResult{
		UpdatedSlots: updated,
		ItemName:     name,
		Message:      fmt.Sprintf("🔓 Unattuned from **%s**. (%d/%d attunement slots)", name, len(updated), maxAttunementSlots),
	}, nil
}

// meetsClassRestriction checks if any of the character's classes match the restriction.
func meetsClassRestriction(classes []character.ClassEntry, restriction string) bool {
	restriction = strings.ToLower(restriction)
	for _, c := range classes {
		if strings.ToLower(c.Class) == restriction {
			return true
		}
	}
	return false
}
