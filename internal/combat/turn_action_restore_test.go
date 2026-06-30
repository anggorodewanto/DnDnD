package combat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// restoreActionStore wires a defaultMockStore for the active turn / combatant /
// character lookups RestoreTurnAction needs, capturing the persisted turn params.
func restoreActionStore(turn refdata.Turn, combatant refdata.Combatant) (*mockStore, *refdata.UpdateTurnActionsParams) {
	store := defaultMockStore()
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encounterID uuid.UUID) (refdata.Turn, error) {
		return turn, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return combatant, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{ID: id, SpeedFt: 30, Classes: json.RawMessage(`[]`)}, nil
	}
	captured := &refdata.UpdateTurnActionsParams{}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		*captured = arg
		return refdata.Turn{ID: arg.ID}, nil
	}
	return store, captured
}

func TestRestoreTurnAction_GivesBackActionAndAttacks(t *testing.T) {
	encID, turnID, combID, charID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	turn := refdata.Turn{
		ID:                  turnID,
		EncounterID:         encID,
		CombatantID:         combID,
		Status:              "active",
		MovementRemainingFt: 30,
		ActionUsed:          true,
		ActionSpellCast:     true,
		AttacksRemaining:    0,
	}
	combatant := refdata.Combatant{
		ID:          combID,
		DisplayName: "Vale",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		Conditions:  json.RawMessage(`[]`),
	}
	store, captured := restoreActionStore(turn, combatant)

	res, err := NewService(store).RestoreTurnAction(context.Background(), encID, combID)

	require.NoError(t, err)
	assert.Equal(t, combID, res.CombatantID)
	assert.Equal(t, "Vale", res.CombatantName)
	// Action handed back: both spent flags cleared, attacks reseeded to 1.
	assert.False(t, captured.ActionUsed, "action_used should be cleared")
	assert.False(t, captured.ActionSpellCast, "action_spell_cast should be cleared")
	assert.Equal(t, int32(1), captured.AttacksRemaining, "attacks reseeded for the restored action")
	// Movement is untouched — restoring the action does not refund movement.
	assert.Equal(t, int32(30), captured.MovementRemainingFt)
}

func TestRestoreTurnAction_NotActiveCombatantRejected(t *testing.T) {
	encID, combID := uuid.New(), uuid.New()
	turn := refdata.Turn{
		ID:          uuid.New(),
		EncounterID: encID,
		CombatantID: uuid.New(), // someone else is active
		Status:      "active",
		ActionUsed:  true,
	}
	store, _ := restoreActionStore(turn, refdata.Combatant{ID: combID, Conditions: json.RawMessage(`[]`)})

	_, err := NewService(store).RestoreTurnAction(context.Background(), encID, combID)

	assert.ErrorIs(t, err, ErrNotActiveCombatant)
}

func TestRestoreTurnAction_NoActionToRestoreRejected(t *testing.T) {
	encID, combID := uuid.New(), uuid.New()
	turn := refdata.Turn{
		ID:          uuid.New(),
		EncounterID: encID,
		CombatantID: combID,
		Status:      "active",
		ActionUsed:  false, // nothing spent
	}
	store, _ := restoreActionStore(turn, refdata.Combatant{ID: combID, Conditions: json.RawMessage(`[]`)})

	_, err := NewService(store).RestoreTurnAction(context.Background(), encID, combID)

	assert.ErrorIs(t, err, ErrNoActionToRestore)
}

func TestHandlerRestoreTurnAction_HappyPath(t *testing.T) {
	encID, combID, charID := uuid.New(), uuid.New(), uuid.New()
	turn := refdata.Turn{
		ID: uuid.New(), EncounterID: encID, CombatantID: combID, Status: "active",
		MovementRemainingFt: 30, ActionUsed: true, ActionSpellCast: true, AttacksRemaining: 0,
	}
	combatant := refdata.Combatant{
		ID: combID, DisplayName: "Vale",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		Conditions:  json.RawMessage(`[]`),
	}
	store, captured := restoreActionStore(turn, combatant)
	poster := &fakeCombatLogPoster{}
	r := newPendingSaveRouter(store, poster, nil)

	req := httptest.NewRequest("POST",
		"/api/combat/"+encID.String()+"/combatants/"+combID.String()+"/restore-action", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Vale", resp["combatant_name"])
	assert.Equal(t, true, resp["restored"])
	assert.False(t, captured.ActionUsed, "action_used persisted as cleared")

	calls := poster.Calls()
	require.Len(t, calls, 1, "a #combat-log correction must be posted")
	msg := strings.ToLower(calls[0].Message)
	assert.Contains(t, msg, "action restored")
	assert.NotContains(t, msg, "hp", "HP must never be revealed")
}

func TestHandlerRestoreTurnAction_NotActiveCombatant409(t *testing.T) {
	encID, combID := uuid.New(), uuid.New()
	turn := refdata.Turn{
		ID: uuid.New(), EncounterID: encID, CombatantID: uuid.New(), Status: "active", ActionUsed: true,
	}
	store, _ := restoreActionStore(turn, refdata.Combatant{ID: combID, Conditions: json.RawMessage(`[]`)})
	r := newPendingSaveRouter(store, &fakeCombatLogPoster{}, nil)

	req := httptest.NewRequest("POST",
		"/api/combat/"+encID.String()+"/combatants/"+combID.String()+"/restore-action", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}
