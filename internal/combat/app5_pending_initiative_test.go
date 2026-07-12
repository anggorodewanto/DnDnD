package combat

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// APP-5: player-staged initiative (submitted via /initiative before combat)
// folded into the APP-1 character_initiatives map at StartCombat.

func TestMergePendingInitiatives_FoldsStagedForCombatants(t *testing.T) {
	inCombat := uuid.New()
	absent := uuid.New()

	staged := []refdata.ClearAndReturnPendingInitiativesRow{
		{CharacterID: inCombat, Roll: 17},
		{CharacterID: absent, Roll: 9},
	}
	merged := mergePendingInitiatives(nil, []uuid.UUID{inCombat}, staged)
	require.Len(t, merged, 1, "only the staged char that is in this combat is folded in")
	assert.Equal(t, int32(17), merged[inCombat].Roll)
	_, ok := merged[absent]
	assert.False(t, ok, "a staged char not in this combat is ignored")
}

func TestMergePendingInitiatives_ExplicitSuppliedWins(t *testing.T) {
	charID := uuid.New()

	staged := []refdata.ClearAndReturnPendingInitiativesRow{{CharacterID: charID, Roll: 5}}
	supplied := map[uuid.UUID]InitiativeInput{charID: {Roll: 20}}
	merged := mergePendingInitiatives(supplied, []uuid.UUID{charID}, staged)
	assert.Equal(t, int32(20), merged[charID].Roll, "an explicit start-body value overrides the staged one")
}

func TestMergePendingInitiatives_NoStagedReturnsSuppliedUnchanged(t *testing.T) {
	charID := uuid.New()

	supplied := map[uuid.UUID]InitiativeInput{charID: {Roll: 12}}
	merged := mergePendingInitiatives(supplied, []uuid.UUID{charID}, nil)
	require.Len(t, merged, 1)
	assert.Equal(t, int32(12), merged[charID].Roll, "no staged rows leaves the supplied map untouched")
}

// End-to-end: a player-staged roll is consumed by StartCombat (no body-supplied
// initiatives) and the staging rows are cleared afterwards.
func TestHandler_StartCombat_ConsumesAndClearsStagedInitiative(t *testing.T) {
	templateID := uuid.New()
	encounterID := uuid.New()
	charID := uuid.New()

	store := startCombatMockStore(templateID, encounterID, charID)
	// 25 is unreachable from the fixed test roller (15 + DEX 2 = 17), so seeing
	// it stored proves the staged value — not an auto-roll — was used. The single
	// ClearAndReturn call both surfaces the staged row and clears it.
	var clearedCampaign uuid.UUID
	store.clearAndReturnPendingInitiativesFn = func(_ context.Context, cid uuid.UUID) ([]refdata.ClearAndReturnPendingInitiativesRow, error) {
		clearedCampaign = cid
		return []refdata.ClearAndReturnPendingInitiativesRow{{CharacterID: charID, Roll: 25}}, nil
	}
	store.updateCombatantInitiativeFn = func(_ context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, EncounterID: encounterID, InitiativeRoll: arg.InitiativeRoll, InitiativeOrder: arg.InitiativeOrder, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, IsAlive: true, Conditions: json.RawMessage(`[]`), DisplayName: "Aragorn"}, nil
	}

	_, r := newTestCombatRouter(store)

	body := map[string]any{
		"template_id":   templateID.String(),
		"character_ids": []string{charID.String()},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/combat/start", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp startCombatResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var found bool
	for _, c := range resp.Combatants {
		if c.InitiativeRoll == 25 {
			found = true
		}
	}
	assert.True(t, found, "the player-staged initiative total (25) is used verbatim")
	assert.NotEqual(t, uuid.Nil, clearedCampaign, "staged rows are cleared after combat starts")
}
