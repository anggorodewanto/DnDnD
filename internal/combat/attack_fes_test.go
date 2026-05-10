package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// makeRagingBarbarianFixture builds the common service+attacker+target+turn used by
// the FES integration tests. The barbarian is level 5 (rage damage bonus +2),
// equipped with a longsword (melee STR weapon) and is already raging.
func makeRagingBarbarianFixture(t *testing.T) (*Service, refdata.Combatant, refdata.Combatant, refdata.Turn, uuid.UUID) {
	t.Helper()

	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	classes := []CharacterClass{{Class: "Barbarian", Level: 5}}
	feats := []CharacterFeature{{Name: "Rage", MechanicalEffect: "rage"}}
	char := makeCharacterWithFeats(16, 12, 3, "longsword", feats, classes)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeLongsword(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)

	attacker := refdata.Combatant{
		ID:          attackerID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Grog",
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
		IsRaging:    true,
	}
	target := refdata.Combatant{
		ID:          targetID,
		EncounterID: encounterID,
		DisplayName: "Goblin",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          12,
		IsAlive:     true,
		IsNpc:       true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	turn := refdata.Turn{
		ID:               turnID,
		EncounterID:      encounterID,
		CombatantID:      attackerID,
		AttacksRemaining: 1,
	}

	return svc, attacker, target, turn, encounterID
}

// TestServiceAttack_RagingBarbarian_AddsRageDamageBonus is the end-to-end
// integration check that wires the Feature Effect System into Service.Attack.
//
// A level-5 raging Barbarian hits with a longsword (1d8 slashing + STR 16
// modifier +3 = base 8 with a fixed d8 roll of 5). Rage at level 5 grants
// +2 damage on STR-based melee attacks (rage damage bonus). The expected
// damage is therefore 5 + 3 + 2 = 10. Without the FES wiring fix, the
// IsRaging context flag is never populated and the rage damage bonus
// silently no-ops, producing 8.
func TestServiceAttack_RagingBarbarian_AddsRageDamageBonus(t *testing.T) {
	svc, attacker, target, turn, _ := makeRagingBarbarianFixture(t)
	ctx := context.Background()

	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15 // hits AC 12
		}
		return 5 // d8 damage roll
	})

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	require.True(t, result.Hit, "expected attack to hit")
	assert.Equal(t, 10, result.DamageTotal,
		"expected rage damage bonus (+2) on top of 1d8(5) + STR(+3)")
}

// TestServiceAttack_NonRagingBarbarian_OmitsRageDamageBonus is the negative
// control: same character/weapon/dice but IsRaging=false. The rage damage
// bonus must NOT apply, so the damage is the bare 1d8(5) + STR(+3) = 8.
func TestServiceAttack_NonRagingBarbarian_OmitsRageDamageBonus(t *testing.T) {
	svc, attacker, target, turn, _ := makeRagingBarbarianFixture(t)
	attacker.IsRaging = false
	ctx := context.Background()

	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 5
	})

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 8, result.DamageTotal,
		"non-raging barbarian must not get rage damage bonus")
}

// TestServiceAttack_Rogue_SneakAttackFiresOnAdvantage proves Sneak Attack
// extra damage dice are rolled into DamageTotal when the attack is made
// with advantage and a finesse weapon.
//
// Setup: a level-5 rogue (sneak attack 3d6) attacks a prone goblin within
// 5ft using a rapier (finesse 1d8 piercing). The prone-within-5ft target
// grants advantage (DetectAdvantage). DEX(+3) is used because rapier is
// finesse and DEX > STR (10 vs 16... wait STR>DEX with 16/10).
// Actually: scores are STR 10, DEX 16 so finesse picks DEX(+3).
// Expected damage: 1d8(5) + DEX(+3) + 3d6(each rolled as 5) = 5+3+15 = 23.
func TestServiceAttack_Rogue_SneakAttackFiresOnAdvantage(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	classes := []CharacterClass{{Class: "Rogue", Level: 5}}
	feats := []CharacterFeature{{Name: "Sneak Attack", MechanicalEffect: "sneak_attack"}}
	char := makeCharacterWithFeats(10, 16, 3, "rapier", feats, classes)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeRapier(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15 // any of the d20s — adv picks higher anyway
		}
		// All damage dice (d8 weapon + 3d6 sneak) return 5
		return 5
	})

	attacker := refdata.Combatant{
		ID:          attackerID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Snik",
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	// Target is prone and within 5ft → grants advantage to attacker
	proneCond, _ := json.Marshal([]CombatCondition{{Condition: "prone"}})
	target := refdata.Combatant{
		ID:          targetID,
		EncounterID: encounterID,
		DisplayName: "Goblin",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          12,
		IsAlive:     true,
		IsNpc:       true,
		IsVisible:   true,
		Conditions:  json.RawMessage(proneCond),
	}
	turn := refdata.Turn{
		ID:               turnID,
		EncounterID:      encounterID,
		CombatantID:      attackerID,
		AttacksRemaining: 1,
	}

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	require.True(t, result.Hit, "expected attack to hit")
	assert.Equal(t, dice.Advantage, result.RollMode,
		"prone target within 5ft should grant advantage")
	// 1d8(5) + DEX(+3) + 3d6 sneak attack with each die = 5 → 5+3+15 = 23
	assert.Equal(t, 23, result.DamageTotal,
		"sneak attack should add 3d6 (15) to base damage 5+3=8, total 23")
}

// TestBuildAttackEffectContext_WiresAllRequestedFields documents the contract
// between Service.Attack and BuildAttackEffectContext: every flag the chunk-4
// review called out (HasAdvantage, AllyWithinFt, WearingArmor,
// OneHandedMeleeOnly, IsRaging, AbilityUsed, UsedThisTurn) must be
// propagated into the EffectContext, not silently dropped.
func TestBuildAttackEffectContext_WiresAllRequestedFields(t *testing.T) {
	used := map[string]bool{string(EffectExtraDamageDice): true}
	ctx := BuildAttackEffectContext(AttackEffectInput{
		Weapon:             makeRapier(),
		HasAdvantage:       true,
		AllyWithinFt:       5,
		WearingArmor:       true,
		OneHandedMeleeOnly: true,
		IsRaging:           true,
		AbilityUsed:        "dex",
		UsedThisTurn:       used,
	})

	assert.True(t, ctx.HasAdvantage)
	assert.Equal(t, 5, ctx.AllyWithinFt)
	assert.True(t, ctx.WearingArmor)
	assert.True(t, ctx.OneHandedMeleeOnly)
	assert.True(t, ctx.IsRaging)
	assert.Equal(t, "dex", ctx.AbilityUsed)
	assert.True(t, ctx.UsedThisTurn[string(EffectExtraDamageDice)])
}

// TestAttackAbilityUsed_FinesseRapierPicksHigher documents the helper used
// to populate EffectContext.AbilityUsed: finesse rapier with DEX>STR returns
// "dex"; non-finesse longsword returns "str"; ranged longbow returns "dex".
func TestAttackAbilityUsed_AllPaths(t *testing.T) {
	t.Run("finesse picks higher ability", func(t *testing.T) {
		assert.Equal(t, "dex",
			attackAbilityUsed(AbilityScores{Str: 10, Dex: 16}, makeRapier(), 0))
		assert.Equal(t, "str",
			attackAbilityUsed(AbilityScores{Str: 16, Dex: 10}, makeRapier(), 0))
	})
	t.Run("ranged uses dex", func(t *testing.T) {
		assert.Equal(t, "dex",
			attackAbilityUsed(AbilityScores{Str: 18, Dex: 10}, makeLongbow(), 0))
	})
	t.Run("melee non-finesse uses str", func(t *testing.T) {
		assert.Equal(t, "str",
			attackAbilityUsed(AbilityScores{Str: 16, Dex: 14}, makeLongsword(), 0))
	})
	t.Run("monk weapon picks higher ability", func(t *testing.T) {
		// shortsword is a monk weapon; with DEX>STR, returns dex
		ss := refdata.Weapon{
			ID: "shortsword", Name: "Shortsword", Damage: "1d6",
			DamageType: "piercing", WeaponType: "martial_melee",
			Properties: []string{"finesse", "light"},
		}
		assert.Equal(t, "dex",
			attackAbilityUsed(AbilityScores{Str: 12, Dex: 16}, ss, 5))
	})
}

// TestCountAlliesWithinFt verifies the helper used to populate
// EffectContext.AllyWithinFt. Allies are combatants on the same side
// (NPC vs PC, ignoring self) within the given foot range of the target.
func TestCountAlliesWithinFt(t *testing.T) {
	encounterID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	allyID := uuid.New()
	farAllyID := uuid.New()

	attacker := refdata.Combatant{
		ID: attackerID, EncounterID: encounterID,
		PositionCol: "A", PositionRow: 1, IsAlive: true, IsNpc: false,
	}
	target := refdata.Combatant{
		ID: targetID, EncounterID: encounterID,
		PositionCol: "C", PositionRow: 3, IsAlive: true, IsNpc: true,
	}
	// Adjacent ally (PC) one square diagonally from target: 5ft (chebyshev)
	ally := refdata.Combatant{
		ID: allyID, EncounterID: encounterID,
		PositionCol: "B", PositionRow: 3, IsAlive: true, IsNpc: false,
	}
	// Far ally PC at G6 (col 6, row 5): chebyshev to target C3 (col 2, row 2)
	// = max(|6-2|, |5-2|) * 5 = max(4, 3) * 5 = 20ft. Matches /move pathfinding
	// (diagonal squares cost 5ft, not 5*sqrt(2)ft).
	farAlly := refdata.Combatant{
		ID: farAllyID, EncounterID: encounterID,
		PositionCol: "G", PositionRow: 6, IsAlive: true, IsNpc: false,
	}
	combatants := []refdata.Combatant{attacker, target, ally, farAlly}

	gotMin := nearestAllyDistanceFt(attacker, target, combatants)
	assert.Equal(t, 5, gotMin, "adjacent ally should register at 5ft")

	// With only the far ally, expect 20 (chebyshev × 5ft).
	gotFar := nearestAllyDistanceFt(attacker, target, []refdata.Combatant{attacker, target, farAlly})
	assert.Equal(t, 20, gotFar)

	// With no allies present, expect a sentinel large value (>5).
	gotEmpty := nearestAllyDistanceFt(attacker, target, []refdata.Combatant{attacker, target})
	assert.Greater(t, gotEmpty, 1000)
}

// TestNearestAllyDistanceFt_DeadOrSelfExcluded ensures the helper ignores
// the attacker itself (so a single PC attacker doesn't count as their own
// ally) and dead/incapacitated combatants.
func TestNearestAllyDistanceFt_DeadOrSelfExcluded(t *testing.T) {
	encounterID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	deadAllyID := uuid.New()

	attacker := refdata.Combatant{
		ID: attackerID, EncounterID: encounterID,
		PositionCol: "B", PositionRow: 3, IsAlive: true, IsNpc: false,
	}
	target := refdata.Combatant{
		ID: targetID, EncounterID: encounterID,
		PositionCol: "C", PositionRow: 3, IsAlive: true, IsNpc: true,
	}
	// Adjacent ally PC, but dead — should be excluded.
	deadAlly := refdata.Combatant{
		ID: deadAllyID, EncounterID: encounterID,
		PositionCol: "D", PositionRow: 3, IsAlive: false, IsNpc: false,
	}

	got := nearestAllyDistanceFt(attacker, target, []refdata.Combatant{attacker, target, deadAlly})
	assert.Greater(t, got, 1000, "dead allies must not count")
}

// _ ensures pqtype is referenced (helper imports stay clean).
var _ = pqtype.NullRawMessage{}
