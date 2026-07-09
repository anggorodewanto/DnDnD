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

// --- fixtures ---

// makeCleaveGreataxe returns a greataxe whose mastery property is "cleave".
func makeCleaveGreataxe() refdata.Weapon {
	return refdata.Weapon{
		ID:         "greataxe",
		Name:       "Greataxe",
		Damage:     "1d12",
		DamageType: "slashing",
		WeaponType: "martial_melee",
		Properties: []string{"heavy", "two-handed"},
		Mastery:    "cleave",
	}
}

// makeCleaveHalberd returns a halberd whose mastery is "cleave" and which has
// the reach property (10ft melee reach) — used for the reach Cleave test.
func makeCleaveHalberd() refdata.Weapon {
	return refdata.Weapon{
		ID:         "halberd",
		Name:       "Halberd",
		Damage:     "1d10",
		DamageType: "slashing",
		WeaponType: "martial_melee",
		Properties: []string{"heavy", "reach", "two-handed"},
		Mastery:    "cleave",
	}
}

// makeCleaveRangedBow returns a (fictional) ranged weapon carrying a cleave
// mastery so we can prove Cleave is melee-only and never fires on a ranged hit.
func makeCleaveRangedBow() refdata.Weapon {
	return refdata.Weapon{
		ID:            "cleave-bow",
		Name:          "Cleave Bow",
		Damage:        "1d8",
		DamageType:    "piercing",
		WeaponType:    "martial_ranged",
		RangeNormalFt: sql.NullInt32{Int32: 80, Valid: true},
		RangeLongFt:   sql.NullInt32{Int32: 320, Valid: true},
		Mastery:       "cleave",
	}
}

// makeNickDagger returns a dagger whose mastery property is "nick".
func makeNickDagger() refdata.Weapon {
	return refdata.Weapon{
		ID:         "dagger",
		Name:       "Dagger",
		Damage:     "1d4",
		DamageType: "piercing",
		WeaponType: "simple_melee",
		Properties: []string{"finesse", "light", "thrown"},
		Mastery:    "nick",
	}
}

// --- Cleave (pure ResolveAttack) ---

func TestResolveAttack_CleaveMeleeHitSetsMasteryProperty(t *testing.T) {
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18 // hit
		}
		return 6
	})

	input := AttackInput{
		AttackerName:    "Aria",
		TargetName:      "Goblin #1",
		TargetAC:        13,
		Weapon:          makeCleaveGreataxe(),
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: []string{"greataxe"},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "cleave", result.MasteryProperty)
}

func TestResolveAttack_CleaveHitUnknownMasteryNoProperty(t *testing.T) {
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18
		}
		return 6
	})

	input := AttackInput{
		AttackerName:    "Aria",
		TargetName:      "Goblin #1",
		TargetAC:        13,
		Weapon:          makeCleaveGreataxe(),
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: nil, // attacker does NOT know greataxe mastery
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty)
}

func TestResolveAttack_CleaveMissNoProperty(t *testing.T) {
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 2 // miss vs AC 20
		}
		return 6
	})

	input := AttackInput{
		AttackerName:    "Aria",
		TargetName:      "Goblin #1",
		TargetAC:        20,
		Weapon:          makeCleaveGreataxe(),
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: []string{"greataxe"},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.False(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty)
}

func TestResolveAttack_CleaveRangedHitNoProperty(t *testing.T) {
	bow := makeCleaveRangedBow()

	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18 // hit
		}
		return 6
	})

	input := AttackInput{
		AttackerName:    "Aria",
		TargetName:      "Goblin #1",
		TargetAC:        13,
		Weapon:          bow,
		Scores:          AbilityScores{Str: 10, Dex: 16},
		ProfBonus:       2,
		DistanceFt:      30,
		WeaponMasteries: []string{"cleave-bow"},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty, "Cleave is melee-only and must not fire on a ranged hit")
}

// --- Cleave (FormatAttackLog surfacing) ---

func TestFormatAttackLog_CleaveHit(t *testing.T) {
	result := AttackResult{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		WeaponName:   "Greataxe",
		IsMelee:      true,
		Hit:          true,
		DamageTotal:  8,
		DamageType:   "slashing",
		DamageDice:   "1d12+3",
		D20Roll:      dice.D20Result{Total: 18, Chosen: 16, Modifier: 2},
		CleaveAttack: &AttackResult{
			TargetName:  "Goblin #2",
			Hit:         true,
			DamageTotal: 5,
			DamageType:  "slashing",
		},
	}
	log := FormatAttackLog(result)
	assert.Contains(t, log, "Cleave hits Goblin #2 for 5 slashing")
}

func TestFormatAttackLog_CleaveMiss(t *testing.T) {
	result := AttackResult{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		WeaponName:   "Greataxe",
		IsMelee:      true,
		Hit:          true,
		DamageTotal:  8,
		DamageType:   "slashing",
		DamageDice:   "1d12+3",
		D20Roll:      dice.D20Result{Total: 18, Chosen: 16, Modifier: 2},
		CleaveAttack: &AttackResult{
			TargetName: "Goblin #2",
			Hit:        false,
		},
	}
	log := FormatAttackLog(result)
	assert.Contains(t, log, "Cleave misses Goblin #2")
}

func TestFormatAttackLog_NoCleaveNoLine(t *testing.T) {
	result := AttackResult{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		WeaponName:   "Greataxe",
		IsMelee:      true,
		Hit:          true,
		DamageTotal:  8,
		DamageType:   "slashing",
		DamageDice:   "1d12+3",
		D20Roll:      dice.D20Result{Total: 18, Chosen: 16, Modifier: 2},
	}
	log := FormatAttackLog(result)
	assert.NotContains(t, log, "Cleave")
}

// --- Cleave (service-level auto-resolution) ---

// cleaveAttacker builds the primary attacker wielding a known-cleave greataxe.
func cleaveAttacker(charID, attackerID, encounterID uuid.UUID) refdata.Combatant {
	return refdata.Combatant{
		ID:          attackerID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Aria",
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
}

// primaryTarget at B1 (adjacent to attacker A1).
func cleavePrimaryTarget(id, encounterID uuid.UUID) refdata.Combatant {
	return refdata.Combatant{
		ID:          id,
		EncounterID: encounterID,
		DisplayName: "Goblin #1",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          13,
		HpCurrent:   20,
		HpMax:       20,
		IsAlive:     true,
		IsNpc:       true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
}

// secondTarget at C1 — adjacent (5ft) to primary at B1 and 10ft from attacker
// at A1 (so it is in reach only for reach weapons; for the greataxe test the
// attacker stands at B-adjacent positioning is handled per-test).
func cleaveSecondTarget(id, encounterID uuid.UUID, col string, row int32) refdata.Combatant {
	return refdata.Combatant{
		ID:          id,
		EncounterID: encounterID,
		DisplayName: "Goblin #2",
		PositionCol: col,
		PositionRow: row,
		Ac:          13,
		HpCurrent:   20,
		HpMax:       20,
		IsAlive:     true,
		IsNpc:       true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
}

func TestServiceAttack_CleaveHitsSecondCreatureNoAbilityMod(t *testing.T) {
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

	// Attacker A1; primary B1 (5ft cardinal); second B2 (5ft from primary,
	// ~5ft diagonal from attacker). All within the greataxe's 5ft reach.
	attacker := cleaveAttacker(charID, attackerID, encounterID)
	primary := cleavePrimaryTarget(primaryID, encounterID)
	second := cleaveSecondTarget(secondID, encounterID, "B", 2)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeCleaveGreataxe(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{attacker, primary, second}, nil
	}
	hpWrites := make(map[uuid.UUID][]int32)
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpWrites[arg.ID] = append(hpWrites[arg.ID], arg.HpCurrent)
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, Conditions: json.RawMessage(`[]`)}, nil
	}

	svc := NewService(ms)
	// Both d20 rolls hit; damage dice roll 5 each.
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18
		}
		return 5
	})

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: primary, Turn: turn}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "cleave", result.MasteryProperty)

	require.NotNil(t, result.CleaveAttack, "expected a Cleave secondary attack")
	assert.True(t, result.CleaveAttack.Hit)
	assert.Equal(t, "Goblin #2", result.CleaveAttack.TargetName)
	// Second-attack damage: d12(5) with NO ability mod (STR +3 omitted) = 5.
	// (The primary attack's own +3 mod proves the secondary omits it.)
	assert.Equal(t, 5, result.CleaveAttack.DamageTotal)
	assert.Equal(t, 8, result.DamageTotal, "primary still adds STR +3 (d12=5 + 3)")

	// Cleave applies the secondary damage itself; the second creature takes 5
	// (no mod) → 20-5 = 15.
	require.NotEmpty(t, hpWrites[secondID], "second creature should take cleave damage")
	assert.Equal(t, int32(15), hpWrites[secondID][len(hpWrites[secondID])-1])
	// The primary hit now applies its damage to the primary's HP too: the
	// greataxe deals d12(5) + STR 3 = 8, so the primary drops 20 → 12.
	require.NotEmpty(t, hpWrites[primaryID], "primary creature should take the primary-hit damage")
	assert.Equal(t, int32(12), hpWrites[primaryID][len(hpWrites[primaryID])-1])
}

func TestServiceAttack_CleaveNoSecondCreatureInRange(t *testing.T) {
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
	// Second creature far away (not within 5ft of primary).
	second := cleaveSecondTarget(secondID, encounterID, "J", 10)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeCleaveGreataxe(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{attacker, primary, second}, nil
	}
	secondHP := false
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		if arg.ID == secondID {
			secondHP = true
		}
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, Conditions: json.RawMessage(`[]`)}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18
		}
		return 5
	})

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: primary, Turn: turn}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Nil(t, result.CleaveAttack, "no valid second creature → no Cleave")
	assert.False(t, secondHP, "far second creature must not take cleave damage")
}

func TestServiceAttack_CleaveAlreadyUsedThisTurnNoSecondAttack(t *testing.T) {
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

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeCleaveGreataxe(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		// Two attacks this turn (Extra Attack), so a second Attack call is legal.
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{attacker, primary, second}, nil
	}
	cleaveCount := 0
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		if arg.ID == secondID {
			cleaveCount++
		}
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, Conditions: json.RawMessage(`[]`)}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18
		}
		return 5
	})

	// First attack fires cleave.
	turn1 := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 2}
	r1, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: primary, Turn: turn1}, roller)
	require.NoError(t, err)
	require.NotNil(t, r1.CleaveAttack, "first attack should fire cleave")

	// Second attack same turn must NOT fire cleave again.
	turn2 := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	r2, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: primary, Turn: turn2}, roller)
	require.NoError(t, err)
	assert.Nil(t, r2.CleaveAttack, "cleave already used this turn → no second attack")
	assert.Equal(t, 1, cleaveCount, "cleave damage should be applied exactly once this turn")
}

func TestServiceAttack_CleaveUnknownMasteryNoSecondAttack(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	primaryID := uuid.New()
	secondID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 10, 2, "greataxe")
	char.ID = charID
	char.CharacterData = charDataWithMasteries(`{}`) // does NOT know greataxe mastery

	attacker := cleaveAttacker(charID, attackerID, encounterID)
	primary := cleavePrimaryTarget(primaryID, encounterID)
	second := cleaveSecondTarget(secondID, encounterID, "B", 2)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeCleaveGreataxe(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{attacker, primary, second}, nil
	}
	secondHP := false
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		if arg.ID == secondID {
			secondHP = true
		}
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, Conditions: json.RawMessage(`[]`)}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18
		}
		return 5
	})

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: primary, Turn: turn}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty)
	assert.Nil(t, result.CleaveAttack)
	assert.False(t, secondHP)
}

func TestServiceAttack_CleaveReachWeaponHitsSecondAt10ft(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	primaryID := uuid.New()
	secondID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 10, 2, "halberd")
	char.ID = charID
	char.CharacterData = charDataWithMasteries(`{"weapon_masteries":["halberd"]}`)

	// Attacker A1, primary B1 (5ft), second C1 (10ft from attacker, 5ft from
	// primary). Greataxe (5ft reach) could not hit C1, but the halberd's reach
	// (10ft) can.
	attacker := cleaveAttacker(charID, attackerID, encounterID)
	primary := cleavePrimaryTarget(primaryID, encounterID)
	second := cleaveSecondTarget(secondID, encounterID, "C", 1)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeCleaveHalberd(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{attacker, primary, second}, nil
	}
	secondHP := false
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		if arg.ID == secondID {
			secondHP = true
		}
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, Conditions: json.RawMessage(`[]`)}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18
		}
		return 5
	})

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: primary, Turn: turn}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	require.NotNil(t, result.CleaveAttack, "reach weapon should cleave the 10ft-away second creature")
	assert.True(t, secondHP)
}

// --- Nick (service-level off-hand) ---

func nickChar(charID uuid.UUID, mainHand, offHand string, masteries string) refdata.Character {
	char := makeCharacter(16, 14, 2, mainHand)
	char.ID = charID
	char.EquippedOffHand = sql.NullString{String: offHand, Valid: offHand != ""}
	if masteries != "" {
		char.CharacterData = charDataWithMasteries(masteries)
	}
	return char
}

func nickAttacker(charID, attackerID, encounterID uuid.UUID) refdata.Combatant {
	return refdata.Combatant{
		ID:          attackerID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Aria",
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
}

func nickTarget(targetID, encounterID uuid.UUID) refdata.Combatant {
	return refdata.Combatant{
		ID:          targetID,
		EncounterID: encounterID,
		DisplayName: "Goblin #1",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          13,
		HpCurrent:   20,
		HpMax:       20,
		IsAlive:     true,
		IsNpc:       true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
}

func TestServiceOffhandAttack_NickDoesNotConsumeBonusAction(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	// Main hand shortsword (light), off-hand dagger with Nick known.
	char := nickChar(charID, "shortsword", "dagger", `{"weapon_masteries":["dagger"]}`)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		switch id {
		case "shortsword":
			return makeShortsword(), nil
		case "dagger":
			return makeNickDagger(), nil
		}
		return refdata.Weapon{}, sql.ErrNoRows
	}
	var persisted []bool
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		persisted = append(persisted, arg.BonusActionUsed)
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 3
	})

	result, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: nickAttacker(charID, attackerID, encounterID),
		Target:   nickTarget(targetID, encounterID),
		Turn:     refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 0},
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit, "the off-hand attack still resolves normally")
	require.NotNil(t, result.RemainingTurn)
	assert.False(t, result.RemainingTurn.BonusActionUsed, "Nick off-hand must NOT consume the bonus action")
	for _, p := range persisted {
		assert.False(t, p, "persisted turn must keep the bonus action available")
	}
}

// A Nick off-hand attack is absorbed into the Attack action and costs no bonus
// action, so it must succeed even when the bonus action was ALREADY spent this
// turn (e.g. a Rogue who used Steady Aim / Cunning Action). Regression for the
// bug where OffhandAttack rejected up-front on a spent bonus action before the
// Nick free-detection ran.
func TestServiceOffhandAttack_NickFreeEvenWhenBonusActionSpent(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := nickChar(charID, "shortsword", "dagger", `{"weapon_masteries":["dagger"]}`)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		switch id {
		case "shortsword":
			return makeShortsword(), nil
		case "dagger":
			return makeNickDagger(), nil
		}
		return refdata.Weapon{}, sql.ErrNoRows
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 3
	})

	// Bonus action already spent (Steady Aim etc.) yet the attack was taken this
	// turn — the Nick off-hand swing must still resolve.
	result, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: nickAttacker(charID, attackerID, encounterID),
		Target:   nickTarget(targetID, encounterID),
		Turn:     refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 0, BonusActionUsed: true},
	}, roller)
	require.NoError(t, err, "Nick off-hand needs no bonus action, so a spent bonus action must not block it")
	assert.True(t, result.Hit, "the off-hand attack still resolves normally")
}

func TestServiceOffhandAttack_NonNickConsumesBonusAction(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	// Off-hand shortsword (light, no Nick) — bonus action consumed as before.
	char := nickChar(charID, "shortsword", "shortsword", "")

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeShortsword(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 3
	})

	result, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: nickAttacker(charID, attackerID, encounterID),
		Target:   nickTarget(targetID, encounterID),
		Turn:     refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 0},
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	require.NotNil(t, result.RemainingTurn)
	assert.True(t, result.RemainingTurn.BonusActionUsed, "non-Nick off-hand consumes the bonus action")
}

func TestServiceOffhandAttack_NickNotKnownConsumesBonusAction(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	// Dagger has Nick mastery but the attacker does NOT know it.
	char := nickChar(charID, "shortsword", "dagger", `{}`)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		switch id {
		case "shortsword":
			return makeShortsword(), nil
		case "dagger":
			return makeNickDagger(), nil
		}
		return refdata.Weapon{}, sql.ErrNoRows
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 3
	})

	result, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: nickAttacker(charID, attackerID, encounterID),
		Target:   nickTarget(targetID, encounterID),
		Turn:     refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 0},
	}, roller)
	require.NoError(t, err)
	require.NotNil(t, result.RemainingTurn)
	assert.True(t, result.RemainingTurn.BonusActionUsed, "Nick not known → bonus action consumed")
}

func TestServiceOffhandAttack_NickOncePerTurn(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := nickChar(charID, "shortsword", "dagger", `{"weapon_masteries":["dagger"]}`)
	// ISSUE-062: a SECOND off-hand swing the same turn now requires the Dual
	// Wielder feat. Nick's free swing is still once-per-turn (this test's point);
	// the second swing is the Dual-Wielder extra and costs the bonus action.
	char.Features = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`[{"name":"Dual Wielder","source":"feat"}]`),
		Valid:      true,
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		switch id {
		case "shortsword":
			return makeShortsword(), nil
		case "dagger":
			return makeNickDagger(), nil
		}
		return refdata.Weapon{}, sql.ErrNoRows
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 3
	})

	// First Nick off-hand: free (bonus action preserved).
	r1, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: nickAttacker(charID, attackerID, encounterID),
		Target:   nickTarget(targetID, encounterID),
		Turn:     refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 0},
	}, roller)
	require.NoError(t, err)
	require.NotNil(t, r1.RemainingTurn)
	assert.False(t, r1.RemainingTurn.BonusActionUsed, "first Nick off-hand is free")

	// Second Nick off-hand same turn: NOT free → consumes the bonus action.
	r2, err := svc.OffhandAttack(ctx, OffhandAttackCommand{
		Attacker: nickAttacker(charID, attackerID, encounterID),
		Target:   nickTarget(targetID, encounterID),
		Turn:     refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 0},
	}, roller)
	require.NoError(t, err)
	require.NotNil(t, r2.RemainingTurn)
	assert.True(t, r2.RemainingTurn.BonusActionUsed, "second Nick off-hand same turn costs the bonus action")
}
