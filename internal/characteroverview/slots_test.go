package characteroverview

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Service.ApplySlots ---

func TestApplySlots_SpellOnly_PersistsSpellNotPact(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store)
	spell := map[int]character.SlotInfo{1: {Current: 1, Max: 4}, 2: {Current: 0, Max: 2}}

	err := svc.ApplySlots(context.Background(), uuid.New(), SlotsUpdate{SpellSlots: &spell})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.persistedSlots == nil {
		t.Fatal("expected persisted slots")
	}
	if store.persistedSlots.SpellSlots == nil {
		t.Fatal("expected spell slots persisted")
	}
	if store.persistedSlots.PactMagicSlots != nil {
		t.Fatal("pact must be untouched when not provided")
	}
	var parsed map[string]character.SlotInfo
	if err := json.Unmarshal(store.persistedSlots.SpellSlots, &parsed); err != nil {
		t.Fatalf("spell slots not valid JSON: %v", err)
	}
	if parsed["1"].Max != 4 || parsed["2"].Max != 2 {
		t.Fatalf("persisted spell slots = %+v", parsed)
	}
}

func TestApplySlots_PactOnly_PersistsPactNotSpell(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store)
	pact := character.PactMagicSlots{SlotLevel: 3, Current: 1, Max: 2}

	err := svc.ApplySlots(context.Background(), uuid.New(), SlotsUpdate{PactMagicSlots: &pact})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.persistedSlots.SpellSlots != nil {
		t.Fatal("spell must be untouched when not provided")
	}
	if store.persistedSlots.PactMagicSlots == nil {
		t.Fatal("expected pact slots persisted")
	}
	var got character.PactMagicSlots
	if err := json.Unmarshal(store.persistedSlots.PactMagicSlots, &got); err != nil {
		t.Fatalf("pact slots not valid JSON: %v", err)
	}
	if got != pact {
		t.Fatalf("persisted pact = %+v", got)
	}
}

func TestApplySlots_Both_PersistsBoth(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store)
	spell := map[int]character.SlotInfo{1: {Current: 2, Max: 4}}
	pact := character.PactMagicSlots{SlotLevel: 2, Current: 2, Max: 2}

	err := svc.ApplySlots(context.Background(), uuid.New(), SlotsUpdate{SpellSlots: &spell, PactMagicSlots: &pact})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.persistedSlots.SpellSlots == nil || store.persistedSlots.PactMagicSlots == nil {
		t.Fatalf("expected both persisted: %+v", store.persistedSlots)
	}
}

func TestApplySlots_ValidationErrors(t *testing.T) {
	spellBad := map[int]character.SlotInfo{1: {Current: 5, Max: 4}} // current > max
	spellLvl0 := map[int]character.SlotInfo{0: {Current: 0, Max: 2}}
	spellLvl10 := map[int]character.SlotInfo{10: {Current: 0, Max: 2}}
	pactBad := character.PactMagicSlots{SlotLevel: 6, Current: 0, Max: 2} // level out of range

	cases := map[string]SlotsUpdate{
		"spell current>max": {SpellSlots: &spellBad},
		"spell level 0":     {SpellSlots: &spellLvl0},
		"spell level 10":    {SpellSlots: &spellLvl10},
		"pact level 6":      {PactMagicSlots: &pactBad},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			store := &fakeStore{}
			svc := NewService(store)
			err := svc.ApplySlots(context.Background(), uuid.New(), in)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}
			if store.persistedSlots != nil {
				t.Fatal("must not persist on validation failure")
			}
		})
	}
}

func TestApplySlots_PersistErrorPropagates(t *testing.T) {
	store := &fakeStore{persistSlotsErr: errors.New("db down")}
	svc := NewService(store)
	spell := map[int]character.SlotInfo{1: {Current: 1, Max: 4}}
	err := svc.ApplySlots(context.Background(), uuid.New(), SlotsUpdate{SpellSlots: &spell})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetSlotsContext_RejectsNilID(t *testing.T) {
	svc := NewService(&fakeStore{})
	_, err := svc.GetSlotsContext(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetSlotsContext_Delegates(t *testing.T) {
	camp := uuid.New()
	store := &fakeStore{slotsCtx: SlotsContext{
		CampaignID:     camp,
		InActiveCombat: true,
		SpellSlots:     map[int]character.SlotInfo{1: {Current: 1, Max: 2}},
		PactMagicSlots: character.PactMagicSlots{SlotLevel: 1, Current: 1, Max: 1},
	}}
	svc := NewService(store)
	got, err := svc.GetSlotsContext(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CampaignID != camp || !got.InActiveCombat || got.SpellSlots[1].Max != 2 {
		t.Fatalf("got %+v", got)
	}
}

// --- Handler.UpdateSlots / GetSlots ---

func postSlots(h *Handler, characterID, body, userID string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/api/character-overview/"+characterID+"/slots", strings.NewReader(body))
	if userID != "" {
		req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), userID))
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func getSlots(h *Handler, characterID, userID string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/character-overview/"+characterID+"/slots", nil)
	if userID != "" {
		req = req.WithContext(auth.ContextWithDiscordUserID(req.Context(), userID))
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

type slotsResp struct {
	SpellSlots     map[string]character.SlotInfo `json:"spell_slots"`
	PactMagicSlots *character.PactMagicSlots     `json:"pact_magic_slots"`
}

func TestHandler_UpdateSlots_SpellOnly_Success(t *testing.T) {
	store := &fakeStore{}
	h := newTestHandler(store)
	rr := postSlots(h, uuid.New().String(), `{"spell_slots":{"1":{"current":1,"max":4}}}`, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var resp slotsResp
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.SpellSlots["1"].Max != 4 {
		t.Fatalf("resp spell = %+v", resp.SpellSlots)
	}
	if resp.PactMagicSlots != nil {
		t.Fatalf("expected null pact, got %+v", resp.PactMagicSlots)
	}
	if store.persistedSlots == nil || store.persistedSlots.SpellSlots == nil {
		t.Fatal("expected spell persistence")
	}
}

func TestHandler_UpdateSlots_PactOnly_Success(t *testing.T) {
	store := &fakeStore{}
	h := newTestHandler(store)
	rr := postSlots(h, uuid.New().String(), `{"pact_magic_slots":{"slot_level":3,"current":1,"max":2}}`, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var resp slotsResp
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.PactMagicSlots == nil || resp.PactMagicSlots.SlotLevel != 3 {
		t.Fatalf("resp pact = %+v", resp.PactMagicSlots)
	}
	if store.persistedSlots == nil || store.persistedSlots.PactMagicSlots == nil {
		t.Fatal("expected pact persistence")
	}
}

func TestHandler_UpdateSlots_Both_Success(t *testing.T) {
	store := &fakeStore{}
	h := newTestHandler(store)
	rr := postSlots(h, uuid.New().String(),
		`{"spell_slots":{"1":{"current":2,"max":4}},"pact_magic_slots":{"slot_level":2,"current":2,"max":2}}`, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if store.persistedSlots == nil || store.persistedSlots.SpellSlots == nil || store.persistedSlots.PactMagicSlots == nil {
		t.Fatalf("expected both persisted: %+v", store.persistedSlots)
	}
}

func TestHandler_UpdateSlots_InvalidCharacterID(t *testing.T) {
	h := newTestHandler(&fakeStore{})
	rr := postSlots(h, "not-a-uuid", `{"spell_slots":{"1":{"current":1,"max":4}}}`, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_UpdateSlots_BadBody(t *testing.T) {
	h := newTestHandler(&fakeStore{})
	rr := postSlots(h, uuid.New().String(), "{bad", "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_UpdateSlots_NotFound(t *testing.T) {
	h := newTestHandler(&fakeStore{slotsCtxErr: ErrCharacterNotFound})
	rr := postSlots(h, uuid.New().String(), `{"spell_slots":{"1":{"current":1,"max":4}}}`, "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_UpdateSlots_StoreContextError(t *testing.T) {
	h := newTestHandler(&fakeStore{slotsCtxErr: errors.New("boom")})
	rr := postSlots(h, uuid.New().String(), `{"spell_slots":{"1":{"current":1,"max":4}}}`, "")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_UpdateSlots_ConflictWhenInCombat(t *testing.T) {
	store := &fakeStore{slotsCtx: SlotsContext{InActiveCombat: true}}
	h := newTestHandler(store)
	rr := postSlots(h, uuid.New().String(), `{"spell_slots":{"1":{"current":1,"max":4}}}`, "")
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if store.persistedSlots != nil {
		t.Fatal("must not persist while in combat")
	}
}

func TestHandler_UpdateSlots_ValidationError(t *testing.T) {
	cases := map[string]string{
		"current>max": `{"spell_slots":{"1":{"current":5,"max":4}}}`,
		"level 0":     `{"spell_slots":{"0":{"current":0,"max":2}}}`,
		"level 10":    `{"spell_slots":{"10":{"current":0,"max":2}}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			store := &fakeStore{}
			h := newTestHandler(store)
			rr := postSlots(h, uuid.New().String(), body, "")
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
			}
			if store.persistedSlots != nil {
				t.Fatal("must not persist on invalid input")
			}
		})
	}
}

func TestHandler_UpdateSlots_InvalidPactJSON(t *testing.T) {
	store := &fakeStore{}
	h := newTestHandler(store)
	rr := postSlots(h, uuid.New().String(), `{"pact_magic_slots":"not-an-object"}`, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if store.persistedSlots != nil {
		t.Fatal("must not persist on bad pact JSON")
	}
}

func TestHandler_UpdateSlots_ApplyInternalError(t *testing.T) {
	store := &fakeStore{persistSlotsErr: errors.New("db down")}
	h := newTestHandler(store)
	rr := postSlots(h, uuid.New().String(), `{"spell_slots":{"1":{"current":1,"max":4}}}`, "")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandler_UpdateSlots_ForbiddenWhenNotDM(t *testing.T) {
	campOwned := uuid.New()
	campOther := uuid.New()
	store := &fakeStore{slotsCtx: SlotsContext{CampaignID: campOther}}
	verifier := &fakeCampaignVerifier{ownedCampaign: campOwned.String()}
	h := NewHandler(NewService(store), WithCampaignVerifier(verifier))
	rr := postSlots(h, uuid.New().String(), `{"spell_slots":{"1":{"current":1,"max":4}}}`, "dm-1")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rr.Code)
	}
	if store.persistedSlots != nil {
		t.Fatal("must not persist when forbidden")
	}
}

func TestHandler_UpdateSlots_AllowedWhenDM(t *testing.T) {
	campOwned := uuid.New()
	store := &fakeStore{slotsCtx: SlotsContext{CampaignID: campOwned}}
	verifier := &fakeCampaignVerifier{ownedCampaign: campOwned.String()}
	h := NewHandler(NewService(store), WithCampaignVerifier(verifier))
	rr := postSlots(h, uuid.New().String(), `{"spell_slots":{"1":{"current":1,"max":4}}}`, "dm-1")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandler_GetSlots_ReturnsCurrentValues(t *testing.T) {
	store := &fakeStore{slotsCtx: SlotsContext{
		SpellSlots:     map[int]character.SlotInfo{1: {Current: 2, Max: 4}, 3: {Current: 0, Max: 1}},
		PactMagicSlots: character.PactMagicSlots{SlotLevel: 3, Current: 1, Max: 2},
	}}
	h := newTestHandler(store)
	rr := getSlots(h, uuid.New().String(), "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var resp slotsResp
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.SpellSlots["1"].Max != 4 || resp.SpellSlots["3"].Max != 1 {
		t.Fatalf("resp spell = %+v", resp.SpellSlots)
	}
	if resp.PactMagicSlots == nil || resp.PactMagicSlots.SlotLevel != 3 {
		t.Fatalf("resp pact = %+v", resp.PactMagicSlots)
	}
}

func TestHandler_GetSlots_NullPactWhenZero(t *testing.T) {
	store := &fakeStore{slotsCtx: SlotsContext{SpellSlots: map[int]character.SlotInfo{}}}
	h := newTestHandler(store)
	rr := getSlots(h, uuid.New().String(), "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var resp slotsResp
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.PactMagicSlots != nil {
		t.Fatalf("expected null pact, got %+v", resp.PactMagicSlots)
	}
	if resp.SpellSlots == nil {
		t.Fatal("expected non-nil (empty) spell map")
	}
}

func TestHandler_GetSlots_AllowedInCombat(t *testing.T) {
	store := &fakeStore{slotsCtx: SlotsContext{InActiveCombat: true, SpellSlots: map[int]character.SlotInfo{1: {Current: 1, Max: 2}}}}
	h := newTestHandler(store)
	rr := getSlots(h, uuid.New().String(), "")
	if rr.Code != http.StatusOK {
		t.Fatalf("reads must be allowed in combat, status = %d", rr.Code)
	}
}

func TestHandler_GetSlots_InvalidCharacterID(t *testing.T) {
	h := newTestHandler(&fakeStore{})
	rr := getSlots(h, "not-a-uuid", "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_GetSlots_NotFound(t *testing.T) {
	h := newTestHandler(&fakeStore{slotsCtxErr: ErrCharacterNotFound})
	rr := getSlots(h, uuid.New().String(), "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_GetSlots_StoreError(t *testing.T) {
	h := newTestHandler(&fakeStore{slotsCtxErr: errors.New("boom")})
	rr := getSlots(h, uuid.New().String(), "")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestHandler_GetSlots_ForbiddenWhenNotDM(t *testing.T) {
	campOwned := uuid.New()
	campOther := uuid.New()
	store := &fakeStore{slotsCtx: SlotsContext{CampaignID: campOther}}
	verifier := &fakeCampaignVerifier{ownedCampaign: campOwned.String()}
	h := NewHandler(NewService(store), WithCampaignVerifier(verifier))
	rr := getSlots(h, uuid.New().String(), "dm-1")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rr.Code)
	}
}

// --- DBStore slot methods ---

func TestDBStore_GetCharacterSlotsContext_ParsesSlots(t *testing.T) {
	camp := uuid.New()
	fake := &fakeRefdata{char: refdata.Character{
		CampaignID:     camp,
		SpellSlots:     pqtype.NullRawMessage{RawMessage: []byte(`{"1":{"current":2,"max":4}}`), Valid: true},
		PactMagicSlots: pqtype.NullRawMessage{RawMessage: []byte(`{"slot_level":3,"current":1,"max":2}`), Valid: true},
	}}
	store := NewDBStore(fake)
	got, err := store.GetCharacterSlotsContext(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CampaignID != camp || got.InActiveCombat {
		t.Fatalf("got %+v", got)
	}
	if got.SpellSlots[1].Max != 4 {
		t.Fatalf("spell = %+v", got.SpellSlots)
	}
	if got.PactMagicSlots.SlotLevel != 3 || got.PactMagicSlots.Current != 1 {
		t.Fatalf("pact = %+v", got.PactMagicSlots)
	}
}

func TestDBStore_GetCharacterSlotsContext_EmptySlots(t *testing.T) {
	fake := &fakeRefdata{char: refdata.Character{CampaignID: uuid.New()}}
	store := NewDBStore(fake)
	got, err := store.GetCharacterSlotsContext(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SpellSlots == nil {
		t.Fatal("expected non-nil (empty) spell map")
	}
	if got.PactMagicSlots != (character.PactMagicSlots{}) {
		t.Fatalf("expected zero pact, got %+v", got.PactMagicSlots)
	}
}

func TestDBStore_GetCharacterSlotsContext_InCombat(t *testing.T) {
	fake := &fakeRefdata{
		char:      refdata.Character{CampaignID: uuid.New()},
		combatant: refdata.Combatant{ID: uuid.New()},
	}
	store := NewDBStore(fake)
	got, err := store.GetCharacterSlotsContext(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.InActiveCombat {
		t.Fatal("expected InActiveCombat = true")
	}
}

func TestDBStore_GetCharacterSlotsContext_NotFound(t *testing.T) {
	fake := &fakeRefdata{charErr: sql.ErrNoRows}
	store := NewDBStore(fake)
	_, err := store.GetCharacterSlotsContext(context.Background(), uuid.New())
	if !errors.Is(err, ErrCharacterNotFound) {
		t.Fatalf("expected ErrCharacterNotFound, got %v", err)
	}
}

func TestDBStore_GetCharacterSlotsContext_GenericError(t *testing.T) {
	fake := &fakeRefdata{charErr: errors.New("boom")}
	store := NewDBStore(fake)
	_, err := store.GetCharacterSlotsContext(context.Background(), uuid.New())
	if err == nil || errors.Is(err, ErrCharacterNotFound) {
		t.Fatalf("expected generic error, got %v", err)
	}
}

func TestDBStore_UpdateCharacterSlots_Both(t *testing.T) {
	fake := &fakeRefdata{}
	store := NewDBStore(fake)
	id := uuid.New()
	err := store.UpdateCharacterSlots(context.Background(), PersistSlotsParams{
		CharacterID:    id,
		SpellSlots:     json.RawMessage(`{"1":{"current":1,"max":4}}`),
		PactMagicSlots: json.RawMessage(`{"slot_level":3,"current":1,"max":2}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.spellSlotsArg.ID != id || !fake.spellSlotsArg.SpellSlots.Valid {
		t.Fatalf("spell arg = %+v", fake.spellSlotsArg)
	}
	if fake.pactSlotsArg.ID != id || !fake.pactSlotsArg.PactMagicSlots.Valid {
		t.Fatalf("pact arg = %+v", fake.pactSlotsArg)
	}
}

func TestDBStore_UpdateCharacterSlots_SpellOnly_SkipsPact(t *testing.T) {
	fake := &fakeRefdata{}
	store := NewDBStore(fake)
	err := store.UpdateCharacterSlots(context.Background(), PersistSlotsParams{
		CharacterID: uuid.New(),
		SpellSlots:  json.RawMessage(`{"1":{"current":1,"max":4}}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fake.spellSlotsArg.SpellSlots.Valid {
		t.Fatal("expected spell update")
	}
	if fake.pactSlotsArg.PactMagicSlots.Valid {
		t.Fatal("must not update pact when not provided")
	}
}

func TestDBStore_UpdateCharacterSlots_PactOnly_SkipsSpell(t *testing.T) {
	fake := &fakeRefdata{}
	store := NewDBStore(fake)
	err := store.UpdateCharacterSlots(context.Background(), PersistSlotsParams{
		CharacterID:    uuid.New(),
		PactMagicSlots: json.RawMessage(`{"slot_level":3,"current":1,"max":2}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.spellSlotsArg.SpellSlots.Valid {
		t.Fatal("must not update spell when not provided")
	}
	if !fake.pactSlotsArg.PactMagicSlots.Valid {
		t.Fatal("expected pact update")
	}
}

func TestDBStore_UpdateCharacterSlots_SpellErrorPropagates(t *testing.T) {
	fake := &fakeRefdata{spellSlotsErr: errors.New("boom")}
	store := NewDBStore(fake)
	err := store.UpdateCharacterSlots(context.Background(), PersistSlotsParams{
		CharacterID: uuid.New(),
		SpellSlots:  json.RawMessage(`{"1":{"current":1,"max":4}}`),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDBStore_UpdateCharacterSlots_PactErrorPropagates(t *testing.T) {
	fake := &fakeRefdata{pactSlotsErr: errors.New("boom")}
	store := NewDBStore(fake)
	err := store.UpdateCharacterSlots(context.Background(), PersistSlotsParams{
		CharacterID:    uuid.New(),
		PactMagicSlots: json.RawMessage(`{"slot_level":3,"current":1,"max":2}`),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSheetFromRefdata_MapsSlotFields(t *testing.T) {
	row := refdata.ListPlayerCharactersByStatusRow{
		CharacterName:  "Aria",
		SpellSlots:     pqtype.NullRawMessage{RawMessage: []byte(`{"1":{"current":2,"max":4}}`), Valid: true},
		PactMagicSlots: pqtype.NullRawMessage{RawMessage: []byte(`{"slot_level":3,"current":1,"max":2}`), Valid: true},
	}
	got := sheetFromRefdata(row)
	if got.SpellSlots["1"].Max != 4 {
		t.Fatalf("spell = %+v", got.SpellSlots)
	}
	if got.PactMagicSlots == nil || got.PactMagicSlots.SlotLevel != 3 {
		t.Fatalf("pact = %+v", got.PactMagicSlots)
	}
}

func TestSheetFromRefdata_EmptySlots(t *testing.T) {
	got := sheetFromRefdata(refdata.ListPlayerCharactersByStatusRow{CharacterName: "Bree"})
	if got.SpellSlots == nil {
		t.Fatal("expected non-nil (empty) spell map")
	}
	if got.PactMagicSlots != nil {
		t.Fatalf("expected nil pact pointer, got %+v", got.PactMagicSlots)
	}
}
