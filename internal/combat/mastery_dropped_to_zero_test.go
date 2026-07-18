package combat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// FormatAttackLog must render a Cleave secondary-target drop-to-0 line right
// after the Cleave hit line, so #combat-log shows the second creature's
// "defeated" AFTER the Cleave that felled it, never before. The line rides on
// result.CleaveAttack.DownLogLine (the service defers its own immediate post).
func TestFormatAttackLog_CleaveDrop_RendersDownLineAfterCleaveHit(t *testing.T) {
	result := AttackResult{
		AttackerName: "Grok", TargetName: "Goblin #1", WeaponName: "greataxe",
		IsMelee: true, DistanceFt: 5, Hit: true, DamageTotal: 8, DamageType: "slashing",
		DamageDice: "1d12+3",
		CleaveAttack: &AttackResult{
			TargetName: "Goblin #2", Hit: true, DamageTotal: 5, DamageType: "slashing",
			DownLogLine: "\U0001f480  Goblin #2 drops to 0 HP — defeated",
		},
	}
	out := FormatAttackLog(result)
	require.Contains(t, out, "defeated")
	assert.Greater(t, strings.Index(out, "defeated"), strings.Index(out, "Cleave hits"),
		"the Cleave drop-to-0 line must come after the Cleave hit line")
}

// applyGrazeDamage: a Graze miss whose flat damage drops the target to 0 must
// defer the #combat-log down-post (the attack handler posts it at the tail of
// FormatAttackLog) and stamp the formatted line onto result.DownLogLine.
func TestApplyGrazeDamage_Drop_DefersDownPost_StampsLine(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	ms, _ := applyDamageMockStore()
	validTurnEncounter(ms, turnID)
	svc := NewService(ms)
	cl := &fakeCombatLog{}
	svc.SetCombatLogNotifier(cl)

	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Goblin",
		HpMax: 14, HpCurrent: 3, IsAlive: true, IsNpc: true,
		Conditions: json.RawMessage(`[]`),
	}
	result := &AttackResult{Hit: false, DamageTotal: 6, DamageType: "slashing"}
	require.NoError(t, svc.applyGrazeDamage(context.Background(), target, result))

	assert.Empty(t, cl.all(), "graze drop: no immediate down-post from the service — the handler posts it after the miss line")
	assert.Contains(t, result.DownLogLine, "defeated", "graze down-line stamped onto the result")
}

// Service.Attack via the Cleave path: when the auto-resolved secondary hit drops
// the second creature, the service must NOT post the "defeated" line inside the
// attack call (it would land above the /attack message). The line rides on
// result.CleaveAttack.DownLogLine instead, folded into FormatAttackLog.
func TestServiceAttack_CleaveDropsSecondCreature_DefersDownPost(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	primaryID := uuid.New()
	secondID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 10, 2, "greataxe")
	char.ID = charID
	char.CharacterData = charDataWithMasteries(`{"weapon_masteries":["greataxe"]}`)

	attacker := cleaveAttacker(charID, attackerID, encounterID)
	primary := cleavePrimaryTarget(primaryID, encounterID)
	second := cleaveSecondTarget(secondID, encounterID, "B", 2)
	second.HpCurrent = 3 // cleave d12(5), no mod → drops the second creature

	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return makeCleaveGreataxe(), nil }
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{attacker, primary, second}, nil
	}
	ms.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, Conditions: json.RawMessage(`[]`)}, nil
	}
	validTurnEncounter(ms, turnID)

	svc := NewService(ms)
	cl := &fakeCombatLog{}
	svc.SetCombatLogNotifier(cl)

	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18
		}
		return 5
	})
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: primary, Turn: turn}, roller)
	require.NoError(t, err)
	require.NotNil(t, result.CleaveAttack, "expected a Cleave secondary attack")
	require.True(t, result.CleaveAttack.Hit)

	assert.Empty(t, cl.all(), "cleave drop: the service must defer the down-post so it rides the /attack message")
	assert.Contains(t, result.CleaveAttack.DownLogLine, "defeated", "cleave down-line stamped onto result.CleaveAttack")
	assert.Greater(t, strings.Index(FormatAttackLog(result), "defeated"), strings.Index(FormatAttackLog(result), "Cleave hits"))
}
