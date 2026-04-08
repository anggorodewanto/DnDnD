package discord

import "fmt"

// DirectMessenger sends Discord DMs to a user via the bot session. It
// implements the messageplayer.Messenger interface so the dashboard can
// deliver DM-to-player messages (Phase 101).
type DirectMessenger struct {
	session Session
}

// NewDirectMessenger constructs a DirectMessenger wrapping a bot session.
func NewDirectMessenger(session Session) *DirectMessenger {
	return &DirectMessenger{session: session}
}

// SendDirectMessage opens a DM channel with the target Discord user and
// sends the body, returning the message IDs of the resulting message(s).
// Long bodies are split via SendContentReturningIDs so multi-chunk sends
// still return a single flat list of IDs.
func (d *DirectMessenger) SendDirectMessage(discordUserID, body string) ([]string, error) {
	ch, err := d.session.UserChannelCreate(discordUserID)
	if err != nil {
		return nil, fmt.Errorf("creating DM channel for user %s: %w", discordUserID, err)
	}
	ids, err := SendContentReturningIDs(d.session, ch.ID, body)
	if err != nil {
		return nil, fmt.Errorf("sending direct message to user %s: %w", discordUserID, err)
	}
	return ids, nil
}
