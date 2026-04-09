package discord

import (
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTurnTimerNotifier_SendMessage_ForwardsToSession(t *testing.T) {
	var gotChannel, gotContent string
	session := &MockSession{
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			gotChannel = channelID
			gotContent = content
			return &discordgo.Message{ID: "m-1"}, nil
		},
	}
	n := NewTurnTimerNotifier(session)

	err := n.SendMessage("chan-42", "hello world")
	require.NoError(t, err)
	assert.Equal(t, "chan-42", gotChannel)
	assert.Equal(t, "hello world", gotContent)
}

func TestTurnTimerNotifier_SendMessage_PropagatesError(t *testing.T) {
	session := &MockSession{
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			return nil, errors.New("discord api down")
		},
	}
	n := NewTurnTimerNotifier(session)

	err := n.SendMessage("c", "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "discord api down")
}
