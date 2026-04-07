package homebrew

import (
	"context"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// CreateClassParams describes the writable fields for a homebrew class.
type CreateClassParams = refdata.UpsertClassParams

// CreateHomebrewClass creates a campaign-scoped homebrew class. Used
// both for full custom classes and for adding new class features (via
// the features_by_level field) to a campaign-specific class.
func (s *Service) CreateHomebrewClass(ctx context.Context, campaignID uuid.UUID, params CreateClassParams) (refdata.Class, error) {
	if err := s.requireCreate(campaignID, params.Name); err != nil {
		return refdata.Class{}, err
	}
	params.ID = s.idGen()
	params.CampaignID, params.Homebrew, params.Source = s.homebrewCols(campaignID)
	if err := s.store.UpsertClass(ctx, params); err != nil {
		return refdata.Class{}, err
	}
	return s.fetchClass(ctx, params.ID, campaignID)
}

// UpdateHomebrewClass updates an existing homebrew class.
func (s *Service) UpdateHomebrewClass(ctx context.Context, campaignID uuid.UUID, id string, params CreateClassParams) (refdata.Class, error) {
	if err := s.requireUpdate(campaignID, id, params.Name); err != nil {
		return refdata.Class{}, err
	}
	existing, err := s.store.GetClass(ctx, id)
	if err != nil {
		return refdata.Class{}, translateGetErr(err)
	}
	if err := ownsRow(existing.Homebrew, existing.CampaignID, campaignID); err != nil {
		return refdata.Class{}, err
	}
	params.ID = id
	params.CampaignID, params.Homebrew, params.Source = s.homebrewCols(campaignID)
	if err := s.store.UpsertClass(ctx, params); err != nil {
		return refdata.Class{}, err
	}
	return s.fetchClass(ctx, id, campaignID)
}

// DeleteHomebrewClass deletes a homebrew class.
func (s *Service) DeleteHomebrewClass(ctx context.Context, campaignID uuid.UUID, id string) error {
	if err := s.requireDelete(campaignID, id); err != nil {
		return err
	}
	rows, err := s.store.DeleteHomebrewClass(ctx, refdata.DeleteHomebrewClassParams{
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

func (s *Service) fetchClass(ctx context.Context, id string, campaignID uuid.UUID) (refdata.Class, error) {
	r, err := s.store.GetClass(ctx, id)
	if err != nil {
		return refdata.Class{}, translateGetErr(err)
	}
	if err := ownsRow(r.Homebrew, r.CampaignID, campaignID); err != nil {
		return refdata.Class{}, err
	}
	return r, nil
}
