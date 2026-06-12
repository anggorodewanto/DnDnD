package dmqueue

import "github.com/bwmarrin/discordgo"

// sessionAPI is the minimal subset of discordgo.Session used by SessionSender.
// Mirrors the methods of internal/discord.Session we need.
type sessionAPI interface {
	ChannelMessageSend(channelID, content string) (*discordgo.Message, error)
	ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error)
	ChannelMessageEdit(channelID, messageID, content string) (*discordgo.Message, error)
	ChannelMessageEditComplex(m *discordgo.MessageEdit) (*discordgo.Message, error)
}

// ResolveButtonCustomIDPrefix is the custom-ID prefix carried by the [✅
// Resolve] button attached to every #dm-queue message. The itemID follows
// the prefix. The discord package routes clicks with this prefix to the
// resolve-modal flow. Defined here (not in discord) because discord imports
// dmqueue, not the reverse.
const ResolveButtonCustomIDPrefix = "dmqueue_resolve:"

// ComponentSender is an optional Sender capability: posting a message with
// Discord message components (buttons). DefaultNotifier.Post uses it when the
// concrete sender implements it, falling back to plain Send otherwise — so
// existing Sender fakes that don't need buttons keep working unchanged.
type ComponentSender interface {
	SendWithComponents(channelID, content string, components []discordgo.MessageComponent) (messageID string, err error)
}

// ComponentEditor is an optional Sender capability: editing a message's
// content together with its components. DefaultNotifier.Resolve/Cancel use it
// to strip the [✅ Resolve] button once an item is handled — by editing with
// an empty component set — falling back to plain content-only Edit when the
// concrete sender does not implement it, so existing Sender fakes keep working
// unchanged.
type ComponentEditor interface {
	EditWithComponents(channelID, messageID, content string, components []discordgo.MessageComponent) error
}

// resolveButtonComponents builds the single-row [✅ Resolve] action attached
// to a dm-queue message, encoding the itemID in the button custom ID.
func resolveButtonComponents(itemID string) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Resolve",
				Style:    discordgo.SuccessButton,
				CustomID: ResolveButtonCustomIDPrefix + itemID,
				Emoji:    &discordgo.ComponentEmoji{Name: "✅"},
			},
		}},
	}
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

// SendWithComponents posts a message carrying Discord components (buttons)
// via ChannelMessageSendComplex and returns the created message's ID. Unlike
// Send it does not split: dm-queue messages are a single short line, well
// under the 2000-char limit.
func (s *SessionSender) SendWithComponents(channelID, content string, components []discordgo.MessageComponent) (string, error) {
	msg, err := s.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content:    content,
		Components: components,
	})
	if err != nil {
		return "", err
	}
	return msg.ID, nil
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

// EditWithComponents updates an existing message's content and components via
// ChannelMessageEditComplex. Passing an empty (non-nil) components slice
// strips any attached buttons: discordgo removes components only when handed a
// non-nil pointer to an empty slice (a nil pointer leaves them untouched),
// which this method guarantees even if the caller passes a nil slice.
func (s *SessionSender) EditWithComponents(channelID, messageID, content string, components []discordgo.MessageComponent) error {
	if components == nil {
		components = []discordgo.MessageComponent{}
	}
	_, err := s.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:    channelID,
		ID:         messageID,
		Content:    &content,
		Components: &components,
	})
	return err
}
