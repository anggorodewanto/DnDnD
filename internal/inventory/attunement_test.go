package inventory

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttune_Success(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
	}
	slots := []character.AttunementSlot{}

	result, err := Attune(AttuneInput{
		Items:    items,
		Slots:    slots,
		ItemID:   "cloak-of-protection",
		Classes:  nil,
	})

	require.NoError(t, err)
	require.Len(t, result.UpdatedSlots, 1)
	assert.Equal(t, "cloak-of-protection", result.UpdatedSlots[0].ItemID)
	assert.Equal(t, "Cloak of Protection", result.UpdatedSlots[0].Name)
	assert.Contains(t, result.Message, "Cloak of Protection")
}

func TestAttune_MaxThreeSlots(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "item-4", Name: "Fourth Item", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
	}
	slots := []character.AttunementSlot{
		{ItemID: "item-1", Name: "Item 1"},
		{ItemID: "item-2", Name: "Item 2"},
		{ItemID: "item-3", Name: "Item 3"},
	}

	_, err := Attune(AttuneInput{
		Items:  items,
		Slots:  slots,
		ItemID: "item-4",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "3 attuned items")
}

func TestAttune_ItemNotInInventory(t *testing.T) {
	_, err := Attune(AttuneInput{
		Items:  nil,
		Slots:  nil,
		ItemID: "cloak-of-protection",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestAttune_ItemDoesNotRequireAttunement(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}

	_, err := Attune(AttuneInput{
		Items:  items,
		Slots:  nil,
		ItemID: "longsword",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not require attunement")
}

func TestAttune_AlreadyAttuned(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
	}
	slots := []character.AttunementSlot{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection"},
	}

	_, err := Attune(AttuneInput{
		Items:  items,
		Slots:  slots,
		ItemID: "cloak-of-protection",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already attuned")
}

func TestAttune_ClassRestriction_Allowed(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "holy-avenger", Name: "Holy Avenger", Quantity: 1, Type: "weapon", IsMagic: true, RequiresAttunement: true},
	}

	result, err := Attune(AttuneInput{
		Items:                 items,
		Slots:                 nil,
		ItemID:                "holy-avenger",
		Classes:               []character.ClassEntry{{Class: "paladin", Level: 5}},
		AttunementRestriction: "paladin",
	})

	require.NoError(t, err)
	assert.Len(t, result.UpdatedSlots, 1)
}

func TestAttune_ClassRestriction_Denied(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "holy-avenger", Name: "Holy Avenger", Quantity: 1, Type: "weapon", IsMagic: true, RequiresAttunement: true},
	}

	_, err := Attune(AttuneInput{
		Items:                 items,
		Slots:                 nil,
		ItemID:                "holy-avenger",
		Classes:               []character.ClassEntry{{Class: "fighter", Level: 5}},
		AttunementRestriction: "paladin",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "restriction")
}

func TestUnattune_Success(t *testing.T) {
	slots := []character.AttunementSlot{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection"},
		{ItemID: "ring-of-protection", Name: "Ring of Protection"},
	}

	result, err := Unattune(UnattuneInput{
		Slots:  slots,
		ItemID: "cloak-of-protection",
	})

	require.NoError(t, err)
	assert.Len(t, result.UpdatedSlots, 1)
	assert.Equal(t, "ring-of-protection", result.UpdatedSlots[0].ItemID)
	assert.Equal(t, "Cloak of Protection", result.ItemName)
	assert.Contains(t, result.Message, "Unattuned")
}

func TestUnattune_NotAttuned(t *testing.T) {
	_, err := Unattune(UnattuneInput{
		Slots:  nil,
		ItemID: "cloak-of-protection",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not attuned")
}
