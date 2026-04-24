package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// CampaignAnnouncer implements campaign.Announcer by sending status-transition
// messages (pause, resume) to a guild's #the-story channel via the injected
// Session. It is deliberately small — no splitting, no embeds — because the
// announcement copy is always short and self-contained per the Phase 115 spec.
type CampaignAnnouncer struct {
	session Session
}

// NewCampaignAnnouncer constructs a CampaignAnnouncer using the given session.
func NewCampaignAnnouncer(session Session) *CampaignAnnouncer {
	return &CampaignAnnouncer{session: session}
}

// AnnounceToStory resolves #the-story in the given guild and posts message.
// Any resolution or send failure is returned so the campaign service can log
// it; the service treats announcement failure as non-fatal by design.
func (a *CampaignAnnouncer) AnnounceToStory(guildID, message string) error {
	channelID, err := a.resolveStoryChannel(guildID)
	if err != nil {
		return err
	}
	if _, err := a.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{Content: message}); err != nil {
		return fmt.Errorf("sending campaign announcement: %w", err)
	}
	return nil
}

// resolveStoryChannel returns the channel ID for #the-story in the given guild.
func (a *CampaignAnnouncer) resolveStoryChannel(guildID string) (string, error) {
	channels, err := a.session.GuildChannels(guildID)
	if err != nil {
		return "", fmt.Errorf("fetching guild channels: %w", err)
	}
	for _, ch := range channels {
		if ch.Type == discordgo.ChannelTypeGuildText && ch.Name == theStoryChannelName {
			return ch.ID, nil
		}
	}
	return "", fmt.Errorf("#%s channel not found in guild %s (run /setup first)", theStoryChannelName, guildID)
}
