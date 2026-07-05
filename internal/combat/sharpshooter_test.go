package combat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
)

// Sharpshooter's passive rider #1: a ranged attack at long range suffers no
// disadvantage. Mirrors the HasCrossbowExpert removal of the hostile-near
// disadvantage — the flag is set whenever the attacker HAS the feat, not by
// the per-attack -5/+10 toggle.
func TestDetectAdvantage_SharpshooterNegatesLongRange(t *testing.T) {
	base := AdvantageInput{Weapon: makeLongbow(), DistanceFt: 200} // >150 → long range

	mode, _, disadv := DetectAdvantage(base)
	assert.Equal(t, dice.Disadvantage, mode, "no feat → long range imposes disadvantage")
	assert.Contains(t, disadv, "long range")

	base.HasSharpshooter = true
	mode2, _, disadv2 := DetectAdvantage(base)
	assert.Equal(t, dice.Normal, mode2, "Sharpshooter removes the long-range disadvantage")
	assert.NotContains(t, disadv2, "long range")
}

func sharpshooterCoverInput(cover CoverLevel, hasSS bool) AttackInput {
	return AttackInput{
		AttackerName: "Vax", TargetName: "Goblin",
		TargetAC: 15, Weapon: makeLongbow(),
		Scores: AbilityScores{Str: 10, Dex: 16}, ProfBonus: 3, // atk = +3 DEX +3 prof
		DistanceFt: 30, Cover: cover, HasSharpshooter: hasSS,
	}
}

// Sharpshooter's passive rider #2: ranged attacks ignore half and
// three-quarters cover (not full — that is blocked earlier at the service
// layer). d20=10 + 6 = 16; base AC 15.
func TestResolveAttack_SharpshooterIgnoresCover(t *testing.T) {
	// Half cover, no feat: +2 → AC 17, roll 16 misses.
	res, err := ResolveAttack(sharpshooterCoverInput(CoverHalf, false), newTestRoller(10))
	require.NoError(t, err)
	assert.Equal(t, 17, res.EffectiveAC, "half cover adds +2 without the feat")
	assert.False(t, res.Hit)

	// Half cover, feat: cover ignored → AC 15, roll 16 hits.
	res2, err := ResolveAttack(sharpshooterCoverInput(CoverHalf, true), newTestRoller(10))
	require.NoError(t, err)
	assert.Equal(t, 15, res2.EffectiveAC, "Sharpshooter ignores half cover")
	assert.True(t, res2.Hit)

	// Three-quarters cover, feat: +5 ignored → AC 15.
	res3, err := ResolveAttack(sharpshooterCoverInput(CoverThreeQuarters, true), newTestRoller(10))
	require.NoError(t, err)
	assert.Equal(t, 15, res3.EffectiveAC, "Sharpshooter ignores three-quarters cover")
}

// The cover-ignore is gated on a RANGED weapon: a Sharpshooter swinging a melee
// weapon still eats the cover AC bonus.
func TestResolveAttack_SharpshooterDoesNotIgnoreMeleeCover(t *testing.T) {
	in := AttackInput{
		AttackerName: "Grog", TargetName: "Goblin",
		TargetAC: 15, Weapon: makeLongsword(), // melee
		Scores: AbilityScores{Str: 16, Dex: 10}, ProfBonus: 3,
		DistanceFt: 5, Cover: CoverHalf, HasSharpshooter: true,
	}
	res, err := ResolveAttack(in, newTestRoller(10))
	require.NoError(t, err)
	assert.Equal(t, 17, res.EffectiveAC, "Sharpshooter only ignores cover for ranged weapons")
}
