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

func TestHexFeature_Adds1d6Necrotic(t *testing.T) {
	def := HexFeature()
	require.Len(t, def.Effects, 1)
	e := def.Effects[0]
	assert.Equal(t, EffectExtraDamageDice, e.Type)
	assert.Equal(t, TriggerOnDamageRoll, e.Trigger)
	assert.Equal(t, "1d6", e.Dice)
	assert.Contains(t, e.DamageTypes, "necrotic")
}

// hexRoller: d20=15 (hit), longsword 1d8 → 5, hex 1d6 → 4.
func hexRoller() *dice.Roller {
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

func hexConditionsJSON(sourceCombatantID uuid.UUID) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`[{"condition":"hexed","source_combatant_id":%q,"source_spell":"hex"}]`, sourceCombatantID.String()))
}

func hexSetup(t *testing.T, targetConditions json.RawMessage) (context.Context, *Service, AttackCommand, *dice.Roller) {
	t.Helper()
	charID := uuid.New()
	classes := []CharacterClass{{Class: "Warlock", Level: 5}}
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
		DisplayName: "Vale", PositionCol: "A", PositionRow: 1,
		IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		DisplayName: "Grix", PositionCol: "B", PositionRow: 1, Ac: 14,
		IsAlive: true, IsNpc: true, IsVisible: true, Conditions: targetConditions,
	}
	turn := refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	cmd := AttackCommand{Attacker: attacker, Target: target, Turn: turn}
	return context.Background(), svc, cmd, hexRoller()
}

func TestServiceAttack_HexedTarget_Adds1d6Necrotic(t *testing.T) {
	ctx, svc, cmd, roller := hexSetup(t, nil)
	cmd.Target.Conditions = hexConditionsJSON(cmd.Attacker.ID) // hexed by the attacker

	result, err := svc.Attack(ctx, cmd, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 12, result.DamageTotal, "1d8(5)+STR(3)+Hex 1d6(4) = 12")

	var found bool
	for _, c := range result.DamageBreakdown {
		if c.SourceName == "Hex" {
			found = true
			assert.Equal(t, 4, c.Amount)
			assert.Equal(t, "necrotic", c.DamageType)
		}
	}
	assert.True(t, found, "Hex must be called out: %+v", result.DamageBreakdown)
}

func TestServiceAttack_NotHexed_NoBonus(t *testing.T) {
	ctx, svc, cmd, roller := hexSetup(t, json.RawMessage(`[]`))
	result, err := svc.Attack(ctx, cmd, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 8, result.DamageTotal, "no hex → 1d8(5)+STR(3)")
}

func TestServiceAttack_HexedBySomeoneElse_NoBonus(t *testing.T) {
	ctx, svc, cmd, roller := hexSetup(t, nil)
	// Target is hexed, but by a different caster — only the hexer gets the bonus.
	cmd.Target.Conditions = hexConditionsJSON(uuid.New())
	result, err := svc.Attack(ctx, cmd, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 8, result.DamageTotal, "hexed by another caster → this attacker gets no bonus")
}

// Cast side: casting Hex marks the target with a source-tagged "hexed"
// condition that targetHexedBy detects for the caster only.
func TestApplyHexConditionFromCast_MarksTargetHexed(t *testing.T) {
	ctx := context.Background()
	casterID := uuid.New()
	targetID := uuid.New()
	caster := refdata.Combatant{ID: casterID}
	spell := refdata.Spell{ID: "hex", Name: "Hex"}

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

	require.NoError(t, svc.applyHexConditionFromCast(ctx, spell, caster, targetID))
	assert.True(t, targetHexedBy(captured, casterID), "hex marker must be detectable for the caster: %s", string(captured))
	assert.False(t, targetHexedBy(captured, uuid.New()), "hex marker must not match a different attacker")
}

func TestApplyHexConditionFromCast_NoTarget_NoOp(t *testing.T) {
	ctx := context.Background()
	called := false
	ms := defaultMockStore()
	ms.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		called = true
		return refdata.Combatant{ID: arg.ID}, nil
	}
	svc := NewService(ms)
	require.NoError(t, svc.applyHexConditionFromCast(ctx, refdata.Spell{ID: "hex", Name: "Hex"}, refdata.Combatant{ID: uuid.New()}, uuid.Nil))
	assert.False(t, called, "no target → no condition write")
}
