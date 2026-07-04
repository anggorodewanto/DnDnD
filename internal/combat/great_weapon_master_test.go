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

// gwmCritRoller forces a natural 20 (critical hit) and stable rider rolls.
func gwmCritRoller() *dice.Roller {
	return dice.NewRoller(func(maxN int) int {
		switch maxN {
		case 20:
			return 20
		case 12:
			return 6
		default:
			return 3
		}
	})
}

// TODO 3 — GWM 2024 bonus-action attack eligibility. A crit with a heavy melee
// weapon by a Great Weapon Master feat-holder flags the follow-up bonus attack.
func TestServiceAttack_GWMBonusAttack_EligibleOnCrit(t *testing.T) {
	ctx := context.Background()
	char := gwmChar(t)
	char.ID = uuid.New()
	svc := NewService(gwmMockStore(char))
	attacker, target, turn := gwmCombatants(char.ID)
	target.HpCurrent = 50 // survives the crit, so eligibility rides on the crit alone

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn}, gwmCritRoller())
	require.NoError(t, err)
	require.True(t, result.CriticalHit, "nat-20 must crit")
	assert.True(t, result.PromptGWMBonusAttackEligible, "crit with heavy melee + GWM feat is eligible for the bonus attack")
}

// Reducing the target to 0 HP with a heavy melee weapon flags the bonus attack
// even on a non-crit hit.
func TestServiceAttack_GWMBonusAttack_EligibleOnDropToZero(t *testing.T) {
	ctx := context.Background()
	char := gwmChar(t)
	char.ID = uuid.New()
	svc := NewService(gwmMockStore(char))
	attacker, target, turn := gwmCombatants(char.ID)
	target.HpCurrent = 8 // 1d12(6)+STR(4)=10 damage drops it to 0

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn}, gwmRoller())
	require.NoError(t, err)
	require.True(t, result.Hit)
	require.False(t, result.CriticalHit, "d20=15 is not a crit")
	assert.True(t, result.PromptGWMBonusAttackEligible, "dropping the target to 0 HP with a heavy weapon is eligible")
}

// A plain hit that neither crits nor drops the target to 0 is NOT eligible.
func TestServiceAttack_GWMBonusAttack_NotEligibleOnPlainHit(t *testing.T) {
	ctx := context.Background()
	char := gwmChar(t)
	char.ID = uuid.New()
	svc := NewService(gwmMockStore(char))
	attacker, target, turn := gwmCombatants(char.ID)
	target.HpCurrent = 50 // survives

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn}, gwmRoller())
	require.NoError(t, err)
	require.True(t, result.Hit)
	require.False(t, result.CriticalHit)
	assert.False(t, result.PromptGWMBonusAttackEligible, "plain surviving hit is not eligible")
}

// A miss is never eligible even against a healthy target (guards the drop-to-0
// signal against the applyHitDamage no-op path).
func TestServiceAttack_GWMBonusAttack_NotEligibleOnMiss(t *testing.T) {
	ctx := context.Background()
	char := gwmChar(t)
	char.ID = uuid.New()
	svc := NewService(gwmMockStore(char))
	attacker, target, turn := gwmCombatants(char.ID)
	target.Ac = 30 // d20=15 misses
	target.HpCurrent = 50

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn}, gwmRoller())
	require.NoError(t, err)
	require.False(t, result.Hit)
	assert.False(t, result.PromptGWMBonusAttackEligible, "a miss is never eligible")
}

// Without the GWM feat, a crit with a heavy weapon is not eligible.
func TestServiceAttack_GWMBonusAttack_NotEligibleWithoutFeat(t *testing.T) {
	ctx := context.Background()
	classes := []CharacterClass{{Class: "Fighter", Level: 5}}
	char := makeCharacterWithFeats(18, 10, 3, "greataxe", nil, classes)
	char.ID = uuid.New()
	svc := NewService(gwmMockStore(char))
	attacker, target, turn := gwmCombatants(char.ID)
	target.HpCurrent = 50

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn}, gwmCritRoller())
	require.NoError(t, err)
	require.True(t, result.CriticalHit)
	assert.False(t, result.PromptGWMBonusAttackEligible, "no GWM feat → not eligible")
}

// A non-heavy weapon (shortsword) is not eligible even on a crit by a feat-holder.
func TestServiceAttack_GWMBonusAttack_NotEligibleNonHeavyWeapon(t *testing.T) {
	ctx := context.Background()
	feats := []CharacterFeature{{Name: "Great Weapon Master", MechanicalEffect: `[{"effect_type":"bonus_action_attack_on_crit_or_kill"}]`}}
	classes := []CharacterClass{{Class: "Fighter", Level: 5}}
	char := makeCharacterWithFeats(18, 10, 3, "shortsword", feats, classes)
	char.ID = uuid.New()
	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return makeShortsword(), nil }
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	svc := NewService(ms)
	attacker, target, turn := gwmCombatants(char.ID)
	target.HpCurrent = 50

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn}, gwmCritRoller())
	require.NoError(t, err)
	require.True(t, result.CriticalHit)
	assert.False(t, result.PromptGWMBonusAttackEligible, "shortsword is not heavy → not eligible")
}

// GWMBonusAttack makes one swing with the main-hand heavy weapon at the full
// ability modifier and spends the bonus action.
func TestServiceGWMBonusAttack_SwingsMainWeaponAtFullMod(t *testing.T) {
	ctx := context.Background()
	char := gwmChar(t)
	char.ID = uuid.New()
	svc := NewService(gwmMockStore(char))
	attacker, target, turn := gwmCombatants(char.ID)
	target.HpCurrent = 50

	result, err := svc.GWMBonusAttack(ctx, GWMBonusAttackCommand{Attacker: attacker, Target: target, Turn: turn}, gwmRoller())
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, "Greataxe", result.WeaponName)
	assert.Equal(t, 10, result.DamageTotal, "1d12(6)+STR(4) at full modifier")
}

func TestServiceGWMBonusAttack_RequiresFeat(t *testing.T) {
	ctx := context.Background()
	classes := []CharacterClass{{Class: "Fighter", Level: 5}}
	char := makeCharacterWithFeats(18, 10, 3, "greataxe", nil, classes)
	char.ID = uuid.New()
	svc := NewService(gwmMockStore(char))
	attacker, target, turn := gwmCombatants(char.ID)

	_, err := svc.GWMBonusAttack(ctx, GWMBonusAttackCommand{Attacker: attacker, Target: target, Turn: turn}, gwmRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "feat")
}

func TestServiceGWMBonusAttack_RequiresHeavyMeleeWeapon(t *testing.T) {
	ctx := context.Background()
	feats := []CharacterFeature{{Name: "Great Weapon Master", MechanicalEffect: `[{"effect_type":"bonus_action_attack_on_crit_or_kill"}]`}}
	classes := []CharacterClass{{Class: "Fighter", Level: 5}}
	char := makeCharacterWithFeats(18, 10, 3, "shortsword", feats, classes)
	char.ID = uuid.New()
	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return makeShortsword(), nil }
	svc := NewService(ms)
	attacker, target, turn := gwmCombatants(char.ID)

	_, err := svc.GWMBonusAttack(ctx, GWMBonusAttackCommand{Attacker: attacker, Target: target, Turn: turn}, gwmRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "heavy melee weapon")
}

func TestServiceGWMBonusAttack_RequiresBonusActionAvailable(t *testing.T) {
	ctx := context.Background()
	char := gwmChar(t)
	char.ID = uuid.New()
	svc := NewService(gwmMockStore(char))
	attacker, target, turn := gwmCombatants(char.ID)
	turn.BonusActionUsed = true

	_, err := svc.GWMBonusAttack(ctx, GWMBonusAttackCommand{Attacker: attacker, Target: target, Turn: turn}, gwmRoller())
	require.Error(t, err)
}
