package dashboard

import (
	"context"
	"errors"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/portal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCharCreateStore implements CharCreateStore for testing.
type mockCharCreateStore struct {
	createCharErr  error
	createPCErr    error
	charID         string
	pcID           string
	lastCharParams portal.CreateCharacterParams
	lastPCParams   portal.CreatePlayerCharacterParams
}

func (m *mockCharCreateStore) CreateCharacterRecord(ctx context.Context, p portal.CreateCharacterParams) (string, error) {
	m.lastCharParams = p
	return m.charID, m.createCharErr
}

func (m *mockCharCreateStore) CreatePlayerCharacterRecord(ctx context.Context, p portal.CreatePlayerCharacterParams) (string, error) {
	m.lastPCParams = p
	return m.pcID, m.createPCErr
}

// mockFeatureProvider implements FeatureProvider for testing.
type mockFeatureProvider struct {
	classFeatures    map[string]map[string][]character.Feature
	subclassFeatures map[string]map[string]map[string][]character.Feature
	racialTraits     map[string][]character.Feature
}

func (m *mockFeatureProvider) ClassFeatures() map[string]map[string][]character.Feature {
	return m.classFeatures
}

func (m *mockFeatureProvider) SubclassFeatures() map[string]map[string]map[string][]character.Feature {
	return m.subclassFeatures
}

func (m *mockFeatureProvider) RacialTraits(race string) []character.Feature {
	if m.racialTraits == nil {
		return nil
	}
	return m.racialTraits[race]
}

func TestDMCharCreateService_CreateCharacter_Success(t *testing.T) {
	store := &mockCharCreateStore{
		charID: "char-123",
		pcID:   "pc-456",
	}
	svc := NewDMCharCreateService(store)

	sub := DMCharacterSubmission{
		Name:       "Thorin",
		Race:       "Dwarf",
		Background: "Soldier",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
		},
		AbilityScores: character.AbilityScores{
			STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 8, CHA: 10,
		},
	}

	result, err := svc.CreateCharacter(context.Background(), "campaign-1", sub)
	require.NoError(t, err)
	assert.Equal(t, "char-123", result.CharacterID)
	assert.Equal(t, "pc-456", result.PlayerCharacterID)

	// Verify character params
	assert.Equal(t, "Thorin", store.lastCharParams.Name)
	assert.Equal(t, "Dwarf", store.lastCharParams.Race)
	assert.Equal(t, "Fighter", store.lastCharParams.Class)
	assert.Equal(t, "Soldier", store.lastCharParams.Background)
	assert.Equal(t, 44, store.lastCharParams.HPMax) // Fighter 5 with CON 14

	// Verify player character params
	assert.Equal(t, "approved", store.lastPCParams.Status)
	assert.Equal(t, "dm_dashboard", store.lastPCParams.CreatedVia)
	assert.Empty(t, store.lastPCParams.DiscordUserID) // No player yet
}

func TestDMCharCreateService_CreateCharacter_ValidationFailure(t *testing.T) {
	store := &mockCharCreateStore{charID: "c1", pcID: "p1"}
	svc := NewDMCharCreateService(store)

	sub := DMCharacterSubmission{} // empty = invalid
	_, err := svc.CreateCharacter(context.Background(), "campaign-1", sub)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestDMCharCreateService_CreateCharacter_StoreError(t *testing.T) {
	store := &mockCharCreateStore{
		createCharErr: errors.New("db error"),
	}
	svc := NewDMCharCreateService(store)

	sub := DMCharacterSubmission{
		Name: "Thorin",
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}

	_, err := svc.CreateCharacter(context.Background(), "campaign-1", sub)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating character")
}

func TestDMCharCreateService_CreateCharacter_PCStoreError(t *testing.T) {
	store := &mockCharCreateStore{
		charID:      "char-1",
		createPCErr: errors.New("pc error"),
	}
	svc := NewDMCharCreateService(store)

	sub := DMCharacterSubmission{
		Name: "Thorin",
		Race: "Dwarf",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}

	_, err := svc.CreateCharacter(context.Background(), "campaign-1", sub)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating player character")
}

func TestDMCharCreateService_CreateCharacter_PassesEquipmentSpellsLanguages(t *testing.T) {
	store := &mockCharCreateStore{
		charID: "char-eq",
		pcID:   "pc-eq",
	}
	svc := NewDMCharCreateService(store)

	sub := DMCharacterSubmission{
		Name: "Elara",
		Race: "Elf",
		Classes: []character.ClassEntry{
			{Class: "Wizard", Level: 1},
		},
		AbilityScores: character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 12, CHA: 10},
		Equipment:     []string{"quarterstaff", "dagger"},
		Spells:        []string{"fire-bolt", "mage-hand", "shield", "magic-missile"},
		Languages:     []string{"Common", "Elvish"},
	}

	result, err := svc.CreateCharacter(context.Background(), "campaign-1", sub)
	require.NoError(t, err)
	assert.Equal(t, "char-eq", result.CharacterID)

	// Verify equipment is passed through
	assert.Equal(t, []string{"quarterstaff", "dagger"}, store.lastCharParams.Equipment)
	// Verify spells are passed through
	assert.Equal(t, []string{"fire-bolt", "mage-hand", "shield", "magic-missile"}, store.lastCharParams.Spells)
	// Verify languages are passed through
	assert.Equal(t, []string{"Common", "Elvish"}, store.lastCharParams.Languages)
}

func TestDMCharCreateService_CreateCharacter_PassesFeatures(t *testing.T) {
	store := &mockCharCreateStore{
		charID: "char-feat",
		pcID:   "pc-feat",
	}
	featureProvider := &mockFeatureProvider{
		classFeatures: map[string]map[string][]character.Feature{
			"Barbarian": {
				"1": {
					{Name: "Rage", Source: "Barbarian", Level: 1, Description: "Enter rage"},
					{Name: "Unarmored Defense", Source: "Barbarian", Level: 1, Description: "AC formula"},
				},
			},
		},
	}
	svc := NewDMCharCreateService(store, WithFeatureProvider(featureProvider))

	sub := DMCharacterSubmission{
		Name: "Grog",
		Race: "Half-Orc",
		Classes: []character.ClassEntry{
			{Class: "Barbarian", Level: 1},
		},
		AbilityScores: character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 8, WIS: 10, CHA: 10},
	}

	result, err := svc.CreateCharacter(context.Background(), "campaign-1", sub)
	require.NoError(t, err)
	assert.Equal(t, "char-feat", result.CharacterID)

	// Features should be populated
	require.NotEmpty(t, store.lastCharParams.Features)
	assert.Equal(t, "Rage", store.lastCharParams.Features[0].Name)
	assert.Equal(t, "Unarmored Defense", store.lastCharParams.Features[1].Name)
}

func TestDMCharCreateService_CreateCharacter_NoFeatureProvider(t *testing.T) {
	store := &mockCharCreateStore{
		charID: "char-nofeat",
		pcID:   "pc-nofeat",
	}
	// No feature provider — features should be empty but no error
	svc := NewDMCharCreateService(store)

	sub := DMCharacterSubmission{
		Name: "Test",
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
	}

	result, err := svc.CreateCharacter(context.Background(), "campaign-1", sub)
	require.NoError(t, err)
	assert.Equal(t, "char-nofeat", result.CharacterID)
	assert.Empty(t, store.lastCharParams.Features)
}

func TestDMCharCreateService_CreateCharacter_PassesEquippedWeaponAndArmor(t *testing.T) {
	store := &mockCharCreateStore{
		charID: "char-equip",
		pcID:   "pc-equip",
	}
	svc := NewDMCharCreateService(store)

	sub := DMCharacterSubmission{
		Name: "Knight",
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores:  character.AbilityScores{STR: 16, DEX: 12, CON: 14, INT: 10, WIS: 10, CHA: 10},
		Equipment:      []string{"longsword", "chain-mail", "shield"},
		EquippedWeapon: "longsword",
		WornArmor:      "chain-mail",
	}

	result, err := svc.CreateCharacter(context.Background(), "campaign-1", sub)
	require.NoError(t, err)
	assert.Equal(t, "char-equip", result.CharacterID)

	// Equipped weapon and worn armor should be in params
	assert.Equal(t, "longsword", store.lastCharParams.EquippedWeapon)
	assert.Equal(t, "chain-mail", store.lastCharParams.WornArmor)

	// AC should reflect armor: chain-mail = 16, + shield = 18
	assert.Equal(t, 18, store.lastCharParams.AC)
}

func TestDMCharCreateService_CreateCharacter_Multiclass(t *testing.T) {
	store := &mockCharCreateStore{
		charID: "char-mc",
		pcID:   "pc-mc",
	}
	svc := NewDMCharCreateService(store)

	sub := DMCharacterSubmission{
		Name: "Multiclass",
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 5},
			{Class: "Rogue", Subclass: "Assassin", Level: 3},
		},
		AbilityScores: character.AbilityScores{
			STR: 14, DEX: 16, CON: 12, INT: 10, WIS: 10, CHA: 8,
		},
	}

	result, err := svc.CreateCharacter(context.Background(), "campaign-1", sub)
	require.NoError(t, err)
	assert.Equal(t, "char-mc", result.CharacterID)

	// Primary class is stored in Class field
	assert.Equal(t, "Fighter", store.lastCharParams.Class)
	// Save proficiencies from primary class (Fighter: STR, CON)
	assert.Contains(t, store.lastCharParams.Saves, "str")
	assert.Contains(t, store.lastCharParams.Saves, "con")
}

// TestDMCharCreateService_CreateCharacter_Multiclass_ForwardsClasses
// guards SR-015: a DM-created Fighter-3 / Rogue-2 multiclass must
// persist BOTH class entries at the correct levels and surface the
// correct multiclass proficiency bonus, not collapse to a level-1
// single-class fallback.
func TestDMCharCreateService_CreateCharacter_Multiclass_ForwardsClasses(t *testing.T) {
	store := &mockCharCreateStore{
		charID: "char-fr",
		pcID:   "pc-fr",
	}
	svc := NewDMCharCreateService(store)

	sub := DMCharacterSubmission{
		Name: "Fightrogue",
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Subclass: "Champion", Level: 3},
			{Class: "Rogue", Subclass: "Thief", Level: 2},
		},
		AbilityScores: character.AbilityScores{
			STR: 14, DEX: 16, CON: 14, INT: 10, WIS: 10, CHA: 8,
		},
	}

	_, err := svc.CreateCharacter(context.Background(), "campaign-1", sub)
	require.NoError(t, err)

	// Full multiclass slice must reach the store.
	require.Len(t, store.lastCharParams.Classes, 2,
		"sub.Classes must be forwarded into CreateCharacterParams.Classes")
	assert.Equal(t, "Fighter", store.lastCharParams.Classes[0].Class)
	assert.Equal(t, "Champion", store.lastCharParams.Classes[0].Subclass)
	assert.Equal(t, 3, store.lastCharParams.Classes[0].Level)
	assert.Equal(t, "Rogue", store.lastCharParams.Classes[1].Class)
	assert.Equal(t, "Thief", store.lastCharParams.Classes[1].Subclass)
	assert.Equal(t, 2, store.lastCharParams.Classes[1].Level)

	// Total level 5 → proficiency bonus +3 (not +2 from a L1 fallback).
	assert.Equal(t, 3, store.lastCharParams.ProfBonus,
		"multiclass total level 5 must yield prof bonus +3")
}

func TestDMCharCreateService_CreateCharacter_AbilityMethodsUseCampaignGate(t *testing.T) {
	store := &mockCharCreateStore{charID: "char-method", pcID: "pc-method"}
	provider := portal.StaticAbilityMethodProvider([]portal.AbilityScoreMethod{portal.AbilityMethodRoll})
	svc := NewDMCharCreateService(store, WithDMAbilityMethodProvider(provider))

	sub := DMCharacterSubmission{
		Name: "Roller",
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityMethod: portal.AbilityMethodRoll,
		AbilityScores: character.AbilityScores{STR: 15, DEX: 13, CON: 12, INT: 12, WIS: 9, CHA: 6},
		AbilityRolls: map[string][]int{
			"str": []int{6, 5, 4, 1},
			"dex": []int{6, 4, 3, 1},
			"con": []int{4, 4, 4, 1},
			"int": []int{6, 3, 2, 3},
			"wis": []int{2, 2, 5, 2},
			"cha": []int{1, 2, 3, 1},
		},
	}

	_, err := svc.CreateCharacter(context.Background(), "campaign-1", sub)
	require.NoError(t, err)

	sub.AbilityRolls = nil
	_, err = svc.CreateCharacter(context.Background(), "campaign-1", sub)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "roll must include four d6 results")

	sub.AbilityRolls = map[string][]int{
		"str": []int{6, 5, 4, 1},
		"dex": []int{6, 4, 3, 1},
		"con": []int{4, 4, 4, 1},
		"int": []int{6, 3, 2, 3},
		"wis": []int{2, 2, 5, 2},
		"cha": []int{1, 2, 3, 1},
	}
	sub.AbilityMethod = portal.AbilityMethodStandardArray
	sub.AbilityScores = character.AbilityScores{STR: 15, DEX: 14, CON: 13, INT: 12, WIS: 10, CHA: 8}
	_, err = svc.CreateCharacter(context.Background(), "campaign-1", sub)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ability score method standard_array is not allowed")
}

func TestDMCharCreateService_CreateCharacter_OmittedAbilityMethodCannotBypassCampaignGate(t *testing.T) {
	store := &mockCharCreateStore{charID: "char-method", pcID: "pc-method"}
	provider := portal.StaticAbilityMethodProvider([]portal.AbilityScoreMethod{portal.AbilityMethodRoll})
	svc := NewDMCharCreateService(store, WithDMAbilityMethodProvider(provider))

	sub := DMCharacterSubmission{
		Name: "ManualBypass",
		Race: "Human",
		Classes: []character.ClassEntry{
			{Class: "Fighter", Level: 1},
		},
		AbilityScores: character.AbilityScores{STR: 18, DEX: 18, CON: 18, INT: 18, WIS: 18, CHA: 18},
	}

	_, err := svc.CreateCharacter(context.Background(), "campaign-1", sub)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ability score method point_buy is not allowed")
}
