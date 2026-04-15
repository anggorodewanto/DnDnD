package exploration_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/exploration"
	"github.com/ab/dndnd/internal/refdata"
)

// dashboardStore bundles the store plus a map lister for the dashboard page.
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

func TestDashboardHandler_RendersMapSelector(t *testing.T) {
	store := newDashboardStore()
	campaignID := uuid.New()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, Name: "Forest", TiledJson: buildMapWithPlayerZone(t)}
	store.mapsByCampaign[campaignID] = []refdata.Map{{ID: mapID, Name: "Forest"}}

	svc := exploration.NewService(store.fakeStore)
	h := exploration.NewDashboardHandler(svc, store)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/dashboard/exploration?campaign_id=" + campaignID.String())
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "Forest") {
		t.Errorf("expected map name in output, got: %s", body)
	}
	if !strings.Contains(string(body), "Start Exploration") {
		t.Errorf("expected Start Exploration button, got: %s", body)
	}
}

func TestDashboardHandler_MissingCampaignID(t *testing.T) {
	store := newDashboardStore()
	svc := exploration.NewService(store.fakeStore)
	h := exploration.NewDashboardHandler(svc, store)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/dashboard/exploration")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_StartExplorationPost(t *testing.T) {
	store := newDashboardStore()
	campaignID := uuid.New()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, Name: "Forest", TiledJson: buildMapWithPlayerZone(t)}
	store.mapsByCampaign[campaignID] = []refdata.Map{{ID: mapID, Name: "Forest"}}

	svc := exploration.NewService(store.fakeStore)
	h := exploration.NewDashboardHandler(svc, store)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	srv := httptest.NewServer(r)
	defer srv.Close()

	form := url.Values{}
	form.Set("campaign_id", campaignID.String())
	form.Set("map_id", mapID.String())
	form.Set("name", "Forest Exploration")

	resp, err := http.PostForm(srv.URL+"/dashboard/exploration/start", form)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	// Response should be JSON with the encounter ID.
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

// TestDashboardHandler_TransitionToCombat_AppliesOverrides verifies Phase 110
// it2 clarification Q3/Q4: the combat-transition endpoint captures current
// exploration positions AND accepts per-PC override_<characterID>=<coord>
// form fields that override spawn placement before combat starts.
func TestDashboardHandler_TransitionToCombat_AppliesOverrides(t *testing.T) {
	store := newDashboardStore()
	campaignID := uuid.New()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, Name: "Forest", TiledJson: buildMapWithPlayerZone(t)}

	// Seed one exploration encounter with one PC at A1.
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

	svc := exploration.NewService(store.fakeStore)
	h := exploration.NewDashboardHandler(svc, store)

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Override this PC's position to D5.
	form := url.Values{}
	form.Set("encounter_id", encID.String())
	form.Set("override_"+charID.String(), "D5")

	resp, err := http.PostForm(srv.URL+"/dashboard/exploration/transition-to-combat", form)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
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

	svc := exploration.NewService(store.fakeStore)
	h := exploration.NewDashboardHandler(svc, store)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	form := url.Values{}
	form.Set("encounter_id", encID.String())

	resp, err := http.PostForm(srv.URL+"/dashboard/exploration/transition-to-combat", form)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	body, _ := io.ReadAll(resp.Body)
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

func TestDashboardHandler_TransitionToCombat_InvalidEncounterID(t *testing.T) {
	store := newDashboardStore()
	svc := exploration.NewService(store.fakeStore)
	h := exploration.NewDashboardHandler(svc, store)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	form := url.Values{}
	form.Set("encounter_id", "not-a-uuid")
	resp, err := http.PostForm(srv.URL+"/dashboard/exploration/transition-to-combat", form)
	if err != nil {
		t.Fatal(err)
	}
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
	svc := exploration.NewService(store.fakeStore)
	h := exploration.NewDashboardHandler(svc, store)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	form := url.Values{}
	form.Set("encounter_id", encID.String())
	form.Set("override_"+charID.String(), "ZZ")
	resp, err := http.PostForm(srv.URL+"/dashboard/exploration/transition-to-combat", form)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
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
	svc := exploration.NewService(store.fakeStore)
	h := exploration.NewDashboardHandler(svc, store)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	form := url.Values{}
	form.Set("encounter_id", encID.String())
	resp, err := http.PostForm(srv.URL+"/dashboard/exploration/transition-to-combat", form)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDashboardHandler_StartExplorationPost_MissingFields(t *testing.T) {
	store := newDashboardStore()
	svc := exploration.NewService(store.fakeStore)
	h := exploration.NewDashboardHandler(svc, store)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.PostForm(srv.URL+"/dashboard/exploration/start", url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
