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

func TestAlertInitiativeBonus(t *testing.T) {
	alert, _ := json.Marshal([]CharacterFeature{{Name: "Alert"}})
	// case-insensitive match, mirroring HasFeatureByName.
	alertLower, _ := json.Marshal([]CharacterFeature{{Name: "alert"}})
	other, _ := json.Marshal([]CharacterFeature{{Name: "Tough"}})

	assert.Equal(t, 5, alertInitiativeBonus(alert), "Alert feat grants +5 initiative")
	assert.Equal(t, 5, alertInitiativeBonus(alertLower), "match is case-insensitive")
	assert.Equal(t, 0, alertInitiativeBonus(other), "no Alert → no bonus")
	assert.Equal(t, 0, alertInitiativeBonus(nil), "no features → no bonus")
}

// A character with the Alert feat adds +5 to the initiative roll total, enough to
// out-roll a higher-DEX combatant on the same d20. The +5 lands in the roll total,
// not the DexMod tie-break field.
func TestService_RollInitiative_AlertFeatAddsFive(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	alertCharID := uuid.New()
	fastCharID := uuid.New()

	alertPC := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Scout",
		CharacterID: uuid.NullUUID{UUID: alertCharID, Valid: true},
		Conditions:  json.RawMessage(`[]`),
	}
	// Higher DEX, but no Alert — should lose to the Alert PC on the +5.
	fastPC := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Rogue",
		CharacterID: uuid.NullUUID{UUID: fastCharID, Valid: true},
		Conditions:  json.RawMessage(`[]`),
	}

	alertChar := makeCharacterWithFeats(10, 10, 2, "", []CharacterFeature{{Name: "Alert"}}, nil) // DEX 10 → +0
	alertChar.ID = alertCharID
	fastChar := makeCharacterWithFeats(10, 18, 2, "", nil, nil) // DEX 18 → +4
	fastChar.ID = fastCharID

	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(context.Context, uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{alertPC, fastPC}, nil
	}
	store.getCharacterFn = func(_ context.Context, id uuid.UUID) (refdata.Character, error) {
		if id == alertCharID {
			return alertChar, nil
		}
		return fastChar, nil
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

	// Alert PC: 10 roll + 0 DEX + 5 Alert = 15. Fast PC: 10 roll + 4 DEX = 14.
	assert.Equal(t, int32(15), rolls[alertPC.ID], "Alert adds +5 to the initiative roll")
	assert.Equal(t, int32(14), rolls[fastPC.ID], "no Alert → DEX only, no bonus")
	assert.Equal(t, alertPC.ID, result[0].ID, "Alert PC beats the higher-DEX rogue via the +5")
}

// Creatures carry no features, so getInitiativeModifiers adds no Alert bonus to
// their rollBonus (Alert's +5 never applies to a monster's initiative). With no
// exhaustion here, rollBonus is 0; the exhaustion-on-creatures case is covered in
// exhaustion_initiative_test.go.
func TestService_getInitiativeModifiers_CreatureHasNoFeatBonus(t *testing.T) {
	store := defaultMockStore()
	store.getCreatureFn = func(context.Context, string) (refdata.Creature, error) {
		return refdata.Creature{ID: "goblin", AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`)}, nil
	}
	svc := NewService(store)
	combatant := refdata.Combatant{CreatureRefID: sql.NullString{String: "goblin", Valid: true}}

	dexMod, rollBonus, err := svc.getInitiativeModifiers(context.Background(), combatant)
	require.NoError(t, err)
	assert.Equal(t, 2, dexMod, "DEX 14 → +2")
	assert.Equal(t, 0, rollBonus, "no Alert feat and no exhaustion → zero roll bonus")
}
