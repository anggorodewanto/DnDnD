package inventory

import (
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
)

// Item type categories for display grouping.
const (
	TypeWeapon     = "weapon"
	TypeArmor      = "armor"
	TypeMagicItem  = "magic_item"
	TypeConsumable = "consumable"
	TypeAmmunition = "ammunition"
	TypeOther      = "other"
)

// categoryOrder defines the display order and emoji for each category.
var categoryOrder = []struct {
	Type  string
	Emoji string
	Label string
}{
	{TypeWeapon, "\u2694\ufe0f", "Weapons"},
	{TypeArmor, "\U0001f6e1\ufe0f", "Armor"},
	{TypeMagicItem, "\U0001f48d", "Magic Items"},
	{TypeConsumable, "\U0001f9ea", "Consumables"},
	{TypeAmmunition, "\U0001f3f9", "Ammunition"},
	{TypeOther, "\U0001f4e6", "Other"},
}

// findItemIndex returns the index of the item with the given ID, or -1.
func findItemIndex(items []character.InventoryItem, itemID string) int {
	for i, item := range items {
		if item.ItemID == itemID {
			return i
		}
	}
	return -1
}

// decrementItem removes qty units of the item at idx, removing the entry if quantity reaches 0.
// Returns a new slice (does not mutate the input).
func decrementItem(items []character.InventoryItem, idx int, qty int) []character.InventoryItem {
	updated := make([]character.InventoryItem, len(items))
	copy(updated, items)
	updated[idx].Quantity -= qty
	if updated[idx].Quantity <= 0 {
		updated = append(updated[:idx], updated[idx+1:]...)
	}
	return updated
}

// AddItemQuantity adds qty of the item (by ID) to a slice, either incrementing existing or appending.
// source provides the item metadata when adding a new entry.
func AddItemQuantity(items []character.InventoryItem, source character.InventoryItem, qty int) []character.InventoryItem {
	updated := make([]character.InventoryItem, len(items))
	copy(updated, items)
	idx := findItemIndex(updated, source.ItemID)
	if idx >= 0 {
		updated[idx].Quantity += qty
		return updated
	}
	newItem := source
	newItem.Quantity = qty
	newItem.Equipped = false
	newItem.EquipSlot = ""
	return append(updated, newItem)
}

// Service handles inventory business logic.
type Service struct {
	roller *dice.Roller
}

// NewService creates a new inventory Service.
func NewService(randFn dice.RandSource) *Service {
	return &Service{roller: dice.NewRoller(randFn)}
}

// UseInput holds parameters for using a consumable item.
type UseInput struct {
	Items     []character.InventoryItem
	ItemID    string
	HPCurrent int
	HPMax     int
}

// UseResult holds the result of using a consumable item.
type UseResult struct {
	UpdatedItems    []character.InventoryItem
	HealingDone     int
	HPAfter         int
	AutoResolved    bool
	DMQueueRequired bool
	Message         string
}

// autoResolveItems maps item IDs to their dice expressions for auto-resolve.
var autoResolveItems = map[string]string{
	"healing-potion":         "2d4+2",
	"greater-healing-potion": "4d4+4",
}

// IsPotion reports whether the given item ID is a potion (auto-resolvable
// healing consumable). Used by callers (e.g. /use combat-cost wiring) to
// decide whether to deduct a bonus action vs an action.
func IsPotion(itemID string) bool {
	_, ok := autoResolveItems[itemID]
	return ok
}

// UseConsumable consumes an item and applies its effect if auto-resolvable.
func (s *Service) UseConsumable(input UseInput) (UseResult, error) {
	idx := findItemIndex(input.Items, input.ItemID)
	if idx == -1 {
		return UseResult{}, fmt.Errorf("item %q not found in inventory", input.ItemID)
	}

	item := input.Items[idx]
	if item.Type != TypeConsumable {
		return UseResult{}, fmt.Errorf("%q is not a consumable", item.Name)
	}
	if item.Quantity <= 0 {
		return UseResult{}, fmt.Errorf("%q: none left", item.Name)
	}

	updated := decrementItem(input.Items, idx, 1)

	result := UseResult{UpdatedItems: updated}

	// Check for antitoxin
	if item.ItemID == "antitoxin" {
		result.AutoResolved = true
		result.HPAfter = input.HPCurrent
		result.Message = fmt.Sprintf("\U0001f9ea %s used **%s** \u2014 grants advantage on saving throws vs poison for 1 hour.", item.Name, item.Name)
		return result, nil
	}

	// Check for auto-resolve healing items
	expr, isAutoResolve := autoResolveItems[item.ItemID]
	if !isAutoResolve {
		result.DMQueueRequired = true
		result.HPAfter = input.HPCurrent
		result.Message = fmt.Sprintf("\U0001f9ea Used **%s** \u2014 sent to DM for adjudication.", item.Name)
		return result, nil
	}

	// Roll healing
	roll, err := s.roller.Roll(expr)
	if err != nil {
		return UseResult{}, fmt.Errorf("rolling %s: %w", expr, err)
	}

	healing := roll.Total
	if input.HPCurrent+healing > input.HPMax {
		healing = input.HPMax - input.HPCurrent
	}
	result.HealingDone = healing
	result.HPAfter = input.HPCurrent + healing
	result.AutoResolved = true
	result.Message = fmt.Sprintf("\U0001f9ea Used **%s** \u2014 healed %d HP (%s) \u2192 %d/%d HP",
		item.Name, healing, roll.Breakdown, result.HPAfter, input.HPMax)

	return result, nil
}

// GiveInput holds parameters for giving an item to another character.
type GiveInput struct {
	GiverItems    []character.InventoryItem
	ReceiverItems []character.InventoryItem
	ItemID        string
	GiverName     string
	ReceiverName  string
}

// GiveResult holds the result of a give operation.
type GiveResult struct {
	UpdatedGiverItems    []character.InventoryItem
	UpdatedReceiverItems []character.InventoryItem
	Message              string
}

// GiveItem transfers one unit of an item from giver to receiver.
func GiveItem(input GiveInput) (GiveResult, error) {
	idx := findItemIndex(input.GiverItems, input.ItemID)
	if idx == -1 {
		return GiveResult{}, fmt.Errorf("item %q not found in inventory", input.ItemID)
	}

	item := input.GiverItems[idx]
	if item.Quantity <= 0 {
		return GiveResult{}, fmt.Errorf("%q: none left", item.Name)
	}

	giverUpdated := decrementItem(input.GiverItems, idx, 1)
	receiverUpdated := AddItemQuantity(input.ReceiverItems, item, 1)

	msg := fmt.Sprintf("\U0001f91d **%s** gave **%s** to **%s**.", input.GiverName, item.Name, input.ReceiverName)

	return GiveResult{
		UpdatedGiverItems:    giverUpdated,
		UpdatedReceiverItems: receiverUpdated,
		Message:              msg,
	}, nil
}

// FormatInventory produces a formatted inventory display string.
func FormatInventory(charName string, gold int32, items []character.InventoryItem, attunement []character.AttunementSlot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\U0001f392 %s's Inventory (%d gp)\n", charName, gold)

	if len(items) == 0 {
		b.WriteString("Your inventory is empty.")
		return b.String()
	}

	attunedSet := make(map[string]bool, len(attunement))
	for _, a := range attunement {
		attunedSet[a.ItemID] = true
	}

	grouped := make(map[string][]character.InventoryItem)
	for _, item := range items {
		grouped[item.Type] = append(grouped[item.Type], item)
	}

	for _, cat := range categoryOrder {
		catItems, ok := grouped[cat.Type]
		if !ok {
			continue
		}
		var parts []string
		for _, item := range catItems {
			parts = append(parts, formatItem(item, attunedSet))
		}
		fmt.Fprintf(&b, "%s %s: %s\n", cat.Emoji, cat.Label, strings.Join(parts, ", "))
	}

	fmt.Fprintf(&b, "\n\u2728 = attuned (%d/%d slots)", len(attunement), 3)

	return b.String()
}

// isUnidentified returns true if an item is explicitly marked as unidentified.
func isUnidentified(item character.InventoryItem) bool {
	return item.Identified != nil && !*item.Identified
}

// formatItem produces the display string for a single inventory item.
func formatItem(item character.InventoryItem, attunedSet map[string]bool) string {
	// Consumables and ammunition always show quantity only
	if item.Type == TypeConsumable || item.Type == TypeAmmunition {
		return fmt.Sprintf("%s \u00d7%d", item.Name, item.Quantity)
	}

	// Unidentified magic items show generic description
	if isUnidentified(item) {
		return fmt.Sprintf("Unidentified %s", item.Type)
	}

	if item.Quantity > 1 {
		return fmt.Sprintf("%s \u00d7%d", item.Name, item.Quantity)
	}

	var b strings.Builder
	b.WriteString(item.Name)

	if item.Rarity != "" {
		fmt.Fprintf(&b, " [%s]", item.Rarity)
	}

	isAttuned := attunedSet[item.ItemID]
	if isAttuned {
		b.WriteString(" \u2728")
	}

	var tags []string
	if item.Equipped {
		tags = append(tags, "equipped")
	}
	if item.EquipSlot != "" {
		tags = append(tags, item.EquipSlot)
	}
	if isAttuned {
		tags = append(tags, "attuned")
	}
	if item.Charges > 0 || item.MaxCharges > 0 {
		tags = append(tags, fmt.Sprintf("%d/%d charges", item.Charges, item.MaxCharges))
	}

	if len(tags) > 0 {
		fmt.Fprintf(&b, " (%s)", strings.Join(tags, ", "))
	}

	return b.String()
}
