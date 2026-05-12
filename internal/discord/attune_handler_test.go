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
	"github.com/ab/dndnd/internal/refdata"
)

// mockAttuneStore captures attunement slot updates.
type mockAttuneStore struct {
	updatedAttunement json.RawMessage
	char              refdata.Character
	err               error
}

// stubAttunePublisher records PublishForCharacter calls.
type stubAttunePublisher struct {
	calls []uuid.UUID
}

func (s *stubAttunePublisher) PublishForCharacter(_ context.Context, characterID uuid.UUID) {
	s.calls = append(s.calls, characterID)
}

func (m *mockAttuneStore) UpdateCharacterAttunementSlots(ctx context.Context, arg refdata.UpdateCharacterAttunementSlotsParams) (refdata.Character, error) {
	if m.err != nil {
		return refdata.Character{}, m.err
	}
	m.updatedAttunement = arg.AttunementSlots.RawMessage
	return m.char, nil
}

func makeAttuneInteraction(guildID, userID, itemID string) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: guildID,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: userID},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "attune",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "item", Type: discordgo.ApplicationCommandOptionString, Value: itemID},
			},
		},
	}
}

func TestAttuneHandler_Success(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
	}
	itemsJSON, _ := json.Marshal(items)

	slots := []character.AttunementSlot{}
	slotsJSON, _ := json.Marshal(slots)

	store := &mockAttuneStore{char: refdata.Character{ID: charID}}

	handler := NewAttuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		store,
	)

	interaction := makeAttuneInteraction("guild1", "user1", "cloak-of-protection")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Attuned")
	assert.Contains(t, sess.lastResponse, "Cloak of Protection")

	// Verify store was called with updated attunement
	var updatedSlots []character.AttunementSlot
	require.NoError(t, json.Unmarshal(store.updatedAttunement, &updatedSlots))
	assert.Len(t, updatedSlots, 1)
	assert.Equal(t, "cloak-of-protection", updatedSlots[0].ItemID)
}

func TestAttuneHandler_MaxSlots(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "item-4", Name: "Fourth Item", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
	}
	itemsJSON, _ := json.Marshal(items)

	slots := []character.AttunementSlot{
		{ItemID: "item-1", Name: "Item 1"},
		{ItemID: "item-2", Name: "Item 2"},
		{ItemID: "item-3", Name: "Item 3"},
	}
	slotsJSON, _ := json.Marshal(slots)

	handler := NewAttuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              uuid.New(),
			CampaignID:      campID,
			Name:            "Aria",
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		&mockAttuneStore{},
	)

	interaction := makeAttuneInteraction("guild1", "user1", "item-4")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "3 attuned items")
}

func TestAttuneHandler_NoCampaign(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewAttuneHandler(sess,
		&mockInventoryCampaignProvider{err: assert.AnError},
		&mockInventoryCharacterLookup{},
		&mockAttuneStore{},
	)

	interaction := makeAttuneInteraction("guild1", "user1", "cloak-of-protection")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "No campaign")
}

func TestAttuneHandler_NoCharacter(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	handler := NewAttuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{err: assert.AnError},
		&mockAttuneStore{},
	)

	interaction := makeAttuneInteraction("guild1", "user1", "cloak-of-protection")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Could not find your character")
}

func TestAttuneHandler_EmptyItemID(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewAttuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockInventoryCharacterLookup{char: refdata.Character{}},
		&mockAttuneStore{},
	)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "attune",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
	}
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "specify an item")
}

func TestAttuneHandler_BadInventoryJSON(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	handler := NewAttuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         uuid.New(),
			CampaignID: campID,
			Name:       "Aria",
			Inventory:  pqtype.NullRawMessage{RawMessage: []byte("{bad json"), Valid: true},
		}},
		&mockAttuneStore{},
	)

	interaction := makeAttuneInteraction("guild1", "user1", "cloak-of-protection")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed to read inventory")
}

func TestAttuneHandler_BadAttunementJSON(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	itemsJSON, _ := json.Marshal([]character.InventoryItem{})

	handler := NewAttuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              uuid.New(),
			CampaignID:      campID,
			Name:            "Aria",
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: []byte("{bad json"), Valid: true},
		}},
		&mockAttuneStore{},
	)

	interaction := makeAttuneInteraction("guild1", "user1", "cloak-of-protection")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed to read attunement")
}

func TestAttuneHandler_StoreError(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
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
		&mockAttuneStore{err: assert.AnError},
	)

	interaction := makeAttuneInteraction("guild1", "user1", "cloak-of-protection")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed to save")
}

func TestAttuneHandler_ClassRestrictionDenied(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "holy-avenger", Name: "Holy Avenger", Quantity: 1, Type: "weapon", IsMagic: true, RequiresAttunement: true, AttunementRestriction: "paladin"},
	}
	itemsJSON, _ := json.Marshal(items)

	slots := []character.AttunementSlot{}
	slotsJSON, _ := json.Marshal(slots)

	classes, _ := json.Marshal([]character.ClassEntry{{Class: "fighter", Level: 5}})

	store := &mockAttuneStore{char: refdata.Character{ID: charID}}

	handler := NewAttuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			Classes:         classes,
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		store,
	)

	interaction := makeAttuneInteraction("guild1", "user1", "holy-avenger")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "restriction")
}

// F-9: a successful attune must fan out a dashboard encounter snapshot for
// the character via the injected publisher.
func TestAttuneHandler_Success_PublishesSnapshot(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
	}
	itemsJSON, _ := json.Marshal(items)
	slotsJSON, _ := json.Marshal([]character.AttunementSlot{})

	store := &mockAttuneStore{char: refdata.Character{ID: charID}}
	publisher := &stubAttunePublisher{}

	handler := NewAttuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		store,
	)
	handler.SetPublisher(publisher)

	interaction := makeAttuneInteraction("guild1", "user1", "cloak-of-protection")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Attuned")
	require.Len(t, publisher.calls, 1)
	assert.Equal(t, charID, publisher.calls[0])
}

// F-9: when persistence fails the publisher must NOT be invoked — we must
// never broadcast a snapshot that doesn't reflect committed state.
func TestAttuneHandler_StoreError_DoesNotPublish(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
	}
	itemsJSON, _ := json.Marshal(items)
	slotsJSON, _ := json.Marshal([]character.AttunementSlot{})

	publisher := &stubAttunePublisher{}

	handler := NewAttuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		&mockAttuneStore{err: assert.AnError},
	)
	handler.SetPublisher(publisher)

	interaction := makeAttuneInteraction("guild1", "user1", "cloak-of-protection")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed to save")
	assert.Empty(t, publisher.calls)
}

func TestAttuneHandler_ClassRestrictionAllowed(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "holy-avenger", Name: "Holy Avenger", Quantity: 1, Type: "weapon", IsMagic: true, RequiresAttunement: true, AttunementRestriction: "paladin"},
	}
	itemsJSON, _ := json.Marshal(items)

	slots := []character.AttunementSlot{}
	slotsJSON, _ := json.Marshal(slots)

	classes, _ := json.Marshal([]character.ClassEntry{{Class: "paladin", Level: 5}})

	store := &mockAttuneStore{char: refdata.Character{ID: charID}}

	handler := NewAttuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			Classes:         classes,
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		store,
	)

	interaction := makeAttuneInteraction("guild1", "user1", "holy-avenger")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Attuned")
	assert.Contains(t, sess.lastResponse, "Holy Avenger")
}
