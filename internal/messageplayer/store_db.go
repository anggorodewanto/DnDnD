package messageplayer

import (
	"context"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// RefdataQueries is the minimal subset of *refdata.Queries the DBStore needs.
// Declared as an interface so unit tests can swap in a fake.
type RefdataQueries interface {
	InsertDMPlayerMessage(ctx context.Context, arg refdata.InsertDMPlayerMessageParams) (refdata.DmPlayerMessage, error)
	ListDMPlayerMessages(ctx context.Context, arg refdata.ListDMPlayerMessagesParams) ([]refdata.DmPlayerMessage, error)
}

// DBStore is a Store implementation backed by sqlc-generated refdata queries.
type DBStore struct {
	q RefdataQueries
}

// NewDBStore constructs a DBStore wrapping the given refdata queries.
func NewDBStore(q RefdataQueries) *DBStore {
	return &DBStore{q: q}
}

// InsertDMMessage records a DM-to-player message row.
func (s *DBStore) InsertDMMessage(ctx context.Context, p InsertParams) (Message, error) {
	row, err := s.q.InsertDMPlayerMessage(ctx, refdata.InsertDMPlayerMessageParams{
		CampaignID:        p.CampaignID,
		PlayerCharacterID: p.PlayerCharacterID,
		AuthorUserID:      p.AuthorUserID,
		Body:              p.Body,
		DiscordMessageIds: orEmptyStrings(p.DiscordMessageIDs),
	})
	if err != nil {
		return Message{}, err
	}
	return messageFromRefdata(row), nil
}

// ListDMMessages returns recent DM messages for a player, newest-first.
func (s *DBStore) ListDMMessages(ctx context.Context, campaignID, playerCharacterID uuid.UUID, limit, offset int) ([]Message, error) {
	rows, err := s.q.ListDMPlayerMessages(ctx, refdata.ListDMPlayerMessagesParams{
		CampaignID:        campaignID,
		PlayerCharacterID: playerCharacterID,
		Limit:             int32(limit),
		Offset:            int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(rows))
	for _, r := range rows {
		out = append(out, messageFromRefdata(r))
	}
	return out, nil
}

func messageFromRefdata(r refdata.DmPlayerMessage) Message {
	return Message{
		ID:                r.ID,
		CampaignID:        r.CampaignID,
		PlayerCharacterID: r.PlayerCharacterID,
		AuthorUserID:      r.AuthorUserID,
		Body:              r.Body,
		DiscordMessageIDs: orEmptyStrings(r.DiscordMessageIds),
		SentAt:            r.SentAt,
	}
}

// orEmptyStrings replaces a nil slice with an empty one so JSON output is []
// rather than null.
func orEmptyStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
