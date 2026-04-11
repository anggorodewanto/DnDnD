package combat_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// recordingPoster records DM correction calls for assertions in integration tests.
type recordingPoster struct {
	mu    sync.Mutex
	calls []string
}

func (p *recordingPoster) PostCorrection(ctx context.Context, encounterID uuid.UUID, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, message)
}

// newDMDashboardRouterForIntegration wires a real-DB-backed handler with the lock and a recording poster.
func newDMDashboardRouterForIntegration(t *testing.T, td turnLockTestData, poster combat.CombatLogPoster) http.Handler {
	t.Helper()
	queries := refdata.New(td.DB)
	svc := combat.NewService(combat.NewStoreAdapter(queries))
	handler := combat.NewDMDashboardHandlerWithDeps(svc, td.DB, poster)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	return r
}

// TestIntegration_OverrideHP_UsesTurnLock verifies that the HP override endpoint
// completes successfully against a real DB (acquiring and releasing the per-turn lock).
func TestIntegration_OverrideHP_UsesTurnLock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	poster := &recordingPoster{}
	r := newDMDashboardRouterForIntegration(t, td, poster)

	body := `{"hp_current":30,"temp_hp":5,"reason":"healing potion missed"}`
	req := httptest.NewRequest("POST",
		"/api/combat/"+td.EncounterID.String()+"/override/combatant/"+td.CombatantID.String()+"/hp",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify HP was updated in DB
	queries := refdata.New(td.DB)
	c, err := queries.GetCombatant(context.Background(), td.CombatantID)
	require.NoError(t, err)
	assert.Equal(t, int32(30), c.HpCurrent)
	assert.Equal(t, int32(5), c.TempHp)

	// Verify action_log row created
	logs, err := queries.ListActionLogByTurnID(context.Background(), td.TurnID)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "dm_override", logs[0].ActionType)
	assert.Equal(t, "healing potion missed", logs[0].Description.String)

	// Verify Discord poster was called
	assert.Len(t, poster.calls, 1)
	assert.Contains(t, poster.calls[0], "DM Correction")
}

// TestIntegration_UndoLastAction_UsesTurnLock verifies that the undo endpoint
// successfully restores HP from a previous action_log entry's before_state.
func TestIntegration_UndoLastAction_UsesTurnLock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	queries := refdata.New(td.DB)
	ctx := context.Background()

	// Damage the combatant to HP=20 and write an action_log entry with before_state HP=45
	_, err := queries.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
		ID: td.CombatantID, HpCurrent: 20, TempHp: 0, IsAlive: true,
	})
	require.NoError(t, err)

	beforeState := json.RawMessage(`{"hp_current":45,"temp_hp":0,"is_alive":true}`)
	_, err = queries.CreateActionLog(ctx, refdata.CreateActionLogParams{
		TurnID:      td.TurnID,
		EncounterID: td.EncounterID,
		ActionType:  "damage",
		ActorID:     td.CombatantID,
		BeforeState: beforeState,
		AfterState:  json.RawMessage(`{"hp_current":20}`),
	})
	require.NoError(t, err)

	poster := &recordingPoster{}
	r := newDMDashboardRouterForIntegration(t, td, poster)

	req := httptest.NewRequest("POST",
		"/api/combat/"+td.EncounterID.String()+"/undo-last-action",
		strings.NewReader(`{"reason":"missed resistance"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify HP restored to 45
	c, err := queries.GetCombatant(ctx, td.CombatantID)
	require.NoError(t, err)
	assert.Equal(t, int32(45), c.HpCurrent)

	// Verify dm_override_undo log row exists
	logs, err := queries.ListActionLogByTurnID(ctx, td.TurnID)
	require.NoError(t, err)
	require.Len(t, logs, 2)
	assert.Equal(t, "dm_override_undo", logs[1].ActionType)

	// Discord poster called
	assert.Len(t, poster.calls, 1)
	assert.Contains(t, poster.calls[0], "DM Correction")
	assert.Contains(t, poster.calls[0], "Aragorn")
}
