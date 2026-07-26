package discord

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
)

// --- The Vex equip bug: two items claiming one slot ---
//
// mirrorInventoryFlag mirrors combat.Equip's column write into the inventory
// JSON. It flipped the newly equipped item on but never turned OFF whatever was
// already equipped in that slot, so a character could end up with two items
// both {equipped: true, equip_slot: "main_hand"}. Combat resolves the wielded
// weapon from the scalar equipped_main_hand column, so the sheet showed the new
// weapon while the engine kept swinging the old one — and a weapon mastery
// keyed to the new weapon's id (Vex on the Silent Blade) never fired.
//
// The legacy inventory-only path (inventory.Equip) always cleared the slot;
// only the SR-004 combat-routed path — the one that actually runs in
// production — did not.

func mainHandFixture() []character.InventoryItem {
	return []character.InventoryItem{
		{ItemID: "shortsword", Name: "Shortsword", Quantity: 1, Type: "weapon", Equipped: true, EquipSlot: "main_hand"},
		{ItemID: "hb_silent", Name: "Silent Blade", Quantity: 1, Type: "weapon"},
		{ItemID: "leather", Name: "Leather Armor", Quantity: 1, Type: "armor", Equipped: true, EquipSlot: "armor"},
	}
}

func byItemID(items []character.InventoryItem) map[string]character.InventoryItem {
	out := make(map[string]character.InventoryItem, len(items))
	for _, it := range items {
		out[it.ItemID] = it
	}
	return out
}

func TestMirrorInventoryFlag_ClearsPreviousOccupantOfSlot(t *testing.T) {
	updated, ok := mirrorInventoryFlag(mainHandFixture(), "hb_silent", false, false)

	require.True(t, ok)
	got := byItemID(updated)
	assert.True(t, got["hb_silent"].Equipped, "the newly equipped weapon is on")
	assert.Equal(t, "main_hand", got["hb_silent"].EquipSlot)
	assert.False(t, got["shortsword"].Equipped, "the weapon it replaced must be unequipped")
	assert.Empty(t, got["shortsword"].EquipSlot)
	assert.True(t, got["leather"].Equipped, "an unrelated slot is untouched")
	assert.Equal(t, "armor", got["leather"].EquipSlot)
}

func TestMirrorInventoryFlag_OffHandDoesNotDisturbMainHand(t *testing.T) {
	updated, ok := mirrorInventoryFlag(mainHandFixture(), "hb_silent", true, false)

	require.True(t, ok)
	got := byItemID(updated)
	assert.Equal(t, "off_hand", got["hb_silent"].EquipSlot)
	assert.True(t, got["shortsword"].Equipped, "equipping the off-hand leaves the main hand alone")
	assert.Equal(t, "main_hand", got["shortsword"].EquipSlot)
}

func TestMirrorInventoryFlag_DoesNotMutateInput(t *testing.T) {
	items := mainHandFixture()

	_, ok := mirrorInventoryFlag(items, "hb_silent", false, false)

	require.True(t, ok)
	assert.True(t, items[0].Equipped, "the caller's slice must not be modified in place")
	assert.Equal(t, "main_hand", items[0].EquipSlot)
}

// --- Equipping by display name ---
//
// /equip takes an item id, and looted/homebrew weapons carry opaque ids like
// "hb_6d6a20d35c09". A player holding the "Silent Blade" has no way to know
// that string, so the one command that repairs a wrong-weapon slot was
// unusable exactly when it was needed. Resolve what the player typed against
// their own inventory (id, display name, or slug) before hitting refdata.

func TestResolveEquipItemID_MatchesDisplayName(t *testing.T) {
	items := mainHandFixture()

	assert.Equal(t, "hb_silent", resolveEquipItemID(items, "Silent Blade"))
	assert.Equal(t, "hb_silent", resolveEquipItemID(items, "silent blade"))
	assert.Equal(t, "hb_silent", resolveEquipItemID(items, "silent-blade"))
}

func TestResolveEquipItemID_PassesThroughIDsAndUnknowns(t *testing.T) {
	items := mainHandFixture()

	assert.Equal(t, "hb_silent", resolveEquipItemID(items, "hb_silent"), "an exact id still resolves")
	assert.Equal(t, "none", resolveEquipItemID(items, "none"), "the unequip sentinel is never rewritten")
	assert.Equal(t, "chain-mail", resolveEquipItemID(items, "chain-mail"),
		"an id absent from inventory passes through so refdata can answer")
}
