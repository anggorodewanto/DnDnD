package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// --- C-35: DM advantage/disadvantage override — service-layer consumption ---

// TestServiceAttack_ConsumesDMAdvantageOverride verifies that when an
// attacker has a persisted "advantage" override (set by the DM dashboard),
// the next service-level Attack rolls with advantage and the override is
// cleared exactly once.
func TestServiceAttack_ConsumesDMAdvantageOverride(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacter(16, 14, 2, "longsword")
	char.ID = charID

	var clearedFor []uuid.UUID
	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeLongsword(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.clearCombatantNextAttackAdvOverrideFn = func(ctx context.Context, id uuid.UUID) error {
		clearedFor = append(clearedFor, id)
		return nil
	}

	svc := NewService(ms)
	callIdx := 0
	roller := dice.NewRoller(func(max int) int {
		callIdx++
		if max == 20 {
			if callIdx == 1 {
				return 6
			}
			return 18
		}
		return 5
	})

	attacker := refdata.Combatant{
		ID:                    attackerID,
		CharacterID:           uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName:           "Aria",
		PositionCol:           "A",
		PositionRow:           1,
		IsAlive:               true,
		IsVisible:             true,
		Conditions:            json.RawMessage(`[]`),
		NextAttackAdvOverride: sql.NullString{String: "advantage", Valid: true},
	}
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin #1",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          13,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.NoError(t, err)
	assert.Equal(t, dice.Advantage, result.RollMode)
	assert.Contains(t, result.AdvantageReasons, "DM override")
	require.Len(t, clearedFor, 1)
	assert.Equal(t, attackerID, clearedFor[0])
}

// TestServiceAttack_ConsumesDMDisadvantageOverride mirrors the advantage
// test for a stored "disadvantage" override.
func TestServiceAttack_ConsumesDMDisadvantageOverride(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacter(16, 14, 2, "longsword")
	char.ID = charID

	var clearedFor []uuid.UUID
	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeLongsword(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.clearCombatantNextAttackAdvOverrideFn = func(ctx context.Context, id uuid.UUID) error {
		clearedFor = append(clearedFor, id)
		return nil
	}

	svc := NewService(ms)
	callIdx := 0
	roller := dice.NewRoller(func(max int) int {
		callIdx++
		if max == 20 {
			if callIdx == 1 {
				return 18
			}
			return 4
		}
		return 5
	})

	attacker := refdata.Combatant{
		ID:                    attackerID,
		CharacterID:           uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName:           "Aria",
		PositionCol:           "A",
		PositionRow:           1,
		IsAlive:               true,
		IsVisible:             true,
		Conditions:            json.RawMessage(`[]`),
		NextAttackAdvOverride: sql.NullString{String: "disadvantage", Valid: true},
	}
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin #1",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          13,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.NoError(t, err)
	assert.Equal(t, dice.Disadvantage, result.RollMode)
	assert.Contains(t, result.DisadvantageReasons, "DM override")
	require.Len(t, clearedFor, 1)
	assert.Equal(t, attackerID, clearedFor[0])
}

// TestServiceAttack_NoDMOverride_NoClear ensures the clear path is NOT
// invoked when the combatant has no stored override — saves a DB round-trip
// on the common case.
func TestServiceAttack_NoDMOverride_NoClear(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacter(16, 14, 2, "longsword")
	char.ID = charID

	cleared := false
	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeLongsword(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.clearCombatantNextAttackAdvOverrideFn = func(ctx context.Context, id uuid.UUID) error {
		cleared = true
		return nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 12
		}
		return 5
	})

	attacker := refdata.Combatant{
		ID:          attackerID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Aria",
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin #1",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          13,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}

	_, err := svc.Attack(ctx, AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.NoError(t, err)
	assert.False(t, cleared, "clear should not be called when no override is set")
}

// --- C-35: DM advantage/disadvantage override — HTTP endpoint ---

// TestOverrideCombatantNextAttackAdvantage verifies that POST
// /api/combat/{eid}/override/combatant/{cid}/advantage with {"mode":"advantage"}
// persists the override via the store and emits a DM correction message.
func TestOverrideCombatantNextAttackAdvantage(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	var got refdata.SetCombatantNextAttackAdvOverrideParams
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Aria"}, nil
		},
		setCombatantNextAttackAdvOverrideFn: func(ctx context.Context, arg refdata.SetCombatantNextAttackAdvOverrideParams) error {
			got = arg
			return nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	poster := &fakeCombatLogPoster{}
	r := newDMDashboardRouterWithPoster(store, poster)

	body := `{"mode":"advantage","reason":"hero's inspiration"}`
	req := httptest.NewRequest("POST",
		"/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/advantage",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, combatantID, got.ID)
	require.True(t, got.NextAttackAdvOverride.Valid)
	assert.Equal(t, "advantage", got.NextAttackAdvOverride.String)

	calls := poster.Calls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Message, "Aria")
	assert.Contains(t, calls[0].Message, "advantage")
}

// TestOverrideCombatantNextAttackDisadvantage covers the disadvantage mode.
func TestOverrideCombatantNextAttackDisadvantage(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	var got refdata.SetCombatantNextAttackAdvOverrideParams
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Aria"}, nil
		},
		setCombatantNextAttackAdvOverrideFn: func(ctx context.Context, arg refdata.SetCombatantNextAttackAdvOverrideParams) error {
			got = arg
			return nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	body := `{"mode":"disadvantage","reason":"slippery footing"}`
	req := httptest.NewRequest("POST",
		"/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/advantage",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.True(t, got.NextAttackAdvOverride.Valid)
	assert.Equal(t, "disadvantage", got.NextAttackAdvOverride.String)
}

// TestOverrideCombatantNextAttackClear verifies that mode:"none" clears
// any existing override on the targeted combatant.
func TestOverrideCombatantNextAttackClear(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	cleared := false
	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Aria"}, nil
		},
		clearCombatantNextAttackAdvOverrideFn: func(ctx context.Context, id uuid.UUID) error {
			cleared = true
			return nil
		},
		createActionLogFn: func(ctx context.Context, arg refdata.CreateActionLogParams) (refdata.ActionLog, error) {
			return refdata.ActionLog{ID: uuid.New()}, nil
		},
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	body := `{"mode":"none"}`
	req := httptest.NewRequest("POST",
		"/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/advantage",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, cleared)
}

// TestOverrideCombatantNextAttackAdvantage_InvalidMode rejects unknown modes.
func TestOverrideCombatantNextAttackAdvantage_InvalidMode(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id}, nil
		},
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	body := `{"mode":"bogus"}`
	req := httptest.NewRequest("POST",
		"/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/advantage",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestOverrideCombatantNextAttackAdvantage_InvalidIDs covers malformed
// encounter / combatant URL params.
func TestOverrideCombatantNextAttackAdvantage_InvalidIDs(t *testing.T) {
	r := newDMDashboardRouterWithPoster(&mockStore{}, &fakeCombatLogPoster{})

	body := `{"mode":"advantage"}`
	req := httptest.NewRequest("POST",
		"/api/combat/not-a-uuid/override/combatant/"+uuid.New().String()+"/advantage",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestOverrideCombatantNextAttackAdvantage_InvalidBody rejects malformed JSON.
func TestOverrideCombatantNextAttackAdvantage_InvalidBody(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	r := newDMDashboardRouterWithPoster(&mockStore{}, &fakeCombatLogPoster{})

	req := httptest.NewRequest("POST",
		"/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/advantage",
		strings.NewReader(`{"mode":}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestOverrideCombatantNextAttackAdvantage_StoreError surfaces store failures.
func TestOverrideCombatantNextAttackAdvantage_StoreError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, DisplayName: "Aria"}, nil
		},
		setCombatantNextAttackAdvOverrideFn: func(ctx context.Context, arg refdata.SetCombatantNextAttackAdvOverrideParams) error {
			return errors.New("db down")
		},
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	body := `{"mode":"advantage"}`
	req := httptest.NewRequest("POST",
		"/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/advantage",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestOverrideCombatantNextAttackAdvantage_GetCombatantError returns 500
// when looking up the combatant fails.
func TestOverrideCombatantNextAttackAdvantage_GetCombatantError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, EncounterID: encounterID}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("not found")
		},
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	body := `{"mode":"advantage"}`
	req := httptest.NewRequest("POST",
		"/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/advantage",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestServiceAttack_ClearsUnknownDMOverrideValue ensures that a corrupt /
// unknown next_attack_adv_override value is cleared (so it cannot wedge
// future rolls) without granting advantage or disadvantage.
func TestServiceAttack_ClearsUnknownDMOverrideValue(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacter(16, 14, 2, "longsword")
	char.ID = charID

	cleared := false
	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeLongsword(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.clearCombatantNextAttackAdvOverrideFn = func(ctx context.Context, id uuid.UUID) error {
		cleared = true
		return nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 12
		}
		return 5
	})

	attacker := refdata.Combatant{
		ID:                    attackerID,
		CharacterID:           uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName:           "Aria",
		PositionCol:           "A",
		PositionRow:           1,
		IsAlive:               true,
		IsVisible:             true,
		Conditions:            json.RawMessage(`[]`),
		NextAttackAdvOverride: sql.NullString{String: "bogus", Valid: true},
	}
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin #1",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          13,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.NoError(t, err)
	assert.Equal(t, dice.Normal, result.RollMode)
	assert.True(t, cleared, "unknown override value should still trigger clear")
}

// TestServiceAttack_ClearErrorIsBestEffort verifies that a failure of the
// clear-override store call does NOT fail the attack — the override has
// already been applied to the in-memory roll inputs, so the user-visible
// effect is the same. The next attack will, in the worst case, also use
// the override (idempotent) and surface the same clear error.
func TestServiceAttack_ClearErrorIsBestEffort(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()

	char := makeCharacter(16, 14, 2, "longsword")
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeLongsword(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.clearCombatantNextAttackAdvOverrideFn = func(ctx context.Context, id uuid.UUID) error {
		return errors.New("db transient")
	}

	svc := NewService(ms)
	callIdx := 0
	roller := dice.NewRoller(func(max int) int {
		callIdx++
		if max == 20 {
			if callIdx == 1 {
				return 6
			}
			return 16
		}
		return 5
	})

	attacker := refdata.Combatant{
		ID:                    attackerID,
		CharacterID:           uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName:           "Aria",
		PositionCol:           "A",
		PositionRow:           1,
		IsAlive:               true,
		IsVisible:             true,
		Conditions:            json.RawMessage(`[]`),
		NextAttackAdvOverride: sql.NullString{String: "advantage", Valid: true},
	}
	target := refdata.Combatant{
		ID: targetID, DisplayName: "Goblin #1", PositionCol: "B", PositionRow: 1, Ac: 13,
		IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}

	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: attacker,
		Target:   target,
		Turn:     refdata.Turn{ID: turnID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.NoError(t, err)
	assert.Equal(t, dice.Advantage, result.RollMode)
}

// TestSetDMAdvOverride_InvalidMode covers the Service-level guard that
// rejects unknown mode values — defense-in-depth in case a future caller
// bypasses the handler-side validateOverrideMode check.
func TestSetDMAdvOverride_InvalidMode(t *testing.T) {
	svc := NewService(&mockStore{})
	err := svc.setDMAdvOverride(context.Background(), uuid.New(), "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

// TestOverrideCombatantNextAttackAdvantage_NoActiveTurn returns 404 when
// no encounter turn is active.
func TestOverrideCombatantNextAttackAdvantage_NoActiveTurn(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := &mockStore{
		getActiveTurnByEncounterIDFn: func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, errNoActiveTurn{}
		},
	}

	r := newDMDashboardRouterWithPoster(store, &fakeCombatLogPoster{})

	body := `{"mode":"advantage"}`
	req := httptest.NewRequest("POST",
		"/api/combat/"+encounterID.String()+"/override/combatant/"+combatantID.String()+"/advantage",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
