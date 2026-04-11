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
	Kind        EventKind
	PlayerName  string
	Summary     string
	ResolvePath string // dashboard URL path, e.g. /dashboard/queue/{id}
	GuildID     string
	CampaignID  string
	ExtraMetadata map[string]string
}

// kindLabel describes how each event kind renders its emoji + label and
// whether the player name is followed by a colon (quoted summary) or a space.
type kindLabel struct {
	emoji   string
	label   string
	useColon bool // "Name: summary" vs "Name summary"
}

var kindLabels = map[EventKind]kindLabel{
	KindFreeformAction:      {emoji: "🎭", label: "Action", useColon: true},
	KindReactionDeclaration: {emoji: "⚡", label: "Reaction", useColon: true},
	KindRestRequest:         {emoji: "🛏️", label: "Rest", useColon: false},
	KindSkillCheckNarration: {emoji: "🎲", label: "Check", useColon: true},
	KindConsumable:          {emoji: "🧪", label: "Item", useColon: false},
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
	return fmt.Sprintf("[Resolve →](%s)", path)
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

// stripResolveLink removes a trailing " — [Resolve →](...)" from a posted message.
func stripResolveLink(s string) string {
	idx := strings.LastIndex(s, " — [Resolve →](")
	if idx < 0 {
		return s
	}
	return s[:idx]
}
