package inventory

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUseCharges_Success(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, Charges: 7, MaxCharges: 7},
	}
	attunement := []character.AttunementSlot{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs"},
	}

	result, err := UseCharges(UseChargesInput{
		Items:      items,
		Attunement: attunement,
		ItemID:     "wand-of-fireballs",
		Amount:     3,
	})

	require.NoError(t, err)
	assert.Equal(t, 4, result.UpdatedItems[0].Charges)
	assert.Equal(t, "Wand of Fireballs", result.ItemName)
	assert.Contains(t, result.Message, "3 charges")
}

func TestUseCharges_InsufficientCharges(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, Charges: 2, MaxCharges: 7},
	}
	attunement := []character.AttunementSlot{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs"},
	}

	_, err := UseCharges(UseChargesInput{
		Items:      items,
		Attunement: attunement,
		ItemID:     "wand-of-fireballs",
		Amount:     3,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient charges")
}

func TestUseCharges_ItemNotFound(t *testing.T) {
	_, err := UseCharges(UseChargesInput{
		Items:  nil,
		ItemID: "wand-of-fireballs",
		Amount: 1,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUseCharges_NotMagic(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}

	_, err := UseCharges(UseChargesInput{
		Items:  items,
		ItemID: "longsword",
		Amount: 1,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a magic item")
}

func TestUseCharges_RequiresAttunement_NotAttuned(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true, Charges: 7, MaxCharges: 7},
	}

	_, err := UseCharges(UseChargesInput{
		Items:      items,
		Attunement: nil,
		ItemID:     "wand-of-fireballs",
		Amount:     1,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "attunement")
}

func TestUseCharges_NoChargesOnItem(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true},
	}

	_, err := UseCharges(UseChargesInput{
		Items:  items,
		ItemID: "cloak-of-protection",
		Amount: 1,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no charges")
}

func TestUseCharges_DestroyOnZero_Destroyed(t *testing.T) {
	// When charges hit 0 and DestroyOnZero is true, roll d20. On a 1, item is destroyed.
	svc := NewService(func(max int) int { return 1 }) // always roll 1 → destroyed

	items := []character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, Charges: 1, MaxCharges: 7},
	}
	attunement := []character.AttunementSlot{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs"},
	}

	result, err := svc.UseCharges(UseChargesInput{
		Items:         items,
		Attunement:    attunement,
		ItemID:        "wand-of-fireballs",
		Amount:        1,
		DestroyOnZero: true,
	})

	require.NoError(t, err)
	assert.True(t, result.Destroyed)
	assert.Equal(t, 0, result.ChargesLeft)
}

func TestUseCharges_DestroyOnZero_Survives(t *testing.T) {
	// When charges hit 0 and DestroyOnZero is true, roll d20. On != 1, item survives.
	svc := NewService(func(max int) int { return 10 }) // roll 10 → survives

	items := []character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, Charges: 1, MaxCharges: 7},
	}
	attunement := []character.AttunementSlot{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs"},
	}

	result, err := svc.UseCharges(UseChargesInput{
		Items:         items,
		Attunement:    attunement,
		ItemID:        "wand-of-fireballs",
		Amount:        1,
		DestroyOnZero: true,
	})

	require.NoError(t, err)
	assert.False(t, result.Destroyed)
	assert.Equal(t, 0, result.ChargesLeft)
}

func TestUseCharges_DestroyOnZero_NotAtZero(t *testing.T) {
	// When charges don't hit 0, no destroy roll even with DestroyOnZero=true.
	svc := NewService(func(max int) int { return 1 }) // would destroy if checked

	items := []character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, Charges: 3, MaxCharges: 7},
	}
	attunement := []character.AttunementSlot{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs"},
	}

	result, err := svc.UseCharges(UseChargesInput{
		Items:         items,
		Attunement:    attunement,
		ItemID:        "wand-of-fireballs",
		Amount:        1,
		DestroyOnZero: true,
	})

	require.NoError(t, err)
	assert.False(t, result.Destroyed)
	assert.Equal(t, 2, result.ChargesLeft)
}

func TestUseCharges_VariableChargeAmount_ForSpellCasting(t *testing.T) {
	// Finding 8: spell-casting items like Wand of Fireballs can spend
	// variable charges (1 charge = 3rd level, 2 = 4th, etc.)
	items := []character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, Charges: 7, MaxCharges: 7},
	}
	attunement := []character.AttunementSlot{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs"},
	}

	// Spend 3 charges (upcast to 5th level)
	result, err := UseCharges(UseChargesInput{
		Items:      items,
		Attunement: attunement,
		ItemID:     "wand-of-fireballs",
		Amount:     3,
	})

	require.NoError(t, err)
	assert.Equal(t, 4, result.UpdatedItems[0].Charges)
	assert.Equal(t, 3, result.ChargesUsed)
	assert.Equal(t, 4, result.ChargesLeft)
	assert.Contains(t, result.Message, "3 charges")
}
