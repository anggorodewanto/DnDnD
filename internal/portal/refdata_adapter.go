package portal

import (
	"context"
	"encoding/json"

	"github.com/ab/dndnd/internal/open5e"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
)

// Open5eCampaignLookup resolves the enabled Open5e document slugs for a
// campaign. Matches open5e.CampaignSourceLookup / statblocklibrary's
// CampaignLookup so a single CampaignSourceLookup instance can be shared.
type Open5eCampaignLookup interface {
	EnabledOpen5eSources(campaignID uuid.UUID) []string
}

// RefDataQuerier is the subset of refdata.Queries methods needed by the portal.
type RefDataQuerier interface {
	ListRaces(ctx context.Context) ([]refdata.Race, error)
	ListClasses(ctx context.Context) ([]refdata.Class, error)
	ListSpellsByClass(ctx context.Context, class string) ([]refdata.Spell, error)
	ListWeapons(ctx context.Context) ([]refdata.Weapon, error)
	ListArmor(ctx context.Context) ([]refdata.Armor, error)
}

// RefDataAdapter adapts refdata.Queries to the portal RefDataStore interface.
type RefDataAdapter struct {
	q      RefDataQuerier
	lookup Open5eCampaignLookup
}

// NewRefDataAdapter creates a new RefDataAdapter with no Open5e campaign
// lookup wired in. The adapter will strip all open5e:* rows from spell
// lists (safe default for deploys that haven't enabled Open5e).
func NewRefDataAdapter(q RefDataQuerier) *RefDataAdapter {
	return &RefDataAdapter{q: q}
}

// NewRefDataAdapterWithOpen5eLookup creates a RefDataAdapter that gates
// Open5e spells per campaign via the given lookup. When a request carries
// a valid campaign_id the adapter resolves that campaign's enabled
// document slugs and passes them to open5e.FilterSpellsByOpen5eSources.
// When the id is empty/invalid the adapter falls back to the safe
// default (hide all open5e:* rows) so cached third-party content never
// leaks into campaigns that have not opted in.
func NewRefDataAdapterWithOpen5eLookup(q RefDataQuerier, lookup Open5eCampaignLookup) *RefDataAdapter {
	return &RefDataAdapter{q: q, lookup: lookup}
}

// ListRaces converts refdata races to portal RaceInfo.
func (a *RefDataAdapter) ListRaces(ctx context.Context) ([]RaceInfo, error) {
	races, err := a.q.ListRaces(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]RaceInfo, len(races))
	for i, r := range races {
		result[i] = RaceInfo{
			ID:             r.ID,
			Name:           r.Name,
			SpeedFt:        int(r.SpeedFt),
			Size:           r.Size,
			AbilityBonuses: r.AbilityBonuses,
			Languages:      r.Languages,
			Traits:         r.Traits,
			Subraces:       nullRawToJSON(r.Subraces),
		}
	}
	return result, nil
}

// ListClasses converts refdata classes to portal ClassInfo.
func (a *RefDataAdapter) ListClasses(ctx context.Context) ([]ClassInfo, error) {
	classes, err := a.q.ListClasses(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ClassInfo, len(classes))
	for i, c := range classes {
		result[i] = ClassInfo{
			ID:                  c.ID,
			Name:                c.Name,
			HitDie:              c.HitDie,
			PrimaryAbility:      c.PrimaryAbility,
			SaveProficiencies:   c.SaveProficiencies,
			ArmorProficiencies:  c.ArmorProficiencies,
			WeaponProficiencies: c.WeaponProficiencies,
			SkillChoices:        nullRawToJSON(c.SkillChoices),
			Spellcasting:        nullRawToJSON(c.Spellcasting),
			Subclasses:          c.Subclasses,
			SubclassLevel:       int(c.SubclassLevel),
		}
	}
	return result, nil
}

// ListSpellsByClass converts refdata spells to portal SpellInfo, applying
// Open5e per-campaign gating and campaign-scoped homebrew filtering. Rows
// whose source starts with "open5e:" are kept only when their document slug
// is enabled on the campaign; SRD and homebrew rows are always included
// (provided the homebrew belongs to this campaign or is global).
func (a *RefDataAdapter) ListSpellsByClass(ctx context.Context, class, campaignID string) ([]SpellInfo, error) {
	spells, err := a.q.ListSpellsByClass(ctx, class)
	if err != nil {
		return nil, err
	}
	enabled := a.resolveEnabledOpen5eSources(campaignID)
	spells = open5e.FilterSpellsByOpen5eSources(spells, enabled)
	cid := parseCampaignUUID(campaignID)
	result := make([]SpellInfo, 0, len(spells))
	for _, s := range spells {
		if !campaignVisibleUUID(s.CampaignID, cid) {
			continue
		}
		result = append(result, SpellInfo{
			ID:          s.ID,
			Name:        s.Name,
			Level:       int(s.Level),
			School:      s.School,
			CastingTime: s.CastingTime,
			Duration:    s.Duration,
			Description: s.Description,
			Classes:     s.Classes,
		})
	}
	return result, nil
}

// resolveEnabledOpen5eSources returns the campaign's enabled Open5e
// document slugs or nil when no lookup is wired or the id is empty /
// unparseable. Returning nil strips all open5e:* rows, which is the safe
// default for requests with no campaign context.
func (a *RefDataAdapter) resolveEnabledOpen5eSources(campaignID string) []string {
	if a.lookup == nil || campaignID == "" {
		return nil
	}
	id, err := uuid.Parse(campaignID)
	if err != nil {
		return nil
	}
	return a.lookup.EnabledOpen5eSources(id)
}

// ListEquipment returns all weapons and armor combined as EquipmentItems,
// filtered by campaign_id so homebrew from other campaigns is not exposed.
func (a *RefDataAdapter) ListEquipment(ctx context.Context, campaignID string) ([]EquipmentItem, error) {
	weapons, err := a.q.ListWeapons(ctx)
	if err != nil {
		return nil, err
	}
	armors, err := a.q.ListArmor(ctx)
	if err != nil {
		return nil, err
	}

	cid := parseCampaignUUID(campaignID)
	items := make([]EquipmentItem, 0, len(weapons)+len(armors))
	for _, w := range weapons {
		if !campaignVisibleUUID(w.CampaignID, cid) {
			continue
		}
		items = append(items, EquipmentItem{
			ID:         w.ID,
			Name:       w.Name,
			Category:   "weapon",
			WeaponType: w.WeaponType,
			Damage:     w.Damage,
			DamageType: w.DamageType,
			Properties: w.Properties,
		})
	}
	for _, ar := range armors {
		items = append(items, EquipmentItem{
			ID:        ar.ID,
			Name:      ar.Name,
			Category:  "armor",
			ArmorType: ar.ArmorType,
			ACBase:    int(ar.AcBase),
		})
	}
	return items, nil
}

// parseCampaignUUID parses a campaign ID string into a uuid.UUID.
// Returns uuid.Nil on empty/invalid input.
func parseCampaignUUID(campaignID string) uuid.UUID {
	if campaignID == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(campaignID)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// campaignVisibleUUID returns true if a row should be visible to the given campaign.
func campaignVisibleUUID(rowCampaignID uuid.NullUUID, campaignID uuid.UUID) bool {
	if !rowCampaignID.Valid || rowCampaignID.UUID == uuid.Nil {
		return true
	}
	if campaignID == uuid.Nil {
		return false
	}
	return rowCampaignID.UUID == campaignID
}

func nullRawToJSON(msg pqtype.NullRawMessage) json.RawMessage {
	if !msg.Valid {
		return nil
	}
	return json.RawMessage(msg.RawMessage)
}
