package combat

import (
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

// --- TDD Cycle 8: GET /api/combat/{encounterID}/reactions/panel returns enriched reactions ---

func TestHandler_ListReactionsPanel_Success(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	reactionID := uuid.New()

	store := defaultMockStore()
	store.listReactionDeclarationsByEncounterFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{
				ID:          reactionID,
				EncounterID: encounterID,
				CombatantID: combatantID,
				Description: "Shield if I get hit",
				Status:      "active",
			},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: combatantID, ShortID: "AR", DisplayName: "Aragorn", Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{EncounterID: encounterID, RoundNumber: 1}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{{CombatantID: combatantID, ReactionUsed: false}}, nil
	}

	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/reactions/panel", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []ReactionPanelEntry
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Len(t, resp, 1)
	assert.Equal(t, reactionID.String(), resp[0].ID.String())
	assert.Equal(t, "Aragorn", resp[0].CombatantDisplayName)
	assert.Equal(t, "AR", resp[0].CombatantShortID)
	assert.Equal(t, "active", resp[0].Status)
	assert.False(t, resp[0].ReactionUsedThisRound)
}

func TestHandler_ListReactionsPanel_InvalidEncounterID(t *testing.T) {
	store := defaultMockStore()
	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/invalid/reactions/panel", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ListReactionsPanel_ServiceError(t *testing.T) {
	store := defaultMockStore()
	store.listReactionDeclarationsByEncounterFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
		return nil, assert.AnError
	}

	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/"+uuid.New().String()+"/reactions/panel", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
