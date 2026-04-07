package homebrew

import (
	"context"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// CreateRaceParams describes the writable fields for a homebrew race.
type CreateRaceParams = refdata.UpsertRaceParams

// CreateHomebrewRace creates a campaign-scoped homebrew race.
func (s *Service) CreateHomebrewRace(ctx context.Context, campaignID uuid.UUID, params CreateRaceParams) (refdata.Race, error) {
	if err := s.requireCreate(campaignID, params.Name); err != nil {
		return refdata.Race{}, err
	}
	params.ID = s.idGen()
	params.CampaignID, params.Homebrew, params.Source = s.homebrewCols(campaignID)
	if err := s.store.UpsertRace(ctx, params); err != nil {
		return refdata.Race{}, err
	}
	return s.fetchRace(ctx, params.ID, campaignID)
}

// UpdateHomebrewRace updates an existing homebrew race.
func (s *Service) UpdateHomebrewRace(ctx context.Context, campaignID uuid.UUID, id string, params CreateRaceParams) (refdata.Race, error) {
	if err := s.requireUpdate(campaignID, id, params.Name); err != nil {
		return refdata.Race{}, err
	}
	existing, err := s.store.GetRace(ctx, id)
	if err != nil {
		return refdata.Race{}, translateGetErr(err)
	}
	if err := ownsRow(existing.Homebrew, existing.CampaignID, campaignID); err != nil {
		return refdata.Race{}, err
	}
	params.ID = id
	params.CampaignID, params.Homebrew, params.Source = s.homebrewCols(campaignID)
	if err := s.store.UpsertRace(ctx, params); err != nil {
		return refdata.Race{}, err
	}
	return s.fetchRace(ctx, id, campaignID)
}

// DeleteHomebrewRace deletes a homebrew race.
func (s *Service) DeleteHomebrewRace(ctx context.Context, campaignID uuid.UUID, id string) error {
	if err := s.requireDelete(campaignID, id); err != nil {
		return err
	}
	rows, err := s.store.DeleteHomebrewRace(ctx, refdata.DeleteHomebrewRaceParams{
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

func (s *Service) fetchRace(ctx context.Context, id string, campaignID uuid.UUID) (refdata.Race, error) {
	r, err := s.store.GetRace(ctx, id)
	if err != nil {
		return refdata.Race{}, translateGetErr(err)
	}
	if err := ownsRow(r.Homebrew, r.CampaignID, campaignID); err != nil {
		return refdata.Race{}, err
	}
	return r, nil
}
