package homebrew

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// CreateFeatParams describes the writable fields for a homebrew feat.
type CreateFeatParams = refdata.UpsertFeatParams

func (s *Service) featOps() typeOps[refdata.UpsertFeatParams, refdata.Feat] {
	return typeOps[refdata.UpsertFeatParams, refdata.Feat]{
		get:    s.store.GetFeat,
		upsert: s.store.UpsertFeat,
		deleteHomebrew: func(ctx context.Context, id string, campaignID uuid.UUID) (int64, error) {
			return s.store.DeleteHomebrewFeat(ctx, refdata.DeleteHomebrewFeatParams{
				ID:         id,
				CampaignID: uuid.NullUUID{UUID: campaignID, Valid: true},
			})
		},
		nameOf: func(p refdata.UpsertFeatParams) string { return p.Name },
		setIdentity: func(p *refdata.UpsertFeatParams, id string, cid uuid.NullUUID, hb sql.NullBool, src sql.NullString) {
			p.ID, p.CampaignID, p.Homebrew, p.Source = id, cid, hb, src
		},
		ownership: func(r refdata.Feat) (sql.NullBool, uuid.NullUUID) { return r.Homebrew, r.CampaignID },
	}
}

// CreateHomebrewFeat creates a campaign-scoped homebrew feat.
func (s *Service) CreateHomebrewFeat(ctx context.Context, campaignID uuid.UUID, params CreateFeatParams) (refdata.Feat, error) {
	return genericCreate(ctx, s, s.featOps(), campaignID, params)
}

// UpdateHomebrewFeat updates an existing homebrew feat.
func (s *Service) UpdateHomebrewFeat(ctx context.Context, campaignID uuid.UUID, id string, params CreateFeatParams) (refdata.Feat, error) {
	return genericUpdate(ctx, s, s.featOps(), campaignID, id, params)
}

// DeleteHomebrewFeat deletes a homebrew feat.
func (s *Service) DeleteHomebrewFeat(ctx context.Context, campaignID uuid.UUID, id string) error {
	return genericDelete(ctx, s, s.featOps(), campaignID, id)
}
