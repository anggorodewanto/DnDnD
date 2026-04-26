package combat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// nu is a tiny helper that wraps a uuid.UUID as a Valid uuid.NullUUID. It
// keeps the test row construction readable now that action_log columns are
// nullable post-Phase 118c.
func nu(id uuid.UUID) uuid.NullUUID { return uuid.NullUUID{UUID: id, Valid: true} }

func buildActionLogStore(rows []refdata.ListActionLogWithRoundsRow, combatants []refdata.Combatant) *mockStore {
	store := defaultMockStore()
	store.listActionLogWithRoundsFn = func(ctx context.Context, eid uuid.NullUUID) ([]refdata.ListActionLogWithRoundsRow, error) {
		return rows, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return combatants, nil
	}
	return store
}

// --- TDD Cycle 4: GET /api/combat/{encounterID}/action-log ---

func TestHandler_ListActionLogViewer_Success(t *testing.T) {
	encounterID := uuid.New()
	actorA := uuid.New()
	t1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 1, 10, 1, 0, 0, time.UTC)

	rows := []refdata.ListActionLogWithRoundsRow{
		{
			ID: uuid.New(), TurnID: nu(uuid.New()), EncounterID: nu(encounterID),
			ActionType: "attack", ActorID: nu(actorA),
			BeforeState: json.RawMessage(`{}`),
			AfterState:  json.RawMessage(`{}`),
			RoundNumber: 1, CreatedAt: t1,
		},
		{
			ID: uuid.New(), TurnID: nu(uuid.New()), EncounterID: nu(encounterID),
			ActionType: "dm_override", ActorID: nu(actorA),
			BeforeState: json.RawMessage(`{}`),
			AfterState:  json.RawMessage(`{}`),
			RoundNumber: 2, CreatedAt: t2,
		},
	}
	combatants := []refdata.Combatant{
		{ID: actorA, ShortID: "AR", DisplayName: "Aragorn"},
	}

	store := buildActionLogStore(rows, combatants)
	r := newDMDashboardRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/action-log", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []ActionLogViewerEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 2)
	// Default newest-first
	assert.Equal(t, "dm_override", resp[0].ActionType)
	assert.True(t, resp[0].IsOverride)
	assert.Equal(t, "Aragorn", resp[0].ActorDisplayName)
}

func TestHandler_ListActionLogViewer_InvalidEncounterID(t *testing.T) {
	r := newDMDashboardRouter(&mockStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/combat/not-a-uuid/action-log", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ListActionLogViewer_FilterByType(t *testing.T) {
	encounterID := uuid.New()
	actorA := uuid.New()
	rows := []refdata.ListActionLogWithRoundsRow{
		{ID: uuid.New(), ActionType: "attack", ActorID: nu(actorA), BeforeState: json.RawMessage(`{}`), AfterState: json.RawMessage(`{}`), RoundNumber: 1, CreatedAt: time.Now()},
		{ID: uuid.New(), ActionType: "dm_override", ActorID: nu(actorA), BeforeState: json.RawMessage(`{}`), AfterState: json.RawMessage(`{}`), RoundNumber: 1, CreatedAt: time.Now().Add(time.Second)},
		{ID: uuid.New(), ActionType: "cast", ActorID: nu(actorA), BeforeState: json.RawMessage(`{}`), AfterState: json.RawMessage(`{}`), RoundNumber: 1, CreatedAt: time.Now().Add(2 * time.Second)},
	}
	store := buildActionLogStore(rows, []refdata.Combatant{{ID: actorA, ShortID: "AR", DisplayName: "Aragorn"}})
	r := newDMDashboardRouter(store)

	// Comma-separated
	req := httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/action-log?action_type=attack,dm_override", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []ActionLogViewerEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 2)

	// Repeated query params
	req = httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/action-log?action_type=attack&action_type=cast", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 2)
}

func TestHandler_ListActionLogViewer_FilterByActorRoundTurnSort(t *testing.T) {
	encounterID := uuid.New()
	actorA := uuid.New()
	actorB := uuid.New()
	turnX := uuid.New()
	t1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 1, 10, 1, 0, 0, time.UTC)
	rows := []refdata.ListActionLogWithRoundsRow{
		{ID: uuid.New(), TurnID: nu(turnX), ActionType: "attack", ActorID: nu(actorA), BeforeState: json.RawMessage(`{}`), AfterState: json.RawMessage(`{}`), RoundNumber: 1, CreatedAt: t1},
		{ID: uuid.New(), TurnID: nu(uuid.New()), ActionType: "attack", ActorID: nu(actorB), BeforeState: json.RawMessage(`{}`), AfterState: json.RawMessage(`{}`), RoundNumber: 2, CreatedAt: t2},
	}
	store := buildActionLogStore(rows, []refdata.Combatant{
		{ID: actorA, DisplayName: "A"}, {ID: actorB, DisplayName: "B"},
	})
	r := newDMDashboardRouter(store)

	// Filter by actor_id
	req := httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/action-log?actor_id="+actorA.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []ActionLogViewerEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, actorA, resp[0].ActorID)

	// Filter by round
	req = httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/action-log?round=2", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, actorB, resp[0].ActorID)

	// Filter by turn_id
	req = httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/action-log?turn_id="+turnX.String(), nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, actorA, resp[0].ActorID)

	// Sort asc
	req = httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/action-log?sort=asc", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 2)
	assert.Equal(t, actorA, resp[0].ActorID) // oldest first

	// Sort desc default
	req = httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/action-log?sort=desc", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 2)
	assert.Equal(t, actorB, resp[0].ActorID) // newest first
}

func TestHandler_ListActionLogViewer_FilterByTarget(t *testing.T) {
	encounterID := uuid.New()
	actorA := uuid.New()
	targetA := uuid.New()
	rows := []refdata.ListActionLogWithRoundsRow{
		{ID: uuid.New(), ActionType: "attack", ActorID: nu(actorA), TargetID: uuid.NullUUID{UUID: targetA, Valid: true}, BeforeState: json.RawMessage(`{}`), AfterState: json.RawMessage(`{}`), RoundNumber: 1, CreatedAt: time.Now()},
		{ID: uuid.New(), ActionType: "cast", ActorID: nu(actorA), BeforeState: json.RawMessage(`{}`), AfterState: json.RawMessage(`{}`), RoundNumber: 1, CreatedAt: time.Now().Add(time.Second)},
	}
	store := buildActionLogStore(rows, []refdata.Combatant{
		{ID: actorA, DisplayName: "A"}, {ID: targetA, DisplayName: "T"},
	})
	r := newDMDashboardRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/action-log?target_id="+targetA.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []ActionLogViewerEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "T", resp[0].TargetDisplayName)
}

func TestHandler_ListActionLogViewer_InvalidQueryParams(t *testing.T) {
	encounterID := uuid.New()
	store := buildActionLogStore(nil, nil)
	r := newDMDashboardRouter(store)

	// invalid actor_id
	req := httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/action-log?actor_id=not-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// invalid target_id
	req = httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/action-log?target_id=bad", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// invalid turn_id
	req = httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/action-log?turn_id=bad", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// invalid round
	req = httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/action-log?round=abc", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ListActionLogViewer_EmptyCommaActionType(t *testing.T) {
	encounterID := uuid.New()
	actorA := uuid.New()
	rows := []refdata.ListActionLogWithRoundsRow{
		{ID: uuid.New(), ActionType: "attack", ActorID: nu(actorA), BeforeState: json.RawMessage(`{}`), AfterState: json.RawMessage(`{}`), RoundNumber: 1, CreatedAt: time.Now()},
	}
	store := buildActionLogStore(rows, []refdata.Combatant{{ID: actorA, DisplayName: "A"}})
	r := newDMDashboardRouter(store)

	// "attack,," — has an empty segment that should be skipped
	req := httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/action-log?action_type=attack,", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []ActionLogViewerEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
}

func TestHandler_ListActionLogViewer_ServiceError(t *testing.T) {
	encounterID := uuid.New()
	store := defaultMockStore()
	store.listActionLogWithRoundsFn = func(ctx context.Context, eid uuid.NullUUID) ([]refdata.ListActionLogWithRoundsRow, error) {
		return nil, assert.AnError
	}
	r := newDMDashboardRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/action-log", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
