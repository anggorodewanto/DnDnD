package campaign

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// Campaign status constants.
const (
	StatusActive   = "active"
	StatusPaused   = "paused"
	StatusArchived = "archived"
)

// Settings represents the JSONB campaign settings.
type Settings struct {
	TurnTimeoutHours  int               `json:"turn_timeout_hours"`
	DiagonalRule      string            `json:"diagonal_rule"`
	Open5eSources     []string          `json:"open5e_sources,omitempty"`
	ChannelIDs        map[string]string `json:"channel_ids,omitempty"`
	PotionBonusAction bool              `json:"potion_bonus_action,omitempty"`
}

// DefaultSettings returns campaign settings with sensible defaults.
func DefaultSettings() Settings {
	return Settings{
		TurnTimeoutHours: 24,
		DiagonalRule:     "standard",
	}
}

// Store defines the database operations needed by the campaign service.
type Store interface {
	CreateCampaign(ctx context.Context, arg refdata.CreateCampaignParams) (refdata.Campaign, error)
	GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error)
	GetCampaignByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)
	UpdateCampaignStatus(ctx context.Context, arg refdata.UpdateCampaignStatusParams) (refdata.Campaign, error)
	UpdateCampaignSettings(ctx context.Context, arg refdata.UpdateCampaignSettingsParams) (refdata.Campaign, error)
	UpdateCampaignName(ctx context.Context, arg refdata.UpdateCampaignNameParams) (refdata.Campaign, error)
	ListCampaigns(ctx context.Context) ([]refdata.Campaign, error)
}

// Announcer posts messages to a guild's #the-story channel.
type Announcer interface {
	AnnounceToStory(guildID, message string) error
}

// Service manages campaign CRUD and status transitions.
type Service struct {
	store     Store
	announcer Announcer
}

// NewService creates a new campaign Service.
func NewService(store Store, announcer Announcer) *Service {
	return &Service{store: store, announcer: announcer}
}

// CreateCampaign validates input and creates a new campaign.
// If settings is nil, default settings are used.
func (s *Service) CreateCampaign(ctx context.Context, guildID, dmUserID, name string, settings *Settings) (refdata.Campaign, error) {
	if guildID == "" {
		return refdata.Campaign{}, fmt.Errorf("guild_id must not be empty")
	}
	if dmUserID == "" {
		return refdata.Campaign{}, fmt.Errorf("dm_user_id must not be empty")
	}
	if name == "" {
		return refdata.Campaign{}, fmt.Errorf("name must not be empty")
	}

	if settings == nil {
		d := DefaultSettings()
		settings = &d
	}

	settingsJSON, err := settingsToNullRawMessage(settings)
	if err != nil {
		return refdata.Campaign{}, fmt.Errorf("marshaling settings: %w", err)
	}

	return s.store.CreateCampaign(ctx, refdata.CreateCampaignParams{
		GuildID:  guildID,
		DmUserID: dmUserID,
		Name:     name,
		Settings: settingsJSON,
	})
}

// GetByGuildID retrieves a campaign by its guild ID.
func (s *Service) GetByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error) {
	if guildID == "" {
		return refdata.Campaign{}, fmt.Errorf("guild_id must not be empty")
	}
	return s.store.GetCampaignByGuildID(ctx, guildID)
}

// GetByID retrieves a campaign by its ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
	return s.store.GetCampaignByID(ctx, id)
}

// PauseCampaign transitions a campaign from active to paused and announces to Discord.
func (s *Service) PauseCampaign(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
	return s.transitionStatus(ctx, id, StatusPaused, "The campaign has been **paused**. See you soon, adventurers!")
}

// ResumeCampaign transitions a campaign from paused to active and announces to Discord.
func (s *Service) ResumeCampaign(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
	return s.transitionStatus(ctx, id, StatusActive, "The campaign has been **resumed**! The adventure continues!")
}

// ArchiveCampaign transitions a campaign to archived status.
func (s *Service) ArchiveCampaign(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
	return s.transitionStatus(ctx, id, StatusArchived, "")
}

// transitionStatus handles the shared fetch-validate-update-announce pattern for status changes.
// An empty announcement string skips the announcement step.
func (s *Service) transitionStatus(ctx context.Context, id uuid.UUID, target, announcement string) (refdata.Campaign, error) {
	c, err := s.store.GetCampaignByID(ctx, id)
	if err != nil {
		return refdata.Campaign{}, fmt.Errorf("fetching campaign: %w", err)
	}
	if c.Status == target {
		return refdata.Campaign{}, fmt.Errorf("campaign is already %s", target)
	}
	if c.Status == StatusArchived {
		return refdata.Campaign{}, fmt.Errorf("cannot transition an archived campaign")
	}

	updated, err := s.store.UpdateCampaignStatus(ctx, refdata.UpdateCampaignStatusParams{
		ID:     id,
		Status: target,
	})
	if err != nil {
		return refdata.Campaign{}, fmt.Errorf("updating status: %w", err)
	}

	// Announcement is best-effort; failure doesn't block the transition.
	if s.announcer != nil && announcement != "" {
		_ = s.announcer.AnnounceToStory(c.GuildID, announcement)
	}

	return updated, nil
}

// UpdateSettings updates the campaign's settings JSONB.
func (s *Service) UpdateSettings(ctx context.Context, id uuid.UUID, settings *Settings) (refdata.Campaign, error) {
	if settings == nil {
		return refdata.Campaign{}, fmt.Errorf("settings must not be nil")
	}

	settingsJSON, err := settingsToNullRawMessage(settings)
	if err != nil {
		return refdata.Campaign{}, fmt.Errorf("marshaling settings: %w", err)
	}

	return s.store.UpdateCampaignSettings(ctx, refdata.UpdateCampaignSettingsParams{
		ID:       id,
		Settings: settingsJSON,
	})
}

// UpdateName updates the campaign's name.
func (s *Service) UpdateName(ctx context.Context, id uuid.UUID, name string) (refdata.Campaign, error) {
	if name == "" {
		return refdata.Campaign{}, fmt.Errorf("name must not be empty")
	}
	return s.store.UpdateCampaignName(ctx, refdata.UpdateCampaignNameParams{
		ID:   id,
		Name: name,
	})
}

// ListCampaigns returns all campaigns (admin/debug).
func (s *Service) ListCampaigns(ctx context.Context) ([]refdata.Campaign, error) {
	return s.store.ListCampaigns(ctx)
}

// settingsToNullRawMessage marshals settings to pqtype.NullRawMessage.
func settingsToNullRawMessage(s *Settings) (pqtype.NullRawMessage, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return pqtype.NullRawMessage{}, err
	}
	return pqtype.NullRawMessage{RawMessage: data, Valid: true}, nil
}
