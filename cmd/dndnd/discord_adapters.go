package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"

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
