package homebrew

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// CreateRaceParams describes the writable fields for a homebrew race.
type CreateRaceParams = refdata.UpsertRaceParams

func (s *Service) raceOps() typeOps[refdata.UpsertRaceParams, refdata.Race] {
	return typeOps[refdata.UpsertRaceParams, refdata.Race]{
		get:    s.store.GetRace,
		upsert: s.store.UpsertRace,
		deleteHomebrew: func(ctx context.Context, id string, campaignID uuid.UUID) (int64, error) {
			return s.store.DeleteHomebrewRace(ctx, refdata.DeleteHomebrewRaceParams{
				ID:         id,
				CampaignID: uuid.NullUUID{UUID: campaignID, Valid: true},
			})
		},
		nameOf: func(p refdata.UpsertRaceParams) string { return p.Name },
		setIdentity: func(p *refdata.UpsertRaceParams, id string, cid uuid.NullUUID, hb sql.NullBool, src sql.NullString) {
			p.ID, p.CampaignID, p.Homebrew, p.Source = id, cid, hb, src
		},
		ownership: func(r refdata.Race) (sql.NullBool, uuid.NullUUID) { return r.Homebrew, r.CampaignID },
	}
}

// CreateHomebrewRace creates a campaign-scoped homebrew race.
func (s *Service) CreateHomebrewRace(ctx context.Context, campaignID uuid.UUID, params CreateRaceParams) (refdata.Race, error) {
	return genericCreate(ctx, s, s.raceOps(), campaignID, params)
}

// UpdateHomebrewRace updates an existing homebrew race.
func (s *Service) UpdateHomebrewRace(ctx context.Context, campaignID uuid.UUID, id string, params CreateRaceParams) (refdata.Race, error) {
	return genericUpdate(ctx, s, s.raceOps(), campaignID, id, params)
}

// DeleteHomebrewRace deletes a homebrew race.
func (s *Service) DeleteHomebrewRace(ctx context.Context, campaignID uuid.UUID, id string) error {
	return genericDelete(ctx, s, s.raceOps(), campaignID, id)
}
