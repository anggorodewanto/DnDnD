package inventory_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/inventory"
	"github.com/ab/dndnd/internal/refdata"
)

func setupTestDB(t *testing.T) (*sql.DB, *refdata.Queries) {
	t.Helper()
	db := sharedDB.AcquireDB(t)
	return db, refdata.New(db)
}

func createTestCampaign(t *testing.T, q *refdata.Queries) refdata.Campaign {
	t.Helper()
	camp, err := q.CreateCampaign(context.Background(), refdata.CreateCampaignParams{
		GuildID:  "guild-inv-test",
		DmUserID: "dm-1",
		Name:     "Test Campaign",
		Settings: pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true},
	})
	require.NoError(t, err)
	return camp
}

func createTestCharacter(t *testing.T, q *refdata.Queries, campID uuid.UUID, name string, gold int32, items []character.InventoryItem) refdata.Character {
	t.Helper()
	invJSON, _ := json.Marshal(items)
	char, err := q.CreateCharacter(context.Background(), refdata.CreateCharacterParams{
		CampaignID:     campID,
		Name:           name,
		Race:           "human",
		Classes:        json.RawMessage(`[{"class":"fighter","level":5}]`),
		Level:          5,
		AbilityScores:  json.RawMessage(`{"str":16,"dex":14,"con":14,"int":10,"wis":12,"cha":8}`),
		HpMax:          44,
		HpCurrent:      30,
		TempHp:         0,
		Ac:             18,
		SpeedFt:        30,
		ProficiencyBonus: 3,
		HitDiceRemaining: json.RawMessage(`{"d10":5}`),
		Gold:           gold,
		Inventory:      pqtype.NullRawMessage{RawMessage: invJSON, Valid: true},
		Languages:      []string{"common"},
	})
	require.NoError(t, err)
	return char
}

func TestIntegration_AddAndRemoveItems(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createTestCampaign(t, q)

	items := []character.InventoryItem{
		{ItemID: "torch", Name: "Torch", Quantity: 3, Type: "other"},
	}
	char := createTestCharacter(t, q, camp.ID, "Aria", 50, items)

	handler := inventory.NewAPIHandler(q)

	// Add an item
	body, _ := json.Marshal(inventory.AddItemRequest{
		CharacterID: char.ID.String(),
		Item: character.InventoryItem{
			ItemID:   "healing-potion",
			Name:     "Healing Potion",
			Quantity: 2,
			Type:     "consumable",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/add", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAddItem(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify
	updated, err := q.GetCharacter(context.Background(), char.ID)
	require.NoError(t, err)

	var updatedItems []character.InventoryItem
	require.NoError(t, json.Unmarshal(updated.Inventory.RawMessage, &updatedItems))
	require.Len(t, updatedItems, 2)
	assert.Equal(t, "healing-potion", updatedItems[1].ItemID)
	assert.Equal(t, 2, updatedItems[1].Quantity)

	// Remove one healing potion
	body, _ = json.Marshal(inventory.RemoveItemRequest{
		CharacterID: char.ID.String(),
		ItemID:      "healing-potion",
		Quantity:    1,
	})

	req = httptest.NewRequest(http.MethodPost, "/api/inventory/remove", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.HandleRemoveItem(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify
	updated, err = q.GetCharacter(context.Background(), char.ID)
	require.NoError(t, err)

	require.NoError(t, json.Unmarshal(updated.Inventory.RawMessage, &updatedItems))
	require.Len(t, updatedItems, 2)
	assert.Equal(t, 1, updatedItems[1].Quantity)
}

func TestIntegration_TransferItem(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createTestCampaign(t, q)

	fromItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 3, Type: "consumable"},
	}
	toItems := []character.InventoryItem{}

	from := createTestCharacter(t, q, camp.ID, "Aria", 50, fromItems)
	to := createTestCharacter(t, q, camp.ID, "Gorak", 10, toItems)

	handler := inventory.NewAPIHandler(q)

	body, _ := json.Marshal(inventory.TransferItemRequest{
		FromCharacterID: from.ID.String(),
		ToCharacterID:   to.ID.String(),
		ItemID:          "healing-potion",
		Quantity:        2,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/transfer", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleTransferItem(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify from
	updatedFrom, err := q.GetCharacter(context.Background(), from.ID)
	require.NoError(t, err)
	var fromInv []character.InventoryItem
	require.NoError(t, json.Unmarshal(updatedFrom.Inventory.RawMessage, &fromInv))
	require.Len(t, fromInv, 1)
	assert.Equal(t, 1, fromInv[0].Quantity)

	// Verify to
	updatedTo, err := q.GetCharacter(context.Background(), to.ID)
	require.NoError(t, err)
	var toInv []character.InventoryItem
	require.NoError(t, json.Unmarshal(updatedTo.Inventory.RawMessage, &toInv))
	require.Len(t, toInv, 1)
	assert.Equal(t, 2, toInv[0].Quantity)
}

func TestIntegration_SetGold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createTestCampaign(t, q)

	char := createTestCharacter(t, q, camp.ID, "Aria", 50, nil)

	handler := inventory.NewAPIHandler(q)

	body, _ := json.Marshal(inventory.SetGoldRequest{
		CharacterID: char.ID.String(),
		Gold:        200,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/inventory/gold", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleSetGold(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify
	updated, err := q.GetCharacter(context.Background(), char.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(200), updated.Gold)
}

func TestIntegration_UseConsumableAndPersist(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createTestCampaign(t, q)

	items := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	char := createTestCharacter(t, q, camp.ID, "Aria", 0, items)

	// Use the service to consume
	svc := inventory.NewService(func(max int) int { return 3 })
	result, err := svc.UseConsumable(inventory.UseInput{
		Items:     items,
		ItemID:    "healing-potion",
		HPCurrent: int(char.HpCurrent),
		HPMax:     int(char.HpMax),
	})
	require.NoError(t, err)
	assert.True(t, result.AutoResolved)
	assert.Equal(t, 8, result.HealingDone) // 2d4+2 with d4=3 => 3+3+2=8

	// Persist the changes
	invJSON, _ := json.Marshal(result.UpdatedItems)
	_, err = q.UpdateCharacterInventoryAndHP(context.Background(), refdata.UpdateCharacterInventoryAndHPParams{
		ID:        char.ID,
		Inventory: pqtype.NullRawMessage{RawMessage: invJSON, Valid: true},
		HpCurrent: int32(result.HPAfter),
	})
	require.NoError(t, err)

	// Verify
	updated, err := q.GetCharacter(context.Background(), char.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(38), updated.HpCurrent) // 30 + 8

	var updatedInv []character.InventoryItem
	require.NoError(t, json.Unmarshal(updated.Inventory.RawMessage, &updatedInv))
	require.Len(t, updatedInv, 1)
	assert.Equal(t, 1, updatedInv[0].Quantity)
}
