package discord

import (
	"context"

	"github.com/google/uuid"
)

// DiscordEnemyTurnNotifier posts combat log and regenerated map to Discord
// after an enemy turn is executed from the dashboard.
type DiscordEnemyTurnNotifier struct {
	session                  Session
	campaignSettingsProvider CampaignSettingsProvider
	mapRegenerator           MapRegenerator
}

// NewDiscordEnemyTurnNotifier creates a DiscordEnemyTurnNotifier.
func NewDiscordEnemyTurnNotifier(session Session, csp CampaignSettingsProvider, mr MapRegenerator) *DiscordEnemyTurnNotifier {
	return &DiscordEnemyTurnNotifier{
		session:                  session,
		campaignSettingsProvider: csp,
		mapRegenerator:           mr,
	}
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

	// Post combat log to #combat-log (handles message splitting for long logs)
	if combatLogCh, ok := channelIDs["combat-log"]; ok && combatLogCh != "" && combatLog != "" {
		_ = SendContent(n.session, combatLogCh, combatLog)
	}

	// Regenerate and post map to #combat-map (shared helper)
	PostCombatMap(ctx, n.session, n.mapRegenerator, encounterID, channelIDs)
}
