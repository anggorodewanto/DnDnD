package combat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// spell_marker.go holds the machinery shared by concentration-scoped "on-hit
// rider" spells — a caster marks a target with a source-tagged condition, and
// every hit that caster lands on the marked target adds bonus damage until
// concentration ends. Hex (hex.go) and Hunter's Mark (hunters_mark.go) are the
// two members today; each keeps its own condition/spell-ID constants and rider
// FeatureDefinition, and forwards the shared match/apply logic here so the
// drift-prone unmarshal + condition-write lives in one place.

// targetMarkedBySpell reports whether the target's conditions include this
// attacker's source-tagged spell marker — a condition matching condName, tagged
// with spellID and the attacker's combatant ID. Only the caster who placed the
// marker adds its bonus damage, so the source combatant must match.
func targetMarkedBySpell(targetConditions json.RawMessage, attackerID uuid.UUID, condName, spellID string) bool {
	if len(targetConditions) == 0 {
		return false
	}
	var conds []CombatCondition
	if err := json.Unmarshal(targetConditions, &conds); err != nil {
		return false
	}
	for _, c := range conds {
		if c.Condition == condName && c.SourceSpell == spellID && c.SourceCombatantID == attackerID.String() {
			return true
		}
	}
	return false
}

// newSpellMarkerCond builds a source-tagged marker condition — the single shape
// the on-hit rider matches on (targetMarkedBySpell). Shared by every writer of a
// marker: the /bonus move path, the DM dashboard endpoint (upsertSpellMarkerConds),
// and the on-cast apply (applySpellMarkerCondition).
func newSpellMarkerCond(condName, spellID, sourceCombatantID string) CombatCondition {
	return CombatCondition{
		Condition:         condName,
		SourceCombatantID: sourceCombatantID,
		SourceSpell:       spellID,
	}
}

// stripSpellMarkerConds returns conds with every (condName, spellID) marker
// placed by sourceCombatantID removed, leaving all other conditions intact.
// Shared by the /bonus move path (move_spell_marker.go) and the DM dashboard
// spell-marker endpoint (workspace_handler.go).
func stripSpellMarkerConds(conds []CombatCondition, condName, spellID, sourceCombatantID string) []CombatCondition {
	filtered := make([]CombatCondition, 0, len(conds))
	for _, c := range conds {
		if c.Condition == condName && c.SourceSpell == spellID && c.SourceCombatantID == sourceCombatantID {
			continue
		}
		filtered = append(filtered, c)
	}
	return filtered
}

// upsertSpellMarkerConds returns conds with any existing (condName, spellID)
// marker from sourceCombatantID replaced by a single fresh one, so re-stamping
// the same caster's marker is idempotent rather than stacking duplicates.
func upsertSpellMarkerConds(conds []CombatCondition, condName, spellID, sourceCombatantID string) []CombatCondition {
	return append(stripSpellMarkerConds(conds, condName, spellID, sourceCombatantID),
		newSpellMarkerCond(condName, spellID, sourceCombatantID))
}

// applySpellMarkerCondition marks the target with a source-tagged condition so
// the caster's subsequent attacks add the spell's rider while they concentrate
// (consumed by targetMarkedBySpell + the spell's rider FeatureDefinition). No-op
// without an explicit creature target. The marker is removed when concentration
// ends via RemoveSpellSourcedConditions (matched on caster ID + spell.ID).
func (s *Service) applySpellMarkerCondition(ctx context.Context, condName string, spell refdata.Spell, caster refdata.Combatant, targetID uuid.UUID) error {
	if targetID == uuid.Nil {
		return nil
	}
	cond := newSpellMarkerCond(condName, spell.ID, caster.ID.String())
	if _, _, err := s.ApplyCondition(ctx, targetID, cond); err != nil {
		return fmt.Errorf("applying %s condition: %w", condName, err)
	}
	return nil
}
