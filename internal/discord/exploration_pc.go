package discord

import (
	"context"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// resolveExplorationPC picks the PC combatant that belongs to the invoking
// Discord user. Resolution order:
//
//  1. Collect alive PC combatants (character_id IS NOT NULL).
//  2. If exactly one, return it (no ambiguity).
//  3. Otherwise look up the invoker's character via the supplied campaign /
//     character closures and match combatant.CharacterID.UUID == char.ID.
//  4. If either lookup is unwired (nil) or fails, fall back to the first PC so
//     single-PC deployments stay functional.
//
// If multiple PCs exist but none match the invoker's character, ok is false —
// callers should surface the "not found in this encounter" error.
//
// Accepting closures instead of interfaces lets both MoveHandler and
// ActionHandler share the logic without coupling their (distinct) provider
// interfaces.
func resolveExplorationPC(
	ctx context.Context,
	all []refdata.Combatant,
	guildID, userID string,
	getCampaign func(ctx context.Context, guildID string) (refdata.Campaign, error),
	getCharacter func(ctx context.Context, campaignID uuid.UUID, discordUserID string) (refdata.Character, error),
) (refdata.Combatant, bool) {
	var pcs []refdata.Combatant
	for _, c := range all {
		if c.CharacterID.Valid && c.IsAlive {
			pcs = append(pcs, c)
		}
	}
	if len(pcs) == 0 {
		return refdata.Combatant{}, false
	}
	if len(pcs) == 1 {
		return pcs[0], true
	}
	if getCampaign == nil || getCharacter == nil {
		return pcs[0], true
	}
	campaign, err := getCampaign(ctx, guildID)
	if err != nil {
		return pcs[0], true
	}
	char, err := getCharacter(ctx, campaign.ID, userID)
	if err != nil {
		return pcs[0], true
	}
	for _, c := range pcs {
		if c.CharacterID.UUID == char.ID {
			return c, true
		}
	}
	return refdata.Combatant{}, false
}
