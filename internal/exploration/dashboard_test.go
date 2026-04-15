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
