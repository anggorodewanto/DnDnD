package inventory

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentifyItem_ItemNotFound(t *testing.T) {
	_, err := IdentifyItem(IdentifyInput{
		Items:  nil,
		ItemID: "nonexistent",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestIdentifyItem_NotMagic(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}
	_, err := IdentifyItem(IdentifyInput{Items: items, ItemID: "longsword"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a magic item")
}

func TestIdentifyItem_AlreadyIdentified(t *testing.T) {
	identified := true
	items := []character.InventoryItem{
		{ItemID: "ring", Name: "Ring of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &identified},
	}
	_, err := IdentifyItem(IdentifyInput{Items: items, ItemID: "ring"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already identified")
}

func TestIdentifyItem_NilIdentifiedIsAlreadyIdentified(t *testing.T) {
	// nil Identified means "identified" per convention
	items := []character.InventoryItem{
		{ItemID: "ring", Name: "Ring of Protection", Quantity: 1, Type: "magic_item", IsMagic: true},
	}
	_, err := IdentifyItem(IdentifyInput{Items: items, ItemID: "ring"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already identified")
}

func TestSetItemIdentified_SetToFalse(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "ring", Name: "Ring of Protection", Quantity: 1, Type: "magic_item", IsMagic: true},
	}

	result, err := SetItemIdentified(SetIdentifiedInput{
		Items:      items,
		ItemID:     "ring",
		Identified: false,
	})

	require.NoError(t, err)
	require.NotNil(t, result.UpdatedItems[0].Identified)
	assert.False(t, *result.UpdatedItems[0].Identified)
}

func TestSetItemIdentified_SetToTrue(t *testing.T) {
	unidentified := false
	items := []character.InventoryItem{
		{ItemID: "ring", Name: "Ring of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &unidentified},
	}

	result, err := SetItemIdentified(SetIdentifiedInput{
		Items:      items,
		ItemID:     "ring",
		Identified: true,
	})

	require.NoError(t, err)
	require.NotNil(t, result.UpdatedItems[0].Identified)
	assert.True(t, *result.UpdatedItems[0].Identified)
}

func TestSetItemIdentified_NotMagic(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}
	_, err := SetItemIdentified(SetIdentifiedInput{Items: items, ItemID: "longsword", Identified: false})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a magic item")
}

func TestSetItemIdentified_ItemNotFound(t *testing.T) {
	_, err := SetItemIdentified(SetIdentifiedInput{Items: nil, ItemID: "x", Identified: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDetectMagicItems_ReturnsMagicItems(t *testing.T) {
	unidentified := false
	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
		{ItemID: "ring", Name: "Ring of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &unidentified},
		{ItemID: "potion", Name: "Healing Potion", Quantity: 1, Type: "consumable"},
		{ItemID: "cloak", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true},
	}

	result := DetectMagicItems(items)
	assert.Len(t, result, 2)
	assert.Equal(t, "ring", result[0].ItemID)
	assert.Equal(t, "cloak", result[1].ItemID)
}

func TestDetectMagicItems_NoMagicItems(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}
	result := DetectMagicItems(items)
	assert.Empty(t, result)
}

func TestDetectMagicItems_EmptyInventory(t *testing.T) {
	result := DetectMagicItems(nil)
	assert.Empty(t, result)
}

func TestStudyItemDuringRest_Success(t *testing.T) {
	unidentified := false
	items := []character.InventoryItem{
		{ItemID: "mystery-ring", Name: "Ring of Invisibility", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &unidentified},
	}

	result, err := StudyItemDuringRest(IdentifyInput{Items: items, ItemID: "mystery-ring"})
	require.NoError(t, err)
	require.NotNil(t, result.UpdatedItems[0].Identified)
	assert.True(t, *result.UpdatedItems[0].Identified)
	assert.Contains(t, result.Message, "studied")
}

func TestStudyItemDuringRest_NotMagic(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}
	_, err := StudyItemDuringRest(IdentifyInput{Items: items, ItemID: "longsword"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a magic item")
}

func TestStudyItemDuringRest_AlreadyIdentified(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "ring", Name: "Ring of Protection", Quantity: 1, Type: "magic_item", IsMagic: true},
	}
	_, err := StudyItemDuringRest(IdentifyInput{Items: items, ItemID: "ring"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already identified")
}

func TestStudyItemDuringRest_ItemNotFound(t *testing.T) {
	_, err := StudyItemDuringRest(IdentifyInput{Items: nil, ItemID: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCastIdentify_Success_WithSlot(t *testing.T) {
	unidentified := false
	items := []character.InventoryItem{
		{ItemID: "mystery-ring", Name: "Ring of Invisibility", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &unidentified},
	}

	result, err := CastIdentify(CastIdentifyInput{
		Items:      items,
		ItemID:     "mystery-ring",
		KnowsSpell: true,
		SpellSlots: map[int]int{1: 2},
		SlotLevel:  1,
		IsRitual:   false,
	})

	require.NoError(t, err)
	assert.True(t, *result.UpdatedItems[0].Identified)
	assert.Equal(t, 1, result.SlotsRemaining)
	assert.False(t, result.IsRitual)
	assert.Contains(t, result.Message, "Identify")
}

func TestCastIdentify_Success_Ritual(t *testing.T) {
	unidentified := false
	items := []character.InventoryItem{
		{ItemID: "mystery-ring", Name: "Ring of Invisibility", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &unidentified},
	}

	result, err := CastIdentify(CastIdentifyInput{
		Items:      items,
		ItemID:     "mystery-ring",
		KnowsSpell: true,
		SpellSlots: map[int]int{1: 0}, // no slots, but ritual doesn't need one
		IsRitual:   true,
	})

	require.NoError(t, err)
	assert.True(t, *result.UpdatedItems[0].Identified)
	assert.True(t, result.IsRitual)
	assert.Equal(t, 0, result.SlotsRemaining)
}

func TestCastIdentify_DoesNotKnowSpell(t *testing.T) {
	unidentified := false
	items := []character.InventoryItem{
		{ItemID: "ring", Name: "Ring", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &unidentified},
	}
	_, err := CastIdentify(CastIdentifyInput{
		Items:      items,
		ItemID:     "ring",
		KnowsSpell: false,
		SpellSlots: map[int]int{1: 2},
		SlotLevel:  1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not know")
}

func TestCastIdentify_NoSlotAvailable(t *testing.T) {
	unidentified := false
	items := []character.InventoryItem{
		{ItemID: "ring", Name: "Ring", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &unidentified},
	}
	_, err := CastIdentify(CastIdentifyInput{
		Items:      items,
		ItemID:     "ring",
		KnowsSpell: true,
		SpellSlots: map[int]int{1: 0},
		SlotLevel:  1,
		IsRitual:   false,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no spell slot")
}

func TestCastIdentify_ItemNotMagic(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}
	_, err := CastIdentify(CastIdentifyInput{
		Items:      items,
		ItemID:     "longsword",
		KnowsSpell: true,
		SpellSlots: map[int]int{1: 2},
		SlotLevel:  1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a magic item")
}

func TestCastIdentify_ItemNotFound(t *testing.T) {
	_, err := CastIdentify(CastIdentifyInput{
		Items:      nil,
		ItemID:     "nonexistent",
		KnowsSpell: true,
		SpellSlots: map[int]int{1: 2},
		SlotLevel:  1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCastIdentify_AlreadyIdentified(t *testing.T) {
	items := []character.InventoryItem{
		{ItemID: "ring", Name: "Ring of Protection", Quantity: 1, Type: "magic_item", IsMagic: true},
	}
	_, err := CastIdentify(CastIdentifyInput{
		Items:      items,
		ItemID:     "ring",
		KnowsSpell: true,
		SpellSlots: map[int]int{1: 2},
		SlotLevel:  1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already identified")
}

func TestShortRestStudy_Integration(t *testing.T) {
	// Full flow: unidentified item -> short rest study -> identified
	unidentified := false
	items := []character.InventoryItem{
		{ItemID: "mystery-ring", Name: "Ring of Invisibility", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &unidentified},
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}

	// First verify item shows as unidentified
	display := FormatInventory("Aria", 10, items, nil)
	assert.Contains(t, display, "Unidentified magic_item")
	assert.NotContains(t, display, "Ring of Invisibility")

	// Study during short rest
	result, err := StudyItemDuringRest(IdentifyInput{Items: items, ItemID: "mystery-ring"})
	require.NoError(t, err)

	// Now item should be identified
	assert.True(t, *result.UpdatedItems[0].Identified)

	// Display should show the real name
	display = FormatInventory("Aria", 10, result.UpdatedItems, nil)
	assert.Contains(t, display, "Ring of Invisibility")
	assert.NotContains(t, display, "Unidentified")
}

func TestIntegration_FullIdentificationFlow(t *testing.T) {
	// Full flow: DM assigns unidentified item -> detect magic -> cast identify -> item revealed
	unidentified := false
	items := []character.InventoryItem{
		{ItemID: "mystery-ring", Name: "Ring of Invisibility", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &unidentified},
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
		{ItemID: "cloak", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true},
	}

	// Step 1: Detect Magic reveals presence of magic items
	magicItems := DetectMagicItems(items)
	assert.Len(t, magicItems, 2)

	// Step 2: Unidentified item shows as "Unidentified"
	display := FormatInventory("Aria", 10, items, nil)
	assert.Contains(t, display, "Unidentified magic_item")
	assert.NotContains(t, display, "Ring of Invisibility")

	// Step 3: Cast Identify on the mystery ring
	castResult, err := CastIdentify(CastIdentifyInput{
		Items:      items,
		ItemID:     "mystery-ring",
		KnowsSpell: true,
		SpellSlots: map[int]int{1: 2},
		SlotLevel:  1,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, castResult.SlotsRemaining)

	// Step 4: Item is now identified and shows real name
	display = FormatInventory("Aria", 10, castResult.UpdatedItems, nil)
	assert.Contains(t, display, "Ring of Invisibility")
	assert.NotContains(t, display, "Unidentified")
}

func TestIntegration_DMIdentificationToggle(t *testing.T) {
	// DM can hide and reveal item properties
	items := []character.InventoryItem{
		{ItemID: "ring", Name: "Ring of Protection", Quantity: 1, Type: "magic_item", IsMagic: true},
	}

	// DM sets to unidentified
	hideResult, err := SetItemIdentified(SetIdentifiedInput{Items: items, ItemID: "ring", Identified: false})
	require.NoError(t, err)
	assert.False(t, *hideResult.UpdatedItems[0].Identified)

	// Shows as unidentified
	display := FormatInventory("Aria", 10, hideResult.UpdatedItems, nil)
	assert.Contains(t, display, "Unidentified magic_item")

	// DM reveals
	revealResult, err := SetItemIdentified(SetIdentifiedInput{Items: hideResult.UpdatedItems, ItemID: "ring", Identified: true})
	require.NoError(t, err)
	assert.True(t, *revealResult.UpdatedItems[0].Identified)

	// Shows real name now
	display = FormatInventory("Aria", 10, revealResult.UpdatedItems, nil)
	assert.Contains(t, display, "Ring of Protection")
}

func TestIntegration_CastIdentifyRitualNoSlotCost(t *testing.T) {
	unidentified := false
	items := []character.InventoryItem{
		{ItemID: "ring", Name: "Ring of Invisibility", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &unidentified},
	}

	// Cast as ritual with zero slots
	result, err := CastIdentify(CastIdentifyInput{
		Items:      items,
		ItemID:     "ring",
		KnowsSpell: true,
		SpellSlots: map[int]int{1: 0},
		IsRitual:   true,
	})
	require.NoError(t, err)
	assert.True(t, result.IsRitual)
	assert.True(t, *result.UpdatedItems[0].Identified)
	assert.Equal(t, 0, result.SlotsRemaining)
}

func TestIdentifyItem_Success(t *testing.T) {
	unidentified := false
	items := []character.InventoryItem{
		{ItemID: "mystery-ring", Name: "Ring of Invisibility", Quantity: 1, Type: "magic_item", IsMagic: true, Identified: &unidentified},
	}

	result, err := IdentifyItem(IdentifyInput{
		Items:  items,
		ItemID: "mystery-ring",
	})

	require.NoError(t, err)
	require.NotNil(t, result.UpdatedItems[0].Identified)
	assert.True(t, *result.UpdatedItems[0].Identified)
	assert.Contains(t, result.Message, "Ring of Invisibility")
	assert.Contains(t, result.Message, "identified")
}
