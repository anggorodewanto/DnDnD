package combat

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLifedrinkerFeature_FlatChaNecrotic(t *testing.T) {
	f := LifedrinkerFeature(4)
	require.Len(t, f.Effects, 1)
	e := f.Effects[0]
	assert.Equal(t, EffectModifyDamageRoll, e.Type) // flat modifier, not dice
	assert.Equal(t, TriggerOnDamageRoll, e.Trigger)
	assert.Equal(t, 4, e.Modifier)
	assert.Equal(t, []string{"necrotic"}, e.DamageTypes)
}

func TestServiceAttack_Lifedrinker_AddsChaNecroticOnPactWeaponHit(t *testing.T) {
	svc, cmd, roller := pactBladeAttackFixture(t,
		CharacterFeature{Name: "Pact of the Blade", MechanicalEffect: "pact_of_the_blade"},
		CharacterFeature{Name: "Lifedrinker", MechanicalEffect: "lifedrinker"},
	)
	result, err := svc.Attack(context.Background(), cmd, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	// club 1d4(3) + CHA(4, pact-blade weapon ability) + CHA(4, Lifedrinker) = 11
	assert.Equal(t, 11, result.DamageTotal)

	var found bool
	for _, c := range result.DamageBreakdown {
		if c.SourceName == "Lifedrinker" {
			assert.Equal(t, "necrotic", c.DamageType)
			assert.Equal(t, 4, c.Amount)
			found = true
		}
	}
	assert.True(t, found, "expected a Lifedrinker necrotic damage component")
}

func TestServiceAttack_Lifedrinker_RequiresPactBlade(t *testing.T) {
	// Lifedrinker without Pact of the Blade: the pact-weapon gate (PactBladeCHA) is
	// false, so no necrotic rider and the club falls back to STR(-1).
	svc, cmd, roller := pactBladeAttackFixture(t,
		CharacterFeature{Name: "Lifedrinker", MechanicalEffect: "lifedrinker"},
	)
	result, err := svc.Attack(context.Background(), cmd, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	// 3 (d4) + (-1 STR) = 2, no necrotic
	assert.Equal(t, 2, result.DamageTotal)
}
