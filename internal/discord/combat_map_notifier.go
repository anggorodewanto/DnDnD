package discord

import (
	"context"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// CombatMapEncounterLookup fetches the encounter row so the combat-map
// notifier can derive the Phase 105 "⚔️ <name> — Round N" label that lets
// players in a shared #combat-map tell simultaneous encounters apart.
type CombatMapEncounterLookup interface {
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
}

// DiscordCombatMapNotifier renders the current battle-map and posts it to
// #combat-map. It backs two paths:
//   - combat.Service's start-of-combat post (so the opening board appears
//     before turn 1 — see combat.CombatMapNotifier), and
//   - the /map slash command (an on-demand re-post for whoever asks).
//
// Best-effort throughout: a missing settings provider / #combat-map channel /
// regenerator is a silent no-op, and a render *failure* is logged and (when a
// CombatMapRenderFailureNotifier is wired) surfaced to the DM — both handled
// inside the package-level PostCombatMap.
type DiscordCombatMapNotifier struct {
	session                  Session
	campaignSettingsProvider CampaignSettingsProvider
	mapRegenerator           MapRegenerator
	encounterLookup          CombatMapEncounterLookup
	mapRenderFailNotifier    CombatMapRenderFailureNotifier
}

// NewDiscordCombatMapNotifier creates a DiscordCombatMapNotifier. The lookup
// may be nil (the map is then posted without the "⚔️ <name> — Round N" label).
func NewDiscordCombatMapNotifier(session Session, csp CampaignSettingsProvider, mr MapRegenerator, lookup CombatMapEncounterLookup) *DiscordCombatMapNotifier {
	return &DiscordCombatMapNotifier{
		session:                  session,
		campaignSettingsProvider: csp,
		mapRegenerator:           mr,
		encounterLookup:          lookup,
	}
}

// SetMapRenderFailureNotifier wires the optional notifier told when a
// combat-map PNG render fails, so the DM learns players may be acting on a
// stale (or missing) board. (T09 / Finding 6f)
func (n *DiscordCombatMapNotifier) SetMapRenderFailureNotifier(fn CombatMapRenderFailureNotifier) {
	n.mapRenderFailNotifier = fn
}

// PostCombatMap resolves the encounter's channel IDs + label and posts the
// regenerated battle-map to #combat-map. Best-effort: a nil settings provider
// or a channel-id lookup error is a silent no-op.
func (n *DiscordCombatMapNotifier) PostCombatMap(ctx context.Context, encounterID uuid.UUID) {
	if n.campaignSettingsProvider == nil {
		return
	}
	channelIDs, err := n.campaignSettingsProvider.GetChannelIDs(ctx, encounterID)
	if err != nil {
		return
	}
	PostCombatMap(ctx, n.session, n.mapRegenerator, encounterID, channelIDs, n.encounterLabel(ctx, encounterID), n.mapRenderFailNotifier)
}

// encounterLabel returns the "⚔️ <name> — Round N" prefix for the encounter,
// or "" when the lookup is unset or returns an error.
func (n *DiscordCombatMapNotifier) encounterLabel(ctx context.Context, encounterID uuid.UUID) string {
	if n.encounterLookup == nil {
		return ""
	}
	enc, err := n.encounterLookup.GetEncounter(ctx, encounterID)
	if err != nil {
		return ""
	}
	return combat.EncounterLabel(enc)
}
