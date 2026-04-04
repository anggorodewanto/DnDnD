package portal_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/portal"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeCharacterSheet_TemplateError(t *testing.T) {
	svc := &fakeCharacterSheetService{
		data: &portal.CharacterSheetData{
			Name:             "Thorn",
			AbilityModifiers: map[string]int{"STR": 0, "DEX": 0, "CON": 0, "INT": 0, "WIS": 0, "CHA": 0},
		},
	}
	h := portal.NewCharacterSheetHandler(slog.Default(), svc)
	h.SetSheetTemplate(template.Must(template.New("broken").Funcs(template.FuncMap{
		"fail": func() (string, error) { return "", fmt.Errorf("forced template error") },
	}).Parse(`{{fail}}`)))

	rec := httptest.NewRecorder()
	req := newCharacterSheetRequest("char-1", "user-123")

	h.ServeCharacterSheet(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestServeCharacterSheet_NilLogger(t *testing.T) {
	h := portal.NewCharacterSheetHandler(nil, nil)
	assert.NotNil(t, h)
}

func TestCharacterSheetStoreAdapter_GetCharacterOwner_PCNotFound(t *testing.T) {
	charID := uuid.New()
	campID := uuid.New()

	q := &mockCharacterQuerier{
		character: refdata.Character{ID: charID, CampaignID: campID},
		pcErr:     fmt.Errorf("not found"),
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	_, err := store.GetCharacterOwner(context.Background(), charID.String())

	require.Error(t, err)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_WithSpellSlots(t *testing.T) {
	charID := uuid.New()
	campID := uuid.New()

	scores := character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 16, WIS: 10, CHA: 10}
	scoresJSON, _ := json.Marshal(scores)
	classes := []character.ClassEntry{{Class: "Wizard", Level: 5}}
	classesJSON, _ := json.Marshal(classes)

	spellSlots := map[string]character.SlotInfo{"1": {Current: 3, Max: 4}, "2": {Current: 2, Max: 3}}
	spellSlotsJSON, _ := json.Marshal(spellSlots)

	pactSlots := character.PactMagicSlots{SlotLevel: 3, Current: 2, Max: 2}
	pactSlotsJSON, _ := json.Marshal(pactSlots)

	featureUses := map[string]character.FeatureUse{"Arcane Recovery": {Current: 1, Max: 1, Recharge: "long"}}
	featureUsesJSON, _ := json.Marshal(featureUses)

	hitDice := map[string]int{"wizard": 5}
	hitDiceJSON, _ := json.Marshal(hitDice)

	attunement := []character.AttunementSlot{{ItemID: "wand-1", Name: "Wand of Fire"}}
	attunementJSON, _ := json.Marshal(attunement)

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:               charID,
			CampaignID:       campID,
			Name:             "Gandalf",
			Race:             "Elf",
			Level:            5,
			Classes:          classesJSON,
			AbilityScores:    scoresJSON,
			HpMax:            22,
			HpCurrent:        22,
			Ac:               12,
			SpeedFt:          30,
			ProficiencyBonus: 3,
			SpellSlots:       pqtype.NullRawMessage{RawMessage: spellSlotsJSON, Valid: true},
			PactMagicSlots:   pqtype.NullRawMessage{RawMessage: pactSlotsJSON, Valid: true},
			FeatureUses:      pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
			HitDiceRemaining: hitDiceJSON,
			AttunementSlots:  pqtype.NullRawMessage{RawMessage: attunementJSON, Valid: true},
			Gold:             100,
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	assert.Equal(t, "Gandalf", data.Name)
	assert.NotNil(t, data.SpellSlots)
	assert.Equal(t, 3, data.SpellSlots["1"].Current)
	assert.NotNil(t, data.PactMagicSlots)
	assert.Equal(t, 3, data.PactMagicSlots.SlotLevel)
	assert.NotNil(t, data.FeatureUses)
	assert.Equal(t, 1, data.FeatureUses["Arcane Recovery"].Current)
	assert.Equal(t, 5, data.HitDiceRemaining["wizard"])
	assert.Len(t, data.AttunementSlots, 1)
	assert.Equal(t, "Wand of Fire", data.AttunementSlots[0].Name)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_EquippedItems(t *testing.T) {
	charID := uuid.New()
	scoresJSON, _ := json.Marshal(character.AbilityScores{})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 1}})

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:               charID,
			Name:             "Tank",
			Level:            1,
			Classes:          classesJSON,
			AbilityScores:    scoresJSON,
			EquippedMainHand: newNullString("Longsword"),
			EquippedOffHand:  newNullString("Shield"),
			EquippedArmor:    newNullString("Chain Mail"),
			AcFormula:        newNullString("10 + DEX + CON"),
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	assert.Equal(t, "Longsword", data.EquippedMainHand)
	assert.Equal(t, "Shield", data.EquippedOffHand)
	assert.Equal(t, "Chain Mail", data.EquippedArmor)
	assert.Equal(t, "10 + DEX + CON", data.ACFormula)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_EmptyJSONB(t *testing.T) {
	charID := uuid.New()
	scoresJSON, _ := json.Marshal(character.AbilityScores{})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 1}})

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Empty",
			Level:         1,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	assert.Nil(t, data.SpellSlots)
	assert.Nil(t, data.PactMagicSlots)
	assert.Nil(t, data.FeatureUses)
	assert.Nil(t, data.Features)
	assert.Nil(t, data.Inventory)
	assert.Nil(t, data.AttunementSlots)
}

func TestLoadCharacterSheet_EmptyProficiencies(t *testing.T) {
	store := &mockCharacterSheetStore{
		ownerID: "user-123",
		character: &portal.CharacterSheetData{
			Name:          "Simple",
			Level:         1,
			AbilityScores: character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10},
		},
	}

	svc := portal.NewCharacterSheetService(store)
	data, err := svc.LoadCharacterSheet(context.Background(), "char-1", "user-123")

	require.NoError(t, err)
	assert.Len(t, data.Skills, 18)
	assert.Len(t, data.SavingThrows, 6)
	for _, s := range data.Skills {
		assert.Equal(t, 0, s.Modifier)
		assert.False(t, s.Proficient)
	}
}

func TestServeCharacterSheet_FullRender(t *testing.T) {
	svc := &fakeCharacterSheetService{
		data: &portal.CharacterSheetData{
			ID:               "char-1",
			Name:             "Gandalf",
			Race:             "Elf",
			Level:            5,
			ProficiencyBonus: 3,
			Classes: []character.ClassEntry{
				{Class: "Wizard", Level: 5},
			},
			AbilityScores:    character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10},
			HpMax:            22,
			HpCurrent:        18,
			TempHP:           5,
			AC:               15,
			SpeedFt:          30,
			EquippedMainHand: "Staff",
			EquippedArmor:    "Mage Armor",
			Gold:             200,
			Languages:        []string{"Common", "Elvish", "Draconic"},
			Features: []character.Feature{
				{Name: "Arcane Recovery", Source: "Wizard", Level: 1, Description: "Recover spell slots", MechanicalEffect: "Recover slots = ceil(level/2)"},
			},
			SpellSlots:     map[string]character.SlotInfo{"1": {Current: 3, Max: 4}, "2": {Current: 2, Max: 3}},
			PactMagicSlots: &character.PactMagicSlots{SlotLevel: 3, Current: 2, Max: 2},
			Inventory: []character.InventoryItem{
				{Name: "Staff", Quantity: 1, Equipped: true, Type: "weapon"},
				{Name: "Wand of Fireballs", Quantity: 1, IsMagic: true, Rarity: "rare"},
				{Name: "Potion of Healing", Quantity: 3, Type: "consumable"},
			},
			AttunementSlots: []character.AttunementSlot{
				{ItemID: "wand-fb", Name: "Wand of Fireballs"},
			},
			AbilityModifiers: map[string]int{"STR": -1, "DEX": 2, "CON": 1, "INT": 4, "WIS": 1, "CHA": 0},
			Skills: []portal.SkillDisplay{
				{Name: "Arcana", Ability: "INT", Modifier: 7, Proficient: true},
				{Name: "Athletics", Ability: "STR", Modifier: -1, Proficient: false},
			},
			SavingThrows: []portal.SavingThrowDisplay{
				{Ability: "STR", Modifier: -1, Proficient: false},
				{Ability: "INT", Modifier: 7, Proficient: true},
			},
			ClassSummary: "Wizard 5",
		},
	}

	h := portal.NewCharacterSheetHandler(slog.Default(), svc)
	rec := httptest.NewRecorder()
	req := newCharacterSheetRequest("char-1", "user-123")

	h.ServeCharacterSheet(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	// Character header
	assert.Contains(t, body, "Gandalf")
	assert.Contains(t, body, "Wizard 5")
	assert.Contains(t, body, "Elf")
	// Stats
	assert.Contains(t, body, "18/22")
	assert.Contains(t, body, "+5 temp")
	assert.Contains(t, body, "15")
	assert.Contains(t, body, "200gp")
	// Equipment
	assert.Contains(t, body, "Staff")
	assert.Contains(t, body, "Mage Armor")
	// Languages
	assert.Contains(t, body, "Common")
	assert.Contains(t, body, "Draconic")
	// Features
	assert.Contains(t, body, "Arcane Recovery")
	assert.Contains(t, body, "Recover spell slots")
	assert.Contains(t, body, "Recover slots = ceil(level/2)")
	// Spell slots
	assert.Contains(t, body, "Level 1")
	assert.Contains(t, body, "3/4")
	// Pact magic
	assert.Contains(t, body, "Pact Magic")
	assert.Contains(t, body, "Level 3 Pact Slots")
	// Inventory
	assert.Contains(t, body, "Wand of Fireballs")
	assert.Contains(t, body, "rare")
	assert.Contains(t, body, "Equipped")
	assert.Contains(t, body, "x3")
	// Attunement
	assert.Contains(t, body, "Attunement")
}

func TestRegisterRoutes_CharacterSheet_NotMountedWithoutOption(t *testing.T) {
	r := chi.NewRouter()
	h := portal.NewHandler(slog.Default(), nil)
	portal.RegisterRoutes(r, h, fakeAuthMiddleware)

	req := httptest.NewRequest(http.MethodGet, "/portal/character/abc-123", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Without WithCharacterSheet, should get 405 or 404
	assert.True(t, rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed)
}

func newNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}
