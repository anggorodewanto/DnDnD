package dmqueue

import "github.com/bwmarrin/discordgo"

// sessionAPI is the minimal subset of discordgo.Session used by SessionSender.
// Mirrors the methods of internal/discord.Session we need.
type sessionAPI interface {
	ChannelMessageSend(channelID, content string) (*discordgo.Message, error)
	ChannelMessageEdit(channelID, messageID, content string) (*discordgo.Message, error)
}

// SessionSender adapts a discordgo.Session (or any sessionAPI) to the
// Notifier's Sender interface by returning message IDs from sends.
//
// Note: unlike MessageQueue, this does not queue or apply 429 backoff.
// dm-queue writes are low-volume (one per player event), so direct sends
// are acceptable for Phase 106a. If rate-limit issues arise, wrap this
// in a message-ID-aware queue later.
type SessionSender struct {
	session sessionAPI
}

// NewSessionSender constructs a SessionSender.
func NewSessionSender(s sessionAPI) *SessionSender {
	return &SessionSender{session: s}
}

// Send posts a new message and returns the created message's ID.
func (s *SessionSender) Send(channelID, content string) (string, error) {
	msg, err := s.session.ChannelMessageSend(channelID, content)
	if err != nil {
		return "", err
	}
	return msg.ID, nil
}

// Edit updates an existing message's content.
func (s *SessionSender) Edit(channelID, messageID, content string) error {
	_, err := s.session.ChannelMessageEdit(channelID, messageID, content)
	return err
}
