package inventory

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/stretchr/testify/assert"
)

func TestFormatInventory_FullInventory(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "longsword-1", Name: "+1 Longsword", Quantity: 1, Equipped: true, EquipSlot: "main hand", Type: "weapon", IsMagic: true, RequiresAttunement: true, Rarity: "uncommon"},
		{ItemID: "shortbow", Name: "Shortbow", Quantity: 1, Type: "weapon"},
		{ItemID: "chain-mail", Name: "Chain Mail", Quantity: 1, Equipped: true, Type: "armor"},
		{ItemID: "shield", Name: "Shield", Quantity: 1, Equipped: true, EquipSlot: "off-hand", Type: "armor"},
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Equipped: true, Type: "magic_item", IsMagic: true, RequiresAttunement: true, Rarity: "uncommon"},
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true, Rarity: "rare", Charges: 5, MaxCharges: 7},
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
		{ItemID: "antitoxin", Name: "Antitoxin", Quantity: 1, Type: "consumable"},
		{ItemID: "arrows", Name: "Arrows", Quantity: 18, Type: "ammunition"},
		{ItemID: "rope-50ft", Name: "Rope (50ft)", Quantity: 1, Type: "other"},
		{ItemID: "torch", Name: "Torch", Quantity: 3, Type: "other"},
	}

	attunement := []character.AttunementSlot{
		{ItemID: "longsword-1", Name: "+1 Longsword"},
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection"},
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs"},
	}

	result := FormatInventory("Aria", 23, items, attunement)

	assert.Contains(t, result, "Aria's Inventory (23 gp)")
	assert.Contains(t, result, "+1 Longsword [uncommon]")
	assert.Contains(t, result, "(equipped, main hand, attuned)")
	assert.Contains(t, result, "Shortbow")
	assert.Contains(t, result, "Chain Mail (equipped)")
	assert.Contains(t, result, "Shield (equipped, off-hand)")
	assert.Contains(t, result, "Cloak of Protection [uncommon]")
	assert.Contains(t, result, "Healing Potion ×2")
	assert.Contains(t, result, "Antitoxin ×1")
	assert.Contains(t, result, "Arrows ×18")
	assert.Contains(t, result, "Rope (50ft)")
	assert.Contains(t, result, "Torch ×3")
	assert.Contains(t, result, "Wand of Fireballs [rare]")
	assert.Contains(t, result, "5/7 charges")
	assert.Contains(t, result, "3/3 slots")
}

func TestFormatInventory_Empty(t *testing.T) {
	result := FormatInventory("Gorak", 0, nil, nil)
	assert.Contains(t, result, "Gorak's Inventory (0 gp)")
	assert.Contains(t, result, "empty")
}

func TestUseConsumable_HealingPotion(t *testing.T) {
	roller := func(max int) int { return 3 } // always roll 3
	svc := NewService(roller)

	items := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}

	result, err := svc.UseConsumable(UseInput{
		Items:     items,
		ItemID:    "healing-potion",
		HPCurrent: 10,
		HPMax:     30,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, result.UpdatedItems[0].Quantity)
	assert.Equal(t, 8, result.HealingDone) // 2d4+2 with always-3 = 3+3+2 = 8
	assert.Equal(t, 18, result.HPAfter)
	assert.True(t, result.AutoResolved)
	assert.Contains(t, result.Message, "Healing Potion")
}

func TestUseConsumable_ItemNotFound(t *testing.T) {
	svc := NewService(nil)

	_, err := svc.UseConsumable(UseInput{
		Items:  nil,
		ItemID: "healing-potion",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUseConsumable_NotConsumable(t *testing.T) {
	svc := NewService(nil)

	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}

	_, err := svc.UseConsumable(UseInput{
		Items:  items,
		ItemID: "longsword",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a consumable")
}

func TestUseConsumable_QuantityZero(t *testing.T) {
	svc := NewService(nil)

	items := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 0, Type: "consumable"},
	}

	_, err := svc.UseConsumable(UseInput{
		Items:  items,
		ItemID: "healing-potion",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "none left")
}

func TestUseConsumable_RemovesItemAtZero(t *testing.T) {
	roller := func(max int) int { return 2 }
	svc := NewService(roller)

	items := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 1, Type: "consumable"},
	}

	result, err := svc.UseConsumable(UseInput{
		Items:     items,
		ItemID:    "healing-potion",
		HPCurrent: 10,
		HPMax:     30,
	})

	assert.NoError(t, err)
	assert.Empty(t, result.UpdatedItems) // removed when quantity hits 0
}

func TestUseConsumable_GreaterHealingPotion(t *testing.T) {
	roller := func(max int) int { return 4 } // always roll 4
	svc := NewService(roller)

	items := []character.InventoryItem{
		{ItemID: "greater-healing-potion", Name: "Greater Healing Potion", Quantity: 1, Type: "consumable"},
	}

	result, err := svc.UseConsumable(UseInput{
		Items:     items,
		ItemID:    "greater-healing-potion",
		HPCurrent: 5,
		HPMax:     50,
	})

	assert.NoError(t, err)
	assert.Equal(t, 20, result.HealingDone) // 4d4+4 with always-4 = 4*4+4 = 20
	assert.Equal(t, 25, result.HPAfter)
	assert.True(t, result.AutoResolved)
}

func TestUseConsumable_UnknownRoutesDM(t *testing.T) {
	svc := NewService(nil)

	items := []character.InventoryItem{
		{ItemID: "ball-bearings", Name: "Ball Bearings", Quantity: 1, Type: "consumable"},
	}

	result, err := svc.UseConsumable(UseInput{
		Items:     items,
		ItemID:    "ball-bearings",
		HPCurrent: 10,
		HPMax:     30,
	})

	assert.NoError(t, err)
	assert.False(t, result.AutoResolved)
	assert.True(t, result.DMQueueRequired)
	assert.Contains(t, result.Message, "DM")
}

func TestUseConsumable_HealingCappedAtMax(t *testing.T) {
	roller := func(max int) int { return 4 }
	svc := NewService(roller)

	items := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 1, Type: "consumable"},
	}

	result, err := svc.UseConsumable(UseInput{
		Items:     items,
		ItemID:    "healing-potion",
		HPCurrent: 28,
		HPMax:     30,
	})

	assert.NoError(t, err)
	assert.Equal(t, 2, result.HealingDone) // capped: max 30, current 28
	assert.Equal(t, 30, result.HPAfter)
}

func TestGiveItem_Success(t *testing.T) {
	giverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	receiverItems := []character.InventoryItem{
		{ItemID: "torch", Name: "Torch", Quantity: 3, Type: "other"},
	}

	result, err := GiveItem(GiveInput{
		GiverItems:    giverItems,
		ReceiverItems: receiverItems,
		ItemID:        "healing-potion",
		GiverName:     "Aria",
		ReceiverName:  "Gorak",
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, result.UpdatedGiverItems[0].Quantity)
	assert.Equal(t, 2, len(result.UpdatedReceiverItems)) // torch + healing potion
	// Receiver should have the healing potion
	found := false
	for _, item := range result.UpdatedReceiverItems {
		if item.ItemID == "healing-potion" {
			found = true
			assert.Equal(t, 1, item.Quantity)
		}
	}
	assert.True(t, found)
	assert.Contains(t, result.Message, "Aria")
	assert.Contains(t, result.Message, "Gorak")
	assert.Contains(t, result.Message, "Healing Potion")
}

func TestGiveItem_ReceiverAlreadyHasItem(t *testing.T) {
	giverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	receiverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 1, Type: "consumable"},
	}

	result, err := GiveItem(GiveInput{
		GiverItems:    giverItems,
		ReceiverItems: receiverItems,
		ItemID:        "healing-potion",
		GiverName:     "Aria",
		ReceiverName:  "Gorak",
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, result.UpdatedGiverItems[0].Quantity)
	assert.Equal(t, 2, result.UpdatedReceiverItems[0].Quantity)
}

func TestGiveItem_RemovesAtZero(t *testing.T) {
	giverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 1, Type: "consumable"},
	}

	result, err := GiveItem(GiveInput{
		GiverItems:    giverItems,
		ReceiverItems: nil,
		ItemID:        "healing-potion",
		GiverName:     "Aria",
		ReceiverName:  "Gorak",
	})

	assert.NoError(t, err)
	assert.Empty(t, result.UpdatedGiverItems)
	assert.Equal(t, 1, len(result.UpdatedReceiverItems))
}

func TestGiveItem_NotFound(t *testing.T) {
	_, err := GiveItem(GiveInput{
		GiverItems: nil,
		ItemID:     "healing-potion",
		GiverName:  "Aria",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGiveItem_NoneLeft(t *testing.T) {
	giverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 0, Type: "consumable"},
	}
	_, err := GiveItem(GiveInput{
		GiverItems: giverItems,
		ItemID:     "healing-potion",
		GiverName:  "Aria",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "none left")
}

func TestFormatInventory_UnidentifiedItem(t *testing.T) {
	identified := true
	unidentified := false
	items := []character.InventoryItem{
		{ItemID: "mystery-sword", Name: "Flame Tongue", Quantity: 1, Type: "weapon", IsMagic: true, Identified: &unidentified},
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &identified},
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}

	result := FormatInventory("Aria", 10, items, nil)

	// Unidentified magic item should show as "Unidentified weapon"
	assert.Contains(t, result, "Unidentified weapon")
	assert.NotContains(t, result, "Flame Tongue")

	// Identified items show normally
	assert.Contains(t, result, "Cloak of Protection")

	// Non-magic items with nil Identified show normally
	assert.Contains(t, result, "Longsword")
}

func TestFormatInventory_UnidentifiedItemNilMeansIdentified(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true},
	}

	result := FormatInventory("Aria", 10, items, nil)
	// nil Identified means identified by default
	assert.Contains(t, result, "Cloak of Protection")
}

func TestUseConsumable_Antitoxin(t *testing.T) {
	svc := NewService(nil)

	items := []character.InventoryItem{
		{ItemID: "antitoxin", Name: "Antitoxin", Quantity: 1, Type: "consumable"},
	}

	result, err := svc.UseConsumable(UseInput{
		Items:  items,
		ItemID: "antitoxin",
	})

	assert.NoError(t, err)
	assert.True(t, result.AutoResolved)
	assert.Contains(t, result.Message, "advantage")
	assert.Contains(t, result.Message, "poison")
	assert.Equal(t, "antitoxin", result.AppliedCondition)
}
