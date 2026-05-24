package homebrew

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// listHomebrew lists every row of one refdata type and keeps only the rows
// that are flagged homebrew AND owned by the given campaign. SRD rows
// (campaign_id NULL / homebrew false) and rows owned by other campaigns are
// filtered out, mirroring the ownership rule enforced on update/delete.
//
// The result is always a non-nil slice so handlers serialize an empty list
// as `[]` rather than `null`.
func listHomebrew[R any](
	ctx context.Context,
	campaignID uuid.UUID,
	list func(context.Context) ([]R, error),
	ownership func(R) (sql.NullBool, uuid.NullUUID),
) ([]R, error) {
	if err := validateCampaign(campaignID); err != nil {
		return nil, err
	}
	rows, err := list(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]R, 0, len(rows))
	for _, row := range rows {
		hb, owner := ownership(row)
		if ownsRow(hb, owner, campaignID) == nil {
			out = append(out, row)
		}
	}
	return out, nil
}

// ListHomebrewCreatures returns the campaign's homebrew creatures.
func (s *Service) ListHomebrewCreatures(ctx context.Context, campaignID uuid.UUID) ([]refdata.Creature, error) {
	return listHomebrew(ctx, campaignID, s.store.ListCreatures,
		func(r refdata.Creature) (sql.NullBool, uuid.NullUUID) { return r.Homebrew, r.CampaignID })
}

// ListHomebrewSpells returns the campaign's homebrew spells.
func (s *Service) ListHomebrewSpells(ctx context.Context, campaignID uuid.UUID) ([]refdata.Spell, error) {
	return listHomebrew(ctx, campaignID, s.store.ListSpells,
		func(r refdata.Spell) (sql.NullBool, uuid.NullUUID) { return r.Homebrew, r.CampaignID })
}

// ListHomebrewWeapons returns the campaign's homebrew weapons.
func (s *Service) ListHomebrewWeapons(ctx context.Context, campaignID uuid.UUID) ([]refdata.Weapon, error) {
	return listHomebrew(ctx, campaignID, s.store.ListWeapons,
		func(r refdata.Weapon) (sql.NullBool, uuid.NullUUID) { return r.Homebrew, r.CampaignID })
}

// ListHomebrewMagicItems returns the campaign's homebrew magic items.
func (s *Service) ListHomebrewMagicItems(ctx context.Context, campaignID uuid.UUID) ([]refdata.MagicItem, error) {
	return listHomebrew(ctx, campaignID, s.store.ListMagicItems,
		func(r refdata.MagicItem) (sql.NullBool, uuid.NullUUID) { return r.Homebrew, r.CampaignID })
}

// ListHomebrewRaces returns the campaign's homebrew races.
func (s *Service) ListHomebrewRaces(ctx context.Context, campaignID uuid.UUID) ([]refdata.Race, error) {
	return listHomebrew(ctx, campaignID, s.store.ListRaces,
		func(r refdata.Race) (sql.NullBool, uuid.NullUUID) { return r.Homebrew, r.CampaignID })
}

// ListHomebrewFeats returns the campaign's homebrew feats.
func (s *Service) ListHomebrewFeats(ctx context.Context, campaignID uuid.UUID) ([]refdata.Feat, error) {
	return listHomebrew(ctx, campaignID, s.store.ListFeats,
		func(r refdata.Feat) (sql.NullBool, uuid.NullUUID) { return r.Homebrew, r.CampaignID })
}

// ListHomebrewClasses returns the campaign's homebrew classes.
func (s *Service) ListHomebrewClasses(ctx context.Context, campaignID uuid.UUID) ([]refdata.Class, error) {
	return listHomebrew(ctx, campaignID, s.store.ListClasses,
		func(r refdata.Class) (sql.NullBool, uuid.NullUUID) { return r.Homebrew, r.CampaignID })
}
