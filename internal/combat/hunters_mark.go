package combat

import (
	"encoding/json"

	"github.com/google/uuid"
)

// huntersMarkConditionName is the marker condition placed on a target while a
// ranger concentrates on Hunter's Mark against it. Tagged with SourceSpell
// "hunters-mark" and the caster's combatant ID so only that caster's attacks
// add the rider. Mirrors the Hex marker (hex.go).
const huntersMarkConditionName = "hunters_mark"

// huntersMarkSpellID is the reference-data spell ID for Hunter's Mark.
const huntersMarkSpellID = "hunters-mark"

// targetHuntersMarkedBy reports whether the target's conditions include this
// attacker's Hunter's Mark marker — a "hunters_mark" condition tagged with
// source_spell "hunters-mark" and the attacker's combatant ID. Only the ranger
// who cast the mark adds its bonus damage, so the source combatant must match.
// Shared match logic lives in targetMarkedBySpell (spell_marker.go).
func targetHuntersMarkedBy(targetConditions json.RawMessage, attackerID uuid.UUID) bool {
	return targetMarkedBySpell(targetConditions, attackerID, huntersMarkConditionName, huntersMarkSpellID)
}
