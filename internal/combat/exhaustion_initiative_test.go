package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// Exhaustion is a 2024 d20-Test penalty (-2 x level). Initiative is a Dexterity
// check, so it takes the penalty on the roll total — but NOT on the DexMod
// tie-break field (exhaustion lowers your d20 results, not your Dexterity score).
// Unlike the Alert feat, exhaustion applies to monsters too, so the penalty is
// read from the combatant (c.ExhaustionLevel) in every getInitiativeModifiers
// branch, and composes with the Alert bonus.
func TestService_getInitiativeModifiers_Exhaustion(t *testing.T) {
	ctx := context.Background()

	t.Run("creature exhaustion penalizes the roll but not DexMod", func(t *testing.T) {
		store := defaultMockStore()
		store.getCreatureFn = func(context.Context, string) (refdata.Creature, error) {
			return refdata.Creature{ID: "goblin", AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`)}, nil
		}
		svc := NewService(store)
		combatant := refdata.Combatant{CreatureRefID: sql.NullString{String: "goblin", Valid: true}, ExhaustionLevel: 1}

		dexMod, rollBonus, err := svc.getInitiativeModifiers(ctx, combatant)
		require.NoError(t, err)
		assert.Equal(t, 2, dexMod, "DEX 14 → +2, unaffected by exhaustion")
		assert.Equal(t, -2, rollBonus, "exhaustion level 1 → -2 on a creature's roll")
	})

	t.Run("Alert bonus and exhaustion penalty compose on a character", func(t *testing.T) {
		charID := uuid.New()
		char := makeCharacterWithFeats(10, 10, 2, "", []CharacterFeature{{Name: "Alert"}}, nil) // DEX 10 → +0
		char.ID = charID
		store := defaultMockStore()
		store.getCharacterFn = func(context.Context, uuid.UUID) (refdata.Character, error) { return char, nil }
		svc := NewService(store)
		combatant := refdata.Combatant{CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, ExhaustionLevel: 2}

		dexMod, rollBonus, err := svc.getInitiativeModifiers(ctx, combatant)
		require.NoError(t, err)
		assert.Equal(t, 0, dexMod, "DEX 10 → +0")
		assert.Equal(t, 1, rollBonus, "Alert +5 and exhaustion -4 compose to +1")
	})
}

// An exhausted PC subtracts 2 x level from the initiative roll total, enough to
// drop below an unexhausted, same-DEX rival on the same d20.
func TestService_RollInitiative_ExhaustionLowersRoll(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	tiredCharID := uuid.New()
	freshCharID := uuid.New()

	tiredPC := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Weary",
		CharacterID:     uuid.NullUUID{UUID: tiredCharID, Valid: true},
		Conditions:      json.RawMessage(`[]`),
		ExhaustionLevel: 2, // -4 to the roll
	}
	freshPC := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Rested",
		CharacterID: uuid.NullUUID{UUID: freshCharID, Valid: true},
		Conditions:  json.RawMessage(`[]`),
	}

	tiredChar := makeCharacterWithFeats(10, 10, 2, "", nil, nil) // DEX 10 → +0
	tiredChar.ID = tiredCharID
	freshChar := makeCharacterWithFeats(10, 10, 2, "", nil, nil) // DEX 10 → +0
	freshChar.ID = freshCharID

	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(context.Context, uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{tiredPC, freshPC}, nil
	}
	store.getCharacterFn = func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
		if id == tiredCharID {
			return tiredChar, nil
		}
		return freshChar, nil
	}
	rolls := map[uuid.UUID]int32{}
	store.updateCombatantInitiativeFn = func(_ context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
		rolls[arg.ID] = arg.InitiativeRoll
		return refdata.Combatant{ID: arg.ID, InitiativeRoll: arg.InitiativeRoll, InitiativeOrder: arg.InitiativeOrder}, nil
	}

	svc := NewService(store)
	result, err := svc.RollInitiative(ctx, encounterID, newTestRoller(10))
	require.NoError(t, err)
	require.Len(t, result, 2)

	// Weary: 10 roll + 0 DEX - 4 exhaustion = 6. Rested: 10 roll + 0 DEX = 10.
	assert.Equal(t, int32(6), rolls[tiredPC.ID], "exhaustion level 2 subtracts 4 from the roll")
	assert.Equal(t, int32(10), rolls[freshPC.ID], "no exhaustion → DEX only")
	assert.Equal(t, freshPC.ID, result[0].ID, "the rested PC beats the exhausted one")
}
