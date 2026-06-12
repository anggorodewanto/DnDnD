package discord

import (
	"context"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/ab/dndnd/internal/dmqueue"
)

const (
	// dmQueueResolveModalPrefix is the modal custom-ID prefix; the itemID
	// follows it. Distinct from dmqueue.ResolveButtonCustomIDPrefix (the
	// button) so the two never collide in routing.
	dmQueueResolveModalPrefix = "dmqueue_resolve_modal:"
	// dmQueueResolveInputID is the text-input field within the resolve modal.
	dmQueueResolveInputID = "dmqueue_resolve_outcome"
)

// DMQueueResolveHandler turns the [✅ Resolve] button on a #dm-queue message
// into a text-input modal that captures the DM's outcome/narration/reply,
// then dispatches to the resolve path that matches the item's kind. This lets
// a Discord-only DM resolve queue items without opening the dashboard (T46).
type DMQueueResolveHandler struct {
	session  Session
	notifier dmqueue.Notifier
}

// NewDMQueueResolveHandler constructs a DMQueueResolveHandler.
func NewDMQueueResolveHandler(session Session, notifier dmqueue.Notifier) *DMQueueResolveHandler {
	return &DMQueueResolveHandler{session: session, notifier: notifier}
}

// ShowResolveModal opens the resolve modal for a pending item, or replies
// ephemerally when the item is gone or already handled.
func (h *DMQueueResolveHandler) ShowResolveModal(interaction *discordgo.Interaction, itemID string) {
	item, ok := h.notifier.Get(itemID)
	if !ok {
		respondEphemeral(h.session, interaction, "That queue item is no longer available.")
		return
	}
	if item.Status != dmqueue.StatusPending {
		respondEphemeral(h.session, interaction, "That queue item has already been handled.")
		return
	}

	label, placeholder := resolveModalCopy(item.Event.Kind)
	_ = h.session.InteractionRespond(interaction, textInputModalFrom(
		dmQueueResolveModalPrefix+itemID, "Resolve Queue Item",
		discordgo.TextInput{
			CustomID:    dmQueueResolveInputID,
			Label:       label,
			Style:       discordgo.TextInputParagraph,
			Placeholder: placeholder,
			Required:    true,
			MaxLength:   1000,
		},
	))
}

// HandleResolveSubmit reads the modal text and resolves the item along the
// path matching its kind: whisper reply, skill-check narration, or a plain
// outcome. Empty input is rejected before any state change.
func (h *DMQueueResolveHandler) HandleResolveSubmit(interaction *discordgo.Interaction, itemID string) {
	text := strings.TrimSpace(modalTextValue(interaction, dmQueueResolveInputID))
	if text == "" {
		respondEphemeral(h.session, interaction, "Resolution text is required.")
		return
	}

	item, ok := h.notifier.Get(itemID)
	if !ok {
		respondEphemeral(h.session, interaction, "That queue item is no longer available.")
		return
	}

	ctx := context.Background()
	var err error
	switch item.Event.Kind {
	case dmqueue.KindPlayerWhisper:
		err = h.notifier.ResolveWhisper(ctx, itemID, text)
	case dmqueue.KindSkillCheckNarration:
		err = h.notifier.ResolveSkillCheckNarration(ctx, itemID, text)
	default:
		err = h.notifier.Resolve(ctx, itemID, text)
	}
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not resolve: "+err.Error())
		return
	}
	respondEphemeral(h.session, interaction, "✅ Resolved.")
}

// resolveModalCopy tailors the modal's input label/placeholder to the item
// kind so the DM knows where their text goes.
func resolveModalCopy(kind dmqueue.EventKind) (label, placeholder string) {
	switch kind {
	case dmqueue.KindPlayerWhisper:
		return "Reply (DM'd to the player)", "Your whispered reply…"
	case dmqueue.KindSkillCheckNarration:
		return "Narration (posted to the player's channel)", "Describe what they notice…"
	default:
		return "Outcome", "How does this resolve?"
	}
}
