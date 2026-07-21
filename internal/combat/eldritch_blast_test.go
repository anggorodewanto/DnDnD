package combat

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// seqD20Roller returns the given d20 faces in order (cycling) and the max value
// for every other die, so a test can force specific per-beam hit/miss results
// while keeping damage dice deterministic.
func seqD20Roller(d20s ...int) *dice.Roller {
	i := 0
	return dice.NewRoller(func(n int) int {
		if n == 20 {
			v := d20s[i%len(d20s)]
			i++
			return v
		}
		return 10
	})
}

func TestCast_EldritchBlast_FiresTwoIndependentBeams(t *testing.T) {
	svc, cmd, appliedHP := castEBSetup(t, true)
	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	require.Len(t, result.Beams, 2, "an L5 warlock fires 2 beams")
	for _, beam := range result.Beams {
		assert.True(t, beam.Hit)
		assert.Equal(t, 13, beam.Damage, "each beam: 1d10(10) + Agonizing CHA(+3)")
	}
	assert.Equal(t, 26, result.DamageTotal)
	assert.Equal(t, int32(50-26), *appliedHP,
		"two beams on one target apply once (no stale-snapshot clobber)")
}

func TestCast_EldritchBlast_PartialMiss_AgonizingRidesOnlyHitBeams(t *testing.T) {
	svc, cmd, _ := castEBSetup(t, true)
	// beam 1 rolls a 20 (hit); beam 2 rolls a 1 (miss).
	result, err := svc.Cast(context.Background(), cmd, seqD20Roller(20, 1))
	require.NoError(t, err)
	require.Len(t, result.Beams, 2)
	assert.True(t, result.Beams[0].Hit)
	assert.False(t, result.Beams[1].Hit)
	assert.Equal(t, 13, result.Beams[0].Damage, "hit beam: 1d10(10) + Agonizing(3)")
	assert.Equal(t, 0, result.Beams[1].Damage, "missed beam deals nothing")
	assert.Equal(t, 13, result.DamageTotal)

	var agonizing int
	for _, c := range result.DamageBreakdown {
		if c.SourceName == "Agonizing Blast" {
			agonizing = c.Amount
		}
	}
	assert.Equal(t, 3, agonizing, "Agonizing Blast rides only the beam that hit, not the whole cast")
}

func TestCast_EldritchBlast_SplitFire_DifferentTargets(t *testing.T) {
	charID := uuid.New()
	char := warlockWithAgonizing(charID, 5, true) // 2 beams, CHA +3
	caster := makeSpellCaster(charID)

	targetA := makeSpellTarget()
	targetA.DisplayName = "Goblin A"
	targetA.PositionRow = 6
	targetA.HpCurrent, targetA.HpMax = 40, 40

	targetB := makeSpellTarget()
	targetB.DisplayName = "Goblin B"
	targetB.PositionRow = 7
	targetB.HpCurrent, targetB.HpMax = 40, 40

	applied := map[uuid.UUID]int32{}
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeEldritchBlast(), nil }
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		switch id {
		case caster.ID:
			return caster, nil
		case targetA.ID:
			return targetA, nil
		case targetB.ID:
			return targetB, nil
		}
		return refdata.Combatant{}, fmt.Errorf("unknown combatant %s", id)
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		applied[arg.ID] = arg.HpCurrent
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
	}
	svc := NewService(store)
	cmd := CastCommand{
		SpellID:       "eldritch-blast",
		CasterID:      caster.ID,
		TargetID:      targetA.ID,
		BeamTargetIDs: []uuid.UUID{targetA.ID, targetB.ID},
		Turn:          refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}
	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	require.Len(t, result.Beams, 2)
	assert.Equal(t, "Goblin A", result.Beams[0].TargetName)
	assert.Equal(t, "Goblin B", result.Beams[1].TargetName)
	assert.Equal(t, int32(40-13), applied[targetA.ID], "beam 1 hits Goblin A for 1d10+CHA")
	assert.Equal(t, int32(40-13), applied[targetB.ID], "beam 2 hits Goblin B for 1d10+CHA")
	assert.Equal(t, 26, result.DamageTotal)
}

func TestFormatCastLog_EldritchBlastBeams(t *testing.T) {
	result := CastResult{
		CasterName: "Vale", SpellName: "Eldritch Blast", IsAttack: true,
		DamageType: "force", DamageTotal: 26,
		DamageBreakdown: []DamageComponent{{SourceName: "Agonizing Blast", Amount: 6, DamageType: "force"}},
		Beams: []BeamOutcome{
			{Index: 1, TargetName: "Goblin A", AttackRoll: 15, AttackTotal: 21, Hit: true, Damage: 13},
			{Index: 2, TargetName: "Goblin B", AttackRoll: 3, AttackTotal: 9, Hit: false},
		},
	}
	out := FormatCastLog(result)
	assert.Contains(t, out, "Beam 1 → Goblin A")
	assert.Contains(t, out, "Hit!")
	assert.Contains(t, out, "Beam 2 → Goblin B")
	assert.Contains(t, out, "Miss!")
	assert.Contains(t, out, "Total: 26 force")
	assert.Contains(t, out, "Agonizing Blast")
	assert.NotContains(t, out, "Attack: d20", "beam casts render per-beam lines, not the single-attack line")
}
