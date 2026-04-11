package discord

import "fmt"

// SkillCheckNarrationDeliverer posts a non-ephemeral follow-up message
// directly to a Discord channel via the bot session. It satisfies the
// dmqueue.SkillCheckNarrationDeliverer shape so DefaultNotifier can deliver
// /check narration follow-ups when the dashboard resolves a queue item.
//
// Phase 106d: kept as a thin wrapper alongside DirectMessenger so /whisper's
// DM-based delivery and /check's channel-based delivery do not interfere.
type SkillCheckNarrationDeliverer struct {
	session Session
}

// NewSkillCheckNarrationDeliverer constructs a deliverer wrapping a session.
func NewSkillCheckNarrationDeliverer(session Session) *SkillCheckNarrationDeliverer {
	return &SkillCheckNarrationDeliverer{session: session}
}

// DeliverSkillCheckNarration sends body as a regular (non-ephemeral) message
// to channelID. Long bodies are split via SendContentReturningIDs so a
// single send call still returns once all chunks are flushed.
func (d *SkillCheckNarrationDeliverer) DeliverSkillCheckNarration(channelID, body string) error {
	if channelID == "" {
		return fmt.Errorf("skill check narration: empty channel id")
	}
	if _, err := SendContentReturningIDs(d.session, channelID, body); err != nil {
		return fmt.Errorf("sending skill check narration to channel %s: %w", channelID, err)
	}
	return nil
}
