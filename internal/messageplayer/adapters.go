package messageplayer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// PlayerLookupQueries is the minimal refdata surface used by PlayerLookupAdapter.
type PlayerLookupQueries interface {
	GetPlayerCharacter(ctx context.Context, id uuid.UUID) (refdata.PlayerCharacter, error)
}

// PlayerLookupAdapter adapts refdata queries into a PlayerLookup so the
// service can resolve a player_character_id to the Discord user it belongs to.
type PlayerLookupAdapter struct {
	q PlayerLookupQueries
}

// NewPlayerLookupAdapter constructs a PlayerLookupAdapter.
func NewPlayerLookupAdapter(q PlayerLookupQueries) *PlayerLookupAdapter {
	return &PlayerLookupAdapter{q: q}
}

// LookupPlayer fetches the player_characters row and returns the subset of
// fields the messageplayer service needs.
func (a *PlayerLookupAdapter) LookupPlayer(ctx context.Context, playerCharacterID uuid.UUID) (PlayerInfo, error) {
	row, err := a.q.GetPlayerCharacter(ctx, playerCharacterID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PlayerInfo{}, ErrPlayerNotFound
		}
		return PlayerInfo{}, fmt.Errorf("getting player character: %w", err)
	}
	return PlayerInfo{
		DiscordUserID: row.DiscordUserID,
		CampaignID:    row.CampaignID,
	}, nil
}
