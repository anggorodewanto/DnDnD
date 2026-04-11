package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// fakePublisher records PublishEncounterSnapshot calls so tests can verify
// that inventory.APIHandler invokes the publisher after DB-committing
// inventory/gold mutations when the affected character is in an active
// encounter.
type fakePublisher struct {
	mu        sync.Mutex
	published []uuid.UUID
	err       error
}

func (f *fakePublisher) PublishEncounterSnapshot(_ context.Context, encounterID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published = append(f.published, encounterID)
	return f.err
}

func (f *fakePublisher) calls() []uuid.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]uuid.UUID, len(f.published))
	copy(out, f.published)
	return out
}

// fakeEncounterLookup returns a preset encounter ID (or zero / error) for a
// character lookup.
type fakeEncounterLookup struct {
	encID uuid.UUID
	err   error
}

func (f *fakeEncounterLookup) ActiveEncounterIDForCharacter(_ context.Context, _ uuid.UUID) (uuid.UUID, bool, error) {
	if f.err != nil {
		return uuid.Nil, false, f.err
	}
	if f.encID == uuid.Nil {
		return uuid.Nil, false, nil
	}
	return f.encID, true, nil
}

func newHandlerWithChar(charID uuid.UUID) (*APIHandler, *mockCharacterStore) {
	store := newMockStore()
	store.chars[charID] = refdata.Character{ID: charID, Name: "Hero", Gold: 50}
	return NewAPIHandler(store), store
}

func postJSON(t *testing.T, handler http.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	bs, err := json.Marshal(body)
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bs))
	w := httptest.NewRecorder()
	handler(w, r)
	return w
}

func TestAPIHandler_SetGold_PublishesSnapshot(t *testing.T) {
	charID := uuid.New()
	encID := uuid.New()
	h, _ := newHandlerWithChar(charID)
	pub := &fakePublisher{}
	h.SetPublisher(pub, &fakeEncounterLookup{encID: encID})

	w := postJSON(t, h.HandleSetGold, SetGoldRequest{CharacterID: charID.String(), Gold: 99})
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []uuid.UUID{encID}, pub.calls())
}

func TestAPIHandler_SetGold_CharacterNotInActiveEncounter_NoPublish(t *testing.T) {
	charID := uuid.New()
	h, _ := newHandlerWithChar(charID)
	pub := &fakePublisher{}
	h.SetPublisher(pub, &fakeEncounterLookup{}) // encID zero => not in combat

	w := postJSON(t, h.HandleSetGold, SetGoldRequest{CharacterID: charID.String(), Gold: 99})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, pub.calls())
}

func TestAPIHandler_SetGold_NilPublisherTolerated(t *testing.T) {
	charID := uuid.New()
	h, _ := newHandlerWithChar(charID)
	// Intentionally do NOT call SetPublisher.

	w := postJSON(t, h.HandleSetGold, SetGoldRequest{CharacterID: charID.String(), Gold: 99})
	require.Equal(t, http.StatusOK, w.Code)
}

func TestAPIHandler_SetGold_PublishErrorSwallowed(t *testing.T) {
	charID := uuid.New()
	encID := uuid.New()
	h, _ := newHandlerWithChar(charID)
	pub := &fakePublisher{err: errors.New("hub full")}
	h.SetPublisher(pub, &fakeEncounterLookup{encID: encID})

	w := postJSON(t, h.HandleSetGold, SetGoldRequest{CharacterID: charID.String(), Gold: 99})
	require.Equal(t, http.StatusOK, w.Code, "publish failure must not surface as HTTP error")
	assert.Len(t, pub.calls(), 1)
}

func TestAPIHandler_SetGold_ValidationError_NoPublish(t *testing.T) {
	h, _ := newHandlerWithChar(uuid.New())
	pub := &fakePublisher{}
	h.SetPublisher(pub, &fakeEncounterLookup{encID: uuid.New()})

	// Negative gold triggers validation failure before any DB mutation.
	w := postJSON(t, h.HandleSetGold, SetGoldRequest{CharacterID: uuid.New().String(), Gold: -1})
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, pub.calls())
}

func TestAPIHandler_SetGold_StoreError_NoPublish(t *testing.T) {
	charID := uuid.New()
	h, store := newHandlerWithChar(charID)
	store.chars = map[uuid.UUID]refdata.Character{} // force GetCharacter failure
	pub := &fakePublisher{}
	h.SetPublisher(pub, &fakeEncounterLookup{encID: uuid.New()})

	w := postJSON(t, h.HandleSetGold, SetGoldRequest{CharacterID: charID.String(), Gold: 99})
	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Empty(t, pub.calls())
}

func TestAPIHandler_AddItem_PublishesSnapshot(t *testing.T) {
	charID := uuid.New()
	encID := uuid.New()
	h, _ := newHandlerWithChar(charID)
	pub := &fakePublisher{}
	h.SetPublisher(pub, &fakeEncounterLookup{encID: encID})

	w := postJSON(t, h.HandleAddItem, AddItemRequest{
		CharacterID: charID.String(),
		Item:        character.InventoryItem{ItemID: "potion", Name: "Potion", Quantity: 1, Type: "consumable"},
	})
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []uuid.UUID{encID}, pub.calls())
}

func TestAPIHandler_RemoveItem_PublishesSnapshot(t *testing.T) {
	charID := uuid.New()
	encID := uuid.New()
	h, store := newHandlerWithChar(charID)
	// Seed character with one item to remove.
	invJSON, err := json.Marshal([]character.InventoryItem{{ItemID: "torch", Name: "Torch", Quantity: 2, Type: "other"}})
	require.NoError(t, err)
	c := store.chars[charID]
	c.Inventory.RawMessage = invJSON
	c.Inventory.Valid = true
	store.chars[charID] = c
	pub := &fakePublisher{}
	h.SetPublisher(pub, &fakeEncounterLookup{encID: encID})

	w := postJSON(t, h.HandleRemoveItem, RemoveItemRequest{CharacterID: charID.String(), ItemID: "torch", Quantity: 1})
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []uuid.UUID{encID}, pub.calls())
}

func TestAPIHandler_IdentifyItem_PublishesSnapshot(t *testing.T) {
	charID := uuid.New()
	encID := uuid.New()
	h, store := newHandlerWithChar(charID)
	identified := false
	invJSON, err := json.Marshal([]character.InventoryItem{{ItemID: "sword", Name: "Sword", Quantity: 1, Type: "weapon", IsMagic: true, Identified: &identified}})
	require.NoError(t, err)
	c := store.chars[charID]
	c.Inventory.RawMessage = invJSON
	c.Inventory.Valid = true
	store.chars[charID] = c
	pub := &fakePublisher{}
	h.SetPublisher(pub, &fakeEncounterLookup{encID: encID})

	w := postJSON(t, h.HandleIdentifyItem, IdentifyItemRequest{CharacterID: charID.String(), ItemID: "sword", Identified: true})
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []uuid.UUID{encID}, pub.calls())
}

func TestAPIHandler_TransferItem_PublishesBothEncounters(t *testing.T) {
	fromID := uuid.New()
	toID := uuid.New()
	fromEnc := uuid.New()
	toEnc := uuid.New()
	store := newMockStore()
	invJSON, err := json.Marshal([]character.InventoryItem{{ItemID: "torch", Name: "Torch", Quantity: 2, Type: "other"}})
	require.NoError(t, err)
	store.chars[fromID] = refdata.Character{ID: fromID, Name: "A"}
	c := store.chars[fromID]
	c.Inventory.RawMessage = invJSON
	c.Inventory.Valid = true
	store.chars[fromID] = c
	store.chars[toID] = refdata.Character{ID: toID, Name: "B"}

	h := NewAPIHandler(store)
	pub := &fakePublisher{}
	h.SetPublisher(pub, &perCharLookup{active: map[uuid.UUID]uuid.UUID{fromID: fromEnc, toID: toEnc}})

	w := postJSON(t, h.HandleTransferItem, TransferItemRequest{
		FromCharacterID: fromID.String(), ToCharacterID: toID.String(), ItemID: "torch", Quantity: 1,
	})
	require.Equal(t, http.StatusOK, w.Code)
	calls := pub.calls()
	require.Len(t, calls, 2)
	assert.ElementsMatch(t, []uuid.UUID{fromEnc, toEnc}, calls)
}

// perCharLookup returns a distinct encounter ID per character for the
// transfer test.
type perCharLookup struct {
	active map[uuid.UUID]uuid.UUID
}

func (p *perCharLookup) ActiveEncounterIDForCharacter(_ context.Context, charID uuid.UUID) (uuid.UUID, bool, error) {
	if encID, ok := p.active[charID]; ok {
		return encID, true, nil
	}
	return uuid.Nil, false, nil
}
