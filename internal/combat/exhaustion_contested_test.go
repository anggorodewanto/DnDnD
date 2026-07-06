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

// Contested checks (grapple / shove) are ability checks — D20 Tests — so 2024
// exhaustion's -2×level penalty applies to BOTH sides of the roll. These cases
// pin the same fixed dice that WITHOUT exhaustion decide one way, then flip the
// outcome purely by raising a combatant's ExhaustionLevel. The grappler/shover
// STR is +3 (makeBasicChar STR 16); the goblin defends with Acrobatics +2
// (DEX 14). Exhaustion 2 = -4.
func TestContestedCheck_Exhaustion(t *testing.T) {
	goblinScores := json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`)

	t.Run("grappler exhaustion lowers the grapple contest", func(t *testing.T) {
		encounterID, combatantID, charID, ms := makeStdTestSetup()
		grappler := makePCCombatant(combatantID, encounterID, charID, "Aria")
		grappler.PositionCol = "C"
		grappler.PositionRow = 3
		grappler.ExhaustionLevel = 2 // -4 to the contest

		target := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Goblin", "goblin")
		target.PositionCol = "D"
		target.PositionRow = 3

		ms.getCharacterFn = func(context.Context, uuid.UUID) (refdata.Character, error) {
			return makeBasicChar(charID, 30), nil
		}
		ms.getCreatureFn = func(context.Context, string) (refdata.Creature, error) {
			return refdata.Creature{ID: "goblin", Size: "Small", AbilityScores: goblinScores}, nil
		}
		setupUpdateTurnActions(ms)

		svc := NewService(ms)
		// die 12 vs 12. No exhaustion: 15 vs 14 → success. Exh 2: 11 vs 14 → fail.
		roller := dice.NewRoller(fixedRand(12, 12))
		result, err := svc.Grapple(context.Background(), GrappleCommand{
			Grappler: grappler, Target: target, Turn: makeBasicTurn(),
			Encounter: makeBasicEncounter(encounterID, 1),
		}, roller)

		require.NoError(t, err)
		assert.Equal(t, 11, result.GrapplerRoll, "12 + 3 STR - 4 exhaustion = 11")
		assert.False(t, result.Success, "exhausted grappler (11) loses to goblin (14)")
	})

	t.Run("shover exhaustion lowers the shove contest", func(t *testing.T) {
		encounterID, combatantID, charID, ms := makeStdTestSetup()
		shover := makePCCombatant(combatantID, encounterID, charID, "Aria")
		shover.PositionCol = "C"
		shover.PositionRow = 3
		shover.ExhaustionLevel = 2 // -4 to the contest

		target := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Goblin", "goblin")
		target.PositionCol = "D"
		target.PositionRow = 3

		ms.getCharacterFn = func(context.Context, uuid.UUID) (refdata.Character, error) {
			return makeBasicChar(charID, 30), nil
		}
		ms.getCreatureFn = func(context.Context, string) (refdata.Creature, error) {
			return refdata.Creature{ID: "goblin", Size: "Small", AbilityScores: goblinScores}, nil
		}
		setupUpdateTurnActions(ms)

		svc := NewService(ms)
		roller := dice.NewRoller(fixedRand(12, 12))
		result, err := svc.Shove(context.Background(), ShoveCommand{
			Shover: shover, Target: target, Turn: makeBasicTurn(),
			Encounter: makeBasicEncounter(encounterID, 1), Mode: ShoveProne,
		}, roller)

		require.NoError(t, err)
		assert.Equal(t, 11, result.ShoverRoll, "12 + 3 STR - 4 exhaustion = 11")
		assert.False(t, result.Success, "exhausted shover (11) loses to goblin (14)")
	})

	t.Run("target exhaustion eases the contest", func(t *testing.T) {
		encounterID, combatantID, charID, ms := makeStdTestSetup()
		grappler := makePCCombatant(combatantID, encounterID, charID, "Aria")
		grappler.PositionCol = "C"
		grappler.PositionRow = 3

		target := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Goblin", "goblin")
		target.PositionCol = "D"
		target.PositionRow = 3
		target.ExhaustionLevel = 2 // -4 to the target's defense

		ms.getCharacterFn = func(context.Context, uuid.UUID) (refdata.Character, error) {
			return makeBasicChar(charID, 30), nil
		}
		ms.getCreatureFn = func(context.Context, string) (refdata.Creature, error) {
			return refdata.Creature{ID: "goblin", Size: "Small", AbilityScores: goblinScores}, nil
		}
		ms.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			c := target
			c.Conditions = arg.Conditions
			return c, nil
		}
		setupUpdateTurnActions(ms)

		svc := NewService(ms)
		// die 10 vs 12. No exhaustion: 13 vs 14 → fail. Target exh 2: 13 vs 10 → success.
		roller := dice.NewRoller(fixedRand(10, 12))
		result, err := svc.Grapple(context.Background(), GrappleCommand{
			Grappler: grappler, Target: target, Turn: makeBasicTurn(),
			Encounter: makeBasicEncounter(encounterID, 1),
		}, roller)

		require.NoError(t, err)
		assert.Equal(t, 10, result.TargetRoll, "12 + 2 Acrobatics - 4 exhaustion = 10")
		assert.True(t, result.Success, "grappler (13) beats the exhausted goblin (10)")
	})

	t.Run("escapee exhaustion lowers the escape contest", func(t *testing.T) {
		encounterID, combatantID, charID, ms := makeStdTestSetup()
		escapee := makePCCombatant(combatantID, encounterID, charID, "Kael")
		grappledCond, _ := json.Marshal([]CombatCondition{{Condition: "grappled"}})
		escapee.Conditions = grappledCond
		escapee.ExhaustionLevel = 2 // -4 to the escape check

		grappler := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Ogre", "ogre")

		ms.getCharacterFn = func(context.Context, uuid.UUID) (refdata.Character, error) {
			return makeBasicChar(charID, 30), nil // STR 16 → +3
		}
		ms.getCreatureFn = func(context.Context, string) (refdata.Creature, error) {
			return refdata.Creature{ID: "ogre", AbilityScores: json.RawMessage(`{"str":10,"dex":8,"con":12,"int":6,"wis":10,"cha":8}`)}, nil // STR 10 → +0
		}
		setupUpdateTurnActions(ms)

		svc := NewService(ms)
		// die 12 vs 12. No exhaustion: 15 vs 12 → escape. Exh 2: 11 vs 12 → fail.
		roller := dice.NewRoller(fixedRand(12, 12))
		result, err := svc.Escape(context.Background(), EscapeCommand{
			Escapee: escapee, Grappler: grappler, Turn: makeBasicTurn(),
			Encounter: makeBasicEncounter(encounterID, 1), UseAthletics: true,
		}, roller)

		require.NoError(t, err)
		assert.Equal(t, 11, result.EscapeeRoll, "12 + 3 STR - 4 exhaustion = 11")
		assert.False(t, result.Success, "exhausted escapee (11) stays grappled by the ogre (12)")
	})

	t.Run("grappler exhaustion eases the escape contest", func(t *testing.T) {
		encounterID, combatantID, charID, ms := makeStdTestSetup()
		escapee := makePCCombatant(combatantID, encounterID, charID, "Kael")
		grappledCond, _ := json.Marshal([]CombatCondition{{Condition: "grappled"}})
		escapee.Conditions = grappledCond

		grappler := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Ogre", "ogre")
		grappler.ExhaustionLevel = 2 // -4 to the grappler's hold

		ms.getCharacterFn = func(context.Context, uuid.UUID) (refdata.Character, error) {
			return makeBasicChar(charID, 30), nil // STR 16 → +3
		}
		ms.getCreatureFn = func(context.Context, string) (refdata.Creature, error) {
			return refdata.Creature{ID: "ogre", AbilityScores: json.RawMessage(`{"str":14,"dex":8,"con":16,"int":6,"wis":10,"cha":8}`)}, nil // STR 14 → +2
		}
		ms.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			c := escapee
			c.Conditions = arg.Conditions
			return c, nil
		}
		setupUpdateTurnActions(ms)

		svc := NewService(ms)
		// die 10 vs 12. No exhaustion: 13 vs 14 → fail. Grappler exh 2: 13 vs 10 → escape.
		roller := dice.NewRoller(fixedRand(10, 12))
		result, err := svc.Escape(context.Background(), EscapeCommand{
			Escapee: escapee, Grappler: grappler, Turn: makeBasicTurn(),
			Encounter: makeBasicEncounter(encounterID, 1), UseAthletics: true,
		}, roller)

		require.NoError(t, err)
		assert.Equal(t, 10, result.GrapplerRoll, "12 + 2 STR - 4 exhaustion = 10")
		assert.True(t, result.Success, "escapee (13) breaks free of the exhausted ogre (10)")
	})
}
