package open5e

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// CampaignSettingsReader is the minimal read surface needed to resolve
// a campaign's open5e_sources list from the campaigns table settings
// JSONB column. refdata.Queries satisfies this directly.
type CampaignSettingsReader interface {
	GetCampaignByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)
}

// CampaignSourceLookup resolves a campaign id to its enabled Open5e
// document slugs by reading the campaign's settings JSONB. It implements
// statblocklibrary.CampaignLookup.
type CampaignSourceLookup struct {
	reader CampaignSettingsReader
	logger *slog.Logger
}

// NewCampaignSourceLookup constructs a lookup backed by the given reader.
func NewCampaignSourceLookup(reader CampaignSettingsReader) *CampaignSourceLookup {
	return &CampaignSourceLookup{reader: reader, logger: slog.Default()}
}

// settingsShape matches just the JSON shape we need — any other fields in
// campaigns.settings are ignored.
type settingsShape struct {
	Open5eSources []string `json:"open5e_sources,omitempty"`
}

// EnabledOpen5eSources returns the slugs of Open5e documents the campaign
// has enabled. Returns nil for zero ids, unknown campaigns, or malformed
// settings JSON (errors are logged).
func (l *CampaignSourceLookup) EnabledOpen5eSources(campaignID uuid.UUID) []string {
	if campaignID == uuid.Nil {
		return nil
	}
	c, err := l.reader.GetCampaignByID(context.Background(), campaignID)
	if err != nil {
		l.logger.Warn("open5e: failed to load campaign for source lookup", "campaign_id", campaignID, "error", err)
		return nil
	}
	return parseOpen5eSources(c.Settings, campaignID, l.logger)
}

// parseOpen5eSources decodes the settings JSONB field. Returns nil for
// null/empty settings.
func parseOpen5eSources(settings pqtype.NullRawMessage, campaignID uuid.UUID, logger *slog.Logger) []string {
	if !settings.Valid || len(settings.RawMessage) == 0 {
		return nil
	}
	var s settingsShape
	if err := json.Unmarshal(settings.RawMessage, &s); err != nil {
		logger.Warn("open5e: malformed campaign settings", "campaign_id", campaignID, "error", err)
		return nil
	}
	return s.Open5eSources
}
