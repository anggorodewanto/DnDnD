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

// makeRogueFixture builds a level-5 rogue attacking a prone goblin within
// 5ft using a rapier — i.e. the canonical "sneak attack with advantage"
// scenario. The combatant IDs and encounter ID are returned so tests can
// reuse them across multiple Service.Attack calls.
func makeRogueFixture(t *testing.T) (svc *Service, attacker, target refdata.Combatant, encounterID uuid.UUID) {
	t.Helper()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	encounterID = uuid.New()

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

	svc = NewService(ms)
	attacker = refdata.Combatant{
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
	proneCond, _ := json.Marshal([]CombatCondition{{Condition: "prone"}})
	target = refdata.Combatant{
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
	return svc, attacker, target, encounterID
}

// TestServiceAttack_Rogue_SneakAttackOncePerTurn_ExtraAttack proves that a
// rogue making two attacks on the same turn (e.g., via Action Surge or
// Haste) only adds Sneak Attack dice once. Without the OncePerTurn wiring
// in populateAttackFES, every qualifying attack re-adds 3d6.
//
// First attack: 1d8(5) + DEX(+3) + 3d6 sneak (each 5) = 5+3+15 = 23.
// Second attack (same turn): 1d8(5) + DEX(+3) only = 8.
func TestServiceAttack_Rogue_SneakAttackOncePerTurn_ExtraAttack(t *testing.T) {
	ctx := context.Background()
	svc, attacker, target, encounterID := makeRogueFixture(t)
	turnID := uuid.New()

	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 5
	})

	turn := refdata.Turn{
		ID:               turnID,
		EncounterID:      encounterID,
		CombatantID:      attacker.ID,
		AttacksRemaining: 2,
	}

	first, err := svc.Attack(ctx, AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	require.True(t, first.Hit)
	assert.Equal(t, 23, first.DamageTotal, "first attack should add 3d6 sneak attack")

	// Second attack on the same turn — sneak attack must NOT re-apply.
	turn.AttacksRemaining = 1
	second, err := svc.Attack(ctx, AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	require.True(t, second.Hit)
	assert.Equal(t, 8, second.DamageTotal,
		"second attack on the same turn must not re-add sneak attack dice (OncePerTurn)")
}

// TestServiceAttack_Rogue_SneakAttack_ReactionAttackOnEnemyTurnApplies proves
// that a rogue's reaction attack during *another creature's* turn — when the
// rogue has not yet used sneak attack since their own last turn started —
// still triggers the sneak attack dice. The "turn" RAW for once-per-turn is
// "since your turn started", not "the currently active turn".
//
// In this test cmd.Turn refers to the enemy's turn row (different CombatantID),
// but the rogue has not used sneak attack this round, so it must fire.
func TestServiceAttack_Rogue_SneakAttack_ReactionAttackOnEnemyTurnApplies(t *testing.T) {
	ctx := context.Background()
	svc, attacker, target, encounterID := makeRogueFixture(t)

	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 5
	})

	// The active turn belongs to the *target* (enemy) — i.e. this is a
	// reaction attack provoked by the enemy's movement.
	enemyTurnID := uuid.New()
	enemyTurn := refdata.Turn{
		ID:               enemyTurnID,
		EncounterID:      encounterID,
		CombatantID:      target.ID, // enemy is currently up
		AttacksRemaining: 1,         // pretend reaction-as-attack budget
	}

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     enemyTurn,
	}, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 23, result.DamageTotal,
		"reaction attack on enemy's turn (not yet used since rogue's turn started) must apply sneak attack")
}

// TestServiceAttack_Rogue_SneakAttack_ReactionAfterOwnTurnUseDoesNotReapply
// proves the negative side of the reaction rider: if the rogue already used
// sneak attack on their own turn, a follow-up reaction attack during the
// next creature's turn (before the rogue's own next turn starts) must NOT
// re-apply sneak attack dice. The tracker must persist across turn rows
// keyed on the attacker's combatant identity.
func TestServiceAttack_Rogue_SneakAttack_ReactionAfterOwnTurnUseDoesNotReapply(t *testing.T) {
	ctx := context.Background()
	svc, attacker, target, encounterID := makeRogueFixture(t)

	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 5
	})

	// (1) Rogue's own turn — fires sneak attack.
	ownTurn := refdata.Turn{
		ID:               uuid.New(),
		EncounterID:      encounterID,
		CombatantID:      attacker.ID,
		AttacksRemaining: 1,
	}
	first, err := svc.Attack(ctx, AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     ownTurn,
	}, roller)
	require.NoError(t, err)
	require.True(t, first.Hit)
	assert.Equal(t, 23, first.DamageTotal, "first attack on rogue's own turn should add sneak attack")

	// (2) Enemy's turn — rogue makes a reaction attack BEFORE their next own
	// turn starts. Sneak attack must NOT re-apply.
	enemyTurn := refdata.Turn{
		ID:               uuid.New(),
		EncounterID:      encounterID,
		CombatantID:      target.ID,
		AttacksRemaining: 1,
	}
	second, err := svc.Attack(ctx, AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     enemyTurn,
	}, roller)
	require.NoError(t, err)
	require.True(t, second.Hit)
	assert.Equal(t, 8, second.DamageTotal,
		"reaction attack after sneak attack was already used this turn must not re-apply")
}

// TestServiceClearUsedEffects_ResetsAtRogueTurnStart proves the tracker is
// cleared when the rogue's *own* turn starts again — i.e., sneak attack is
// rearmed for the next round.
func TestServiceClearUsedEffects_ResetsAtRogueTurnStart(t *testing.T) {
	ctx := context.Background()
	svc, attacker, target, encounterID := makeRogueFixture(t)

	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 5
	})

	turn1 := refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: attacker.ID, AttacksRemaining: 1}
	first, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn1}, roller)
	require.NoError(t, err)
	assert.Equal(t, 23, first.DamageTotal)

	// Simulate a new round starting for the rogue (the initiative-side hook).
	svc.clearUsedEffectsForCombatant(encounterID, attacker.ID)

	turn2 := refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: attacker.ID, AttacksRemaining: 1}
	second, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn2}, roller)
	require.NoError(t, err)
	assert.Equal(t, 23, second.DamageTotal,
		"new turn for rogue must re-arm sneak attack (tracker cleared)")
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
			attackAbilityUsed(AbilityScores{Str: 10, Dex: 16}, makeRapier(), 0, false))
		assert.Equal(t, "str",
			attackAbilityUsed(AbilityScores{Str: 16, Dex: 10}, makeRapier(), 0, false))
	})
	t.Run("ranged uses dex", func(t *testing.T) {
		assert.Equal(t, "dex",
			attackAbilityUsed(AbilityScores{Str: 18, Dex: 10}, makeLongbow(), 0, false))
	})
	t.Run("melee non-finesse uses str", func(t *testing.T) {
		assert.Equal(t, "str",
			attackAbilityUsed(AbilityScores{Str: 16, Dex: 14}, makeLongsword(), 0, false))
	})
	t.Run("monk weapon picks higher ability", func(t *testing.T) {
		// shortsword is a monk weapon; with DEX>STR, returns dex
		ss := refdata.Weapon{
			ID: "shortsword", Name: "Shortsword", Damage: "1d6",
			DamageType: "piercing", WeaponType: "martial_melee",
			Properties: []string{"finesse", "light"},
		}
		assert.Equal(t, "dex",
			attackAbilityUsed(AbilityScores{Str: 12, Dex: 16}, ss, 5, false))
	})
	t.Run("raging finesse forces str even with higher dex", func(t *testing.T) {
		assert.Equal(t, "str",
			attackAbilityUsed(AbilityScores{Str: 14, Dex: 16}, makeRapier(), 0, true))
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

// TestServiceAttack_SacredWeapon_AddsCHAModToAttackRoll proves that when
// the attacker has the sacred_weapon condition, the CHA modifier is added
// to the attack roll via the Feature Effect System (SR-058).
//
// Setup: Paladin with STR 16 (+3), CHA 16 (+3), prof +2, longsword.
// Attack modifier = STR(+3) + prof(+2) + Sacred Weapon CHA(+3) = +8.
// With a d20 roll of 10, total = 18, which hits AC 15.
// Without Sacred Weapon the total would be 15 (barely hits AC 15).
// We verify the hit happens and the D20Roll.Total includes the +3 bonus.
func TestServiceAttack_SacredWeapon_AddsCHAModToAttackRoll(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	encounterID := uuid.New()

	classes := []CharacterClass{{Class: "Paladin", Level: 3}}
	// No mechanical_effect needed for sacred_weapon — it's condition-driven.
	char := refdata.Character{
		ID:               charID,
		AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":10,"wis":12,"cha":16}`),
		ProficiencyBonus: 2,
		Classes:          json.RawMessage(`[{"class":"Paladin","level":3}]`),
		EquippedMainHand: sql.NullString{String: "longsword", Valid: true},
	}
	_ = classes

	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) {
		return makeLongsword(), nil
	}
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	svc := NewService(ms)

	attacker := refdata.Combatant{
		ID:          attackerID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Oath",
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[{"condition":"sacred_weapon","duration_rounds":10,"started_round":1,"source_combatant_id":"` + attackerID.String() + `","expires_on":"end_of_turn"}]`),
	}
	target := refdata.Combatant{
		ID:          targetID,
		EncounterID: encounterID,
		DisplayName: "Goblin",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          18,
		IsAlive:     true,
		IsNpc:       true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	turn := refdata.Turn{
		ID:               uuid.New(),
		EncounterID:      encounterID,
		CombatantID:      attackerID,
		AttacksRemaining: 1,
	}

	// d20 roll of 10: base attack mod = STR(+3) + prof(+2) = +5 → total 15.
	// With Sacred Weapon CHA(+3): total = 10 + 5 + 3 = 18, hits AC 18.
	// Without Sacred Weapon: total = 10 + 5 = 15, misses AC 18.
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 10
		}
		return 5
	})

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit,
		"Sacred Weapon should add CHA mod (+3) to attack roll, making total 18 vs AC 18")
	assert.Equal(t, 18, result.D20Roll.Total,
		"attack roll total should include Sacred Weapon CHA bonus: 10 + STR(3) + prof(2) + CHA(3) = 18")
}

// _ ensures pqtype is referenced (helper imports stay clean).
var _ = pqtype.NullRawMessage{}

// TestResolveAttack_F07_DefenseFightingStyle_ACBonusCausesMiss proves that
// TestResolveAttack_F07_DefenseFightingStyle_ACBonusCausesMiss is removed.
// Defense fighting style +1 AC is correctly applied via RecalculateAC (stored AC)
// rather than through the attacker's FES pipeline. See equip_test.go F07 tests.
