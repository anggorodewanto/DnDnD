package discord

import (
	"context"

	"github.com/google/uuid"
)

// DMCorrectionPoster implements combat.CombatLogPoster by posting DM correction
// messages to the campaign's #combat-log Discord channel. Best-effort: failures
// are silently ignored to avoid blocking dashboard operations.
//
// Per the Phase 97b spec, original combat-log messages are NEVER edited or
// deleted; corrections are appended as new messages.
type DMCorrectionPoster struct {
	session                  Session
	campaignSettingsProvider CampaignSettingsProvider
}

// NewDMCorrectionPoster creates a poster wired to a Discord session and
// campaign settings provider for channel lookup.
func NewDMCorrectionPoster(session Session, csp CampaignSettingsProvider) *DMCorrectionPoster {
	return &DMCorrectionPoster{session: session, campaignSettingsProvider: csp}
}

// PostCorrection posts the message to #combat-log for the campaign tied to encounterID.
func (p *DMCorrectionPoster) PostCorrection(ctx context.Context, encounterID uuid.UUID, message string) {
	if p.campaignSettingsProvider == nil || message == "" {
		return
	}
	channelIDs, err := p.campaignSettingsProvider.GetChannelIDs(ctx, encounterID)
	if err != nil {
		return
	}
	combatLogCh, ok := channelIDs["combat-log"]
	if !ok || combatLogCh == "" {
		return
	}
	_ = SendContent(p.session, combatLogCh, message)
}
