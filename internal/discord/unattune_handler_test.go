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

func makeUnattuneInteraction(guildID, userID, itemID string) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: guildID,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: userID},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "unattune",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "item", Type: discordgo.ApplicationCommandOptionString, Value: itemID},
			},
		},
	}
}

func TestUnattuneHandler_Success(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	slots := []character.AttunementSlot{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection"},
		{ItemID: "ring-of-protection", Name: "Ring of Protection"},
	}
	slotsJSON, _ := json.Marshal(slots)

	store := &mockAttuneStore{char: refdata.Character{ID: charID}}

	handler := NewUnattuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		store,
	)

	interaction := makeUnattuneInteraction("guild1", "user1", "cloak-of-protection")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Unattuned")
	assert.Contains(t, sess.lastResponse, "Cloak of Protection")

	var updatedSlots []character.AttunementSlot
	require.NoError(t, json.Unmarshal(store.updatedAttunement, &updatedSlots))
	assert.Len(t, updatedSlots, 1)
	assert.Equal(t, "ring-of-protection", updatedSlots[0].ItemID)
}

func TestUnattuneHandler_NotAttuned(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	slotsJSON, _ := json.Marshal([]character.AttunementSlot{})

	handler := NewUnattuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              uuid.New(),
			CampaignID:      campID,
			Name:            "Aria",
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		&mockAttuneStore{},
	)

	interaction := makeUnattuneInteraction("guild1", "user1", "cloak-of-protection")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "not attuned")
}

func TestUnattuneHandler_NoCampaign(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewUnattuneHandler(sess,
		&mockInventoryCampaignProvider{err: assert.AnError},
		&mockInventoryCharacterLookup{},
		&mockAttuneStore{},
	)

	interaction := makeUnattuneInteraction("guild1", "user1", "cloak-of-protection")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "No campaign")
}

func TestUnattuneHandler_EmptyItemID(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewUnattuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: uuid.New()}},
		&mockInventoryCharacterLookup{char: refdata.Character{}},
		&mockAttuneStore{},
	)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "unattune",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
	}
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "specify an item")
}

func TestUnattuneHandler_NoCharacter(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	handler := NewUnattuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{err: assert.AnError},
		&mockAttuneStore{},
	)

	interaction := makeUnattuneInteraction("guild1", "user1", "cloak-of-protection")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Could not find your character")
}

func TestUnattuneHandler_BadAttunementJSON(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	handler := NewUnattuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              uuid.New(),
			CampaignID:      campID,
			Name:            "Aria",
			AttunementSlots: pqtype.NullRawMessage{RawMessage: []byte("{bad json"), Valid: true},
		}},
		&mockAttuneStore{},
	)

	interaction := makeUnattuneInteraction("guild1", "user1", "cloak-of-protection")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed to read attunement")
}

func TestUnattuneHandler_StoreError(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()

	slots := []character.AttunementSlot{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection"},
	}
	slotsJSON, _ := json.Marshal(slots)

	handler := NewUnattuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              uuid.New(),
			CampaignID:      campID,
			Name:            "Aria",
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		&mockAttuneStore{err: assert.AnError},
	)

	interaction := makeUnattuneInteraction("guild1", "user1", "cloak-of-protection")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Failed to save")
}

// --- Finding 10 test: /unattune publishes dashboard snapshot ---

// mockAttunePublisher records PublishForCharacter calls for unattune tests.
type mockAttunePublisher struct {
	publishedFor uuid.UUID
}

func (m *mockAttunePublisher) PublishForCharacter(_ context.Context, characterID uuid.UUID) {
	m.publishedFor = characterID
}

func TestUnattuneHandler_PublishesSnapshot(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	charID := uuid.New()

	slots := []character.AttunementSlot{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection"},
	}
	slotsJSON, _ := json.Marshal(slots)

	store := &mockAttuneStore{char: refdata.Character{ID: charID}}

	handler := NewUnattuneHandler(sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockInventoryCharacterLookup{char: refdata.Character{
			ID:              charID,
			CampaignID:      campID,
			Name:            "Aria",
			AttunementSlots: pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		}},
		store,
	)

	pub := &mockAttunePublisher{}
	handler.SetPublisher(pub)

	interaction := makeUnattuneInteraction("guild1", "user1", "cloak-of-protection")
	handler.Handle(interaction)

	assert.Contains(t, sess.lastResponse, "Unattuned")
	assert.Equal(t, charID, pub.publishedFor, "expected PublishForCharacter to be called with the character ID")
}
