package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"github.com/ab/dndnd/internal/narration"
)

// ChannelPoster sends messages to an arbitrary channel ID as the bot via the
// injected Session. Unlike NarrationPoster (which resolves #the-story by name),
// the caller supplies the channel ID directly, so the DM dashboard can post to
// any of a campaign's configured channels. Long bodies are split into
// Discord-sized chunks and read-aloud embeds ride the first chunk.
type ChannelPoster struct {
	session Session
}

// NewChannelPoster constructs a ChannelPoster using the given session.
func NewChannelPoster(session Session) *ChannelPoster {
	return &ChannelPoster{session: session}
}

// PostToChannel splits body into chunks and sends them sequentially to
// channelID. Read-aloud embeds are attached to the first chunk only, mirroring
// NarrationPoster.PostToStory. Returns the IDs of every message successfully
// sent; on the first send failure it returns the error and no IDs so the caller
// can treat the whole post as failed.
func (p *ChannelPoster) PostToChannel(channelID, body string, embeds []narration.DiscordEmbed) ([]string, error) {
	chunks := SplitMessage(body)
	if chunks == nil {
		chunks = []string{body}
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
			return nil, fmt.Errorf("sending channel-post chunk %d: %w", i+1, err)
		}
		ids = append(ids, msg.ID)
	}
	return ids, nil
}
