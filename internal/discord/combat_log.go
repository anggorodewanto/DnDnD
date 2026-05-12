package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
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

// FormatZoneTriggerResults renders a slice of zone trigger results as a
// combat-log block (one line per trigger). Returns "" when results is
// empty so callers can safely concatenate without producing blank lines.
//
// E-67-followup: turn-start / on-enter zone effects (Spirit Guardians,
// Wall of Fire, Cloud of Daggers, Moonbeam) need to surface in #combat-log
// for DM-driven damage / save resolution. The combat side carries
// Effect="damage" / "save" plus the source spell ID so the DM knows which
// spell's Damage / SaveAbility / SaveEffect to apply. Per-round dedupe
// happens inside Service.CheckZoneTriggers.
func FormatZoneTriggerResults(combatantName string, results []combat.ZoneTriggerResult) string {
	if len(results) == 0 {
		return ""
	}
	var lines []string
	header := fmt.Sprintf("\U0001f4a5 Zone effects affecting **%s**:", combatantName)
	lines = append(lines, header)
	for _, r := range results {
		lines = append(lines, formatZoneTriggerLine(r))
	}
	return strings.Join(lines, "\n")
}

// formatZoneTriggerLine produces a single bullet line summarising one zone
// trigger result. The format names the source spell, the effect (damage /
// save), and the trigger event so DMs can resolve the prompt without
// guessing context.
func formatZoneTriggerLine(r combat.ZoneTriggerResult) string {
	source := r.SourceSpell
	if source == "" {
		source = r.ZoneType
	}
	switch r.Effect {
	case "damage":
		return fmt.Sprintf("- %s (%s) — DM: roll damage", source, r.Trigger)
	case "save":
		return fmt.Sprintf("- %s (%s) — DM: prompt save", source, r.Trigger)
	default:
		return fmt.Sprintf("- %s (%s) — DM: resolve %s", source, r.Trigger, r.Effect)
	}
}

// PostZoneTriggerResultsToCombatLog mirrors postCombatLogChannel for the
// E-67 zone-trigger surface. Callers in the turn-advance flow pass the
// just-advanced combatant + the TurnInfo.ZoneTriggerResults slice; this
// helper formats and posts to #combat-log when wired. nil-safe / silent
// no-op when either CSP or results are empty.
func PostZoneTriggerResultsToCombatLog(
	ctx context.Context,
	sess Session,
	csp CampaignSettingsProvider,
	encounterID uuid.UUID,
	combatant refdata.Combatant,
	results []combat.ZoneTriggerResult,
) string {
	if len(results) == 0 {
		return ""
	}
	msg := FormatZoneTriggerResults(combatant.DisplayName, results)
	if msg == "" {
		return ""
	}
	return postCombatLogChannel(ctx, sess, csp, encounterID, msg)
}
