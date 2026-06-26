package portal_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/portal"
	"github.com/stretchr/testify/assert"
)

// TestServeCharacterSheet_PossibleActions verifies the rendered sheet derives
// the "Possible Actions" section from the character's classes (template-prep in
// renderSheet), listing universal actions for everyone and the class-gated
// ability with its real Discord command.
func TestServeCharacterSheet_PossibleActions(t *testing.T) {
	svc := &fakeCharacterSheetService{
		data: &portal.CharacterSheetData{
			ID:               "char-1",
			Name:             "Grog",
			Race:             "Half-Orc",
			Level:            3,
			ProficiencyBonus: 2,
			Classes:          []character.ClassEntry{{Class: "Barbarian", Level: 3}},
			AbilityScores:    character.AbilityScores{STR: 18, DEX: 14, CON: 16, INT: 8, WIS: 10, CHA: 12},
			HpMax:            38,
			HpCurrent:        38,
			AC:               15,
			SpeedFt:          40,
			ClassSummary:     "Barbarian 3",
		},
	}

	h := portal.NewCharacterSheetHandler(slog.Default(), svc)
	rec := httptest.NewRecorder()
	req := newCharacterSheetRequest("char-1", "user-123")

	h.ServeCharacterSheet(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	// Section heading + economy group labels.
	assert.Contains(t, body, "<h3>Actions</h3>")
	assert.Contains(t, body, "Bonus Actions")
	assert.Contains(t, body, "Reactions")

	// Universal action available to everyone.
	assert.Contains(t, body, "Dash")
	assert.Contains(t, body, "/action dash")

	// Class-gated ability with its real command and class tag.
	assert.Contains(t, body, "Rage")
	assert.Contains(t, body, "/bonus rage")
	assert.Contains(t, body, "Barbarian")
}

// TestServeCharacterSheet_PossibleActions_NoClassLeak asserts a spellcaster's
// sheet does not advertise martial class abilities it never earned.
func TestServeCharacterSheet_PossibleActions_NoClassLeak(t *testing.T) {
	svc := &fakeCharacterSheetService{
		data: &portal.CharacterSheetData{
			ID:               "char-2",
			Name:             "Mirena",
			Race:             "Elf",
			Level:            3,
			ProficiencyBonus: 2,
			Classes:          []character.ClassEntry{{Class: "Wizard", Level: 3}},
			AbilityScores:    character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 17, WIS: 12, CHA: 10},
			HpMax:            20,
			HpCurrent:        20,
			AC:               12,
			SpeedFt:          30,
			ClassSummary:     "Wizard 3",
		},
	}

	h := portal.NewCharacterSheetHandler(slog.Default(), svc)
	rec := httptest.NewRecorder()
	req := newCharacterSheetRequest("char-2", "user-123")

	h.ServeCharacterSheet(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, "<h3>Actions</h3>")
	assert.Contains(t, body, "Cast a Spell")
	assert.NotContains(t, body, "/bonus rage")
	assert.NotContains(t, body, "Cunning Action")
}
