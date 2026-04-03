package inventory

import (
	"fmt"

	"github.com/ab/dndnd/internal/character"
)

// IdentifyInput holds parameters for identifying a magic item.
type IdentifyInput struct {
	Items  []character.InventoryItem
	ItemID string
}

// IdentifyResult holds the result of an identification operation.
type IdentifyResult struct {
	UpdatedItems []character.InventoryItem
	ItemName     string
	Message      string
}

// IdentifyItem sets identified = true on the specified inventory item.
func IdentifyItem(input IdentifyInput) (IdentifyResult, error) {
	idx := findItemIndex(input.Items, input.ItemID)
	if idx == -1 {
		return IdentifyResult{}, fmt.Errorf("item %q not found in inventory", input.ItemID)
	}

	item := input.Items[idx]
	if !item.IsMagic {
		return IdentifyResult{}, fmt.Errorf("%q is not a magic item", item.Name)
	}

	if !isUnidentified(item) {
		return IdentifyResult{}, fmt.Errorf("%q is already identified", item.Name)
	}

	updated := make([]character.InventoryItem, len(input.Items))
	copy(updated, input.Items)
	identified := true
	updated[idx].Identified = &identified

	return IdentifyResult{
		UpdatedItems: updated,
		ItemName:     item.Name,
		Message:      fmt.Sprintf("🔍 **%s** has been identified!", item.Name),
	}, nil
}

// CastIdentifyInput holds parameters for casting the Identify spell on an item.
type CastIdentifyInput struct {
	Items      []character.InventoryItem
	ItemID     string
	KnowsSpell bool              // whether the caster knows/has Identify prepared
	SpellSlots map[int]int       // slot level -> remaining slots
	SlotLevel  int               // which slot level to use (ignored if ritual)
	IsRitual   bool              // cast as ritual (no slot consumed, extra 10 min)
}

// CastIdentifyResult holds the result of casting Identify.
type CastIdentifyResult struct {
	UpdatedItems   []character.InventoryItem
	ItemName       string
	SlotsRemaining int
	IsRitual       bool
	Message        string
}

// CastIdentify validates and applies the Identify spell to a magic item.
func CastIdentify(input CastIdentifyInput) (CastIdentifyResult, error) {
	if !input.KnowsSpell {
		return CastIdentifyResult{}, fmt.Errorf("caster does not know the Identify spell")
	}

	if !input.IsRitual {
		remaining, ok := input.SpellSlots[input.SlotLevel]
		if !ok || remaining <= 0 {
			return CastIdentifyResult{}, fmt.Errorf("no spell slot available at level %d", input.SlotLevel)
		}
	}

	idx := findItemIndex(input.Items, input.ItemID)
	if idx == -1 {
		return CastIdentifyResult{}, fmt.Errorf("item %q not found in inventory", input.ItemID)
	}

	item := input.Items[idx]
	if !item.IsMagic {
		return CastIdentifyResult{}, fmt.Errorf("%q is not a magic item", item.Name)
	}

	if !isUnidentified(item) {
		return CastIdentifyResult{}, fmt.Errorf("%q is already identified", item.Name)
	}

	updated := make([]character.InventoryItem, len(input.Items))
	copy(updated, input.Items)
	identified := true
	updated[idx].Identified = &identified

	slotsRemaining := 0
	if !input.IsRitual {
		slotsRemaining = input.SpellSlots[input.SlotLevel] - 1
	}

	return CastIdentifyResult{
		UpdatedItems:   updated,
		ItemName:       item.Name,
		SlotsRemaining: slotsRemaining,
		IsRitual:       input.IsRitual,
		Message:        fmt.Sprintf("✨ Cast **Identify** on **%s** — its properties are now revealed!", item.Name),
	}, nil
}

// StudyItemDuringRest identifies a magic item during a short rest (1-hour study).
// This is an alternative to the Identify spell per D&D 5e SRD.
func StudyItemDuringRest(input IdentifyInput) (IdentifyResult, error) {
	idx := findItemIndex(input.Items, input.ItemID)
	if idx == -1 {
		return IdentifyResult{}, fmt.Errorf("item %q not found in inventory", input.ItemID)
	}

	item := input.Items[idx]
	if !item.IsMagic {
		return IdentifyResult{}, fmt.Errorf("%q is not a magic item", item.Name)
	}

	if !isUnidentified(item) {
		return IdentifyResult{}, fmt.Errorf("%q is already identified", item.Name)
	}

	updated := make([]character.InventoryItem, len(input.Items))
	copy(updated, input.Items)
	identified := true
	updated[idx].Identified = &identified

	return IdentifyResult{
		UpdatedItems: updated,
		ItemName:     item.Name,
		Message:      fmt.Sprintf("📖 You studied **%s** during your short rest and identified its properties!", item.Name),
	}, nil
}

// DetectMagicItems returns all magic items in the inventory.
// This reveals the presence of magic but not full properties.
func DetectMagicItems(items []character.InventoryItem) []character.InventoryItem {
	var result []character.InventoryItem
	for _, item := range items {
		if item.IsMagic {
			result = append(result, item)
		}
	}
	return result
}

// SetIdentifiedInput holds parameters for DM setting identified status.
type SetIdentifiedInput struct {
	Items      []character.InventoryItem
	ItemID     string
	Identified bool
}

// SetItemIdentified allows a DM to set the identified status on a magic item.
func SetItemIdentified(input SetIdentifiedInput) (IdentifyResult, error) {
	idx := findItemIndex(input.Items, input.ItemID)
	if idx == -1 {
		return IdentifyResult{}, fmt.Errorf("item %q not found in inventory", input.ItemID)
	}

	item := input.Items[idx]
	if !item.IsMagic {
		return IdentifyResult{}, fmt.Errorf("%q is not a magic item", item.Name)
	}

	updated := make([]character.InventoryItem, len(input.Items))
	copy(updated, input.Items)
	val := input.Identified
	updated[idx].Identified = &val

	action := "revealed"
	if !input.Identified {
		action = "hidden"
	}

	return IdentifyResult{
		UpdatedItems: updated,
		ItemName:     item.Name,
		Message:      fmt.Sprintf("🔍 DM %s properties of **%s**.", action, item.Name),
	}, nil
}
