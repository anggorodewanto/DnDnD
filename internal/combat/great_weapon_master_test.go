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

func TestGreatWeaponMasterFeature_AddsProfBonusHeavyMeleeOncePerTurn(t *testing.T) {
	def := GreatWeaponMasterFeature(3)
	require.Len(t, def.Effects, 1)
	e := def.Effects[0]
	assert.Equal(t, EffectModifyDamageRoll, e.Type)
	assert.Equal(t, TriggerOnDamageRoll, e.Trigger)
	assert.Equal(t, 3, e.Modifier)
	assert.Equal(t, "melee", e.Conditions.AttackType)
	assert.Contains(t, e.Conditions.WeaponProperties, "heavy")
	assert.True(t, e.Conditions.OncePerTurn, "2024 GWM extra damage is once per turn")
}

// gwmChar builds a level-5 fighter (prof +3, STR 18) with the Great Weapon
// Master feat and a greataxe equipped.
func gwmChar(t *testing.T) refdata.Character {
	t.Helper()
	feats := []CharacterFeature{{Name: "Great Weapon Master", MechanicalEffect: `[{"effect_type":"bonus_action_attack_on_crit_or_kill"},{"effect_type":"power_attack_minus_5_plus_10","condition":"heavy_weapon"}]`}}
	classes := []CharacterClass{{Class: "Fighter", Level: 5}}
	return makeCharacterWithFeats(18, 10, 3, "greataxe", feats, classes)
}

func gwmMockStore(char refdata.Character) *mockStore {
	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return makeGreataxe(), nil }
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	return ms
}

func gwmCombatants(charID uuid.UUID) (refdata.Combatant, refdata.Combatant, refdata.Turn) {
	encounterID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	attacker := refdata.Combatant{
		ID: attackerID, EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Vale", PositionCol: "A", PositionRow: 1,
		IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID: targetID, EncounterID: encounterID,
		DisplayName: "Grix", PositionCol: "B", PositionRow: 1, Ac: 15,
		IsAlive: true, IsNpc: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	turn := refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	return attacker, target, turn
}

// d20=15 hit; greataxe 1d12 → 6; STR 18 (+4). Base weapon dmg = 10.
// With GWM opted-in: +profBonus(3) → 13, plus a "Great Weapon Master" breakdown.
func gwmRoller() *dice.Roller {
	return dice.NewRoller(func(maxN int) int {
		switch maxN {
		case 20:
			return 15
		case 12:
			return 6
		default:
			return 3
		}
	})
}

func TestServiceAttack_GWM2024_OptedIn_AddsProfBonusDamageAndCallout(t *testing.T) {
	ctx := context.Background()
	char := gwmChar(t)
	char.ID = uuid.New()
	svc := NewService(gwmMockStore(char))
	attacker, target, turn := gwmCombatants(char.ID)

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn, GWM2024: true}, gwmRoller())
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 13, result.DamageTotal, "1d12(6)+STR(4)+GWM PB(3) = 13")

	var found bool
	for _, c := range result.DamageBreakdown {
		if c.SourceName == "Great Weapon Master" {
			found = true
			assert.Equal(t, 3, c.Amount)
		}
	}
	assert.True(t, found, "GWM +PB must be called out in the damage breakdown: %+v", result.DamageBreakdown)
}

func TestServiceAttack_GWM2024_NotOptedIn_NoBonus(t *testing.T) {
	ctx := context.Background()
	char := gwmChar(t)
	char.ID = uuid.New()
	svc := NewService(gwmMockStore(char))
	attacker, target, turn := gwmCombatants(char.ID)

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn, GWM2024: false}, gwmRoller())
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 10, result.DamageTotal, "no GWM opt-in → 1d12(6)+STR(4) only")
	for _, c := range result.DamageBreakdown {
		assert.NotEqual(t, "Great Weapon Master", c.SourceName, "GWM must not fire when not opted in")
	}
}

func TestServiceAttack_GWM2024_NoFeat_NoBonus(t *testing.T) {
	ctx := context.Background()
	// Same build but WITHOUT the feat.
	classes := []CharacterClass{{Class: "Fighter", Level: 5}}
	char := makeCharacterWithFeats(18, 10, 3, "greataxe", nil, classes)
	char.ID = uuid.New()
	svc := NewService(gwmMockStore(char))
	attacker, target, turn := gwmCombatants(char.ID)

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn, GWM2024: true}, gwmRoller())
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 10, result.DamageTotal, "no GWM feat → opting in is a no-op")
}
