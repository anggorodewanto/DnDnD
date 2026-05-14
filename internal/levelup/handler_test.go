package levelup

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	if resp.HPGained <= 0 {
		t.Errorf("resp.HPGained = %d, want > 0", resp.HPGained)
	}
	if resp.NewAttacksPerAction != 2 {
		t.Errorf("resp.NewAttacksPerAction = %d, want 2", resp.NewAttacksPerAction)
	}
	// Fighter at level 5 didn't have a subclass set, so even at level 6
	// the subclass is still needed (subclassLevel=3, hasSubclass=false).
	if !resp.NeedsSubclass {
		t.Error("resp.NeedsSubclass = false, want true (fighter has no subclass selected)")
	}
}

func TestHandler_HandleLevelUp_NeedsSubclass(t *testing.T) {
	h, charStore, classStore, _ := setupTestHandler(t)

	charID := uuid.New()
	classes := []character.ClassEntry{{Class: "fighter", Level: 2}}
	classesJSON, _ := json.Marshal(classes)
	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 12, CHA: 8}

	charStore.chars[charID] = &StoredCharacter{
		ID:            charID,
		Name:          "Brom",
		DiscordUserID: "user456",
		Level:         2,
		HPMax:         22,
		HPCurrent:     22,
		Classes:       classesJSON,
		AbilityScores: mustJSON(t, scores),
	}

	classStore.classes["fighter"] = &ClassRefData{
		HitDie:           "d10",
		AttacksPerAction: map[int]int{1: 1, 5: 2},
		SubclassLevel:    3,
	}

	body, _ := json.Marshal(LevelUpRequest{
		CharacterID: charID,
		ClassID:     "fighter",
		NewLevel:    3,
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
	if !resp.NeedsSubclass {
		t.Error("resp.NeedsSubclass = false, want true (fighter at level 3 needs subclass)")
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

func TestHandler_HandleApplyFeat_RejectsMissingRequiredFeatChoices(t *testing.T) {
	tests := []struct {
		name string
		feat FeatInfo
	}{
		{
			name: "resilient",
			feat: FeatInfo{ID: "resilient", Name: "Resilient"},
		},
		{
			name: "skilled",
			feat: FeatInfo{ID: "skilled", Name: "Skilled"},
		},
		{
			name: "elemental adept",
			feat: FeatInfo{ID: "elemental-adept", Name: "Elemental Adept"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, charStore, _, _ := setupTestHandler(t)
			charID := uuid.New()
			charStore.chars[charID] = &StoredCharacter{
				ID:            charID,
				Name:          "Cira",
				AbilityScores: mustJSON(t, character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10}),
				Features:      mustJSON(t, []character.Feature{}),
				Proficiencies: mustJSON(t, character.Proficiencies{}),
			}

			body, _ := json.Marshal(FeatApplyRequest{
				CharacterID: charID,
				Feat:        tt.feat,
			})
			req := httptest.NewRequest(http.MethodPost, "/api/levelup/feat/apply", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.HandleApplyFeat(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
			}

			var features []character.Feature
			if err := json.Unmarshal(charStore.chars[charID].Features, &features); err != nil {
				t.Fatalf("unmarshal features: %v", err)
			}
			if len(features) != 0 {
				t.Fatalf("expected no feature appended, got %+v", features)
			}
		})
	}
}

func TestHandler_HandleApplyFeat_RejectsInvalidRequiredFeatChoices(t *testing.T) {
	tests := []struct {
		name string
		feat FeatInfo
	}{
		{
			name: "resilient invalid ability",
			feat: FeatInfo{ID: "resilient", Name: "Resilient", Choices: FeatChoices{Ability: "luck"}},
		},
		{
			name: "skilled invalid skill",
			feat: FeatInfo{ID: "skilled", Name: "Skilled", Choices: FeatChoices{Skills: []string{"arcana", "history", "luck"}}},
		},
		{
			name: "elemental adept invalid damage type",
			feat: FeatInfo{ID: "elemental-adept", Name: "Elemental Adept", Choices: FeatChoices{DamageType: "force"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, charStore, _, _ := setupTestHandler(t)
			charID := uuid.New()
			charStore.chars[charID] = &StoredCharacter{
				ID:            charID,
				Name:          "Cira",
				AbilityScores: mustJSON(t, character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10}),
				Features:      mustJSON(t, []character.Feature{}),
				Proficiencies: mustJSON(t, character.Proficiencies{}),
			}

			body, _ := json.Marshal(FeatApplyRequest{
				CharacterID: charID,
				Feat:        tt.feat,
			})
			req := httptest.NewRequest(http.MethodPost, "/api/levelup/feat/apply", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.HandleApplyFeat(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
			}
		})
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

func TestHandler_ServeLevelUpPage(t *testing.T) {
	h, _, _, _ := setupTestHandler(t)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/levelup", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Level Up") {
		t.Error("expected 'Level Up' in page body")
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
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
