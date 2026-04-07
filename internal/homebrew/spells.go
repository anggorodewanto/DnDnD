package homebrew

import (
	"context"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// CreateSpellParams describes the writable fields when creating a
// homebrew spell.
type CreateSpellParams = refdata.UpsertSpellParams

// CreateHomebrewSpell creates a campaign-scoped homebrew spell.
func (s *Service) CreateHomebrewSpell(ctx context.Context, campaignID uuid.UUID, params CreateSpellParams) (refdata.Spell, error) {
	if err := s.requireCreate(campaignID, params.Name); err != nil {
		return refdata.Spell{}, err
	}
	params.ID = s.idGen()
	params.CampaignID, params.Homebrew, params.Source = s.homebrewCols(campaignID)
	if err := s.store.UpsertSpell(ctx, params); err != nil {
		return refdata.Spell{}, err
	}
	return s.fetchSpell(ctx, params.ID, campaignID)
}

// UpdateHomebrewSpell updates an existing homebrew spell owned by the
// given campaign.
func (s *Service) UpdateHomebrewSpell(ctx context.Context, campaignID uuid.UUID, id string, params CreateSpellParams) (refdata.Spell, error) {
	if err := s.requireUpdate(campaignID, id, params.Name); err != nil {
		return refdata.Spell{}, err
	}
	existing, err := s.store.GetSpell(ctx, id)
	if err != nil {
		return refdata.Spell{}, translateGetErr(err)
	}
	if err := ownsRow(existing.Homebrew, existing.CampaignID, campaignID); err != nil {
		return refdata.Spell{}, err
	}
	params.ID = id
	params.CampaignID, params.Homebrew, params.Source = s.homebrewCols(campaignID)
	if err := s.store.UpsertSpell(ctx, params); err != nil {
		return refdata.Spell{}, err
	}
	return s.fetchSpell(ctx, id, campaignID)
}

// DeleteHomebrewSpell deletes a homebrew spell.
func (s *Service) DeleteHomebrewSpell(ctx context.Context, campaignID uuid.UUID, id string) error {
	if err := s.requireDelete(campaignID, id); err != nil {
		return err
	}
	rows, err := s.store.DeleteHomebrewSpell(ctx, refdata.DeleteHomebrewSpellParams{
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

func (s *Service) fetchSpell(ctx context.Context, id string, campaignID uuid.UUID) (refdata.Spell, error) {
	r, err := s.store.GetSpell(ctx, id)
	if err != nil {
		return refdata.Spell{}, translateGetErr(err)
	}
	if err := ownsRow(r.Homebrew, r.CampaignID, campaignID); err != nil {
		return refdata.Spell{}, err
	}
	return r, nil
}
