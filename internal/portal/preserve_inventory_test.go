package portal

import (
	"encoding/json"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func invMsg(t *testing.T, items []character.InventoryItem) pqtype.NullRawMessage {
	t.Helper()
	raw, err := json.Marshal(items)
	require.NoError(t, err)
	return pqtype.NullRawMessage{RawMessage: raw, Valid: true}
}

func parseInv(t *testing.T, msg pqtype.NullRawMessage) []character.InventoryItem {
	t.Helper()
	require.True(t, msg.Valid)
	var items []character.InventoryItem
	require.NoError(t, json.Unmarshal(msg.RawMessage, &items))
	return items
}

func TestPreserveInventory_AdditiveKeepsCustomItems(t *testing.T) {
	unidentified := false
	// Existing inventory carries rich quest/loot items the catalog cannot rebuild.
	existing := invMsg(t, []character.InventoryItem{
		{ItemID: "shortsword", Name: "Shortsword", Quantity: 1, Type: "weapon", Equipped: true, EquipSlot: "main_hand"},
		{ItemID: "throwing-awls", Name: "Throwing Awls", Quantity: 6, Type: "gear", Description: "A slim bandolier of six blackened throwing spikes."},
		{ItemID: "kept-name-scrap", Name: "THE SEAL", Quantity: 1, Type: "gear", IsMagic: true, Description: "A warded scrap holding a living true name."},
		{ItemID: "sealed-vial", Name: "Sealed Vial", Quantity: 1, Type: "gear", Identified: &unidentified, Description: "A wax-sealed, unlabeled vial."},
	})
	// The fresh build reconstructs each id from the equipment list — off-catalog
	// ids degrade to a bare {id-as-name, gear, qty 1} shell with no description,
	// and the equipped weapon is now a different id (hb_silent).
	fresh := invMsg(t, []character.InventoryItem{
		{ItemID: "shortsword", Name: "shortsword", Quantity: 1, Type: "gear"},
		{ItemID: "throwing-awls", Name: "throwing-awls", Quantity: 1, Type: "gear"},
		{ItemID: "kept-name-scrap", Name: "kept-name-scrap", Quantity: 1, Type: "gear"},
		{ItemID: "sealed-vial", Name: "sealed-vial", Quantity: 1, Type: "gear"},
		{ItemID: "hb_silent", Name: "hb_silent", Quantity: 1, Type: "gear", Equipped: true, EquipSlot: "main_hand"},
	})

	got := parseInv(t, preserveInventory(existing, fresh, true))
	byID := map[string]character.InventoryItem{}
	for _, it := range got {
		byID[it.ItemID] = it
	}

	// Rich fields survive.
	assert.Equal(t, "Throwing Awls", byID["throwing-awls"].Name)
	assert.Equal(t, 6, byID["throwing-awls"].Quantity, "stack quantity preserved, not collapsed to 1")
	assert.NotEmpty(t, byID["throwing-awls"].Description)
	assert.True(t, byID["kept-name-scrap"].IsMagic, "magic flag preserved")
	assert.Equal(t, "THE SEAL", byID["kept-name-scrap"].Name)
	require.NotNil(t, byID["sealed-vial"].Identified)
	assert.False(t, *byID["sealed-vial"].Identified, "unidentified flag preserved")

	// Equip changes from the fresh build still land.
	assert.False(t, byID["shortsword"].Equipped, "old main hand unequipped")
	assert.True(t, byID["hb_silent"].Equipped, "new main hand equipped")
	assert.Equal(t, "main_hand", byID["hb_silent"].EquipSlot)
}

func TestPreserveInventory_NonAdditiveRebuildsFresh(t *testing.T) {
	existing := invMsg(t, []character.InventoryItem{
		{ItemID: "quest-item", Name: "Quest Item", Quantity: 1, Type: "gear", Description: "flavor"},
	})
	fresh := invMsg(t, []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	})
	// A genuine respec (non-additive) discards the old inventory entirely.
	got := parseInv(t, preserveInventory(existing, fresh, false))
	require.Len(t, got, 1)
	assert.Equal(t, "longsword", got[0].ItemID)
}

func TestPreserveInventory_MissingSideFallsBack(t *testing.T) {
	fresh := invMsg(t, []character.InventoryItem{{ItemID: "dagger", Name: "Dagger", Quantity: 1, Type: "weapon"}})
	// No existing inventory → return the fresh build unchanged.
	got := preserveInventory(pqtype.NullRawMessage{}, fresh, true)
	assert.Equal(t, fresh, got)
}
