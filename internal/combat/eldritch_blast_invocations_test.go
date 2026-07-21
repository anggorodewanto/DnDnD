package combat

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"encoding/json"

	"github.com/ab/dndnd/internal/refdata"
)

// warlockWithInvocations builds a warlock carrying the given invocation slugs as
// Feature{MechanicalEffect:<slug>} entries (the clean-slug shape the combat
// matchers read — see refdata/invocation_catalog.go slug contract). CHA 16 (+3),
// prof +3, mirrors warlockWithAgonizing.
func warlockWithInvocations(id uuid.UUID, level int, slugs ...string) refdata.Character {
	scoresJSON, _ := json.Marshal(AbilityScores{Str: 8, Dex: 14, Con: 12, Int: 10, Wis: 10, Cha: 16})
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "warlock", Level: level}})
	c := refdata.Character{
		ID: id, Name: "Vale", ProficiencyBonus: 3,
		Classes: classesJSON, AbilityScores: scoresJSON, Level: int32(level),
	}
	if len(slugs) > 0 {
		feats := make([]CharacterFeature, len(slugs))
		for i, s := range slugs {
			feats[i] = CharacterFeature{Name: s, MechanicalEffect: s}
		}
		featsJSON, _ := json.Marshal(feats)
		c.Features = pqtype.NullRawMessage{RawMessage: featsJSON, Valid: true}
	}
	return c
}

func TestCastTriggersRepellingBlast_Gating(t *testing.T) {
	eb := makeEldritchBlast()
	assert.True(t, castTriggersRepellingBlast(eb, warlockWithInvocations(uuid.New(), 5, "repelling_blast")),
		"Eldritch Blast + Repelling Blast invocation → push")
	assert.False(t, castTriggersRepellingBlast(eb, warlockWithInvocations(uuid.New(), 5)),
		"no invocation → no push")
	assert.False(t, castTriggersRepellingBlast(makeFireBolt(), warlockWithInvocations(uuid.New(), 5, "repelling_blast")),
		"Repelling Blast must not apply to a non-Eldritch-Blast cantrip")
}

func TestApplyEldritchSpearRange(t *testing.T) {
	eb := makeEldritchBlast()

	got := applyEldritchSpearRange(eb, warlockWithInvocations(uuid.New(), 5, "eldritch_spear"))
	assert.Equal(t, int32(300), got.RangeFt.Int32, "Eldritch Spear extends Eldritch Blast to 300 ft")

	got = applyEldritchSpearRange(eb, warlockWithInvocations(uuid.New(), 5))
	assert.Equal(t, int32(120), got.RangeFt.Int32, "no invocation → base 120 ft")

	fb := makeFireBolt()
	got = applyEldritchSpearRange(fb, warlockWithInvocations(uuid.New(), 5, "eldritch_spear"))
	assert.Equal(t, fb.RangeFt, got.RangeFt, "Eldritch Spear must not touch a non-Eldritch-Blast spell")
}

// ebPushCapture records whether the push store method was called and its params.
type ebPushCapture struct {
	called bool
	params refdata.UpdateCombatantPositionParams
}

func castEBRepellingSetup(t *testing.T, slugs ...string) (*Service, CastCommand, *ebPushCapture) {
	t.Helper()
	charID := uuid.New()
	char := warlockWithInvocations(charID, 5, slugs...)
	caster := makeSpellCaster(charID) // E5
	target := makeSpellTarget()       // col E
	target.PositionRow = 6            // caster E5 → target E6 (5 ft apart, in range)
	target.HpCurrent = 50
	target.HpMax = 50

	cap := &ebPushCapture{}
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
	store.updateCombatantPositionFn = func(_ context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
		cap.called = true
		cap.params = arg
		return refdata.Combatant{ID: arg.ID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow, Conditions: json.RawMessage(`[]`)}, nil
	}
	svc := NewService(store)
	cmd := CastCommand{SpellID: "eldritch-blast", CasterID: caster.ID, TargetID: target.ID, Turn: refdata.Turn{ID: uuid.New(), CombatantID: caster.ID}}
	return svc, cmd, cap
}

func TestCast_EldritchBlast_RepellingBlast_PushesTargetOnHit(t *testing.T) {
	svc, cmd, cap := castEBRepellingSetup(t, "repelling_blast")
	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	require.True(t, result.Hit, "EB must hit for Repelling Blast to trigger")
	assert.True(t, result.RepellingBlastPushed, "hit + invocation → target pushed")
	require.True(t, cap.called, "push must reach the position store")
	// caster E5, target E6. Both beams hit, and Repelling Blast pushes 10 ft
	// (2 squares) per beam that hits → 4 squares straight away → E10.
	assert.Equal(t, "E", cap.params.PositionCol)
	assert.Equal(t, int32(10), cap.params.PositionRow)
}

func TestCast_EldritchBlast_NoRepelling_NoPush(t *testing.T) {
	svc, cmd, cap := castEBRepellingSetup(t) // no invocations
	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.False(t, result.RepellingBlastPushed, "no invocation → no push")
	assert.False(t, cap.called, "position store must not be touched")
}

func TestCast_EldritchSpear_ExtendsEldritchBlastRange(t *testing.T) {
	charID := uuid.New()
	caster := makeSpellCaster(charID) // E5
	target := makeSpellTarget()       // col E
	target.PositionRow = 35           // 30 squares from row 5 → 150 ft (> base 120, < spear 300)
	target.HpCurrent = 50
	target.HpMax = 50

	newSvc := func(char refdata.Character) *Service {
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
		return NewService(store)
	}
	cmd := CastCommand{SpellID: "eldritch-blast", CasterID: caster.ID, TargetID: target.ID, Turn: refdata.Turn{ID: uuid.New(), CombatantID: caster.ID}}

	_, err := newSvc(warlockWithInvocations(charID, 5)).Cast(context.Background(), cmd, testRoller())
	require.Error(t, err, "150 ft is beyond base Eldritch Blast range")
	assert.Contains(t, err.Error(), "out of range")

	result, err := newSvc(warlockWithInvocations(charID, 5, "eldritch_spear")).Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err, "Eldritch Spear extends the range to 300 ft")
	assert.True(t, result.IsAttack)
}

func TestFormatCastLog_RepellingBlast(t *testing.T) {
	out := FormatCastLog(CastResult{
		CasterName: "Vale", SpellName: "Eldritch Blast",
		IsAttack: true, Hit: true, RepellingBlastPushed: true,
	})
	assert.Contains(t, out, "Repelling Blast")
	assert.Contains(t, out, "pushed")
}
