package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// newPendingSaveRouter wires a DM dashboard router with a fake combat-log poster
// and (optionally) a deterministic roller so the resolve endpoint is testable.
func newPendingSaveRouter(store Store, poster CombatLogPoster, roller *dice.Roller) http.Handler {
	svc := NewService(store)
	if roller != nil {
		svc.SetRoller(roller)
	}
	handler := NewDMDashboardHandlerWithDeps(svc, nil, poster)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	return r
}

func monsterSaveStore(encounterID, saveID, combatantID uuid.UUID, source string) *mockStore {
	comb := refdata.Combatant{
		ID: combatantID, EncounterID: encounterID, DisplayName: "Goblin",
		CreatureRefID: sql.NullString{String: "goblin", Valid: true},
		IsNpc:         true, IsAlive: true, HpMax: 30, HpCurrent: 30,
		Conditions: json.RawMessage(`[]`),
	}
	store := defaultMockStore()
	store.getPendingSaveFn = func(_ context.Context, id uuid.UUID) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: id, EncounterID: encounterID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: source, Status: "pending"}, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return comb, nil }
	store.getCreatureFn = func(_ context.Context, _ string) (refdata.Creature, error) {
		return refdata.Creature{AbilityScores: json.RawMessage(`{"dex":10}`)}, nil
	}
	store.listPendingSavesByCombatantFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{{ID: saveID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: source, Status: "pending"}}, nil
	}
	store.listSavesByEncounterFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{{ID: saveID, EncounterID: encounterID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: source, Status: "rolled", Success: sql.NullBool{Bool: false, Valid: true}}}, nil
	}
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return newDamagingFireball(), nil }
	store.updatePendingSaveResultFn = func(_ context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: arg.ID, Source: source, Status: "rolled", RollResult: arg.RollResult, Success: arg.Success}, nil
	}
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}
	return store
}

func TestHandlerResolveMonsterPendingSave_HappyPath(t *testing.T) {
	encounterID := uuid.New()
	saveID := uuid.New()
	combatantID := uuid.New()
	source := AoEPendingSaveSource("fireball")

	store := monsterSaveStore(encounterID, saveID, combatantID, source)
	poster := &fakeCombatLogPoster{}
	r := newPendingSaveRouter(store, poster, rollerFor(4, 4)) // d20=4 → fail; 8d6 of 4 = 32

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-saves/"+saveID.String()+"/resolve", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	assert.Equal(t, "Goblin", resp["combatant_name"])
	assert.Equal(t, "dex", resp["ability"])

	calls := poster.Calls()
	require.Len(t, calls, 1, "a #combat-log line must be posted")
	msg := calls[0].Message
	assert.Contains(t, msg, "save")
	assert.Contains(t, msg, "32 damage", "damage dealt is shown")
	assert.NotContains(t, strings.ToLower(msg), "hp", "enemy HP must never be revealed")
	assert.NotContains(t, strings.ToLower(msg), "remaining")
}

func TestHandlerResolveMonsterPendingSave_PlayerSave409(t *testing.T) {
	encounterID := uuid.New()
	saveID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getPendingSaveFn = func(_ context.Context, id uuid.UUID) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: id, EncounterID: encounterID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: AoEPendingSaveSource("fireball"), Status: "pending"}, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Aria", CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true}, Conditions: json.RawMessage(`[]`)}, nil
	}
	r := newPendingSaveRouter(store, nil, nil)

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-saves/"+saveID.String()+"/resolve", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, strings.ToLower(w.Body.String()), "/save")
}

func TestHandlerResolveMonsterPendingSave_AlreadyResolved409(t *testing.T) {
	encounterID := uuid.New()
	saveID := uuid.New()

	store := defaultMockStore()
	// ISSUE-044: 'applied' is the terminal 409 state. ('rolled' now recovers.)
	store.getPendingSaveFn = func(_ context.Context, id uuid.UUID) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: id, EncounterID: encounterID, CombatantID: uuid.New(), Ability: "dex", Dc: 15, Source: AoEPendingSaveSource("fireball"), Status: "applied"}, nil
	}
	r := newPendingSaveRouter(store, nil, nil)

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-saves/"+saveID.String()+"/resolve", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestHandlerResolveMonsterPendingSave_Missing404(t *testing.T) {
	encounterID := uuid.New()
	saveID := uuid.New()

	store := defaultMockStore()
	store.getPendingSaveFn = func(_ context.Context, _ uuid.UUID) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{}, sql.ErrNoRows
	}
	r := newPendingSaveRouter(store, nil, nil)

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-saves/"+saveID.String()+"/resolve", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlerResolveMonsterPendingSave_WrongEncounter400(t *testing.T) {
	encounterID := uuid.New()
	saveID := uuid.New()

	store := defaultMockStore()
	store.getPendingSaveFn = func(_ context.Context, id uuid.UUID) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: id, EncounterID: uuid.New(), CombatantID: uuid.New(), Ability: "dex", Dc: 15, Source: AoEPendingSaveSource("fireball"), Status: "pending"}, nil
	}
	r := newPendingSaveRouter(store, nil, nil)

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-saves/"+saveID.String()+"/resolve", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlerListPendingSaves(t *testing.T) {
	encounterID := uuid.New()
	saveID := uuid.New()
	combatantID := uuid.New()
	source := AoEPendingSaveSource("fireball")

	store := defaultMockStore()
	store.listPendingSavesByEncounterFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{
			{ID: saveID, EncounterID: encounterID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: source, Status: "pending", CoverBonus: 2},
			// non-AoE pending row must be filtered out
			{ID: uuid.New(), EncounterID: encounterID, CombatantID: uuid.New(), Ability: "con", Source: ConcentrationSaveSource, Status: "pending"},
		}, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`)}, nil
	}
	r := newPendingSaveRouter(store, nil, nil)

	req := httptest.NewRequest("GET", "/api/combat/"+encounterID.String()+"/pending-saves", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rows))
	require.Len(t, rows, 1, "only the pending AoE save is listed")
	assert.Equal(t, saveID.String(), rows[0]["id"])
	assert.Equal(t, "Goblin", rows[0]["combatant_name"])
	assert.Equal(t, "dex", rows[0]["ability"])
	assert.Equal(t, float64(15), rows[0]["dc"])
	assert.Equal(t, "fireball", rows[0]["spell_id"])
	assert.Equal(t, float64(2), rows[0]["cover_bonus"])
}

func TestHandlerResolveMonsterPendingSave_SuccessHalvedLog(t *testing.T) {
	encounterID := uuid.New()
	saveID := uuid.New()
	combatantID := uuid.New()
	source := AoEPendingSaveSource("fireball")

	store := monsterSaveStore(encounterID, saveID, combatantID, source)
	// Override the encounter-level (all-status) list to report a successful save
	// so damage is halved on apply.
	store.listSavesByEncounterFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{{ID: saveID, EncounterID: encounterID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: source, Status: "rolled", Success: sql.NullBool{Bool: true, Valid: true}}}, nil
	}
	poster := &fakeCombatLogPoster{}
	r := newPendingSaveRouter(store, poster, rollerFor(20, 4)) // d20=20 success; 8d6 of 4 = 32, halved 16

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-saves/"+saveID.String()+"/resolve", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	calls := poster.Calls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Message, "Success")
	assert.Contains(t, calls[0].Message, "16 damage")
	assert.Contains(t, calls[0].Message, "halved on save")
	assert.NotContains(t, strings.ToLower(calls[0].Message), "hp")
}

// COV-2: resolving a monster's FAILED save against a condition-only spell
// (Hold Person) both persists the condition and surfaces it in the #combat-log
// line — the DM-facing proof that a save-or-suck condition actually landed.
func TestHandlerResolveMonsterPendingSave_ConditionLandsOnFail(t *testing.T) {
	encounterID := uuid.New()
	saveID := uuid.New()
	combatantID := uuid.New()
	source := AoEPendingSaveSource("hold-person")

	store := monsterSaveStore(encounterID, saveID, combatantID, source)
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeHoldPerson(), nil }
	// casterConcentratingOn scans the encounter; nobody concentrates here, so
	// the condition applies un-scoped but still lands on the failed target.
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{}, nil
	}
	conditionPersisted := false
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		conditionPersisted = true
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	poster := &fakeCombatLogPoster{}
	r := newPendingSaveRouter(store, poster, rollerFor(3, 4)) // d20=3 → fail vs DC 15

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-saves/"+saveID.String()+"/resolve", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.True(t, conditionPersisted, "failed save must persist the condition")
	calls := poster.Calls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Message, "paralyzed", "combat log surfaces the landed condition")
}

func TestHandlerResolveMonsterPendingSave_NotAoE400(t *testing.T) {
	encounterID := uuid.New()
	saveID := uuid.New()

	store := defaultMockStore()
	store.getPendingSaveFn = func(_ context.Context, id uuid.UUID) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: id, EncounterID: encounterID, CombatantID: uuid.New(), Ability: "con", Dc: 12, Source: ConcentrationSaveSource, Status: "pending"}, nil
	}
	r := newPendingSaveRouter(store, nil, nil)

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-saves/"+saveID.String()+"/resolve", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlerResolveMonsterPendingSave_InvalidSaveID400(t *testing.T) {
	encounterID := uuid.New()
	r := newPendingSaveRouter(defaultMockStore(), nil, nil)

	req := httptest.NewRequest("POST", "/api/combat/"+encounterID.String()+"/pending-saves/not-a-uuid/resolve", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlerListPendingSaves_InvalidEncounter400(t *testing.T) {
	r := newPendingSaveRouter(defaultMockStore(), nil, nil)

	req := httptest.NewRequest("GET", "/api/combat/not-a-uuid/pending-saves", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
