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

// stubRetirePCStore records calls to MarkPlayerCharacterRetireRequested.
type stubRetirePCStore struct {
	calls  []refdata.MarkPlayerCharacterRetireRequestedParams
	result refdata.PlayerCharacter
	err    error
}

func (s *stubRetirePCStore) MarkPlayerCharacterRetireRequested(ctx context.Context, arg refdata.MarkPlayerCharacterRetireRequestedParams) (refdata.PlayerCharacter, error) {
	s.calls = append(s.calls, arg)
	return s.result, s.err
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

// A-08-retire-created-via-schema / A-16-retire-approval-unreachable:
// /retire must flag the existing player_characters row with
// created_via='retire' so the dashboard approval handler's retire branch
// (approval_handler.go:248) becomes reachable. The handler should still
// also post a dm-queue notification (existing behaviour, kept).
func TestRetireHandler_FlagsPlayerCharacterAsRetireRequest(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	pcStore := &stubRetirePCStore{}
	notifier := &stubUndoNotifier{}

	handler := NewRetireHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&stubRetireCharLookup{char: refdata.Character{ID: uuid.New(), Name: "Aria"}},
		notifier,
	)
	handler.SetPCStore(pcStore)

	handler.Handle(makeRetireInteraction("guild1", "user1", "going to college"))

	require.Len(t, pcStore.calls, 1, "MarkPlayerCharacterRetireRequested should be called exactly once")
	assert.Equal(t, campID, pcStore.calls[0].CampaignID)
	assert.Equal(t, "user1", pcStore.calls[0].DiscordUserID)

	// The dm-queue notifier must still fire so the DM is pinged.
	require.Len(t, notifier.posted, 1)
	assert.Equal(t, dmqueue.KindRetireRequest, notifier.posted[0].Kind)
}

// Wiring guard: when the player_characters store update fails, /retire must
// surface a clear error to the player rather than silently completing and
// leaving the DM thinking it was queued.
func TestRetireHandler_PCStoreError_RespondsWithFailure(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	pcStore := &stubRetirePCStore{err: errors.New("db down")}
	notifier := &stubUndoNotifier{}

	handler := NewRetireHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&stubRetireCharLookup{char: refdata.Character{ID: uuid.New(), Name: "Aria"}},
		notifier,
	)
	handler.SetPCStore(pcStore)

	handler.Handle(makeRetireInteraction("guild1", "user1", "going to college"))

	assert.Contains(t, sess.lastResponse, "Could not record retire request")
	// Notifier should NOT have fired because the flag never landed.
	assert.Empty(t, notifier.posted)
}

// Backwards-compatibility guard: with no PC store wired (the bare
// NewRetireHandler call from earlier wiring), the handler must still post to
// the dm-queue and respond, so existing tests do not regress.
func TestRetireHandler_NoPCStore_NotifierStillFires(t *testing.T) {
	sess := &mockInventorySession{}
	campID := uuid.New()
	notifier := &stubUndoNotifier{}

	handler := NewRetireHandler(
		sess,
		&mockInventoryCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&stubRetireCharLookup{char: refdata.Character{ID: uuid.New(), Name: "Aria"}},
		notifier,
	)
	// no SetPCStore call

	handler.Handle(makeRetireInteraction("guild1", "user1", "x"))

	require.Len(t, notifier.posted, 1)
	assert.Equal(t, dmqueue.KindRetireRequest, notifier.posted[0].Kind)
}
