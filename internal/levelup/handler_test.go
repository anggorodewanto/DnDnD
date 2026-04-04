package levelup

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func setupTestHandler(t *testing.T) (*Handler, *mockCharacterStore, *mockClassStore, *mockNotifier) {
	t.Helper()
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	notifier := &mockNotifier{}
	svc := NewService(charStore, classStore, notifier)
	h := NewHandler(svc, nil)
	return h, charStore, classStore, notifier
}

func TestHandler_HandleLevelUp_Success(t *testing.T) {
	h, charStore, classStore, _ := setupTestHandler(t)

	charID := uuid.New()
	classes := []character.ClassEntry{{Class: "fighter", Level: 5}}
	classesJSON, _ := json.Marshal(classes)
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}

	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Aria",
		DiscordUserID: "user123",
		Level:         5,
		HPMax:         44,
		HPCurrent:     44,
		Classes:       classesJSON,
		AbilityScores: mustJSON(t, scores),
	}

	classStore.classes["fighter"] = &ClassRefData{
		HitDie:           "d10",
		AttacksPerAction: map[int]int{1: 1, 5: 2, 11: 3, 20: 4},
		SubclassLevel:    3,
	}

	body, _ := json.Marshal(LevelUpRequest{
		CharacterID: charID,
		ClassID:     "fighter",
		NewLevel:    6,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/levelup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleLevelUp(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp LevelUpResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.NewLevel != 6 {
		t.Errorf("resp.NewLevel = %d, want 6", resp.NewLevel)
	}
}

func TestHandler_HandleLevelUp_InvalidBody(t *testing.T) {
	h, _, _, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/levelup", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()

	h.HandleLevelUp(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_HandleLevelUp_CharacterNotFound(t *testing.T) {
	h, _, classStore, _ := setupTestHandler(t)

	classStore.classes["fighter"] = &ClassRefData{
		HitDie:           "d10",
		AttacksPerAction: map[int]int{1: 1},
		SubclassLevel:    3,
	}

	body, _ := json.Marshal(LevelUpRequest{
		CharacterID: uuid.New(),
		ClassID:     "fighter",
		NewLevel:    2,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/levelup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleLevelUp(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandler_HandleApproveASI_Success(t *testing.T) {
	h, charStore, _, _ := setupTestHandler(t)

	charID := uuid.New()
	classes := []character.ClassEntry{{Class: "fighter", Level: 4}}
	classesJSON, _ := json.Marshal(classes)
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}

	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Aria",
		DiscordUserID: "user123",
		Level:         4,
		Classes:       classesJSON,
		AbilityScores: mustJSON(t, scores),
	}

	body, _ := json.Marshal(ASIApprovalRequest{
		CharacterID: charID,
		Choice: ASIChoice{
			Type:    ASIPlus2,
			Ability: "str",
		},
	})

	r := chi.NewRouter()
	r.Post("/api/levelup/asi/approve", h.HandleApproveASI)

	req := httptest.NewRequest(http.MethodPost, "/api/levelup/asi/approve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandler_HandleDenyASI_Success(t *testing.T) {
	h, charStore, _, _ := setupTestHandler(t)

	charID := uuid.New()
	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Brom",
		DiscordUserID: "user456",
	}

	body, _ := json.Marshal(ASIDenyRequest{
		CharacterID: charID,
		Reason:      "Choose a different ability",
	})

	r := chi.NewRouter()
	r.Post("/api/levelup/asi/deny", h.HandleDenyASI)

	req := httptest.NewRequest(http.MethodPost, "/api/levelup/asi/deny", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandler_HandleApplyFeat_Success(t *testing.T) {
	h, charStore, _, _ := setupTestHandler(t)

	charID := uuid.New()
	classes := []character.ClassEntry{{Class: "fighter", Level: 4}}
	classesJSON, _ := json.Marshal(classes)
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}

	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Cira",
		DiscordUserID: "user789",
		Level:         4,
		Classes:       classesJSON,
		AbilityScores: mustJSON(t, scores),
		Features:      mustJSON(t, []character.Feature{}),
	}

	body, _ := json.Marshal(FeatApplyRequest{
		CharacterID: charID,
		Feat: FeatInfo{
			ID:   "alert",
			Name: "Alert",
		},
	})

	r := chi.NewRouter()
	r.Post("/api/levelup/feat/apply", h.HandleApplyFeat)

	req := httptest.NewRequest(http.MethodPost, "/api/levelup/feat/apply", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandler_HandleCheckFeatPrereqs(t *testing.T) {
	h, _, _, _ := setupTestHandler(t)

	body, _ := json.Marshal(FeatPrereqCheckRequest{
		Prerequisites: FeatPrerequisites{
			Ability: map[string]int{"dex": 13},
		},
		Scores: character.AbilityScores{STR: 10, DEX: 14, CON: 10, INT: 10, WIS: 10, CHA: 10},
	})

	r := chi.NewRouter()
	r.Post("/api/levelup/feat/check", h.HandleCheckFeatPrereqs)

	req := httptest.NewRequest(http.MethodPost, "/api/levelup/feat/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp FeatPrereqCheckResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Eligible {
		t.Error("expected eligible = true")
	}
}

// Ensure RegisterRoutes sets up routes correctly and they respond
func TestRegisterRoutes(t *testing.T) {
	charStore := newMockCharacterStore()
	classStore := newMockClassStore()
	notifier := &mockNotifier{}
	svc := NewService(charStore, classStore, notifier)
	h := NewHandler(svc, nil)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	// Test that route exists by sending a request (should get 400 for bad body, not 404)
	req := httptest.NewRequest(http.MethodPost, "/api/levelup", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Error("expected route to be registered, got 404")
	}
}

func TestHandler_HandleApproveASI_InvalidBody(t *testing.T) {
	h, _, _, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/levelup/asi/approve", bytes.NewReader([]byte("bad")))
	w := httptest.NewRecorder()
	h.HandleApproveASI(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_HandleDenyASI_InvalidBody(t *testing.T) {
	h, _, _, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/levelup/asi/deny", bytes.NewReader([]byte("bad")))
	w := httptest.NewRecorder()
	h.HandleDenyASI(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_HandleApplyFeat_InvalidBody(t *testing.T) {
	h, _, _, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/levelup/feat/apply", bytes.NewReader([]byte("bad")))
	w := httptest.NewRecorder()
	h.HandleApplyFeat(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_HandleCheckFeatPrereqs_InvalidBody(t *testing.T) {
	h, _, _, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/levelup/feat/check", bytes.NewReader([]byte("bad")))
	w := httptest.NewRecorder()
	h.HandleCheckFeatPrereqs(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_HandleLevelUp_MissingFields(t *testing.T) {
	h, _, _, _ := setupTestHandler(t)

	body, _ := json.Marshal(LevelUpRequest{})
	req := httptest.NewRequest(http.MethodPost, "/api/levelup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleLevelUp(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_HandleApproveASI_CharNotFound(t *testing.T) {
	h, _, _, _ := setupTestHandler(t)

	body, _ := json.Marshal(ASIApprovalRequest{
		CharacterID: uuid.New(),
		Choice:      ASIChoice{Type: ASIPlus2, Ability: "str"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/levelup/asi/approve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleApproveASI(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandler_HandleDenyASI_CharNotFound(t *testing.T) {
	h, _, _, _ := setupTestHandler(t)

	body, _ := json.Marshal(ASIDenyRequest{
		CharacterID: uuid.New(),
		Reason:      "test",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/levelup/asi/deny", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleDenyASI(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandler_HandleApplyFeat_CharNotFound(t *testing.T) {
	h, _, _, _ := setupTestHandler(t)

	body, _ := json.Marshal(FeatApplyRequest{
		CharacterID: uuid.New(),
		Feat:        FeatInfo{ID: "alert", Name: "Alert"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/levelup/feat/apply", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleApplyFeat(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// Ensure the handler works via middleware auth context
func TestHandler_HandleLevelUp_WithAuthContext(t *testing.T) {
	h, charStore, classStore, _ := setupTestHandler(t)

	charID := uuid.New()
	classes := []character.ClassEntry{{Class: "fighter", Level: 5}}
	classesJSON, _ := json.Marshal(classes)
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}

	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Aria",
		DiscordUserID: "user123",
		Level:         5,
		HPMax:         44,
		HPCurrent:     44,
		Classes:       classesJSON,
		AbilityScores: mustJSON(t, scores),
	}

	classStore.classes["fighter"] = &ClassRefData{
		HitDie:           "d10",
		AttacksPerAction: map[int]int{1: 1, 5: 2, 11: 3, 20: 4},
		SubclassLevel:    3,
	}

	body, _ := json.Marshal(LevelUpRequest{
		CharacterID: charID,
		ClassID:     "fighter",
		NewLevel:    6,
	})

	// Use context with value (simulating auth middleware)
	req := httptest.NewRequest(http.MethodPost, "/api/levelup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), "dm_user_id", "dm123"))
	w := httptest.NewRecorder()

	h.HandleLevelUp(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}
}
