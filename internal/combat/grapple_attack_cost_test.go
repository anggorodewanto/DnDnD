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

// --- Bug 1b: Grapple / Shove cost ONE attack, not the whole Action ---
//
// 2024 rules make Grapple and Shove options of an Unarmed Strike, and an Unarmed
// Strike is one of the attacks of the Attack action. Charging the whole Action
// was wrong in both directions: a Fighter with Extra Attack lost a swing they
// were owed, and (before the UseResource fix) the untouched AttacksRemaining
// handed them the entire Attack action back for free.

func TestGrapple_ConsumesOneAttackNotWholeAction(t *testing.T) {
	svc, cmd, roller := grappleCostFixture(t, 2)

	result, err := svc.Grapple(context.Background(), cmd, roller)

	require.NoError(t, err)
	assert.Equal(t, int32(1), result.Turn.AttacksRemaining,
		"a grapple spends one attack; Extra Attack keeps the second")
	assert.True(t, result.Turn.ActionUsed,
		"the first attack of the turn still marks the Action used")
}

func TestGrapple_SingleAttackCombatantHasNothingLeft(t *testing.T) {
	svc, cmd, roller := grappleCostFixture(t, 1)

	result, err := svc.Grapple(context.Background(), cmd, roller)

	require.NoError(t, err)
	assert.Equal(t, int32(0), result.Turn.AttacksRemaining)
	assert.True(t, result.Turn.ActionUsed)
}

func TestGrapple_RejectedWhenNoAttacksRemain(t *testing.T) {
	svc, cmd, roller := grappleCostFixture(t, 0)

	_, err := svc.Grapple(context.Background(), cmd, roller)

	assert.ErrorIs(t, err, ErrResourceSpent)
}

func TestShove_ConsumesOneAttackNotWholeAction(t *testing.T) {
	svc, cmd, roller := grappleCostFixture(t, 2)

	result, err := svc.Shove(context.Background(), ShoveCommand{
		Shover:    cmd.Grappler,
		Target:    cmd.Target,
		Turn:      cmd.Turn,
		Encounter: cmd.Encounter,
		Mode:      ShoveProne,
	}, roller)

	require.NoError(t, err)
	assert.Equal(t, int32(1), result.Turn.AttacksRemaining)
	assert.True(t, result.Turn.ActionUsed)
}

// grappleCostFixture builds an adjacent grappler/target pair whose contested
// roll always succeeds, with the grappler's turn seeded to the given number of
// remaining attacks.
func grappleCostFixture(t *testing.T, attacks int32) (*Service, GrappleCommand, *dice.Roller) {
	t.Helper()

	encounterID, combatantID, charID, ms := makeStdTestSetup()
	grappler := makePCCombatant(combatantID, encounterID, charID, "Aria")
	grappler.PositionCol = "C"
	grappler.PositionRow = 3

	target := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Goblin #1", "goblin")
	target.PositionCol = "D"
	target.PositionRow = 3

	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return makeBasicChar(charID, 30), nil
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
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, id uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{grappler, target}, nil
	}
	setupUpdateTurnActions(ms)

	turn := makeBasicTurn()
	turn.AttacksRemaining = attacks

	return NewService(ms), GrappleCommand{
			Grappler:  grappler,
			Target:    target,
			Turn:      turn,
			Encounter: makeBasicEncounter(encounterID, 1),
		},
		// Grappler 13+3=16 vs target 9+2=11 — always a success.
		dice.NewRoller(fixedRand(13, 9))
}
