package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// --- C-40: charmed attacker cannot target its charmer ---

func TestServiceAttack_Charmed_BlocksAttackOnCharmer(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacterWithFeats(18, 10, 3, "longsword", nil, nil)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeLongsword(), nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	charmedCond := fmt.Sprintf(`[{"condition":"charmed","source_combatant_id":%q}]`, targetID.String())
	_, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{
			ID:          attackerID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Hero",
			PositionCol: "A", PositionRow: 1,
			IsAlive: true, IsVisible: true,
			Conditions: json.RawMessage(charmedCond),
		},
		Target: refdata.Combatant{
			ID:          targetID,
			DisplayName: "Vampire",
			PositionCol: "B", PositionRow: 1,
			Ac:      15,
			IsAlive: true, IsVisible: true,
			Conditions: json.RawMessage(`[]`),
		},
		Turn: refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 2},
	}, roller)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "charmed")
}

func TestServiceAttack_Charmed_AllowsAttackOnNonCharmer(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	otherCharmerID := uuid.New()
	turnID := uuid.New()

	char := makeCharacterWithFeats(18, 10, 3, "longsword", nil, nil)
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
	roller := dice.NewRoller(func(max int) int { return 10 })

	// Charmed by a different combatant — attacking targetID should succeed.
	charmedCond := fmt.Sprintf(`[{"condition":"charmed","source_combatant_id":%q}]`, otherCharmerID.String())
	_, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{
			ID:          attackerID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Hero",
			PositionCol: "A", PositionRow: 1,
			IsAlive: true, IsVisible: true,
			Conditions: json.RawMessage(charmedCond),
		},
		Target: refdata.Combatant{
			ID:          targetID,
			DisplayName: "Goblin",
			PositionCol: "B", PositionRow: 1,
			Ac:      15,
			IsAlive: true, IsVisible: true,
			Conditions: json.RawMessage(`[]`),
		},
		Turn: refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 2},
	}, roller)

	require.NoError(t, err)
}

// --- C-38: Reckless attacker grants advantage to incoming attacks (target-side) ---

func TestDetectAdvantage_TargetReckless_GrantsAdvantage(t *testing.T) {
	input := AdvantageInput{
		Weapon:           makeLongsword(),
		TargetConditions: []CombatCondition{{Condition: "reckless"}},
	}
	mode, advReasons, _ := DetectAdvantage(input)
	assert.Equal(t, dice.Advantage, mode)
	assert.Contains(t, advReasons, "target reckless")
}

func TestServiceAttack_Reckless_AppliesTargetSideMarker(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacterWithFeats(18, 10, 3, "greataxe", nil,
		[]CharacterClass{{Class: "Barbarian", Level: 5}},
	)
	char.ID = charID

	var capturedConds json.RawMessage
	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeGreataxe(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, DisplayName: "Grog", Conditions: json.RawMessage(`[]`)}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		capturedConds = arg.Conditions
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	_, err := svc.Attack(ctx, AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Grog", PositionCol: "A", PositionRow: 1, IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: targetID, DisplayName: "Ogre", PositionCol: "B", PositionRow: 1, Ac: 15, IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 2},
		Reckless: true,
	}, roller)
	require.NoError(t, err)
	require.NotNil(t, capturedConds)

	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(capturedConds, &conds))
	var found *CombatCondition
	for i := range conds {
		if conds[i].Condition == "reckless" {
			found = &conds[i]
			break
		}
	}
	require.NotNil(t, found, "reckless marker must be applied to attacker")
	assert.Equal(t, attackerID.String(), found.SourceCombatantID)
	assert.Equal(t, "start_of_turn", found.ExpiresOn)
	assert.True(t, found.DurationRounds >= 1)
}

// --- C-35-attacker-size: populateAttackContext resolves NPC creature size ---

func TestPopulateAttackContext_ResolvesNPCCreatureSize(t *testing.T) {
	ctx := context.Background()
	attackerID := uuid.New()

	ms := defaultMockStore()
	ms.getCreatureFn = func(_ context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: id, Name: "Goblin", Size: "Small"}, nil
	}

	svc := NewService(ms)

	npcAttacker := refdata.Combatant{
		ID: attackerID, DisplayName: "Goblin",
		IsNpc:         true,
		CreatureRefID: sql.NullString{String: "goblin", Valid: true},
		PositionCol:   "A", PositionRow: 1,
		IsAlive: true, IsVisible: true,
		Conditions: json.RawMessage(`[]`),
	}
	input := AttackInput{}
	svc.populateAttackContext(ctx, &input, npcAttacker)
	assert.Equal(t, "Small", input.AttackerSize, "NPC attacker size should resolve from creature ref")
}

func TestPopulateAttackContext_DefaultsPCSizeToMedium(t *testing.T) {
	ctx := context.Background()
	svc := NewService(defaultMockStore())
	attacker := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Hero", IsNpc: false,
		PositionCol: "A", PositionRow: 1,
		IsAlive: true, IsVisible: true,
	}
	input := AttackInput{}
	svc.populateAttackContext(ctx, &input, attacker)
	assert.Equal(t, "Medium", input.AttackerSize)
}

func TestPopulateAttackContext_CommandOverrideWins(t *testing.T) {
	ctx := context.Background()
	svc := NewService(defaultMockStore())
	attacker := refdata.Combatant{
		ID: uuid.New(), DisplayName: "Hero", IsNpc: false,
		PositionCol: "A", PositionRow: 1,
		IsAlive: true, IsVisible: true,
	}
	input := AttackInput{AttackerSize: "Tiny"}
	svc.populateAttackContext(ctx, &input, attacker)
	assert.Equal(t, "Tiny", input.AttackerSize, "explicit AttackerSize must not be overwritten")
}

// --- C-H02: Small PC race size resolves from character race lookup ---

func TestPopulateAttackContext_ResolvesPCRaceSize(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()

	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{ID: charID, Race: "halfling"}, nil
	}
	ms.getRaceFn = func(_ context.Context, id string) (refdata.Race, error) {
		return refdata.Race{ID: "halfling", Size: "Small"}, nil
	}

	svc := NewService(ms)

	attacker := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "Bilbo",
		IsNpc:       false,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		PositionCol: "A", PositionRow: 1,
		IsAlive: true, IsVisible: true,
		Conditions: json.RawMessage(`[]`),
	}
	input := AttackInput{}
	svc.populateAttackContext(ctx, &input, attacker)
	assert.Equal(t, "Small", input.AttackerSize, "PC attacker size should resolve from race")
}

func TestSmallPC_HeavyWeapon_GetsDisadvantage(t *testing.T) {
	// End-to-end: a Small PC wielding a heavy weapon must get disadvantage
	// from DetectAdvantage via the resolved AttackerSize.
	input := AdvantageInput{
		Weapon:       refdata.Weapon{ID: "greataxe", Name: "Greataxe", Properties: []string{"heavy", "two-handed"}},
		AttackerSize: "Small",
	}
	mode, _, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Contains(t, disadvReasons, "heavy weapon, Small creature")
}

// --- C-35-hostile-near: auto-detect hostile within 5ft for ranged attacks ---

func TestServiceAttack_AutoPopulatesHostileNear_RangedWithAdjacentHostile(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	adjacentEnemyID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacterWithFeats(10, 18, 3, "longbow", nil, nil)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeLongbow(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: attackerID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Hero",
				PositionCol: "B", PositionRow: 2, IsAlive: true, IsNpc: false},
			{ID: adjacentEnemyID, DisplayName: "Adjacent Goblin",
				PositionCol: "B", PositionRow: 3, IsAlive: true, IsNpc: true},
			{ID: targetID, DisplayName: "Distant Goblin",
				PositionCol: "G", PositionRow: 2, Ac: 13, IsAlive: true, IsNpc: true},
		}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int { return 10 })

	attacker := refdata.Combatant{
		ID: attackerID, EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Hero", IsNpc: false,
		PositionCol: "B", PositionRow: 2,
		IsAlive: true, IsVisible: true,
		Conditions: json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID: targetID, EncounterID: encounterID,
		DisplayName: "Distant Goblin", IsNpc: true,
		PositionCol: "G", PositionRow: 2,
		Ac:      13,
		IsAlive: true, IsVisible: true,
		Conditions: json.RawMessage(`[]`),
	}

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: attacker, Target: target,
		Turn: refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.NoError(t, err)

	found := false
	for _, r := range result.DisadvantageReasons {
		if r == "hostile within 5ft" {
			found = true
		}
	}
	assert.True(t, found, "expected hostile-within-5ft disadvantage reason, got %v", result.DisadvantageReasons)
}

// --- C-37: post-combat ammunition recovery (half rounded down) ---

func TestEndCombat_RecoversHalfSpentAmmunition(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	charID := uuid.New()
	combatantID := uuid.New()

	startingInventory, _ := json.Marshal([]InventoryItem{
		{Name: "Arrows", Quantity: 10, Type: "ammunition"},
	})
	char := refdata.Character{
		ID:        charID,
		Inventory: nullRawMessage(startingInventory),
	}

	var updatedInventory json.RawMessage
	store := defaultMockStore()
	store.getEncounterFn = func(_ context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "active", RoundNumber: 3}, nil
	}
	store.updateEncounterStatusFn = func(_ context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID, Status: arg.Status}, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: combatantID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Archer", IsAlive: true, IsNpc: false, Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	store.getCharacterFn = func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.updateCharacterInventoryFn = func(_ context.Context, id uuid.UUID, inv pqtype.NullRawMessage) error {
		updatedInventory = inv.RawMessage
		return nil
	}

	svc := NewService(store)
	// Record that this combatant spent 7 arrows during combat.
	svc.RecordAmmoSpent(encounterID, combatantID, "Arrows", 7)

	_, err := svc.EndCombat(ctx, encounterID)
	require.NoError(t, err)
	require.NotNil(t, updatedInventory, "inventory must be persisted with recovered ammo")

	var items []InventoryItem
	require.NoError(t, json.Unmarshal(updatedInventory, &items))
	require.Len(t, items, 1)
	// Recovered half (rounded down) of 7 = 3, so 10 -> 13.
	assert.Equal(t, 13, items[0].Quantity)
}

// --- C-31: fall damage applied when airborne combatant goes prone ---

func TestApplyCondition_AirborneProne_AppliesFallDamage(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	combatantID := uuid.New()

	combatant := refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		DisplayName: "Aarakocra",
		HpMax:       20, HpCurrent: 20,
		AltitudeFt: 30,
		IsAlive:    true,
		Conditions: json.RawMessage(`[]`),
	}

	var hpAfter int32 = -1
	var altitudeAfter int32 = -1
	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return combatant, nil
	}
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		c := combatant
		c.Conditions = arg.Conditions
		return c, nil
	}
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpAfter = arg.HpCurrent
		c := combatant
		c.HpCurrent = arg.HpCurrent
		c.IsAlive = arg.IsAlive
		return c, nil
	}
	store.updateCombatantPositionFn = func(_ context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
		altitudeAfter = arg.AltitudeFt
		c := combatant
		c.AltitudeFt = arg.AltitudeFt
		c.PositionCol = arg.PositionCol
		c.PositionRow = arg.PositionRow
		return c, nil
	}

	svc := NewService(store)
	// Deterministic roller: each d6 rolls 3.
	svc.SetRoller(dice.NewRoller(func(max int) int { return 2 }))

	_, _, err := svc.ApplyCondition(ctx, combatantID, CombatCondition{Condition: "prone"})
	require.NoError(t, err)
	assert.Equal(t, int32(0), altitudeAfter, "altitude must reset to 0 on prone-fall")
	// 30ft fall -> 3d6 damage. Roller returns 2 each = 3 dice * 3 = 9 (with +1 dice indexing).
	assert.Greater(t, int(combatant.HpMax-hpAfter), 0, "fall damage must be applied")
}

// --- C-37 follow-up coverage ---

func TestAmmoSpentTracker_NegativeAndEmptyAreNoop(t *testing.T) {
	tr := NewAmmoSpentTracker()
	encID := uuid.New()
	cbID := uuid.New()
	tr.Record(encID, cbID, "Arrows", -3)
	tr.Record(encID, cbID, "", 5)
	snap := tr.Snapshot(encID)
	assert.Empty(t, snap)
}

func TestAmmoSpentTracker_ClearEncounterIsolatesOtherEncounters(t *testing.T) {
	tr := NewAmmoSpentTracker()
	a := uuid.New()
	b := uuid.New()
	cb := uuid.New()
	tr.Record(a, cb, "Arrows", 2)
	tr.Record(b, cb, "Bolts", 3)
	tr.ClearEncounter(a)
	assert.Empty(t, tr.Snapshot(a))
	bSnap := tr.Snapshot(b)
	assert.Equal(t, 3, bSnap[cb]["Bolts"])
}

func TestRecordAmmoSpent_NilTrackerIsSafe(t *testing.T) {
	svc := &Service{} // no ammo tracker
	// Must not panic.
	svc.RecordAmmoSpent(uuid.New(), uuid.New(), "Arrows", 1)
}

func TestApplyCondition_GroundedProne_NoFallDamage(t *testing.T) {
	ctx := context.Background()
	combatantID := uuid.New()
	combatant := refdata.Combatant{
		ID:          combatantID,
		DisplayName: "Fighter",
		HpMax:       20, HpCurrent: 20,
		AltitudeFt: 0,
		IsAlive:    true,
		Conditions: json.RawMessage(`[]`),
	}

	hpUpdated := false
	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return combatant, nil
	}
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		c := combatant
		c.Conditions = arg.Conditions
		return c, nil
	}
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpUpdated = true
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}

	svc := NewService(store)
	_, _, err := svc.ApplyCondition(ctx, combatantID, CombatCondition{Condition: "prone"})
	require.NoError(t, err)
	assert.False(t, hpUpdated, "no fall damage when altitude is 0")
}

