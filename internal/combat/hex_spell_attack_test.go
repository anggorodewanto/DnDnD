package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// ebRiderRoller: every die returns 10 (d20 hit, d10 beam) except a d6 returns 4,
// so a Hex / Hunter's Mark 1d6 rider is distinguishable in the damage total.
func ebRiderRoller() *dice.Roller {
	return dice.NewRoller(func(n int) int {
		if n == 6 {
			return 4
		}
		return 10
	})
}

// runEBCastAgainst casts a level-5 Agonizing-Blast Eldritch Blast (2 beams,
// CHA +3) at a 50-HP target whose conditions are produced by condFn(casterID).
// Returns the cast result and the HP written to the target.
func runEBCastAgainst(t *testing.T, condFn func(casterID uuid.UUID) json.RawMessage) (CastResult, int32) {
	t.Helper()
	charID := uuid.New()
	char := warlockWithAgonizing(charID, 5, true) // 2 beams, CHA +3 → Agonizing +6
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()
	target.HpCurrent = 50
	target.HpMax = 50
	if condFn != nil {
		target.Conditions = condFn(caster.ID)
	}

	appliedHP := new(int32)
	*appliedHP = -1
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeEldritchBlast(), nil }
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		*appliedHP = arg.HpCurrent
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
	}
	svc := NewService(store)
	cmd := CastCommand{SpellID: "eldritch-blast", CasterID: caster.ID, TargetID: target.ID, Turn: refdata.Turn{ID: uuid.New(), CombatantID: caster.ID}}
	result, err := svc.Cast(context.Background(), cmd, ebRiderRoller())
	require.NoError(t, err)
	return result, *appliedHP
}

func breakdownComponent(t *testing.T, result CastResult, name string) (DamageComponent, bool) {
	t.Helper()
	for _, c := range result.DamageBreakdown {
		if c.SourceName == name {
			return c, true
		}
	}
	return DamageComponent{}, false
}

// A hexed target hit by Eldritch Blast takes the +1d6 necrotic Hex rider, just
// like a weapon hit does — the spell-attack path must honour the marker too.
func TestCast_EldritchBlast_HexedTarget_Adds1d6Necrotic(t *testing.T) {
	result, appliedHP := runEBCastAgainst(t, hexConditionsJSON)
	require.True(t, result.Hit)
	// Per beam: 1d10(10) + Agonizing(CHA+3) + Hex 1d6(4) = 17; both beams hit the
	// hexed target, so Hex rides EACH beam (RAW: "whenever you hit"): 2*17 = 34.
	assert.Equal(t, 34, result.DamageTotal, "2*(1d10(10)+Agonizing(3)+Hex(4))")
	assert.Equal(t, int32(50-34), appliedHP, "the Hex die must reach the target's HP, not just the log")

	hex, found := breakdownComponent(t, result, "Hex")
	require.True(t, found, "Hex must be called out: %+v", result.DamageBreakdown)
	assert.Equal(t, 8, hex.Amount, "1d6(4) per hit beam * 2 beams")
	assert.Equal(t, "necrotic", hex.DamageType)
}

func TestCast_EldritchBlast_NotHexed_NoNecrotic(t *testing.T) {
	result, _ := runEBCastAgainst(t, nil)
	require.True(t, result.Hit)
	assert.Equal(t, 26, result.DamageTotal, "no hex → 2d10(20)+Agonizing(6) only")
	_, found := breakdownComponent(t, result, "Hex")
	assert.False(t, found, "no Hex marker → no Hex component: %+v", result.DamageBreakdown)
}

// Only the caster who cast Hex adds the rider — a mark from a different caster
// must not ride this caster's Eldritch Blast.
func TestCast_EldritchBlast_HexedByAnotherCaster_NoNecrotic(t *testing.T) {
	result, _ := runEBCastAgainst(t, func(_ uuid.UUID) json.RawMessage {
		return hexConditionsJSON(uuid.New()) // hexed, but by someone else
	})
	require.True(t, result.Hit)
	assert.Equal(t, 26, result.DamageTotal, "hexed by another caster → no rider for this caster")
	_, found := breakdownComponent(t, result, "Hex")
	assert.False(t, found)
}

// Hunter's Mark is the symmetric rider: a marked target hit by a spell attack
// takes +1d6 force, mirroring the weapon path.
func TestCast_EldritchBlast_HuntersMarkedTarget_Adds1d6Force(t *testing.T) {
	result, appliedHP := runEBCastAgainst(t, huntersMarkConditionsJSON)
	require.True(t, result.Hit)
	// Hunter's Mark rides each hit beam, mirroring the per-beam Hex rider.
	assert.Equal(t, 34, result.DamageTotal, "2*(1d10(10)+Agonizing(3)+Hunter's Mark(4))")
	assert.Equal(t, int32(50-34), appliedHP)

	hm, found := breakdownComponent(t, result, "Hunter's Mark")
	require.True(t, found, "Hunter's Mark must be called out: %+v", result.DamageBreakdown)
	assert.Equal(t, 8, hm.Amount, "1d6(4) per hit beam * 2 beams")
	assert.Equal(t, "force", hm.DamageType)
}
