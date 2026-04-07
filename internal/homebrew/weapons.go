package homebrew

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// CreateWeaponParams describes the writable fields when creating a
// homebrew weapon.
type CreateWeaponParams = refdata.UpsertWeaponParams

func (s *Service) weaponOps() typeOps[refdata.UpsertWeaponParams, refdata.Weapon] {
	return typeOps[refdata.UpsertWeaponParams, refdata.Weapon]{
		get:    s.store.GetWeapon,
		upsert: s.store.UpsertWeapon,
		deleteHomebrew: func(ctx context.Context, id string, campaignID uuid.UUID) (int64, error) {
			return s.store.DeleteHomebrewWeapon(ctx, refdata.DeleteHomebrewWeaponParams{
				ID:         id,
				CampaignID: uuid.NullUUID{UUID: campaignID, Valid: true},
			})
		},
		nameOf: func(p refdata.UpsertWeaponParams) string { return p.Name },
		setIdentity: func(p *refdata.UpsertWeaponParams, id string, cid uuid.NullUUID, hb sql.NullBool, src sql.NullString) {
			p.ID, p.CampaignID, p.Homebrew, p.Source = id, cid, hb, src
		},
		ownership: func(r refdata.Weapon) (sql.NullBool, uuid.NullUUID) { return r.Homebrew, r.CampaignID },
	}
}

// CreateHomebrewWeapon creates a campaign-scoped homebrew weapon.
func (s *Service) CreateHomebrewWeapon(ctx context.Context, campaignID uuid.UUID, params CreateWeaponParams) (refdata.Weapon, error) {
	return genericCreate(ctx, s, s.weaponOps(), campaignID, params)
}

// UpdateHomebrewWeapon updates an existing homebrew weapon.
func (s *Service) UpdateHomebrewWeapon(ctx context.Context, campaignID uuid.UUID, id string, params CreateWeaponParams) (refdata.Weapon, error) {
	return genericUpdate(ctx, s, s.weaponOps(), campaignID, id, params)
}

// DeleteHomebrewWeapon deletes a homebrew weapon.
func (s *Service) DeleteHomebrewWeapon(ctx context.Context, campaignID uuid.UUID, id string) error {
	return genericDelete(ctx, s, s.weaponOps(), campaignID, id)
}
