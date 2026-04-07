package homebrew

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// CreateSpellParams describes the writable fields when creating a
// homebrew spell.
type CreateSpellParams = refdata.UpsertSpellParams

func (s *Service) spellOps() typeOps[refdata.UpsertSpellParams, refdata.Spell] {
	return typeOps[refdata.UpsertSpellParams, refdata.Spell]{
		get:    s.store.GetSpell,
		upsert: s.store.UpsertSpell,
		deleteHomebrew: func(ctx context.Context, id string, campaignID uuid.UUID) (int64, error) {
			return s.store.DeleteHomebrewSpell(ctx, refdata.DeleteHomebrewSpellParams{
				ID:         id,
				CampaignID: uuid.NullUUID{UUID: campaignID, Valid: true},
			})
		},
		nameOf: func(p refdata.UpsertSpellParams) string { return p.Name },
		setIdentity: func(p *refdata.UpsertSpellParams, id string, cid uuid.NullUUID, hb sql.NullBool, src sql.NullString) {
			p.ID, p.CampaignID, p.Homebrew, p.Source = id, cid, hb, src
		},
		ownership: func(r refdata.Spell) (sql.NullBool, uuid.NullUUID) { return r.Homebrew, r.CampaignID },
	}
}

// CreateHomebrewSpell creates a campaign-scoped homebrew spell.
func (s *Service) CreateHomebrewSpell(ctx context.Context, campaignID uuid.UUID, params CreateSpellParams) (refdata.Spell, error) {
	return genericCreate(ctx, s, s.spellOps(), campaignID, params)
}

// UpdateHomebrewSpell updates an existing homebrew spell owned by the
// given campaign.
func (s *Service) UpdateHomebrewSpell(ctx context.Context, campaignID uuid.UUID, id string, params CreateSpellParams) (refdata.Spell, error) {
	return genericUpdate(ctx, s, s.spellOps(), campaignID, id, params)
}

// DeleteHomebrewSpell deletes a homebrew spell.
func (s *Service) DeleteHomebrewSpell(ctx context.Context, campaignID uuid.UUID, id string) error {
	return genericDelete(ctx, s, s.spellOps(), campaignID, id)
}
