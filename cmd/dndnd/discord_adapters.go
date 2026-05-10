package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/discord"
	"github.com/ab/dndnd/internal/portal"
	"github.com/ab/dndnd/internal/refdata"
)

// portalTokenCreateCharacterPurpose is the purpose tag stored alongside every
// portal token minted by the /create-character flow. Persisted in
// portal_tokens.purpose so future audits / cleanup can tell at a glance which
// flow issued the token.
const portalTokenCreateCharacterPurpose = "create_character"

// portalTokenCreateCharacterTTL is the validity window for /create-character
// portal links per Phase 91a spec ("one-time link, 24 h expiry").
const portalTokenCreateCharacterTTL = 24 * time.Hour

// newPortalTokenIssuer returns a function shaped like discord.RegistrationDeps
// .TokenFunc that mints a portal token via the supplied TokenService. The
// returned closure captures the application context so each call participates
// in graceful shutdown without forcing the registration handler to thread one
// through. Replaces the legacy "e2e-token" placeholder (Phase 14 follow-up)
// per crit-06.
func newPortalTokenIssuer(ctx context.Context, svc *portal.TokenService) func(campaignID uuid.UUID, discordUserID string) (string, error) {
	return func(campaignID uuid.UUID, discordUserID string) (string, error) {
		return svc.CreateToken(ctx, campaignID, discordUserID, portalTokenCreateCharacterPurpose, portalTokenCreateCharacterTTL)
	}
}

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

// playerDirectMessenger is the subset of discord.DirectMessenger that
// playerNotifierAdapter depends on. Declaring it as an interface keeps the
// adapter unit-testable without a live Discord session.
type playerDirectMessenger interface {
	SendDirectMessage(discordUserID, body string) ([]string, error)
}

// playerNotifierAdapter implements dashboard.PlayerNotifier by sending
// Discord DMs through the bot's existing DirectMessenger. Wired into
// dashboard.NewApprovalHandler so approve / changes-requested / reject all
// notify the player out-of-band per Phase 16 done-when (spec lines 41 + 53).
type playerNotifierAdapter struct {
	dm playerDirectMessenger
}

func newPlayerNotifierAdapter(dm playerDirectMessenger) *playerNotifierAdapter {
	return &playerNotifierAdapter{dm: dm}
}

// NotifyApproval pings the player that their character was approved.
func (a *playerNotifierAdapter) NotifyApproval(_ context.Context, discordUserID, characterName string) error {
	body := fmt.Sprintf("✅ **%s** has been approved! You can now play.", characterName)
	if _, err := a.dm.SendDirectMessage(discordUserID, body); err != nil {
		return fmt.Errorf("notifying approval to %s: %w", discordUserID, err)
	}
	return nil
}

// NotifyChangesRequested pings the player that the DM requested changes,
// including the DM's feedback verbatim.
func (a *playerNotifierAdapter) NotifyChangesRequested(_ context.Context, discordUserID, characterName, feedback string) error {
	body := fmt.Sprintf("📝 **%s** needs changes before approval.\n\n**DM feedback:** %s", characterName, feedback)
	if _, err := a.dm.SendDirectMessage(discordUserID, body); err != nil {
		return fmt.Errorf("notifying changes-requested to %s: %w", discordUserID, err)
	}
	return nil
}

// NotifyRejection pings the player that their character was rejected,
// including the DM's reason verbatim.
func (a *playerNotifierAdapter) NotifyRejection(_ context.Context, discordUserID, characterName, feedback string) error {
	body := fmt.Sprintf("❌ **%s** was rejected.\n\n**DM feedback:** %s", characterName, feedback)
	if _, err := a.dm.SendDirectMessage(discordUserID, body); err != nil {
		return fmt.Errorf("notifying rejection to %s: %w", discordUserID, err)
	}
	return nil
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
