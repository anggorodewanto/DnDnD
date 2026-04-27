package combat_test

import (
	"context"
	"database/sql"
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

// TestIntegration_OverridePosition_IntoSilenceZone_BreaksConcentration is the
// Phase 118b done-when (1) integration test: a DM forward correction that moves
// a concentrating caster (V/S spell) into an existing Silence zone must trigger
// the silence-zone concentration-break hook, just like a player-initiated move
// does. Before Phase 118b, OverrideCombatantPosition wrote directly to the
// store and bypassed the hook in Service.UpdateCombatantPosition.
func TestIntegration_OverridePosition_IntoSilenceZone_BreaksConcentration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	queries := refdata.New(td.DB)
	ctx := context.Background()

	// Insert a minimal Hold Person spell with V/S components so the
	// silence-zone check has component metadata to read.
	_, err := td.DB.Exec(`INSERT INTO spells (id, name, school, level, casting_time, range_type, components, duration, description, classes)
		VALUES ('hold-person', 'Hold Person', 'enchantment', 2, '1 action', 'ranged', '{V,S,M}', '1 minute', '', '{wizard}')
		ON CONFLICT (id) DO NOTHING`)
	require.NoError(t, err)

	// Caster (Aragorn @ A,1) is concentrating on Hold Person.
	_, err = td.DB.Exec(`UPDATE combatants SET concentration_spell_id='hold-person', concentration_spell_name='Hold Person' WHERE id=$1`,
		td.CombatantID)
	require.NoError(t, err)

	// Drop a Silence zone over tile B,1 (the override target tile).
	_, err = queries.CreateEncounterZone(ctx, refdata.CreateEncounterZoneParams{
		EncounterID:           td.EncounterID,
		SourceCombatantID:     td.CombatantID,
		SourceSpell:           "silence",
		Shape:                 "square",
		OriginCol:             "B",
		OriginRow:             1,
		Dimensions:            json.RawMessage(`{"side_ft":5}`),
		AnchorMode:            "fixed",
		ZoneType:              "silence",
		OverlayColor:          "#888888",
		RequiresConcentration: false,
	})
	require.NoError(t, err)

	poster := &recordingPoster{}
	r := newDMDashboardRouterForIntegration(t, td, poster)

	// DM forward correction: move Aragorn from A,1 to B,1 (into the silence zone).
	body := `{"position_col":"B","position_row":1,"altitude_ft":0,"reason":"misplaced token"}`
	req := httptest.NewRequest("POST",
		"/api/combat/"+td.EncounterID.String()+"/override/combatant/"+td.CombatantID.String()+"/position",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Position updated.
	c, err := queries.GetCombatant(ctx, td.CombatantID)
	require.NoError(t, err)
	assert.Equal(t, "B", c.PositionCol)
	assert.Equal(t, int32(1), c.PositionRow)

	// Phase 118b: concentration must have been broken by the silence-zone
	// hook firing through Service.UpdateCombatantPosition. Read the columns
	// directly from the DB since refdata.Combatant doesn't surface them.
	var spellID, spellName sql.NullString
	require.NoError(t, td.DB.QueryRow(
		`SELECT concentration_spell_id, concentration_spell_name FROM combatants WHERE id=$1`,
		td.CombatantID,
	).Scan(&spellID, &spellName))
	assert.False(t, spellID.Valid, "DM-overridden move into silence must clear concentration_spell_id")
	assert.False(t, spellName.Valid, "DM-overridden move into silence must clear concentration_spell_name")
}
