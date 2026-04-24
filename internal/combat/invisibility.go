package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/ab/dndnd/internal/refdata"
)

// InvisibilitySpellID is the spell ID for the standard Invisibility spell.
// Standard Invisibility breaks when the affected creature attacks or casts a spell.
const InvisibilitySpellID = "invisibility"

// GreaterInvisibilitySpellID is the spell ID for Greater Invisibility.
// Greater Invisibility persists through attacks/casts; it ends only via
// duration expiration or concentration loss.
const GreaterInvisibilitySpellID = "greater-invisibility"

// BreakInvisibilityOnAction removes an "invisible" condition whose SourceSpell
// matches standard Invisibility (the non-Greater variant). Greater Invisibility
// and non-spell sources are preserved. Returns the updated conditions JSONB,
// a flag indicating whether a condition was removed, and any parse error.
func BreakInvisibilityOnAction(conditions json.RawMessage) (json.RawMessage, bool, error) {
	conds, err := parseConditions(conditions)
	if err != nil {
		return nil, false, err
	}
	filtered := make([]CombatCondition, 0, len(conds))
	removed := false
	for _, c := range conds {
		if c.Condition == "invisible" && c.SourceSpell == InvisibilitySpellID {
			removed = true
			continue
		}
		filtered = append(filtered, c)
	}
	updated, err := json.Marshal(filtered)
	if err != nil {
		return nil, false, err
	}
	return updated, removed, nil
}

// ValidateSeeTarget enforces the "spells requiring to see the target" restriction:
// single-target, non-AoE, non-self spells cannot target creatures with the
// invisible condition. Returns a user-facing error when blocked.
func ValidateSeeTarget(spell refdata.Spell, target refdata.Combatant) error {
	if !HasCondition(target.Conditions, "invisible") {
		return nil
	}
	isAoE := spell.AreaOfEffect.Valid && len(spell.AreaOfEffect.RawMessage) > 0
	isSelf := spell.RangeType == "self" || spell.RangeType == "self (radius)"
	if isAoE || isSelf {
		return nil
	}
	return fmt.Errorf("⚠️ You can't target %s — they are invisible and you can't see them.", target.DisplayName)
}

// breakInvisibilityAndPersist removes a non-Greater Invisibility condition
// from the given combatant and persists the updated conditions. Returns true
// if a condition was actually removed. Shared between attack and cast paths.
func (s *Service) breakInvisibilityAndPersist(ctx context.Context, c refdata.Combatant) (bool, error) {
	updatedConds, broken, err := BreakInvisibilityOnAction(c.Conditions)
	if err != nil {
		return false, fmt.Errorf("checking invisibility break: %w", err)
	}
	if !broken {
		return false, nil
	}
	if _, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              c.ID,
		Conditions:      updatedConds,
		ExhaustionLevel: c.ExhaustionLevel,
	}); err != nil {
		return false, fmt.Errorf("breaking invisibility: %w", err)
	}
	return true, nil
}

// invisibilitySpellFixture builds a minimal test fixture for the Invisibility
// or Greater Invisibility spell. Test helper only.
func invisibilitySpellFixture(id, name string, level int32, concentration bool) refdata.Spell {
	return refdata.Spell{
		ID:                id,
		Name:              name,
		Level:             level,
		CastingTime:       "1 action",
		RangeType:         "touch",
		Components:        []string{"V", "S"},
		Duration:          "Concentration, up to 1 hour",
		Concentration:     sql.NullBool{Bool: concentration, Valid: true},
		ResolutionMode:    "auto",
		ConditionsApplied: []string{"invisible"},
	}
}
