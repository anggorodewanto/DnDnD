package dashboard

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
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

type stubCheckStore struct {
	combatants     []refdata.Combatant
	combatantErr   error
	characters     map[uuid.UUID]refdata.Character
	upsertCalls    []refdata.UpsertPendingCheckParams
	upsertReturn   refdata.PendingCheck
	upsertErr      error
	characterErr   error
}

func (s *stubCheckStore) ListCombatantsByEncounterID(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
	return s.combatants, s.combatantErr
}

func (s *stubCheckStore) GetCharacter(_ context.Context, id uuid.UUID) (refdata.Character, error) {
	if s.characterErr != nil {
		return refdata.Character{}, s.characterErr
	}
	if c, ok := s.characters[id]; ok {
		return c, nil
	}
	return refdata.Character{}, sql.ErrNoRows
}

func (s *stubCheckStore) UpsertPendingCheck(_ context.Context, arg refdata.UpsertPendingCheckParams) (refdata.PendingCheck, error) {
	s.upsertCalls = append(s.upsertCalls, arg)
	if s.upsertErr != nil {
		return refdata.PendingCheck{}, s.upsertErr
	}
	if s.upsertReturn.ID == uuid.Nil {
		s.upsertReturn = refdata.PendingCheck{
			ID:          uuid.New(),
			EncounterID: arg.EncounterID,
			CombatantID: arg.CombatantID,
			Skill:       arg.Skill,
			Dc:          arg.Dc,
			Status:      "pending",
		}
	}
	return s.upsertReturn, nil
}

func chiCheckCtx(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func makeCheckTestCharacter(name string) refdata.Character {
	scores, _ := json.Marshal(character.AbilityScores{STR: 14, DEX: 16, CON: 12, INT: 10, WIS: 16, CHA: 8})
	profs, _ := json.Marshal(map[string]interface{}{
		"skills": []string{"medicine", "stealth"},
	})
	return refdata.Character{
		ID:               uuid.New(),
		Name:             name,
		Level:            5,
		ProficiencyBonus: 3,
		AbilityScores:    scores,
		Proficiencies:    pqtype.NullRawMessage{RawMessage: profs, Valid: true},
	}
}

func TestDashboardCheckHandler_GroupCheck_AggregatesRolls(t *testing.T) {
	charA := makeCheckTestCharacter("Aria")
	charB := makeCheckTestCharacter("Bjorn")
	encID := uuid.New()
	combAID := uuid.New()
	combBID := uuid.New()
	store := &stubCheckStore{
		combatants: []refdata.Combatant{
			{ID: combAID, DisplayName: "Aria", CharacterID: uuid.NullUUID{UUID: charA.ID, Valid: true}, IsAlive: true},
			{ID: combBID, DisplayName: "Bjorn", CharacterID: uuid.NullUUID{UUID: charB.ID, Valid: true}, IsAlive: true},
		},
		characters: map[uuid.UUID]refdata.Character{
			charA.ID: charA,
			charB.ID: charB,
		},
	}
	// Deterministic roller: always returns 12 on d20.
	roller := dice.NewRoller(func(_ int) int { return 12 })
	h := NewCheckHandler(store, roller)

	body := `{"skill":"medicine","dc":10}`
	req := httptest.NewRequest(http.MethodPost, "/api/encounters/x/group-check", strings.NewReader(body))
	req = chiCheckCtx(req, map[string]string{"encounterID": encID.String()})
	rec := httptest.NewRecorder()
	h.HandleGroupCheck(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp GroupCheckResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(resp.Results))
	}
	if resp.Skill != "medicine" {
		t.Errorf("skill mismatch: %s", resp.Skill)
	}
	if resp.DC != 10 {
		t.Errorf("dc mismatch: %d", resp.DC)
	}
	// d20=12 + WIS(+3) + medicine prof(+3) = 18 >= 10, passes
	for _, r := range resp.Results {
		if !r.Passed {
			t.Errorf("participant %s expected to pass (total %d), got passed=%v", r.Name, r.Total, r.Passed)
		}
	}
	if !resp.Success {
		t.Error("expected group success")
	}
}

func TestDashboardCheckHandler_GroupCheck_NoParticipants_400(t *testing.T) {
	store := &stubCheckStore{}
	h := NewCheckHandler(store, dice.NewRoller(func(_ int) int { return 10 }))
	body := `{"skill":"perception","dc":10}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req = chiCheckCtx(req, map[string]string{"encounterID": uuid.New().String()})
	rec := httptest.NewRecorder()
	h.HandleGroupCheck(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDashboardCheckHandler_GroupCheck_InvalidEncounter_400(t *testing.T) {
	h := NewCheckHandler(&stubCheckStore{}, dice.NewRoller(func(_ int) int { return 10 }))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"skill":"x"}`))
	req = chiCheckCtx(req, map[string]string{"encounterID": "not-uuid"})
	rec := httptest.NewRecorder()
	h.HandleGroupCheck(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestDashboardCheckHandler_GroupCheck_MissingSkill_400(t *testing.T) {
	h := NewCheckHandler(&stubCheckStore{}, dice.NewRoller(func(_ int) int { return 10 }))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	req = chiCheckCtx(req, map[string]string{"encounterID": uuid.New().String()})
	rec := httptest.NewRecorder()
	h.HandleGroupCheck(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestDashboardCheckHandler_GroupCheck_BadJSON_400(t *testing.T) {
	h := NewCheckHandler(&stubCheckStore{}, dice.NewRoller(func(_ int) int { return 10 }))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{garbage`))
	req = chiCheckCtx(req, map[string]string{"encounterID": uuid.New().String()})
	rec := httptest.NewRecorder()
	h.HandleGroupCheck(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestDashboardCheckHandler_PromptCheck_PersistsRow(t *testing.T) {
	store := &stubCheckStore{}
	h := NewCheckHandler(store, dice.NewRoller(func(_ int) int { return 10 }))
	combatantID := uuid.New()
	encounterID := uuid.New()
	body := `{"combatant_id":"` + combatantID.String() + `","skill":"perception","dc":15,"reason":"hear footsteps"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req = chiCheckCtx(req, map[string]string{"encounterID": encounterID.String()})
	rec := httptest.NewRecorder()
	h.HandlePromptCheck(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(store.upsertCalls) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(store.upsertCalls))
	}
	call := store.upsertCalls[0]
	if call.EncounterID != encounterID || call.CombatantID != combatantID || call.Skill != "perception" || call.Dc != 15 {
		t.Errorf("unexpected upsert params: %+v", call)
	}
	if !call.Reason.Valid || call.Reason.String != "hear footsteps" {
		t.Errorf("reason missing or wrong: %+v", call.Reason)
	}
}

func TestDashboardCheckHandler_PromptCheck_BadJSON_400(t *testing.T) {
	h := NewCheckHandler(&stubCheckStore{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{not"))
	req = chiCheckCtx(req, map[string]string{"encounterID": uuid.New().String()})
	rec := httptest.NewRecorder()
	h.HandlePromptCheck(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status=%d", rec.Code)
	}
}

func TestDashboardCheckHandler_PromptCheck_MissingSkill_400(t *testing.T) {
	h := NewCheckHandler(&stubCheckStore{}, nil)
	body := `{"combatant_id":"` + uuid.New().String() + `","dc":10}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req = chiCheckCtx(req, map[string]string{"encounterID": uuid.New().String()})
	rec := httptest.NewRecorder()
	h.HandlePromptCheck(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status=%d", rec.Code)
	}
}

func TestDashboardCheckHandler_PromptCheck_MissingCombatantID_400(t *testing.T) {
	h := NewCheckHandler(&stubCheckStore{}, nil)
	body := `{"skill":"perception","dc":10}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req = chiCheckCtx(req, map[string]string{"encounterID": uuid.New().String()})
	rec := httptest.NewRecorder()
	h.HandlePromptCheck(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status=%d", rec.Code)
	}
}
