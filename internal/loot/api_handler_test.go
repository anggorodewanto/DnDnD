package loot_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/loot"
	"github.com/ab/dndnd/internal/refdata"
)

func setupAPITest(t *testing.T) (*refdata.Queries, *loot.APIHandler, refdata.Campaign, refdata.Encounter) {
	t.Helper()
	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "completed")

	svc := loot.NewService(q)
	handler := loot.NewAPIHandler(svc)
	return q, handler, camp, enc
}

func chiCtx(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestAPI_CreateAndGetLootPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q, handler, camp, enc := setupAPITest(t)

	// Create defeated NPC
	npcItems := []character.InventoryItem{
		{ItemID: "shortsword", Name: "Shortsword", Quantity: 1, Type: "weapon"},
	}
	npcChar := createCharWithInventory(t, q, camp.ID, "Goblin", 10, npcItems)
	createCombatant(t, q, enc.ID,
		uuid.NullUUID{UUID: npcChar.ID, Valid: true},
		sql.NullString{}, "Goblin", true, false)

	// POST to create pool
	req := httptest.NewRequest(http.MethodPost, "/api/campaigns/x/encounters/y/loot", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateLootPool(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)

	// GET the pool
	req = httptest.NewRequest(http.MethodGet, "/api/campaigns/x/encounters/y/loot", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandleGetLootPool(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var result loot.LootPoolResult
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&result))
	assert.Equal(t, int32(10), result.Pool.GoldTotal)
	assert.Len(t, result.Items, 1)
}

func TestAPI_AddItem(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, enc := setupAPITest(t)

	// Create pool first
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateLootPool(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Add item
	body, _ := json.Marshal(loot.AddItemRequest{
		Name:        "Mysterious Key",
		Description: "Glows faintly",
		Quantity:    1,
		Type:        "other",
	})
	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandleAddItem(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestAPI_RemoveItem(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q, handler, camp, enc := setupAPITest(t)

	npcItems := []character.InventoryItem{
		{ItemID: "shortsword", Name: "Shortsword", Quantity: 1, Type: "weapon"},
	}
	npcChar := createCharWithInventory(t, q, camp.ID, "Goblin", 0, npcItems)
	createCombatant(t, q, enc.ID,
		uuid.NullUUID{UUID: npcChar.ID, Valid: true},
		sql.NullString{}, "Goblin", true, false)

	// Create pool
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateLootPool(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var createResult loot.LootPoolResult
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&createResult))
	require.Len(t, createResult.Items, 1)
	itemID := createResult.Items[0].ID

	// Remove item
	req = httptest.NewRequest(http.MethodDelete, "/", nil)
	req = chiCtx(req, map[string]string{
		"encounterID": enc.ID.String(),
		"itemID":      itemID.String(),
	})
	rec = httptest.NewRecorder()
	handler.HandleRemoveItem(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAPI_SplitGold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	q, handler, camp, enc := setupAPITest(t)

	npcChar := createCharWithInventory(t, q, camp.ID, "Goblin", 20, nil)
	createCombatant(t, q, enc.ID,
		uuid.NullUUID{UUID: npcChar.ID, Valid: true},
		sql.NullString{}, "Goblin", true, false)

	pc := createCharWithInventory(t, q, camp.ID, "Aria", 5, nil)
	createPlayerCharacter(t, q, camp.ID, pc.ID, "discord-1")

	var combatLogMsg string
	handler.SetCombatLogFunc(func(msg string) { combatLogMsg = msg })

	// Create pool
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateLootPool(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Split gold
	req = httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandleSplitGold(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, combatLogMsg, "20 gp each")
}

func TestAPI_PostAnnouncement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, enc := setupAPITest(t)

	// Create pool
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateLootPool(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var combatLogMsg string
	handler.SetCombatLogFunc(func(msg string) { combatLogMsg = msg })

	// Post announcement
	req = httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandlePostAnnouncement(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, combatLogMsg)
}

func TestAPI_ClearPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, enc := setupAPITest(t)

	// Create pool
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateLootPool(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Clear pool
	req = httptest.NewRequest(http.MethodDelete, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandleClearPool(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAPI_SetGold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, enc := setupAPITest(t)

	// Create pool
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateLootPool(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Set gold
	body, _ := json.Marshal(loot.SetGoldRequest{Gold: 100})
	req = httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandleSetGold(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAPI_CreateLootPool_NotCompleted(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	enc := createEncounter(t, q, camp.ID, "active")

	svc := loot.NewService(q)
	handler := loot.NewAPIHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateLootPool(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_CreateLootPool_Duplicate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, enc := setupAPITest(t)

	// Create pool
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateLootPool(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Duplicate
	req = httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandleCreateLootPool(rec, req)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestAPI_GetLootPool_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, enc := setupAPITest(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleGetLootPool(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPI_InvalidEncounterID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, _ := setupAPITest(t)

	// Test each handler with bad encounter ID
	handlers := []struct {
		name   string
		method string
		fn     func(http.ResponseWriter, *http.Request)
	}{
		{"get", http.MethodGet, handler.HandleGetLootPool},
		{"create", http.MethodPost, handler.HandleCreateLootPool},
		{"clear", http.MethodDelete, handler.HandleClearPool},
		{"splitGold", http.MethodPost, handler.HandleSplitGold},
		{"postAnnouncement", http.MethodPost, handler.HandlePostAnnouncement},
	}

	for _, h := range handlers {
		t.Run(h.name, func(t *testing.T) {
			req := httptest.NewRequest(h.method, "/", nil)
			req = chiCtx(req, map[string]string{"encounterID": "bad-uuid"})
			rec := httptest.NewRecorder()
			h.fn(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestAPI_AddItem_InvalidBody(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, enc := setupAPITest(t)

	// Create pool first
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleCreateLootPool(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Invalid body
	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandleAddItem(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Missing name
	body, _ := json.Marshal(loot.AddItemRequest{Quantity: 1})
	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec = httptest.NewRecorder()
	handler.HandleAddItem(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_AddItem_NoPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, enc := setupAPITest(t)

	body, _ := json.Marshal(loot.AddItemRequest{Name: "Test", Quantity: 1})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleAddItem(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPI_RemoveItem_InvalidItemID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, enc := setupAPITest(t)

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req = chiCtx(req, map[string]string{
		"encounterID": enc.ID.String(),
		"itemID":      "bad-uuid",
	})
	rec := httptest.NewRecorder()
	handler.HandleRemoveItem(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_SetGold_InvalidBody(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, enc := setupAPITest(t)

	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("bad")))
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleSetGold(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_SetGold_NoPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, enc := setupAPITest(t)

	body, _ := json.Marshal(loot.SetGoldRequest{Gold: 100})
	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleSetGold(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPI_SplitGold_NoPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, enc := setupAPITest(t)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleSplitGold(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPI_ClearPool_NoPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, enc := setupAPITest(t)

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleClearPool(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPI_PostAnnouncement_NoPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, enc := setupAPITest(t)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = chiCtx(req, map[string]string{"encounterID": enc.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandlePostAnnouncement(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPI_RemoveItem_NoPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, enc := setupAPITest(t)

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req = chiCtx(req, map[string]string{
		"encounterID": enc.ID.String(),
		"itemID":      uuid.New().String(),
	})
	rec := httptest.NewRecorder()
	handler.HandleRemoveItem(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPI_AddItem_BadEncounterID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, _ := setupAPITest(t)

	body, _ := json.Marshal(loot.AddItemRequest{Name: "Test"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"encounterID": "bad-uuid"})
	rec := httptest.NewRecorder()
	handler.HandleAddItem(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_RemoveItem_BadEncounterID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, _ := setupAPITest(t)

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req = chiCtx(req, map[string]string{
		"encounterID": "bad-uuid",
		"itemID":      uuid.New().String(),
	})
	rec := httptest.NewRecorder()
	handler.HandleRemoveItem(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_SetGold_BadEncounterID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, handler, _, _ := setupAPITest(t)

	body, _ := json.Marshal(loot.SetGoldRequest{Gold: 100})
	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	req = chiCtx(req, map[string]string{"encounterID": "bad-uuid"})
	rec := httptest.NewRecorder()
	handler.HandleSetGold(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- F-13: eligible-encounter listing for the LootPoolPanel ---

func TestAPI_ListEligibleEncounters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	camp := createCampaign(t, q)
	completed := createEncounter(t, q, camp.ID, "completed")
	_ = createEncounter(t, q, camp.ID, "active") // should be filtered out

	svc := loot.NewService(q)
	handler := loot.NewAPIHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": camp.ID.String()})
	rec := httptest.NewRecorder()
	handler.HandleListEligibleEncounters(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Encounters []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"encounters"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Len(t, resp.Encounters, 1)
	assert.Equal(t, completed.ID.String(), resp.Encounters[0].ID)
	assert.Equal(t, "completed", resp.Encounters[0].Status)
}

func TestAPI_ListEligibleEncounters_BadCampaignID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, q := setupTestDB(t)
	svc := loot.NewService(q)
	handler := loot.NewAPIHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = chiCtx(req, map[string]string{"campaignID": "not-a-uuid"})
	rec := httptest.NewRecorder()
	handler.HandleListEligibleEncounters(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
