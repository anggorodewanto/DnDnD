package homebrew

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// CreateClassParams describes the writable fields for a homebrew class.
type CreateClassParams = refdata.UpsertClassParams

func (s *Service) classOps() typeOps[refdata.UpsertClassParams, refdata.Class] {
	return typeOps[refdata.UpsertClassParams, refdata.Class]{
		get:    s.store.GetClass,
		upsert: s.store.UpsertClass,
		deleteHomebrew: func(ctx context.Context, id string, campaignID uuid.UUID) (int64, error) {
			return s.store.DeleteHomebrewClass(ctx, refdata.DeleteHomebrewClassParams{
				ID:         id,
				CampaignID: uuid.NullUUID{UUID: campaignID, Valid: true},
			})
		},
		nameOf: func(p refdata.UpsertClassParams) string { return p.Name },
		setIdentity: func(p *refdata.UpsertClassParams, id string, cid uuid.NullUUID, hb sql.NullBool, src sql.NullString) {
			p.ID, p.CampaignID, p.Homebrew, p.Source = id, cid, hb, src
		},
		ownership: func(r refdata.Class) (sql.NullBool, uuid.NullUUID) { return r.Homebrew, r.CampaignID },
	}
}

// CreateHomebrewClass creates a campaign-scoped homebrew class. Used
// both for full custom classes and for adding new class features (via
// the features_by_level field) to a campaign-specific class.
func (s *Service) CreateHomebrewClass(ctx context.Context, campaignID uuid.UUID, params CreateClassParams) (refdata.Class, error) {
	return genericCreate(ctx, s, s.classOps(), campaignID, params)
}

// UpdateHomebrewClass updates an existing homebrew class.
func (s *Service) UpdateHomebrewClass(ctx context.Context, campaignID uuid.UUID, id string, params CreateClassParams) (refdata.Class, error) {
	return genericUpdate(ctx, s, s.classOps(), campaignID, id, params)
}

// DeleteHomebrewClass deletes a homebrew class.
func (s *Service) DeleteHomebrewClass(ctx context.Context, campaignID uuid.UUID, id string) error {
	return genericDelete(ctx, s, s.classOps(), campaignID, id)
}
