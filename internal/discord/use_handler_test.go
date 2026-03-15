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

// mockUseCharacterStore captures inventory+HP updates.
type mockUseCharacterStore struct {
	updatedInventory json.RawMessage
	updatedHPCurrent int32
	char             refdata.Character
}

func (m *mockUseCharacterStore) UpdateCharacterInventoryAndHP(ctx context.Context, arg refdata.UpdateCharacterInventoryAndHPParams) (refdata.Character, error) {
	m.updatedInventory = arg.Inventory.RawMessage
	m.updatedHPCurrent = arg.HpCurrent
	return m.char, nil
}

func (m *mockUseCharacterStore) UpdateCharacterInventory(ctx context.Context, arg refdata.UpdateCharacterInventoryParams) (refdata.Character, error) {
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
