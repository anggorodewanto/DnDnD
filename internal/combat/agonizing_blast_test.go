package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

func warlockWithAgonizing(id uuid.UUID, level int, hasAgonizing bool) refdata.Character {
	scoresJSON, _ := json.Marshal(AbilityScores{Str: 8, Dex: 14, Con: 12, Int: 10, Wis: 10, Cha: 16})
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "warlock", Level: level}})
	c := refdata.Character{
		ID: id, Name: "Vale", ProficiencyBonus: 3,
		Classes: classesJSON, AbilityScores: scoresJSON, Level: int32(level),
	}
	if hasAgonizing {
		featsJSON, _ := json.Marshal([]CharacterFeature{{Name: "Agonizing Blast", MechanicalEffect: "agonizing_blast"}})
		c.Features = pqtype.NullRawMessage{RawMessage: featsJSON, Valid: true}
	}
	return c
}

func makeEldritchBlast() refdata.Spell {
	return refdata.Spell{
		ID: "eldritch-blast", Name: "Eldritch Blast", Level: 0,
		CastingTime: "1 action", RangeType: "ranged",
		RangeFt:        sql.NullInt32{Int32: 120, Valid: true},
		AttackType:     sql.NullString{String: "ranged", Valid: true},
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
		Damage:         pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"dice":"1d10","type":"force","cantrip_scaling":true}`), Valid: true},
	}
}

func TestHasInvocation_DetectsAgonizingBlast(t *testing.T) {
	featsJSON, _ := json.Marshal([]CharacterFeature{{Name: "Agonizing Blast", MechanicalEffect: "agonizing_blast"}})
	feats := pqtype.NullRawMessage{RawMessage: featsJSON, Valid: true}
	assert.True(t, HasInvocation(feats, "agonizing_blast"))
	assert.False(t, HasInvocation(feats, "devils_sight"))

	other, _ := json.Marshal([]CharacterFeature{{Name: "Devil's Sight", MechanicalEffect: "devils_sight"}})
	assert.False(t, HasInvocation(pqtype.NullRawMessage{RawMessage: other, Valid: true}, "agonizing_blast"))
}

func TestAgonizingBlastBonus_PerBeamAndGating(t *testing.T) {
	eb := makeEldritchBlast()
	const chaScore = 16 // +3
	for _, tc := range []struct {
		level, want int
	}{{1, 3}, {5, 6}, {11, 9}, {17, 12}} {
		char := warlockWithAgonizing(uuid.New(), tc.level, true)
		bonus, ok := agonizingBlastBonus(eb, char, chaScore)
		require.True(t, ok, "level %d", tc.level)
		assert.Equal(t, tc.want, bonus, "level %d: beams * CHA(+3)", tc.level)
	}

	if _, ok := agonizingBlastBonus(eb, warlockWithAgonizing(uuid.New(), 5, false), chaScore); ok {
		t.Fatal("no invocation → no Agonizing bonus")
	}
	if _, ok := agonizingBlastBonus(makeFireBolt(), warlockWithAgonizing(uuid.New(), 5, true), chaScore); ok {
		t.Fatal("Agonizing Blast must not apply to a non-Eldritch-Blast cantrip")
	}
}

func castEBSetup(t *testing.T, hasAgonizing bool) (*Service, CastCommand, *int32) {
	t.Helper()
	charID := uuid.New()
	char := warlockWithAgonizing(charID, 5, hasAgonizing) // 2 beams, CHA +3 → +6
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()
	target.PositionRow = 6
	target.HpCurrent = 50
	target.HpMax = 50

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
	return svc, cmd, appliedHP
}

func TestCast_EldritchBlast_AgonizingAddsChaPerBeam(t *testing.T) {
	svc, cmd, appliedHP := castEBSetup(t, true)
	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	require.True(t, result.Hit)
	// testRoller = 10 per die → 2d10 = 20; Agonizing 2 beams * CHA(+3) = +6 → 26.
	assert.Equal(t, 26, result.DamageTotal, "2d10(20) + Agonizing(6)")

	var found bool
	for _, c := range result.DamageBreakdown {
		if c.SourceName == "Agonizing Blast" {
			found = true
			assert.Equal(t, 6, c.Amount)
			assert.Equal(t, "force", c.DamageType)
		}
	}
	assert.True(t, found, "Agonizing Blast must be called out: %+v", result.DamageBreakdown)
	assert.Equal(t, int32(50-26), *appliedHP, "the boosted total must reach the target's HP, not just DamageTotal")
}

func TestCast_EldritchBlast_NoInvocation_NoBonus(t *testing.T) {
	svc, cmd, _ := castEBSetup(t, false)
	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 20, result.DamageTotal, "no invocation → 2d10 only")
	for _, c := range result.DamageBreakdown {
		assert.NotEqual(t, "Agonizing Blast", c.SourceName)
	}
}
