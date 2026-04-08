package characteroverview

import (
	"context"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// StatusApproved is the player_characters.status value used for the
// read-only character overview — only approved party members are exposed.
const StatusApproved = "approved"

// RefdataQueries is the minimal subset of *refdata.Queries that DBStore
// depends on. Declared as an interface so tests can substitute a fake.
type RefdataQueries interface {
	ListPlayerCharactersByStatus(ctx context.Context, arg refdata.ListPlayerCharactersByStatusParams) ([]refdata.ListPlayerCharactersByStatusRow, error)
}

// DBStore is a Store implementation backed by sqlc-generated refdata queries.
type DBStore struct {
	q RefdataQueries
}

// NewDBStore constructs a DBStore wrapping the given refdata queries.
func NewDBStore(q RefdataQueries) *DBStore {
	return &DBStore{q: q}
}

// ListApprovedPartyCharacters returns the approved player characters of a
// campaign, shaped into CharacterSheet domain structs for the dashboard.
func (s *DBStore) ListApprovedPartyCharacters(ctx context.Context, campaignID uuid.UUID) ([]CharacterSheet, error) {
	rows, err := s.q.ListPlayerCharactersByStatus(ctx, refdata.ListPlayerCharactersByStatusParams{
		CampaignID: campaignID,
		Status:     StatusApproved,
	})
	if err != nil {
		return nil, err
	}
	out := make([]CharacterSheet, 0, len(rows))
	for _, r := range rows {
		out = append(out, sheetFromRefdata(r))
	}
	return out, nil
}

func sheetFromRefdata(r refdata.ListPlayerCharactersByStatusRow) CharacterSheet {
	ddbURL := ""
	if r.DdbUrl.Valid {
		ddbURL = r.DdbUrl.String
	}
	languages := r.Languages
	if languages == nil {
		languages = []string{}
	}
	return CharacterSheet{
		PlayerCharacterID: r.ID,
		CharacterID:       r.CharacterID,
		DiscordUserID:     r.DiscordUserID,
		Name:              r.CharacterName,
		Race:              r.Race,
		Level:             r.Level,
		Classes:           r.Classes,
		HPMax:             r.HpMax,
		HPCurrent:         r.HpCurrent,
		AC:                r.Ac,
		SpeedFt:           r.SpeedFt,
		AbilityScores:     r.AbilityScores,
		Languages:         languages,
		DDBURL:            ddbURL,
	}
}
