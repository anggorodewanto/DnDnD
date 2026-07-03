package combat

import (
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

const (
	eldritchBlastSpellID   = "eldritch-blast"
	agonizingBlastEffectID = "agonizing_blast"
)

// HasInvocation reports whether the character's features include a warlock
// invocation with the given mechanical-effect slug (e.g. "agonizing_blast").
// Invocations are persisted as Feature{Source:"invocation", MechanicalEffect:<slug>}
// in the same features JSONB column that holds feats and class features, so the
// existing slug matcher recognizes them.
func HasInvocation(features pqtype.NullRawMessage, slug string) bool {
	return hasFeatureEffect(features, slug)
}

// agonizingBlastBonus returns the extra damage the Agonizing Blast invocation
// adds to an Eldritch Blast cast: the caster's spellcasting-ability modifier
// per beam. Beam count follows cantrip scaling (1/2/3/4 at levels 1/5/11/17).
// Returns (0,false) unless the spell is Eldritch Blast AND the caster has the
// invocation.
func agonizingBlastBonus(spell refdata.Spell, char refdata.Character, spellAbilityScore int) (int, bool) {
	if spell.ID != eldritchBlastSpellID {
		return 0, false
	}
	if !HasInvocation(char.Features, agonizingBlastEffectID) {
		return 0, false
	}
	beams := CantripDiceMultiplier(int(char.Level)) // always >= 1
	return beams * AbilityModifier(spellAbilityScore), true
}
