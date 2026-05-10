package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/discord"
	"github.com/ab/dndnd/internal/refdata"
)

// resolverQueries is the subset of refdata.Queries used by
// discordUserEncounterResolver. Declaring it as an interface keeps the
// resolver unit-testable without a live Postgres instance.
type resolverQueries interface {
	GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error)
	GetPlayerCharacterByDiscordUser(ctx context.Context, arg refdata.GetPlayerCharacterByDiscordUserParams) (refdata.PlayerCharacter, error)
	GetActiveEncounterIDByCharacterID(ctx context.Context, characterID uuid.NullUUID) (uuid.UUID, error)
}

// discordUserEncounterResolver implements the Phase 105 per-user encounter
// routing contract by walking:
//
//	guild_id -> campaign_id -> (campaign_id, discord_user_id) -> character_id -> active encounter
type discordUserEncounterResolver struct {
	queries resolverQueries
}

func newDiscordUserEncounterResolver(q resolverQueries) *discordUserEncounterResolver {
	return &discordUserEncounterResolver{queries: q}
}

// ActiveEncounterForUser returns the active encounter ID the invoking Discord
// user is currently a combatant in, or a non-nil error if they are not
// registered or not in any active encounter.
func (r *discordUserEncounterResolver) ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error) {
	campaign, err := r.queries.GetCampaignByGuildID(ctx, guildID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("campaign lookup for guild %q: %w", guildID, err)
	}

	pc, err := r.queries.GetPlayerCharacterByDiscordUser(ctx, refdata.GetPlayerCharacterByDiscordUserParams{
		CampaignID:    campaign.ID,
		DiscordUserID: discordUserID,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("player character lookup for user %q: %w", discordUserID, err)
	}

	encID, err := r.queries.GetActiveEncounterIDByCharacterID(ctx, uuid.NullUUID{UUID: pc.CharacterID, Valid: true})
	if err != nil {
		return uuid.Nil, fmt.Errorf("active encounter lookup for character %s: %w", pc.CharacterID, err)
	}

	return encID, nil
}

// setupQueries is the subset of refdata.Queries used by setupCampaignLookup.
// Declaring it as an interface keeps the adapter unit-testable without a live
// Postgres instance.
type setupQueries interface {
	GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error)
	UpdateCampaignSettings(ctx context.Context, arg refdata.UpdateCampaignSettingsParams) (refdata.Campaign, error)
}

// setupCampaignLookup adapts refdata.Queries to discord.CampaignLookup so the
// Phase 12 /setup handler can resolve the campaign's DM user and persist the
// channel IDs it creates. Channel IDs are merged into the existing campaign
// settings JSONB; default settings are seeded when the row's settings column
// is null.
type setupCampaignLookup struct {
	queries setupQueries
}

func newSetupCampaignLookup(q setupQueries) *setupCampaignLookup {
	return &setupCampaignLookup{queries: q}
}

// GetCampaignForSetup returns the campaign info /setup needs (currently just
// the DM's Discord user id, used for permission overwrites on private channels).
func (l *setupCampaignLookup) GetCampaignForSetup(guildID string) (discord.SetupCampaignInfo, error) {
	c, err := l.queries.GetCampaignByGuildID(context.Background(), guildID)
	if err != nil {
		return discord.SetupCampaignInfo{}, fmt.Errorf("campaign lookup for guild %q: %w", guildID, err)
	}
	return discord.SetupCampaignInfo{DMUserID: c.DmUserID}, nil
}

// SaveChannelIDs merges channelIDs into the campaign settings JSONB and
// persists via UpdateCampaignSettings. Existing settings (turn timeout,
// diagonal rule, open5e sources, etc.) are preserved.
func (l *setupCampaignLookup) SaveChannelIDs(guildID string, channelIDs map[string]string) error {
	c, err := l.queries.GetCampaignByGuildID(context.Background(), guildID)
	if err != nil {
		return fmt.Errorf("campaign lookup for guild %q: %w", guildID, err)
	}

	settings := campaign.DefaultSettings()
	if c.Settings.Valid {
		if err := json.Unmarshal(c.Settings.RawMessage, &settings); err != nil {
			return fmt.Errorf("decoding existing settings: %w", err)
		}
	}
	settings.ChannelIDs = channelIDs

	raw, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("encoding updated settings: %w", err)
	}

	_, err = l.queries.UpdateCampaignSettings(context.Background(), refdata.UpdateCampaignSettingsParams{
		ID:       c.ID,
		Settings: pqtype.NullRawMessage{RawMessage: raw, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("updating campaign settings: %w", err)
	}
	return nil
}
