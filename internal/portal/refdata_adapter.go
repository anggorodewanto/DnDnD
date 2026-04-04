package portal

import (
	"context"
	"encoding/json"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/sqlc-dev/pqtype"
)

// RefDataQuerier is the subset of refdata.Queries methods needed by the portal.
type RefDataQuerier interface {
	ListRaces(ctx context.Context) ([]refdata.Race, error)
	ListClasses(ctx context.Context) ([]refdata.Class, error)
	ListSpellsByClass(ctx context.Context, class string) ([]refdata.Spell, error)
}

// RefDataAdapter adapts refdata.Queries to the portal RefDataStore interface.
type RefDataAdapter struct {
	q RefDataQuerier
}

// NewRefDataAdapter creates a new RefDataAdapter.
func NewRefDataAdapter(q RefDataQuerier) *RefDataAdapter {
	return &RefDataAdapter{q: q}
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
			ID:                c.ID,
			Name:              c.Name,
			HitDie:            c.HitDie,
			PrimaryAbility:    c.PrimaryAbility,
			SaveProficiencies: c.SaveProficiencies,
			SkillChoices:      nullRawToJSON(c.SkillChoices),
			Spellcasting:      nullRawToJSON(c.Spellcasting),
			Subclasses:        c.Subclasses,
			SubclassLevel:     int(c.SubclassLevel),
		}
	}
	return result, nil
}

// ListSpellsByClass converts refdata spells to portal SpellInfo.
func (a *RefDataAdapter) ListSpellsByClass(ctx context.Context, class string) ([]SpellInfo, error) {
	spells, err := a.q.ListSpellsByClass(ctx, class)
	if err != nil {
		return nil, err
	}
	result := make([]SpellInfo, len(spells))
	for i, s := range spells {
		result[i] = SpellInfo{
			ID:          s.ID,
			Name:        s.Name,
			Level:       int(s.Level),
			School:      s.School,
			CastingTime: s.CastingTime,
			Duration:    s.Duration,
			Description: s.Description,
			Classes:     s.Classes,
		}
	}
	return result, nil
}

func nullRawToJSON(msg pqtype.NullRawMessage) json.RawMessage {
	if !msg.Valid {
		return nil
	}
	return json.RawMessage(msg.RawMessage)
}
