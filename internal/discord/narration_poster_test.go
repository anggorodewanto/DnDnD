package discord

import (
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"

	"github.com/ab/dndnd/internal/narration"
)

func TestNarrationPoster_PostToStory_ResolvesChannelAndSends(t *testing.T) {
	var sends []*discordgo.MessageSend
	var sendChannels []string
	mock := &MockSession{
		GuildChannelsFunc: func(guildID string) ([]*discordgo.Channel, error) {
			return []*discordgo.Channel{
				{ID: "c-chat", Name: "player-chat", Type: discordgo.ChannelTypeGuildText},
				{ID: "c-story", Name: "the-story", Type: discordgo.ChannelTypeGuildText},
			}, nil
		},
		ChannelMessageSendComplexFunc: func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
			sendChannels = append(sendChannels, channelID)
			sends = append(sends, data)
			return &discordgo.Message{ID: "m-1"}, nil
		},
	}

	poster := NewNarrationPoster(mock)
	ids, err := poster.PostToStory("guild-1", "Hello adventurers!",
		[]narration.DiscordEmbed{{Description: "Boxed text.", Color: 0xD4AF37}},
		[]string{"https://example.com/scene.png"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "m-1" {
		t.Fatalf("ids = %v", ids)
	}
	if len(sendChannels) != 1 || sendChannels[0] != "c-story" {
		t.Fatalf("expected send to c-story, got %v", sendChannels)
	}
	if !strings.Contains(sends[0].Content, "Hello adventurers!") {
		t.Fatalf("content = %q", sends[0].Content)
	}
	if len(sends[0].Embeds) != 1 || sends[0].Embeds[0].Description != "Boxed text." {
		t.Fatalf("embed missing/wrong: %+v", sends[0].Embeds)
	}
	if sends[0].Embeds[0].Color != 0xD4AF37 {
		t.Fatalf("embed color = %d", sends[0].Embeds[0].Color)
	}
	// Attachment URL should appear somewhere — either in content or as a secondary message.
	full := sends[0].Content
	for _, s := range sends[1:] {
		full += "\n" + s.Content
	}
	if !strings.Contains(full, "https://example.com/scene.png") {
		t.Fatalf("attachment URL not present in any sent message")
	}
}

func TestNarrationPoster_PostToStory_ChannelMissing(t *testing.T) {
	mock := &MockSession{
		GuildChannelsFunc: func(guildID string) ([]*discordgo.Channel, error) {
			return []*discordgo.Channel{
				{ID: "c-chat", Name: "player-chat", Type: discordgo.ChannelTypeGuildText},
			}, nil
		},
	}
	poster := NewNarrationPoster(mock)
	_, err := poster.PostToStory("guild-1", "hi", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "the-story") {
		t.Fatalf("expected missing channel error, got %v", err)
	}
}

func TestNarrationPoster_PostToStory_GuildChannelsError(t *testing.T) {
	mock := &MockSession{
		GuildChannelsFunc: func(guildID string) ([]*discordgo.Channel, error) {
			return nil, errors.New("api broken")
		},
	}
	poster := NewNarrationPoster(mock)
	_, err := poster.PostToStory("guild-1", "hi", nil, nil)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestNarrationPoster_PostToStory_SplitsLongBody(t *testing.T) {
	var sentContents []string
	mock := &MockSession{
		GuildChannelsFunc: func(guildID string) ([]*discordgo.Channel, error) {
			return []*discordgo.Channel{{ID: "c-story", Name: "the-story", Type: discordgo.ChannelTypeGuildText}}, nil
		},
		ChannelMessageSendComplexFunc: func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
			sentContents = append(sentContents, data.Content)
			return &discordgo.Message{ID: "m-" + itoa(len(sentContents))}, nil
		},
	}
	// Build a 2500-char body with newlines so splitting works cleanly.
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString(strings.Repeat("A", 49))
		b.WriteString("\n")
	}
	body := b.String() // ~2500 chars

	poster := NewNarrationPoster(mock)
	ids, err := poster.PostToStory("guild-1", body, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) < 2 {
		t.Fatalf("expected multiple message ids for split body, got %d", len(ids))
	}
	if len(sentContents) != len(ids) {
		t.Fatalf("sends (%d) != ids (%d)", len(sentContents), len(ids))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
