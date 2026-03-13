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

	"github.com/ab/dndnd/internal/refdata"
)

// TDD Cycle 1: LayOnHandsPoolMax

func TestLayOnHandsPoolMax(t *testing.T) {
	tests := []struct {
		level int
		want  int
	}{
		{1, 5},
		{2, 10},
		{5, 25},
		{10, 50},
		{20, 100},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, LayOnHandsPoolMax(tc.level), "paladin level %d", tc.level)
	}
}

// TDD Cycle 2: isUndeadOrConstruct

func TestIsUndeadOrConstruct(t *testing.T) {
	assert.True(t, isUndeadOrConstruct("undead"))
	assert.True(t, isUndeadOrConstruct("Undead"))
	assert.True(t, isUndeadOrConstruct("construct"))
	assert.True(t, isUndeadOrConstruct("Construct"))
	assert.False(t, isUndeadOrConstruct("humanoid"))
	assert.False(t, isUndeadOrConstruct("fiend"))
	assert.False(t, isUndeadOrConstruct(""))
}

// TDD Cycle 3: DeductFeaturePool

func TestDeductFeaturePool(t *testing.T) {
	charID := uuid.New()

	t.Run("deducts specified amount from pool", func(t *testing.T) {
		ms := defaultMockStore()
		ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
			return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
		}
		svc := NewService(ms)
		char := refdata.Character{
			ID:          charID,
			FeatureUses: pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"lay-on-hands": 25}`), Valid: true},
		}
		featureUses := map[string]int{"lay-on-hands": 25}

		newRemaining, err := svc.DeductFeaturePool(ctx, char, FeatureKeyLayOnHands, featureUses, 25, 10)
		require.NoError(t, err)
		assert.Equal(t, 15, newRemaining)
	})

	t.Run("error when amount exceeds remaining", func(t *testing.T) {
		ms := defaultMockStore()
		svc := NewService(ms)
		char := refdata.Character{
			ID:          charID,
			FeatureUses: pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"lay-on-hands": 5}`), Valid: true},
		}
		featureUses := map[string]int{"lay-on-hands": 5}

		_, err := svc.DeductFeaturePool(ctx, char, FeatureKeyLayOnHands, featureUses, 5, 10)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient")
	})
}

// helpers for Lay on Hands tests

func layOnHandsTestSetup() (uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, *mockStore) {
	encounterID := uuid.New()
	paladinCombatantID := uuid.New()
	targetCombatantID := uuid.New()
	charID := uuid.New()
	ms := defaultMockStore()
	return encounterID, paladinCombatantID, targetCombatantID, charID, ms
}

func makePaladinChar(charID uuid.UUID, paladinLevel int, poolRemaining int) refdata.Character {
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Paladin", Level: paladinLevel}})
	featureUsesJSON, _ := json.Marshal(map[string]int{FeatureKeyLayOnHands: poolRemaining})
	return refdata.Character{
		ID:               charID,
		Name:             "Thorn",
		Classes:          classesJSON,
		AbilityScores:    json.RawMessage(`{"str":16,"dex":10,"con":14,"int":8,"wis":12,"cha":16}`),
		Level:            int32(paladinLevel),
		HpMax:            40,
		HpCurrent:        40,
		ProficiencyBonus: 2,
		FeatureUses:      pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
	}
}

func makePaladinCombatant(combatantID, encounterID uuid.UUID, charID uuid.UUID) refdata.Combatant {
	return refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		ShortID:     "TH",
		DisplayName: "Thorn",
		HpMax:       40,
		HpCurrent:   40,
		Ac:          18,
		PositionCol: "C",
		PositionRow: 3,
		IsNpc:       false,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}
}

func makeTargetCombatant(combatantID, encounterID uuid.UUID) refdata.Combatant {
	return refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		ShortID:     "AR",
		DisplayName: "Aria",
		HpMax:       30,
		HpCurrent:   15,
		Ac:          14,
		PositionCol: "C",
		PositionRow: 4,
		IsNpc:       false,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
		CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
	}
}

func makeAvailableTurn() refdata.Turn {
	return refdata.Turn{
		ID:         uuid.New(),
		ActionUsed: false,
	}
}

// TDD Cycle 4: LayOnHands happy path

func TestLayOnHands_HappyPath(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()

	paladin := makePaladinCombatant(paladinCombatantID, encounterID, charID)
	target := makeTargetCombatant(targetCombatantID, encounterID)
	char := makePaladinChar(charID, 5, 25) // pool of 25

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{paladin, target}, nil
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin: paladin,
		Target:  target,
		Turn:    makeAvailableTurn(),
		HP:      15,
	}

	result, err := svc.LayOnHands(ctx, cmd)
	require.NoError(t, err)
	assert.Equal(t, 10, result.PoolRemaining)
	assert.Equal(t, int32(30), result.HPAfter)
	assert.Contains(t, result.CombatLog, "Thorn")
	assert.Contains(t, result.CombatLog, "Aria")
	assert.Contains(t, result.CombatLog, "15 HP")
	assert.Contains(t, result.CombatLog, "10/25")
	assert.True(t, result.Turn.ActionUsed)
}

// TDD Cycle 5: Validation errors

func TestLayOnHands_ActionAlreadyUsed(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()
	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin: makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:  makeTargetCombatant(targetCombatantID, encounterID),
		Turn:    refdata.Turn{ID: uuid.New(), ActionUsed: true},
		HP:      10,
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrResourceSpent)
}

func TestLayOnHands_NotCharacter(t *testing.T) {
	encounterID, _, targetCombatantID, _, ms := layOnHandsTestSetup()
	svc := NewService(ms)

	npc := refdata.Combatant{
		ID:          uuid.New(),
		EncounterID: encounterID,
		IsNpc:       true,
		DisplayName: "NPC",
		PositionCol: "C",
		PositionRow: 3,
		Conditions:  json.RawMessage(`[]`),
	}

	cmd := LayOnHandsCommand{
		Paladin: npc,
		Target:  makeTargetCombatant(targetCombatantID, encounterID),
		Turn:    makeAvailableTurn(),
		HP:      10,
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not NPC")
}

func TestLayOnHands_UndeadTarget(t *testing.T) {
	encounterID, paladinCombatantID, _, charID, ms := layOnHandsTestSetup()

	undeadTarget := refdata.Combatant{
		ID:            uuid.New(),
		EncounterID:   encounterID,
		ShortID:       "ZM",
		DisplayName:   "Zombie",
		CreatureRefID: sql.NullString{String: "zombie", Valid: true},
		PositionCol:   "C",
		PositionRow:   4,
		HpMax:         22,
		HpCurrent:     10,
		IsNpc:         true,
		IsAlive:       true,
		Conditions:    json.RawMessage(`[]`),
	}

	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "zombie", Type: "undead"}, nil
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin: makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:  undeadTarget,
		Turn:    makeAvailableTurn(),
		HP:      10,
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "undead or constructs")
}

func TestLayOnHands_ConstructTarget(t *testing.T) {
	encounterID, paladinCombatantID, _, charID, ms := layOnHandsTestSetup()

	constructTarget := refdata.Combatant{
		ID:            uuid.New(),
		EncounterID:   encounterID,
		ShortID:       "GL",
		DisplayName:   "Golem",
		CreatureRefID: sql.NullString{String: "golem", Valid: true},
		PositionCol:   "C",
		PositionRow:   4,
		HpMax:         50,
		HpCurrent:     30,
		IsNpc:         true,
		IsAlive:       true,
		Conditions:    json.RawMessage(`[]`),
	}

	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "golem", Type: "construct"}, nil
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin: makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:  constructTarget,
		Turn:    makeAvailableTurn(),
		HP:      10,
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "undead or constructs")
}

func TestLayOnHands_OutOfRange(t *testing.T) {
	encounterID, paladinCombatantID, _, charID, ms := layOnHandsTestSetup()

	farTarget := refdata.Combatant{
		ID:          uuid.New(),
		EncounterID: encounterID,
		ShortID:     "AR",
		DisplayName: "Aria",
		PositionCol: "H", // far away
		PositionRow: 10,
		HpMax:       30,
		HpCurrent:   15,
		IsNpc:       false,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
		CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
	}

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return makePaladinChar(charID, 5, 25), nil
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin: makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:  farTarget,
		Turn:    makeAvailableTurn(),
		HP:      10,
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestLayOnHands_NotPaladin(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()

	// Create a fighter character (not paladin)
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Fighter", Level: 5}})
	char := refdata.Character{
		ID:          charID,
		Name:        "NotPaladin",
		Classes:     classesJSON,
		FeatureUses: pqtype.NullRawMessage{RawMessage: json.RawMessage(`{}`), Valid: true},
	}

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin: makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:  makeTargetCombatant(targetCombatantID, encounterID),
		Turn:    makeAvailableTurn(),
		HP:      10,
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Paladin class")
}

func TestLayOnHands_PoolExceeded(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()
	char := makePaladinChar(charID, 5, 5) // only 5 HP left in pool

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin: makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:  makeTargetCombatant(targetCombatantID, encounterID),
		Turn:    makeAvailableTurn(),
		HP:      10, // requesting more than pool has
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient")
}

func TestLayOnHands_ZeroHP(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return makePaladinChar(charID, 5, 25), nil
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin: makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:  makeTargetCombatant(targetCombatantID, encounterID),
		Turn:    makeAvailableTurn(),
		HP:      0,
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least 1 HP")
}

// TDD Cycle 6: Self-targeting

func TestLayOnHands_SelfTarget(t *testing.T) {
	encounterID, paladinCombatantID, _, charID, ms := layOnHandsTestSetup()

	paladin := makePaladinCombatant(paladinCombatantID, encounterID, charID)
	paladin.HpCurrent = 20 // injured
	char := makePaladinChar(charID, 5, 25)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
	}

	svc := NewService(ms)

	// Self-targeting: paladin is both caster and target
	cmd := LayOnHandsCommand{
		Paladin: paladin,
		Target:  paladin, // same combatant
		Turn:    makeAvailableTurn(),
		HP:      10,
	}

	result, err := svc.LayOnHands(ctx, cmd)
	require.NoError(t, err)
	assert.Equal(t, int32(30), result.HPAfter)
	assert.Equal(t, 15, result.PoolRemaining)
	assert.Contains(t, result.CombatLog, "Thorn")
}

// TDD Cycle 7: HP capped at max

func TestLayOnHands_HPCappedAtMax(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()

	target := makeTargetCombatant(targetCombatantID, encounterID)
	target.HpCurrent = 28 // only 2 HP missing (max is 30)

	char := makePaladinChar(charID, 5, 25)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin: makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:  target,
		Turn:    makeAvailableTurn(),
		HP:      10, // requesting 10 but only 2 missing
	}

	result, err := svc.LayOnHands(ctx, cmd)
	require.NoError(t, err)
	assert.Equal(t, int32(30), result.HPAfter)    // capped at max
	assert.Equal(t, int32(2), result.HPRestored)   // only healed 2
}

// TDD Cycle 8: Cure poison

func TestLayOnHands_CurePoison(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()

	target := makeTargetCombatant(targetCombatantID, encounterID)
	// Add poisoned condition
	poisonedConds, _ := json.Marshal([]CombatCondition{{Condition: "poisoned"}})
	target.Conditions = poisonedConds

	char := makePaladinChar(charID, 5, 25)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return target, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin:    makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:     target,
		Turn:       makeAvailableTurn(),
		HP:         5,
		CurePoison: true,
	}

	result, err := svc.LayOnHands(ctx, cmd)
	require.NoError(t, err)
	assert.True(t, result.CuredPoison)
	assert.Equal(t, 15, result.PoolRemaining) // 25 - 5 (heal) - 5 (cure) = 15
	assert.Contains(t, result.CombatLog, "Poison")
}

// TDD Cycle 9: Cure disease

func TestLayOnHands_CureDisease(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()

	target := makeTargetCombatant(targetCombatantID, encounterID)
	diseasedConds, _ := json.Marshal([]CombatCondition{{Condition: "diseased"}})
	target.Conditions = diseasedConds

	char := makePaladinChar(charID, 5, 25)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return target, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin:     makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:      target,
		Turn:        makeAvailableTurn(),
		HP:          0,
		CureDisease: true,
	}

	result, err := svc.LayOnHands(ctx, cmd)
	require.NoError(t, err)
	assert.True(t, result.CuredDisease)
	assert.Equal(t, 20, result.PoolRemaining) // 25 - 5 (cure) = 20
	assert.Contains(t, result.CombatLog, "Disease")
}

// TDD Cycle 10: Cure poison + heal combined cost

func TestLayOnHands_CurePoisonPoolInsufficient(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()

	target := makeTargetCombatant(targetCombatantID, encounterID)
	poisonedConds, _ := json.Marshal([]CombatCondition{{Condition: "poisoned"}})
	target.Conditions = poisonedConds

	char := makePaladinChar(charID, 1, 4) // only 4 HP in pool, need 5 for cure

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin:    makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:     target,
		Turn:       makeAvailableTurn(),
		HP:         0,
		CurePoison: true,
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient")
}

// TDD Cycle 11: formatLayOnHandsLog

func TestFormatLayOnHandsLog(t *testing.T) {
	t.Run("healing only", func(t *testing.T) {
		log := formatLayOnHandsLog("Thorn", "Aria", 15, 10, 25, false, false)
		assert.Contains(t, log, "Thorn uses Lay on Hands on Aria")
		assert.Contains(t, log, "restores 15 HP")
		assert.Contains(t, log, "10/25")
	})

	t.Run("cure poison only", func(t *testing.T) {
		log := formatLayOnHandsLog("Thorn", "Aria", 0, 20, 25, true, false)
		assert.Contains(t, log, "cures Aria of Poison")
		assert.Contains(t, log, "20/25")
	})

	t.Run("cure disease only", func(t *testing.T) {
		log := formatLayOnHandsLog("Thorn", "Aria", 0, 20, 25, false, true)
		assert.Contains(t, log, "cures Aria of Disease")
	})

	t.Run("heal + cure poison", func(t *testing.T) {
		log := formatLayOnHandsLog("Thorn", "Aria", 5, 15, 25, true, false)
		assert.Contains(t, log, "restores 5 HP")
		assert.Contains(t, log, "cures Aria of Poison")
	})
}

// TDD Cycle 12: ParseFeatureUses error handling

func TestLayOnHands_ParseFeatureUsesError(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()

	char := makePaladinChar(charID, 5, 25)
	char.FeatureUses = pqtype.NullRawMessage{RawMessage: json.RawMessage(`invalid`), Valid: true}

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin: makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:  makeTargetCombatant(targetCombatantID, encounterID),
		Turn:    makeAvailableTurn(),
		HP:      10,
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing feature_uses")
}

// TDD Cycle 13: Healing + cure combined cost

func TestLayOnHands_CurePoisonAndHeal(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()

	target := makeTargetCombatant(targetCombatantID, encounterID)
	poisonedConds, _ := json.Marshal([]CombatCondition{{Condition: "poisoned"}})
	target.Conditions = poisonedConds

	char := makePaladinChar(charID, 5, 25)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent}, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return target, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin:    makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:     target,
		Turn:       makeAvailableTurn(),
		HP:         10,
		CurePoison: true,
	}

	result, err := svc.LayOnHands(ctx, cmd)
	require.NoError(t, err)
	assert.Equal(t, 10, result.PoolRemaining) // 25 - 10 (heal) - 5 (cure) = 10
	assert.True(t, result.CuredPoison)
	assert.Equal(t, int32(10), result.HPRestored)
}

// TDD Cycle 14: Cure both poison and disease

func TestLayOnHands_CureBothPoisonAndDisease(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()

	target := makeTargetCombatant(targetCombatantID, encounterID)
	bothConds, _ := json.Marshal([]CombatCondition{{Condition: "poisoned"}, {Condition: "diseased"}})
	target.Conditions = bothConds

	char := makePaladinChar(charID, 5, 25)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return target, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin:     makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:      target,
		Turn:        makeAvailableTurn(),
		HP:          0,
		CurePoison:  true,
		CureDisease: true,
	}

	result, err := svc.LayOnHands(ctx, cmd)
	require.NoError(t, err)
	assert.True(t, result.CuredPoison)
	assert.True(t, result.CuredDisease)
	assert.Equal(t, 15, result.PoolRemaining) // 25 - 5 - 5 = 15
}

// TDD Cycle 15: Error paths for DB failures

func TestLayOnHands_GetCharacterError(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin: makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:  makeTargetCombatant(targetCombatantID, encounterID),
		Turn:    makeAvailableTurn(),
		HP:      10,
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting character")
}

func TestLayOnHands_GetCreatureError(t *testing.T) {
	encounterID, paladinCombatantID, _, charID, ms := layOnHandsTestSetup()

	creatureTarget := refdata.Combatant{
		ID:            uuid.New(),
		EncounterID:   encounterID,
		ShortID:       "GH",
		DisplayName:   "Ghost",
		CreatureRefID: sql.NullString{String: "ghost", Valid: true},
		PositionCol:   "C",
		PositionRow:   4,
		HpMax:         22,
		HpCurrent:     10,
		IsNpc:         true,
		IsAlive:       true,
		Conditions:    json.RawMessage(`[]`),
	}

	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin: makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:  creatureTarget,
		Turn:    makeAvailableTurn(),
		HP:      10,
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting creature")
}

func TestLayOnHands_UpdateTurnActionsError(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()

	char := makePaladinChar(charID, 5, 25)
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin: makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:  makeTargetCombatant(targetCombatantID, encounterID),
		Turn:    makeAvailableTurn(),
		HP:      10,
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

func TestLayOnHands_UpdateCombatantHPError(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()

	char := makePaladinChar(charID, 5, 25)
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin: makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:  makeTargetCombatant(targetCombatantID, encounterID),
		Turn:    makeAvailableTurn(),
		HP:      10,
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating target HP")
}

func TestLayOnHands_RemoveConditionError(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()

	target := makeTargetCombatant(targetCombatantID, encounterID)
	poisonedConds, _ := json.Marshal([]CombatCondition{{Condition: "poisoned"}})
	target.Conditions = poisonedConds

	char := makePaladinChar(charID, 5, 25)
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin:    makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:     target,
		Turn:       makeAvailableTurn(),
		HP:         5,
		CurePoison: true,
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "removing poisoned condition")
}

func TestLayOnHands_RemoveDiseaseConditionError(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()

	target := makeTargetCombatant(targetCombatantID, encounterID)
	diseasedConds, _ := json.Marshal([]CombatCondition{{Condition: "diseased"}})
	target.Conditions = diseasedConds

	char := makePaladinChar(charID, 5, 25)
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin:     makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:      target,
		Turn:        makeAvailableTurn(),
		HP:          0,
		CureDisease: true,
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "removing diseased condition")
}

func TestLayOnHands_DeductPoolError(t *testing.T) {
	encounterID, paladinCombatantID, targetCombatantID, charID, ms := layOnHandsTestSetup()

	char := makePaladinChar(charID, 5, 25)
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)

	cmd := LayOnHandsCommand{
		Paladin: makePaladinCombatant(paladinCombatantID, encounterID, charID),
		Target:  makeTargetCombatant(targetCombatantID, encounterID),
		Turn:    makeAvailableTurn(),
		HP:      10,
	}

	_, err := svc.LayOnHands(ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating feature_uses")
}
