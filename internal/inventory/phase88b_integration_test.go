package inventory

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_AttuneLimitAndUnattune tests the full attune/unattune cycle.
func TestIntegration_AttuneLimitAndUnattune(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "item-1", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
		{ItemID: "item-2", Name: "Ring of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
		{ItemID: "item-3", Name: "Amulet of Health", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
		{ItemID: "item-4", Name: "Boots of Speed", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
	}

	// Attune to 3 items
	var slots []character.AttunementSlot
	for i := 0; i < 3; i++ {
		result, err := Attune(AttuneInput{Items: items, Slots: slots, ItemID: items[i].ItemID})
		require.NoError(t, err)
		slots = result.UpdatedSlots
	}
	assert.Len(t, slots, 3)

	// 4th attunement should fail
	_, err := Attune(AttuneInput{Items: items, Slots: slots, ItemID: "item-4"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "3 attuned items")

	// Unattune from one
	uResult, err := Unattune(UnattuneInput{Slots: slots, ItemID: "item-2"})
	require.NoError(t, err)
	slots = uResult.UpdatedSlots
	assert.Len(t, slots, 2)

	// Now 4th attunement should succeed
	aResult, err := Attune(AttuneInput{Items: items, Slots: slots, ItemID: "item-4"})
	require.NoError(t, err)
	assert.Len(t, aResult.UpdatedSlots, 3)
}

// TestIntegration_ActiveAbilityChargeDeduction tests using charges from a magic item.
func TestIntegration_ActiveAbilityChargeDeduction(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true, Charges: 7, MaxCharges: 7},
	}
	attunement := []character.AttunementSlot{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs"},
	}

	// Use 3 charges
	result, err := UseCharges(UseChargesInput{
		Items:      items,
		Attunement: attunement,
		ItemID:     "wand-of-fireballs",
		Amount:     3,
	})
	require.NoError(t, err)
	assert.Equal(t, 4, result.ChargesLeft)

	// Use 4 more charges
	result2, err := UseCharges(UseChargesInput{
		Items:      result.UpdatedItems,
		Attunement: attunement,
		ItemID:     "wand-of-fireballs",
		Amount:     4,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, result2.ChargesLeft)

	// Now using 1 more should fail
	_, err = UseCharges(UseChargesInput{
		Items:      result2.UpdatedItems,
		Attunement: attunement,
		ItemID:     "wand-of-fireballs",
		Amount:     1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient charges")
}

// TestIntegration_UnattuneSuppressionAndIdentification tests the combined flow.
func TestIntegration_UnattuneSuppressionAndIdentification(t *testing.T) {
	unidentified := false
	items := []character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true, Charges: 7, MaxCharges: 7},
		{ItemID: "mystery-ring", Name: "Ring of Invisibility", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &unidentified},
	}

	// Unattuned item should not be usable
	_, err := UseCharges(UseChargesInput{
		Items:      items,
		Attunement: nil,
		ItemID:     "wand-of-fireballs",
		Amount:     1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "attunement")

	// Unidentified item in format should show generic type
	result := FormatInventory("Aria", 10, items, nil)
	assert.Contains(t, result, "Unidentified magic_item")
	assert.NotContains(t, result, "Ring of Invisibility")
}

// TestIntegration_DawnRechargeWithDestruction tests dawn recharge no longer destroys items.
// Destroy-on-zero check is now in UseCharges (at time of last charge spent).
func TestIntegration_DawnRechargeWithDestruction(t *testing.T) {
	// Roller: d6=1 → recharge=1+1=2 for wand, d6=4 → recharge=4+1=5 for staff
	svc := NewService(sequentialRoller([]int{1, 4}))

	items := []character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, Charges: 0, MaxCharges: 7},
		{ItemID: "staff-of-healing", Name: "Staff of Healing", Quantity: 1, Type: "magic_item", IsMagic: true, Charges: 5, MaxCharges: 10},
	}

	result, err := svc.DawnRecharge(DawnRechargeInput{
		Items: items,
		RechargeInfo: map[string]RechargeInfo{
			"wand-of-fireballs": {Dice: "1d6+1", DestroyOnZero: true},
			"staff-of-healing":  {Dice: "1d6+1"},
		},
	})

	require.NoError(t, err)
	// Both items survive — no destroy check at dawn anymore
	assert.Len(t, result.UpdatedItems, 2)
	assert.Equal(t, "wand-of-fireballs", result.UpdatedItems[0].ItemID)
	assert.Equal(t, 2, result.UpdatedItems[0].Charges) // 0 + (1+1) = 2
	assert.Equal(t, "staff-of-healing", result.UpdatedItems[1].ItemID)
	assert.Equal(t, 10, result.UpdatedItems[1].Charges) // 5 + (4+1) = 9, capped at 10

	// Check recharged info
	require.Len(t, result.Recharged, 2)
	assert.False(t, result.Recharged[0].Destroyed)
	assert.False(t, result.Recharged[1].Destroyed)
}
