package discord

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/ab/dndnd/internal/narration"
)

// theStoryChannelName is the canonical name of the Narration posting channel
// created by /setup.
const theStoryChannelName = "the-story"

// NarrationPoster implements narration.Poster by sending messages to a
// guild's #the-story text channel via the injected Session. Long messages
// are split using the existing message splitter. Attachment URLs are
// appended so Discord auto-embeds them as images.
type NarrationPoster struct {
	session Session
}

// NewNarrationPoster constructs a NarrationPoster using the given session.
func NewNarrationPoster(session Session) *NarrationPoster {
	return &NarrationPoster{session: session}
}

// PostToStory resolves the guild's #the-story channel, splits the body into
// Discord-sized chunks, and sends them sequentially. Attachments are appended
// to the last chunk as URLs (Discord auto-embeds image URLs). Read-aloud
// embeds are attached to the first chunk so they appear near the narration
// opening.
//
// Returns the IDs of every message successfully sent. If any send fails the
// error is returned and the IDs collected so far are NOT returned, so that
// the caller can treat the post as failed and skip recording it.
func (p *NarrationPoster) PostToStory(guildID, body string, embeds []narration.DiscordEmbed, attachmentURLs []string) ([]string, error) {
	channelID, err := p.resolveStoryChannel(guildID)
	if err != nil {
		return nil, err
	}

	chunks := SplitMessage(body)
	if chunks == nil {
		chunks = []string{body}
	}
	if len(attachmentURLs) > 0 {
		chunks = appendAttachmentsToLastChunk(chunks, attachmentURLs)
	}

	discordEmbeds := toDiscordgoEmbeds(embeds)

	var ids []string
	for i, chunk := range chunks {
		send := &discordgo.MessageSend{Content: chunk}
		if i == 0 {
			send.Embeds = discordEmbeds
		}
		msg, err := p.session.ChannelMessageSendComplex(channelID, send)
		if err != nil {
			return nil, fmt.Errorf("sending narration chunk %d: %w", i+1, err)
		}
		ids = append(ids, msg.ID)
	}
	return ids, nil
}

// resolveStoryChannel returns the channel ID for #the-story in the given guild.
func (p *NarrationPoster) resolveStoryChannel(guildID string) (string, error) {
	channels, err := p.session.GuildChannels(guildID)
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

// appendAttachmentsToLastChunk appends URL lines to the final chunk, or to a
// new chunk if that would exceed the per-message limit.
func appendAttachmentsToLastChunk(chunks []string, urls []string) []string {
	trailer := strings.Join(urls, "\n")
	last := chunks[len(chunks)-1]
	candidate := last
	if candidate != "" {
		candidate += "\n"
	}
	candidate += trailer
	if len(candidate) <= MaxMessageLen {
		chunks[len(chunks)-1] = candidate
		return chunks
	}
	return append(chunks, trailer)
}

// toDiscordgoEmbeds converts the generic narration embed type to
// discordgo's wire format.
func toDiscordgoEmbeds(embeds []narration.DiscordEmbed) []*discordgo.MessageEmbed {
	if len(embeds) == 0 {
		return nil
	}
	out := make([]*discordgo.MessageEmbed, 0, len(embeds))
	for _, e := range embeds {
		out = append(out, &discordgo.MessageEmbed{
			Description: e.Description,
			Color:       e.Color,
		})
	}
	return out
}
