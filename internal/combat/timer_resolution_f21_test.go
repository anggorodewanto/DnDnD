package combat

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// TestAutoResolveTurn_F21_SaveIncludesAbilityModifier proves that timeout
// auto-resolved saves add the character's save modifier (ability mod +
// proficiency if proficient) to the d20 roll before comparing to DC.
func TestAutoResolveTurn_F21_SaveIncludesAbilityModifier(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	encounterID := uuid.New()
	characterID := uuid.New()
	saveID := uuid.New()

	store := defaultMockStore()
	store.getTurnFn = func(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID: turnID, EncounterID: encounterID, CombatantID: combatantID,
			Status: "active", ActionUsed: true,
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: combatantID, EncounterID: encounterID, DisplayName: "Aria",
			HpCurrent: 20, HpMax: 30, IsAlive: true,
			CharacterID: uuid.NullUUID{UUID: characterID, Valid: true},
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCampaignByEncounterIDFn = func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{ID: uuid.New(), Settings: settingsJSON(24)}, nil
	}

	// Character: WIS 16 (+3 mod), proficient in WIS saves, prof bonus +2 → total save mod = +5
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		scores, _ := json.Marshal(map[string]int{
			"str": 10, "dex": 12, "con": 14, "int": 10, "wis": 16, "cha": 8,
		})
		profs, _ := json.Marshal(map[string][]string{"saves": {"wis", "cha"}})
		return refdata.Character{
			ID:               characterID,
			AbilityScores:    scores,
			ProficiencyBonus: 2,
			Proficiencies:    pqtype.NullRawMessage{RawMessage: profs, Valid: true},
		}, nil
	}

	// Pending WIS save DC 15
	store.listPendingSavesByCombatantFn = func(ctx context.Context, cID uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{{
			ID: saveID, EncounterID: encounterID, CombatantID: combatantID,
			Ability: "WIS", Dc: 15, Source: "Hold Person", Status: "pending",
		}}, nil
	}

	var capturedResult refdata.UpdatePendingSaveResultParams
	store.updatePendingSaveResultFn = func(ctx context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error) {
		capturedResult = arg
		return refdata.PendingSafe{ID: arg.ID, Status: "rolled", RollResult: arg.RollResult, Success: arg.Success}, nil
	}

	notifier := &mockNotifier{}
	timer := NewTurnTimer(store, notifier, 30*time.Second)

	// Roll 10 on d20. Without modifier: 10 < 15 = FAIL.
	// With modifier (+5): 10 + 5 = 15 >= 15 = SUCCESS.
	roller := dice.NewRoller(func(max int) int { return 10 })

	actions, err := timer.AutoResolveTurn(context.Background(), turnID, roller)
	require.NoError(t, err)

	// The total stored should be 15 (10 + 5), and the save should succeed.
	assert.Equal(t, saveID, capturedResult.ID)
	assert.True(t, capturedResult.RollResult.Valid)
	assert.Equal(t, int32(15), capturedResult.RollResult.Int32, "roll result should include save modifier: 10 (d20) + 5 (WIS mod + prof)")
	assert.True(t, capturedResult.Success.Valid)
	assert.True(t, capturedResult.Success.Bool, "save should succeed: total 15 >= DC 15")

	// Action message should show total 15
	assert.True(t, actionsContain(actions, "rolled 15"), "action message should show modified total, got: %v", actions)
}
