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

// mockEquipStore captures inventory updates for equip tests.
type mockEquipStore struct {
	updatedInventory json.RawMessage
	char             refdata.Character
	err              error
}

func (m *mockEquipStore) UpdateCharacterInventory(ctx context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error) {
	if m.err != nil {
		return refdata.Character{}, m.err
	}
	m.updatedInventory = arg.Inventory.RawMessage
	return m.char, nil
}

func makeEquipInteraction(guildID, userID, itemID string, offhand, armor bool) *discordgo.Interaction {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "item", Type: discordgo.ApplicationCommandOptionString, Value: itemID},
	}
	if offhand {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name: "offhand", Type: discordgo.ApplicationCommandOptionBoolean, Value: true,
		})
	}
	if armor {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name: "armor", Type: discordgo.ApplicationCommandOptionBoolean, Value: true,
		})
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: guildID,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: userID},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "equip",
			Options: opts,
		},
	}
}

func TestEquipHandler_Success(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}
	itemsJSON, _ := json.Marshal(items)
	slotsJSON, _ := json.Marshal([]character.AttunementSlot{})

	store := &mockEquipStore{char: refdata.Character{ID: charID}}

	handler := NewEquipHandler(sess,
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

	interaction := makeEquipInteraction("guild1", "user1", "longsword", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Longsword")
	assert.Contains(t, sess.lastResponse, "Equipped")

	var updatedItems []character.InventoryItem
	require.NoError(t, json.Unmarshal(store.updatedInventory, &updatedItems))
	assert.True(t, updatedItems[0].Equipped)
	assert.Equal(t, "main_hand", updatedItems[0].EquipSlot)
}

func TestEquipHandler_UnattunedWarning(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
	}
	itemsJSON, _ := json.Marshal(items)
	slotsJSON, _ := json.Marshal([]character.AttunementSlot{})

	store := &mockEquipStore{char: refdata.Character{ID: charID}}

	handler := NewEquipHandler(sess,
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

	interaction := makeEquipInteraction("guild1", "user1", "cloak-of-protection", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Equipped")
	assert.Contains(t, sess.lastResponse, "requires attunement")
	assert.Contains(t, sess.lastResponse, "/attune")
}

func TestEquipHandler_AttunedNoWarning(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection", Quantity: 1, Type: "magic_item", IsMagic: true, RequiresAttunement: true},
	}
	itemsJSON, _ := json.Marshal(items)
	slotsJSON, _ := json.Marshal([]character.AttunementSlot{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection"},
	})

	store := &mockEquipStore{char: refdata.Character{ID: charID}}

	handler := NewEquipHandler(sess,
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

	interaction := makeEquipInteraction("guild1", "user1", "cloak-of-protection", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Equipped")
	assert.NotContains(t, sess.lastResponse, "requires attunement")
}

func TestEquipHandler_NoCampaign(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{err: assert.AnError},
		&mockInventoryCharacterLookup{},
		&mockEquipStore{},
	)

	interaction := makeEquipInteraction("guild1", "user1", "longsword", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "No campaign")
}

func TestEquipHandler_NoCharacter(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{err: assert.AnError},
		&mockEquipStore{},
	)

	interaction := makeEquipInteraction("guild1", "user1", "longsword", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Could not find your character")
}

func TestEquipHandler_EmptyItemID(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{},
		&mockInventoryCharacterLookup{},
		&mockEquipStore{},
	)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "equip",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
	}
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "specify an item")
}

func TestEquipHandler_BadInventoryJSON(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:         uuid.New(),
			CampaignID: campID,
			Name:       "Aria",
			Inventory:  pqtype.NullRawMessage{RawMessage: []byte("{bad json"), Valid: true},
		}},
		&mockEquipStore{},
	)

	interaction := makeEquipInteraction("guild1", "user1", "longsword", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed to read inventory")
}

func TestEquipHandler_BadAttunementJSON(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	itemsJSON, _ := json.Marshal([]character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	})

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              uuid.New(),
			CampaignID:      campID,
			Name:            "Aria",
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: []byte("{bad json"), Valid: true},
		}},
		&mockEquipStore{},
	)

	interaction := makeEquipInteraction("guild1", "user1", "longsword", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed to read attunement")
}

func TestEquipHandler_ItemNotFound(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	itemsJSON, _ := json.Marshal([]character.InventoryItem{})
	slotsJSON, _ := json.Marshal([]character.AttunementSlot{})

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		&mockEquipStore{},
	)

	interaction := makeEquipInteraction("guild1", "user1", "longsword", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "not found")
}

func TestEquipHandler_StoreError(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	items := []character.InventoryItem{
		{ItemID: "longsword", Name: "Longsword", Quantity: 1, Type: "weapon"},
	}
	itemsJSON, _ := json.Marshal(items)
	slotsJSON, _ := json.Marshal([]character.AttunementSlot{})

	handler := NewEquipHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			Inventory:       pqtype.NullRawMessage{RawMessage: itemsJSON, Valid: true},
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		&mockEquipStore{err: assert.AnError},
	)

	interaction := makeEquipInteraction("guild1", "user1", "longsword", false, false)
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed to save")
}
