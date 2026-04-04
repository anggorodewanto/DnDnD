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

func TestDeriveCharacterSpeed_Default(t *testing.T) {
	// classHitDie is tested indirectly; test the exported DeriveSpeed.
	assert.Equal(t, 30, portal.DeriveSpeed("human"))
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
