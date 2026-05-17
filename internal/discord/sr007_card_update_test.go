package discord

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/loot"
	"github.com/ab/dndnd/internal/refdata"
)

// SR-007: per-handler subscriber tests. Each handler's success path must
// fire OnCharacterUpdated exactly once (or twice for /give — once per
// character). Tests use the recordingCardUpdater fake.

// --- /use ---

func TestUseHandler_FiresCardUpdater_OnSuccess(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	itemsJSON, _ := json.Marshal(items)
	store := &mockUseCharacterStore{char: refdata.Character{ID: charID}}

	handler := NewUseHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         charID,
			CampaignID: campID,
			Name:       "Aria",
			HpCurrent:  10,
			HpMax:      30,
			Inventory:  pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
		}},
		store,
		func(max int) int { return 3 },
		nil,
	)
	rec := &recordingCardUpdater{}
	handler.SetCardUpdater(rec)

	handler.Handle(makeUseInteraction("guild1", "user1", "healing-potion"))

	require.Len(t, rec.calls, 1, "/use success must fire OnCharacterUpdated exactly once")
	assert.Equal(t, charID, rec.calls[0])
}

// --- /give ---

func TestGiveHandler_FiresCardUpdater_OnSuccess(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()

	giverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	giverItemsJSON, _ := json.Marshal(giverItems)
	receiverItemsJSON, _ := json.Marshal([]character.InventoryItem{})

	store := &mockGiveCharacterStore{}
	handler := NewGiveHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         giverID,
			CampaignID: campID,
			Name:       "Aria",
			Inventory:  pqtype.NullRawMessage{RawMessage: giverItemsJSON, Valid: true},
		}},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{
			"GK": {
				ID:         receiverID,
				CampaignID: campID,
				Name:       "Gorak",
				Inventory:  pqtype.NullRawMessage{RawMessage: receiverItemsJSON, Valid: true},
			},
		}},
		store,
		nil,
	)
	rec := &recordingCardUpdater{}
	handler.SetCardUpdater(rec)

	handler.Handle(makeGiveInteraction("guild1", "user1", "healing-potion", "GK"))

	require.Len(t, rec.calls, 2, "/give success must fire OnCharacterUpdated for both parties")
	got := map[uuid.UUID]bool{rec.calls[0]: true, rec.calls[1]: true}
	assert.True(t, got[giverID], "expected giver in card-update calls")
	assert.True(t, got[receiverID], "expected receiver in card-update calls")
}

// --- /attune ---

func TestAttuneHandler_FiresCardUpdater_OnSuccess(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "cloak", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
	}
	itemsJSON, _ := json.Marshal(items)
	slotsJSON, _ := json.Marshal([]character.AttunementSlot{})

	handler := NewAttuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		&mockAttuneStore{},
	)
	rec := &recordingCardUpdater{}
	handler.SetCardUpdater(rec)

	handler.Handle(makeAttuneInteraction("guild1", "user1", "cloak"))

	require.Len(t, rec.calls, 1, "/attune success must fire OnCharacterUpdated exactly once")
	assert.Equal(t, charID, rec.calls[0])
}

// --- /loot HandleLootClaim ---

// minimalLootStore implements the loot.Store methods exercised by ClaimItem
// (GetLootPool, ClaimLootPoolItem, GetCharacter, UpdateCharacterInventory).
// Returns zero values for any unused method so the test stays focused on the
// success path.
type minimalLootStore struct{}

func (m *minimalLootStore) GetEncounter(context.Context, uuid.UUID) (refdata.Encounter, error) {
	return refdata.Encounter{}, nil
}
func (m *minimalLootStore) ListEncountersByCampaignID(context.Context, uuid.UUID) ([]refdata.Encounter, error) {
	return nil, nil
}
func (m *minimalLootStore) ListCombatantsByEncounterID(context.Context, uuid.UUID) ([]refdata.Combatant, error) {
	return nil, nil
}
func (m *minimalLootStore) GetCharacter(context.Context, uuid.UUID) (refdata.Character, error) {
	return refdata.Character{}, nil
}
func (m *minimalLootStore) CreateLootPool(context.Context, refdata.CreateLootPoolParams) (refdata.LootPool, error) {
	return refdata.LootPool{}, nil
}
func (m *minimalLootStore) CreateLootPoolItem(context.Context, refdata.CreateLootPoolItemParams) (refdata.LootPoolItem, error) {
	return refdata.LootPoolItem{}, nil
}
func (m *minimalLootStore) GetLootPoolByEncounter(context.Context, uuid.UUID) (refdata.LootPool, error) {
	return refdata.LootPool{}, nil
}
func (m *minimalLootStore) GetLootPool(context.Context, uuid.UUID) (refdata.LootPool, error) {
	return refdata.LootPool{Status: "open"}, nil
}
func (m *minimalLootStore) ListLootPoolItems(context.Context, uuid.UUID) ([]refdata.LootPoolItem, error) {
	return nil, nil
}
func (m *minimalLootStore) ClaimLootPoolItem(_ context.Context, arg refdata.ClaimLootPoolItemParams) (refdata.LootPoolItem, error) {
	return refdata.LootPoolItem{ID: arg.ID, Name: "Test Sword", Quantity: 1, Type: "weapon"}, nil
}
func (m *minimalLootStore) UpdateLootPoolGold(context.Context, refdata.UpdateLootPoolGoldParams) (refdata.LootPool, error) {
	return refdata.LootPool{}, nil
}
func (m *minimalLootStore) UpdateLootPoolStatus(context.Context, refdata.UpdateLootPoolStatusParams) (refdata.LootPool, error) {
	return refdata.LootPool{}, nil
}
func (m *minimalLootStore) DeleteLootPoolItem(context.Context, uuid.UUID) error { return nil }
func (m *minimalLootStore) DeleteUnclaimedLootPoolItems(context.Context, uuid.UUID) error {
	return nil
}
func (m *minimalLootStore) DeleteLootPool(context.Context, uuid.UUID) error { return nil }
func (m *minimalLootStore) UpdateLootPoolItem(_ context.Context, arg refdata.UpdateLootPoolItemParams) (refdata.LootPoolItem, error) {
	return refdata.LootPoolItem{ID: arg.ID}, nil
}
func (m *minimalLootStore) ListPlayerCharactersByCampaignApproved(context.Context, uuid.UUID) ([]refdata.ListPlayerCharactersByCampaignApprovedRow, error) {
	return nil, nil
}
func (m *minimalLootStore) UpdateCharacterGold(context.Context, refdata.UpdateCharacterGoldParams) (refdata.Character, error) {
	return refdata.Character{}, nil
}
func (m *minimalLootStore) UpdateCharacterInventory(context.Context, refdata.UpdateCharacterInventoryParams) (refdata.Character, error) {
	return refdata.Character{}, nil
}

func TestLootHandler_FiresCardUpdater_OnClaim(t *testing.T) {
	sess := &mockInventorySession{}
	charID := uuid.New()
	poolID := uuid.New()
	itemID := uuid.New()

	lootSvc := loot.NewService(&minimalLootStore{})
	handler := NewLootHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockInventoryCharacterLookup{char: refdata.Character{ID: charID}},
		&mockLootEncounterProvider{},
		lootSvc,
	)
	rec := &recordingCardUpdater{}
	handler.SetCardUpdater(rec)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}
	handler.HandleLootClaim(interaction, poolID, itemID, charID)

	require.Len(t, rec.calls, 1, "/loot claim success must fire OnCharacterUpdated exactly once")
	assert.Equal(t, charID, rec.calls[0])
}

// --- /rest long ---

func TestRestHandler_FiresCardUpdater_OnLongRest(t *testing.T) {
	sess := newTestMock()
	sess.InteractionRespondFunc = func(*discordgo.Interaction, *discordgo.InteractionResponse) error {
		return nil
	}

	campaignID := uuid.New()
	char := makeRestTestCharacter()
	char.CampaignID = campaignID
	roller := dice.NewRoller(func(max int) int { return 6 })

	h := NewRestHandler(
		sess,
		roller,
		&mockCheckCampaignProvider{fn: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campaignID}, nil
		}},
		&mockCheckCharacterLookup{fn: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return char, nil
		}},
		&mockCheckEncounterProvider{fn: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.Nil, errNoEncounter
		}},
		&mockRestCharacterUpdater{
			updateCharacterFn: func(_ context.Context, arg refdata.UpdateCharacterParams) (refdata.Character, error) {
				return refdata.Character{ID: arg.ID}, nil
			},
		},
		nil,
		nil,
	)
	rec := &recordingCardUpdater{}
	h.SetCardUpdater(rec)

	h.Handle(makeRestInteraction("long"))

	require.Len(t, rec.calls, 1, "/rest long success must fire OnCharacterUpdated exactly once")
	assert.Equal(t, char.ID, rec.calls[0])
}

// --- /prepare ---

func TestPrepareHandler_FiresCardUpdater_OnCommit(t *testing.T) {
	h, _, _, _, charLookup := setupPrepareHandler()
	rec := &recordingCardUpdater{}
	h.SetCardUpdater(rec)

	h.Handle(makePrepareInteraction(map[string]string{
		"spells": "bless, cure-wounds",
	}))

	require.Len(t, rec.calls, 1, "/prepare commit success must fire OnCharacterUpdated exactly once")
	assert.Equal(t, charLookup.char.ID, rec.calls[0])
}

// --- /unattune ---

func TestUnattuneHandler_FiresCardUpdater_OnSuccess(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	slots := []character.AttunementSlot{
		{ItemID: "cloak", Name: "Cloak of Protection"},
	}
	slotsJSON, _ := json.Marshal(slots)

	handler := NewUnattuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		&mockAttuneStore{},
	)
	rec := &recordingCardUpdater{}
	handler.SetCardUpdater(rec)

	handler.Handle(makeUnattuneInteraction("guild1", "user1", "cloak"))

	require.Len(t, rec.calls, 1, "/unattune success must fire OnCharacterUpdated exactly once")
	assert.Equal(t, charID, rec.calls[0])
}
