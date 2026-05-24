package portal_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/portal"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testCharID = "11111111-1111-1111-1111-111111111111"

// mockPrepareService implements portal.PrepareService.
type mockPrepareService struct {
	info    combat.PreparationInfo
	infoErr error
	result  combat.PrepareSpellsResult
	prepErr error

	gotPrepareInput combat.PrepareSpellsInput
}

func (m *mockPrepareService) GetPreparationInfo(_ context.Context, _ uuid.UUID, _, _ string) (combat.PreparationInfo, error) {
	return m.info, m.infoErr
}

func (m *mockPrepareService) PrepareSpells(_ context.Context, input combat.PrepareSpellsInput) (combat.PrepareSpellsResult, error) {
	m.gotPrepareInput = input
	return m.result, m.prepErr
}

// mockPrepStore implements portal.PreparationStore.
type mockPrepStore struct {
	ownerID  string
	ownerErr error
	char     refdata.Character
	charErr  error
}

func (m *mockPrepStore) GetCharacterOwner(_ context.Context, _ string) (string, error) {
	return m.ownerID, m.ownerErr
}

func (m *mockPrepStore) GetCharacter(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
	return m.char, m.charErr
}

// mockCardUpdater implements portal.CardUpdater and records the characters it
// was asked to refresh.
type mockCardUpdater struct {
	calls []uuid.UUID
	err   error
}

func (m *mockCardUpdater) OnCharacterUpdated(_ context.Context, id uuid.UUID) error {
	m.calls = append(m.calls, id)
	return m.err
}

// errRefDataStore implements portal.RefDataStore and fails ListSpellsByClass.
type errRefDataStore struct{ mockRefDataStore }

func (e *errRefDataStore) ListSpellsByClass(_ context.Context, _, _ string) ([]portal.SpellInfo, error) {
	return nil, errors.New("refdata down")
}

// prepGETResponse mirrors the GET API contract exactly.
type prepGETResponse struct {
	CharacterName       string             `json:"character_name"`
	Class               string             `json:"class"`
	Subclass            string             `json:"subclass"`
	MaxPrepared         int                `json:"max_prepared"`
	CurrentPrepared     []string           `json:"current_prepared"`
	AlwaysPrepared      []string           `json:"always_prepared"`
	AvailableSlotLevels []int              `json:"available_slot_levels"`
	Spells              []portal.SpellInfo `json:"spells"`
}

// prepPOSTResponse mirrors the POST API contract exactly.
type prepPOSTResponse struct {
	PreparedCount  int      `json:"prepared_count"`
	MaxPrepared    int      `json:"max_prepared"`
	AlwaysPrepared []string `json:"always_prepared"`
}

func clericCharacter() refdata.Character {
	id := uuid.MustParse(testCharID)
	return refdata.Character{
		ID:      id,
		Name:    "Brother Aldric",
		Classes: json.RawMessage(`[{"class":"Cleric","subclass":"Life","level":5}]`),
	}
}

func newPrepHandler(svc portal.PrepareService, store portal.PreparationStore, refData portal.RefDataStore) *portal.PreparationHandler {
	return portal.NewPreparationHandler(slog.Default(), svc, store, refData)
}

func newPrepRequest(method, charID, userID string, body []byte) *http.Request {
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, "/portal/api/characters/"+charID+"/preparation", bytes.NewReader(body))
	} else {
		r = httptest.NewRequest(method, "/portal/api/characters/"+charID+"/preparation", nil)
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("characterID", charID)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	if userID != "" {
		ctx = auth.ContextWithDiscordUserID(ctx, userID)
	}
	return r.WithContext(ctx)
}

func TestGetPreparation_Success(t *testing.T) {
	svc := &mockPrepareService{
		info: combat.PreparationInfo{
			MaxPrepared:         8,
			CurrentPrepared:     []string{"bless", "guiding-bolt"},
			AlwaysPrepared:      []string{"cure-wounds"},
			AvailableSlotLevels: map[int]bool{2: true, 1: true},
			// ClassSpells is intentionally pre-filtered; the handler must
			// NOT use it and instead build "spells" from RefDataStore.
			ClassSpells: []refdata.Spell{{ID: "filtered-only"}},
		},
	}
	store := &mockPrepStore{ownerID: "user-1", char: clericCharacter()}
	refData := &mockRefDataStore{
		spells: []portal.SpellInfo{
			{ID: "guidance", Name: "Guidance", Level: 0, School: "Divination", Classes: []string{"cleric"}},
			{ID: "bless", Name: "Bless", Level: 1, School: "Enchantment", Classes: []string{"cleric"}},
			{ID: "guiding-bolt", Name: "Guiding Bolt", Level: 1, School: "Evocation", Classes: []string{"cleric"}},
			{ID: "spiritual-weapon", Name: "Spiritual Weapon", Level: 2, School: "Evocation", Classes: []string{"cleric"}},
		},
	}
	h := newPrepHandler(svc, store, refData)

	rec := httptest.NewRecorder()
	h.GetPreparation(rec, newPrepRequest(http.MethodGet, testCharID, "user-1", nil))

	require.Equal(t, http.StatusOK, rec.Code)

	var got prepGETResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))

	assert.Equal(t, "Brother Aldric", got.CharacterName)
	assert.Equal(t, "cleric", got.Class)
	assert.Equal(t, "life", got.Subclass)
	assert.Equal(t, 8, got.MaxPrepared)
	assert.Equal(t, []string{"bless", "guiding-bolt"}, got.CurrentPrepared)
	assert.Equal(t, []string{"cure-wounds"}, got.AlwaysPrepared)
	assert.Equal(t, []int{1, 2}, got.AvailableSlotLevels) // ascending sorted
	// Full class spell list (all levels, including the cantrip) from RefDataStore.
	require.Len(t, got.Spells, 4)
	assert.Equal(t, "guidance", got.Spells[0].ID)
	assert.Equal(t, 0, got.Spells[0].Level)
	// The pre-filtered ClassSpells entry must NOT leak through.
	for _, s := range got.Spells {
		assert.NotEqual(t, "filtered-only", s.ID)
	}
}

func TestGetPreparation_EmptySlicesMarshalAsArrays(t *testing.T) {
	svc := &mockPrepareService{
		info: combat.PreparationInfo{
			MaxPrepared:         5,
			CurrentPrepared:     nil,
			AlwaysPrepared:      nil,
			AvailableSlotLevels: nil,
		},
	}
	store := &mockPrepStore{ownerID: "user-1", char: clericCharacter()}
	refData := &mockRefDataStore{}
	h := newPrepHandler(svc, store, refData)

	rec := httptest.NewRecorder()
	h.GetPreparation(rec, newPrepRequest(http.MethodGet, testCharID, "user-1", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	// nil slices must serialize as [] not null
	assert.Contains(t, body, `"current_prepared":[]`)
	assert.Contains(t, body, `"always_prepared":[]`)
	assert.Contains(t, body, `"available_slot_levels":[]`)
	assert.Contains(t, body, `"spells":[]`)
}

func TestGetPreparation_NotOwner(t *testing.T) {
	store := &mockPrepStore{ownerID: "owner-real", char: clericCharacter()}
	h := newPrepHandler(&mockPrepareService{}, store, &mockRefDataStore{})

	rec := httptest.NewRecorder()
	h.GetPreparation(rec, newPrepRequest(http.MethodGet, testCharID, "attacker", nil))

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestGetPreparation_NotFound(t *testing.T) {
	store := &mockPrepStore{ownerErr: portal.ErrCharacterNotFound}
	h := newPrepHandler(&mockPrepareService{}, store, &mockRefDataStore{})

	rec := httptest.NewRecorder()
	h.GetPreparation(rec, newPrepRequest(http.MethodGet, testCharID, "user-1", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetPreparation_NotPreparedCaster(t *testing.T) {
	fighter := clericCharacter()
	fighter.Classes = json.RawMessage(`[{"class":"Fighter","level":5}]`)
	store := &mockPrepStore{ownerID: "user-1", char: fighter}
	h := newPrepHandler(&mockPrepareService{}, store, &mockRefDataStore{})

	rec := httptest.NewRecorder()
	h.GetPreparation(rec, newPrepRequest(http.MethodGet, testCharID, "user-1", nil))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "not a prepared caster")
}

func TestGetPreparation_BadUUID(t *testing.T) {
	h := newPrepHandler(&mockPrepareService{}, &mockPrepStore{ownerID: "user-1"}, &mockRefDataStore{})

	rec := httptest.NewRecorder()
	h.GetPreparation(rec, newPrepRequest(http.MethodGet, "not-a-uuid", "user-1", nil))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetPreparation_OwnerLookupError(t *testing.T) {
	store := &mockPrepStore{ownerErr: errors.New("db down")}
	h := newPrepHandler(&mockPrepareService{}, store, &mockRefDataStore{})

	rec := httptest.NewRecorder()
	h.GetPreparation(rec, newPrepRequest(http.MethodGet, testCharID, "user-1", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestGetPreparation_CharacterLookupError(t *testing.T) {
	store := &mockPrepStore{ownerID: "user-1", charErr: errors.New("db down")}
	h := newPrepHandler(&mockPrepareService{}, store, &mockRefDataStore{})

	rec := httptest.NewRecorder()
	h.GetPreparation(rec, newPrepRequest(http.MethodGet, testCharID, "user-1", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestGetPreparation_InfoServiceError(t *testing.T) {
	svc := &mockPrepareService{infoErr: errors.New("boom")}
	store := &mockPrepStore{ownerID: "user-1", char: clericCharacter()}
	h := newPrepHandler(svc, store, &mockRefDataStore{})

	rec := httptest.NewRecorder()
	h.GetPreparation(rec, newPrepRequest(http.MethodGet, testCharID, "user-1", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestGetPreparation_RefDataError(t *testing.T) {
	svc := &mockPrepareService{info: combat.PreparationInfo{MaxPrepared: 5}}
	store := &mockPrepStore{ownerID: "user-1", char: clericCharacter()}
	h := newPrepHandler(svc, store, &errRefDataStore{})

	rec := httptest.NewRecorder()
	h.GetPreparation(rec, newPrepRequest(http.MethodGet, testCharID, "user-1", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestGetPreparation_Unauthenticated(t *testing.T) {
	h := newPrepHandler(&mockPrepareService{}, &mockPrepStore{}, &mockRefDataStore{})

	rec := httptest.NewRecorder()
	h.GetPreparation(rec, newPrepRequest(http.MethodGet, testCharID, "", nil))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestPostPreparation_Success(t *testing.T) {
	svc := &mockPrepareService{
		result: combat.PrepareSpellsResult{
			PreparedCount:  3,
			MaxPrepared:    8,
			AlwaysPrepared: []string{"cure-wounds"},
		},
	}
	store := &mockPrepStore{ownerID: "user-1", char: clericCharacter()}
	h := newPrepHandler(svc, store, &mockRefDataStore{})

	body, _ := json.Marshal(map[string][]string{"spells": {"bless", "guiding-bolt", "aid"}})
	rec := httptest.NewRecorder()
	h.PostPreparation(rec, newPrepRequest(http.MethodPost, testCharID, "user-1", body))

	require.Equal(t, http.StatusOK, rec.Code)
	var got prepPOSTResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, 3, got.PreparedCount)
	assert.Equal(t, 8, got.MaxPrepared)
	assert.Equal(t, []string{"cure-wounds"}, got.AlwaysPrepared)

	// Confirm the handler resolved class/subclass and passed selections through.
	assert.Equal(t, uuid.MustParse(testCharID), svc.gotPrepareInput.CharacterID)
	assert.Equal(t, "cleric", svc.gotPrepareInput.ClassName)
	assert.Equal(t, "life", svc.gotPrepareInput.Subclass)
	assert.Equal(t, []string{"bless", "guiding-bolt", "aid"}, svc.gotPrepareInput.Selected)
}

func TestPostPreparation_EmptyAlwaysMarshalsAsArray(t *testing.T) {
	svc := &mockPrepareService{
		result: combat.PrepareSpellsResult{PreparedCount: 1, MaxPrepared: 5, AlwaysPrepared: nil},
	}
	store := &mockPrepStore{ownerID: "user-1", char: clericCharacter()}
	h := newPrepHandler(svc, store, &mockRefDataStore{})

	body, _ := json.Marshal(map[string][]string{"spells": {"bless"}})
	rec := httptest.NewRecorder()
	h.PostPreparation(rec, newPrepRequest(http.MethodPost, testCharID, "user-1", body))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"always_prepared":[]`)
}

func TestPostPreparation_ValidationError(t *testing.T) {
	svc := &mockPrepareService{
		prepErr: errors.New("too many spells prepared: 9 selected, maximum 8"),
	}
	store := &mockPrepStore{ownerID: "user-1", char: clericCharacter()}
	h := newPrepHandler(svc, store, &mockRefDataStore{})

	body, _ := json.Marshal(map[string][]string{"spells": {"a", "b"}})
	rec := httptest.NewRecorder()
	h.PostPreparation(rec, newPrepRequest(http.MethodPost, testCharID, "user-1", body))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "too many spells prepared")
}

func TestPostPreparation_NotOwner(t *testing.T) {
	store := &mockPrepStore{ownerID: "owner-real", char: clericCharacter()}
	h := newPrepHandler(&mockPrepareService{}, store, &mockRefDataStore{})

	body, _ := json.Marshal(map[string][]string{"spells": {"bless"}})
	rec := httptest.NewRecorder()
	h.PostPreparation(rec, newPrepRequest(http.MethodPost, testCharID, "attacker", body))

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestPostPreparation_NotFound(t *testing.T) {
	store := &mockPrepStore{ownerErr: portal.ErrCharacterNotFound}
	h := newPrepHandler(&mockPrepareService{}, store, &mockRefDataStore{})

	body, _ := json.Marshal(map[string][]string{"spells": {"bless"}})
	rec := httptest.NewRecorder()
	h.PostPreparation(rec, newPrepRequest(http.MethodPost, testCharID, "user-1", body))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPostPreparation_NotPreparedCaster(t *testing.T) {
	fighter := clericCharacter()
	fighter.Classes = json.RawMessage(`[{"class":"Fighter","level":5}]`)
	store := &mockPrepStore{ownerID: "user-1", char: fighter}
	h := newPrepHandler(&mockPrepareService{}, store, &mockRefDataStore{})

	body, _ := json.Marshal(map[string][]string{"spells": {"bless"}})
	rec := httptest.NewRecorder()
	h.PostPreparation(rec, newPrepRequest(http.MethodPost, testCharID, "user-1", body))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "not a prepared caster")
}

func TestPostPreparation_BadUUID(t *testing.T) {
	h := newPrepHandler(&mockPrepareService{}, &mockPrepStore{ownerID: "user-1"}, &mockRefDataStore{})

	body, _ := json.Marshal(map[string][]string{"spells": {"bless"}})
	rec := httptest.NewRecorder()
	h.PostPreparation(rec, newPrepRequest(http.MethodPost, "not-a-uuid", "user-1", body))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPostPreparation_BadJSON(t *testing.T) {
	store := &mockPrepStore{ownerID: "user-1", char: clericCharacter()}
	h := newPrepHandler(&mockPrepareService{}, store, &mockRefDataStore{})

	rec := httptest.NewRecorder()
	h.PostPreparation(rec, newPrepRequest(http.MethodPost, testCharID, "user-1", []byte("{bad json")))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPostPreparation_FiresCardUpdater(t *testing.T) {
	svc := &mockPrepareService{result: combat.PrepareSpellsResult{PreparedCount: 1, MaxPrepared: 5}}
	store := &mockPrepStore{ownerID: "user-1", char: clericCharacter()}
	card := &mockCardUpdater{}
	h := newPrepHandler(svc, store, &mockRefDataStore{})
	h.SetCardUpdater(card)

	body, _ := json.Marshal(map[string][]string{"spells": {"bless"}})
	rec := httptest.NewRecorder()
	h.PostPreparation(rec, newPrepRequest(http.MethodPost, testCharID, "user-1", body))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, card.calls, 1)
	assert.Equal(t, uuid.MustParse(testCharID), card.calls[0])
}

func TestPostPreparation_CardUpdaterErrorDoesNotFailRequest(t *testing.T) {
	svc := &mockPrepareService{result: combat.PrepareSpellsResult{PreparedCount: 1, MaxPrepared: 5}}
	store := &mockPrepStore{ownerID: "user-1", char: clericCharacter()}
	card := &mockCardUpdater{err: errors.New("discord down")}
	h := newPrepHandler(svc, store, &mockRefDataStore{})
	h.SetCardUpdater(card)

	body, _ := json.Marshal(map[string][]string{"spells": {"bless"}})
	rec := httptest.NewRecorder()
	h.PostPreparation(rec, newPrepRequest(http.MethodPost, testCharID, "user-1", body))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, card.calls, 1)
}

func TestPostPreparation_NoCardUpdateOnValidationError(t *testing.T) {
	svc := &mockPrepareService{prepErr: errors.New("too many spells prepared")}
	store := &mockPrepStore{ownerID: "user-1", char: clericCharacter()}
	card := &mockCardUpdater{}
	h := newPrepHandler(svc, store, &mockRefDataStore{})
	h.SetCardUpdater(card)

	body, _ := json.Marshal(map[string][]string{"spells": {"a", "b"}})
	rec := httptest.NewRecorder()
	h.PostPreparation(rec, newPrepRequest(http.MethodPost, testCharID, "user-1", body))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, card.calls)
}

func TestPreparationStoreAdapter_GetCharacter(t *testing.T) {
	id := uuid.MustParse(testCharID)
	q := &mockCharacterQuerier{character: refdata.Character{ID: id, Name: "Aldric"}}
	store := portal.NewPreparationStoreAdapter(q)

	char, err := store.GetCharacter(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "Aldric", char.Name)
}

func TestPreparationStoreAdapter_GetCharacter_NotFound(t *testing.T) {
	q := &mockCharacterQuerier{charErr: sql.ErrNoRows}
	store := portal.NewPreparationStoreAdapter(q)

	_, err := store.GetCharacter(context.Background(), uuid.MustParse(testCharID))
	assert.ErrorIs(t, err, portal.ErrCharacterNotFound)
}

func TestPreparationStoreAdapter_GetCharacter_OtherError(t *testing.T) {
	q := &mockCharacterQuerier{charErr: errors.New("db down")}
	store := portal.NewPreparationStoreAdapter(q)

	_, err := store.GetCharacter(context.Background(), uuid.MustParse(testCharID))
	require.Error(t, err)
	assert.NotErrorIs(t, err, portal.ErrCharacterNotFound)
}
