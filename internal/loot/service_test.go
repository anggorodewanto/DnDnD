package loot_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/loot"
	"github.com/ab/dndnd/internal/refdata"
)

func setupTestDB(t *testing.T) (*sql.DB, *refdata.Queries) {
	t.Helper()
	db := sharedDB.AcquireDB(t)
	return db, refdata.New(db)
}

func createCampaign(t *testing.T, q *refdata.Queries) refdata.Campaign {
	t.Helper()
	camp, err := q.CreateCampaign(context.Background(), refdata.CreateCampaignParams{
		GuildID:  "guild-loot-test",
		DmUserID: "dm-1",
		Name:     "Loot Test Campaign",
		Settings: pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true},
	})
	require.NoError(t, err)
	return camp
}

func createEncounter(t *testing.T, q *refdata.Queries, campID uuid.UUID, status string) refdata.Encounter {
	t.Helper()
	enc, err := q.CreateEncounter(context.Background(), refdata.CreateEncounterParams{
		CampaignID:  campID,
		Name:        "Test Encounter",
		Status:      status,
		RoundNumber: 3,
	})
	require.NoError(t, err)
	return enc
}

func createCharWithInventory(t *testing.T, q *refdata.Queries, campID uuid.UUID, name string, gold int32, items []character.InventoryItem) refdata.Character {
	t.Helper()
	invJSON, _ := json.Marshal(items)
	char, err := q.CreateCharacter(context.Background(), refdata.CreateCharacterParams{
		CampaignID:       campID,
		Name:             name,
		Race:             "human",
		Classes:          json.RawMessage(`[{"class":"fighter","level":5}]`),
		Level:            5,
		AbilityScores:    json.RawMessage(`{"str":16,"dex":14,"con":14,"int":10,"wis":12,"cha":8}`),
		HpMax:            44,
		HpCurrent:        30,
		TempHp:           0,
		Ac:               18,
		SpeedFt:          30,
		ProficiencyBonus: 3,
		HitDiceRemaining: json.RawMessage(`{"d10":5}`),
		Gold:             gold,
		Inventory:        pqtype.NullRawMessage{RawMessage: invJSON, Valid: true},
		Languages:        []string{"common"},
	})
	require.NoError(t, err)
	return char
}

func createCombatant(t *testing.T, q *refdata.Queries, encID uuid.UUID, charID uuid.NullUUID, creatureRef sql.NullString, name string, isNPC bool, isAlive bool) refdata.Combatant {
	t.Helper()
	c, err := q.CreateCombatant(context.Background(), refdata.CreateCombatantParams{
		EncounterID: encID,
		CharacterID: charID,
		CreatureRefID: creatureRef,
		ShortID:     name[:2],
		DisplayName: name,
		HpMax:       20,
		HpCurrent:   0,
		Ac:          14,
		Conditions:  json.RawMessage(`[]`),
		IsVisible:   true,
		IsAlive:     isAlive,
		IsNpc:       isNPC,
	})
	require.NoError(t, err)
	return c
}

func createPlayerCharacter(t *testing.T, q *refdata.Queries, campID, charID uuid.UUID, discordUserID string) refdata.PlayerCharacter {
	t.Helper()
	pc, err := q.CreatePlayerCharacter(context.Background(), refdata.CreatePlayerCharacterParams{
		CampaignID:    campID,
		CharacterID:   charID,
		DiscordUserID: discordUserID,
		Status:        "approved",
		CreatedVia:    "import",
	})
	require.NoError(t, err)
	return pc
}

func TestCreateLootPool_NotFoundEncounter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)

	svc := loot.NewService(q)
	_, err := svc.CreateLootPool(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting encounter")
}

func TestGetLootPool_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	svc := loot.NewService(q)

	_, err := svc.GetLootPool(context.Background(), uuid.New())
	assert.ErrorIs(t, err, loot.ErrPoolNotFound)
}

func TestAddItem_PoolNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	svc := loot.NewService(q)

	_, err := svc.AddItem(context.Background(), uuid.New(), refdata.CreateLootPoolItemParams{
		Name: "Test", Quantity: 1, Type: "other",
	})
	assert.ErrorIs(t, err, loot.ErrPoolNotFound)
}

func TestRemoveItem_PoolNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	svc := loot.NewService(q)

	err := svc.RemoveItem(context.Background(), uuid.New(), uuid.New())
	assert.ErrorIs(t, err, loot.ErrPoolNotFound)
}

func TestClaimItem_PoolNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	svc := loot.NewService(q)

	_, err := svc.ClaimItem(context.Background(), uuid.New(), uuid.New(), uuid.New())
	assert.ErrorIs(t, err, loot.ErrPoolNotFound)
}

func TestSetGold_PoolNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	svc := loot.NewService(q)

	_, err := svc.SetGold(context.Background(), uuid.New(), 100)
	assert.ErrorIs(t, err, loot.ErrPoolNotFound)
}

func TestSplitGold_PoolNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	svc := loot.NewService(q)

	_, err := svc.SplitGold(context.Background(), uuid.New())
	assert.ErrorIs(t, err, loot.ErrPoolNotFound)
}

func TestClearPool_PoolNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	svc := loot.NewService(q)

	err := svc.ClearPool(context.Background(), uuid.New())
	assert.ErrorIs(t, err, loot.ErrPoolNotFound)
}

func TestSplitGold_ZeroGold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	svc := loot.NewService(q)
	ctx := context.Background()

	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), pool.Pool.GoldTotal)

	share, err := svc.SplitGold(ctx, pool.Pool.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), share)
}

func TestSplitGold_NoPartyMembers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	npcChar := createCharWithInventory(t, q, camp.ID, "Goblin", 30, nil)
	createCombatant(t, q, enc.ID,
		uuid.NullUUID{UUID: npcChar.ID, Valid: true},
		sql.NullString{}, "Goblin", true, false)

	svc := loot.NewService(q)
	ctx := context.Background()

	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)

	_, err = svc.SplitGold(ctx, pool.Pool.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no approved party members")
}

func TestClaimItem_ClosedPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	npcItems := []character.InventoryItem{
		{ItemID: "shortsword", Name: "Shortsword", Quantity: 1, Type: "weapon"},
	}
	npcChar := createCharWithInventory(t, q, camp.ID, "Goblin", 0, npcItems)
	createCombatant(t, q, enc.ID,
		uuid.NullUUID{UUID: npcChar.ID, Valid: true},
		sql.NullString{}, "Goblin", true, false)

	pcChar := createCharWithInventory(t, q, camp.ID, "Aria", 0, nil)

	svc := loot.NewService(q)
	ctx := context.Background()

	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)

	// Close pool
	err = svc.ClearPool(ctx, pool.Pool.ID)
	require.NoError(t, err)

	// Attempt claim on closed pool
	_, err = svc.ClaimItem(ctx, pool.Pool.ID, pool.Items[0].ID, pcChar.ID)
	assert.ErrorIs(t, err, loot.ErrPoolClosed)
}

func TestAddItem_ClosedPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	svc := loot.NewService(q)
	ctx := context.Background()

	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)

	err = svc.ClearPool(ctx, pool.Pool.ID)
	require.NoError(t, err)

	_, err = svc.AddItem(ctx, pool.Pool.ID, refdata.CreateLootPoolItemParams{
		Name: "Test", Quantity: 1, Type: "other",
	})
	assert.ErrorIs(t, err, loot.ErrPoolClosed)
}

func TestRemoveItem_ClosedPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	svc := loot.NewService(q)
	ctx := context.Background()

	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)

	err = svc.ClearPool(ctx, pool.Pool.ID)
	require.NoError(t, err)

	err = svc.RemoveItem(ctx, pool.Pool.ID, uuid.New())
	assert.ErrorIs(t, err, loot.ErrPoolClosed)
}

func TestSetGold_ClosedPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	svc := loot.NewService(q)
	ctx := context.Background()

	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)

	err = svc.ClearPool(ctx, pool.Pool.ID)
	require.NoError(t, err)

	_, err = svc.SetGold(ctx, pool.Pool.ID, 100)
	assert.ErrorIs(t, err, loot.ErrPoolClosed)
}

func TestSplitGold_ClosedPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	svc := loot.NewService(q)
	ctx := context.Background()

	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)

	err = svc.ClearPool(ctx, pool.Pool.ID)
	require.NoError(t, err)

	_, err = svc.SplitGold(ctx, pool.Pool.ID)
	assert.ErrorIs(t, err, loot.ErrPoolClosed)
}

func TestClearPool_AlreadyClosed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	svc := loot.NewService(q)
	ctx := context.Background()

	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)

	err = svc.ClearPool(ctx, pool.Pool.ID)
	require.NoError(t, err)

	err = svc.ClearPool(ctx, pool.Pool.ID)
	assert.ErrorIs(t, err, loot.ErrPoolClosed)
}

func TestFormatLootAnnouncement_GoldOnly(t *testing.T) {
	pool := refdata.LootPool{GoldTotal: 50}
	msg := loot.FormatLootAnnouncement(pool, nil)
	assert.Contains(t, msg, "50 gp")
	assert.Contains(t, msg, "/loot")
}

func TestFormatLootAnnouncement_SkipsClaimed(t *testing.T) {
	pool := refdata.LootPool{GoldTotal: 0}
	items := []refdata.LootPoolItem{
		{Name: "Shortsword", Quantity: 1, ClaimedBy: uuid.NullUUID{UUID: uuid.New(), Valid: true}},
	}
	msg := loot.FormatLootAnnouncement(pool, items)
	assert.Contains(t, msg, "No loot available")
}

func TestCreateLootPool_NotCompletedEncounter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "active")

	svc := loot.NewService(q)
	_, err := svc.CreateLootPool(context.Background(), enc.ID)
	assert.ErrorIs(t, err, loot.ErrEncounterNotCompleted)
}

func TestCreateLootPool_CompletedEncounter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	// Create a defeated NPC combatant with a character_id that has inventory
	npcItems := []character.InventoryItem{
		{ItemID: "shortsword", Name: "Shortsword", Quantity: 2, Type: "weapon"},
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 1, Type: "consumable"},
	}
	npcChar := createCharWithInventory(t, q, camp.ID, "Goblin Boss", 15, npcItems)
	createCombatant(t, q, enc.ID,
		uuid.NullUUID{UUID: npcChar.ID, Valid: true},
		sql.NullString{},
		"Goblin Boss", true, false) // NPC, dead

	svc := loot.NewService(q)
	ctx := context.Background()

	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)

	assert.Equal(t, enc.ID, pool.Pool.EncounterID)
	assert.Equal(t, camp.ID, pool.Pool.CampaignID)
	assert.Equal(t, int32(15), pool.Pool.GoldTotal)
	assert.Equal(t, "open", pool.Pool.Status)
	assert.Len(t, pool.Items, 2)
	assert.Equal(t, "Shortsword", pool.Items[0].Name)
	assert.Equal(t, int32(2), pool.Items[0].Quantity)
}

func TestClaimItem_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	npcItems := []character.InventoryItem{
		{ItemID: "shortsword", Name: "Shortsword", Quantity: 1, Type: "weapon"},
	}
	npcChar := createCharWithInventory(t, q, camp.ID, "Goblin", 0, npcItems)
	createCombatant(t, q, enc.ID,
		uuid.NullUUID{UUID: npcChar.ID, Valid: true},
		sql.NullString{}, "Goblin", true, false)

	// Create a PC to claim the item
	pcChar := createCharWithInventory(t, q, camp.ID, "Aria", 10, nil)

	svc := loot.NewService(q)
	ctx := context.Background()

	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)
	require.Len(t, pool.Items, 1)

	// Claim item
	claimed, err := svc.ClaimItem(ctx, pool.Pool.ID, pool.Items[0].ID, pcChar.ID)
	require.NoError(t, err)
	assert.True(t, claimed.ClaimedBy.Valid)
	assert.Equal(t, pcChar.ID, claimed.ClaimedBy.UUID)

	// Verify item is in character's inventory
	updatedChar, err := q.GetCharacter(ctx, pcChar.ID)
	require.NoError(t, err)
	var inv []character.InventoryItem
	require.NoError(t, json.Unmarshal(updatedChar.Inventory.RawMessage, &inv))
	require.Len(t, inv, 1)
	assert.Equal(t, "Shortsword", inv[0].Name)
}

func TestClaimItem_AlreadyClaimed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	npcItems := []character.InventoryItem{
		{ItemID: "shortsword", Name: "Shortsword", Quantity: 1, Type: "weapon"},
	}
	npcChar := createCharWithInventory(t, q, camp.ID, "Goblin", 0, npcItems)
	createCombatant(t, q, enc.ID,
		uuid.NullUUID{UUID: npcChar.ID, Valid: true},
		sql.NullString{}, "Goblin", true, false)

	pcChar1 := createCharWithInventory(t, q, camp.ID, "Aria", 0, nil)
	pcChar2 := createCharWithInventory(t, q, camp.ID, "Gorak", 0, nil)

	svc := loot.NewService(q)
	ctx := context.Background()

	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)

	// First claim succeeds
	_, err = svc.ClaimItem(ctx, pool.Pool.ID, pool.Items[0].ID, pcChar1.ID)
	require.NoError(t, err)

	// Second claim fails
	_, err = svc.ClaimItem(ctx, pool.Pool.ID, pool.Items[0].ID, pcChar2.ID)
	assert.ErrorIs(t, err, loot.ErrItemAlreadyClaimed)
}

func TestSplitGold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	npcChar := createCharWithInventory(t, q, camp.ID, "Goblin", 30, nil)
	createCombatant(t, q, enc.ID,
		uuid.NullUUID{UUID: npcChar.ID, Valid: true},
		sql.NullString{}, "Goblin", true, false)

	// Create two PCs as approved party members
	pc1 := createCharWithInventory(t, q, camp.ID, "Aria", 10, nil)
	pc2 := createCharWithInventory(t, q, camp.ID, "Gorak", 20, nil)
	createPlayerCharacter(t, q, camp.ID, pc1.ID, "discord-1")
	createPlayerCharacter(t, q, camp.ID, pc2.ID, "discord-2")

	svc := loot.NewService(q)
	ctx := context.Background()

	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(30), pool.Pool.GoldTotal)

	// Split gold
	share, err := svc.SplitGold(ctx, pool.Pool.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(15), share)

	// Verify characters received gold
	updPC1, err := q.GetCharacter(ctx, pc1.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(25), updPC1.Gold) // 10 + 15

	updPC2, err := q.GetCharacter(ctx, pc2.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(35), updPC2.Gold) // 20 + 15

	// Verify pool gold is zeroed
	updPool, err := q.GetLootPoolByEncounter(ctx, enc.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), updPool.GoldTotal)
}

func TestClearPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	npcItems := []character.InventoryItem{
		{ItemID: "shortsword", Name: "Shortsword", Quantity: 1, Type: "weapon"},
		{ItemID: "shield", Name: "Shield", Quantity: 1, Type: "armor"},
	}
	npcChar := createCharWithInventory(t, q, camp.ID, "Goblin", 0, npcItems)
	createCombatant(t, q, enc.ID,
		uuid.NullUUID{UUID: npcChar.ID, Valid: true},
		sql.NullString{}, "Goblin", true, false)

	pcChar := createCharWithInventory(t, q, camp.ID, "Aria", 0, nil)

	svc := loot.NewService(q)
	ctx := context.Background()

	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)

	// Claim one item
	_, err = svc.ClaimItem(ctx, pool.Pool.ID, pool.Items[0].ID, pcChar.ID)
	require.NoError(t, err)

	// Clear pool
	err = svc.ClearPool(ctx, pool.Pool.ID)
	require.NoError(t, err)

	// Verify pool is closed
	updPool, err := q.GetLootPool(ctx, pool.Pool.ID)
	require.NoError(t, err)
	assert.Equal(t, "closed", updPool.Status)

	// Verify only claimed item remains
	items, err := q.ListLootPoolItems(ctx, pool.Pool.ID)
	require.NoError(t, err)
	assert.Len(t, items, 1) // Only the claimed item remains
	assert.True(t, items[0].ClaimedBy.Valid)
}

func TestGetLootPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	svc := loot.NewService(q)
	ctx := context.Background()

	// Create empty pool
	_, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)

	result, err := svc.GetLootPool(ctx, enc.ID)
	require.NoError(t, err)
	assert.Equal(t, enc.ID, result.Pool.EncounterID)
}

func TestAddItem(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	svc := loot.NewService(q)
	ctx := context.Background()

	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)

	item, err := svc.AddItem(ctx, pool.Pool.ID, refdata.CreateLootPoolItemParams{
		Name:        "Mysterious Key",
		Description: "An ancient key that glows faintly blue",
		Quantity:    1,
		Type:        "other",
	})
	require.NoError(t, err)
	assert.Equal(t, "Mysterious Key", item.Name)
	assert.Equal(t, "An ancient key that glows faintly blue", item.Description)

	// Verify pool has item
	result, err := svc.GetLootPool(ctx, enc.ID)
	require.NoError(t, err)
	assert.Len(t, result.Items, 1)
}

func TestRemoveItem(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	npcItems := []character.InventoryItem{
		{ItemID: "shortsword", Name: "Shortsword", Quantity: 1, Type: "weapon"},
	}
	npcChar := createCharWithInventory(t, q, camp.ID, "Goblin", 0, npcItems)
	createCombatant(t, q, enc.ID,
		uuid.NullUUID{UUID: npcChar.ID, Valid: true},
		sql.NullString{}, "Goblin", true, false)

	svc := loot.NewService(q)
	ctx := context.Background()

	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)
	require.Len(t, pool.Items, 1)

	err = svc.RemoveItem(ctx, pool.Pool.ID, pool.Items[0].ID)
	require.NoError(t, err)

	result, err := svc.GetLootPool(ctx, enc.ID)
	require.NoError(t, err)
	assert.Len(t, result.Items, 0)
}

func TestSetGold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	svc := loot.NewService(q)
	ctx := context.Background()

	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)

	updated, err := svc.SetGold(ctx, pool.Pool.ID, 100)
	require.NoError(t, err)
	assert.Equal(t, int32(100), updated.GoldTotal)
}

func TestFormatLootAnnouncement(t *testing.T) {
	pool := refdata.LootPool{GoldTotal: 15}
	items := []refdata.LootPoolItem{
		{Name: "Shortsword", Quantity: 2},
		{Name: "Healing Potion", Quantity: 1},
		{Name: "Mysterious Key", Quantity: 1},
	}

	msg := loot.FormatLootAnnouncement(pool, items)
	assert.Contains(t, msg, "Shortsword \u00d72")
	assert.Contains(t, msg, "Healing Potion")
	assert.Contains(t, msg, "Mysterious Key")
	assert.Contains(t, msg, "15 gp")
	assert.Contains(t, msg, "/loot")
}

func TestFormatLootAnnouncement_Empty(t *testing.T) {
	pool := refdata.LootPool{GoldTotal: 0}
	msg := loot.FormatLootAnnouncement(pool, nil)
	assert.Contains(t, msg, "No loot available")
}

func TestCreateLootPool_DuplicateRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	svc := loot.NewService(q)
	ctx := context.Background()

	_, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)

	_, err = svc.CreateLootPool(ctx, enc.ID)
	assert.ErrorIs(t, err, loot.ErrPoolAlreadyExists)
}

func TestPoolLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	npcItems := []character.InventoryItem{
		{ItemID: "shortsword", Name: "Shortsword", Quantity: 1, Type: "weapon"},
	}
	npcChar := createCharWithInventory(t, q, camp.ID, "Goblin", 30, npcItems)
	createCombatant(t, q, enc.ID,
		uuid.NullUUID{UUID: npcChar.ID, Valid: true},
		sql.NullString{}, "Goblin", true, false)

	pcChar := createCharWithInventory(t, q, camp.ID, "Aria", 5, nil)
	createPlayerCharacter(t, q, camp.ID, pcChar.ID, "discord-1")

	svc := loot.NewService(q)
	ctx := context.Background()

	// Create pool
	pool, err := svc.CreateLootPool(ctx, enc.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(30), pool.Pool.GoldTotal)
	assert.Len(t, pool.Items, 1)

	// Add item
	_, err = svc.AddItem(ctx, pool.Pool.ID, refdata.CreateLootPoolItemParams{
		Name:     "Mysterious Key",
		Quantity: 1,
		Type:     "other",
	})
	require.NoError(t, err)

	// Claim item
	_, err = svc.ClaimItem(ctx, pool.Pool.ID, pool.Items[0].ID, pcChar.ID)
	require.NoError(t, err)

	// Split gold
	share, err := svc.SplitGold(ctx, pool.Pool.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(30), share)

	// Verify character has gold and item
	updChar, err := q.GetCharacter(ctx, pcChar.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(35), updChar.Gold) // 5 + 30

	// Clear pool (removes unclaimed Mysterious Key, keeps claimed Shortsword)
	err = svc.ClearPool(ctx, pool.Pool.ID)
	require.NoError(t, err)

	// Verify pool is closed
	result, err := svc.GetLootPool(ctx, enc.ID)
	require.NoError(t, err)
	assert.Equal(t, "closed", result.Pool.Status)
	assert.Len(t, result.Items, 1) // Only claimed item
}
