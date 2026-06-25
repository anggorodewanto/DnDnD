package discord

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMapEncounterProvider struct {
	fn func(ctx context.Context, guildID, userID string) (uuid.UUID, error)
}

func (f *fakeMapEncounterProvider) ActiveEncounterForUser(ctx context.Context, guildID, userID string) (uuid.UUID, error) {
	return f.fn(ctx, guildID, userID)
}

type recordingCombatMapPoster struct {
	calls []uuid.UUID
}

func (r *recordingCombatMapPoster) PostCombatMap(_ context.Context, encounterID uuid.UUID) {
	r.calls = append(r.calls, encounterID)
}

func mapInteraction() *discordgo.Interaction {
	return &discordgo.Interaction{
		GuildID: "guild-1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user-1"}},
	}
}

// TestMapHandler_PostsMap: with an active encounter, /map acknowledges the
// invoker and posts the board for that encounter.
func TestMapHandler_PostsMap(t *testing.T) {
	encounterID := uuid.New()

	var respContent string
	session := &MockSession{
		InteractionRespondFunc: func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			respContent = resp.Data.Content
			return nil
		},
	}
	var gotGuild, gotUser string
	provider := &fakeMapEncounterProvider{
		fn: func(_ context.Context, guildID, userID string) (uuid.UUID, error) {
			gotGuild, gotUser = guildID, userID
			return encounterID, nil
		},
	}
	poster := &recordingCombatMapPoster{}

	h := NewMapHandler(session, provider, poster)
	h.Handle(mapInteraction())

	assert.Equal(t, "guild-1", gotGuild)
	assert.Equal(t, "user-1", gotUser)
	assert.Contains(t, respContent, "#combat-map")
	require.Equal(t, []uuid.UUID{encounterID}, poster.calls, "the active encounter's board must be posted")
}

// TestMapHandler_NoActiveEncounter_Ephemeral: with no active encounter, /map
// replies with a friendly ephemeral and posts nothing.
func TestMapHandler_NoActiveEncounter_Ephemeral(t *testing.T) {
	var respContent string
	session := &MockSession{
		InteractionRespondFunc: func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			respContent = resp.Data.Content
			return nil
		},
	}
	provider := &fakeMapEncounterProvider{
		fn: func(_ context.Context, _, _ string) (uuid.UUID, error) {
			return uuid.Nil, errors.New("no active encounter")
		},
	}
	poster := &recordingCombatMapPoster{}

	h := NewMapHandler(session, provider, poster)
	h.Handle(mapInteraction())

	assert.Contains(t, respContent, "No active combat")
	assert.Empty(t, poster.calls, "nothing should be posted when there is no active encounter")
}

// TestMapHandler_AsyncDispatch routes the post through the wired dispatcher
// instead of running it inline.
func TestMapHandler_AsyncDispatch(t *testing.T) {
	encounterID := uuid.New()
	session := &MockSession{
		InteractionRespondFunc: func(_ *discordgo.Interaction, _ *discordgo.InteractionResponse) error { return nil },
	}
	provider := &fakeMapEncounterProvider{
		fn: func(_ context.Context, _, _ string) (uuid.UUID, error) { return encounterID, nil },
	}
	poster := &recordingCombatMapPoster{}

	dispatched := false
	h := NewMapHandler(session, provider, poster)
	h.SetNotifyDispatcher(func(f func()) { dispatched = true; f() })
	h.Handle(mapInteraction())

	assert.True(t, dispatched, "the post must route through the wired dispatcher")
	assert.Equal(t, []uuid.UUID{encounterID}, poster.calls)
}
