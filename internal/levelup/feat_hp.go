package levelup

import "github.com/ab/dndnd/internal/character"

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
