package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

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

// SendASIPrompt opens a DM channel with the target Discord user and sends the
// ASI/Feat choice prompt WITH the interactive buttons (+2 / +1+1 / feat). The
// buttons embed the character ID in their custom IDs (asi_choice:<charID>:...)
// so the router can route the player's click back to the right character. This
// is the path the level-up flow relies on to make the choice actionable; a
// plain-text DM would render no buttons and leave the player stuck.
func (d *DirectMessenger) SendASIPrompt(discordUserID string, characterID uuid.UUID, body string) ([]string, error) {
	ch, err := d.session.UserChannelCreate(discordUserID)
	if err != nil {
		return nil, fmt.Errorf("creating DM channel for user %s: %w", discordUserID, err)
	}
	msg, err := d.session.ChannelMessageSendComplex(ch.ID, &discordgo.MessageSend{
		Content:    body,
		Components: BuildASIPromptComponents(characterID),
	})
	if err != nil {
		return nil, fmt.Errorf("sending ASI prompt to user %s: %w", discordUserID, err)
	}
	return []string{msg.ID}, nil
}
