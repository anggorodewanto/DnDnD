package levelup

import "github.com/ab/dndnd/internal/character"

// hpPerLevelEffectSlug is the mechanical-effect type a feat carries when it adds
// hit points per character level. Tough (refdata/seed_feats.go) is the only such
// seeded feat today: +2 HP per level.
const hpPerLevelEffectSlug = "hp_plus_2_per_level"

// featMaxHPBonus returns the hit-point-maximum bonus a feat grants a character of
// the given total level: +2 per level for a feat carrying hpPerLevelEffectSlug
// (Tough), 0 otherwise. Detection is by effect slug, not feat name, so any future
// feat with the same effect earns it for free. COV-9.
func featMaxHPBonus(feat FeatInfo, totalLevel int32) int32 {
	for _, effect := range feat.MechanicalEffect {
		if effect["effect_type"] == hpPerLevelEffectSlug {
			return 2 * totalLevel
		}
	}
	return 0
}

// conHPDelta returns the hit-point-maximum change owed to a Constitution swing
// from oldScores to newScores for a character of the given total level. Each
// point of CON modifier is worth one hit point per level, mirroring
// character.CalculateHP's CON term. A feat that raises CON across an even
// boundary grants +totalLevel HP; an odd bump that leaves the modifier
// unchanged grants 0. COV-9.
func conHPDelta(oldScores, newScores character.AbilityScores, totalLevel int32) int32 {
	delta := character.AbilityModifier(newScores.CON) - character.AbilityModifier(oldScores.CON)
	return int32(delta) * totalLevel
}
