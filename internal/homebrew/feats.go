package homebrew

import (
	"context"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// CreateFeatParams describes the writable fields for a homebrew feat.
type CreateFeatParams = refdata.UpsertFeatParams

// CreateHomebrewFeat creates a campaign-scoped homebrew feat.
func (s *Service) CreateHomebrewFeat(ctx context.Context, campaignID uuid.UUID, params CreateFeatParams) (refdata.Feat, error) {
	if err := s.requireCreate(campaignID, params.Name); err != nil {
		return refdata.Feat{}, err
	}
	params.ID = s.idGen()
	params.CampaignID, params.Homebrew, params.Source = s.homebrewCols(campaignID)
	if err := s.store.UpsertFeat(ctx, params); err != nil {
		return refdata.Feat{}, err
	}
	return s.fetchFeat(ctx, params.ID, campaignID)
}

// UpdateHomebrewFeat updates an existing homebrew feat.
func (s *Service) UpdateHomebrewFeat(ctx context.Context, campaignID uuid.UUID, id string, params CreateFeatParams) (refdata.Feat, error) {
	if err := s.requireUpdate(campaignID, id, params.Name); err != nil {
		return refdata.Feat{}, err
	}
	existing, err := s.store.GetFeat(ctx, id)
	if err != nil {
		return refdata.Feat{}, translateGetErr(err)
	}
	if err := ownsRow(existing.Homebrew, existing.CampaignID, campaignID); err != nil {
		return refdata.Feat{}, err
	}
	params.ID = id
	params.CampaignID, params.Homebrew, params.Source = s.homebrewCols(campaignID)
	if err := s.store.UpsertFeat(ctx, params); err != nil {
		return refdata.Feat{}, err
	}
	return s.fetchFeat(ctx, id, campaignID)
}

// DeleteHomebrewFeat deletes a homebrew feat.
func (s *Service) DeleteHomebrewFeat(ctx context.Context, campaignID uuid.UUID, id string) error {
	if err := s.requireDelete(campaignID, id); err != nil {
		return err
	}
	rows, err := s.store.DeleteHomebrewFeat(ctx, refdata.DeleteHomebrewFeatParams{
		ID:         id,
		CampaignID: uuid.NullUUID{UUID: campaignID, Valid: true},
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) fetchFeat(ctx context.Context, id string, campaignID uuid.UUID) (refdata.Feat, error) {
	r, err := s.store.GetFeat(ctx, id)
	if err != nil {
		return refdata.Feat{}, translateGetErr(err)
	}
	if err := ownsRow(r.Homebrew, r.CampaignID, campaignID); err != nil {
		return refdata.Feat{}, err
	}
	return r, nil
}
