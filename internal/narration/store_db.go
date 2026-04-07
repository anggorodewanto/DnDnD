package narration

import (
	"context"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// RefdataQueries is the minimal subset of *refdata.Queries the DBStore needs.
// Declared as an interface so unit tests can swap in a fake.
type RefdataQueries interface {
	InsertNarrationPost(ctx context.Context, arg refdata.InsertNarrationPostParams) (refdata.NarrationPost, error)
	ListNarrationPostsByCampaign(ctx context.Context, arg refdata.ListNarrationPostsByCampaignParams) ([]refdata.NarrationPost, error)
}

// DBStore is a Store implementation backed by sqlc-generated refdata queries.
type DBStore struct {
	q RefdataQueries
}

// NewDBStore constructs a DBStore wrapping the given refdata queries.
func NewDBStore(q RefdataQueries) *DBStore {
	return &DBStore{q: q}
}

// InsertNarrationPost records a narration post row.
func (s *DBStore) InsertNarrationPost(ctx context.Context, p InsertPostParams) (Post, error) {
	row, err := s.q.InsertNarrationPost(ctx, refdata.InsertNarrationPostParams{
		CampaignID:         p.CampaignID,
		AuthorUserID:       p.AuthorUserID,
		Body:               p.Body,
		AttachmentAssetIds: nonNilUUIDs(p.AttachmentAssetIDs),
		DiscordMessageIds:  nonNilStrings(p.DiscordMessageIDs),
	})
	if err != nil {
		return Post{}, err
	}
	return postFromRefdata(row), nil
}

// ListNarrationPostsByCampaign returns recent posts newest-first.
func (s *DBStore) ListNarrationPostsByCampaign(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]Post, error) {
	rows, err := s.q.ListNarrationPostsByCampaign(ctx, refdata.ListNarrationPostsByCampaignParams{
		CampaignID: campaignID,
		Limit:      int32(limit),
		Offset:     int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Post, 0, len(rows))
	for _, r := range rows {
		out = append(out, postFromRefdata(r))
	}
	return out, nil
}

func postFromRefdata(r refdata.NarrationPost) Post {
	return Post{
		ID:                 r.ID,
		CampaignID:         r.CampaignID,
		AuthorUserID:       r.AuthorUserID,
		Body:               r.Body,
		AttachmentAssetIDs: nonNilUUIDs(r.AttachmentAssetIds),
		DiscordMessageIDs:  nonNilStrings(r.DiscordMessageIds),
		PostedAt:           r.PostedAt,
	}
}

// nonNilUUIDs ensures nil slices become empty slices so JSON output is `[]`
// rather than `null`, matching the repo's emit_empty_slices sqlc setting.
func nonNilUUIDs(v []uuid.UUID) []uuid.UUID {
	if v == nil {
		return []uuid.UUID{}
	}
	return v
}

// nonNilStrings mirrors nonNilUUIDs for string slices.
func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
