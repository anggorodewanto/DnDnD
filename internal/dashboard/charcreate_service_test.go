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
	createCharErr    error
	createPCErr      error
	charID           string
	pcID             string
	lastCharParams   portal.CreateCharacterParams
	lastPCParams     portal.CreatePlayerCharacterParams
}

func (m *mockCharCreateStore) CreateCharacterRecord(ctx context.Context, p portal.CreateCharacterParams) (string, error) {
	m.lastCharParams = p
	return m.charID, m.createCharErr
}

func (m *mockCharCreateStore) CreatePlayerCharacterRecord(ctx context.Context, p portal.CreatePlayerCharacterParams) (string, error) {
	m.lastPCParams = p
	return m.pcID, m.createPCErr
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

func TestDMCreateStoreAdapter_Delegates(t *testing.T) {
	inner := &mockCharCreateStore{
		charID: "char-del",
		pcID:   "pc-del",
	}
	adapter := NewDMCreateStoreAdapter(inner)

	charID, err := adapter.CreateCharacterRecord(context.Background(), portal.CreateCharacterParams{Name: "Test"})
	require.NoError(t, err)
	assert.Equal(t, "char-del", charID)

	pcID, err := adapter.CreatePlayerCharacterRecord(context.Background(), portal.CreatePlayerCharacterParams{Status: "approved"})
	require.NoError(t, err)
	assert.Equal(t, "pc-del", pcID)
}

func TestClassSaveProficienciesLookup(t *testing.T) {
	saves := classSaveProficienciesLookup("Fighter")
	assert.Contains(t, saves, "str")
	assert.Contains(t, saves, "con")

	unknown := classSaveProficienciesLookup("Unknown")
	assert.Nil(t, unknown)
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
