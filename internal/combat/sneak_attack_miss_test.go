package combat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Bug 3: a MISS must not burn a once-per-turn damage rider ---
//
// ResolveAttack harvested every OncePerTurn on-damage effect straight off the
// Feature Effect System result, which is computed BEFORE the d20 is rolled. The
// miss path then returned with those effects still sitting in
// OncePerTurnEffectsFired, and Service.Attack / OffhandAttack call
// markUsedEffects on them unconditionally — so a Rogue who whiffed their first
// swing lost Sneak Attack for the whole turn. RAW: "Once per turn you can deal
// extra damage to one creature you HIT."
//
// Frenzy already had this right (recordFrenzy is hit-only, see
// TestResolveAttack_Frenzy_MissDoesNotSpendIt); the generic FES riders did not.

// sneakAttackResolveInput is a finesse shortsword swing by a rogue with
// advantage, so Sneak Attack's AdvantageOrAllyWithin condition passes and the
// only variable left is whether the attack lands.
func sneakAttackResolveInput() AttackInput {
	return AttackInput{
		AttackerName: "Windreth",
		TargetName:   "The House Collector",
		TargetAC:     16,
		Weapon: refdata.Weapon{
			ID: "shortsword", Name: "Shortsword",
			Damage: "1d6", DamageType: "piercing",
			Properties: []string{"finesse", "light"},
		},
		Scores:         AbilityScores{Str: 10, Dex: 18},
		ProfBonus:      3,
		DistanceFt:     5,
		AbilityUsed:    "dex",
		AttackerHidden: true, // hidden attacker → advantage
		Features:       []FeatureDefinition{SneakAttackFeature(5)},
		UsedThisTurn:   map[string]bool{},
	}
}

// sneakRoller: d20 drives the hit, d6 the shortsword + Sneak Attack dice.
func sneakRoller(d20, d6 int) *dice.Roller {
	return dice.NewRoller(func(maxN int) int {
		if maxN == 20 {
			return d20
		}
		return d6
	})
}

func TestResolveAttack_Miss_DoesNotConsumeSneakAttack(t *testing.T) {
	// d20 2 (with advantage, both dice are 2) + DEX 4 + prof 3 = 9 < AC 16.
	result, err := ResolveAttack(sneakAttackResolveInput(), sneakRoller(2, 3))

	require.NoError(t, err)
	require.False(t, result.Hit, "fixture must miss for this test to mean anything")
	assert.NotContains(t, result.OncePerTurnEffectsFired, string(EffectExtraDamageDice),
		"a miss must not mark Sneak Attack used for the turn")
	assert.NotContains(t, result.OncePerTurnEffectNames, "Sneak Attack")
}

func TestResolveAttack_Hit_StillConsumesSneakAttack(t *testing.T) {
	// d20 18 + DEX 4 + prof 3 = 25 >= AC 16.
	result, err := ResolveAttack(sneakAttackResolveInput(), sneakRoller(18, 3))

	require.NoError(t, err)
	require.True(t, result.Hit, "fixture must hit for this test to mean anything")
	assert.Contains(t, result.OncePerTurnEffectsFired, string(EffectExtraDamageDice),
		"a hit still spends the once-per-turn rider")
	assert.Contains(t, result.OncePerTurnEffectNames, "Sneak Attack")
}
