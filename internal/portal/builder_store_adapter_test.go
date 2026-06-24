package portal_test

import (
	"context"
	"database/sql"
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

	// getByUserResult / getByUserErr drive GetPlayerCharacterByDiscordUser.
	getByUserResult refdata.PlayerCharacter
	getByUserErr    error
	// capturedRelink records the last RelinkPlayerCharacter call.
	capturedRelink refdata.RelinkPlayerCharacterParams
	relinkResult   refdata.PlayerCharacter
	relinkErr      error

	// draft persistence capture (T11 / Finding 4·b).
	capturedUpsertDraft refdata.UpsertCharacterDraftParams
	upsertDraftErr      error
	getDraftResult      json.RawMessage
	getDraftErr         error
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

func (c *captureCharacterCreator) GetPlayerCharacterByDiscordUser(_ context.Context, _ refdata.GetPlayerCharacterByDiscordUserParams) (refdata.PlayerCharacter, error) {
	return c.getByUserResult, c.getByUserErr
}

func (c *captureCharacterCreator) RelinkPlayerCharacter(_ context.Context, arg refdata.RelinkPlayerCharacterParams) (refdata.PlayerCharacter, error) {
	c.capturedRelink = arg
	return c.relinkResult, c.relinkErr
}

func (c *captureCharacterCreator) UpsertCharacterDraft(_ context.Context, arg refdata.UpsertCharacterDraftParams) error {
	c.capturedUpsertDraft = arg
	return c.upsertDraftErr
}

func (c *captureCharacterCreator) GetCharacterDraft(_ context.Context, _ refdata.GetCharacterDraftParams) (json.RawMessage, error) {
	return c.getDraftResult, c.getDraftErr
}

func TestBuilderStoreAdapter_ActivePlayerCharacter_Found(t *testing.T) {
	pcID := uuid.New()
	creator := &captureCharacterCreator{
		getByUserResult: refdata.PlayerCharacter{ID: pcID, Status: "pending"},
	}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	got, err := adapter.ActivePlayerCharacter(context.Background(), uuid.New().String(), "user-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, pcID.String(), got.ID)
	assert.Equal(t, "pending", got.Status)
}

func TestBuilderStoreAdapter_ActivePlayerCharacter_NoRow(t *testing.T) {
	creator := &captureCharacterCreator{getByUserErr: sql.ErrNoRows}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	got, err := adapter.ActivePlayerCharacter(context.Background(), uuid.New().String(), "user-1")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestBuilderStoreAdapter_ActivePlayerCharacter_BadCampaignID(t *testing.T) {
	adapter := portal.NewBuilderStoreAdapter(&captureCharacterCreator{}, nil)

	_, err := adapter.ActivePlayerCharacter(context.Background(), "not-a-uuid", "user-1")
	require.Error(t, err)
}

func TestBuilderStoreAdapter_RelinkPlayerCharacterRecord(t *testing.T) {
	newPCID := uuid.New()
	creator := &captureCharacterCreator{relinkResult: refdata.PlayerCharacter{ID: newPCID}}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)
	pcID := uuid.New()
	charID := uuid.New()

	got, err := adapter.RelinkPlayerCharacterRecord(context.Background(), pcID.String(), charID.String(), "create")
	require.NoError(t, err)
	assert.Equal(t, newPCID.String(), got)
	assert.Equal(t, pcID, creator.capturedRelink.ID)
	assert.Equal(t, charID, creator.capturedRelink.CharacterID)
	assert.Equal(t, "create", creator.capturedRelink.CreatedVia)
}

func TestBuilderStoreAdapter_RelinkPlayerCharacterRecord_BadIDs(t *testing.T) {
	adapter := portal.NewBuilderStoreAdapter(&captureCharacterCreator{}, nil)

	_, err := adapter.RelinkPlayerCharacterRecord(context.Background(), "bad-pc-id", uuid.New().String(), "create")
	require.Error(t, err)

	_, err = adapter.RelinkPlayerCharacterRecord(context.Background(), uuid.New().String(), "bad-char-id", "create")
	require.Error(t, err)
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

func TestBuilderStoreAdapter_EquipmentToInventory_SkipsPlaceholders(t *testing.T) {
	items := portal.EquipmentToInventory([]string{"longsword", "any-martial", "shield", "any-simple-melee"})
	assert.Len(t, items, 2)
	assert.Equal(t, "longsword", items[0].ItemID)
	assert.Equal(t, "shield", items[1].ItemID)
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

// A submission with no languages (the portal builder does not yet collect
// concrete languages) must still persist a non-nil array: the characters.
// languages column is TEXT[] NOT NULL, and pq.Array of a nil slice writes
// SQL NULL, which 500s the create. Guarantee a non-nil (possibly empty) slice.
func TestBuilderStoreAdapter_CreateCharacterRecord_NilLanguagesPersistsEmptyArray(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Mute",
		Race:          "Human",
		Class:         "Warlock",
		AbilityScores: character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 10, WIS: 13, CHA: 16},
		HPMax:         9,
		AC:            12,
		SpeedFt:       30,
		ProfBonus:     2,
		Languages:     nil, // builder sent none
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	require.NotNil(t, creator.capturedParams.Languages, "Languages must be non-nil to satisfy NOT NULL column")
	assert.Empty(t, creator.capturedParams.Languages, "no languages submitted → empty array")
}

// Submitted languages pass through unchanged.
func TestBuilderStoreAdapter_CreateCharacterRecord_PassesThroughLanguages(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Polyglot",
		Race:          "Elf",
		Class:         "Wizard",
		AbilityScores: character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10},
		HPMax:         8,
		AC:            12,
		SpeedFt:       30,
		ProfBonus:     2,
		Languages:     []string{"Common", "Elvish", "Draconic"},
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	assert.Equal(t, []string{"Common", "Elvish", "Draconic"}, creator.capturedParams.Languages)
}

func TestBuilderStoreAdapter_CreateCharacterRecord_RejectsBadCampaignID(t *testing.T) {
	for _, campaignID := range []string{"", "not-a-uuid"} {
		creator := &captureCharacterCreator{}
		adapter := portal.NewBuilderStoreAdapter(creator, nil)

		params := portal.CreateCharacterParams{
			CampaignID:    campaignID,
			Name:          "Gandalf",
			Race:          "Elf",
			Class:         "Wizard",
			AbilityScores: character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10},
			HPMax:         8,
		}

		_, err := adapter.CreateCharacterRecord(context.Background(), params)
		require.Error(t, err, "campaign_id %q should be rejected, not silently replaced", campaignID)
		assert.Empty(t, creator.capturedParams.Name, "should not attempt insert for campaign_id %q", campaignID)
	}
}

func TestBuilderStoreAdapter_CreateCharacterRecord_PersistsWeaponMasteries(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:      uuid.New().String(),
		Name:            "Aragorn",
		Race:            "Human",
		Class:           "Fighter",
		AbilityScores:   character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 13},
		HPMax:           12,
		AC:              16,
		SpeedFt:         30,
		ProfBonus:       2,
		WeaponMasteries: []string{"longsword", "shortbow", "greatsword"},
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	// character_data should contain weapon_masteries
	require.True(t, creator.capturedParams.CharacterData.Valid, "CharacterData should be valid")

	var charData map[string]json.RawMessage
	err = json.Unmarshal(creator.capturedParams.CharacterData.RawMessage, &charData)
	require.NoError(t, err)

	masteriesRaw, ok := charData["weapon_masteries"]
	require.True(t, ok, "character_data should have 'weapon_masteries' key")

	var masteries []string
	err = json.Unmarshal(masteriesRaw, &masteries)
	require.NoError(t, err)
	assert.Equal(t, []string{"longsword", "shortbow", "greatsword"}, masteries)
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

// TestBuilderStoreAdapter_CreateCharacterRecord_PersistsPactMagicSlots_Warlock
// verifies a created single-class warlock persists pact_magic_slots equal to
// PactMagicSlotsForLevel(totalWarlockLevel). Without this the stored character
// has no pact slots and cannot cast leveled spells in play.
func TestBuilderStoreAdapter_CreateCharacterRecord_PersistsPactMagicSlots_Warlock(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Mordenkainen",
		Race:          "Human",
		Class:         "Warlock",
		AbilityScores: character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 10, WIS: 12, CHA: 16},
		HPMax:         24,
		AC:            12,
		SpeedFt:       30,
		ProfBonus:     2,
		Classes:       []character.ClassEntry{{Class: "Warlock", Level: 3}},
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	require.True(t, creator.capturedParams.PactMagicSlots.Valid, "PactMagicSlots should be set for a warlock")

	var slots character.PactMagicSlots
	require.NoError(t, json.Unmarshal(creator.capturedParams.PactMagicSlots.RawMessage, &slots))

	want := character.PactMagicSlotsForLevel(3)
	assert.Equal(t, want, slots)
	assert.Equal(t, 2, slots.SlotLevel)
	assert.Equal(t, 2, slots.Current)
	assert.Equal(t, 2, slots.Max)
}

// TestBuilderStoreAdapter_CreateCharacterRecord_NoPactMagicSlots_NonWarlock
// verifies non-pact classes do not write pact_magic_slots.
func TestBuilderStoreAdapter_CreateCharacterRecord_NoPactMagicSlots_NonWarlock(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Gandalf",
		Race:          "Human",
		Class:         "Wizard",
		AbilityScores: character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 16, WIS: 10, CHA: 10},
		HPMax:         8,
		AC:            12,
		SpeedFt:       30,
		ProfBonus:     2,
		Classes:       []character.ClassEntry{{Class: "Wizard", Level: 3}},
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	assert.False(t, creator.capturedParams.PactMagicSlots.Valid, "non-warlock should not persist pact magic slots")
}

// TestBuilderStoreAdapter_CreateCharacterRecord_PersistsSpellSlots_FullCaster
// (ISSUE-002) verifies a created full caster persists spell_slots in the
// canonical string-keyed {current,max} shape the play/read path expects.
// Without this the stored character has spell_slots = NULL and cannot cast
// leveled spells until a later level-up backfills the column.
func TestBuilderStoreAdapter_CreateCharacterRecord_PersistsSpellSlots_FullCaster(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Gandalf",
		Race:          "Human",
		Class:         "Wizard",
		AbilityScores: character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 16, WIS: 10, CHA: 10},
		HPMax:         18,
		AC:            12,
		SpeedFt:       30,
		ProfBonus:     2,
		Classes:       []character.ClassEntry{{Class: "Wizard", Level: 3}},
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	require.True(t, creator.capturedParams.SpellSlots.Valid, "SpellSlots should be set for a full caster")

	// Read path (combat.ParseSpellSlots / dashboard characterToPartyInfo) unmarshals
	// into map[string]character.SlotInfo — assert the persisted bytes match that shape.
	var slots map[string]character.SlotInfo
	require.NoError(t, json.Unmarshal(creator.capturedParams.SpellSlots.RawMessage, &slots))

	// Wizard L3 == caster level 3 == {level1:4, level2:2}.
	assert.Equal(t, character.SlotInfo{Current: 4, Max: 4}, slots["1"])
	assert.Equal(t, character.SlotInfo{Current: 2, Max: 2}, slots["2"])
	assert.Len(t, slots, 2, "Wizard L3 has exactly level-1 and level-2 slots")
}

// TestBuilderStoreAdapter_CreateCharacterRecord_NoSpellSlots_NonCaster
// (ISSUE-002) verifies a non-caster leaves spell_slots NULL — unchanged
// behavior for fighters/rogues/barbarians.
func TestBuilderStoreAdapter_CreateCharacterRecord_NoSpellSlots_NonCaster(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Conan",
		Race:          "Human",
		Class:         "Fighter",
		AbilityScores: character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 8, WIS: 10, CHA: 10},
		HPMax:         28,
		AC:            16,
		SpeedFt:       30,
		ProfBonus:     2,
		Classes:       []character.ClassEntry{{Class: "Fighter", Level: 3}},
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	assert.False(t, creator.capturedParams.SpellSlots.Valid, "non-caster should not persist spell slots")
}

// TestBuilderStoreAdapter_CreateCharacterRecord_PersistsSpellSlots_HalfCaster
// (ISSUE-002) verifies a half caster at level 2 (paladin) persists level-1
// slots in the canonical shape. Stays at level 2 to avoid the separate
// level-1 phantom-slot bug (ISSUE-006).
func TestBuilderStoreAdapter_CreateCharacterRecord_PersistsSpellSlots_HalfCaster(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	params := portal.CreateCharacterParams{
		CampaignID:    uuid.New().String(),
		Name:          "Aric",
		Race:          "Human",
		Class:         "Paladin",
		AbilityScores: character.AbilityScores{STR: 16, DEX: 10, CON: 14, INT: 8, WIS: 10, CHA: 14},
		HPMax:         20,
		AC:            18,
		SpeedFt:       30,
		ProfBonus:     2,
		Classes:       []character.ClassEntry{{Class: "Paladin", Level: 2}},
	}

	_, err := adapter.CreateCharacterRecord(context.Background(), params)
	require.NoError(t, err)

	require.True(t, creator.capturedParams.SpellSlots.Valid, "SpellSlots should be set for a half caster at level 2")

	var slots map[string]character.SlotInfo
	require.NoError(t, json.Unmarshal(creator.capturedParams.SpellSlots.RawMessage, &slots))

	// Paladin L2 == caster level 1 == {level1:2}.
	assert.Equal(t, character.SlotInfo{Current: 2, Max: 2}, slots["1"])
	assert.Len(t, slots, 1, "Paladin L2 has only level-1 slots")
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

// --- Draft persistence (T11 / Finding 4·b) ---------------------------------

func TestBuilderStoreAdapter_SaveCharacterDraft(t *testing.T) {
	creator := &captureCharacterCreator{}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)
	campID := uuid.New()
	draft := json.RawMessage(`{"v":1,"name":"Gimli"}`)

	err := adapter.SaveCharacterDraft(context.Background(), campID.String(), "user-1", "player", draft)
	require.NoError(t, err)
	assert.Equal(t, campID, creator.capturedUpsertDraft.CampaignID)
	assert.Equal(t, "user-1", creator.capturedUpsertDraft.DiscordUserID)
	assert.Equal(t, "player", creator.capturedUpsertDraft.Mode)
	assert.JSONEq(t, string(draft), string(creator.capturedUpsertDraft.Draft))
}

func TestBuilderStoreAdapter_SaveCharacterDraft_BadCampaignID(t *testing.T) {
	adapter := portal.NewBuilderStoreAdapter(&captureCharacterCreator{}, nil)

	err := adapter.SaveCharacterDraft(context.Background(), "not-a-uuid", "user-1", "player", json.RawMessage(`{}`))
	require.Error(t, err)
}

func TestBuilderStoreAdapter_LoadCharacterDraft(t *testing.T) {
	stored := json.RawMessage(`{"v":1,"race":"dwarf"}`)
	creator := &captureCharacterCreator{getDraftResult: stored}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	got, err := adapter.LoadCharacterDraft(context.Background(), uuid.New().String(), "user-1", "player")
	require.NoError(t, err)
	assert.JSONEq(t, string(stored), string(got))
}

func TestBuilderStoreAdapter_LoadCharacterDraft_NoRow(t *testing.T) {
	creator := &captureCharacterCreator{getDraftErr: sql.ErrNoRows}
	adapter := portal.NewBuilderStoreAdapter(creator, nil)

	got, err := adapter.LoadCharacterDraft(context.Background(), uuid.New().String(), "user-1", "player")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestBuilderStoreAdapter_LoadCharacterDraft_BadCampaignID(t *testing.T) {
	adapter := portal.NewBuilderStoreAdapter(&captureCharacterCreator{}, nil)

	_, err := adapter.LoadCharacterDraft(context.Background(), "not-a-uuid", "user-1", "player")
	require.Error(t, err)
}
