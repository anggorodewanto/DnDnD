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
//
// The dashboard's Message Player picker sends each option's **character_id**
// (MessagePlayerPanel.svelte `value={c.character_id}`), so the id reaching the
// service is a characters.id — NOT the player_characters PK. We resolve it via
// the character's campaign + the (campaign, character) player_characters row.
// (The earlier GetPlayerCharacter-by-PK lookup never matched → every send 500'd
// with "player character not found".)
type PlayerLookupQueries interface {
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	GetPlayerCharacterByCharacter(ctx context.Context, arg refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error)
}

// PlayerLookupAdapter adapts refdata queries into a PlayerLookup so the
// service can resolve a character_id to the Discord user it belongs to.
type PlayerLookupAdapter struct {
	q PlayerLookupQueries
}

// NewPlayerLookupAdapter constructs a PlayerLookupAdapter.
func NewPlayerLookupAdapter(q PlayerLookupQueries) *PlayerLookupAdapter {
	return &PlayerLookupAdapter{q: q}
}

// LookupPlayer resolves a character_id to the Discord user it belongs to:
// characters → campaign, then the player_characters row for (campaign,
// character). A missing character or missing roster row maps to
// ErrPlayerNotFound.
func (a *PlayerLookupAdapter) LookupPlayer(ctx context.Context, characterID uuid.UUID) (PlayerInfo, error) {
	char, err := a.q.GetCharacter(ctx, characterID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PlayerInfo{}, ErrPlayerNotFound
		}
		return PlayerInfo{}, fmt.Errorf("getting character: %w", err)
	}
	row, err := a.q.GetPlayerCharacterByCharacter(ctx, refdata.GetPlayerCharacterByCharacterParams{
		CampaignID:  char.CampaignID,
		CharacterID: characterID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PlayerInfo{}, ErrPlayerNotFound
		}
		return PlayerInfo{}, fmt.Errorf("getting player character: %w", err)
	}
	return PlayerInfo{
		DiscordUserID: row.DiscordUserID,
		CampaignID:    row.CampaignID,
		// RowID is the player_characters PK — the FK target for
		// dm_player_messages.player_character_id. Omitting it leaves RowID nil,
		// which 500s the log insert on the FK constraint *after* the DM is sent.
		RowID: row.ID,
	}, nil
}
