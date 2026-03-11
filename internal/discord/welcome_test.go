package discord

import (
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestSendWelcomeDM_Success(t *testing.T) {
	var sentChannelID, sentContent string
	mock := &MockSession{
		UserChannelCreateFunc: func(recipientID string) (*discordgo.Channel, error) {
			return &discordgo.Channel{ID: "dm-ch-1"}, nil
		},
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			sentChannelID = channelID
			sentContent = content
			return &discordgo.Message{}, nil
		},
	}

	err := SendWelcomeDM(mock, "user-1", "Curse of Strahd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sentChannelID != "dm-ch-1" {
		t.Fatalf("expected dm-ch-1, got %s", sentChannelID)
	}
	if !strings.Contains(sentContent, "Curse of Strahd") {
		t.Fatal("expected campaign name in message")
	}
}

func TestSendWelcomeDM_ChannelCreateError(t *testing.T) {
	mock := &MockSession{
		UserChannelCreateFunc: func(recipientID string) (*discordgo.Channel, error) {
			return nil, errors.New("cannot DM user")
		},
	}

	err := SendWelcomeDM(mock, "user-1", "Test Campaign")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "creating DM channel") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSendWelcomeDM_SendError(t *testing.T) {
	mock := &MockSession{
		UserChannelCreateFunc: func(recipientID string) (*discordgo.Channel, error) {
			return &discordgo.Channel{ID: "dm-ch-1"}, nil
		},
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			return nil, errors.New("send failed")
		},
	}

	err := SendWelcomeDM(mock, "user-1", "Test Campaign")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "sending welcome DM") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
