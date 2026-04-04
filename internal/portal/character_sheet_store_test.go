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
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCharacterQuerier implements portal.CharacterQuerier for unit tests.
type mockCharacterQuerier struct {
	character refdata.Character
	charErr   error
	pc        refdata.PlayerCharacter
	pcErr     error
}

func (m *mockCharacterQuerier) GetCharacter(_ context.Context, id uuid.UUID) (refdata.Character, error) {
	return m.character, m.charErr
}

func (m *mockCharacterQuerier) GetPlayerCharacterByCharacter(_ context.Context, _ refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error) {
	return m.pc, m.pcErr
}

func TestCharacterSheetStoreAdapter_GetCharacterOwner(t *testing.T) {
	charID := uuid.New()
	campID := uuid.New()

	q := &mockCharacterQuerier{
		character: refdata.Character{ID: charID, CampaignID: campID},
		pc: refdata.PlayerCharacter{
			CharacterID:   charID,
			CampaignID:    campID,
			DiscordUserID: "user-123",
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	ownerID, err := store.GetCharacterOwner(context.Background(), charID.String())

	require.NoError(t, err)
	assert.Equal(t, "user-123", ownerID)
}

func TestCharacterSheetStoreAdapter_GetCharacterOwner_InvalidID(t *testing.T) {
	store := portal.NewCharacterSheetStoreAdapter(&mockCharacterQuerier{})
	_, err := store.GetCharacterOwner(context.Background(), "not-a-uuid")

	require.Error(t, err)
}

func TestCharacterSheetStoreAdapter_GetCharacterOwner_CharNotFound(t *testing.T) {
	q := &mockCharacterQuerier{
		charErr: sql.ErrNoRows,
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	_, err := store.GetCharacterOwner(context.Background(), uuid.New().String())

	require.Error(t, err)
	assert.ErrorIs(t, err, portal.ErrCharacterNotFound)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet(t *testing.T) {
	charID := uuid.New()
	campID := uuid.New()

	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 12, INT: 10, WIS: 8, CHA: 13}
	scoresJSON, _ := json.Marshal(scores)

	classes := []character.ClassEntry{{Class: "Fighter", Level: 3}}
	classesJSON, _ := json.Marshal(classes)

	profs := character.Proficiencies{Saves: []string{"str", "con"}, Skills: []string{"athletics"}}
	profsJSON, _ := json.Marshal(profs)

	features := []character.Feature{{Name: "Second Wind", Source: "Fighter", Level: 1, Description: "Heal"}}
	featuresJSON, _ := json.Marshal(features)

	inventory := []character.InventoryItem{{ItemID: "longsword", Name: "Longsword", Quantity: 1, Equipped: true, Type: "weapon"}}
	inventoryJSON, _ := json.Marshal(inventory)

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:               charID,
			CampaignID:       campID,
			Name:             "Thorn",
			Race:             "Human",
			Level:            3,
			Classes:          classesJSON,
			AbilityScores:    scoresJSON,
			HpMax:            28,
			HpCurrent:        25,
			TempHp:           5,
			Ac:               18,
			SpeedFt:          30,
			ProficiencyBonus: 2,
			EquippedMainHand: sql.NullString{String: "Longsword", Valid: true},
			Proficiencies:    pqtype.NullRawMessage{RawMessage: profsJSON, Valid: true},
			Features:         pqtype.NullRawMessage{RawMessage: featuresJSON, Valid: true},
			Languages:        []string{"Common", "Elvish"},
			Inventory:        pqtype.NullRawMessage{RawMessage: inventoryJSON, Valid: true},
			Gold:             50,
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	assert.Equal(t, "Thorn", data.Name)
	assert.Equal(t, "Human", data.Race)
	assert.Equal(t, 3, data.Level)
	assert.Equal(t, 28, data.HpMax)
	assert.Equal(t, 25, data.HpCurrent)
	assert.Equal(t, 5, data.TempHP)
	assert.Equal(t, 18, data.AC)
	assert.Equal(t, 30, data.SpeedFt)
	assert.Equal(t, 2, data.ProficiencyBonus)
	assert.Equal(t, "Longsword", data.EquippedMainHand)
	assert.Equal(t, 16, data.AbilityScores.STR)
	assert.Len(t, data.Classes, 1)
	assert.Equal(t, "Fighter", data.Classes[0].Class)
	assert.Equal(t, []string{"str", "con"}, data.Proficiencies.Saves)
	assert.Len(t, data.Features, 1)
	assert.Equal(t, "Second Wind", data.Features[0].Name)
	assert.Len(t, data.Inventory, 1)
	assert.Equal(t, "Longsword", data.Inventory[0].Name)
	assert.Equal(t, []string{"Common", "Elvish"}, data.Languages)
	assert.Equal(t, 50, data.Gold)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_InvalidID(t *testing.T) {
	store := portal.NewCharacterSheetStoreAdapter(&mockCharacterQuerier{})
	_, err := store.GetCharacterForSheet(context.Background(), "not-a-uuid")

	require.Error(t, err)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_NotFound(t *testing.T) {
	q := &mockCharacterQuerier{
		charErr: sql.ErrNoRows,
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	_, err := store.GetCharacterForSheet(context.Background(), uuid.New().String())

	require.Error(t, err)
	assert.ErrorIs(t, err, portal.ErrCharacterNotFound)
}
