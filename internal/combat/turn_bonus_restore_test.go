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

func TestRestoreTurnBonusAction_GivesBackBonusActionOnly(t *testing.T) {
	encID, turnID, combID, charID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	turn := refdata.Turn{
		ID:                   turnID,
		EncounterID:          encID,
		CombatantID:          combID,
		Status:               "active",
		MovementRemainingFt:  30,
		ActionUsed:           true, // main action stays spent
		AttacksRemaining:     0,    // attack count untouched
		BonusActionUsed:      true,
		BonusActionSpellCast: true,
	}
	combatant := refdata.Combatant{
		ID:          combID,
		DisplayName: "Vale",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		Conditions:  json.RawMessage(`[]`),
	}
	store, captured := restoreActionStore(turn, combatant)

	res, err := NewService(store).RestoreTurnBonusAction(context.Background(), encID, combID)

	require.NoError(t, err)
	assert.Equal(t, combID, res.CombatantID)
	assert.Equal(t, "Vale", res.CombatantName)
	// Bonus action handed back: both bonus flags cleared.
	assert.False(t, captured.BonusActionUsed, "bonus_action_used should be cleared")
	assert.False(t, captured.BonusActionSpellCast, "bonus_action_spell_cast should be cleared")
	// The main action, attacks and movement are all left untouched.
	assert.True(t, captured.ActionUsed, "action_used must not be touched")
	assert.Equal(t, int32(0), captured.AttacksRemaining, "attacks_remaining must not be touched")
	assert.Equal(t, int32(30), captured.MovementRemainingFt, "movement must not be touched")
}

func TestRestoreTurnBonusAction_NotActiveCombatantRejected(t *testing.T) {
	encID, combID := uuid.New(), uuid.New()
	turn := refdata.Turn{
		ID:              uuid.New(),
		EncounterID:     encID,
		CombatantID:     uuid.New(), // someone else is active
		Status:          "active",
		BonusActionUsed: true,
	}
	store, _ := restoreActionStore(turn, refdata.Combatant{ID: combID, Conditions: json.RawMessage(`[]`)})

	_, err := NewService(store).RestoreTurnBonusAction(context.Background(), encID, combID)

	assert.ErrorIs(t, err, ErrNotActiveCombatant)
}

func TestRestoreTurnBonusAction_NoBonusActionToRestoreRejected(t *testing.T) {
	encID, combID := uuid.New(), uuid.New()
	turn := refdata.Turn{
		ID:              uuid.New(),
		EncounterID:     encID,
		CombatantID:     combID,
		Status:          "active",
		BonusActionUsed: false, // nothing spent
	}
	store, _ := restoreActionStore(turn, refdata.Combatant{ID: combID, Conditions: json.RawMessage(`[]`)})

	_, err := NewService(store).RestoreTurnBonusAction(context.Background(), encID, combID)

	assert.ErrorIs(t, err, ErrNoBonusActionToRestore)
}

func TestHandlerRestoreTurnBonusAction_HappyPath(t *testing.T) {
	encID, combID, charID := uuid.New(), uuid.New(), uuid.New()
	turn := refdata.Turn{
		ID: uuid.New(), EncounterID: encID, CombatantID: combID, Status: "active",
		MovementRemainingFt: 30, ActionUsed: true, BonusActionUsed: true, BonusActionSpellCast: true,
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
		"/api/combat/"+encID.String()+"/combatants/"+combID.String()+"/restore-bonus-action", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Vale", resp["combatant_name"])
	assert.Equal(t, true, resp["restored"])
	assert.False(t, captured.BonusActionUsed, "bonus_action_used persisted as cleared")
	assert.True(t, captured.ActionUsed, "action_used persisted untouched")

	calls := poster.Calls()
	require.Len(t, calls, 1, "a #combat-log correction must be posted")
	msg := strings.ToLower(calls[0].Message)
	assert.Contains(t, msg, "bonus action restored")
	assert.NotContains(t, msg, "hp", "HP must never be revealed")
}

func TestHandlerRestoreTurnBonusAction_NotActiveCombatant409(t *testing.T) {
	encID, combID := uuid.New(), uuid.New()
	turn := refdata.Turn{
		ID: uuid.New(), EncounterID: encID, CombatantID: uuid.New(), Status: "active", BonusActionUsed: true,
	}
	store, _ := restoreActionStore(turn, refdata.Combatant{ID: combID, Conditions: json.RawMessage(`[]`)})
	r := newPendingSaveRouter(store, &fakeCombatLogPoster{}, nil)

	req := httptest.NewRequest("POST",
		"/api/combat/"+encID.String()+"/combatants/"+combID.String()+"/restore-bonus-action", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}

func TestHandlerRestoreTurnBonusAction_NoBonusAction409(t *testing.T) {
	encID, combID := uuid.New(), uuid.New()
	turn := refdata.Turn{
		ID: uuid.New(), EncounterID: encID, CombatantID: combID, Status: "active", BonusActionUsed: false,
	}
	store, _ := restoreActionStore(turn, refdata.Combatant{ID: combID, Conditions: json.RawMessage(`[]`)})
	r := newPendingSaveRouter(store, &fakeCombatLogPoster{}, nil)

	req := httptest.NewRequest("POST",
		"/api/combat/"+encID.String()+"/combatants/"+combID.String()+"/restore-bonus-action", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}
