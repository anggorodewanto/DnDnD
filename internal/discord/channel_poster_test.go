package discord

import (
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"

	"github.com/ab/dndnd/internal/narration"
)

func TestChannelPoster_PostToChannel_SendsWithEmbeds(t *testing.T) {
	var sends []*discordgo.MessageSend
	var channels []string
	mock := &MockSession{
		ChannelMessageSendComplexFunc: func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
			channels = append(channels, channelID)
			sends = append(sends, data)
			return &discordgo.Message{ID: "m-1"}, nil
		},
	}

	poster := NewChannelPoster(mock)
	ids, err := poster.PostToChannel("c-inchar", "OOC: heads up.",
		[]narration.DiscordEmbed{{Description: "Boxed.", Color: 0xD4AF37}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "m-1" {
		t.Fatalf("ids = %v", ids)
	}
	if len(channels) != 1 || channels[0] != "c-inchar" {
		t.Fatalf("expected send to c-inchar, got %v", channels)
	}
	if !strings.Contains(sends[0].Content, "OOC: heads up.") {
		t.Fatalf("content = %q", sends[0].Content)
	}
	if len(sends[0].Embeds) != 1 || sends[0].Embeds[0].Description != "Boxed." {
		t.Fatalf("embed missing/wrong: %+v", sends[0].Embeds)
	}
	if sends[0].Embeds[0].Color != 0xD4AF37 {
		t.Fatalf("embed color = %d", sends[0].Embeds[0].Color)
	}
}

func TestChannelPoster_PostToChannel_SplitsLongBody(t *testing.T) {
	var sent []string
	mock := &MockSession{
		ChannelMessageSendComplexFunc: func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
			sent = append(sent, data.Content)
			return &discordgo.Message{ID: "m-" + itoa(len(sent))}, nil
		},
	}
	var b strings.Builder
	for range 50 {
		b.WriteString(strings.Repeat("A", 49))
		b.WriteString("\n")
	}

	poster := NewChannelPoster(mock)
	ids, err := poster.PostToChannel("c-x", b.String(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) < 2 {
		t.Fatalf("expected split into multiple messages, got %d ids", len(ids))
	}
	if len(sent) != len(ids) {
		t.Fatalf("sent %d != ids %d", len(sent), len(ids))
	}
}

func TestChannelPoster_PostToChannel_SendError(t *testing.T) {
	mock := &MockSession{
		ChannelMessageSendComplexFunc: func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
			return nil, errors.New("discord down")
		},
	}
	poster := NewChannelPoster(mock)
	if _, err := poster.PostToChannel("c-x", "hi", nil); err == nil {
		t.Fatalf("expected error")
	}
}
