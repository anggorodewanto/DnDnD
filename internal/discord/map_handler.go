package discord

import (
	"context"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

// MapEncounterProvider resolves the invoker's active encounter so /map posts
// the board for the right encounter when two simultaneous encounters share a
// #combat-map channel.
type MapEncounterProvider interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
}

// CombatMapPoster renders + posts the current battle-map to #combat-map for an
// encounter. Satisfied by *DiscordCombatMapNotifier.
type CombatMapPoster interface {
	PostCombatMap(ctx context.Context, encounterID uuid.UUID)
}

// MapHandler handles the /map slash command: an on-demand re-post of the
// current combat board to #combat-map. The board otherwise appears only at
// combat start and after each turn, so any player who scrolled past it (or
// joined late) can summon a fresh copy. Anyone in the active encounter may
// invoke it.
type MapHandler struct {
	session           Session
	encounterProvider MapEncounterProvider
	mapPoster         CombatMapPoster
	notifyAsync       func(func())
}

// NewMapHandler creates a MapHandler.
func NewMapHandler(session Session, encounterProvider MapEncounterProvider, mapPoster CombatMapPoster) *MapHandler {
	return &MapHandler{
		session:           session,
		encounterProvider: encounterProvider,
		mapPoster:         mapPoster,
	}
}

// SetNotifyDispatcher wires the dispatcher used to render + post the map off
// the interaction-response path (a large / Tiled-asset PNG can exceed
// Discord's 3-second ACK window). In production this is `go f()`; when unset
// (tests) the post runs inline so channel-post assertions stay deterministic.
func (h *MapHandler) SetNotifyDispatcher(dispatch func(func())) {
	h.notifyAsync = dispatch
}

// Handle processes the /map command interaction.
func (h *MapHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	encounterID, err := h.encounterProvider.ActiveEncounterForUser(ctx, interaction.GuildID, discordUserID(interaction))
	if err != nil {
		respondEphemeral(h.session, interaction, "No active combat right now — there's no battle-map to show.")
		return
	}

	// Acknowledge within Discord's 3-second window, then render + post the PNG
	// to #combat-map asynchronously (the render can exceed that window).
	respondEphemeral(h.session, interaction, "📍 Posting the current battle-map to #combat-map…")
	h.dispatchNotify(func() {
		h.mapPoster.PostCombatMap(ctx, encounterID)
	})
}

// dispatchNotify runs f asynchronously when a dispatcher is wired (production),
// else inline (tests / headless), per SetNotifyDispatcher.
func (h *MapHandler) dispatchNotify(f func()) {
	if h.notifyAsync != nil {
		h.notifyAsync(f)
		return
	}
	f()
}
