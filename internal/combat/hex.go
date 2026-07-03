package combat

import (
	"encoding/json"

	"github.com/google/uuid"
)

// hexConditionName is the marker condition placed on a target while a caster
// concentrates on Hex against it. Tagged with SourceSpell "hex" and the
// caster's combatant ID so only that caster's attacks add the rider.
const hexConditionName = "hexed"

// hexSpellID is the reference-data spell ID for Hex.
const hexSpellID = "hex"

// targetHexedBy reports whether the target's conditions include this attacker's
// Hex marker — a "hexed" condition tagged with source_spell "hex" and the
// attacker's combatant ID. Only the caster who placed the Hex adds its bonus
// damage, so the source combatant must match.
func targetHexedBy(targetConditions json.RawMessage, attackerID uuid.UUID) bool {
	if len(targetConditions) == 0 {
		return false
	}
	var conds []CombatCondition
	if err := json.Unmarshal(targetConditions, &conds); err != nil {
		return false
	}
	for _, c := range conds {
		if c.Condition == hexConditionName && c.SourceSpell == hexSpellID && c.SourceCombatantID == attackerID.String() {
			return true
		}
	}
	return false
}
