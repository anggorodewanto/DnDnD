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
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// mockUseCharacterStore captures inventory+HP updates.
type mockUseCharacterStore struct {
	updatedInventory json.RawMessage
	updatedHPCurrent int32
	char             refdata.Character
	invAndHPErr      error
	invErr           error
}

func (m *mockUseCharacterStore) UpdateCharacterInventoryAndHP(ctx context.Context, arg refdata.UpdateCharacterInventoryAndHPParams) (refdata.Character, error) {
	if m.invAndHPErr != nil {
		return refdata.Character{}, m.invAndHPErr
	}
	m.updatedInventory = arg.Inventory.RawMessage
	m.updatedHPCurrent = arg.HpCurrent
	return m.char, nil
}

func (m *mockUseCharacterStore) UpdateCharacterInventory(ctx context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error) {
	if m.invErr != nil {
		return refdata.Character{}, m.invErr
	}
	m.updatedInventory = arg.Inventory.RawMessage
	return m.char, nil
}

func makeUseInteraction(guildID, userID, itemID string) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: guildID,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: userID},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "use",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "item", Type: discordgo.ApplicationCommandOptionString, Value: itemID},
			},
		},
	}
}

func TestUseHandler_HealingPotion(t *testing.T) {
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
		func(max int) int { return 3 }, // deterministic roller
		nil, // no combat provider
	)

	interaction := makeUseInteraction("guild1", "user1", "healing-potion")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Healing Potion")
	assert.Contains(t, sess.lastResponse, "healed")

	// Verify inventory was updated
	var updatedItems []character.InventoryItem
	_ = json.Unmarshal(store.updatedInventory, &updatedItems)
	assert.Equal(t, 1, updatedItems[0].Quantity)
	assert.Equal(t, int32(18), store.updatedHPCurrent) // 10 + 2d4+2 where d4=3 => 3+3+2=8
}

func TestUseHandler_ItemNotFound(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	handler := NewUseHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         uuid.New(),
			CampaignID: campID,
			Name:       "Aria",
		}},
		&mockUseCharacterStore{},
		nil,
		nil,
	)

	interaction := makeUseInteraction("guild1", "user1", "healing-potion")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "not found")
}

func TestUseHandler_EmptyItemID(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewUseHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockInventoryCharacterLookup{char: refdata.Character{}},
		&mockUseCharacterStore{},
		nil, nil,
	)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "use",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
	}
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "specify an item")
}

func TestUseHandler_NoCampaign(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewUseHandler(sess,
		&mockInventoryCampaignProvider{err: assert.AnError},
		&mockInventoryCharacterLookup{},
		&mockUseCharacterStore{},
		nil, nil,
	)

	interaction := makeUseInteraction("guild1", "user1", "healing-potion")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "No campaign")
}

func TestUseHandler_NoCharacter(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	handler := NewUseHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{err: assert.AnError},
		&mockUseCharacterStore{},
		nil, nil,
	)

	interaction := makeUseInteraction("guild1", "user1", "healing-potion")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Could not find your character")
}

func TestUseHandler_PersistInventoryAndHPError(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "healing-potion", Name: "Healing Potion", Quantity: 2, Type: "consumable"},
	}
	itemsJSON, _ := json.Marshal(items)

	store := &mockUseCharacterStore{
		char:        refdata.Character{ID: charID},
		invAndHPErr: assert.AnError,
	}

	handler := NewUseHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID: charID, CampaignID: campID, Name: "Aria",
			HpCurrent: 10, HpMax: 30,
			Inventory: pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
		}},
		store,
		func(max int) int { return 3 },
		nil,
	)

	interaction := makeUseInteraction("guild1", "user1", "healing-potion")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed to save")
}

func TestUseHandler_PersistInventoryError(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "antitoxin", Name: "Antitoxin", Quantity: 1, Type: "consumable"},
	}
	itemsJSON, _ := json.Marshal(items)

	store := &mockUseCharacterStore{
		char:   refdata.Character{ID: charID},
		invErr: assert.AnError,
	}

	handler := NewUseHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID: charID, CampaignID: campID, Name: "Aria",
			HpCurrent: 10, HpMax: 30,
			Inventory: pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
		}},
		store,
		nil,
		nil,
	)

	interaction := makeUseInteraction("guild1", "user1", "antitoxin")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed to save")
}

func TestUseHandler_DMQueuePost(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "ball-bearings", Name: "Ball Bearings", Quantity: 1, Type: "consumable"},
	}
	itemsJSON, _ := json.Marshal(items)

	store := &mockUseCharacterStore{}

	handler := NewUseHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID: uuid.New(), CampaignID: campID, Name: "Aria",
			Inventory: pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
		}},
		store,
		nil,
		nil,
	)
	handler.SetDMQueueFunc(func(guildID string) string { return "dm-queue-ch" })

	interaction := makeUseInteraction("guild1", "user1", "ball-bearings")
	handler.Handle(interaction)

	assert.Equal(t, "dm-queue-ch", sess.sentChannelID)
	assert.Contains(t, sess.sentChannelMsg, "Ball Bearings")
	assert.Contains(t, sess.sentChannelMsg, "Aria")
}

// recordingNotifier captures dmqueue.Event posts for test assertions.
type recordingNotifier struct {
	posted []dmqueue.Event
}

func (r *recordingNotifier) Post(_ context.Context, e dmqueue.Event) (string, error) {
	r.posted = append(r.posted, e)
	return "item-1", nil
}
func (r *recordingNotifier) Cancel(_ context.Context, _, _ string) error { return nil }
func (r *recordingNotifier) Resolve(_ context.Context, _, _ string) error { return nil }
func (r *recordingNotifier) Get(string) (dmqueue.Item, bool)              { return dmqueue.Item{}, false }
func (r *recordingNotifier) ListPending() []dmqueue.Item                  { return nil }

func TestUseHandler_PostsViaNotifier(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "ball-bearings", Name: "Ball Bearings", Quantity: 1, Type: "consumable"},
	}
	itemsJSON, _ := json.Marshal(items)

	handler := NewUseHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID: uuid.New(), CampaignID: campID, Name: "Aria",
			Inventory: pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
		}},
		&mockUseCharacterStore{},
		nil,
		nil,
	)

	rec := &recordingNotifier{}
	handler.SetNotifier(rec)
	// Even with a legacy DMQueueFunc set, the notifier path takes precedence.
	handler.SetDMQueueFunc(func(guildID string) string { return "legacy-ch" })

	interaction := makeUseInteraction("guild1", "user1", "ball-bearings")
	handler.Handle(interaction)

	require := assert.New(t)
	require.Len(rec.posted, 1, "expected one notifier post")
	ev := rec.posted[0]
	require.Equal(dmqueue.KindConsumable, ev.Kind)
	require.Equal("Aria", ev.PlayerName)
	require.Contains(ev.Summary, "Ball Bearings")
	require.Equal("guild1", ev.GuildID)
	// Legacy channel path must NOT be invoked when notifier is set.
	require.Empty(sess.sentChannelID, "legacy dmQueueFunc path should be bypassed")
}

func TestUseHandler_DMQueueItem(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "ball-bearings", Name: "Ball Bearings", Quantity: 1, Type: "consumable"},
	}
	itemsJSON, _ := json.Marshal(items)

	store := &mockUseCharacterStore{}

	handler := NewUseHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         uuid.New(),
			CampaignID: campID,
			Name:       "Aria",
			Inventory:  pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
		}},
		store,
		nil,
		nil,
	)

	interaction := makeUseInteraction("guild1", "user1", "ball-bearings")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "DM")
}
