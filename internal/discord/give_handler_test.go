package discord

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

var _ GiveCharacterStore = (*mockGiveCharacterStore)(nil)

// mockGiveCharacterStore captures updates for both giver and receiver.
type mockGiveCharacterStore struct {
	updates map[uuid.UUID]json.RawMessage
	err     error // return error on update
}

func (m *mockGiveCharacterStore) UpdateTwoCharacterInventories(ctx context.Context, id1 uuid.UUID, inv1 pqtype.NullRawMessage, id2 uuid.UUID, inv2 pqtype.NullRawMessage) error {
	if m.err != nil {
		return m.err
	}
	if m.updates == nil {
		m.updates = make(map[uuid.UUID]json.RawMessage)
	}
	m.updates[id1] = inv1.RawMessage
	m.updates[id2] = inv2.RawMessage
	return nil
}

// mockGiveTargetResolver resolves a name/ID to a character.
type mockGiveTargetResolver struct {
	chars map[string]refdata.Character
}

func (m *mockGiveTargetResolver) ResolveTarget(ctx context.Context, campaignID uuid.UUID, nameOrID string) (refdata.Character, error) {
	char, ok := m.chars[nameOrID]
	if !ok {
		return refdata.Character{}, assert.AnError
	}
	return char, nil
}

// mockGiveCombatProvider returns combatant positions for adjacency check.
type mockGiveCombatProvider struct {
	combatants []refdata.Combatant
	inCombat   bool
}

func (m *mockGiveCombatProvider) GetCombatantsForGuild(ctx context.Context, guildID string) ([]refdata.Combatant, bool, error) {
	return m.combatants, m.inCombat, nil
}

func makeGiveInteraction(guildID, userID, itemID, target string) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: guildID,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: userID},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "give",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "item", Type: discordgo.ApplicationCommandOptionString, Value: itemID},
				{Name: "target", Type: discordgo.ApplicationCommandOptionString, Value: target},
			},
		},
	}
}

func TestGiveHandler_Success(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()

	giverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	giverItemsJSON, _ := json.Marshal(giverItems)

	receiverItems := []character.InventoryItem{}
	receiverItemsJSON, _ := json.Marshal(receiverItems)

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
		nil, // no combat
	)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "GK")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Aria")
	assert.Contains(t, sess.lastResponse, "Gorak")
	assert.Contains(t, sess.lastResponse, "Healing Potion")

	// Both inventories should be updated
	assert.Len(t, store.updates, 2)
}

func TestGiveHandler_ItemNotFound(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	handler := NewGiveHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         uuid.New(),
			CampaignID: campID,
			Name:       "Aria",
		}},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{
			"GK": {ID: uuid.New(), Name: "Gorak"},
		}},
		&mockGiveCharacterStore{},
		nil,
	)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "GK")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "not found")
}

func TestGiveHandler_TargetNotFound(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	giverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	giverItemsJSON, _ := json.Marshal(giverItems)

	handler := NewGiveHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         uuid.New(),
			CampaignID: campID,
			Name:       "Aria",
			Inventory:  pqtype.NullRawMessage{RawMessage: giverItemsJSON, Valid: true},
		}},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{}},
		&mockGiveCharacterStore{},
		nil,
	)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "NOBODY")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "target")
}

func TestGiveHandler_NoCampaign(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewGiveHandler(sess,
		&mockInventoryCampaignProvider{err: assert.AnError},
		&mockInventoryCharacterLookup{},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{}},
		&mockGiveCharacterStore{},
		nil,
	)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "GK")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "No campaign")
}

func TestGiveHandler_NoCharacter(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	handler := NewGiveHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{err: assert.AnError},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{}},
		&mockGiveCharacterStore{},
		nil,
	)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "GK")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Could not find your character")
}

func TestGiveHandler_PersistError(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()

	giverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	giverItemsJSON, _ := json.Marshal(giverItems)

	store := &mockGiveCharacterStore{err: assert.AnError}

	handler := NewGiveHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID: giverID, CampaignID: campID, Name: "Aria",
			Inventory: pqtype.NullRawMessage{RawMessage: giverItemsJSON, Valid: true},
		}},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{
			"GK": {ID: receiverID, CampaignID: campID, Name: "Gorak"},
		}},
		store,
		nil,
	)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "GK")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed to save")
}

// mockGiveTurnProvider implements GiveTurnProvider for med-35 tests.
type mockGiveTurnProvider struct {
	turn      refdata.Turn
	inCombat  bool
	lookupErr error
	updates   []refdata.UpdateTurnActionsParams
}

func (m *mockGiveTurnProvider) GetActiveTurnForCharacter(_ context.Context, _ string, _ uuid.UUID) (refdata.Turn, bool, error) {
	if m.lookupErr != nil {
		return refdata.Turn{}, false, m.lookupErr
	}
	return m.turn, m.inCombat, nil
}

func (m *mockGiveTurnProvider) UpdateTurnActions(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
	m.updates = append(m.updates, arg)
	return refdata.Turn{ID: arg.ID, FreeInteractUsed: arg.FreeInteractUsed}, nil
}

func TestGiveHandler_InCombat_DeductsFreeInteraction(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()
	turnID := uuid.New()

	giverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	giverItemsJSON, _ := json.Marshal(giverItems)
	receiverItemsJSON, _ := json.Marshal([]character.InventoryItem{})

	store := &mockGiveCharacterStore{}
	turnProv := &mockGiveTurnProvider{turn: refdata.Turn{ID: turnID, FreeInteractUsed: false}, inCombat: true}

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
	handler.SetTurnProvider(turnProv)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "GK")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Healing Potion")
	if assert.Len(t, turnProv.updates, 1, "expected one turn update") {
		assert.True(t, turnProv.updates[0].FreeInteractUsed, "/give in combat should consume the free interaction")
	}
	assert.Len(t, store.updates, 2, "both inventories should still update")
}

func TestGiveHandler_InCombat_FreeInteractionAlreadySpent_Rejected(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	giverID := uuid.New()
	receiverID := uuid.New()
	turnID := uuid.New()

	giverItems := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	giverItemsJSON, _ := json.Marshal(giverItems)

	store := &mockGiveCharacterStore{}
	turnProv := &mockGiveTurnProvider{turn: refdata.Turn{ID: turnID, FreeInteractUsed: true}, inCombat: true}

	handler := NewGiveHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         giverID,
			CampaignID: campID,
			Name:       "Aria",
			Inventory:  pqtype.NullRawMessage{RawMessage: giverItemsJSON, Valid: true},
		}},
		&mockGiveTargetResolver{chars: map[string]refdata.Character{
			"GK": {ID: receiverID, CampaignID: campID, Name: "Gorak"},
		}},
		store,
		nil,
	)
	handler.SetTurnProvider(turnProv)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "GK")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Cannot give item")
	assert.Empty(t, store.updates, "rejection must not transfer items")
	assert.Empty(t, turnProv.updates, "rejection must not persist a turn update")
}

func TestGiveHandler_OutOfCombat_NoCost(t *testing.T) {
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
	turnProv := &mockGiveTurnProvider{inCombat: false}

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
	handler.SetTurnProvider(turnProv)

	interaction := makeGiveInteraction("guild1", "user1", "healing-potion", "GK")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Healing Potion")
	assert.Empty(t, turnProv.updates, "out-of-combat /give must not deduct any resource")
	assert.Len(t, store.updates, 2)
}
