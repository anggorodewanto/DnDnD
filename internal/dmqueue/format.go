// Package dmqueue provides the DM notification framework that posts
// structured messages to #dm-queue, persists them for later reference,
// and supports cancel/resolve edits.
package dmqueue

import (
	"fmt"
	"strings"
)

// EventKind enumerates the event types that can be posted to #dm-queue.
type EventKind string

const (
	KindFreeformAction      EventKind = "freeform_action"
	KindReactionDeclaration EventKind = "reaction_declaration"
	KindRestRequest         EventKind = "rest_request"
	KindSkillCheckNarration EventKind = "skill_check_narration"
	KindConsumable          EventKind = "consumable"
	KindEnemyTurnReady      EventKind = "enemy_turn_ready"
	KindNarrativeTeleport   EventKind = "narrative_teleport"
	KindPlayerWhisper       EventKind = "player_whisper"
	// KindUndoRequest is a player-initiated request for the DM to undo
	// their last action. The dashboard's Phase 97b undo flow performs the
	// actual rollback; this event simply surfaces the request in #dm-queue.
	KindUndoRequest EventKind = "undo_request"
	// KindRetireRequest is a player-initiated request to retire their
	// character. The dashboard's approval handler calls registration.
	// Service.Retire when the DM accepts; this event surfaces the request.
	KindRetireRequest EventKind = "retire_request"
)

// WhisperTargetDiscordUserIDKey is the ExtraMetadata key under which the
// Discord user ID of a whisper's target is stashed. The DM dashboard reads
// this to deliver a reply via Discord DM when resolving a whisper item.
const WhisperTargetDiscordUserIDKey = "whisper_target_discord_user_id"

// Phase 106d: ExtraMetadata keys for KindSkillCheckNarration items. The
// /check handler stashes these so ResolveSkillCheckNarration can deliver a
// follow-up message in the originating Discord channel mentioning the
// invoking player along with the rolled total.
const (
	SkillCheckChannelIDKey       = "skill_check_channel_id"
	SkillCheckPlayerDiscordIDKey = "skill_check_player_discord_id"
	SkillCheckSkillLabelKey      = "skill_check_skill_label"
	SkillCheckTotalKey           = "skill_check_total"
	SkillCheckCharNameKey        = "skill_check_char_name"
)

// Event describes a structured dm-queue notification.
//
// PlayerName is the display name of the acting character/player.
// Summary is the event-specific context. For quoted freeform text callers
// should include the quotes themselves. For rest/check/item-usage events
// the Summary typically begins with a verb phrase ("uses Ball Bearings",
// "requests a short rest") so the formatter can render naturally without
// an extra colon.
type Event struct {
	Kind          EventKind
	PlayerName    string
	Summary       string
	ResolvePath   string // dashboard URL path, e.g. /dashboard/queue/{id}
	GuildID       string
	CampaignID    string
	ExtraMetadata map[string]string
}

// kindLabel describes how each event kind renders its emoji + label and
// whether the player name is followed by a colon (quoted summary) or a space.
type kindLabel struct {
	emoji    string
	label    string
	useColon bool // "Name: summary" vs "Name summary"
}

var kindLabels = map[EventKind]kindLabel{
	KindFreeformAction:      {emoji: "🎭", label: "Action", useColon: true},
	KindReactionDeclaration: {emoji: "⚡", label: "Reaction", useColon: true},
	KindRestRequest:         {emoji: "🛏️", label: "Rest", useColon: false},
	KindSkillCheckNarration: {emoji: "🎲", label: "Check", useColon: true},
	KindConsumable:          {emoji: "🧪", label: "Item", useColon: false},
	KindEnemyTurnReady:      {emoji: "⚔️", label: "Enemy Turn", useColon: false},
	KindNarrativeTeleport:   {emoji: "✨", label: "Spell", useColon: false},
	KindPlayerWhisper:       {emoji: "🤫", label: "Whisper", useColon: true},
	KindUndoRequest:         {emoji: "⏪", label: "Undo Request", useColon: false},
	KindRetireRequest:       {emoji: "🪦", label: "Retire Request", useColon: false},
}

var defaultLabel = kindLabel{emoji: "📨", label: "Notification", useColon: true}

// FormatEvent renders an Event as a Discord message string with a
// trailing "Resolve →" markdown link to the dashboard.
func FormatEvent(e Event) string {
	return formatBody(e) + " — " + formatResolveLink(e.ResolvePath)
}

// formatBody renders only the emoji/label/player/summary portion of the
// message, without the trailing Resolve link. Used for cancelled edits.
func formatBody(e Event) string {
	lbl, ok := kindLabels[e.Kind]
	if !ok {
		lbl = defaultLabel
	}
	sep := " "
	if lbl.useColon {
		sep = ": "
	}
	return fmt.Sprintf("%s **%s** — %s%s%s", lbl.emoji, lbl.label, e.PlayerName, sep, e.Summary)
}

// formatResolveLink renders the trailing dashboard link.
func formatResolveLink(path string) string {
	return "[Resolve →](" + path + ")"
}

// FormatCancelled transforms a posted message into a cancelled edit:
// strike-through the body and append " Cancelled by player". Any trailing
// " — [Resolve →](...)" link is removed.
func FormatCancelled(posted string) string {
	body := stripResolveLink(posted)
	return "~~" + body + "~~ Cancelled by player"
}

// FormatResolved transforms a posted message into a resolved edit:
// prepend ✅ and append " — <outcome>" (stripping the Resolve link).
func FormatResolved(posted, outcome string) string {
	body := stripResolveLink(posted)
	return "✅ " + body + " — " + outcome
}

// FormatSkillCheckNarrationFollowup renders the non-ephemeral follow-up
// message that surfaces a DM's skill check narration back into the
// originating channel. The message mentions the invoking player by Discord
// user ID, names the skill, includes the rolled total for log context, and
// then quotes the narration text.
//
// Required ExtraMetadata: SkillCheckPlayerDiscordIDKey, SkillCheckSkillLabelKey,
// SkillCheckTotalKey. Missing values are tolerated (the corresponding
// segment is omitted) so the formatter never panics on partial events.
func FormatSkillCheckNarrationFollowup(e Event, narration string) string {
	userID := e.ExtraMetadata[SkillCheckPlayerDiscordIDKey]
	skill := e.ExtraMetadata[SkillCheckSkillLabelKey]
	total := e.ExtraMetadata[SkillCheckTotalKey]

	var b strings.Builder
	if userID != "" {
		b.WriteString("<@")
		b.WriteString(userID)
		b.WriteString("> ")
	}
	if skill != "" {
		b.WriteString("**")
		b.WriteString(skill)
		b.WriteString(" Check**")
		if total != "" {
			b.WriteString(" (rolled ")
			b.WriteString(total)
			b.WriteString(")")
		}
		b.WriteString("\n")
	} else if total != "" {
		b.WriteString("**Check** (rolled ")
		b.WriteString(total)
		b.WriteString(")\n")
	}
	b.WriteString(narration)
	return b.String()
}

// stripResolveLink removes a trailing " — [Resolve →](...)" from a posted message.
func stripResolveLink(s string) string {
	idx := strings.LastIndex(s, " — [Resolve →](")
	if idx < 0 {
		return s
	}
	return s[:idx]
}
