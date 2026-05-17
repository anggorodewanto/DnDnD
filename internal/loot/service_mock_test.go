package loot_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/loot"
	"github.com/ab/dndnd/internal/refdata"
)

// mockStore implements loot.Store for unit testing error branches.
type mockStore struct {
	getEncounterFn                          func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	listEncountersByCampaignIDFn            func(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error)
	listCombatantsByEncounterIDFn           func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	getCharacterFn                          func(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	createLootPoolFn                        func(ctx context.Context, arg refdata.CreateLootPoolParams) (refdata.LootPool, error)
	createLootPoolItemFn                    func(ctx context.Context, arg refdata.CreateLootPoolItemParams) (refdata.LootPoolItem, error)
	getLootPoolByEncounterFn                func(ctx context.Context, encounterID uuid.UUID) (refdata.LootPool, error)
	getLootPoolFn                           func(ctx context.Context, id uuid.UUID) (refdata.LootPool, error)
	listLootPoolItemsFn                     func(ctx context.Context, lootPoolID uuid.UUID) ([]refdata.LootPoolItem, error)
	claimLootPoolItemFn                     func(ctx context.Context, arg refdata.ClaimLootPoolItemParams) (refdata.LootPoolItem, error)
	updateLootPoolGoldFn                    func(ctx context.Context, arg refdata.UpdateLootPoolGoldParams) (refdata.LootPool, error)
	updateLootPoolStatusFn                  func(ctx context.Context, arg refdata.UpdateLootPoolStatusParams) (refdata.LootPool, error)
	deleteLootPoolItemFn                    func(ctx context.Context, id uuid.UUID) error
	deleteUnclaimedLootPoolItemsFn          func(ctx context.Context, lootPoolID uuid.UUID) error
	deleteLootPoolFn                        func(ctx context.Context, id uuid.UUID) error
	listPlayerCharactersByCampaignApprovedFn func(ctx context.Context, campaignID uuid.UUID) ([]refdata.ListPlayerCharactersByCampaignApprovedRow, error)
	updateCharacterGoldFn                   func(ctx context.Context, arg refdata.UpdateCharacterGoldParams) (refdata.Character, error)
	updateCharacterInventoryFn              func(ctx context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error)
}

func (m *mockStore) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	return m.getEncounterFn(ctx, id)
}
func (m *mockStore) ListEncountersByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Encounter, error) {
	if m.listEncountersByCampaignIDFn == nil {
		return nil, nil
	}
	return m.listEncountersByCampaignIDFn(ctx, campaignID)
}
func (m *mockStore) ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return m.listCombatantsByEncounterIDFn(ctx, encounterID)
}
func (m *mockStore) GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
	return m.getCharacterFn(ctx, id)
}
func (m *mockStore) CreateLootPool(ctx context.Context, arg refdata.CreateLootPoolParams) (refdata.LootPool, error) {
	return m.createLootPoolFn(ctx, arg)
}
func (m *mockStore) CreateLootPoolItem(ctx context.Context, arg refdata.CreateLootPoolItemParams) (refdata.LootPoolItem, error) {
	return m.createLootPoolItemFn(ctx, arg)
}
func (m *mockStore) GetLootPoolByEncounter(ctx context.Context, encounterID uuid.UUID) (refdata.LootPool, error) {
	return m.getLootPoolByEncounterFn(ctx, encounterID)
}
func (m *mockStore) GetLootPool(ctx context.Context, id uuid.UUID) (refdata.LootPool, error) {
	return m.getLootPoolFn(ctx, id)
}
func (m *mockStore) ListLootPoolItems(ctx context.Context, lootPoolID uuid.UUID) ([]refdata.LootPoolItem, error) {
	return m.listLootPoolItemsFn(ctx, lootPoolID)
}
func (m *mockStore) ClaimLootPoolItem(ctx context.Context, arg refdata.ClaimLootPoolItemParams) (refdata.LootPoolItem, error) {
	return m.claimLootPoolItemFn(ctx, arg)
}
func (m *mockStore) UpdateLootPoolGold(ctx context.Context, arg refdata.UpdateLootPoolGoldParams) (refdata.LootPool, error) {
	return m.updateLootPoolGoldFn(ctx, arg)
}
func (m *mockStore) UpdateLootPoolStatus(ctx context.Context, arg refdata.UpdateLootPoolStatusParams) (refdata.LootPool, error) {
	return m.updateLootPoolStatusFn(ctx, arg)
}
func (m *mockStore) DeleteLootPoolItem(ctx context.Context, id uuid.UUID) error {
	return m.deleteLootPoolItemFn(ctx, id)
}
func (m *mockStore) DeleteUnclaimedLootPoolItems(ctx context.Context, lootPoolID uuid.UUID) error {
	return m.deleteUnclaimedLootPoolItemsFn(ctx, lootPoolID)
}
func (m *mockStore) UpdateLootPoolItem(_ context.Context, arg refdata.UpdateLootPoolItemParams) (refdata.LootPoolItem, error) {
	return refdata.LootPoolItem{ID: arg.ID, Name: arg.Name.String}, nil
}
func (m *mockStore) DeleteLootPool(ctx context.Context, id uuid.UUID) error {
	return m.deleteLootPoolFn(ctx, id)
}
func (m *mockStore) ListPlayerCharactersByCampaignApproved(ctx context.Context, campaignID uuid.UUID) ([]refdata.ListPlayerCharactersByCampaignApprovedRow, error) {
	return m.listPlayerCharactersByCampaignApprovedFn(ctx, campaignID)
}
func (m *mockStore) UpdateCharacterGold(ctx context.Context, arg refdata.UpdateCharacterGoldParams) (refdata.Character, error) {
	return m.updateCharacterGoldFn(ctx, arg)
}
func (m *mockStore) UpdateCharacterInventory(ctx context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error) {
	return m.updateCharacterInventoryFn(ctx, arg)
}

var (
	errDB     = errors.New("db error")
	testPool  = refdata.LootPool{ID: uuid.New(), Status: "open", CampaignID: uuid.New()}
	testItem  = refdata.LootPoolItem{ID: uuid.New(), Name: "Sword", Quantity: 1, Type: "weapon"}
)

// --- ClaimItem error branch tests ---

func TestClaimItem_GetCharacterFailure(t *testing.T) {
	charID := uuid.New()
	store := &mockStore{
		getLootPoolFn: func(_ context.Context, _ uuid.UUID) (refdata.LootPool, error) {
			return testPool, nil
		},
		claimLootPoolItemFn: func(_ context.Context, _ refdata.ClaimLootPoolItemParams) (refdata.LootPoolItem, error) {
			return testItem, nil
		},
		getCharacterFn: func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
			return refdata.Character{}, errDB
		},
	}

	svc := loot.NewService(store)
	claimed, err := svc.ClaimItem(context.Background(), testPool.ID, testItem.ID, charID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
	assert.Equal(t, testItem.Name, claimed.Name) // claimed item still returned
}

func TestClaimItem_UpdateCharacterInventoryFailure(t *testing.T) {
	charID := uuid.New()
	store := &mockStore{
		getLootPoolFn: func(_ context.Context, _ uuid.UUID) (refdata.LootPool, error) {
			return testPool, nil
		},
		claimLootPoolItemFn: func(_ context.Context, _ refdata.ClaimLootPoolItemParams) (refdata.LootPoolItem, error) {
			return testItem, nil
		},
		getCharacterFn: func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
			return refdata.Character{
				ID:        charID,
				Inventory: pqtype.NullRawMessage{RawMessage: []byte(`[]`), Valid: true},
			}, nil
		},
		updateCharacterInventoryFn: func(_ context.Context, _ refdata.UpdateCharacterInventoryParams) (refdata.Character, error) {
			return refdata.Character{}, errDB
		},
	}

	svc := loot.NewService(store)
	claimed, err := svc.ClaimItem(context.Background(), testPool.ID, testItem.ID, charID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating inventory")
	assert.Equal(t, testItem.Name, claimed.Name)
}

// --- CreateLootPool error branch: CreateLootPoolItem failure ---

func TestCreateLootPool_CreateLootPoolItemFailure(t *testing.T) {
	encID := uuid.New()
	campID := uuid.New()
	charID := uuid.New()

	store := &mockStore{
		getEncounterFn: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encID, CampaignID: campID, Status: "completed"}, nil
		},
		getLootPoolByEncounterFn: func(_ context.Context, _ uuid.UUID) (refdata.LootPool, error) {
			return refdata.LootPool{}, sql.ErrNoRows
		},
		listCombatantsByEncounterIDFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{
					IsNpc:       true,
					IsAlive:     false,
					CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
				},
			}, nil
		},
		getCharacterFn: func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
			return refdata.Character{
				ID:        charID,
				Gold:      10,
				Inventory: pqtype.NullRawMessage{RawMessage: []byte(`[{"name":"Sword","quantity":1,"type":"weapon"}]`), Valid: true},
			}, nil
		},
		updateCharacterGoldFn: func(_ context.Context, _ refdata.UpdateCharacterGoldParams) (refdata.Character, error) {
			return refdata.Character{}, nil
		},
		updateCharacterInventoryFn: func(_ context.Context, _ refdata.UpdateCharacterInventoryParams) (refdata.Character, error) {
			return refdata.Character{}, nil
		},
		createLootPoolFn: func(_ context.Context, _ refdata.CreateLootPoolParams) (refdata.LootPool, error) {
			return refdata.LootPool{ID: uuid.New(), EncounterID: encID, CampaignID: campID, GoldTotal: 10, Status: "open"}, nil
		},
		createLootPoolItemFn: func(_ context.Context, _ refdata.CreateLootPoolItemParams) (refdata.LootPoolItem, error) {
			return refdata.LootPoolItem{}, errDB
		},
	}

	svc := loot.NewService(store)
	_, err := svc.CreateLootPool(context.Background(), encID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating loot pool item")
}

// --- ClearPool error branches ---

func TestClearPool_DeleteUnclaimedItemsFailure(t *testing.T) {
	store := &mockStore{
		getLootPoolFn: func(_ context.Context, _ uuid.UUID) (refdata.LootPool, error) {
			return testPool, nil
		},
		deleteUnclaimedLootPoolItemsFn: func(_ context.Context, _ uuid.UUID) error {
			return errDB
		},
	}

	svc := loot.NewService(store)
	err := svc.ClearPool(context.Background(), testPool.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deleting unclaimed items")
}

func TestClearPool_UpdateStatusFailure(t *testing.T) {
	store := &mockStore{
		getLootPoolFn: func(_ context.Context, _ uuid.UUID) (refdata.LootPool, error) {
			return testPool, nil
		},
		deleteUnclaimedLootPoolItemsFn: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
		updateLootPoolStatusFn: func(_ context.Context, _ refdata.UpdateLootPoolStatusParams) (refdata.LootPool, error) {
			return refdata.LootPool{}, errDB
		},
	}

	svc := loot.NewService(store)
	err := svc.ClearPool(context.Background(), testPool.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closing pool")
}

// --- GetLootPool error branch: ListLootPoolItems failure ---

func TestGetLootPool_ListItemsFailure(t *testing.T) {
	encID := uuid.New()
	store := &mockStore{
		getLootPoolByEncounterFn: func(_ context.Context, _ uuid.UUID) (refdata.LootPool, error) {
			return testPool, nil
		},
		listLootPoolItemsFn: func(_ context.Context, _ uuid.UUID) ([]refdata.LootPoolItem, error) {
			return nil, errDB
		},
	}

	svc := loot.NewService(store)
	_, err := svc.GetLootPool(context.Background(), encID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "listing loot pool items")
}

// --- SplitGold error branches ---

func TestSplitGold_ListPartyMembersFailure(t *testing.T) {
	pool := refdata.LootPool{ID: uuid.New(), Status: "open", CampaignID: uuid.New(), GoldTotal: 100}
	store := &mockStore{
		getLootPoolFn: func(_ context.Context, _ uuid.UUID) (refdata.LootPool, error) {
			return pool, nil
		},
		listPlayerCharactersByCampaignApprovedFn: func(_ context.Context, _ uuid.UUID) ([]refdata.ListPlayerCharactersByCampaignApprovedRow, error) {
			return nil, errDB
		},
	}

	svc := loot.NewService(store)
	_, err := svc.SplitGold(context.Background(), pool.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "listing party members")
}

func TestSplitGold_UpdateCharacterGoldFailure(t *testing.T) {
	pool := refdata.LootPool{ID: uuid.New(), Status: "open", CampaignID: uuid.New(), GoldTotal: 100}
	charID := uuid.New()
	store := &mockStore{
		getLootPoolFn: func(_ context.Context, _ uuid.UUID) (refdata.LootPool, error) {
			return pool, nil
		},
		listPlayerCharactersByCampaignApprovedFn: func(_ context.Context, _ uuid.UUID) ([]refdata.ListPlayerCharactersByCampaignApprovedRow, error) {
			return []refdata.ListPlayerCharactersByCampaignApprovedRow{
				{CharacterID: charID, CharacterName: "Aria", Gold: 10},
			}, nil
		},
		updateCharacterGoldFn: func(_ context.Context, _ refdata.UpdateCharacterGoldParams) (refdata.Character, error) {
			return refdata.Character{}, errDB
		},
	}

	svc := loot.NewService(store)
	_, err := svc.SplitGold(context.Background(), pool.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating gold for Aria")
}

func TestSplitGold_ZeroPoolGoldFailure(t *testing.T) {
	pool := refdata.LootPool{ID: uuid.New(), Status: "open", CampaignID: uuid.New(), GoldTotal: 100}
	charID := uuid.New()
	store := &mockStore{
		getLootPoolFn: func(_ context.Context, _ uuid.UUID) (refdata.LootPool, error) {
			return pool, nil
		},
		listPlayerCharactersByCampaignApprovedFn: func(_ context.Context, _ uuid.UUID) ([]refdata.ListPlayerCharactersByCampaignApprovedRow, error) {
			return []refdata.ListPlayerCharactersByCampaignApprovedRow{
				{CharacterID: charID, CharacterName: "Aria", Gold: 10},
			}, nil
		},
		updateCharacterGoldFn: func(_ context.Context, _ refdata.UpdateCharacterGoldParams) (refdata.Character, error) {
			return refdata.Character{}, nil
		},
		updateLootPoolGoldFn: func(_ context.Context, _ refdata.UpdateLootPoolGoldParams) (refdata.LootPool, error) {
			return refdata.LootPool{}, errDB
		},
	}

	svc := loot.NewService(store)
	_, err := svc.SplitGold(context.Background(), pool.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating pool gold remainder")
}

// --- CreateLootPool: combatant listing failure ---

func TestCreateLootPool_ListCombatantsFailure(t *testing.T) {
	encID := uuid.New()
	store := &mockStore{
		getEncounterFn: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encID, Status: "completed"}, nil
		},
		getLootPoolByEncounterFn: func(_ context.Context, _ uuid.UUID) (refdata.LootPool, error) {
			return refdata.LootPool{}, sql.ErrNoRows
		},
		listCombatantsByEncounterIDFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return nil, errDB
		},
	}

	svc := loot.NewService(store)
	_, err := svc.CreateLootPool(context.Background(), encID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "listing combatants")
}

// --- CreateLootPool: CreateLootPool DB failure ---

func TestCreateLootPool_DBCreatePoolFailure(t *testing.T) {
	encID := uuid.New()
	store := &mockStore{
		getEncounterFn: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encID, Status: "completed"}, nil
		},
		getLootPoolByEncounterFn: func(_ context.Context, _ uuid.UUID) (refdata.LootPool, error) {
			return refdata.LootPool{}, sql.ErrNoRows
		},
		listCombatantsByEncounterIDFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return nil, nil
		},
		createLootPoolFn: func(_ context.Context, _ refdata.CreateLootPoolParams) (refdata.LootPool, error) {
			return refdata.LootPool{}, errDB
		},
	}

	svc := loot.NewService(store)
	_, err := svc.CreateLootPool(context.Background(), encID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating loot pool")
}

// --- CreateLootPool: NPC with no character_id, alive NPC, non-NPC are skipped ---

func TestCreateLootPool_SkipsAliveAndNonNPCs(t *testing.T) {
	encID := uuid.New()
	campID := uuid.New()

	store := &mockStore{
		getEncounterFn: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encID, CampaignID: campID, Status: "completed"}, nil
		},
		getLootPoolByEncounterFn: func(_ context.Context, _ uuid.UUID) (refdata.LootPool, error) {
			return refdata.LootPool{}, sql.ErrNoRows
		},
		listCombatantsByEncounterIDFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{IsNpc: false, IsAlive: false}, // non-NPC, skip
				{IsNpc: true, IsAlive: true},   // alive NPC, skip
				{IsNpc: true, IsAlive: false, CharacterID: uuid.NullUUID{Valid: false}}, // no char ID, skip
			}, nil
		},
		createLootPoolFn: func(_ context.Context, _ refdata.CreateLootPoolParams) (refdata.LootPool, error) {
			return refdata.LootPool{ID: uuid.New(), EncounterID: encID, CampaignID: campID, Status: "open"}, nil
		},
	}

	svc := loot.NewService(store)
	result, err := svc.CreateLootPool(context.Background(), encID)
	assert.NoError(t, err)
	assert.Empty(t, result.Items)
}

// --- CreateLootPool: NPC character with invalid inventory JSON is skipped ---

func TestCreateLootPool_SkipsInvalidInventory(t *testing.T) {
	encID := uuid.New()
	campID := uuid.New()
	charID := uuid.New()

	store := &mockStore{
		getEncounterFn: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encID, CampaignID: campID, Status: "completed"}, nil
		},
		getLootPoolByEncounterFn: func(_ context.Context, _ uuid.UUID) (refdata.LootPool, error) {
			return refdata.LootPool{}, sql.ErrNoRows
		},
		listCombatantsByEncounterIDFn: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{IsNpc: true, IsAlive: false, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
			}, nil
		},
		getCharacterFn: func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
			return refdata.Character{
				ID:        charID,
				Gold:      5,
				Inventory: pqtype.NullRawMessage{RawMessage: []byte(`not valid json`), Valid: true},
			}, nil
		},
		createLootPoolFn: func(_ context.Context, _ refdata.CreateLootPoolParams) (refdata.LootPool, error) {
			return refdata.LootPool{ID: uuid.New(), EncounterID: encID, CampaignID: campID, GoldTotal: 5, Status: "open"}, nil
		},
		updateCharacterGoldFn: func(_ context.Context, _ refdata.UpdateCharacterGoldParams) (refdata.Character, error) {
			return refdata.Character{}, nil
		},
		updateCharacterInventoryFn: func(_ context.Context, _ refdata.UpdateCharacterInventoryParams) (refdata.Character, error) {
			return refdata.Character{}, nil
		},
	}

	svc := loot.NewService(store)
	result, err := svc.CreateLootPool(context.Background(), encID)
	assert.NoError(t, err)
	assert.Equal(t, int32(5), result.Pool.GoldTotal) // gold still collected
	assert.Empty(t, result.Items)                    // but no items because JSON was bad
}

// --- UpdateItem ---

func TestUpdateItem_PoolNotFound(t *testing.T) {
	store := &mockStore{
		getLootPoolFn: func(_ context.Context, _ uuid.UUID) (refdata.LootPool, error) {
			return refdata.LootPool{}, sql.ErrNoRows
		},
	}
	svc := loot.NewService(store)
	_, err := svc.UpdateItem(context.Background(), uuid.New(), uuid.New(), refdata.UpdateLootPoolItemParams{})
	assert.ErrorIs(t, err, loot.ErrPoolNotFound)
}

func TestUpdateItem_PoolClosed(t *testing.T) {
	store := &mockStore{
		getLootPoolFn: func(_ context.Context, _ uuid.UUID) (refdata.LootPool, error) {
			return refdata.LootPool{Status: "closed"}, nil
		},
	}
	svc := loot.NewService(store)
	_, err := svc.UpdateItem(context.Background(), uuid.New(), uuid.New(), refdata.UpdateLootPoolItemParams{})
	assert.ErrorIs(t, err, loot.ErrPoolClosed)
}

func TestUpdateItem_Success(t *testing.T) {
	itemID := uuid.New()
	store := &mockStore{
		getLootPoolFn: func(_ context.Context, _ uuid.UUID) (refdata.LootPool, error) {
			return refdata.LootPool{Status: "open"}, nil
		},
	}
	svc := loot.NewService(store)
	item, err := svc.UpdateItem(context.Background(), uuid.New(), itemID, refdata.UpdateLootPoolItemParams{
		Name: sql.NullString{String: "Updated Name", Valid: true},
	})
	assert.NoError(t, err)
	assert.Equal(t, itemID, item.ID)
}
