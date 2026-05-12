package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/narration"
	"github.com/ab/dndnd/internal/refdata"
)

// fakeLevelUpStoryQueries is an in-memory stub for the two refdata.Queries
// methods levelUpStoryPosterAdapter touches. Keeps the unit test free of a
// real DB while exercising the character → campaign → guild chain.
type fakeLevelUpStoryQueries struct {
	chars     map[uuid.UUID]refdata.Character
	camps     map[uuid.UUID]refdata.Campaign
	charErr   error
	campErr   error
}

func (f *fakeLevelUpStoryQueries) GetCharacter(_ context.Context, id uuid.UUID) (refdata.Character, error) {
	if f.charErr != nil {
		return refdata.Character{}, f.charErr
	}
	c, ok := f.chars[id]
	if !ok {
		return refdata.Character{}, errors.New("character not found")
	}
	return c, nil
}

func (f *fakeLevelUpStoryQueries) GetCampaignByID(_ context.Context, id uuid.UUID) (refdata.Campaign, error) {
	if f.campErr != nil {
		return refdata.Campaign{}, f.campErr
	}
	c, ok := f.camps[id]
	if !ok {
		return refdata.Campaign{}, errors.New("campaign not found")
	}
	return c, nil
}

// recordingNarrationPoster captures PostToStory invocations.
type recordingNarrationPoster struct {
	calls []narrationCall
	err   error
}

type narrationCall struct {
	guildID string
	body    string
}

func (r *recordingNarrationPoster) PostToStory(guildID, body string, _ []narration.DiscordEmbed, _ []string) ([]string, error) {
	r.calls = append(r.calls, narrationCall{guildID: guildID, body: body})
	if r.err != nil {
		return nil, r.err
	}
	return []string{"msg-1"}, nil
}

func TestLevelUpStoryPosterAdapter_PostsFormattedAnnouncementToStory(t *testing.T) {
	// H-104c: a level-up posts "🎉 <name> has reached Level N!" to the
	// owning campaign's #the-story (resolved character → campaign → guild).
	campID := uuid.New()
	charID := uuid.New()
	q := &fakeLevelUpStoryQueries{
		chars: map[uuid.UUID]refdata.Character{
			charID: {ID: charID, CampaignID: campID, Name: "Aria"},
		},
		camps: map[uuid.UUID]refdata.Campaign{
			campID: {ID: campID, GuildID: "guild-42"},
		},
	}
	p := &recordingNarrationPoster{}

	adapter := newLevelUpStoryPosterAdapter(q, p)
	require.NotNil(t, adapter)

	err := adapter.PostPublicLevelUp(context.Background(), charID, "Aria", 6)
	require.NoError(t, err)

	require.Len(t, p.calls, 1)
	assert.Equal(t, "guild-42", p.calls[0].guildID)
	assert.True(t, strings.Contains(p.calls[0].body, "Aria"), "body should mention character name")
	assert.True(t, strings.Contains(p.calls[0].body, "Level 6"), "body should mention new level")
}

func TestLevelUpStoryPosterAdapter_NilDependencies_NoOp(t *testing.T) {
	// Either queries or poster being nil yields a nil adapter and any
	// callers (notifierAdapter wraps that nil) silently no-op.
	assert.Nil(t, newLevelUpStoryPosterAdapter(nil, &recordingNarrationPoster{}))
	assert.Nil(t, newLevelUpStoryPosterAdapter(&fakeLevelUpStoryQueries{}, nil))
}

func TestLevelUpStoryPosterAdapter_MissingCharacter_SurfacesError(t *testing.T) {
	q := &fakeLevelUpStoryQueries{chars: map[uuid.UUID]refdata.Character{}}
	p := &recordingNarrationPoster{}
	adapter := newLevelUpStoryPosterAdapter(q, p)

	err := adapter.PostPublicLevelUp(context.Background(), uuid.New(), "Ghost", 2)
	require.Error(t, err)
	assert.Empty(t, p.calls)
}

func TestLevelUpStoryPosterAdapter_MissingCampaign_SurfacesError(t *testing.T) {
	charID := uuid.New()
	q := &fakeLevelUpStoryQueries{
		chars: map[uuid.UUID]refdata.Character{
			charID: {ID: charID, CampaignID: uuid.New(), Name: "Aria"},
		},
	}
	p := &recordingNarrationPoster{}
	adapter := newLevelUpStoryPosterAdapter(q, p)

	err := adapter.PostPublicLevelUp(context.Background(), charID, "Aria", 6)
	require.Error(t, err)
	assert.Empty(t, p.calls)
}

func TestLevelUpStoryPosterAdapter_EmptyGuildID_NoOp(t *testing.T) {
	// A campaign without a guild_id (e.g. mid-setup) silently no-ops so a
	// dashboard level-up still succeeds even if the bot has not joined a
	// guild yet.
	campID := uuid.New()
	charID := uuid.New()
	q := &fakeLevelUpStoryQueries{
		chars: map[uuid.UUID]refdata.Character{
			charID: {ID: charID, CampaignID: campID, Name: "Aria"},
		},
		camps: map[uuid.UUID]refdata.Campaign{
			campID: {ID: campID, GuildID: ""},
		},
	}
	p := &recordingNarrationPoster{}
	adapter := newLevelUpStoryPosterAdapter(q, p)

	err := adapter.PostPublicLevelUp(context.Background(), charID, "Aria", 6)
	require.NoError(t, err)
	assert.Empty(t, p.calls)
}

func TestLevelUpStoryPosterAdapter_PosterError_Surfaces(t *testing.T) {
	campID := uuid.New()
	charID := uuid.New()
	q := &fakeLevelUpStoryQueries{
		chars: map[uuid.UUID]refdata.Character{
			charID: {ID: charID, CampaignID: campID, Name: "Aria"},
		},
		camps: map[uuid.UUID]refdata.Campaign{
			campID: {ID: campID, GuildID: "guild-42"},
		},
	}
	p := &recordingNarrationPoster{err: errors.New("discord 500")}
	adapter := newLevelUpStoryPosterAdapter(q, p)

	err := adapter.PostPublicLevelUp(context.Background(), charID, "Aria", 6)
	require.Error(t, err)
}
