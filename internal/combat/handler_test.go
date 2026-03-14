package combat

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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

func newTestCombatRouter(store Store) (*Handler, chi.Router) {
	svc := NewService(store)
	roller := dice.NewRoller(func(max int) int { return 15 })
	h := NewHandler(svc, roller)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return h, r
}

// --- TDD Cycle 34: POST /api/combat/start success ---

func TestHandler_StartCombat_Success(t *testing.T) {
	templateID := uuid.New()
	encounterID := uuid.New()
	charID := uuid.New()

	store := startCombatMockStore(templateID, encounterID, charID)
	_, r := newTestCombatRouter(store)

	body := map[string]interface{}{
		"template_id":    templateID.String(),
		"character_ids":  []string{charID.String()},
		"character_positions": map[string]interface{}{
			charID.String(): map[string]interface{}{"col": "D", "row": 5},
		},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/combat/start", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp startCombatResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, encounterID.String(), resp.Encounter.ID)
	assert.NotEmpty(t, resp.InitiativeTracker)
	assert.NotEmpty(t, resp.Combatants)
	assert.NotEmpty(t, resp.FirstTurn.TurnID)
}

// --- TDD Cycle 35: POST /api/combat/start invalid JSON ---

func TestHandler_StartCombat_InvalidJSON(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	req := httptest.NewRequest(http.MethodPost, "/api/combat/start", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 36: POST /api/combat/start invalid template_id ---

func TestHandler_StartCombat_InvalidTemplateID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	body := map[string]interface{}{
		"template_id": "not-a-uuid",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/combat/start", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 37: POST /api/combat/start invalid character_id ---

func TestHandler_StartCombat_InvalidCharacterID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	body := map[string]interface{}{
		"template_id":   uuid.New().String(),
		"character_ids": []string{"not-uuid"},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/combat/start", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 38: POST /api/combat/start invalid position key ---

func TestHandler_StartCombat_InvalidPositionKey(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	body := map[string]interface{}{
		"template_id":   uuid.New().String(),
		"character_ids": []string{},
		"character_positions": map[string]interface{}{
			"not-uuid": map[string]interface{}{"col": "A", "row": 1},
		},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/combat/start", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 39: POST /api/combat/start service error ---

func TestHandler_StartCombat_ServiceError(t *testing.T) {
	store := defaultMockStore()
	// Template not found will cause service error
	_, r := newTestCombatRouter(store)

	body := map[string]interface{}{
		"template_id":   uuid.New().String(),
		"character_ids": []string{},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/combat/start", bytes.NewReader(b))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// startCombatMockStore builds a fully-wired mock store for the start combat flow.
func startCombatMockStore(templateID, encounterID, charID uuid.UUID) *mockStore {
	store := defaultMockStore()

	store.getEncounterTemplateFn = func(ctx context.Context, id uuid.UUID) (refdata.EncounterTemplate, error) {
		return refdata.EncounterTemplate{
			ID:         templateID,
			CampaignID: uuid.New(),
			Name:       "Goblin Ambush",
			Creatures: json.RawMessage(`[
				{"creature_ref_id":"goblin","short_id":"G1","display_name":"Goblin","position_col":"A","position_row":1,"quantity":1}
			]`),
		}, nil
	}
	store.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{
			ID: "goblin", Name: "Goblin", Ac: 15, HpAverage: 7,
			Speed:         json.RawMessage(`{"walk":30}`),
			AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`),
		}, nil
	}
	store.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID: charID, Name: "Aragorn", HpMax: 45, HpCurrent: 45, Ac: 18, SpeedFt: 30,
			AbilityScores: json.RawMessage(`{"str":16,"dex":14,"con":14,"int":10,"wis":12,"cha":14}`),
		}, nil
	}

	createdCombatantIDs := []uuid.UUID{}
	store.createEncounterFn = func(ctx context.Context, arg refdata.CreateEncounterParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: encounterID, CampaignID: arg.CampaignID, Name: arg.Name, Status: arg.Status}, nil
	}
	store.createCombatantFn = func(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
		cID := uuid.New()
		createdCombatantIDs = append(createdCombatantIDs, cID)
		return refdata.Combatant{
			ID: cID, EncounterID: arg.EncounterID, ShortID: arg.ShortID, DisplayName: arg.DisplayName,
			HpMax: arg.HpMax, HpCurrent: arg.HpCurrent, Ac: arg.Ac, IsAlive: true, IsNpc: arg.IsNpc,
			IsVisible: true, Conditions: json.RawMessage(`[]`),
			CharacterID: arg.CharacterID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow,
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		if len(createdCombatantIDs) < 2 {
			return []refdata.Combatant{}, nil
		}
		return []refdata.Combatant{
			{ID: createdCombatantIDs[0], EncounterID: encounterID, DisplayName: "Goblin", ShortID: "G1", IsAlive: true, IsNpc: true, HpMax: 7, HpCurrent: 7, Conditions: json.RawMessage(`[]`), CreatureRefID: sql.NullString{String: "goblin", Valid: true}},
			{ID: createdCombatantIDs[1], EncounterID: encounterID, DisplayName: "Aragorn", ShortID: "AR", IsAlive: true, IsNpc: false, HpMax: 45, HpCurrent: 45, Conditions: json.RawMessage(`[]`), CharacterID: uuid.NullUUID{UUID: charID, Valid: true}},
		}, nil
	}
	store.updateCombatantInitiativeFn = func(ctx context.Context, arg refdata.UpdateCombatantInitiativeParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, EncounterID: encounterID, InitiativeRoll: arg.InitiativeRoll, InitiativeOrder: arg.InitiativeOrder, IsAlive: true, Conditions: json.RawMessage(`[]`), DisplayName: "Test"}, nil
	}
	store.updateEncounterRoundFn = func(ctx context.Context, arg refdata.UpdateEncounterRoundParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID, RoundNumber: arg.RoundNumber, Name: "Goblin Ambush", Status: "active"}, nil
	}
	store.updateEncounterStatusFn = func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID, Status: arg.Status, Name: "Goblin Ambush", RoundNumber: 1}, nil
	}
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Name: "Goblin Ambush", Status: "active", RoundNumber: 1}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.createTurnFn = func(ctx context.Context, arg refdata.CreateTurnParams) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), EncounterID: arg.EncounterID, CombatantID: arg.CombatantID, RoundNumber: arg.RoundNumber, Status: arg.Status}, nil
	}
	store.updateEncounterCurrentTurnFn = func(ctx context.Context, arg refdata.UpdateEncounterCurrentTurnParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID, CurrentTurnID: arg.CurrentTurnID, RoundNumber: 1, Name: "Goblin Ambush"}, nil
	}

	return store
}

// --- TDD Cycle 40: GET /api/combat/characters success ---

func TestHandler_ListCharacters_Success(t *testing.T) {
	charID := uuid.New()
	campaignID := uuid.New()
	store := defaultMockStore()
	store.listCharactersByCampaignFn = func(ctx context.Context, cID uuid.UUID) ([]refdata.Character, error) {
		assert.Equal(t, campaignID, cID)
		return []refdata.Character{
			{ID: charID, Name: "Aragorn", Race: "Human", Level: 5, HpMax: 45, Ac: 18, SpeedFt: 30},
		}, nil
	}
	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/characters?campaign_id="+campaignID.String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []characterListResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Len(t, resp, 1)
	assert.Equal(t, charID.String(), resp[0].ID)
	assert.Equal(t, "Aragorn", resp[0].Name)
	assert.Equal(t, int32(5), resp[0].Level)
}

// --- TDD Cycle 41: GET /api/combat/characters missing campaign_id ---

func TestHandler_ListCharacters_MissingCampaignID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	req := httptest.NewRequest(http.MethodGet, "/api/combat/characters", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 42: GET /api/combat/characters invalid campaign_id ---

func TestHandler_ListCharacters_InvalidCampaignID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	req := httptest.NewRequest(http.MethodGet, "/api/combat/characters?campaign_id=not-uuid", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 43: GET /api/combat/characters service error ---

func TestHandler_ListCharacters_ServiceError(t *testing.T) {
	store := defaultMockStore()
	store.listCharactersByCampaignFn = func(ctx context.Context, cID uuid.UUID) ([]refdata.Character, error) {
		return nil, errors.New("db error")
	}
	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/characters?campaign_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle 56: POST /api/combat/{encounterID}/end success ---

func TestHandler_EndCombat_Success(t *testing.T) {
	encounterID := uuid.New()
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID: id, Name: "Goblin Ambush", Status: "active", RoundNumber: 3,
			CurrentTurnID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
		}, nil
	}
	store.updateEncounterStatusFn = func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
		return refdata.Encounter{ID: arg.ID, Status: "completed", RoundNumber: 3, Name: "Goblin Ambush"}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`)},
			{ID: uuid.New(), IsNpc: false, IsAlive: true, HpCurrent: 30, DisplayName: "Aragorn", Conditions: json.RawMessage(`[]`)},
		}, nil
	}

	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+encounterID.String()+"/end", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp endCombatResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "completed", resp.Encounter.Status)
	assert.Equal(t, int32(3), resp.RoundsElapsed)
	assert.Equal(t, 1, resp.Casualties)
	assert.Contains(t, resp.Summary, "3 rounds")
}

// --- TDD Cycle 57: POST /api/combat/{encounterID}/end invalid ID ---

func TestHandler_EndCombat_InvalidID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	req := httptest.NewRequest(http.MethodPost, "/api/combat/not-a-uuid/end", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 73: POST /api/combat/{encounterID}/end non-active returns 409 ---

func TestHandler_EndCombat_NotActive_Returns409(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, Status: "preparing"}, nil
	}
	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+uuid.New().String()+"/end", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

// --- TDD Cycle 74: POST /api/combat/{encounterID}/end success includes initiative_tracker ---

func TestHandler_EndCombat_IncludesInitiativeTracker(t *testing.T) {
	encounterID := uuid.New()
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID: id, Name: "Goblin Ambush", Status: "active", RoundNumber: 3,
			DisplayName: sql.NullString{String: "The Goblin Ambush", Valid: true},
		}, nil
	}
	store.updateEncounterStatusFn = func(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
		return refdata.Encounter{
			ID: arg.ID, Status: "completed", RoundNumber: 3, Name: "Goblin Ambush",
			DisplayName: sql.NullString{String: "The Goblin Ambush", Valid: true},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, DisplayName: "Goblin", Conditions: json.RawMessage(`[]`)},
			{ID: uuid.New(), IsNpc: false, IsAlive: true, HpCurrent: 30, HpMax: 45, DisplayName: "Aragorn", Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+encounterID.String()+"/end", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp endCombatResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp.InitiativeTracker, "--- Combat Complete ---")
	assert.Contains(t, resp.InitiativeTracker, "The Goblin Ambush")
}

// --- TDD Cycle 58: POST /api/combat/{encounterID}/end service error ---

func TestHandler_EndCombat_ServiceError(t *testing.T) {
	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{}, errors.New("db error")
	}
	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+uuid.New().String()+"/end", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle 59: GET /api/combat/{encounterID}/hostiles-defeated success (all defeated) ---

func TestHandler_CheckHostilesDefeated_AllDefeated(t *testing.T) {
	encounterID := uuid.New()
	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), IsNpc: true, IsAlive: false, HpCurrent: 0, Conditions: json.RawMessage(`[]`)},
			{ID: uuid.New(), IsNpc: false, IsAlive: true, HpCurrent: 20, Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/hostiles-defeated", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp hostilesDefeatedResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.AllDefeated)
}

// --- TDD Cycle 60: GET /api/combat/{encounterID}/hostiles-defeated not all defeated ---

func TestHandler_CheckHostilesDefeated_NotAllDefeated(t *testing.T) {
	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: uuid.New(), IsNpc: true, IsAlive: true, HpCurrent: 10, Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/"+uuid.New().String()+"/hostiles-defeated", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp hostilesDefeatedResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.AllDefeated)
}

// --- TDD Cycle 61: GET /api/combat/{encounterID}/hostiles-defeated invalid ID ---

func TestHandler_CheckHostilesDefeated_InvalidID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	req := httptest.NewRequest(http.MethodGet, "/api/combat/not-uuid/hostiles-defeated", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle 62: GET /api/combat/{encounterID}/hostiles-defeated service error ---

func TestHandler_CheckHostilesDefeated_ServiceError(t *testing.T) {
	store := defaultMockStore()
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return nil, errors.New("db error")
	}
	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/"+uuid.New().String()+"/hostiles-defeated", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- TDD Cycle: POST /api/combat/{encounterID}/reactions success ---

func TestHandler_DeclareReaction_Success(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	declID := uuid.New()

	store := defaultMockStore()
	store.createReactionDeclarationFn = func(ctx context.Context, arg refdata.CreateReactionDeclarationParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID: declID, EncounterID: encounterID, CombatantID: combatantID,
			Description: arg.Description, Status: "active",
		}, nil
	}

	_, r := newTestCombatRouter(store)

	body := map[string]interface{}{
		"combatant_id": combatantID.String(),
		"description":  "Shield if I get hit",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+encounterID.String()+"/reactions", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp reactionDeclarationResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, declID.String(), resp.ID)
	assert.Equal(t, "active", resp.Status)
	assert.Equal(t, "Shield if I get hit", resp.Description)
}

func TestHandler_DeclareReaction_InvalidEncounterID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	body := map[string]interface{}{"combatant_id": uuid.New().String(), "description": "Shield"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/combat/not-uuid/reactions", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_DeclareReaction_InvalidJSON(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+uuid.New().String()+"/reactions", bytes.NewReader([]byte("bad")))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_DeclareReaction_InvalidCombatantID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	body := map[string]interface{}{"combatant_id": "not-uuid", "description": "Shield"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+uuid.New().String()+"/reactions", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle: GET /api/combat/{encounterID}/reactions success ---

func TestHandler_ListReactions_Success(t *testing.T) {
	encounterID := uuid.New()

	store := defaultMockStore()
	store.listActiveReactionDeclarationsByEncounterFn = func(ctx context.Context, encID uuid.UUID) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{ID: uuid.New(), EncounterID: encounterID, CombatantID: uuid.New(), Description: "Shield", Status: "active"},
		}, nil
	}

	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/"+encounterID.String()+"/reactions", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []reactionDeclarationResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Len(t, resp, 1)
}

func TestHandler_ListReactions_InvalidEncounterID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	req := httptest.NewRequest(http.MethodGet, "/api/combat/not-uuid/reactions", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle: POST /api/combat/{encounterID}/reactions/{reactionID}/resolve success ---

func TestHandler_ResolveReaction_Success(t *testing.T) {
	encounterID := uuid.New()
	declID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, EncounterID: encounterID, CombatantID: combatantID, Status: "active"}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 2, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 2, Status: "active"},
		}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, Status: "used"}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, ReactionUsed: true}, nil
	}

	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+encounterID.String()+"/reactions/"+declID.String()+"/resolve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp reactionDeclarationResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "used", resp.Status)
}

func TestHandler_ResolveReaction_InvalidReactionID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+uuid.New().String()+"/reactions/not-uuid/resolve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle: POST /api/combat/{encounterID}/reactions/{reactionID}/cancel success ---

func TestHandler_CancelReaction_Success(t *testing.T) {
	encounterID := uuid.New()
	declID := uuid.New()

	store := defaultMockStore()
	store.cancelReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, Status: "cancelled"}, nil
	}

	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+encounterID.String()+"/reactions/"+declID.String()+"/cancel", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp reactionDeclarationResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", resp.Status)
}

func TestHandler_CancelReaction_InvalidReactionID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+uuid.New().String()+"/reactions/not-uuid/cancel", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- TDD Cycle: POST /api/combat/{encounterID}/combatants/{combatantID}/reactions/cancel-all ---

func TestHandler_CancelAllReactions_Success(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.cancelAllReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.CancelAllReactionDeclarationsByCombatantParams) error {
		assert.Equal(t, combatantID, arg.CombatantID)
		assert.Equal(t, encounterID, arg.EncounterID)
		return nil
	}

	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+encounterID.String()+"/combatants/"+combatantID.String()+"/reactions/cancel-all", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandler_CancelAllReactions_InvalidEncounterID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	req := httptest.NewRequest(http.MethodPost, "/api/combat/not-uuid/combatants/"+uuid.New().String()+"/reactions/cancel-all", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_CancelAllReactions_InvalidCombatantID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+uuid.New().String()+"/combatants/not-uuid/reactions/cancel-all", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_CancelAllReactions_ServiceError(t *testing.T) {
	store := defaultMockStore()
	store.cancelAllReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.CancelAllReactionDeclarationsByCombatantParams) error {
		return errors.New("db error")
	}
	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+uuid.New().String()+"/combatants/"+uuid.New().String()+"/reactions/cancel-all", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_ListReactions_ServiceError(t *testing.T) {
	store := defaultMockStore()
	store.listActiveReactionDeclarationsByEncounterFn = func(ctx context.Context, encID uuid.UUID) ([]refdata.ReactionDeclaration, error) {
		return nil, errors.New("db error")
	}
	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/api/combat/"+uuid.New().String()+"/reactions", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_ResolveReaction_InvalidEncounterID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	req := httptest.NewRequest(http.MethodPost, "/api/combat/not-uuid/reactions/"+uuid.New().String()+"/resolve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ResolveReaction_ServiceError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("db error")
	}
	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+uuid.New().String()+"/reactions/"+uuid.New().String()+"/resolve", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_CancelReaction_InvalidEncounterID(t *testing.T) {
	_, r := newTestCombatRouter(defaultMockStore())

	req := httptest.NewRequest(http.MethodPost, "/api/combat/not-uuid/reactions/"+uuid.New().String()+"/cancel", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_CancelReaction_ServiceError(t *testing.T) {
	store := defaultMockStore()
	store.cancelReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("not found")
	}
	_, r := newTestCombatRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+uuid.New().String()+"/reactions/"+uuid.New().String()+"/cancel", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_DeclareReaction_ServiceError(t *testing.T) {
	store := defaultMockStore()
	store.createReactionDeclarationFn = func(ctx context.Context, arg refdata.CreateReactionDeclarationParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("db error")
	}
	_, r := newTestCombatRouter(store)

	body := map[string]interface{}{"combatant_id": uuid.New().String(), "description": "Shield"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/combat/"+uuid.New().String()+"/reactions", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

