package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Grapple Tests ---

func TestGrapple_Success(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	grappler := makePCCombatant(combatantID, encounterID, charID, "Aria")
	grappler.PositionCol = "C"
	grappler.PositionRow = 3

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Goblin #1", "goblin")
	target.PositionCol = "D"
	target.PositionRow = 3

	turn := makeBasicTurn()
	char := makeBasicChar(charID, 30) // STR 16 => +3

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "goblin",
			Size:          "Small",
			AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`),
		}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		c := target
		c.Conditions = arg.Conditions
		return c, nil
	}
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	// Grappler rolls 13 (13+3=16), target rolls 9 (9+2=11 Acrobatics DEX +2)
	roller := dice.NewRoller(fixedRand(13, 9))

	result, err := svc.Grapple(context.Background(), GrappleCommand{
		Grappler:  grappler,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
	}, roller)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.CombatLog, "grapples")
	assert.Contains(t, result.CombatLog, "Aria")
	assert.Contains(t, result.CombatLog, "Goblin #1")
	assert.Contains(t, result.CombatLog, "Grappled!")
	assert.True(t, HasCondition(result.Target.Conditions, "grappled"))
	assert.True(t, result.Turn.ActionUsed)
}

func TestGrapple_Failure(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	grappler := makePCCombatant(combatantID, encounterID, charID, "Aria")
	grappler.PositionCol = "C"
	grappler.PositionRow = 3

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Orc Shaman", "orc_shaman")
	target.PositionCol = "D"
	target.PositionRow = 3

	turn := makeBasicTurn()
	char := makeBasicChar(charID, 30)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "orc_shaman",
			Size:          "Medium",
			AbilityScores: json.RawMessage(`{"str":18,"dex":10,"con":14,"int":12,"wis":14,"cha":10}`),
		}, nil
	}
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	// Grappler rolls 7 (7+3=10), target rolls 14 (14+4=18 Athletics STR +4)
	roller := dice.NewRoller(fixedRand(7, 14))

	result, err := svc.Grapple(context.Background(), GrappleCommand{
		Grappler:  grappler,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
	}, roller)

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.CombatLog, "attempts to grapple")
	assert.Contains(t, result.CombatLog, "Failed")
	assert.True(t, result.Turn.ActionUsed)
}

func TestGrapple_NoFreeHand(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	grappler := makePCCombatant(combatantID, encounterID, charID, "Aria")
	grappler.PositionCol = "C"
	grappler.PositionRow = 3

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Goblin", "goblin")
	target.PositionCol = "D"
	target.PositionRow = 3

	turn := makeBasicTurn()
	char := makeBasicChar(charID, 30)
	char.EquippedMainHand = sql.NullString{String: "longsword", Valid: true}
	char.EquippedOffHand = sql.NullString{String: "shield", Valid: true}

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "goblin", Size: "Small", AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`)}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(fixedRand(15, 10))

	_, err := svc.Grapple(context.Background(), GrappleCommand{
		Grappler:  grappler,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
	}, roller)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "free hand")
}

func TestGrapple_TargetTooLarge(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	grappler := makePCCombatant(combatantID, encounterID, charID, "Aria")
	grappler.PositionCol = "C"
	grappler.PositionRow = 3

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Dragon", "dragon")
	target.PositionCol = "D"
	target.PositionRow = 3

	turn := makeBasicTurn()
	char := makeBasicChar(charID, 30)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "dragon", Size: "Huge", AbilityScores: json.RawMessage(`{"str":20,"dex":10,"con":18,"int":14,"wis":12,"cha":16}`)}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(fixedRand(15, 10))

	_, err := svc.Grapple(context.Background(), GrappleCommand{
		Grappler:  grappler,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
	}, roller)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "size")
}

func TestGrapple_NotAdjacent(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	grappler := makePCCombatant(combatantID, encounterID, charID, "Aria")
	grappler.PositionCol = "A"
	grappler.PositionRow = 1

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Goblin", "goblin")
	target.PositionCol = "D"
	target.PositionRow = 5

	turn := makeBasicTurn()
	char := makeBasicChar(charID, 30)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "goblin", Size: "Small", AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`)}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(fixedRand(15, 10))

	_, err := svc.Grapple(context.Background(), GrappleCommand{
		Grappler:  grappler,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
	}, roller)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "5ft")
}

func TestGrapple_NoAction(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	grappler := makePCCombatant(combatantID, encounterID, charID, "Aria")
	grappler.PositionCol = "C"
	grappler.PositionRow = 3

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Goblin", "goblin")
	target.PositionCol = "D"
	target.PositionRow = 3

	turn := makeBasicTurn()
	turn.ActionUsed = true

	svc := NewService(ms)
	roller := dice.NewRoller(fixedRand(15, 10))

	_, err := svc.Grapple(context.Background(), GrappleCommand{
		Grappler:  grappler,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
	}, roller)

	require.Error(t, err)
}

func TestGrapple_Incapacitated(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	grappler := makePCCombatant(combatantID, encounterID, charID, "Aria")
	stunnedCond, _ := json.Marshal([]CombatCondition{{Condition: "stunned"}})
	grappler.Conditions = stunnedCond

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Goblin", "goblin")

	svc := NewService(ms)
	roller := dice.NewRoller(fixedRand(15, 10))

	_, err := svc.Grapple(context.Background(), GrappleCommand{
		Grappler:  grappler,
		Target:    target,
		Turn:      makeBasicTurn(),
		Encounter: makeBasicEncounter(encounterID, 1),
	}, roller)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot act")
}

func TestGrapple_NPCFreeHandAssumed(t *testing.T) {
	// NPCs always assumed to have a free hand
	encounterID, _, _, ms := makeStdTestSetup()
	grapplerID := uuid.New()
	grappler := makeNPCCombatantWithCreature(grapplerID, encounterID, "Ogre", "ogre")
	grappler.PositionCol = "C"
	grappler.PositionRow = 3

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Goblin", "goblin")
	target.PositionCol = "D"
	target.PositionRow = 3

	turn := makeBasicTurn()

	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		if id == "ogre" {
			return refdata.Creature{ID: "ogre", Size: "Large", AbilityScores: json.RawMessage(`{"str":19,"dex":8,"con":16,"int":5,"wis":7,"cha":7}`)}, nil
		}
		return refdata.Creature{ID: "goblin", Size: "Small", AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`)}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		c := target
		c.Conditions = arg.Conditions
		return c, nil
	}
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	roller := dice.NewRoller(fixedRand(15, 5))

	result, err := svc.Grapple(context.Background(), GrappleCommand{
		Grappler:  grappler,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
	}, roller)

	require.NoError(t, err)
	assert.True(t, result.Success)
}

// --- Shove Tests ---

func TestShove_Prone_Success(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	shover := makePCCombatant(combatantID, encounterID, charID, "Aria")
	shover.PositionCol = "C"
	shover.PositionRow = 3

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Goblin #1", "goblin")
	target.PositionCol = "D"
	target.PositionRow = 3

	turn := makeBasicTurn()
	char := makeBasicChar(charID, 30)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "goblin", Size: "Small", AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`)}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		c := target
		c.Conditions = arg.Conditions
		return c, nil
	}
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	// Shover rolls 12 (12+3=15), target rolls 10 (10+2=12 Acrobatics)
	roller := dice.NewRoller(fixedRand(12, 10))

	result, err := svc.Shove(context.Background(), ShoveCommand{
		Shover:    shover,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
		Mode:      ShoveProne,
	}, roller)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.CombatLog, "shoves")
	assert.Contains(t, result.CombatLog, "prone")
	assert.Contains(t, result.CombatLog, "Knocked prone!")
	assert.True(t, HasCondition(result.Target.Conditions, "prone"))
	assert.True(t, result.Turn.ActionUsed)
}

func TestShove_Push_Success(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	shover := makePCCombatant(combatantID, encounterID, charID, "Aria")
	shover.PositionCol = "C"
	shover.PositionRow = 3

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Goblin #1", "goblin")
	target.PositionCol = "D"
	target.PositionRow = 3

	turn := makeBasicTurn()
	char := makeBasicChar(charID, 30)

	allCombatants := []refdata.Combatant{shover, target}

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "goblin", Size: "Small", AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`)}, nil
	}
	ms.updateCombatantPositionFn = func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
		c := target
		c.PositionCol = arg.PositionCol
		c.PositionRow = arg.PositionRow
		return c, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
		return allCombatants, nil
	}
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	roller := dice.NewRoller(fixedRand(12, 10))

	result, err := svc.Shove(context.Background(), ShoveCommand{
		Shover:    shover,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
		Mode:      ShovePush,
	}, roller)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.CombatLog, "shoves")
	assert.Contains(t, result.CombatLog, "push")
	assert.Contains(t, result.CombatLog, "Pushed to")
	assert.True(t, result.Turn.ActionUsed)
}

func TestShove_Failure(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	shover := makePCCombatant(combatantID, encounterID, charID, "Aria")
	shover.PositionCol = "C"
	shover.PositionRow = 3

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Orc Shaman", "orc_shaman")
	target.PositionCol = "D"
	target.PositionRow = 3

	turn := makeBasicTurn()
	char := makeBasicChar(charID, 30)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "orc_shaman", Size: "Medium", AbilityScores: json.RawMessage(`{"str":18,"dex":10,"con":14,"int":12,"wis":14,"cha":10}`)}, nil
	}
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	// Shover rolls 5 (5+3=8), target rolls 10 (10+4=14 Athletics)
	roller := dice.NewRoller(fixedRand(5, 10))

	result, err := svc.Shove(context.Background(), ShoveCommand{
		Shover:    shover,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
		Mode:      ShovePush,
	}, roller)

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.CombatLog, "attempts to shove")
	assert.Contains(t, result.CombatLog, "Failed")
}

func TestShove_TargetTooLarge(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	shover := makePCCombatant(combatantID, encounterID, charID, "Aria")
	shover.PositionCol = "C"
	shover.PositionRow = 3

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Dragon", "dragon")
	target.PositionCol = "D"
	target.PositionRow = 3

	turn := makeBasicTurn()
	char := makeBasicChar(charID, 30)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "dragon", Size: "Huge", AbilityScores: json.RawMessage(`{"str":20,"dex":10,"con":18,"int":14,"wis":12,"cha":16}`)}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(fixedRand(15, 10))

	_, err := svc.Shove(context.Background(), ShoveCommand{
		Shover:    shover,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
		Mode:      ShoveProne,
	}, roller)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "size")
}

func TestShove_Push_OccupiedDestination(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	shover := makePCCombatant(combatantID, encounterID, charID, "Aria")
	shover.PositionCol = "C"
	shover.PositionRow = 3

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Goblin", "goblin")
	target.PositionCol = "D"
	target.PositionRow = 3

	// Blocker at E3 (the push destination)
	blockerID := uuid.New()
	blocker := makeNPCCombatantWithCreature(blockerID, encounterID, "Orc", "orc")
	blocker.PositionCol = "E"
	blocker.PositionRow = 3

	allCombatants := []refdata.Combatant{shover, target, blocker}

	turn := makeBasicTurn()
	char := makeBasicChar(charID, 30)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		if id == "goblin" {
			return refdata.Creature{ID: "goblin", Size: "Small", AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`)}, nil
		}
		return refdata.Creature{ID: "orc", Size: "Medium", AbilityScores: json.RawMessage(`{"str":16,"dex":12,"con":14,"int":7,"wis":11,"cha":10}`)}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
		return allCombatants, nil
	}
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	roller := dice.NewRoller(fixedRand(15, 5))

	_, err := svc.Shove(context.Background(), ShoveCommand{
		Shover:    shover,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
		Mode:      ShovePush,
	}, roller)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "occupied")
}

// --- Dragging Tests ---

func TestDragCheck_HasGrappledTargets(t *testing.T) {
	encounterID, combatantID, _, ms := makeStdTestSetup()
	grappler := makePCCombatant(combatantID, encounterID, uuid.New(), "Aria")

	grappledID := uuid.New()
	grappledTarget := makeNPCCombatant(grappledID, encounterID, "Goblin #1")
	grappledCond, _ := json.Marshal([]CombatCondition{{Condition: "grappled", SourceCombatantID: combatantID.String()}})
	grappledTarget.Conditions = grappledCond

	otherID := uuid.New()
	otherCombatant := makeNPCCombatant(otherID, encounterID, "Orc")

	allCombatants := []refdata.Combatant{grappler, grappledTarget, otherCombatant}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
		return allCombatants, nil
	}

	svc := NewService(ms)
	result, err := svc.CheckDragTargets(context.Background(), encounterID, grappler)
	require.NoError(t, err)
	assert.Len(t, result.GrappledTargets, 1)
	assert.Equal(t, "Goblin #1", result.GrappledTargets[0].DisplayName)
	assert.True(t, result.HasTargets)
}

func TestDragCheck_NoGrappledTargets(t *testing.T) {
	encounterID, combatantID, _, ms := makeStdTestSetup()
	grappler := makePCCombatant(combatantID, encounterID, uuid.New(), "Aria")

	otherID := uuid.New()
	otherCombatant := makeNPCCombatant(otherID, encounterID, "Orc")

	allCombatants := []refdata.Combatant{grappler, otherCombatant}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
		return allCombatants, nil
	}

	svc := NewService(ms)
	result, err := svc.CheckDragTargets(context.Background(), encounterID, grappler)
	require.NoError(t, err)
	assert.False(t, result.HasTargets)
	assert.Len(t, result.GrappledTargets, 0)
}

func TestFormatDragPrompt(t *testing.T) {
	targets := []refdata.Combatant{
		{DisplayName: "Goblin #1"},
		{DisplayName: "Orc Shaman"},
	}
	prompt := FormatDragPrompt(targets)
	assert.Contains(t, prompt, "Goblin #1")
	assert.Contains(t, prompt, "Orc Shaman")
	assert.Contains(t, prompt, "Drag")
	assert.Contains(t, prompt, "Release")
}

func TestDragMovementCost(t *testing.T) {
	// Dragging always costs x2 regardless of number of targets
	assert.Equal(t, 20, DragMovementCost(10))
	assert.Equal(t, 10, DragMovementCost(5))
	assert.Equal(t, 0, DragMovementCost(0))
}

func TestReleaseDrag_RemovesGrappleConditions(t *testing.T) {
	encounterID, combatantID, _, ms := makeStdTestSetup()
	grappler := makePCCombatant(combatantID, encounterID, uuid.New(), "Aria")

	grappledID := uuid.New()
	grappledTarget := makeNPCCombatant(grappledID, encounterID, "Goblin #1")
	grappledCond, _ := json.Marshal([]CombatCondition{{Condition: "grappled", SourceCombatantID: combatantID.String()}})
	grappledTarget.Conditions = grappledCond

	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions, DisplayName: "Goblin #1"}, nil
	}

	svc := NewService(ms)
	result, err := svc.ReleaseDrag(context.Background(), grappler, []refdata.Combatant{grappledTarget})
	require.NoError(t, err)
	assert.Len(t, result.Released, 1)
	assert.False(t, HasCondition(result.Released[0].Conditions, "grappled"))
	assert.Len(t, result.CombatLogs, 1)
	assert.Contains(t, result.CombatLogs[0], "releases")
}

func TestReleaseDrag_MultipleTargets(t *testing.T) {
	encounterID, combatantID, _, ms := makeStdTestSetup()
	grappler := makePCCombatant(combatantID, encounterID, uuid.New(), "Aria")

	grappledCond, _ := json.Marshal([]CombatCondition{{Condition: "grappled", SourceCombatantID: combatantID.String()}})

	t1ID := uuid.New()
	target1 := makeNPCCombatant(t1ID, encounterID, "Goblin #1")
	target1.Conditions = grappledCond

	t2ID := uuid.New()
	target2 := makeNPCCombatant(t2ID, encounterID, "Orc Shaman")
	t2Cond, _ := json.Marshal([]CombatCondition{{Condition: "grappled", SourceCombatantID: combatantID.String()}})
	target2.Conditions = t2Cond

	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions, DisplayName: "target"}, nil
	}

	svc := NewService(ms)
	result, err := svc.ReleaseDrag(context.Background(), grappler, []refdata.Combatant{target1, target2})
	require.NoError(t, err)
	assert.Len(t, result.Released, 2)
	assert.Len(t, result.CombatLogs, 2)
}

// --- Helper Tests ---

func TestCalculatePushDestination(t *testing.T) {
	tests := []struct {
		name           string
		attackerCol    string
		attackerRow    int
		targetCol      string
		targetRow      int
		expectedCol    int
		expectedRow    int
	}{
		{"push east", "C", 3, "D", 3, 5, 3},         // D=4 + 1 = 5 (E), row stays
		{"push north", "C", 3, "C", 2, 3, 1},         // col stays, row 2-1=1
		{"push south", "C", 3, "C", 4, 3, 5},         // col stays, row 4+1=5
		{"push west", "D", 3, "C", 3, 2, 3},          // C=3 - 1 = 2 (B), row stays
		{"push northeast", "C", 4, "D", 3, 5, 2},     // D=4+1=5, 3-1=2
		{"push same col diff row", "C", 3, "C", 4, 3, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col, row := calculatePushDestination(tt.attackerCol, tt.attackerRow, tt.targetCol, tt.targetRow)
			assert.Equal(t, tt.expectedCol, col)
			assert.Equal(t, tt.expectedRow, row)
		})
	}
}

func TestColIntToLabel(t *testing.T) {
	assert.Equal(t, "A", colIntToLabel(1))
	assert.Equal(t, "B", colIntToLabel(2))
	assert.Equal(t, "Z", colIntToLabel(26))
}

func TestDragCheck_MultipleGrappledTargets(t *testing.T) {
	encounterID, combatantID, _, ms := makeStdTestSetup()
	grappler := makePCCombatant(combatantID, encounterID, uuid.New(), "Aria")

	cond1, _ := json.Marshal([]CombatCondition{{Condition: "grappled", SourceCombatantID: combatantID.String()}})
	cond2, _ := json.Marshal([]CombatCondition{{Condition: "grappled", SourceCombatantID: combatantID.String()}})

	t1 := makeNPCCombatant(uuid.New(), encounterID, "Goblin #1")
	t1.Conditions = cond1
	t2 := makeNPCCombatant(uuid.New(), encounterID, "Orc")
	t2.Conditions = cond2

	allCombatants := []refdata.Combatant{grappler, t1, t2}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
		return allCombatants, nil
	}

	svc := NewService(ms)
	result, err := svc.CheckDragTargets(context.Background(), encounterID, grappler)
	require.NoError(t, err)
	assert.Len(t, result.GrappledTargets, 2)
	assert.True(t, result.HasTargets)
}

func TestGrapple_PCWithOneHandFree(t *testing.T) {
	// Main hand occupied, off hand free -> should succeed
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	grappler := makePCCombatant(combatantID, encounterID, charID, "Aria")
	grappler.PositionCol = "C"
	grappler.PositionRow = 3

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Goblin", "goblin")
	target.PositionCol = "D"
	target.PositionRow = 3

	turn := makeBasicTurn()
	char := makeBasicChar(charID, 30)
	char.EquippedMainHand = sql.NullString{String: "longsword", Valid: true}
	// Off hand empty

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "goblin", Size: "Small", AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`)}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		c := target
		c.Conditions = arg.Conditions
		return c, nil
	}
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	roller := dice.NewRoller(fixedRand(15, 5))

	result, err := svc.Grapple(context.Background(), GrappleCommand{
		Grappler:  grappler,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
	}, roller)

	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestShove_NotAdjacent(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	shover := makePCCombatant(combatantID, encounterID, charID, "Aria")
	shover.PositionCol = "A"
	shover.PositionRow = 1

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Goblin", "goblin")
	target.PositionCol = "D"
	target.PositionRow = 5

	turn := makeBasicTurn()
	char := makeBasicChar(charID, 30)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "goblin", Size: "Small", AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`)}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(fixedRand(15, 10))

	_, err := svc.Shove(context.Background(), ShoveCommand{
		Shover:    shover,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
		Mode:      ShoveProne,
	}, roller)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "5ft")
}

func TestShove_NoAction(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	shover := makePCCombatant(combatantID, encounterID, charID, "Aria")

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Goblin", "goblin")

	turn := makeBasicTurn()
	turn.ActionUsed = true

	svc := NewService(ms)
	roller := dice.NewRoller(fixedRand(15, 10))

	_, err := svc.Shove(context.Background(), ShoveCommand{
		Shover:    shover,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
		Mode:      ShoveProne,
	}, roller)

	require.Error(t, err)
}

func TestShove_Incapacitated(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	shover := makePCCombatant(combatantID, encounterID, charID, "Aria")
	stunnedCond, _ := json.Marshal([]CombatCondition{{Condition: "stunned"}})
	shover.Conditions = stunnedCond

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Goblin", "goblin")

	svc := NewService(ms)
	roller := dice.NewRoller(fixedRand(15, 10))

	_, err := svc.Shove(context.Background(), ShoveCommand{
		Shover:    shover,
		Target:    target,
		Turn:      makeBasicTurn(),
		Encounter: makeBasicEncounter(encounterID, 1),
		Mode:      ShoveProne,
	}, roller)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot act")
}

func TestGrapple_TargetUsesSTRWhenHigher(t *testing.T) {
	// Target with higher STR than DEX should use Athletics
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	grappler := makePCCombatant(combatantID, encounterID, charID, "Aria")
	grappler.PositionCol = "C"
	grappler.PositionRow = 3

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Orc", "orc")
	target.PositionCol = "D"
	target.PositionRow = 3

	turn := makeBasicTurn()
	char := makeBasicChar(charID, 30)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		// STR 16 (+3) > DEX 10 (+0), so target picks Athletics
		return refdata.Creature{ID: "orc", Size: "Medium", AbilityScores: json.RawMessage(`{"str":16,"dex":10,"con":14,"int":7,"wis":11,"cha":10}`)}, nil
	}
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	// Grappler rolls 14 (14+3=17), target rolls 10 (10+3=13 Athletics)
	roller := dice.NewRoller(fixedRand(14, 10))

	result, err := svc.Grapple(context.Background(), GrappleCommand{
		Grappler:  grappler,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
	}, roller)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.CombatLog, "Athletics")
	// Both are using Athletics so the log should show "vs Athletics:"
}

func TestGrapple_LargeTargetAllowed(t *testing.T) {
	// Medium grappler can grapple Large target (1 size larger)
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	grappler := makePCCombatant(combatantID, encounterID, charID, "Aria")
	grappler.PositionCol = "C"
	grappler.PositionRow = 3

	targetID := uuid.New()
	target := makeNPCCombatantWithCreature(targetID, encounterID, "Ogre", "ogre")
	target.PositionCol = "D"
	target.PositionRow = 3

	turn := makeBasicTurn()
	char := makeBasicChar(charID, 30)

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "ogre", Size: "Large", AbilityScores: json.RawMessage(`{"str":19,"dex":8,"con":16,"int":5,"wis":7,"cha":7}`)}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		c := target
		c.Conditions = arg.Conditions
		return c, nil
	}
	setupUpdateTurnActions(ms)

	svc := NewService(ms)
	roller := dice.NewRoller(fixedRand(18, 5))

	result, err := svc.Grapple(context.Background(), GrappleCommand{
		Grappler:  grappler,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
	}, roller)

	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestFormatDragPrompt_SingleTarget(t *testing.T) {
	targets := []refdata.Combatant{
		{DisplayName: "Goblin #1"},
	}
	prompt := FormatDragPrompt(targets)
	assert.Contains(t, prompt, "Goblin #1")
	assert.NotContains(t, prompt, ",")
}

// fixedRand for deterministic tests
func fixedRand(values ...int) dice.RandSource {
	i := 0
	return func(max int) int {
		v := values[i%len(values)]
		i++
		return v
	}
}
