package combat

import "github.com/sqlc-dev/pqtype"

// Tactical Master (2024 Fighter, level 9): "When you attack with a weapon whose
// mastery property you can use, you can replace that property with the Push,
// Sap, or Slow property for that attack." It substitutes one usable mastery for
// another — it never grants a mastery to a weapon that has none, and never lets
// a fighter apply a mastery on a weapon they don't already use (hence the
// masteryActive gate). The swapped slug then flows through the existing
// onHitMastery + applyMasteryEffects pipeline unchanged, so no new effect code
// is needed: Push/Sap/Slow already resolve end-to-end.

// tacticalMasteryEffect is the seeded mechanical_effect slug that gates the
// override. Detection is by slug via hasFeatureEffect (mirroring the Evasion /
// Uncanny Dodge class-feature gates and the level-9 seed guard), not by display
// name — the slug is the mechanical contract, so a future rewording of the
// feature's name can't silently break the override.
const tacticalMasteryEffect = "tactical_master"

// tacticalMasterySlugs is the closed set of mastery properties Tactical Master
// may substitute in.
var tacticalMasterySlugs = map[string]bool{"push": true, "sap": true, "slow": true}

// tacticalMasteryOverride returns the mastery slug to substitute for this attack,
// or "" when Tactical Master does not apply. It applies only when the fighter
// (a) chose a substitutable replacement (Push/Sap/Slow), (b) already uses this
// weapon's own mastery (masteryActive — "a weapon whose mastery property you can
// use"), and (c) carries the Tactical Master feature.
func tacticalMasteryOverride(choice string, input AttackInput, features pqtype.NullRawMessage) string {
	if !tacticalMasterySlugs[choice] {
		return ""
	}
	if !masteryActive(input) {
		return ""
	}
	if !hasFeatureEffect(features, tacticalMasteryEffect) {
		return ""
	}
	return choice
}
