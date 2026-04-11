package levelup_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/levelup"
)

// fakeDirectMessenger records DM calls made by the notifier adapter so the
// tests can assert on the rendered Discord-facing message text without
// needing a real Discord session.
type fakeDirectMessenger struct {
	calls []fakeDM
	err   error
}

type fakeDM struct {
	userID string
	body   string
}

func (f *fakeDirectMessenger) SendDirectMessage(userID, body string) ([]string, error) {
	f.calls = append(f.calls, fakeDM{userID: userID, body: body})
	if f.err != nil {
		return nil, f.err
	}
	return []string{"msg-id"}, nil
}

func TestNotifierAdapter_SendPrivateLevelUp_DMsFormattedBody(t *testing.T) {
	fake := &fakeDirectMessenger{}
	adapter := levelup.NewNotifierAdapter(fake)

	err := adapter.SendPrivateLevelUp(context.Background(), "user-1", levelup.LevelUpDetails{
		CharacterName:     "Aria",
		LeveledClass:      "fighter",
		LeveledClassLevel: 6,
		OldLevel:          5,
		NewLevel:          6,
		HPGained:          7,
	})
	require.NoError(t, err)
	require.Len(t, fake.calls, 1)
	assert.Equal(t, "user-1", fake.calls[0].userID)
	assert.Contains(t, fake.calls[0].body, "Aria")
	assert.Contains(t, fake.calls[0].body, "Level")
}

func TestNotifierAdapter_SendASIPrompt_DMsPlayer(t *testing.T) {
	fake := &fakeDirectMessenger{}
	adapter := levelup.NewNotifierAdapter(fake)

	err := adapter.SendASIPrompt(context.Background(), "user-2", uuid.New(), "Bree")
	require.NoError(t, err)
	require.Len(t, fake.calls, 1)
	assert.Equal(t, "user-2", fake.calls[0].userID)
	assert.Contains(t, fake.calls[0].body, "Bree")
}

func TestNotifierAdapter_SendASIDenied_DMsPlayerWithReason(t *testing.T) {
	fake := &fakeDirectMessenger{}
	adapter := levelup.NewNotifierAdapter(fake)

	err := adapter.SendASIDenied(context.Background(), "user-3", "Caro", "no min-maxing")
	require.NoError(t, err)
	require.Len(t, fake.calls, 1)
	assert.Equal(t, "user-3", fake.calls[0].userID)
	assert.Contains(t, fake.calls[0].body, "Caro")
	assert.Contains(t, fake.calls[0].body, "no min-maxing")
}

func TestNotifierAdapter_SendPublicLevelUp_NoOp(t *testing.T) {
	// Phase 104c defers real public-channel posting; the adapter should
	// be callable without error and must NOT fall back to DMing anyone.
	fake := &fakeDirectMessenger{}
	adapter := levelup.NewNotifierAdapter(fake)
	err := adapter.SendPublicLevelUp(context.Background(), "Aria", 6)
	require.NoError(t, err)
	assert.Empty(t, fake.calls)
}

func TestNotifierAdapter_NilMessenger_Tolerated(t *testing.T) {
	// Production may start with no Discord session (DISCORD_BOT_TOKEN
	// unset). The adapter must silently no-op rather than panic so the
	// level-up HTTP handler can still mutate state.
	adapter := levelup.NewNotifierAdapter(nil)
	require.NoError(t, adapter.SendPrivateLevelUp(context.Background(), "u", levelup.LevelUpDetails{CharacterName: "x"}))
	require.NoError(t, adapter.SendASIPrompt(context.Background(), "u", uuid.New(), "x"))
	require.NoError(t, adapter.SendASIDenied(context.Background(), "u", "x", "y"))
	require.NoError(t, adapter.SendPublicLevelUp(context.Background(), "x", 2))
}

func TestNotifierAdapter_EmptyDiscordUserID_SkipsDM(t *testing.T) {
	// DM NPCs / unlinked characters surface an empty discord_user_id. The
	// adapter should skip the DM rather than call the messenger with "".
	fake := &fakeDirectMessenger{}
	adapter := levelup.NewNotifierAdapter(fake)

	err := adapter.SendPrivateLevelUp(context.Background(), "", levelup.LevelUpDetails{CharacterName: "Ghoul"})
	require.NoError(t, err)
	assert.Empty(t, fake.calls)
}

func TestNotifierAdapter_PassesThroughMessageContaining(t *testing.T) {
	// Sanity: the rendered private body should include class/level bits so
	// we know we're using the existing format helpers and not a stub.
	fake := &fakeDirectMessenger{}
	adapter := levelup.NewNotifierAdapter(fake)
	require.NoError(t, adapter.SendPrivateLevelUp(context.Background(), "user-x", levelup.LevelUpDetails{
		CharacterName:     "Dima",
		LeveledClass:      "wizard",
		LeveledClassLevel: 3,
		NewLevel:          3,
		HPGained:          4,
	}))
	require.Len(t, fake.calls, 1)
	assert.True(t, strings.Contains(fake.calls[0].body, "wizard"), "body should mention class name")
}
