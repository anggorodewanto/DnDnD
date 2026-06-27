package characteroverview

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/rest"
)

// --- Service.ApplyStatus ---

func TestApplyStatus_Success_PersistsAndReturns(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store)

	got, err := svc.ApplyStatus(context.Background(), uuid.New(), nil, StatusUpdate{
		HPMax:           30,
		HPCurrent:       18,
		TempHP:          5,
		ExhaustionLevel: 2,
		Conditions:      []string{"Poisoned", "poisoned", "prone"}, // dup + mixed case
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Normalized: lowercased, deduped, sorted.
	if len(got.Conditions) != 2 || got.Conditions[0] != "poisoned" || got.Conditions[1] != "prone" {
		t.Fatalf("conditions = %+v", got.Conditions)
	}
	if got.HPCurrent != 18 || got.TempHP != 5 || got.ExhaustionLevel != 2 {
		t.Fatalf("status = %+v", got)
	}

	if store.persisted == nil {
		t.Fatal("expected persisted params")
	}
	if store.persisted.HPMax != 30 || store.persisted.HPCurrent != 18 || store.persisted.TempHP != 5 {
		t.Fatalf("persisted hp = %+v", store.persisted)
	}
	// Conditions persisted as the CombatCondition-shaped array.
	var conds []persistedCondition
	if err := json.Unmarshal(store.persisted.Conditions, &conds); err != nil {
		t.Fatalf("conditions not valid JSON: %v (%s)", err, store.persisted.Conditions)
	}
	if len(conds) != 2 || conds[0].Condition != "poisoned" {
		t.Fatalf("persisted conditions = %+v", conds)
	}
	// Exhaustion merged into character_data.
	level, ok := rest.ExhaustionLevelFromCharacterData(store.persisted.CharacterData)
	if !ok || level != 2 {
		t.Fatalf("exhaustion in character_data = %d, ok=%v", level, ok)
	}
}

func TestApplyStatus_EmptyConditions_PersistsEmptyArray(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store)
	got, err := svc.ApplyStatus(context.Background(), uuid.New(), nil, StatusUpdate{HPMax: 10, HPCurrent: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Conditions == nil {
		t.Fatal("expected empty (non-nil) conditions slice")
	}
	if string(store.persisted.Conditions) != "[]" {
		t.Fatalf("expected [] persisted, got %s", store.persisted.Conditions)
	}
}

func TestApplyStatus_MergesExhaustionIntoExistingCharacterData(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store)
	existing := []byte(`{"appearance":"tall","exhaustion_level":5}`)
	_, err := svc.ApplyStatus(context.Background(), uuid.New(), existing, StatusUpdate{HPMax: 10, HPCurrent: 10, ExhaustionLevel: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var data map[string]any
	if err := json.Unmarshal(store.persisted.CharacterData, &data); err != nil {
		t.Fatalf("bad character_data: %v", err)
	}
	if data["appearance"] != "tall" {
		t.Fatalf("expected appearance preserved, got %+v", data)
	}
	if lvl, _ := rest.ExhaustionLevelFromCharacterData(store.persisted.CharacterData); lvl != 1 {
		t.Fatalf("exhaustion = %d, want 1", lvl)
	}
}

func TestApplyStatus_ValidationErrors(t *testing.T) {
	cases := map[string]StatusUpdate{
		"hp_max < 1":          {HPMax: 0, HPCurrent: 0},
		"hp_current > hp_max": {HPMax: 10, HPCurrent: 11},
		"hp_current < 0":      {HPMax: 10, HPCurrent: -1},
		"temp_hp < 0":         {HPMax: 10, HPCurrent: 5, TempHP: -1},
		"exhaustion > 6":      {HPMax: 10, HPCurrent: 5, ExhaustionLevel: 7},
		"exhaustion < 0":      {HPMax: 10, HPCurrent: 5, ExhaustionLevel: -1},
		"unknown condition":   {HPMax: 10, HPCurrent: 5, Conditions: []string{"hangry"}},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			store := &fakeStore{}
			svc := NewService(store)
			_, err := svc.ApplyStatus(context.Background(), uuid.New(), nil, in)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}
			if store.persisted != nil {
				t.Fatal("must not persist on validation failure")
			}
		})
	}
}

func TestApplyStatus_PersistErrorPropagates(t *testing.T) {
	store := &fakeStore{persistErr: errors.New("db down")}
	svc := NewService(store)
	_, err := svc.ApplyStatus(context.Background(), uuid.New(), nil, StatusUpdate{HPMax: 10, HPCurrent: 5})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetStatusContext_RejectsNilID(t *testing.T) {
	svc := NewService(&fakeStore{})
	_, err := svc.GetStatusContext(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetStatusContext_Delegates(t *testing.T) {
	camp := uuid.New()
	store := &fakeStore{statusCtx: CharacterStatusContext{CampaignID: camp, InActiveCombat: true}}
	svc := NewService(store)
	got, err := svc.GetStatusContext(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CampaignID != camp || !got.InActiveCombat {
		t.Fatalf("got %+v", got)
	}
}

// --- Handler.UpdateStatus ---

func postStatus(h *Handler, characterID string, body any, userID string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/character-overview/"+characterID+"/status", bytes.NewReader(buf))
	if userID != "" {
		req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), userID))
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func validStatusBody() statusRequest {
	return statusRequest{HPMax: 20, HPCurrent: 12, TempHP: 0, ExhaustionLevel: 1, Conditions: []string{"poisoned"}}
}

func TestHandler_UpdateStatus_Success(t *testing.T) {
	store := &fakeStore{}
	h := newTestHandler(store)
	rr := postStatus(h, uuid.New().String(), validStatusBody(), "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var resp CharacterStatus
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.HPCurrent != 12 || len(resp.Conditions) != 1 || resp.Conditions[0] != "poisoned" {
		t.Fatalf("resp = %+v", resp)
	}
	if store.persisted == nil {
		t.Fatal("expected persistence")
	}
}

func TestHandler_UpdateStatus_InvalidCharacterID(t *testing.T) {
	h := newTestHandler(&fakeStore{})
	rr := postStatus(h, "not-a-uuid", validStatusBody(), "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_UpdateStatus_BadBody(t *testing.T) {
	h := newTestHandler(&fakeStore{})
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/api/character-overview/"+uuid.New().String()+"/status", bytes.NewReader([]byte("{bad")))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_UpdateStatus_NotFound(t *testing.T) {
	h := newTestHandler(&fakeStore{statusCtxErr: ErrCharacterNotFound})
	rr := postStatus(h, uuid.New().String(), validStatusBody(), "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_UpdateStatus_StoreContextError(t *testing.T) {
	h := newTestHandler(&fakeStore{statusCtxErr: errors.New("boom")})
	rr := postStatus(h, uuid.New().String(), validStatusBody(), "")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_UpdateStatus_ConflictWhenInCombat(t *testing.T) {
	store := &fakeStore{statusCtx: CharacterStatusContext{InActiveCombat: true}}
	h := newTestHandler(store)
	rr := postStatus(h, uuid.New().String(), validStatusBody(), "")
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if store.persisted != nil {
		t.Fatal("must not persist while in combat")
	}
}

func TestHandler_UpdateStatus_ValidationError(t *testing.T) {
	h := newTestHandler(&fakeStore{})
	body := validStatusBody()
	body.HPMax = 0 // invalid
	rr := postStatus(h, uuid.New().String(), body, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandler_UpdateStatus_ForbiddenWhenNotDM(t *testing.T) {
	campOwned := uuid.New()
	campOther := uuid.New()
	store := &fakeStore{statusCtx: CharacterStatusContext{CampaignID: campOther}}
	verifier := &fakeCampaignVerifier{ownedCampaign: campOwned.String()}
	h := NewHandler(NewService(store), WithCampaignVerifier(verifier))
	rr := postStatus(h, uuid.New().String(), validStatusBody(), "dm-1")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rr.Code)
	}
	if store.persisted != nil {
		t.Fatal("must not persist when forbidden")
	}
}

func TestHandler_UpdateStatus_AllowedWhenDM(t *testing.T) {
	campOwned := uuid.New()
	store := &fakeStore{statusCtx: CharacterStatusContext{CampaignID: campOwned}}
	verifier := &fakeCampaignVerifier{ownedCampaign: campOwned.String()}
	h := NewHandler(NewService(store), WithCampaignVerifier(verifier))
	rr := postStatus(h, uuid.New().String(), validStatusBody(), "dm-1")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
}

// --- DBStore status methods ---

func TestDBStore_GetCharacterStatusContext_NotInCombat(t *testing.T) {
	camp := uuid.New()
	fake := &fakeRefdata{char: refdata.Character{
		CampaignID:    camp,
		CharacterData: pqtype.NullRawMessage{RawMessage: []byte(`{"exhaustion_level":3}`), Valid: true},
	}}
	store := NewDBStore(fake)
	got, err := store.GetCharacterStatusContext(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CampaignID != camp || got.InActiveCombat {
		t.Fatalf("got %+v", got)
	}
	if string(got.CharacterData) != `{"exhaustion_level":3}` {
		t.Fatalf("character_data = %s", got.CharacterData)
	}
}

func TestDBStore_GetCharacterStatusContext_InCombat(t *testing.T) {
	fake := &fakeRefdata{
		char:      refdata.Character{CampaignID: uuid.New()},
		combatant: refdata.Combatant{ID: uuid.New()}, // non-nil → in combat
	}
	store := NewDBStore(fake)
	got, err := store.GetCharacterStatusContext(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.InActiveCombat {
		t.Fatal("expected InActiveCombat = true")
	}
}

func TestDBStore_GetCharacterStatusContext_NotFound(t *testing.T) {
	fake := &fakeRefdata{charErr: sql.ErrNoRows}
	store := NewDBStore(fake)
	_, err := store.GetCharacterStatusContext(context.Background(), uuid.New())
	if !errors.Is(err, ErrCharacterNotFound) {
		t.Fatalf("expected ErrCharacterNotFound, got %v", err)
	}
}

func TestDBStore_UpdateCharacterStatus_PassesParams(t *testing.T) {
	fake := &fakeRefdata{}
	store := NewDBStore(fake)
	id := uuid.New()
	err := store.UpdateCharacterStatus(context.Background(), PersistStatusParams{
		CharacterID:   id,
		HPMax:         25,
		HPCurrent:     20,
		TempHP:        3,
		Conditions:    json.RawMessage(`[{"condition":"prone"}]`),
		CharacterData: []byte(`{"exhaustion_level":1}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.vitalsArg.ID != id || fake.vitalsArg.HpMax != 25 || fake.vitalsArg.HpCurrent != 20 || fake.vitalsArg.TempHp != 3 {
		t.Fatalf("vitals arg = %+v", fake.vitalsArg)
	}
	if !fake.vitalsArg.CharacterData.Valid {
		t.Fatal("expected character_data marked Valid")
	}
}

func TestDBStore_UpdateCharacterStatus_NilCharacterDataNotValid(t *testing.T) {
	fake := &fakeRefdata{}
	store := NewDBStore(fake)
	err := store.UpdateCharacterStatus(context.Background(), PersistStatusParams{CharacterID: uuid.New(), HPMax: 1, HPCurrent: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.vitalsArg.CharacterData.Valid {
		t.Fatal("expected character_data Valid=false when nil")
	}
}

func TestSheetFromRefdata_MapsStatusFields(t *testing.T) {
	row := refdata.ListPlayerCharactersByStatusRow{
		CharacterName: "Aria",
		HpMax:         22,
		HpCurrent:     18,
		TempHp:        4,
		Conditions:    json.RawMessage(`[{"condition":"poisoned"},{"condition":"prone"}]`),
		CharacterData: pqtype.NullRawMessage{RawMessage: []byte(`{"exhaustion_level":2}`), Valid: true},
	}
	got := sheetFromRefdata(row)
	if got.TempHP != 4 {
		t.Fatalf("temp_hp = %d", got.TempHP)
	}
	if got.ExhaustionLevel != 2 {
		t.Fatalf("exhaustion = %d", got.ExhaustionLevel)
	}
	if len(got.Conditions) != 2 || got.Conditions[0] != "poisoned" {
		t.Fatalf("conditions = %+v", got.Conditions)
	}
}

func TestSheetFromRefdata_EmptyConditionsAndNoCharacterData(t *testing.T) {
	got := sheetFromRefdata(refdata.ListPlayerCharactersByStatusRow{CharacterName: "Bree"})
	if got.Conditions == nil {
		t.Fatal("expected empty (non-nil) conditions")
	}
	if got.ExhaustionLevel != 0 {
		t.Fatalf("exhaustion = %d", got.ExhaustionLevel)
	}
}
