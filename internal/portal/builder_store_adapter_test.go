package portal_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/portal"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureCharacterCreator captures CreateCharacterParams for inspection.
type captureCharacterCreator struct {
	capturedParams refdata.CreateCharacterParams
	returnChar     refdata.Character
	returnErr      error
}

func (c *captureCharacterCreator) CreateCharacter(_ context.Context, arg refdata.CreateCharacterParams) (refdata.Character, error) {
	c.capturedParams = arg
	if c.returnErr != nil {
		return refdata.Character{}, c.returnErr
	}
	c.returnChar.ID = uuid.New()
	return c.returnChar, nil
}

func (c *captureCharacterCreator) CreatePlayerCharacter(_ context.Context, arg refdata.CreatePlayerCharacterParams) (refdata.PlayerCharacter, error) {
	return refdata.PlayerCharacter{ID: uuid.New()}, nil
}

func TestBuilderStoreAdapter_Implements_BuilderStore(t *testing.T) {
	// Compile-time check that BuilderStoreAdapter implements BuilderStore.
	var _ portal.BuilderStore = (*portal.BuilderStoreAdapter)(nil)
	assert.True(t, true)
}

func TestNewBuilderStoreAdapter(t *testing.T) {
	adapter := portal.NewBuilderStoreAdapter(nil, nil)
	assert.NotNil(t, adapter)
}

func TestBuilderStoreAdapter_RedeemToken_NilTokenSvc(t *testing.T) {
	adapter := portal.NewBuilderStoreAdapter(nil, nil)
	err := adapter.RedeemToken(context.Background(), "some-token")
	assert.NoError(t, err)
}


func TestBuilderStoreAdapter_EquipmentToInventory(t *testing.T) {
	// Test that equipment strings are converted to inventory items
	items := portal.EquipmentToInventory([]string{"longsword", "chain-mail", "shield"})
	assert.Len(t, items, 3)
	assert.Equal(t, "longsword", items[0].ItemID)
	assert.Equal(t, "longsword", items[0].Name)
	assert.Equal(t, 1, items[0].Quantity)
	assert.Equal(t, "weapon", items[0].Type)
	assert.Equal(t, "chain-mail", items[1].ItemID)
	assert.Equal(t, "armor", items[1].Type)
	assert.Equal(t, "shield", items[2].ItemID)
	assert.Equal(t, "armor", items[2].Type)
}

func TestBuilderStoreAdapter_EquipmentToInventory_Empty(t *testing.T) {
	items := portal.EquipmentToInventory(nil)
	assert.Empty(t, items)
}

func TestBuilderStoreAdapter_EquipmentToInventoryWithEquipped(t *testing.T) {
	items := portal.EquipmentToInventoryWithEquipped(
		[]string{"longsword", "chain-mail", "shield"},
		"longsword", "chain-mail",
	)
	require.Len(t, items, 3)

	// longsword: equipped as main_hand
	assert.Equal(t, "longsword", items[0].ItemID)
	assert.True(t, items[0].Equipped)
	assert.Equal(t, "main_hand", items[0].EquipSlot)

	// chain-mail: equipped as armor
	assert.Equal(t, "chain-mail", items[1].ItemID)
	assert.True(t, items[1].Equipped)
	assert.Equal(t, "armor", items[1].EquipSlot)

	// shield: auto-equipped as off_hand
	assert.Equal(t, "shield", items[2].ItemID)
	assert.True(t, items[2].Equipped)
	assert.Equal(t, "off_hand", items[2].EquipSlot)
}

func TestBuilderStoreAdapter_EquipmentToInventoryWithEquipped_NoEquipped(t *testing.T) {
	items := portal.EquipmentToInventoryWithEquipped(
		[]string{"longsword", "potion-of-healing"}, "", "",
	)
	require.Len(t, items, 2)
	assert.False(t, items[0].Equipped)
	assert.False(t, items[1].Equipped)
}

func TestBuilderStoreAdapter_EquipmentToInventory_UnknownItems(t *testing.T) {
	items := portal.EquipmentToInventory([]string{"magic-wand", "potion-of-healing"})
	assert.Len(t, items, 2)
	assert.Equal(t, "gear", items[0].Type)
}

func TestBuilderStoreAdapter_CreateCharacterRecord_PersistsSpells(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Gandalf",
		Race:          "Elf",
		Class:         "Wizard",
		AbilityScores: character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10},
		HPMax:         8,
		AC:            12,
		SpeedFt:       30,
		ProfBonus:     2,
		Spells:        []string{"fire-bolt", "mage-hand", "magic-missile", "shield"},
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	// character_data should contain spells
	require.True(t, creator.capturedParams.CharacterData.Valid, "CharacterData should be valid")

	var charData map[string]json.RawMessage
	err = json.Unmarshal(creator.capturedParams.CharacterData.RawMessage, &charData)
	require.NoError(t, err)

	spellsRaw, ok := charData["spells"]
	require.True(t, ok, "character_data should have 'spells' key")

	var spells []string
	err = json.Unmarshal(spellsRaw, &spells)
	require.NoError(t, err)
	assert.Equal(t, []string{"fire-bolt", "mage-hand", "magic-missile", "shield"}, spells)
}

func TestBuilderStoreAdapter_CreateCharacterRecord_NoSpells(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Fighter",
		Race:          "Human",
		Class:         "Fighter",
		AbilityScores: character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 10, CHA: 10},
		HPMax:         12,
		AC:            16,
		SpeedFt:       30,
		ProfBonus:     2,
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	// character_data should not be set when there are no spells
	assert.False(t, creator.capturedParams.CharacterData.Valid, "CharacterData should not be set without spells")
}

func TestBuilderStoreAdapter_CreateCharacterRecord_PersistsFeatures(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	features := []character.Feature{
		{Name: "Rage", Source: "Barbarian", Level: 1, Description: "Enter rage"},
		{Name: "Unarmored Defense", Source: "Barbarian", Level: 1, Description: "AC formula"},
	}

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Grog",
		Race:          "Half-Orc",
		Class:         "Barbarian",
		AbilityScores: character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 8, WIS: 10, CHA: 10},
		HPMax:         14,
		AC:            14,
		SpeedFt:       30,
		ProfBonus:     2,
		Features:      features,
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	// Features JSONB should be set
	require.True(t, creator.capturedParams.Features.Valid, "Features should be valid when provided")

	var storedFeatures []character.Feature
	err = json.Unmarshal(creator.capturedParams.Features.RawMessage, &storedFeatures)
	require.NoError(t, err)
	assert.Len(t, storedFeatures, 2)
	assert.Equal(t, "Rage", storedFeatures[0].Name)
	assert.Equal(t, "Unarmored Defense", storedFeatures[1].Name)
}

func TestBuilderStoreAdapter_CreateCharacterRecord_PersistsEquipped(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:     uuid.New().String(),
		Name:           "Knight",
		Race:           "Human",
		Class:          "Fighter",
		AbilityScores:  character.AbilityScores{STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 10, CHA: 10},
		HPMax:          12,
		AC:             18,
		SpeedFt:        30,
		ProfBonus:      2,
		Equipment:      []string{"longsword", "chain-mail", "shield"},
		EquippedWeapon: "longsword",
		WornArmor:      "chain-mail",
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	// EquippedMainHand should be set
	assert.True(t, creator.capturedParams.EquippedMainHand.Valid)
	assert.Equal(t, "longsword", creator.capturedParams.EquippedMainHand.String)

	// EquippedArmor should be set
	assert.True(t, creator.capturedParams.EquippedArmor.Valid)
	assert.Equal(t, "chain-mail", creator.capturedParams.EquippedArmor.String)

	// Inventory should have equipped items
	require.True(t, creator.capturedParams.Inventory.Valid)
	var items []character.InventoryItem
	err = json.Unmarshal(creator.capturedParams.Inventory.RawMessage, &items)
	require.NoError(t, err)
	assert.Len(t, items, 3)

	// longsword should be equipped
	assert.True(t, items[0].Equipped)
	assert.Equal(t, "main_hand", items[0].EquipSlot)
}

func TestBuilderStoreAdapter_CreateCharacterRecord_NoFeatures(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Fighter",
		Race:          "Human",
		Class:         "Fighter",
		AbilityScores: character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 10, CHA: 10},
		HPMax:         12,
		AC:            16,
		SpeedFt:       30,
		ProfBonus:     2,
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	assert.False(t, creator.capturedParams.Features.Valid, "Features should not be set when empty")
}

func TestDeriveCharacterSpeed_Default(t *testing.T) {
	// classHitDie is tested indirectly; test the exported DeriveSpeed.
	assert.Equal(t, 30, portal.DeriveSpeed("human"))
}

func TestDeriveSpeed_RaceLookup(t *testing.T) {
	assert.Equal(t, 25, portal.DeriveSpeed("dwarf"))
	assert.Equal(t, 25, portal.DeriveSpeed("halfling"))
	assert.Equal(t, 25, portal.DeriveSpeed("gnome"))
	assert.Equal(t, 30, portal.DeriveSpeed("elf"))
	assert.Equal(t, 30, portal.DeriveSpeed("human"))
	assert.Equal(t, 30, portal.DeriveSpeed("unknown-race"))
}

func TestClassHitDie(t *testing.T) {
	tests := []struct {
		class  string
		hitDie string
	}{
		{"barbarian", "d12"},
		{"fighter", "d10"},
		{"paladin", "d10"},
		{"ranger", "d10"},
		{"sorcerer", "d6"},
		{"wizard", "d6"},
		{"rogue", "d8"},
		{"cleric", "d8"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.hitDie, portal.ClassHitDie(tt.class), "class: %s", tt.class)
	}
}

func TestDeriveHP(t *testing.T) {
	scores := character.AbilityScores{STR: 10, DEX: 10, CON: 14, INT: 10, WIS: 10, CHA: 10}
	// Fighter (d10) at level 1 with CON 14 (+2): 10 + 2 = 12
	hp := portal.DeriveHP("fighter", scores)
	assert.Equal(t, 12, hp)
}

func TestDeriveAC(t *testing.T) {
	scores := character.AbilityScores{STR: 10, DEX: 14, INT: 10, WIS: 10, CHA: 10, CON: 10}
	// No armor: 10 + DEX mod (2) = 12
	ac := portal.DeriveAC(scores)
	assert.Equal(t, 12, ac)
}

// TestBuilderStoreAdapter_CreateCharacterRecord_Multiclass verifies the
// builder writes the supplied multiclass entries into the `classes`
// JSONB column and sums level + hit dice across them.
func TestBuilderStoreAdapter_CreateCharacterRecord_Multiclass(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Multi",
		Race:          "human",
		Class:         "fighter",
		AbilityScores: character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 10, CHA: 10},
		HPMax:         12,
		AC:            16,
		SpeedFt:       30,
		ProfBonus:     2,
		Classes: []character.ClassEntry{
			{Class: "fighter", Subclass: "champion", Level: 5},
			{Class: "wizard", Subclass: "evocation", Level: 3},
		},
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	var classes []character.ClassEntry
	require.NoError(t, json.Unmarshal(creator.capturedParams.Classes, &classes))
	require.Len(t, classes, 2)
	assert.Equal(t, "fighter", classes[0].Class)
	assert.Equal(t, "champion", classes[0].Subclass)
	assert.Equal(t, 5, classes[0].Level)
	assert.Equal(t, "wizard", classes[1].Class)
	assert.Equal(t, "evocation", classes[1].Subclass)
	assert.Equal(t, 3, classes[1].Level)

	// Total level should reflect the sum
	assert.Equal(t, int32(8), creator.capturedParams.Level)

	// Hit dice map should include both classes
	var hitDice map[string]int
	require.NoError(t, json.Unmarshal(creator.capturedParams.HitDiceRemaining, &hitDice))
	assert.Equal(t, 5, hitDice["fighter"])
	assert.Equal(t, 3, hitDice["wizard"])
}

// TestBuilderStoreAdapter_CreateCharacterRecord_FallsBackToSingleClass
// verifies the legacy single-class submission path still works.
func TestBuilderStoreAdapter_CreateCharacterRecord_FallsBackToSingleClass(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Solo",
		Race:          "human",
		Class:         "rogue",
		Subclass:      "thief",
		AbilityScores: character.AbilityScores{STR: 10, DEX: 16, CON: 12, INT: 10, WIS: 10, CHA: 10},
		HPMax:         9,
		AC:            13,
		SpeedFt:       30,
		ProfBonus:     2,
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	var classes []character.ClassEntry
	require.NoError(t, json.Unmarshal(creator.capturedParams.Classes, &classes))
	require.Len(t, classes, 1)
	assert.Equal(t, "rogue", classes[0].Class)
	assert.Equal(t, "thief", classes[0].Subclass)
	assert.Equal(t, 1, classes[0].Level)
	assert.Equal(t, int32(1), creator.capturedParams.Level)
}

// TestBuilderStoreAdapter_CreateCharacterRecord_PersistsSubrace verifies
// the subrace field is stashed in character_data.
func TestBuilderStoreAdapter_CreateCharacterRecord_PersistsSubrace(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Legolas",
		Race:          "elf",
		Subrace:       "high-elf",
		Class:         "ranger",
		AbilityScores: character.AbilityScores{STR: 12, DEX: 16, CON: 12, INT: 10, WIS: 14, CHA: 10},
		HPMax:         11,
		AC:            13,
		SpeedFt:       30,
		ProfBonus:     2,
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	require.True(t, creator.capturedParams.CharacterData.Valid)
	var charData map[string]any
	require.NoError(t, json.Unmarshal(creator.capturedParams.CharacterData.RawMessage, &charData))
	assert.Equal(t, "high-elf", charData["subrace"])
}

// TestBuilderStoreAdapter_CreateCharacterRecord_PersistsBackground
// verifies background gets stashed in character_data for the player card.
func TestBuilderStoreAdapter_CreateCharacterRecord_PersistsBackground(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Sage",
		Race:          "human",
		Class:         "wizard",
		Background:    "sage",
		AbilityScores: character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 16, WIS: 12, CHA: 10},
		HPMax:         8,
		AC:            12,
		SpeedFt:       30,
		ProfBonus:     2,
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	require.True(t, creator.capturedParams.CharacterData.Valid)
	var charData map[string]any
	require.NoError(t, json.Unmarshal(creator.capturedParams.CharacterData.RawMessage, &charData))
	assert.Equal(t, "sage", charData["background"])
}

func TestBuilderStoreAdapter_CreateCharacterRecord_InitializesFeatureUses_Barbarian(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Grog",
		Race:          "Half-Orc",
		Class:         "Barbarian",
		AbilityScores: character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 8, WIS: 10, CHA: 10},
		HPMax:         14,
		AC:            14,
		SpeedFt:       30,
		ProfBonus:     2,
		Classes:       []character.ClassEntry{{Class: "Barbarian", Level: 3}},
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	require.True(t, creator.capturedParams.FeatureUses.Valid, "FeatureUses should be set")

	var featureUses map[string]character.FeatureUse
	require.NoError(t, json.Unmarshal(creator.capturedParams.FeatureUses.RawMessage, &featureUses))

	rage, ok := featureUses["rage"]
	require.True(t, ok, "feature_uses should contain 'rage'")
	assert.Equal(t, 3, rage.Current)
	assert.Equal(t, 3, rage.Max)
	assert.Equal(t, "long", rage.Recharge)
}

func TestBuilderStoreAdapter_CreateCharacterRecord_InitializesFeatureUses_Fighter(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Knight",
		Race:          "Human",
		Class:         "Fighter",
		AbilityScores: character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 10, CHA: 10},
		HPMax:         12,
		AC:            16,
		SpeedFt:       30,
		ProfBonus:     2,
		Classes:       []character.ClassEntry{{Class: "Fighter", Level: 2}},
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	require.True(t, creator.capturedParams.FeatureUses.Valid)

	var featureUses map[string]character.FeatureUse
	require.NoError(t, json.Unmarshal(creator.capturedParams.FeatureUses.RawMessage, &featureUses))

	surge := featureUses["action-surge"]
	assert.Equal(t, 1, surge.Current)
	assert.Equal(t, 1, surge.Max)
	assert.Equal(t, "short", surge.Recharge)

	sw := featureUses["second-wind"]
	assert.Equal(t, 1, sw.Current)
	assert.Equal(t, 1, sw.Max)
	assert.Equal(t, "short", sw.Recharge)
}
