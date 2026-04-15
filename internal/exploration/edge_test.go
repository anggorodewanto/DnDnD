package exploration_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/exploration"
	"github.com/ab/dndnd/internal/refdata"
)

// --- spawn.go edges ---

func TestParseSpawnZones_ZeroTileWidth(t *testing.T) {
	raw := json.RawMessage(`{"width":10,"height":10,"tilewidth":0,"tileheight":0,"layers":[]}`)
	zones, err := exploration.ParseSpawnZones(raw)
	if err != nil {
		t.Fatal(err)
	}
	if zones != nil {
		t.Errorf("expected nil zones for zero tile width; got %+v", zones)
	}
}

func TestExpandTiles_ZeroSizedZone(t *testing.T) {
	// Zero-sized zone on the only layer means Tiles slice is nil/empty.
	raw := json.RawMessage(`{
		"width": 10, "height": 10, "tilewidth": 48, "tileheight": 48,
		"layers": [
			{"name":"spawn_zones","type":"objectgroup","objects":[
				{"id":1,"x":0,"y":0,"width":0,"height":0,"type":"player"}
			]}
		]
	}`)
	zones, err := exploration.ParseSpawnZones(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 1 {
		t.Fatalf("got %d zones, want 1", len(zones))
	}
	if len(zones[0].Tiles) != 0 {
		t.Errorf("expected 0 tiles for zero-sized zone, got %d", len(zones[0].Tiles))
	}
}

// --- service.go edges ---

// errorFakeStore wraps fakeStore to inject errors on specific calls.
type errorFakeStore struct {
	*fakeStore
	errOnGetCharacter error
	errOnCreateComb   error
}

func (e *errorFakeStore) GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
	if e.errOnGetCharacter != nil {
		return refdata.Character{}, e.errOnGetCharacter
	}
	return e.fakeStore.GetCharacter(ctx, id)
}

func (e *errorFakeStore) CreateCombatant(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
	if e.errOnCreateComb != nil {
		return refdata.Combatant{}, e.errOnCreateComb
	}
	return e.fakeStore.CreateCombatant(ctx, arg)
}

func TestStartExploration_GetCharacterError(t *testing.T) {
	inner := newFakeStore()
	mapID := uuid.New()
	inner.maps[mapID] = refdata.Map{ID: mapID, TiledJson: buildMapWithPlayerZone(t)}
	charID := uuid.New()

	store := &errorFakeStore{fakeStore: inner, errOnGetCharacter: errors.New("boom")}
	svc := exploration.NewService(store)

	_, err := svc.StartExploration(context.Background(), exploration.StartInput{
		CampaignID:   uuid.New(),
		MapID:        mapID,
		Name:         "x",
		CharacterIDs: []uuid.UUID{charID},
	})
	if err == nil {
		t.Fatal("expected error from GetCharacter")
	}
}

func TestStartExploration_CreateCombatantError(t *testing.T) {
	inner := newFakeStore()
	mapID := uuid.New()
	inner.maps[mapID] = refdata.Map{ID: mapID, TiledJson: buildMapWithPlayerZone(t)}
	charID := uuid.New()
	inner.characters[charID] = refdata.Character{ID: charID, Name: "A", HpMax: 10, HpCurrent: 10, Ac: 10, SpeedFt: 30}

	store := &errorFakeStore{fakeStore: inner, errOnCreateComb: errors.New("db down")}
	svc := exploration.NewService(store)

	_, err := svc.StartExploration(context.Background(), exploration.StartInput{
		CampaignID:   uuid.New(),
		MapID:        mapID,
		Name:         "x",
		CharacterIDs: []uuid.UUID{charID},
	})
	if err == nil {
		t.Fatal("expected error from CreateCombatant")
	}
}

func TestStartExploration_DisplayNameIsPersisted(t *testing.T) {
	store := newFakeStore()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, TiledJson: buildMapWithPlayerZone(t)}
	svc := exploration.NewService(store)

	_, err := svc.StartExploration(context.Background(), exploration.StartInput{
		CampaignID:  uuid.New(),
		MapID:       mapID,
		Name:        "raw-name",
		DisplayName: "Pretty Name",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !store.createdEncounter.DisplayName.Valid || store.createdEncounter.DisplayName.String != "Pretty Name" {
		t.Errorf("display name not persisted: %+v", store.createdEncounter.DisplayName)
	}
}

func TestEndExploration_GetEncounterError(t *testing.T) {
	store := newFakeStore()
	svc := exploration.NewService(store)
	err := svc.EndExploration(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error when encounter not found")
	}
}

func TestCapturePositions_EncounterNotFound(t *testing.T) {
	store := newFakeStore()
	svc := exploration.NewService(store)
	_, err := svc.CapturePositions(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error when encounter missing")
	}
}

func TestCapturePositions_SkipsCreaturesWithoutCharacterID(t *testing.T) {
	store := newFakeStore()
	svc := exploration.NewService(store)
	encID := uuid.New()
	store.encounters[encID] = refdata.Encounter{ID: encID, Mode: "exploration"}
	store.combatants[encID] = []refdata.Combatant{
		{ID: uuid.New(), EncounterID: encID, CharacterID: uuid.NullUUID{}, PositionCol: "X", PositionRow: 9}, // NPC
	}
	positions, err := svc.CapturePositions(context.Background(), encID)
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) != 0 {
		t.Errorf("expected 0 PC positions, got %d", len(positions))
	}
}

// --- dashboard.go edges ---

func TestDashboardHandler_InvalidCampaignUUID(t *testing.T) {
	store := newDashboardStore()
	svc := exploration.NewService(store.fakeStore)
	h := exploration.NewDashboardHandler(svc, store)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/dashboard/exploration?campaign_id=not-a-uuid")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_StartPost_InvalidCampaignID(t *testing.T) {
	store := newDashboardStore()
	svc := exploration.NewService(store.fakeStore)
	h := exploration.NewDashboardHandler(svc, store)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	form := url.Values{}
	form.Set("campaign_id", "bad")
	form.Set("map_id", uuid.New().String())
	resp, err := http.PostForm(srv.URL+"/dashboard/exploration/start", form)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_StartPost_InvalidMapID(t *testing.T) {
	store := newDashboardStore()
	svc := exploration.NewService(store.fakeStore)
	h := exploration.NewDashboardHandler(svc, store)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	form := url.Values{}
	form.Set("campaign_id", uuid.New().String())
	form.Set("map_id", "bad")
	resp, err := http.PostForm(srv.URL+"/dashboard/exploration/start", form)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_StartPost_InvalidCharacterID(t *testing.T) {
	store := newDashboardStore()
	mapID := uuid.New()
	campID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, TiledJson: buildMapWithPlayerZone(t)}
	svc := exploration.NewService(store.fakeStore)
	h := exploration.NewDashboardHandler(svc, store)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	form := url.Values{}
	form.Set("campaign_id", campID.String())
	form.Set("map_id", mapID.String())
	form.Add("character_ids", "not-a-uuid")
	resp, err := http.PostForm(srv.URL+"/dashboard/exploration/start", form)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_StartPost_ServiceError(t *testing.T) {
	// No spawn zones on map + CharacterIDs present => ErrNoPlayerSpawnZones.
	store := newDashboardStore()
	mapID := uuid.New()
	campID := uuid.New()
	store.maps[mapID] = refdata.Map{
		ID: mapID,
		TiledJson: json.RawMessage(`{"width":5,"height":5,"tilewidth":48,"tileheight":48,"layers":[]}`),
	}
	charID := uuid.New()
	store.characters[charID] = refdata.Character{ID: charID, Name: "A"}
	svc := exploration.NewService(store.fakeStore)
	h := exploration.NewDashboardHandler(svc, store)

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	form := url.Values{}
	form.Set("campaign_id", campID.String())
	form.Set("map_id", mapID.String())
	form.Add("character_ids", charID.String())
	resp, err := http.PostForm(srv.URL+"/dashboard/exploration/start", form)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_StartPost_DefaultName(t *testing.T) {
	store := newDashboardStore()
	mapID := uuid.New()
	campID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, TiledJson: buildMapWithPlayerZone(t)}
	svc := exploration.NewService(store.fakeStore)
	h := exploration.NewDashboardHandler(svc, store)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	form := url.Values{}
	form.Set("campaign_id", campID.String())
	form.Set("map_id", mapID.String())
	resp, err := http.PostForm(srv.URL+"/dashboard/exploration/start", form)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	// Default name should have been applied: "Exploration".
	if store.fakeStore.createdEncounter.Name != "Exploration" {
		t.Errorf("default name not applied: got %q", store.fakeStore.createdEncounter.Name)
	}
}

// Ensure combat.Position type is used (cheap compile-time assertion).
var _ = combat.Position{}

// Force import of strings and Context for symmetry with the other files.
var _ = strings.Contains
var _ context.Context
