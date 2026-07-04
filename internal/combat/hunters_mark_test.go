package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

func TestHuntersMarkFeature_Adds1d6Force(t *testing.T) {
	def := HuntersMarkFeature()
	require.Len(t, def.Effects, 1)
	e := def.Effects[0]
	assert.Equal(t, EffectExtraDamageDice, e.Type)
	assert.Equal(t, TriggerOnDamageRoll, e.Trigger)
	assert.Equal(t, "1d6", e.Dice)
	assert.Contains(t, e.DamageTypes, "force")
}

// huntersMarkRoller: d20=15 (hit), longsword 1d8 → 5, mark 1d6 → 4.
func huntersMarkRoller() *dice.Roller {
	return dice.NewRoller(func(maxN int) int {
		switch maxN {
		case 20:
			return 15
		case 8:
			return 5
		case 6:
			return 4
		default:
			return 2
		}
	})
}

func huntersMarkConditionsJSON(sourceCombatantID uuid.UUID) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`[{"condition":"hunters_mark","source_combatant_id":%q,"source_spell":"hunters-mark"}]`, sourceCombatantID.String()))
}

func huntersMarkSetup(t *testing.T, targetConditions json.RawMessage) (context.Context, *Service, AttackCommand, *dice.Roller) {
	t.Helper()
	charID := uuid.New()
	classes := []CharacterClass{{Class: "Ranger", Level: 5}}
	char := makeCharacterWithFeats(16, 12, 3, "longsword", nil, classes)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return makeLongsword(), nil }
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	svc := NewService(ms)

	encounterID := uuid.New()
	attackerID := uuid.New()
	attacker := refdata.Combatant{
		ID: attackerID, EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Kael", PositionCol: "A", PositionRow: 1,
		IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		DisplayName: "Grix", PositionCol: "B", PositionRow: 1, Ac: 14,
		IsAlive: true, IsNpc: true, IsVisible: true, Conditions: targetConditions,
	}
	turn := refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	cmd := AttackCommand{Attacker: attacker, Target: target, Turn: turn}
	return context.Background(), svc, cmd, huntersMarkRoller()
}

func TestServiceAttack_MarkedTarget_Adds1d6Force(t *testing.T) {
	ctx, svc, cmd, roller := huntersMarkSetup(t, nil)
	cmd.Target.Conditions = huntersMarkConditionsJSON(cmd.Attacker.ID) // marked by the attacker

	result, err := svc.Attack(ctx, cmd, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 12, result.DamageTotal, "1d8(5)+STR(3)+Hunter's Mark 1d6(4) = 12")

	var found bool
	for _, c := range result.DamageBreakdown {
		if c.SourceName == "Hunter's Mark" {
			found = true
			assert.Equal(t, 4, c.Amount)
			assert.Equal(t, "force", c.DamageType)
		}
	}
	assert.True(t, found, "Hunter's Mark must be called out: %+v", result.DamageBreakdown)
}

func TestServiceAttack_NotMarked_NoBonus(t *testing.T) {
	ctx, svc, cmd, roller := huntersMarkSetup(t, json.RawMessage(`[]`))
	result, err := svc.Attack(ctx, cmd, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 8, result.DamageTotal, "no mark → 1d8(5)+STR(3)")
}

func TestServiceAttack_MarkedBySomeoneElse_NoBonus(t *testing.T) {
	ctx, svc, cmd, roller := huntersMarkSetup(t, nil)
	// Target is marked, but by a different ranger — only the marker gets the bonus.
	cmd.Target.Conditions = huntersMarkConditionsJSON(uuid.New())
	result, err := svc.Attack(ctx, cmd, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 8, result.DamageTotal, "marked by another ranger → this attacker gets no bonus")
}

// Cast side: casting Hunter's Mark marks the target with a source-tagged
// "hunters_mark" condition that targetHuntersMarkedBy detects for the caster only.
func TestApplyHuntersMarkConditionFromCast_MarksTarget(t *testing.T) {
	ctx := context.Background()
	casterID := uuid.New()
	targetID := uuid.New()
	caster := refdata.Combatant{ID: casterID}
	spell := refdata.Spell{ID: "hunters-mark", Name: "Hunter's Mark"}

	var captured json.RawMessage
	ms := defaultMockStore()
	ms.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, DisplayName: "Grix", Conditions: json.RawMessage(`[]`)}, nil
	}
	ms.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		captured = arg.Conditions
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}
	svc := NewService(ms)

	require.NoError(t, svc.applyHuntersMarkConditionFromCast(ctx, spell, caster, targetID))
	assert.True(t, targetHuntersMarkedBy(captured, casterID), "mark must be detectable for the caster: %s", string(captured))
	assert.False(t, targetHuntersMarkedBy(captured, uuid.New()), "mark must not match a different attacker")
}

func TestApplyHuntersMarkConditionFromCast_NoTarget_NoOp(t *testing.T) {
	ctx := context.Background()
	called := false
	ms := defaultMockStore()
	ms.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		called = true
		return refdata.Combatant{ID: arg.ID}, nil
	}
	svc := NewService(ms)
	require.NoError(t, svc.applyHuntersMarkConditionFromCast(ctx, refdata.Spell{ID: "hunters-mark", Name: "Hunter's Mark"}, refdata.Combatant{ID: uuid.New()}, uuid.Nil))
	assert.False(t, called, "no target → no condition write")
}
