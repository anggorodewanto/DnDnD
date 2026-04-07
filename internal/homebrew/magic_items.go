package homebrew

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// CreateMagicItemParams describes the writable fields when creating a
// homebrew magic item.
type CreateMagicItemParams = refdata.UpsertMagicItemParams

func (s *Service) magicItemOps() typeOps[refdata.UpsertMagicItemParams, refdata.MagicItem] {
	return typeOps[refdata.UpsertMagicItemParams, refdata.MagicItem]{
		get:    s.store.GetMagicItem,
		upsert: s.store.UpsertMagicItem,
		deleteHomebrew: func(ctx context.Context, id string, campaignID uuid.UUID) (int64, error) {
			return s.store.DeleteHomebrewMagicItem(ctx, refdata.DeleteHomebrewMagicItemParams{
				ID:         id,
				CampaignID: uuid.NullUUID{UUID: campaignID, Valid: true},
			})
		},
		nameOf: func(p refdata.UpsertMagicItemParams) string { return p.Name },
		setIdentity: func(p *refdata.UpsertMagicItemParams, id string, cid uuid.NullUUID, hb sql.NullBool, src sql.NullString) {
			p.ID, p.CampaignID, p.Homebrew, p.Source = id, cid, hb, src
		},
		ownership: func(r refdata.MagicItem) (sql.NullBool, uuid.NullUUID) { return r.Homebrew, r.CampaignID },
	}
}

// CreateHomebrewMagicItem creates a campaign-scoped homebrew magic item.
func (s *Service) CreateHomebrewMagicItem(ctx context.Context, campaignID uuid.UUID, params CreateMagicItemParams) (refdata.MagicItem, error) {
	return genericCreate(ctx, s, s.magicItemOps(), campaignID, params)
}

// UpdateHomebrewMagicItem updates an existing homebrew magic item.
func (s *Service) UpdateHomebrewMagicItem(ctx context.Context, campaignID uuid.UUID, id string, params CreateMagicItemParams) (refdata.MagicItem, error) {
	return genericUpdate(ctx, s, s.magicItemOps(), campaignID, id, params)
}

// DeleteHomebrewMagicItem deletes a homebrew magic item.
func (s *Service) DeleteHomebrewMagicItem(ctx context.Context, campaignID uuid.UUID, id string) error {
	return genericDelete(ctx, s, s.magicItemOps(), campaignID, id)
}
