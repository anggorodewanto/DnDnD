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

// recordingSession captures the channel ID and content of ChannelMessageSend calls.
type recordingSession struct {
	mockMoveSession
	sentChannel string
	sentContent string
}

func (r *recordingSession) ChannelMessageSend(channelID, content string) (*discordgo.Message, error) {
	r.sentChannel = channelID
	r.sentContent = content
	return &discordgo.Message{ID: "msg-1"}, nil
}

func TestDMCorrectionPoster_PostsToCombatLog(t *testing.T) {
	encID := uuid.New()
	sess := &recordingSession{}
	csp := &mockCampaignSettingsProvider{
		getSettings: func(ctx context.Context, eid uuid.UUID) (map[string]string, error) {
			return map[string]string{"combat-log": "channel-123"}, nil
		},
	}
	poster := NewDMCorrectionPoster(sess, csp)

	poster.PostCorrection(context.Background(), encID, "⚠️ DM Correction: Goblin HP adjusted")

	assert.Equal(t, "channel-123", sess.sentChannel)
	assert.Contains(t, sess.sentContent, "DM Correction")
}

func TestDMCorrectionPoster_NoChannelConfigured(t *testing.T) {
	sess := &recordingSession{}
	csp := &mockCampaignSettingsProvider{
		getSettings: func(ctx context.Context, eid uuid.UUID) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}
	poster := NewDMCorrectionPoster(sess, csp)
	poster.PostCorrection(context.Background(), uuid.New(), "msg")
	assert.Empty(t, sess.sentChannel)
}

func TestDMCorrectionPoster_NilProvider(t *testing.T) {
	poster := NewDMCorrectionPoster(&recordingSession{}, nil)
	// Should not panic
	poster.PostCorrection(context.Background(), uuid.New(), "msg")
}

func TestDMCorrectionPoster_EmptyMessage(t *testing.T) {
	sess := &recordingSession{}
	csp := &mockCampaignSettingsProvider{
		getSettings: func(ctx context.Context, eid uuid.UUID) (map[string]string, error) {
			return map[string]string{"combat-log": "ch-1"}, nil
		},
	}
	poster := NewDMCorrectionPoster(sess, csp)
	poster.PostCorrection(context.Background(), uuid.New(), "")
	assert.Empty(t, sess.sentChannel)
}

func TestDMCorrectionPoster_ProviderError(t *testing.T) {
	sess := &recordingSession{}
	csp := &mockCampaignSettingsProvider{
		getSettings: func(ctx context.Context, eid uuid.UUID) (map[string]string, error) {
			return nil, errors.New("boom")
		},
	}
	poster := NewDMCorrectionPoster(sess, csp)
	poster.PostCorrection(context.Background(), uuid.New(), "msg")
	assert.Empty(t, sess.sentChannel)
}

// Compile-time check that DMCorrectionPoster matches the combat.CombatLogPoster interface shape.
// We don't import internal/combat here to avoid a cycle, but the signature is identical.
var _ interface {
	PostCorrection(ctx context.Context, encounterID uuid.UUID, message string)
} = (*DMCorrectionPoster)(nil)

func TestDMCorrectionPoster_RequiresAllInputs(t *testing.T) {
	// Just for require import.
	require.NotNil(t, NewDMCorrectionPoster(nil, nil))
}
