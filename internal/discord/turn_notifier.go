package discord

// TurnTimerNotifier adapts a Session to the combat.Notifier interface
// (SendMessage(channelID, content) error). It is a thin forwarder to
// ChannelMessageSend on the injected Session, so the combat package can
// stay independent of discordgo.
type TurnTimerNotifier struct {
	session Session
}

// NewTurnTimerNotifier constructs a TurnTimerNotifier wrapping session.
func NewTurnTimerNotifier(session Session) *TurnTimerNotifier {
	return &TurnTimerNotifier{session: session}
}

// SendMessage posts content to channelID via the Discord session. It returns
// any error from the underlying Discord API call.
func (n *TurnTimerNotifier) SendMessage(channelID, content string) error {
	_, err := n.session.ChannelMessageSend(channelID, content)
	return err
}
