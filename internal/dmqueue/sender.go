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
// If content exceeds Discord's 2000-char limit, it is split into multiple
// messages; the last message's ID is returned.
func (s *SessionSender) Send(channelID, content string) (string, error) {
	const maxLen = 2000
	if len(content) <= maxLen {
		msg, err := s.session.ChannelMessageSend(channelID, content)
		if err != nil {
			return "", err
		}
		return msg.ID, nil
	}

	// Split on newline boundaries, falling back to hard cut.
	var lastID string
	for len(content) > 0 {
		chunk := content
		if len(chunk) > maxLen {
			cut := maxLen
			if idx := lastNewline(content[:maxLen]); idx > 0 {
				cut = idx + 1
			}
			chunk = content[:cut]
			content = content[cut:]
		} else {
			content = ""
		}
		msg, err := s.session.ChannelMessageSend(channelID, chunk)
		if err != nil {
			return lastID, err
		}
		lastID = msg.ID
	}
	return lastID, nil
}

// lastNewline returns the index of the last newline in s, or -1.
func lastNewline(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '\n' {
			return i
		}
	}
	return -1
}

// Edit updates an existing message's content.
func (s *SessionSender) Edit(channelID, messageID, content string) error {
	_, err := s.session.ChannelMessageEdit(channelID, messageID, content)
	return err
}
