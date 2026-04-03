package itempicker_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"database/sql"
	"errors"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/itempicker"
	"github.com/ab/dndnd/internal/refdata"
)

// stubStore implements itempicker.Store for unit tests.
type stubStore struct {
	weapons      []refdata.Weapon
	armor        []refdata.Armor
	magicItems   []refdata.MagicItem
	combatants   []refdata.Combatant
	characters   map[uuid.UUID]refdata.Character
	weaponErr    error
	armorErr     error
	magicErr     error
	combatantErr error
	characterErr error
}

func (s *stubStore) ListWeapons(ctx context.Context) ([]refdata.Weapon, error) {
	return s.weapons, s.weaponErr
}

func (s *stubStore) ListArmor(ctx context.Context) ([]refdata.Armor, error) {
	return s.armor, s.armorErr
}

func (s *stubStore) ListMagicItems(ctx context.Context) ([]refdata.MagicItem, error) {
	return s.magicItems, s.magicErr
}

func (s *stubStore) ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return s.combatants, s.combatantErr
}

func (s *stubStore) GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
	if s.characterErr != nil {
		return refdata.Character{}, s.characterErr
	}
	if s.characters != nil {
		if c, ok := s.characters[id]; ok {
			return c, nil
		}
	}
	return refdata.Character{}, errors.New("not found")
}

func chiCtx(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestHandleSearch_ReturnsArmor(t *testing.T) {
	store := &stubStore{
		armor: []refdata.Armor{
			{ID: "chain-mail", Name: "Chain Mail", AcBase: 16, ArmorType: "heavy"},
			{ID: "leather", Name: "Leather Armor", AcBase: 11, ArmorType: "light"},
		},
	}
	h := itempicker.NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/search?q=chain", nil)
	rec := httptest.NewRecorder()
	h.HandleSearch(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var results []itempicker.SearchResult
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&results))
	require.Len(t, results, 1)
	assert.Equal(t, "chain-mail", results[0].ID)
	assert.Equal(t, "armor", results[0].Type)
}

func TestHandleSearch_ReturnsMagicItems(t *testing.T) {
	store := &stubStore{
		magicItems: []refdata.MagicItem{
			{ID: "cloak-of-protection", Name: "Cloak of Protection", Rarity: "uncommon", Description: "You gain a +1 bonus to AC"},
		},
	}
	h := itempicker.NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/search?q=cloak", nil)
	rec := httptest.NewRecorder()
	h.HandleSearch(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var results []itempicker.SearchResult
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&results))
	require.Len(t, results, 1)
	assert.Equal(t, "cloak-of-protection", results[0].ID)
	assert.Equal(t, "magic_item", results[0].Type)
	assert.Equal(t, "You gain a +1 bonus to AC", results[0].Description)
}

func TestHandleSearch_CategoryFilter(t *testing.T) {
	store := &stubStore{
		weapons: []refdata.Weapon{
			{ID: "longsword", Name: "Longsword"},
		},
		armor: []refdata.Armor{
			{ID: "leather", Name: "Leather Armor"},
		},
		magicItems: []refdata.MagicItem{
			{ID: "cloak", Name: "Cloak of Protection"},
		},
	}
	h := itempicker.NewHandler(store)

	// Filter to weapons only
	req := httptest.NewRequest(http.MethodGet, "/search?category=weapons", nil)
	rec := httptest.NewRecorder()
	h.HandleSearch(rec, req)

	var results []itempicker.SearchResult
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&results))
	require.Len(t, results, 1)
	assert.Equal(t, "weapon", results[0].Type)

	// Filter to armor only
	req = httptest.NewRequest(http.MethodGet, "/search?category=armor", nil)
	rec = httptest.NewRecorder()
	h.HandleSearch(rec, req)

	results = nil
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&results))
	require.Len(t, results, 1)
	assert.Equal(t, "armor", results[0].Type)

	// Filter to magic_items only
	req = httptest.NewRequest(http.MethodGet, "/search?category=magic_items", nil)
	rec = httptest.NewRecorder()
	h.HandleSearch(rec, req)

	results = nil
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&results))
	require.Len(t, results, 1)
	assert.Equal(t, "magic_item", results[0].Type)
}

func TestHandleSearch_NoQuery_ReturnsAll(t *testing.T) {
	store := &stubStore{
		weapons: []refdata.Weapon{
			{ID: "longsword", Name: "Longsword"},
		},
		armor: []refdata.Armor{
			{ID: "leather", Name: "Leather Armor"},
		},
		magicItems: []refdata.MagicItem{
			{ID: "cloak", Name: "Cloak of Protection"},
		},
	}
	h := itempicker.NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	rec := httptest.NewRecorder()
	h.HandleSearch(rec, req)

	var results []itempicker.SearchResult
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&results))
	assert.Len(t, results, 3)
}

func TestHandleSearch_EmptyResults(t *testing.T) {
	store := &stubStore{}
	h := itempicker.NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/search?q=nonexistent", nil)
	rec := httptest.NewRecorder()
	h.HandleSearch(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var results []itempicker.SearchResult
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&results))
	assert.Len(t, results, 0)
}

func TestHandleSearch_WeaponError(t *testing.T) {
	store := &stubStore{
		weaponErr: assert.AnError,
	}
	h := itempicker.NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	rec := httptest.NewRecorder()
	h.HandleSearch(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleSearch_ArmorError(t *testing.T) {
	store := &stubStore{
		armorErr: assert.AnError,
	}
	h := itempicker.NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	rec := httptest.NewRecorder()
	h.HandleSearch(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleSearch_MagicItemError(t *testing.T) {
	store := &stubStore{
		magicErr: assert.AnError,
	}
	h := itempicker.NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	rec := httptest.NewRecorder()
	h.HandleSearch(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleSearch_CaseInsensitive(t *testing.T) {
	store := &stubStore{
		weapons: []refdata.Weapon{
			{ID: "longsword", Name: "Longsword"},
		},
	}
	h := itempicker.NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/search?q=LONG", nil)
	rec := httptest.NewRecorder()
	h.HandleSearch(rec, req)

	var results []itempicker.SearchResult
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&results))
	assert.Len(t, results, 1)
}

func TestHandleSearch_ReturnsWeapons(t *testing.T) {
	store := &stubStore{
		weapons: []refdata.Weapon{
			{ID: "longsword", Name: "Longsword", Damage: "1d8", DamageType: "slashing", WeaponType: "martial_melee"},
			{ID: "shortbow", Name: "Shortbow", Damage: "1d6", DamageType: "piercing", WeaponType: "simple_ranged"},
		},
	}
	h := itempicker.NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/campaigns/abc/items/search?q=sword", nil)
	req = chiCtx(req, map[string]string{"campaignID": "abc"})
	rec := httptest.NewRecorder()

	h.HandleSearch(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var results []itempicker.SearchResult
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&results))
	require.Len(t, results, 1)
	assert.Equal(t, "longsword", results[0].ID)
	assert.Equal(t, "Longsword", results[0].Name)
	assert.Equal(t, "weapon", results[0].Type)
}

func TestHandleCreatureInventories_ReturnsItems(t *testing.T) {
	charID := uuid.New()
	invJSON := `[{"item_id":"shortsword","name":"Shortsword","quantity":2,"type":"weapon"}]`

	store := &stubStore{
		combatants: []refdata.Combatant{
			{
				ID:          uuid.New(),
				DisplayName: "Goblin",
				IsNpc:       true,
				IsAlive:     false,
				CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			},
		},
		characters: map[uuid.UUID]refdata.Character{
			charID: {
				ID:   charID,
				Name: "Goblin",
				Gold: 15,
				Inventory: pqtype.NullRawMessage{
					RawMessage: []byte(invJSON),
					Valid:      true,
				},
			},
		},
	}
	h := itempicker.NewHandler(store)

	encID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": encID.String()})
	rec := httptest.NewRecorder()
	h.HandleCreatureInventories(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result itempicker.CreatureInventoriesResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	require.Len(t, result.Creatures, 1)
	assert.Equal(t, "Goblin", result.Creatures[0].Name)
	assert.Len(t, result.Creatures[0].Items, 1)
	assert.Equal(t, int32(15), result.Creatures[0].Gold)
}

func TestHandleCreatureInventories_InvalidEncounterID(t *testing.T) {
	store := &stubStore{}
	h := itempicker.NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": "bad-uuid"})
	rec := httptest.NewRecorder()
	h.HandleCreatureInventories(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCreatureInventories_CombatantError(t *testing.T) {
	store := &stubStore{
		combatantErr: assert.AnError,
	}
	h := itempicker.NewHandler(store)

	encID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": encID.String()})
	rec := httptest.NewRecorder()
	h.HandleCreatureInventories(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleCreatureInventories_SkipsAliveAndPC(t *testing.T) {
	store := &stubStore{
		combatants: []refdata.Combatant{
			{
				ID:          uuid.New(),
				DisplayName: "Alive Goblin",
				IsNpc:       true,
				IsAlive:     true, // alive, should be skipped
				CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
			},
			{
				ID:          uuid.New(),
				DisplayName: "Player",
				IsNpc:       false, // PC, should be skipped
				IsAlive:     false,
				CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
			},
			{
				ID:          uuid.New(),
				DisplayName: "No Character",
				IsNpc:       true,
				IsAlive:     false,
				CharacterID: uuid.NullUUID{}, // no character, should be skipped
			},
		},
	}
	h := itempicker.NewHandler(store)

	encID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": encID.String()})
	rec := httptest.NewRecorder()
	h.HandleCreatureInventories(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result itempicker.CreatureInventoriesResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.Len(t, result.Creatures, 0)
}

func TestHandleCreatureInventories_CharacterNotFound(t *testing.T) {
	charID := uuid.New()
	store := &stubStore{
		combatants: []refdata.Combatant{
			{
				ID:          uuid.New(),
				DisplayName: "Ghost",
				IsNpc:       true,
				IsAlive:     false,
				CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			},
		},
		// no characters map -- GetCharacter will return error
	}
	h := itempicker.NewHandler(store)

	encID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": encID.String()})
	rec := httptest.NewRecorder()
	h.HandleCreatureInventories(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result itempicker.CreatureInventoriesResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	// Character not found is gracefully skipped
	assert.Len(t, result.Creatures, 0)
}

func TestHandleCreatureInventories_InvalidInventoryJSON(t *testing.T) {
	charID := uuid.New()
	store := &stubStore{
		combatants: []refdata.Combatant{
			{
				ID:          uuid.New(),
				DisplayName: "BadInv",
				IsNpc:       true,
				IsAlive:     false,
				CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			},
		},
		characters: map[uuid.UUID]refdata.Character{
			charID: {
				ID:   charID,
				Name: "BadInv",
				Gold: 5,
				Inventory: pqtype.NullRawMessage{
					RawMessage: []byte(`not-json`),
					Valid:      true,
				},
			},
		},
	}
	h := itempicker.NewHandler(store)

	encID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": encID.String()})
	rec := httptest.NewRecorder()
	h.HandleCreatureInventories(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result itempicker.CreatureInventoriesResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	// Creature still returned but with empty items and gold
	assert.Len(t, result.Creatures, 1)
	assert.Equal(t, "BadInv", result.Creatures[0].Name)
	assert.Equal(t, int32(5), result.Creatures[0].Gold)
}

func TestHandleSearch_ArmorCategoryError(t *testing.T) {
	store := &stubStore{
		armorErr: assert.AnError,
	}
	h := itempicker.NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/search?category=armor", nil)
	rec := httptest.NewRecorder()
	h.HandleSearch(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleSearch_MagicItemCategoryError(t *testing.T) {
	store := &stubStore{
		magicErr: assert.AnError,
	}
	h := itempicker.NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/search?category=magic_items", nil)
	rec := httptest.NewRecorder()
	h.HandleSearch(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleSearch_WeaponCategoryError(t *testing.T) {
	store := &stubStore{
		weaponErr: assert.AnError,
	}
	h := itempicker.NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/search?category=weapons", nil)
	rec := httptest.NewRecorder()
	h.HandleSearch(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleSearch_UnknownCategory_ReturnsEmpty(t *testing.T) {
	store := &stubStore{
		weapons: []refdata.Weapon{{ID: "longsword", Name: "Longsword"}},
	}
	h := itempicker.NewHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/search?category=potions", nil)
	rec := httptest.NewRecorder()
	h.HandleSearch(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var results []itempicker.SearchResult
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&results))
	assert.Len(t, results, 0)
}

// Ensure unused imports are satisfied.
var _ = sql.NullString{}
