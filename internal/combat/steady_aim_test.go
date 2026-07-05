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

// makeSteadyAimRogue builds a Rogue character carrying the seeded Steady Aim
// class feature (name-detected by the combat gate).
func makeSteadyAimRogue(charID uuid.UUID, level int) refdata.Character {
	char := makeRogueChar(charID, level)
	char.Features = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`[{"name":"Steady Aim","source":"rogue"}]`),
		Valid:      true,
	}
	return char
}

func makeSteadyAimCombatant(combatantID, encounterID, charID uuid.UUID) refdata.Combatant {
	return refdata.Combatant{
		ID:          combatantID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Shadow",
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}
}

// COV-8: Steady Aim, as a bonus action, writes a transient advantage marker on
// the rogue and spends the bonus action. The marker grants advantage on the
// rogue's attack this turn and clears at the start of their next turn (the
// generic start_of_turn condition-expiry machinery, mirroring the reckless
// marker) — no separate consume path.
func TestSteadyAim_HappyPath_GrantsMarkerAndSpendsBonusAction(t *testing.T) {
	encounterID, combatantID, charID := uuid.New(), uuid.New(), uuid.New()
	char := makeSteadyAimRogue(charID, 2)
	rogue := makeSteadyAimCombatant(combatantID, encounterID, charID)

	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return rogue, nil }
	var conds refdata.UpdateCombatantConditionsParams
	ms.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		conds = arg
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}
	var turnWrite refdata.UpdateTurnActionsParams
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		turnWrite = arg
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}

	svc := NewService(ms)
	turn := refdata.Turn{ID: uuid.New(), CombatantID: combatantID}
	result, err := svc.SteadyAim(context.Background(), SteadyAimCommand{Rogue: rogue, Turn: turn})
	require.NoError(t, err)
	assert.True(t, turnWrite.BonusActionUsed, "bonus action must be spent")
	assert.Contains(t, string(conds.Conditions), steadyAimAdvantageCondition)
	assert.Contains(t, result.CombatLog, "Steady Aim")
	assert.True(t, result.Turn.BonusActionUsed)
}

func TestSteadyAim_NoFeature_Errors(t *testing.T) {
	charID := uuid.New()
	char := makeRogueChar(charID, 2) // no Steady Aim feature
	rogue := makeSteadyAimCombatant(uuid.New(), uuid.New(), charID)

	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }

	svc := NewService(ms)
	_, err := svc.SteadyAim(context.Background(), SteadyAimCommand{Rogue: rogue, Turn: refdata.Turn{ID: uuid.New()}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Steady Aim")
}

func TestSteadyAim_BonusActionUsed_Errors(t *testing.T) {
	rogue := makeSteadyAimCombatant(uuid.New(), uuid.New(), uuid.New())
	svc := NewService(defaultMockStore())
	turn := refdata.Turn{ID: uuid.New(), BonusActionUsed: true}
	_, err := svc.SteadyAim(context.Background(), SteadyAimCommand{Rogue: rogue, Turn: turn})
	require.Error(t, err)
}

func TestSteadyAim_NPC_Errors(t *testing.T) {
	rogue := refdata.Combatant{ID: uuid.New(), DisplayName: "Goblin", Conditions: json.RawMessage(`[]`)} // no CharacterID
	svc := NewService(defaultMockStore())
	_, err := svc.SteadyAim(context.Background(), SteadyAimCommand{Rogue: rogue, Turn: refdata.Turn{ID: uuid.New()}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "character")
}

// The marker grants the attacker advantage, unconditionally (weapon- and
// target-agnostic — unlike vex/help which are target-scoped, and unlike reckless
// which is melee-STR only and also grants advantage to incoming attacks).
func TestDetectAdvantage_SteadyAim(t *testing.T) {
	mode, adv, disadv := DetectAdvantage(AdvantageInput{
		AttackerConditions: []CombatCondition{{Condition: steadyAimAdvantageCondition}},
	})
	assert.Equal(t, dice.Advantage, mode)
	assert.Contains(t, adv, "Steady Aim")
	assert.Empty(t, disadv)
}
