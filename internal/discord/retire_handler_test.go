package discord

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// stubRetireCharLookup returns a fixed character for a discord user.
type stubRetireCharLookup struct {
	char refdata.Character
	err  error
}

func (s *stubRetireCharLookup) GetCharacterByCampaignAndDiscord(ctx context.Context, campaignID uuid.UUID, discordUserID string) (refdata.Character, error) {
	return s.char, s.err
}

func makeRetireInteraction(guildID, userID, reason string) *discordgo.Interaction {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{}
	if reason != "" {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name:  "reason",
			Type:  discordgo.ApplicationCommandOptionString,
			Value: reason,
		})
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: guildID,
		Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "retire",
			Options: opts,
		},
	}
}

func TestRetireHandler_PostsToDMQueue(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	notifier := &stubUndoNotifier{}

	handler := NewRetireHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&stubRetireCharLookup{char: refdata.Character{ID: uuid.New(), Name: "Aria"}},
		notifier,
	)

	handler.Handle(makeRetireInteraction("guild1", "user1", "going to college"))

	require.Len(t, notifier.posted, 1)
	evt := notifier.posted[0]
	assert.Equal(t, dmqueue.KindRetireRequest, evt.Kind)
	assert.Equal(t, "Aria", evt.PlayerName)
	assert.Contains(t, evt.Summary, "going to college")
	assert.Equal(t, "guild1", evt.GuildID)
	assert.Contains(t, sess.lastResponse, "DM")
}

func TestRetireHandler_NoCampaign_StillResponds(t *testing.T) {
	sess := &mockInventorySession{}
	notifier := &stubUndoNotifier{}
	handler := NewRetireHandler(
		sess,
		&mockInventoryCampaignProvider{err: errors.New("no campaign")},
		&stubRetireCharLookup{},
		notifier,
	)
	handler.Handle(makeRetireInteraction("guild1", "user1", "x"))
	assert.Contains(t, sess.lastResponse, "No campaign")
	assert.Empty(t, notifier.posted)
}

func TestRetireHandler_NoCharacter_StillResponds(t *testing.T) {
	sess := &mockInventorySession{}
	notifier := &stubUndoNotifier{}
	campID := uuid.New()
	handler := NewRetireHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&stubRetireCharLookup{err: errors.New("no char")},
		notifier,
	)
	handler.Handle(makeRetireInteraction("guild1", "user1", "x"))
	assert.Contains(t, sess.lastResponse, "Could not find your character")
	assert.Empty(t, notifier.posted)
}

func TestRetireHandler_NoReason_DefaultsToPlaceholder(t *testing.T) {
	sess := &mockInventorySession{}
	notifier := &stubUndoNotifier{}
	campID := uuid.New()
	handler := NewRetireHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&stubRetireCharLookup{char: refdata.Character{ID: uuid.New(), Name: "Aria"}},
		notifier,
	)
	handler.Handle(makeRetireInteraction("guild1", "user1", ""))
	require.Len(t, notifier.posted, 1)
	assert.Contains(t, notifier.posted[0].Summary, "(no reason given)")
}

func TestRetireHandler_NilCampaignProv_RespondsGracefully(t *testing.T) {
	sess := &mockInventorySession{}
	handler := NewRetireHandler(sess, nil, &stubRetireCharLookup{}, nil)
	handler.Handle(makeRetireInteraction("guild1", "user1", "x"))
	assert.Contains(t, sess.lastResponse, "No campaign")
}

func TestRetireHandler_NilCharacterLookup_RespondsGracefully(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	handler := NewRetireHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		nil,
		nil,
	)
	handler.Handle(makeRetireInteraction("guild1", "user1", "x"))
	assert.Contains(t, sess.lastResponse, "Could not find your character")
}

func TestRetireHandler_NoNotifier_StillRespondsConfirmation(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	handler := NewRetireHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&stubRetireCharLookup{char: refdata.Character{ID: uuid.New(), Name: "Aria"}},
		nil,
	)
	handler.Handle(makeRetireInteraction("guild1", "user1", "x"))
	assert.Contains(t, sess.lastResponse, "DM")
}
