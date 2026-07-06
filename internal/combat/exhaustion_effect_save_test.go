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

// TestEffectSave_Exhaustion covers the 2024 rule that a saving throw is a d20
// Test, so an exhausted target takes -2 x level on the effect saves the engine
// rolls on its behalf: Stunning Strike (monk), Topple (mastery), and Cunning
// Strike (rogue). Each subtest picks a die that would *make* the save at zero
// exhaustion and asserts the penalty flips it into a failure; the control
// asserts the base-rules output is untouched when the target is not exhausted.
func TestEffectSave_Exhaustion(t *testing.T) {
	t.Run("stunning strike: exhaustion flips a made save into a stun", func(t *testing.T) {
		ctx := context.Background()
		charID := uuid.New()
		targetID := uuid.New()

		char := makeMonkCharacterWithKi(10, 16, 16, 5, 5) // DC 13
		char.ID = charID

		ms := defaultMockStore()
		ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
			return char, nil
		}
		ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`)}, nil
		}
		var applied json.RawMessage
		ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			applied = arg.Conditions
			return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
		}

		svc := NewService(ms)
		// Natural 18: passes DC 13 outright, but exhaustion 3 (-6) -> 12 < 13.
		roller := dice.NewRoller(func(max int) int { return 18 })

		result, err := svc.StunningStrike(ctx, StunningStrikeCommand{
			Attacker: refdata.Combatant{
				ID:          uuid.New(),
				CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
				DisplayName: "Kira",
				Conditions:  json.RawMessage(`[]`),
			},
			Target: refdata.Combatant{
				ID:              targetID,
				DisplayName:     "Goblin",
				Conditions:      json.RawMessage(`[]`),
				ExhaustionLevel: 3,
			},
			CurrentRound: 3,
		}, roller)
		require.NoError(t, err)
		assert.False(t, result.SaveSucceeded, "18 - 6 exhaustion = 12 should miss DC 13")
		assert.True(t, result.Stunned)
		assert.Contains(t, result.CombatLog, "stunned")
		assert.Contains(t, result.CombatLog, "exhaustion", "the log should disclose the exhaustion penalty")
		assert.True(t, HasCondition(applied, "stunned"))
	})

	t.Run("stunning strike: zero exhaustion leaves the base output byte-identical", func(t *testing.T) {
		ctx := context.Background()
		charID := uuid.New()

		char := makeMonkCharacterWithKi(10, 16, 16, 5, 5) // DC 13
		char.ID = charID

		ms := defaultMockStore()
		ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
			return char, nil
		}
		ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`)}, nil
		}

		svc := NewService(ms)
		roller := dice.NewRoller(func(max int) int { return 18 })

		result, err := svc.StunningStrike(ctx, StunningStrikeCommand{
			Attacker: refdata.Combatant{
				ID:          uuid.New(),
				CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
				DisplayName: "Kira",
				Conditions:  json.RawMessage(`[]`),
			},
			Target: refdata.Combatant{
				ID:          uuid.New(),
				DisplayName: "Goblin",
				Conditions:  json.RawMessage(`[]`),
				// ExhaustionLevel: 0
			},
			CurrentRound: 3,
		}, roller)
		require.NoError(t, err)
		assert.True(t, result.SaveSucceeded)
		assert.Contains(t, result.CombatLog, "(roll 18 + 0)", "zero-exhaustion breakdown must be unchanged")
		assert.NotContains(t, result.CombatLog, "exhaustion")
	})

	t.Run("topple: exhaustion flips a made save into prone", func(t *testing.T) {
		ctx := context.Background()
		targetID := uuid.New()

		ms := defaultMockStore()
		ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`)}, nil
		}
		var applied json.RawMessage
		ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			applied = arg.Conditions
			return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
		}

		svc := NewService(ms)
		// Natural 13 meets DC 13 outright, but exhaustion 1 (-2) -> 11 < 13.
		roller := dice.NewRoller(func(max int) int { return 13 })

		result := &AttackResult{MasteryToppleSaveDC: 13}
		attacker := refdata.Combatant{ID: uuid.New()}
		target := refdata.Combatant{ID: targetID, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`), ExhaustionLevel: 1}

		require.NoError(t, svc.applyToppleSave(ctx, attacker, target, result, roller))
		assert.True(t, HasCondition(applied, "prone"), "13 - 2 exhaustion = 11 should fail DC 13 and knock prone")
	})

	t.Run("topple: zero exhaustion keeps the base save", func(t *testing.T) {
		ctx := context.Background()

		ms := defaultMockStore()
		ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, Conditions: json.RawMessage(`[]`)}, nil
		}
		applyCalled := false
		ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			applyCalled = true
			return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
		}

		svc := NewService(ms)
		roller := dice.NewRoller(func(max int) int { return 13 })

		result := &AttackResult{MasteryToppleSaveDC: 13}
		attacker := refdata.Combatant{ID: uuid.New()}
		target := refdata.Combatant{ID: uuid.New(), Conditions: json.RawMessage(`[]`)}

		require.NoError(t, svc.applyToppleSave(ctx, attacker, target, result, roller))
		assert.False(t, applyCalled, "13 meets DC 13 with no exhaustion -> save succeeds, no prone")
	})

	t.Run("cunning strike: exhaustion flips a made save into the condition", func(t *testing.T) {
		ctx := context.Background()
		targetID := uuid.New()

		ms := defaultMockStore()
		ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`)}, nil
		}
		var applied json.RawMessage
		ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			applied = arg.Conditions
			return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
		}

		svc := NewService(ms)
		// Natural 13 meets the DC 13 outright, but exhaustion 1 (-2) -> 11 < 13.
		roller := dice.NewRoller(func(max int) int { return 13 })

		result := &AttackResult{CunningStrikeChoice: "trip", CunningStrikeSaveDC: 13}
		attacker := refdata.Combatant{ID: uuid.New()}
		target := refdata.Combatant{ID: targetID, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`), ExhaustionLevel: 1}

		require.NoError(t, svc.applyCunningStrike(ctx, attacker, target, result, roller))
		assert.False(t, result.CunningStrikeSaved, "13 - 2 exhaustion = 11 should fail DC 13")
		assert.True(t, HasCondition(applied, "prone"))
	})

	t.Run("cunning strike: zero exhaustion keeps the base save", func(t *testing.T) {
		ctx := context.Background()

		ms := defaultMockStore()
		ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, Conditions: json.RawMessage(`[]`)}, nil
		}
		ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
		}

		svc := NewService(ms)
		roller := dice.NewRoller(func(max int) int { return 13 })

		result := &AttackResult{CunningStrikeChoice: "trip", CunningStrikeSaveDC: 13}
		attacker := refdata.Combatant{ID: uuid.New()}
		target := refdata.Combatant{ID: uuid.New(), Conditions: json.RawMessage(`[]`)}

		require.NoError(t, svc.applyCunningStrike(ctx, attacker, target, result, roller))
		assert.True(t, result.CunningStrikeSaved, "13 meets DC 13 with no exhaustion -> save succeeds")
	})

	t.Run("turn undead: exhaustion flips a made WIS save into turned", func(t *testing.T) {
		clericID := uuid.New()
		clericCombatantID := uuid.New()
		skeletonCombatantID := uuid.New()
		encounterID := uuid.New()
		turnID := uuid.New()

		char := refdata.Character{
			ID:               clericID,
			Classes:          json.RawMessage(`[{"class":"Cleric","level":3}]`),
			AbilityScores:    json.RawMessage(`{"str":10,"dex":10,"con":14,"int":10,"wis":16,"cha":10}`),
			ProficiencyBonus: 2,
			FeatureUses:      pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"channel-divinity":{"current":1,"max":1,"recharge":"short"}}`), Valid: true},
		}
		cleric := refdata.Combatant{
			ID:          clericCombatantID,
			EncounterID: encounterID,
			CharacterID: uuid.NullUUID{UUID: clericID, Valid: true},
			DisplayName: "Thorn",
			PositionCol: "A",
			PositionRow: 1,
			Conditions:  json.RawMessage(`[]`),
		}
		skeleton := refdata.Combatant{
			ID:              skeletonCombatantID,
			EncounterID:     encounterID,
			CreatureRefID:   sql.NullString{String: "skeleton", Valid: true},
			DisplayName:     "Skeleton #1",
			PositionCol:     "A",
			PositionRow:     3,
			HpCurrent:       13,
			HpMax:           13,
			IsNpc:           true,
			IsAlive:         true,
			Conditions:      json.RawMessage(`[]`),
			ExhaustionLevel: 1,
		}
		creatures := map[string]refdata.Creature{
			"skeleton": {
				ID:            "skeleton",
				Type:          "undead",
				Cr:            "1/4",
				AbilityScores: json.RawMessage(`{"str":10,"dex":14,"con":15,"int":6,"wis":8,"cha":5}`),
			},
		}

		ms := newChannelDivinityMockStore(char, []refdata.Combatant{cleric, skeleton}, creatures)
		svc := NewService(ms)
		turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: clericCombatantID}

		// Skeleton WIS -1, DC 13. Natural 14 -> 13 meets the DC at zero
		// exhaustion, but exhaustion 1 (-2) -> 11 < 13 -> turned.
		roller := dice.NewRoller(func(max int) int { return 14 })

		result, err := svc.TurnUndead(ctx, TurnUndeadCommand{
			Cleric:       cleric,
			Turn:         turn,
			CurrentRound: 1,
		}, roller)
		require.NoError(t, err)
		require.Equal(t, 1, len(result.Targets))
		assert.False(t, result.Targets[0].SaveSucceeded, "14 - 1 - 2 exhaustion = 11 should miss DC 13")
		assert.True(t, result.Targets[0].Turned)
	})
}
