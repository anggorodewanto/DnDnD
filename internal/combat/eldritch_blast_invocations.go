package combat

import (
	"database/sql"

	"github.com/ab/dndnd/internal/refdata"
)

// Eldritch Blast rider invocations (COV-6). Agonizing Blast lives in
// agonizing_blast.go; this file wires the other two EB-cantrip riders that plug
// onto already-live machinery: Repelling Blast (push) and Eldritch Spear
// (range). Both gate on the caster carrying the invocation as a clean-slug
// Feature{MechanicalEffect:<id>} (refdata/invocation_catalog.go slug contract),
// matched via HasInvocation exactly like Agonizing Blast.
//
// The two pact-weapon riders (Thirsting Blade, Lifedrinker) are intentionally
// NOT here: both require Pact of the Blade weapon attacks, which have no combat
// consumer yet (COV-7). Wiring them belongs to that item.
const (
	repellingBlastEffectID = "repelling_blast"
	eldritchSpearEffectID  = "eldritch_spear"
	// eldritchSpearRangeFt is the Eldritch Blast range after Eldritch Spear
	// (2014/2024 PHB: 300 ft, up from the base 120).
	eldritchSpearRangeFt = 300
)

// castTriggersRepellingBlast reports whether an Eldritch Blast hit should push
// the target: the spell is Eldritch Blast AND the caster has the Repelling
// Blast invocation. The push itself reuses the shared applyPushEffect machinery
// (Push mastery / /shove), so this only decides eligibility. Mirrors
// agonizingBlastBonus's gate.
func castTriggersRepellingBlast(spell refdata.Spell, char refdata.Character) bool {
	return spell.ID == eldritchBlastSpellID && HasInvocation(char.Features, repellingBlastEffectID)
}

// applyEldritchSpearRange returns the spell with its range extended to 300 ft
// when it is Eldritch Blast and the caster has the Eldritch Spear invocation;
// otherwise the spell is returned unchanged. Callers pass the result to
// ValidateSpellRange so a caster with the invocation can reach a target the base
// 120 ft range would reject. This is a genuinely consumed effect: ValidateSpellRange
// is the live range chokepoint (unlike the Distant Spell metamagic, whose range
// extension is display-only today).
func applyEldritchSpearRange(spell refdata.Spell, char refdata.Character) refdata.Spell {
	if spell.ID != eldritchBlastSpellID || !HasInvocation(char.Features, eldritchSpearEffectID) {
		return spell
	}
	spell.RangeFt = sql.NullInt32{Int32: eldritchSpearRangeFt, Valid: true}
	return spell
}
