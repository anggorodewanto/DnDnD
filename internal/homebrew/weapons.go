package homebrew

import (
	"context"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// CreateWeaponParams describes the writable fields when creating a
// homebrew weapon.
type CreateWeaponParams = refdata.UpsertWeaponParams

// CreateHomebrewWeapon creates a campaign-scoped homebrew weapon.
func (s *Service) CreateHomebrewWeapon(ctx context.Context, campaignID uuid.UUID, params CreateWeaponParams) (refdata.Weapon, error) {
	if err := s.requireCreate(campaignID, params.Name); err != nil {
		return refdata.Weapon{}, err
	}
	params.ID = s.idGen()
	params.CampaignID, params.Homebrew, params.Source = s.homebrewCols(campaignID)
	if err := s.store.UpsertWeapon(ctx, params); err != nil {
		return refdata.Weapon{}, err
	}
	return s.fetchWeapon(ctx, params.ID, campaignID)
}

// UpdateHomebrewWeapon updates an existing homebrew weapon.
func (s *Service) UpdateHomebrewWeapon(ctx context.Context, campaignID uuid.UUID, id string, params CreateWeaponParams) (refdata.Weapon, error) {
	if err := s.requireUpdate(campaignID, id, params.Name); err != nil {
		return refdata.Weapon{}, err
	}
	existing, err := s.store.GetWeapon(ctx, id)
	if err != nil {
		return refdata.Weapon{}, translateGetErr(err)
	}
	if err := ownsRow(existing.Homebrew, existing.CampaignID, campaignID); err != nil {
		return refdata.Weapon{}, err
	}
	params.ID = id
	params.CampaignID, params.Homebrew, params.Source = s.homebrewCols(campaignID)
	if err := s.store.UpsertWeapon(ctx, params); err != nil {
		return refdata.Weapon{}, err
	}
	return s.fetchWeapon(ctx, id, campaignID)
}

// DeleteHomebrewWeapon deletes a homebrew weapon.
func (s *Service) DeleteHomebrewWeapon(ctx context.Context, campaignID uuid.UUID, id string) error {
	if err := s.requireDelete(campaignID, id); err != nil {
		return err
	}
	rows, err := s.store.DeleteHomebrewWeapon(ctx, refdata.DeleteHomebrewWeaponParams{
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

func (s *Service) fetchWeapon(ctx context.Context, id string, campaignID uuid.UUID) (refdata.Weapon, error) {
	r, err := s.store.GetWeapon(ctx, id)
	if err != nil {
		return refdata.Weapon{}, translateGetErr(err)
	}
	if err := ownsRow(r.Homebrew, r.CampaignID, campaignID); err != nil {
		return refdata.Weapon{}, err
	}
	return r, nil
}
