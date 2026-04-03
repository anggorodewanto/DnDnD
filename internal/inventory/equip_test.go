package inventory

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEquip_MainHand(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}

	result, err := Equip(EquipInput{
		Items:           items,
		ItemID:          "longsword",
		AttunementSlots: nil,
	})

	require.NoError(t, err)
	assert.True(t, result.UpdatedItems[0].Equipped)
	assert.Equal(t, "main_hand", result.UpdatedItems[0].EquipSlot)
	assert.Contains(t, result.Message, "Longsword")
	assert.Empty(t, result.Warning)
}

func TestEquip_OffHand(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "shield", Name: "Shield", Quantity: 1, Type: "armor"},
	}

	result, err := Equip(EquipInput{
		Items:   items,
		ItemID:  "shield",
		OffHand: true,
	})

	require.NoError(t, err)
	assert.True(t, result.UpdatedItems[0].Equipped)
	assert.Equal(t, "off_hand", result.UpdatedItems[0].EquipSlot)
}

func TestEquip_Armor(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "chain-mail", Name: "Chain Mail", Quantity: 1, Type: "armor"},
	}

	result, err := Equip(EquipInput{
		Items:  items,
		ItemID: "chain-mail",
		Armor:  true,
	})

	require.NoError(t, err)
	assert.True(t, result.UpdatedItems[0].Equipped)
	assert.Equal(t, "armor", result.UpdatedItems[0].EquipSlot)
}

func TestEquip_ItemNotFound(t *testing.T) {
	_, err := Equip(EquipInput{
		Items:  nil,
		ItemID: "longsword",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEquip_AlreadyEquipped(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon", Equipped: true, EquipSlot: "main_hand"},
	}

	_, err := Equip(EquipInput{
		Items:  items,
		ItemID: "longsword",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already equipped")
}

func TestEquip_UnattunedWarning(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
	}

	result, err := Equip(EquipInput{
		Items:           items,
		ItemID:          "cloak-of-protection",
		AttunementSlots: []character.AttunementSlot{}, // not attuned
	})

	require.NoError(t, err)
	assert.True(t, result.UpdatedItems[0].Equipped)
	assert.Contains(t, result.Warning, "requires attunement")
	assert.Contains(t, result.Warning, "/attune")
}

func TestEquip_AttunedNoWarning(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
	}

	result, err := Equip(EquipInput{
		Items:  items,
		ItemID: "cloak-of-protection",
		AttunementSlots: []character.AttunementSlot{
			{ItemID: "cloak-of-protection", Name: "Cloak of Protection"},
		},
	})

	require.NoError(t, err)
	assert.True(t, result.UpdatedItems[0].Equipped)
	assert.Empty(t, result.Warning)
}
