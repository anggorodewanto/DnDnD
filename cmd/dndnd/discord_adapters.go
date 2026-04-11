package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// resolverQueries is the subset of refdata.Queries used by
// discordUserEncounterResolver. Declaring it as an interface keeps the
// resolver unit-testable without a live Postgres instance — *refdata.Queries
// already satisfies this shape.
type resolverQueries interface {
	GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error)
	GetPlayerCharacterByDiscordUser(ctx context.Context, arg refdata.GetPlayerCharacterByDiscordUserParams) (refdata.PlayerCharacter, error)
	GetActiveEncounterIDByCharacterID(ctx context.Context, characterID uuid.NullUUID) (uuid.UUID, error)
}

// discordUserEncounterResolver implements the Phase 105 per-user encounter
// routing contract shared by MoveEncounterProvider, CheckEncounterProvider,
// SummonCommandEncounterProvider, RecapEncounterProvider, etc. The chain is:
//
//	guild_id -> campaign_id -> (campaign_id, discord_user_id) -> character_id -> active encounter
//
// Any failure along the chain surfaces as a non-nil error so callers can fall
// through to their "no active encounter for you" response, matching the
// interface convention established in Phase 105.
type discordUserEncounterResolver struct {
	queries resolverQueries
}

// newDiscordUserEncounterResolver constructs a resolver backed by the given
// queries surface. Pass *refdata.Queries in production wiring.
func newDiscordUserEncounterResolver(q resolverQueries) *discordUserEncounterResolver {
	return &discordUserEncounterResolver{queries: q}
}

// ActiveEncounterForUser resolves the active encounter ID the invoking Discord
// user is currently a combatant in, or returns a non-nil error if they are
// not registered or not in any active encounter.
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
