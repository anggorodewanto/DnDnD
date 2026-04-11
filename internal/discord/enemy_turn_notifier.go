package discord

import (
	"context"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// EnemyTurnEncounterLookup fetches the encounter row so the notifier can
// prepend the Phase 105 simultaneous-encounter label ("⚔️ <name> — Round N")
// to messages posted into shared channels.
type EnemyTurnEncounterLookup interface {
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
}

// DiscordEnemyTurnNotifier posts combat log and regenerated map to Discord
// after an enemy turn is executed from the dashboard.
type DiscordEnemyTurnNotifier struct {
	session                  Session
	campaignSettingsProvider CampaignSettingsProvider
	mapRegenerator           MapRegenerator
	encounterLookup          EnemyTurnEncounterLookup
}

// NewDiscordEnemyTurnNotifier creates a DiscordEnemyTurnNotifier.
func NewDiscordEnemyTurnNotifier(session Session, csp CampaignSettingsProvider, mr MapRegenerator) *DiscordEnemyTurnNotifier {
	return &DiscordEnemyTurnNotifier{
		session:                  session,
		campaignSettingsProvider: csp,
		mapRegenerator:           mr,
	}
}

// SetEncounterLookup wires the encounter lookup used to derive the Phase 105
// "⚔️ <encounter> — Round N" label. When not set, messages are posted without
// a label (matching pre-Phase-105 behavior).
func (n *DiscordEnemyTurnNotifier) SetEncounterLookup(lookup EnemyTurnEncounterLookup) {
	n.encounterLookup = lookup
}

// NotifyEnemyTurnExecuted posts the combat log to #combat-log and regenerates
// the map to #combat-map. Best-effort: failures are silently ignored.
func (n *DiscordEnemyTurnNotifier) NotifyEnemyTurnExecuted(ctx context.Context, encounterID uuid.UUID, combatLog string) {
	if n.campaignSettingsProvider == nil {
		return
	}

	channelIDs, err := n.campaignSettingsProvider.GetChannelIDs(ctx, encounterID)
	if err != nil {
		return
	}

	label := n.encounterLabel(ctx, encounterID)

	// Post combat log to #combat-log (handles message splitting for long logs)
	if combatLogCh, ok := channelIDs["combat-log"]; ok && combatLogCh != "" && combatLog != "" {
		content := combatLog
		if label != "" {
			content = label + "\n" + combatLog
		}
		_ = SendContent(n.session, combatLogCh, content)
	}

	// Regenerate and post map to #combat-map with the same label so
	// simultaneous encounters sharing the channel are distinguishable.
	PostCombatMap(ctx, n.session, n.mapRegenerator, encounterID, channelIDs, label)
}

// encounterLabel returns the "⚔️ <name> — Round N" prefix for the encounter,
// or "" if the lookup is unset or returns an error.
func (n *DiscordEnemyTurnNotifier) encounterLabel(ctx context.Context, encounterID uuid.UUID) string {
	if n.encounterLookup == nil {
		return ""
	}
	enc, err := n.encounterLookup.GetEncounter(ctx, encounterID)
	if err != nil {
		return ""
	}
	return combat.EncounterLabel(enc)
}
