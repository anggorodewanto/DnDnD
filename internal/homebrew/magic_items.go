package homebrew

import (
	"context"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// CreateMagicItemParams describes the writable fields when creating a
// homebrew magic item.
type CreateMagicItemParams = refdata.UpsertMagicItemParams

// CreateHomebrewMagicItem creates a campaign-scoped homebrew magic item.
func (s *Service) CreateHomebrewMagicItem(ctx context.Context, campaignID uuid.UUID, params CreateMagicItemParams) (refdata.MagicItem, error) {
	if err := s.requireCreate(campaignID, params.Name); err != nil {
		return refdata.MagicItem{}, err
	}
	params.ID = s.idGen()
	params.CampaignID, params.Homebrew, params.Source = s.homebrewCols(campaignID)
	if err := s.store.UpsertMagicItem(ctx, params); err != nil {
		return refdata.MagicItem{}, err
	}
	return s.fetchMagicItem(ctx, params.ID, campaignID)
}

// UpdateHomebrewMagicItem updates an existing homebrew magic item.
func (s *Service) UpdateHomebrewMagicItem(ctx context.Context, campaignID uuid.UUID, id string, params CreateMagicItemParams) (refdata.MagicItem, error) {
	if err := s.requireUpdate(campaignID, id, params.Name); err != nil {
		return refdata.MagicItem{}, err
	}
	existing, err := s.store.GetMagicItem(ctx, id)
	if err != nil {
		return refdata.MagicItem{}, translateGetErr(err)
	}
	if err := ownsRow(existing.Homebrew, existing.CampaignID, campaignID); err != nil {
		return refdata.MagicItem{}, err
	}
	params.ID = id
	params.CampaignID, params.Homebrew, params.Source = s.homebrewCols(campaignID)
	if err := s.store.UpsertMagicItem(ctx, params); err != nil {
		return refdata.MagicItem{}, err
	}
	return s.fetchMagicItem(ctx, id, campaignID)
}

// DeleteHomebrewMagicItem deletes a homebrew magic item.
func (s *Service) DeleteHomebrewMagicItem(ctx context.Context, campaignID uuid.UUID, id string) error {
	if err := s.requireDelete(campaignID, id); err != nil {
		return err
	}
	rows, err := s.store.DeleteHomebrewMagicItem(ctx, refdata.DeleteHomebrewMagicItemParams{
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

func (s *Service) fetchMagicItem(ctx context.Context, id string, campaignID uuid.UUID) (refdata.MagicItem, error) {
	r, err := s.store.GetMagicItem(ctx, id)
	if err != nil {
		return refdata.MagicItem{}, translateGetErr(err)
	}
	if err := ownsRow(r.Homebrew, r.CampaignID, campaignID); err != nil {
		return refdata.MagicItem{}, err
	}
	return r, nil
}
