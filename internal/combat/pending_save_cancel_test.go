package combat

import (
	"context"
	"database/sql"
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

// cancelCastStore wires a mock store holding a single AoE cast (source) that hit
// two combatants plus an unrelated second cast, so a cancel test can assert only
// the clicked cast's saves are forfeited. forfeited captures the voided IDs.
func cancelCastStore(encID, clicked uuid.UUID, rows []refdata.PendingSafe, forfeited *[]uuid.UUID) *mockStore {
	store := defaultMockStore()
	store.getPendingSaveFn = func(_ context.Context, id uuid.UUID) (refdata.PendingSafe, error) {
		for _, r := range rows {
			if r.ID == id {
				return r, nil
			}
		}
		return refdata.PendingSafe{}, sql.ErrNoRows
	}
	store.listSavesByEncounterFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return rows, nil
	}
	store.forfeitPendingSaveFn = func(_ context.Context, id uuid.UUID) (refdata.PendingSafe, error) {
		*forfeited = append(*forfeited, id)
		return refdata.PendingSafe{ID: id, Status: "forfeited"}, nil
	}
	return store
}

func TestCancelAoEPendingSave_VoidsWholeCast(t *testing.T) {
	encID := uuid.New()
	wightSave := uuid.New()
	forgeSave := uuid.New()
	otherCast := uuid.New()
	wightID := uuid.New()
	rows := []refdata.PendingSafe{
		// The Shatter cast that caught two combatants. The Wight's is still
		// pending (DM-resolved); Forge's already rolled (player /save) but
		// undamaged because the cast deferred. Both must be voided.
		{ID: wightSave, EncounterID: encID, CombatantID: wightID, Source: "aoe:shatter:s2c3", Status: "pending"},
		{ID: forgeSave, EncounterID: encID, CombatantID: uuid.New(), Source: "aoe:shatter:s2c3", Status: "rolled"},
		// An unrelated cast — must NOT be touched.
		{ID: otherCast, EncounterID: encID, CombatantID: uuid.New(), Source: "aoe:fireball:s5c5", Status: "pending"},
	}
	var forfeited []uuid.UUID
	store := cancelCastStore(encID, wightSave, rows, &forfeited)

	svc := NewService(store)
	res, err := svc.CancelAoEPendingSave(context.Background(), encID, wightSave)
	require.NoError(t, err)

	assert.Equal(t, 2, res.Canceled, "both saves of the shatter cast are voided")
	assert.Equal(t, "shatter", res.SpellID)
	assert.Equal(t, wightID, res.CombatantID, "clicked save's combatant parents the audit row")
	assert.ElementsMatch(t, []uuid.UUID{wightSave, forgeSave}, forfeited,
		"only the same-source cast is forfeited, never the unrelated fireball")
}

func TestCancelAoEPendingSave_AppliedIsTerminal(t *testing.T) {
	encID := uuid.New()
	saveID := uuid.New()
	rows := []refdata.PendingSafe{
		{ID: saveID, EncounterID: encID, CombatantID: uuid.New(), Source: "aoe:shatter:s2c3", Status: "applied"},
	}
	var forfeited []uuid.UUID
	store := cancelCastStore(encID, saveID, rows, &forfeited)

	svc := NewService(store)
	_, err := svc.CancelAoEPendingSave(context.Background(), encID, saveID)
	require.ErrorIs(t, err, ErrSaveAlreadyResolved, "damage already landed — nothing to cleanly undo")
	assert.Empty(t, forfeited, "no save is forfeited once the cast applied")
}

func TestCancelAoEPendingSave_WrongEncounterRejected(t *testing.T) {
	encID := uuid.New()
	saveID := uuid.New()
	rows := []refdata.PendingSafe{
		{ID: saveID, EncounterID: uuid.New(), CombatantID: uuid.New(), Source: "aoe:shatter:s2c3", Status: "pending"},
	}
	var forfeited []uuid.UUID
	store := cancelCastStore(encID, saveID, rows, &forfeited)

	svc := NewService(store)
	_, err := svc.CancelAoEPendingSave(context.Background(), encID, saveID)
	require.ErrorIs(t, err, ErrSaveWrongEncounter)
}

func TestCancelAoEPendingSave_NonAoERejected(t *testing.T) {
	encID := uuid.New()
	saveID := uuid.New()
	rows := []refdata.PendingSafe{
		{ID: saveID, EncounterID: encID, CombatantID: uuid.New(), Source: "concentration", Status: "pending"},
	}
	var forfeited []uuid.UUID
	store := cancelCastStore(encID, saveID, rows, &forfeited)

	svc := NewService(store)
	_, err := svc.CancelAoEPendingSave(context.Background(), encID, saveID)
	require.ErrorIs(t, err, ErrSaveNotAoE)
}

func TestHandlerCancelAoEPendingSave_HappyPath(t *testing.T) {
	encID := uuid.New()
	saveID := uuid.New()
	rows := []refdata.PendingSafe{
		{ID: saveID, EncounterID: encID, CombatantID: uuid.New(), Source: "aoe:shatter:s2c3", Status: "pending"},
		{ID: uuid.New(), EncounterID: encID, CombatantID: uuid.New(), Source: "aoe:shatter:s2c3", Status: "rolled"},
	}
	var forfeited []uuid.UUID
	store := cancelCastStore(encID, saveID, rows, &forfeited)
	poster := &fakeCombatLogPoster{}
	r := newPendingSaveRouter(store, poster, nil)

	req := httptest.NewRequest("POST", "/api/combat/"+encID.String()+"/pending-saves/"+saveID.String()+"/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "shatter", resp["spell_id"])
	assert.EqualValues(t, 2, resp["canceled"])

	calls := poster.Calls()
	require.Len(t, calls, 1, "a #combat-log correction must be posted")
	msg := strings.ToLower(calls[0].Message)
	assert.Contains(t, msg, "canceled")
	assert.Contains(t, msg, "no damage applied")
	assert.NotContains(t, msg, "hp", "enemy HP must never be revealed")
}

func TestHandlerCancelAoEPendingSave_AlreadyApplied409(t *testing.T) {
	encID := uuid.New()
	saveID := uuid.New()
	rows := []refdata.PendingSafe{
		{ID: saveID, EncounterID: encID, CombatantID: uuid.New(), Source: "aoe:shatter:s2c3", Status: "applied"},
	}
	var forfeited []uuid.UUID
	store := cancelCastStore(encID, saveID, rows, &forfeited)
	r := newPendingSaveRouter(store, &fakeCombatLogPoster{}, nil)

	req := httptest.NewRequest("POST", "/api/combat/"+encID.String()+"/pending-saves/"+saveID.String()+"/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}
