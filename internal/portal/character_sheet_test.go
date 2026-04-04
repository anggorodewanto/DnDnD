package portal_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/portal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCharacterSheetStore implements portal.CharacterSheetStore for tests.
type mockCharacterSheetStore struct {
	character *portal.CharacterSheetData
	err       error
	ownerID   string
	ownerErr  error
}

func (m *mockCharacterSheetStore) GetCharacterForSheet(_ context.Context, characterID string) (*portal.CharacterSheetData, error) {
	return m.character, m.err
}

func (m *mockCharacterSheetStore) GetCharacterOwner(_ context.Context, characterID string) (string, error) {
	return m.ownerID, m.ownerErr
}

func TestLoadCharacterSheet_Success(t *testing.T) {
	store := &mockCharacterSheetStore{
		ownerID: "user-123",
		character: &portal.CharacterSheetData{
			ID:    "char-1",
			Name:  "Thorn",
			Race:  "Human",
			Level: 3,
			Classes: []character.ClassEntry{
				{Class: "Fighter", Level: 3},
			},
			AbilityScores: character.AbilityScores{STR: 16, DEX: 14, CON: 12, INT: 10, WIS: 8, CHA: 13},
		},
	}

	svc := portal.NewCharacterSheetService(store)
	data, err := svc.LoadCharacterSheet(context.Background(), "char-1", "user-123")

	require.NoError(t, err)
	assert.Equal(t, "Thorn", data.Name)
	assert.Equal(t, "Human", data.Race)
	assert.Equal(t, 3, data.Level)
}

func TestLoadCharacterSheet_WrongOwner(t *testing.T) {
	store := &mockCharacterSheetStore{
		ownerID: "user-other",
		character: &portal.CharacterSheetData{
			ID:   "char-1",
			Name: "Thorn",
		},
	}

	svc := portal.NewCharacterSheetService(store)
	_, err := svc.LoadCharacterSheet(context.Background(), "char-1", "user-attacker")

	require.Error(t, err)
	assert.True(t, errors.Is(err, portal.ErrNotOwner))
}

func TestLoadCharacterSheet_OwnerLookupError(t *testing.T) {
	store := &mockCharacterSheetStore{
		ownerErr: errors.New("db error"),
	}

	svc := portal.NewCharacterSheetService(store)
	_, err := svc.LoadCharacterSheet(context.Background(), "char-1", "user-123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestLoadCharacterSheet_CharacterNotFound(t *testing.T) {
	store := &mockCharacterSheetStore{
		ownerID: "user-123",
		err:     portal.ErrCharacterNotFound,
	}

	svc := portal.NewCharacterSheetService(store)
	_, err := svc.LoadCharacterSheet(context.Background(), "char-1", "user-123")

	require.Error(t, err)
	assert.True(t, errors.Is(err, portal.ErrCharacterNotFound))
}

func TestLoadCharacterSheet_ComputedFields(t *testing.T) {
	store := &mockCharacterSheetStore{
		ownerID: "user-123",
		character: &portal.CharacterSheetData{
			ID:               "char-1",
			Name:             "Thorn",
			Race:             "Human",
			Level:            5,
			ProficiencyBonus: 3,
			Classes: []character.ClassEntry{
				{Class: "Fighter", Level: 3},
				{Class: "Rogue", Subclass: "Thief", Level: 2},
			},
			AbilityScores: character.AbilityScores{STR: 16, DEX: 14, CON: 12, INT: 10, WIS: 8, CHA: 13},
			Proficiencies: character.Proficiencies{
				Saves:  []string{"str", "con"},
				Skills: []string{"athletics", "perception"},
			},
		},
	}

	svc := portal.NewCharacterSheetService(store)
	data, err := svc.LoadCharacterSheet(context.Background(), "char-1", "user-123")
	require.NoError(t, err)

	// Ability modifiers
	assert.Equal(t, 3, data.AbilityModifiers["STR"])
	assert.Equal(t, 2, data.AbilityModifiers["DEX"])
	assert.Equal(t, 1, data.AbilityModifiers["CON"])
	assert.Equal(t, 0, data.AbilityModifiers["INT"])
	assert.Equal(t, -1, data.AbilityModifiers["WIS"])
	assert.Equal(t, 1, data.AbilityModifiers["CHA"])

	// Class summary
	assert.Equal(t, "Fighter 3 / Rogue 2 (Thief)", data.ClassSummary)

	// Skills contain proficiency indicators
	var athletics, stealth portal.SkillDisplay
	for _, s := range data.Skills {
		if s.Name == "Athletics" {
			athletics = s
		}
		if s.Name == "Stealth" {
			stealth = s
		}
	}
	assert.True(t, athletics.Proficient)
	assert.Equal(t, 6, athletics.Modifier) // 3 (STR mod) + 3 (prof bonus)
	assert.False(t, stealth.Proficient)
	assert.Equal(t, 2, stealth.Modifier) // 2 (DEX mod), no proficiency

	// Saving throws
	var strSave, wisSave portal.SavingThrowDisplay
	for _, st := range data.SavingThrows {
		if st.Ability == "STR" {
			strSave = st
		}
		if st.Ability == "WIS" {
			wisSave = st
		}
	}
	assert.True(t, strSave.Proficient)
	assert.Equal(t, 6, strSave.Modifier) // 3 (STR mod) + 3 (prof bonus)
	assert.False(t, wisSave.Proficient)
	assert.Equal(t, -1, wisSave.Modifier) // -1 (WIS mod)

	// 18 skills total
	assert.Len(t, data.Skills, 18)
	// 6 saving throws
	assert.Len(t, data.SavingThrows, 6)
}
