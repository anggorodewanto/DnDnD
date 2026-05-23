package exploration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/exploration"
	"github.com/ab/dndnd/internal/refdata"
)

// dashboardStore bundles the store plus a map lister for the dashboard endpoints.
type dashboardStore struct {
	*fakeStore
	mapsByCampaign map[uuid.UUID][]refdata.Map
}

func (d *dashboardStore) ListMapsByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Map, error) {
	return d.mapsByCampaign[campaignID], nil
}

func newDashboardStore() *dashboardStore {
	return &dashboardStore{
		fakeStore:      newFakeStore(),
		mapsByCampaign: map[uuid.UUID][]refdata.Map{},
	}
}

func newDashboardServer(t *testing.T, store *dashboardStore) *httptest.Server {
	t.Helper()
	svc := exploration.NewService(store.fakeStore)
	h := exploration.NewDashboardHandler(svc, store)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func TestDashboardHandler_GetData_ReturnsMapsAsJSON(t *testing.T) {
	store := newDashboardStore()
	campaignID := uuid.New()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, Name: "Forest", TiledJson: buildMapWithPlayerZone(t)}
	store.mapsByCampaign[campaignID] = []refdata.Map{{ID: mapID, Name: "Forest"}}

	srv := newDashboardServer(t, store)

	resp, err := http.Get(srv.URL + "/api/exploration?campaign_id=" + campaignID.String())
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	ctype := resp.Header.Get("Content-Type")
	if !strings.Contains(ctype, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ctype)
	}
	var out struct {
		CampaignID string `json:"campaign_id"`
		Maps       []struct {
			ID   uuid.UUID `json:"id"`
			Name string    `json:"name"`
		} `json:"maps"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("bad JSON: %v body=%s", err, body)
	}
	if out.CampaignID != campaignID.String() {
		t.Errorf("campaign_id = %q, want %q", out.CampaignID, campaignID.String())
	}
	if len(out.Maps) != 1 || out.Maps[0].Name != "Forest" {
		t.Errorf("maps = %+v, want one Forest map", out.Maps)
	}
}

func TestDashboardHandler_GetData_EmptyMapList(t *testing.T) {
	store := newDashboardStore()
	campaignID := uuid.New()
	srv := newDashboardServer(t, store)

	resp, err := http.Get(srv.URL + "/api/exploration?campaign_id=" + campaignID.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	// Should be a JSON object with maps: [] (not null) so the SPA can iterate.
	if !bytes.Contains(body, []byte(`"maps":[]`)) {
		t.Errorf("expected maps:[] in body, got %s", body)
	}
}

func TestDashboardHandler_GetData_MissingCampaignID(t *testing.T) {
	store := newDashboardStore()
	srv := newDashboardServer(t, store)

	resp, err := http.Get(srv.URL + "/api/exploration")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_GetData_InvalidCampaignID(t *testing.T) {
	store := newDashboardStore()
	srv := newDashboardServer(t, store)

	resp, err := http.Get(srv.URL + "/api/exploration?campaign_id=not-a-uuid")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_StartExploration_JSONPost(t *testing.T) {
	store := newDashboardStore()
	campaignID := uuid.New()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, Name: "Forest", TiledJson: buildMapWithPlayerZone(t)}
	store.mapsByCampaign[campaignID] = []refdata.Map{{ID: mapID, Name: "Forest"}}

	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/start", map[string]any{
		"campaign_id": campaignID.String(),
		"map_id":      mapID.String(),
		"name":        "Forest Exploration",
	})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("bad JSON: %v body=%s", err, body)
	}
	if out["encounter_id"] == nil {
		t.Errorf("missing encounter_id in response: %v", out)
	}
	if out["mode"] != "exploration" {
		t.Errorf("mode = %v, want exploration", out["mode"])
	}
}

func TestDashboardHandler_StartExploration_DefaultsName(t *testing.T) {
	store := newDashboardStore()
	campaignID := uuid.New()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, Name: "Forest", TiledJson: buildMapWithPlayerZone(t)}
	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/start", map[string]any{
		"campaign_id": campaignID.String(),
		"map_id":      mapID.String(),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	enc := store.createdEncounter
	if enc.Name != "Exploration" {
		t.Errorf("encounter name = %q, want default %q", enc.Name, "Exploration")
	}
}

func TestDashboardHandler_StartExploration_AcceptsCharacterIDs(t *testing.T) {
	store := newDashboardStore()
	campaignID := uuid.New()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, Name: "Forest", TiledJson: buildMapWithPlayerZone(t)}
	charID := uuid.New()
	store.characters[charID] = refdata.Character{ID: charID, Name: "Alice"}
	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/start", map[string]any{
		"campaign_id":   campaignID.String(),
		"map_id":        mapID.String(),
		"character_ids": []string{charID.String()},
	})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var out struct {
		PCs map[string]any `json:"pcs"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("bad JSON: %v body=%s", err, body)
	}
	if _, ok := out.PCs[charID.String()]; !ok {
		t.Errorf("expected PC %s in response.pcs, got %+v", charID, out.PCs)
	}
}

func TestDashboardHandler_StartExploration_RejectsInvalidJSON(t *testing.T) {
	store := newDashboardStore()
	srv := newDashboardServer(t, store)

	resp, err := http.Post(srv.URL+"/dashboard/exploration/start", "application/json",
		strings.NewReader("not json"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_StartExploration_RejectsMissingFields(t *testing.T) {
	store := newDashboardStore()
	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/start", map[string]any{})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_StartExploration_RejectsInvalidCampaignID(t *testing.T) {
	store := newDashboardStore()
	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/start", map[string]any{
		"campaign_id": "nope",
		"map_id":      uuid.New().String(),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_StartExploration_RejectsInvalidMapID(t *testing.T) {
	store := newDashboardStore()
	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/start", map[string]any{
		"campaign_id": uuid.New().String(),
		"map_id":      "nope",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_StartExploration_RejectsInvalidCharacterID(t *testing.T) {
	store := newDashboardStore()
	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/start", map[string]any{
		"campaign_id":   uuid.New().String(),
		"map_id":        uuid.New().String(),
		"character_ids": []string{"not-a-uuid"},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_StartExploration_PropagatesServiceError(t *testing.T) {
	// Unknown map id -> service errors -> 400.
	store := newDashboardStore()
	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/start", map[string]any{
		"campaign_id": uuid.New().String(),
		"map_id":      uuid.New().String(),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_TransitionToCombat_AppliesOverrides(t *testing.T) {
	store := newDashboardStore()
	campaignID := uuid.New()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, Name: "Forest", TiledJson: buildMapWithPlayerZone(t)}

	charID := uuid.New()
	store.characters[charID] = refdata.Character{ID: charID, Name: "Alice"}
	encID := uuid.New()
	store.encounters[encID] = refdata.Encounter{
		ID:         encID,
		CampaignID: campaignID,
		MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
		Mode:       "exploration",
		Status:     "active",
	}
	store.combatants[encID] = []refdata.Combatant{
		{
			ID:          uuid.New(),
			EncounterID: encID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			PositionCol: "A", PositionRow: 1, IsAlive: true,
		},
	}

	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/transition-to-combat", map[string]any{
		"encounter_id": encID.String(),
		"overrides":    map[string]string{charID.String(): "D5"},
	})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	var out struct {
		Positions map[string]struct {
			Col string `json:"col"`
			Row int32  `json:"row"`
		} `json:"positions"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("bad JSON: %v body=%s", err, body)
	}
	pos, ok := out.Positions[charID.String()]
	if !ok {
		t.Fatalf("missing position for charID: %v", out)
	}
	if pos.Col != "D" || pos.Row != 5 {
		t.Errorf("got override position %s%d, want D5", pos.Col, pos.Row)
	}
}

func TestDashboardHandler_TransitionToCombat_NoOverrides_UsesCapturedPositions(t *testing.T) {
	store := newDashboardStore()
	campaignID := uuid.New()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, TiledJson: buildMapWithPlayerZone(t)}

	charID := uuid.New()
	store.characters[charID] = refdata.Character{ID: charID, Name: "Bob"}
	encID := uuid.New()
	store.encounters[encID] = refdata.Encounter{
		ID: encID, CampaignID: campaignID,
		MapID: uuid.NullUUID{UUID: mapID, Valid: true},
		Mode:  "exploration", Status: "active",
	}
	store.combatants[encID] = []refdata.Combatant{
		{
			ID: uuid.New(), EncounterID: encID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			PositionCol: "C", PositionRow: 3, IsAlive: true,
		},
	}

	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/transition-to-combat", map[string]any{
		"encounter_id": encID.String(),
	})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var out struct {
		Positions map[string]struct {
			Col string `json:"col"`
			Row int32  `json:"row"`
		} `json:"positions"`
	}
	_ = json.Unmarshal(body, &out)
	pos := out.Positions[charID.String()]
	if pos.Col != "C" || pos.Row != 3 {
		t.Errorf("got %s%d, want C3", pos.Col, pos.Row)
	}
}

func TestDashboardHandler_TransitionToCombat_FlipsEncounterMode(t *testing.T) {
	store := newDashboardStore()
	campaignID := uuid.New()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, TiledJson: buildMapWithPlayerZone(t)}

	charID := uuid.New()
	store.characters[charID] = refdata.Character{ID: charID, Name: "Clara"}
	encID := uuid.New()
	store.encounters[encID] = refdata.Encounter{
		ID: encID, CampaignID: campaignID,
		MapID: uuid.NullUUID{UUID: mapID, Valid: true},
		Mode:  "exploration", Status: "active",
	}
	store.combatants[encID] = []refdata.Combatant{
		{
			ID: uuid.New(), EncounterID: encID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			PositionCol: "B", PositionRow: 4, IsAlive: true,
		},
	}

	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/transition-to-combat", map[string]any{
		"encounter_id": encID.String(),
	})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	var out struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("bad JSON: %v body=%s", err, body)
	}
	if out.Mode != "combat" {
		t.Fatalf("mode = %q, want combat; body=%s", out.Mode, body)
	}
	if got := store.encounters[encID].Mode; got != "combat" {
		t.Fatalf("stored encounter mode = %q, want combat", got)
	}
	if len(store.updateModeCalls) != 1 {
		t.Fatalf("UpdateEncounterMode calls = %d, want 1", len(store.updateModeCalls))
	}
	if store.updateModeCalls[0].ID != encID || store.updateModeCalls[0].Mode != "combat" {
		t.Fatalf("UpdateEncounterMode arg = %+v", store.updateModeCalls[0])
	}
}

func TestDashboardHandler_TransitionToCombat_RejectsInvalidJSON(t *testing.T) {
	store := newDashboardStore()
	srv := newDashboardServer(t, store)

	resp, err := http.Post(srv.URL+"/dashboard/exploration/transition-to-combat",
		"application/json", strings.NewReader("not json"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_TransitionToCombat_MissingEncounterID(t *testing.T) {
	store := newDashboardStore()
	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/transition-to-combat", map[string]any{})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_TransitionToCombat_InvalidEncounterID(t *testing.T) {
	store := newDashboardStore()
	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/transition-to-combat", map[string]any{
		"encounter_id": "not-a-uuid",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_TransitionToCombat_InvalidOverrideCharID(t *testing.T) {
	store := newDashboardStore()
	campaignID := uuid.New()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, TiledJson: buildMapWithPlayerZone(t)}
	encID := uuid.New()
	store.encounters[encID] = refdata.Encounter{
		ID: encID, CampaignID: campaignID,
		MapID: uuid.NullUUID{UUID: mapID, Valid: true},
		Mode:  "exploration", Status: "active",
	}
	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/transition-to-combat", map[string]any{
		"encounter_id": encID.String(),
		"overrides":    map[string]string{"not-a-uuid": "D5"},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_TransitionToCombat_InvalidOverrideCoord(t *testing.T) {
	store := newDashboardStore()
	campaignID := uuid.New()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, TiledJson: buildMapWithPlayerZone(t)}
	charID := uuid.New()
	store.characters[charID] = refdata.Character{ID: charID, Name: "X"}
	encID := uuid.New()
	store.encounters[encID] = refdata.Encounter{
		ID: encID, CampaignID: campaignID,
		MapID: uuid.NullUUID{UUID: mapID, Valid: true},
		Mode:  "exploration", Status: "active",
	}
	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/transition-to-combat", map[string]any{
		"encounter_id": encID.String(),
		"overrides":    map[string]string{charID.String(): "ZZ"},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_TransitionToCombat_IgnoresBlankOverrideCoord(t *testing.T) {
	store := newDashboardStore()
	campaignID := uuid.New()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, TiledJson: buildMapWithPlayerZone(t)}
	charID := uuid.New()
	store.characters[charID] = refdata.Character{ID: charID, Name: "Bob"}
	encID := uuid.New()
	store.encounters[encID] = refdata.Encounter{
		ID: encID, CampaignID: campaignID,
		MapID: uuid.NullUUID{UUID: mapID, Valid: true},
		Mode:  "exploration", Status: "active",
	}
	store.combatants[encID] = []refdata.Combatant{
		{
			ID: uuid.New(), EncounterID: encID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			PositionCol: "C", PositionRow: 3, IsAlive: true,
		},
	}
	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/transition-to-combat", map[string]any{
		"encounter_id": encID.String(),
		"overrides":    map[string]string{charID.String(): "   "},
	})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var out struct {
		Positions map[string]struct {
			Col string `json:"col"`
			Row int32  `json:"row"`
		} `json:"positions"`
	}
	_ = json.Unmarshal(body, &out)
	pos := out.Positions[charID.String()]
	if pos.Col != "C" || pos.Row != 3 {
		t.Errorf("blank override should be ignored; got %s%d, want C3", pos.Col, pos.Row)
	}
}

func TestDashboardHandler_TransitionToCombat_CapturePositionsError(t *testing.T) {
	// Encounter in combat mode -> CapturePositions returns ErrEncounterNotExploration.
	store := newDashboardStore()
	campaignID := uuid.New()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, TiledJson: buildMapWithPlayerZone(t)}
	encID := uuid.New()
	store.encounters[encID] = refdata.Encounter{
		ID: encID, CampaignID: campaignID,
		MapID: uuid.NullUUID{UUID: mapID, Valid: true},
		Mode:  "combat", Status: "active",
	}
	srv := newDashboardServer(t, store)

	resp := postJSON(t, srv.URL+"/dashboard/exploration/transition-to-combat", map[string]any{
		"encounter_id": encID.String(),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
