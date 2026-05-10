package discord

import (
	"context"

	"github.com/google/uuid"
)

// postCombatLogChannel mirrors a combat log line to the encounter's
// #combat-log channel. Centralized so /attack, /bonus, /shove,
// /interact, and /deathsave handlers all use the same shape:
//
//   - csp == nil OR msg == ""  => no-op
//   - csp returns an error     => silently skipped (best-effort)
//   - no combat-log channel    => silently skipped
//   - send error               => silently swallowed (player still
//     receives the ephemeral via the caller)
//
// Returns the channel ID that was posted to, or "" when the post was
// skipped — callers ignore the return today; tests that need to assert
// "did we attempt the post" can read it.
func postCombatLogChannel(ctx context.Context, sess Session, csp CampaignSettingsProvider, encounterID uuid.UUID, msg string) string {
	if csp == nil || msg == "" {
		return ""
	}
	channelIDs, err := csp.GetChannelIDs(ctx, encounterID)
	if err != nil {
		return ""
	}
	combatLogCh, ok := channelIDs["combat-log"]
	if !ok || combatLogCh == "" {
		return ""
	}
	_, _ = sess.ChannelMessageSend(combatLogCh, msg)
	return combatLogCh
}
