package levelup

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// characterStoreAdapter wraps *refdata.Queries to satisfy the
// levelup.CharacterStore contract. The bridging is non-trivial because the
// service uses json.RawMessage for optional JSONB columns while sqlc
// generates pqtype.NullRawMessage, so we translate at the boundary.
type characterStoreAdapter struct {
	queries *refdata.Queries
}

// NewCharacterStoreAdapter returns a CharacterStore backed by the refdata
// sqlc queries. Both cmd/dndnd and integration tests should construct the
// adapter through this helper so the bridging logic lives in one place.
func NewCharacterStoreAdapter(q *refdata.Queries) CharacterStore {
	return &characterStoreAdapter{queries: q}
}

func (a *characterStoreAdapter) GetCharacterForLevelUp(ctx context.Context, id uuid.UUID) (*StoredCharacter, error) {
	row, err := a.queries.GetCharacterForLevelUp(ctx, id)
	if err != nil {
		return nil, err
	}
	return &StoredCharacter{
		ID:               row.ID,
		Name:             row.Name,
		DiscordUserID:    row.DiscordUserID,
		Level:            row.Level,
		HPMax:            row.HpMax,
		HPCurrent:        row.HpCurrent,
		ProficiencyBonus: row.ProficiencyBonus,
		Classes:          row.Classes,
		AbilityScores:    row.AbilityScores,
		SpellSlots:       rawFromNullable(row.SpellSlots),
		PactMagicSlots:   rawFromNullable(row.PactMagicSlots),
		Features:         rawFromNullable(row.Features),
	}, nil
}

func (a *characterStoreAdapter) UpdateCharacterStats(ctx context.Context, id uuid.UUID, update StatsUpdate) error {
	// Load the current row so we can preserve JSONB columns the caller did
	// not supply. This keeps the bridge "update only what changed" even
	// though the underlying SQL rewrites the whole column set. Without
	// this, a nil SpellSlots from Service.ApplyLevelUp (common for
	// non-casters) would clobber any existing slots.
	existing, err := a.queries.GetCharacterForLevelUp(ctx, id)
	if err != nil {
		return fmt.Errorf("loading character for stats update: %w", err)
	}

	spellSlots := existing.SpellSlots
	if len(update.SpellSlots) > 0 {
		spellSlots = pqtype.NullRawMessage{RawMessage: update.SpellSlots, Valid: true}
	}

	pactSlots := existing.PactMagicSlots
	if len(update.PactMagicSlots) > 0 {
		pactSlots = pqtype.NullRawMessage{RawMessage: update.PactMagicSlots, Valid: true}
	}

	features := existing.Features
	if len(update.Features) > 0 {
		features = pqtype.NullRawMessage{RawMessage: update.Features, Valid: true}
	}

	return a.queries.UpdateCharacterLevelUpStats(ctx, refdata.UpdateCharacterLevelUpStatsParams{
		ID:               id,
		Level:            int32(update.Level),
		HpMax:            int32(update.HPMax),
		HpCurrent:        int32(update.HPCurrent),
		ProficiencyBonus: int32(update.ProficiencyBonus),
		Classes:          update.Classes,
		SpellSlots:       spellSlots,
		PactMagicSlots:   pactSlots,
		Features:         features,
	})
}

func (a *characterStoreAdapter) UpdateAbilityScores(ctx context.Context, id uuid.UUID, scores json.RawMessage) error {
	return a.queries.UpdateCharacterAbilityScores(ctx, refdata.UpdateCharacterAbilityScoresParams{
		ID:            id,
		AbilityScores: scores,
	})
}

func (a *characterStoreAdapter) UpdateFeatures(ctx context.Context, id uuid.UUID, features json.RawMessage) error {
	return a.queries.UpdateCharacterFeaturesOnly(ctx, refdata.UpdateCharacterFeaturesOnlyParams{
		ID:       id,
		Features: pqtype.NullRawMessage{RawMessage: features, Valid: true},
	})
}

// classStoreAdapter wraps *refdata.Queries to satisfy levelup.ClassStore.
type classStoreAdapter struct {
	queries *refdata.Queries
}

// NewClassStoreAdapter returns a ClassStore backed by refdata.Queries.
func NewClassStoreAdapter(q *refdata.Queries) ClassStore {
	return &classStoreAdapter{queries: q}
}

func (a *classStoreAdapter) GetClassRefData(ctx context.Context, classID string) (*ClassRefData, error) {
	row, err := a.queries.GetClass(ctx, classID)
	if err != nil {
		return nil, err
	}

	ref := &ClassRefData{
		HitDie:        row.HitDie,
		SubclassLevel: int(row.SubclassLevel),
	}

	if row.Spellcasting.Valid && len(row.Spellcasting.RawMessage) > 0 {
		sc, scErr := parseClassSpellcasting(row.Spellcasting.RawMessage)
		if scErr != nil {
			return nil, fmt.Errorf("parsing spellcasting for %s: %w", classID, scErr)
		}
		ref.Spellcasting = sc
	}

	attacks, err := parseAttacksPerAction(row.AttacksPerAction)
	if err != nil {
		return nil, fmt.Errorf("parsing attacks_per_action for %s: %w", classID, err)
	}
	ref.AttacksPerAction = attacks

	if len(row.Subclasses) > 0 {
		var subs map[string]any
		if err := json.Unmarshal(row.Subclasses, &subs); err == nil {
			ref.Subclasses = subs
		}
	}

	if len(row.FeaturesByLevel) > 0 {
		features, err := parseFeaturesByLevel(row.FeaturesByLevel)
		if err == nil {
			ref.FeaturesByLevel = features
		}
	}

	return ref, nil
}

// rawFromNullable collapses a pqtype.NullRawMessage into a json.RawMessage,
// returning nil when the column is SQL NULL.
func rawFromNullable(m pqtype.NullRawMessage) json.RawMessage {
	if !m.Valid {
		return nil
	}
	return m.RawMessage
}

// parseClassSpellcasting decodes a row's spellcasting JSONB into the
// ClassSpellcasting struct used by level-up calculations. The DB stores keys
// "ability" and "slot_progression"; ClassSpellcasting's json tags expect
// "spell_ability" and "slot_progression", so we decode via an intermediate.
func parseClassSpellcasting(data json.RawMessage) (*character.ClassSpellcasting, error) {
	var raw struct {
		Ability         string `json:"ability"`
		SlotProgression string `json:"slot_progression"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return &character.ClassSpellcasting{
		SpellAbility:    raw.Ability,
		SlotProgression: raw.SlotProgression,
	}, nil
}

// parseAttacksPerAction turns a JSONB blob shaped like `{"1":1,"5":2}` into
// the int->int map the level-up calculator expects.
func parseAttacksPerAction(data json.RawMessage) (map[int]int, error) {
	if len(data) == 0 {
		return map[int]int{}, nil
	}
	var raw map[string]int
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := make(map[int]int, len(raw))
	for k, v := range raw {
		var lvl int
		if _, err := fmt.Sscanf(k, "%d", &lvl); err != nil {
			continue
		}
		out[lvl] = v
	}
	return out, nil
}

// parseFeaturesByLevel decodes the class features JSON into the nested map
// shape consumed by the level-up service. Each entry is a slice of Feature
// objects keyed by the (stringified) level at which it's granted.
func parseFeaturesByLevel(data json.RawMessage) (map[string][]character.Feature, error) {
	var raw map[string][]character.Feature
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
