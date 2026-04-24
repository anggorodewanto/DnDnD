package discord

import (
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// Phase 115: CampaignAnnouncer posts pause/resume announcements to #the-story.
// It implements campaign.Announcer (AnnounceToStory).
func TestCampaignAnnouncer_AnnounceToStory_ResolvesChannelAndSends(t *testing.T) {
	var sentChannel, sentContent string
	mock := &MockSession{
		GuildChannelsFunc: func(guildID string) ([]*discordgo.Channel, error) {
			if guildID != "guild-42" {
				t.Fatalf("unexpected guildID %q", guildID)
			}
			return []*discordgo.Channel{
				{ID: "c-chat", Name: "player-chat", Type: discordgo.ChannelTypeGuildText},
				{ID: "c-story", Name: "the-story", Type: discordgo.ChannelTypeGuildText},
			}, nil
		},
		ChannelMessageSendComplexFunc: func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
			sentChannel = channelID
			sentContent = data.Content
			return &discordgo.Message{ID: "m-9"}, nil
		},
	}

	ann := NewCampaignAnnouncer(mock)
	err := ann.AnnounceToStory("guild-42", "⏸️ **Campaign paused by DM.**")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sentChannel != "c-story" {
		t.Fatalf("expected send to c-story, got %q", sentChannel)
	}
	if sentContent != "⏸️ **Campaign paused by DM.**" {
		t.Fatalf("content = %q", sentContent)
	}
}

func TestCampaignAnnouncer_AnnounceToStory_ChannelMissing(t *testing.T) {
	mock := &MockSession{
		GuildChannelsFunc: func(guildID string) ([]*discordgo.Channel, error) {
			return []*discordgo.Channel{
				{ID: "c-chat", Name: "player-chat", Type: discordgo.ChannelTypeGuildText},
			}, nil
		},
	}
	ann := NewCampaignAnnouncer(mock)
	err := ann.AnnounceToStory("guild-1", "hi")
	if err == nil {
		t.Fatal("expected error when #the-story is missing")
	}
}

func TestCampaignAnnouncer_AnnounceToStory_GuildChannelsError(t *testing.T) {
	mock := &MockSession{
		GuildChannelsFunc: func(guildID string) ([]*discordgo.Channel, error) {
			return nil, errors.New("api broken")
		},
	}
	ann := NewCampaignAnnouncer(mock)
	err := ann.AnnounceToStory("guild-1", "hi")
	if err == nil {
		t.Fatal("expected error when GuildChannels fails")
	}
}

func TestCampaignAnnouncer_AnnounceToStory_SendError(t *testing.T) {
	mock := &MockSession{
		GuildChannelsFunc: func(guildID string) ([]*discordgo.Channel, error) {
			return []*discordgo.Channel{{ID: "c-story", Name: "the-story", Type: discordgo.ChannelTypeGuildText}}, nil
		},
		ChannelMessageSendComplexFunc: func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
			return nil, errors.New("send failed")
		},
	}
	ann := NewCampaignAnnouncer(mock)
	err := ann.AnnounceToStory("guild-1", "hi")
	if err == nil {
		t.Fatal("expected error when send fails")
	}
}
