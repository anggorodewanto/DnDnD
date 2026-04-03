package inventory

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sequentialRoller returns results from a predefined sequence.
func sequentialRoller(results []int) func(int) int {
	idx := 0
	return func(max int) int {
		if idx >= len(results) {
			return 1
		}
		r := results[idx]
		idx++
		return r
	}
}

func TestDawnRecharge_RestoresCharges(t *testing.T) {
	roller := func(max int) int { return 4 } // always roll 4
	svc := NewService(roller)

	items := []character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, Charges: 2, MaxCharges: 7},
	}

	result, err := svc.DawnRecharge(DawnRechargeInput{
		Items:        items,
		RechargeInfo: map[string]RechargeInfo{
			"wand-of-fireballs": {Dice: "1d6+1", DestroyOnZero: true},
		},
	})

	require.NoError(t, err)
	// 1d6+1 with always-4 die = 4+1 = 5, so charges go from 2 to 7 (capped at max)
	assert.Equal(t, 7, result.UpdatedItems[0].Charges)
	assert.Len(t, result.Recharged, 1)
	assert.Equal(t, "wand-of-fireballs", result.Recharged[0].ItemID)
	assert.Equal(t, 5, result.Recharged[0].Restored)
	assert.False(t, result.Recharged[0].Destroyed)
}

func TestDawnRecharge_CapsAtMax(t *testing.T) {
	roller := func(max int) int { return 6 } // always roll 6
	svc := NewService(roller)

	items := []character.InventoryItem{
		{ItemID: "staff-of-healing", Name: "Staff of Healing", Quantity: 1, Type: "magic_item", IsMagic: true, Charges: 8, MaxCharges: 10},
	}

	result, err := svc.DawnRecharge(DawnRechargeInput{
		Items:        items,
		RechargeInfo: map[string]RechargeInfo{
			"staff-of-healing": {Dice: "1d6+1"},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, 10, result.UpdatedItems[0].Charges) // capped at max
}

func TestDawnRecharge_DestroyOnZero(t *testing.T) {
	// Item at 0 charges: first roll is d20 destroy check (returns 1 → destroyed)
	svc := NewService(sequentialRoller([]int{1}))

	items := []character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, Charges: 0, MaxCharges: 7},
	}

	result, err := svc.DawnRecharge(DawnRechargeInput{
		Items:        items,
		RechargeInfo: map[string]RechargeInfo{
			"wand-of-fireballs": {Dice: "1d6+1", DestroyOnZero: true},
		},
	})

	require.NoError(t, err)
	assert.Empty(t, result.UpdatedItems) // item destroyed and removed
	require.Len(t, result.Recharged, 1)
	assert.True(t, result.Recharged[0].Destroyed)
}

func TestDawnRecharge_ZeroChargesNotDestroyed(t *testing.T) {
	// Item at 0 charges: first roll is d20 destroy check (returns 10 → survives),
	// second roll is recharge dice 1d6 (returns 5 → 5+1=6 charges)
	svc := NewService(sequentialRoller([]int{10, 5}))

	items := []character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, Charges: 0, MaxCharges: 7},
	}

	result, err := svc.DawnRecharge(DawnRechargeInput{
		Items:        items,
		RechargeInfo: map[string]RechargeInfo{
			"wand-of-fireballs": {Dice: "1d6+1", DestroyOnZero: true},
		},
	})

	require.NoError(t, err)
	require.Len(t, result.UpdatedItems, 1)
	assert.Equal(t, 6, result.UpdatedItems[0].Charges) // recharged
	assert.False(t, result.Recharged[0].Destroyed)
}

func TestDawnRecharge_DestroyOnZero_ItemNotAtZero(t *testing.T) {
	// Item has charges > 0 with DestroyOnZero: destroy check should NOT trigger
	svc := NewService(func(max int) int { return 3 })

	items := []character.InventoryItem{
		{ItemID: "wand-of-fireballs", Name: "Wand of Fireballs", Quantity: 1, Type: "magic_item", IsMagic: true, Charges: 2, MaxCharges: 7},
	}

	result, err := svc.DawnRecharge(DawnRechargeInput{
		Items:        items,
		RechargeInfo: map[string]RechargeInfo{
			"wand-of-fireballs": {Dice: "1d6+1", DestroyOnZero: true},
		},
	})

	require.NoError(t, err)
	require.Len(t, result.UpdatedItems, 1)
	assert.Equal(t, 6, result.UpdatedItems[0].Charges) // 2 + (3+1) = 6
	assert.False(t, result.Recharged[0].Destroyed)
}

func TestDawnRecharge_NoRechargeInfo_Skipped(t *testing.T) {
	svc := NewService(nil)

	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}

	result, err := svc.DawnRecharge(DawnRechargeInput{
		Items:        items,
		RechargeInfo: nil,
	})

	require.NoError(t, err)
	assert.Len(t, result.UpdatedItems, 1)
	assert.Empty(t, result.Recharged)
}
