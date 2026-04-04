package portal_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/portal"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

// fakeCharacterSheetService implements the interface needed by the handler.
type fakeCharacterSheetService struct {
	data *portal.CharacterSheetData
	err  error
}

func (f *fakeCharacterSheetService) LoadCharacterSheet(_ context.Context, characterID, userID string) (*portal.CharacterSheetData, error) {
	return f.data, f.err
}

func newCharacterSheetRequest(charID, userID string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/portal/character/"+charID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("characterID", charID)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	if userID != "" {
		ctx = auth.ContextWithDiscordUserID(ctx, userID)
	}
	return req.WithContext(ctx)
}

func TestServeCharacterSheet_Success(t *testing.T) {
	svc := &fakeCharacterSheetService{
		data: &portal.CharacterSheetData{
			ID:               "char-1",
			Name:             "Thorn",
			Race:             "Human",
			Level:            5,
			ProficiencyBonus: 3,
			Classes: []character.ClassEntry{
				{Class: "Fighter", Level: 5},
			},
			AbilityScores:    character.AbilityScores{STR: 16, DEX: 14, CON: 12, INT: 10, WIS: 8, CHA: 13},
			HpMax:            42,
			HpCurrent:        35,
			AC:               18,
			SpeedFt:          30,
			Languages:        []string{"Common", "Elvish"},
			AbilityModifiers: map[string]int{"STR": 3, "DEX": 2, "CON": 1, "INT": 0, "WIS": -1, "CHA": 1},
			ClassSummary:     "Fighter 5",
		},
	}

	h := portal.NewCharacterSheetHandler(slog.Default(), svc)
	rec := httptest.NewRecorder()
	req := newCharacterSheetRequest("char-1", "user-123")

	h.ServeCharacterSheet(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	body := rec.Body.String()
	assert.Contains(t, body, "Thorn")
	assert.Contains(t, body, "Human")
	assert.Contains(t, body, "Fighter 5")
}

func TestServeCharacterSheet_Unauthenticated(t *testing.T) {
	h := portal.NewCharacterSheetHandler(slog.Default(), nil)
	rec := httptest.NewRecorder()
	req := newCharacterSheetRequest("char-1", "")

	h.ServeCharacterSheet(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestServeCharacterSheet_MissingCharacterID(t *testing.T) {
	h := portal.NewCharacterSheetHandler(slog.Default(), nil)
	rec := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodGet, "/portal/character/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("characterID", "")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = auth.ContextWithDiscordUserID(ctx, "user-123")
	req = req.WithContext(ctx)

	h.ServeCharacterSheet(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestServeCharacterSheet_NotOwner(t *testing.T) {
	svc := &fakeCharacterSheetService{
		err: portal.ErrNotOwner,
	}

	h := portal.NewCharacterSheetHandler(slog.Default(), svc)
	rec := httptest.NewRecorder()
	req := newCharacterSheetRequest("char-1", "user-attacker")

	h.ServeCharacterSheet(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestServeCharacterSheet_NotFound(t *testing.T) {
	svc := &fakeCharacterSheetService{
		err: portal.ErrCharacterNotFound,
	}

	h := portal.NewCharacterSheetHandler(slog.Default(), svc)
	rec := httptest.NewRecorder()
	req := newCharacterSheetRequest("char-1", "user-123")

	h.ServeCharacterSheet(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestServeCharacterSheet_InternalError(t *testing.T) {
	svc := &fakeCharacterSheetService{
		err: errors.New("db error"),
	}

	h := portal.NewCharacterSheetHandler(slog.Default(), svc)
	rec := httptest.NewRecorder()
	req := newCharacterSheetRequest("char-1", "user-123")

	h.ServeCharacterSheet(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestServeCharacterSheet_HitDiceRemaining(t *testing.T) {
	svc := &fakeCharacterSheetService{
		data: &portal.CharacterSheetData{
			Name:             "Thorn",
			Race:             "Human",
			Level:            5,
			ClassSummary:     "Fighter 5",
			AbilityModifiers: map[string]int{"STR": 0, "DEX": 0, "CON": 0, "INT": 0, "WIS": 0, "CHA": 0},
			HitDiceRemaining: map[string]int{"d10": 3},
		},
	}

	h := portal.NewCharacterSheetHandler(slog.Default(), svc)
	rec := httptest.NewRecorder()
	req := newCharacterSheetRequest("char-1", "user-123")

	h.ServeCharacterSheet(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Hit Dice")
	assert.Contains(t, body, "d10")
	assert.Contains(t, body, "3")
}

func TestServeCharacterSheet_FeatureUses(t *testing.T) {
	svc := &fakeCharacterSheetService{
		data: &portal.CharacterSheetData{
			Name:             "Thorn",
			Race:             "Human",
			Level:            5,
			ClassSummary:     "Fighter 5",
			AbilityModifiers: map[string]int{"STR": 0, "DEX": 0, "CON": 0, "INT": 0, "WIS": 0, "CHA": 0},
			FeatureUses: map[string]character.FeatureUse{
				"Second Wind": {Current: 0, Max: 1, Recharge: "short rest"},
			},
		},
	}

	h := portal.NewCharacterSheetHandler(slog.Default(), svc)
	rec := httptest.NewRecorder()
	req := newCharacterSheetRequest("char-1", "user-123")

	h.ServeCharacterSheet(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Feature Uses")
	assert.Contains(t, body, "Second Wind")
	assert.Contains(t, body, "0/1")
	assert.Contains(t, body, "short rest")
}

func TestServeCharacterSheet_SpellsSection(t *testing.T) {
	svc := &fakeCharacterSheetService{
		data: &portal.CharacterSheetData{
			Name:             "Gandalf",
			Race:             "Elf",
			Level:            5,
			ClassSummary:     "Wizard 5",
			AbilityModifiers: map[string]int{"STR": 0, "DEX": 0, "CON": 0, "INT": 0, "WIS": 0, "CHA": 0},
			Spells: []portal.SpellDisplayEntry{
				{ID: "fire-bolt", Name: "Fire Bolt", Level: 0, School: "Evocation"},
				{ID: "magic-missile", Name: "Magic Missile", Level: 1, School: "Evocation", Prepared: true},
				{ID: "shield", Name: "Shield", Level: 1, School: "Abjuration"},
				{ID: "fireball", Name: "Fireball", Level: 3, School: "Evocation", Prepared: true},
			},
		},
	}

	h := portal.NewCharacterSheetHandler(slog.Default(), svc)
	rec := httptest.NewRecorder()
	req := newCharacterSheetRequest("char-1", "user-123")

	h.ServeCharacterSheet(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Spell List")
	assert.Contains(t, body, "Fire Bolt")
	assert.Contains(t, body, "Magic Missile")
	assert.Contains(t, body, "Fireball")
	assert.Contains(t, body, "Shield")
}

func TestServeCharacterSheet_NoSpellsSection(t *testing.T) {
	svc := &fakeCharacterSheetService{
		data: &portal.CharacterSheetData{
			Name:             "Fighter",
			Race:             "Human",
			Level:            1,
			ClassSummary:     "Fighter 1",
			AbilityModifiers: map[string]int{"STR": 0, "DEX": 0, "CON": 0, "INT": 0, "WIS": 0, "CHA": 0},
		},
	}

	h := portal.NewCharacterSheetHandler(slog.Default(), svc)
	rec := httptest.NewRecorder()
	req := newCharacterSheetRequest("char-1", "user-123")

	h.ServeCharacterSheet(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	// Spells section should not appear for non-casters
	assert.NotContains(t, body, "Spell List")
}

func TestServeCharacterSheet_SpellsGroupedByLevel(t *testing.T) {
	svc := &fakeCharacterSheetService{
		data: &portal.CharacterSheetData{
			Name:             "Gandalf",
			Race:             "Elf",
			Level:            9,
			ClassSummary:     "Wizard 9",
			AbilityModifiers: map[string]int{"STR": 0, "DEX": 0, "CON": 0, "INT": 0, "WIS": 0, "CHA": 0},
			Spells: []portal.SpellDisplayEntry{
				{Name: "Fire Bolt", Level: 0},
				{Name: "Magic Missile", Level: 1},
				{Name: "Fireball", Level: 3},
				{Name: "Polymorph", Level: 4},
			},
		},
	}

	h := portal.NewCharacterSheetHandler(slog.Default(), svc)
	rec := httptest.NewRecorder()
	req := newCharacterSheetRequest("char-1", "user-123")

	h.ServeCharacterSheet(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Cantrips")
	assert.Contains(t, body, "Level 1")
	assert.Contains(t, body, "Level 3")
	assert.Contains(t, body, "Level 4")
}

func TestRegisterRoutes_CharacterSheet(t *testing.T) {
	r := chi.NewRouter()
	svc := &fakeCharacterSheetService{
		data: &portal.CharacterSheetData{
			Name:             "Thorn",
			Race:             "Human",
			Level:            1,
			ClassSummary:     "Fighter 1",
			AbilityModifiers: map[string]int{"STR": 0, "DEX": 0, "CON": 0, "INT": 0, "WIS": 0, "CHA": 0},
		},
	}
	h := portal.NewHandler(slog.Default(), nil)
	csh := portal.NewCharacterSheetHandler(slog.Default(), svc)
	portal.RegisterRoutes(r, h, fakeAuthMiddleware, portal.WithCharacterSheet(csh))

	req := httptest.NewRequest(http.MethodGet, "/portal/character/abc-123", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Thorn")
}
