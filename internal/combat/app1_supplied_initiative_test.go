package combat

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// APP-1: player-authoritative initiative at combat start.

func i32ptr(v int32) *int32 { return &v }

func TestAssignInitiativeOrder_DerivesFromRolls(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	entries := []InitiativeEntry{
		{CombatantID: a, DisplayName: "A", Roll: 10, DexMod: 1},
		{CombatantID: b, DisplayName: "B", Roll: 19, DexMod: 2},
		{CombatantID: c, DisplayName: "C", Roll: 14, DexMod: 0},
	}
	got, err := AssignInitiativeOrder(entries)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, b, got[0].CombatantID, "19 seats first")
	assert.Equal(t, c, got[1].CombatantID, "14 seats second")
	assert.Equal(t, a, got[2].CombatantID, "10 seats third")
}

func TestAssignInitiativeOrder_TiebreakByDex(t *testing.T) {
	// Windreth (DEX 18) and Vale (DEX 10) both total 14 → Windreth first.
	wind, vale := uuid.New(), uuid.New()
	entries := []InitiativeEntry{
		{CombatantID: vale, DisplayName: "Vale", Roll: 14, DexMod: 0},
		{CombatantID: wind, DisplayName: "Windreth", Roll: 14, DexMod: 4},
	}
	got, err := AssignInitiativeOrder(entries)
	require.NoError(t, err)
	assert.Equal(t, wind, got[0].CombatantID)
	assert.Equal(t, vale, got[1].CombatantID)
}

func TestAssignInitiativeOrder_HonorsExplicitOrder(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	// b has the highest roll but is forced to seat 3; a forced to seat 1;
	// c (no explicit order) fills the remaining seat 2.
	entries := []InitiativeEntry{
		{CombatantID: a, DisplayName: "A", Roll: 10, DexMod: 1, ExplicitOrder: i32ptr(1)},
		{CombatantID: b, DisplayName: "B", Roll: 19, DexMod: 2, ExplicitOrder: i32ptr(3)},
		{CombatantID: c, DisplayName: "C", Roll: 14, DexMod: 0},
	}
	got, err := AssignInitiativeOrder(entries)
	require.NoError(t, err)
	assert.Equal(t, a, got[0].CombatantID)
	assert.Equal(t, c, got[1].CombatantID)
	assert.Equal(t, b, got[2].CombatantID)
}

func TestAssignInitiativeOrder_DuplicateOrderRejected(t *testing.T) {
	entries := []InitiativeEntry{
		{CombatantID: uuid.New(), DisplayName: "A", Roll: 10, ExplicitOrder: i32ptr(1)},
		{CombatantID: uuid.New(), DisplayName: "B", Roll: 12, ExplicitOrder: i32ptr(1)},
	}
	_, err := AssignInitiativeOrder(entries)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInitiativeOrder)
}

func TestAssignInitiativeOrder_OutOfRangeRejected(t *testing.T) {
	entries := []InitiativeEntry{
		{CombatantID: uuid.New(), DisplayName: "A", Roll: 10, ExplicitOrder: i32ptr(5)},
	}
	_, err := AssignInitiativeOrder(entries)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInitiativeOrder)
}

// rollInitiative with a supplied PC value must NOT auto-roll that PC: the stored
// initiative_roll is the supplied total verbatim, while NPCs still auto-roll.
func TestService_RollInitiative_SuppliedInitiativeSkipsAutoRoll(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	charID := uuid.New()

	pc := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Forge",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:       false, Conditions: json.RawMessage(`[]`),
	}
	npc := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Follower",
		CreatureRefID: sql.NullString{String: "follower", Valid: true},
		IsNpc:         true, Conditions: json.RawMessage(`[]`),
	}

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "preparing"}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{pc, npc}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{ID: charID, AbilityScores: json.RawMessage(`{"str":10,"dex":14,"con":12,"int":8,"wis":13,"cha":15}`)}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: "follower", AbilityScores: json.RawMessage(`{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10}`)}, nil
	}
	rolls := map[uuid.UUID]int32{}
	orders := map[uuid.UUID]int32{}
	store.updateCombatantInitiativeFn = func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
		rolls[arg.ID] = arg.InitiativeRoll
		orders[arg.ID] = arg.InitiativeOrder
		return refdata.Combatant{ID: arg.ID, InitiativeRoll: arg.InitiativeRoll, InitiativeOrder: arg.InitiativeOrder}, nil
	}

	// Roller returns 2 → an auto-roll would be tiny. Supplied PC total is 19.
	supplied := map[uuid.UUID]InitiativeInput{charID: {Roll: 19}}
	svc := NewService(store)
	_, err := svc.rollInitiative(ctx, encounterID, newTestRoller(2), supplied)
	require.NoError(t, err)

	assert.Equal(t, int32(19), rolls[pc.ID], "PC keeps its supplied total, not a rolled value")
	assert.Equal(t, int32(2), rolls[npc.ID], "NPC still auto-rolls (2 + DEX 0)")
	assert.Equal(t, int32(1), orders[pc.ID], "PC (19) seats first")
	assert.Equal(t, int32(2), orders[npc.ID], "NPC (2) seats second")
}

// The public RollInitiative wrapper preserves the old no-supplied behaviour.
func TestService_RollInitiative_WrapperAutoRollsAll(t *testing.T) {
	ctx := context.Background()
	encounterID := uuid.New()
	charID := uuid.New()

	pc := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Forge",
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		IsNpc:       false, Conditions: json.RawMessage(`[]`),
	}
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "preparing"}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{pc}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{ID: charID, AbilityScores: json.RawMessage(`{"str":10,"dex":14,"con":12,"int":8,"wis":13,"cha":15}`)}, nil
	}
	var gotRoll int32
	store.updateCombatantInitiativeFn = func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
		gotRoll = arg.InitiativeRoll
		return refdata.Combatant{ID: arg.ID, InitiativeRoll: arg.InitiativeRoll, InitiativeOrder: arg.InitiativeOrder}, nil
	}

	svc := NewService(store)
	_, err := svc.RollInitiative(ctx, encounterID, newTestRoller(10))
	require.NoError(t, err)
	assert.Equal(t, int32(12), gotRoll, "10 + DEX 2 auto-rolled")
}

func TestHandler_StartCombat_WithSuppliedInitiative(t *testing.T) {
	templateID := uuid.New()
	encounterID := uuid.New()
	charID := uuid.New()

	store := startCombatMockStore(templateID, encounterID, charID)
	_, r := newTestCombatRouter(store)

	body := map[string]any{
		"template_id":   templateID.String(),
		"character_ids": []string{charID.String()},
		"character_positions": map[string]any{
			charID.String(): map[string]any{"col": "D", "row": 5},
		},
		"character_initiatives": map[string]any{
			charID.String(): map[string]any{"roll": 19},
		},
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
		if c.InitiativeRoll == 19 {
			found = true
		}
	}
	assert.True(t, found, "the PC's supplied initiative total (19) is used verbatim")
}

func TestHandler_StartCombat_InvalidSuppliedOrder(t *testing.T) {
	templateID := uuid.New()
	encounterID := uuid.New()
	charID := uuid.New()

	store := startCombatMockStore(templateID, encounterID, charID)
	_, r := newTestCombatRouter(store)

	body := map[string]any{
		"template_id":   templateID.String(),
		"character_ids": []string{charID.String()},
		"character_initiatives": map[string]any{
			// order 9 is out of range for a 2-combatant encounter → 400.
			charID.String(): map[string]any{"roll": 19, "order": 9},
		},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/combat/start", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// A duplicate explicit seat is rejected before any DB write (P1): the encounter
// is never created, so a malformed payload leaves no inert rows behind.
func TestHandler_StartCombat_DuplicateSuppliedOrderRejectedEarly(t *testing.T) {
	store := defaultMockStore()
	created := false
	store.createEncounterFn = func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
		created = true
		return refdata.Encounter{ID: uuid.New()}, nil
	}
	_, r := newTestCombatRouter(store)

	a, b := uuid.New(), uuid.New()
	body := map[string]any{
		"template_id":   uuid.New().String(),
		"character_ids": []string{a.String(), b.String()},
		"character_initiatives": map[string]any{
			a.String(): map[string]any{"roll": 15, "order": 1},
			b.String(): map[string]any{"roll": 12, "order": 1},
		},
	}
	bb, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/combat/start", bytes.NewReader(bb))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.False(t, created, "no encounter created for a malformed order payload")
}

func TestHandler_StartCombat_NonPositiveSuppliedOrderRejectedEarly(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())
	a := uuid.New()
	body := map[string]any{
		"template_id":   uuid.New().String(),
		"character_ids": []string{a.String()},
		"character_initiatives": map[string]any{
			a.String(): map[string]any{"roll": 15, "order": 0},
		},
	}
	bb, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/combat/start", bytes.NewReader(bb))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_StartCombat_InvalidInitiativeKey(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	body := map[string]any{
		"template_id":   uuid.New().String(),
		"character_ids": []string{},
		"character_initiatives": map[string]any{
			"not-uuid": map[string]any{"roll": 10},
		},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/combat/start", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
