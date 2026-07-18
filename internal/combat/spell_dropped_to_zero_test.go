package combat

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// FormatCastLog must fold the drop-to-0 line into the SAME message as the cast
// narrative, at the tail — so #combat-log shows the "defeated" line AFTER the
// spell that caused it, never before (the Eldritch Blast kill the DM reported).
// Mirrors FormatAttackLog: the line rides on the result (DownLogLine) and the
// service defers its own immediate post for the spell-damage path.
func TestFormatCastLog_AppendsDownLogLineAfterCast(t *testing.T) {
	result := CastResult{
		CasterName: "Windreth", SpellName: "Eldritch Blast", TargetName: "Goblin",
		IsAttack: true, Hit: true, DamageTotal: 26, DamageType: "force",
		ScaledDamageDice: "2d10", AttackRoll: 16, AttackTotal: 22,
		DownLogLine: "\U0001f480  Goblin drops to 0 HP — defeated",
	}
	out := FormatCastLog(result)
	require.Contains(t, out, "defeated")
	assert.Greater(t, strings.Index(out, "defeated"), strings.Index(out, "Damage:"),
		"drop-to-0 line must come after the damage line")
	assert.Greater(t, strings.Index(out, "defeated"), strings.Index(out, "casts"),
		"drop-to-0 line must come after the cast header")
}

// A cast that leaves the target standing carries no DownLogLine, so the cast
// narrative shows no drop-to-0 tail.
func TestFormatCastLog_NoDownLogLine_Omitted(t *testing.T) {
	result := CastResult{
		CasterName: "Windreth", SpellName: "Eldritch Blast", TargetName: "Goblin",
		IsAttack: true, Hit: true, DamageTotal: 4, DamageType: "force",
		ScaledDamageDice: "1d10",
	}
	assert.NotContains(t, FormatCastLog(result), "drops to 0")
}

// Service.Cast: a spell attack that drops the target to 0 HP must defer the
// #combat-log down-post (so the cast handler posts it as the tail of
// FormatCastLog, AFTER the cast) while still carrying the formatted line on the
// result. This is the Eldritch Blast path (spellcasting.go: apply spell damage).
func TestCast_SpellAttackDrop_DefersDownPost_CarriesLine(t *testing.T) {
	charID := uuid.New()
	char := warlockWithAgonizing(charID, 5, true) // 2 beams, CHA +3
	caster := makeSpellCaster(charID)
	target := makeSpellTarget() // NPC Goblin, AC 13
	target.HpCurrent = 6        // 2d10(20)+Agonizing(6)=26 damage → dropped

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
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
	}

	svc := NewService(store)
	cl := &fakeCombatLog{}
	svc.SetCombatLogNotifier(cl)

	cmd := CastCommand{SpellID: "eldritch-blast", CasterID: caster.ID, TargetID: target.ID, Turn: refdata.Turn{ID: uuid.New(), CombatantID: caster.ID}}
	result, err := svc.Cast(context.Background(), cmd, ebRiderRoller())
	require.NoError(t, err)
	require.True(t, result.Hit)

	assert.Empty(t, cl.all(), "deferred: Cast must NOT post the down-line itself — the handler posts it at the tail of FormatCastLog")
	assert.Contains(t, result.DownLogLine, "defeated", "the formatted drop-to-0 line is carried on the result")

	// The folded message renders the down-line after the cast.
	full := FormatCastLog(result)
	assert.Greater(t, strings.Index(full, "defeated"), strings.Index(full, "casts"))
}
