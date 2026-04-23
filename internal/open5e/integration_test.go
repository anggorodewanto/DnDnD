package open5e_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/open5e"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/statblocklibrary"
	"github.com/ab/dndnd/internal/testutil"
)

// Phase 111: integration test that covers the full happy path —
// (1) fetch a monster from a mock Open5e server,
// (2) cache it into creatures via refdata.Queries,
// (3) enable the document slug on a real campaign,
// (4) verify stat block library surfaces the cached row via
//
//	GetStatBlockWithSources using the campaign's open5e_sources list.
func TestIntegration_Open5e_CacheAndListViaCampaignSources(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	shared := testutil.NewSharedTestDB(dbfs.Migrations)
	defer shared.Teardown()
	db := shared.AcquireDB(t)
	queries := refdata.New(db)
	ctx := context.Background()

	// (1) Stand up a mock Open5e API server.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/monsters/aboleth/":
			fmt.Fprint(w, `{
				"slug":"aboleth","name":"Aboleth","size":"Large","type":"aberration",
				"alignment":"lawful evil","armor_class":17,"hit_points":135,"hit_dice":"18d10+36",
				"strength":21,"dexterity":9,"constitution":15,
				"intelligence":18,"wisdom":15,"charisma":18,
				"challenge_rating":"10","document__slug":"tome-of-beasts"
			}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	// (2) Cache into the real DB.
	svc := open5e.NewService(open5e.NewClient(upstream.URL+"/", nil), open5e.NewCache(queries))
	id, err := svc.SearchAndCacheMonster(ctx, "aboleth")
	if err != nil {
		t.Fatalf("SearchAndCacheMonster: %v", err)
	}
	if id != "open5e_aboleth" {
		t.Fatalf("expected id open5e_aboleth, got %s", id)
	}
	got, err := queries.GetCreature(ctx, id)
	if err != nil {
		t.Fatalf("GetCreature: %v", err)
	}
	if got.Name != "Aboleth" {
		t.Fatalf("expected name Aboleth, got %s", got.Name)
	}
	if !got.Source.Valid || got.Source.String != "open5e:tome-of-beasts" {
		t.Fatalf("expected source open5e:tome-of-beasts, got %+v", got.Source)
	}

	// (3) Create a campaign with tome-of-beasts enabled.
	campaignSvc := campaign.NewService(queries, nil)
	c, err := campaignSvc.CreateCampaign(ctx, "guild-999", "dm-1", "Integration", &campaign.Settings{
		TurnTimeoutHours: 24,
		DiagonalRule:     "standard",
		Open5eSources:    []string{"tome-of-beasts"},
	})
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	// (4) Use CampaignSourceLookup + statblocklibrary to verify the cached
	// Open5e row is only visible when the campaign enables the document.
	statSvc := statblocklibrary.NewService(queries)
	lookup := open5e.NewCampaignSourceLookup(queries)
	enabled := lookup.EnabledOpen5eSources(c.ID)
	if len(enabled) != 1 || enabled[0] != "tome-of-beasts" {
		t.Fatalf("expected tome-of-beasts enabled, got %v", enabled)
	}

	entry, err := statSvc.GetStatBlockWithSources(ctx, id, c.ID, enabled)
	if err != nil {
		t.Fatalf("GetStatBlockWithSources (enabled): %v", err)
	}
	if entry.Name != "Aboleth" {
		t.Fatalf("expected Aboleth, got %s", entry.Name)
	}

	// Same campaign, different (disabled) slug list hides the row.
	_, err = statSvc.GetStatBlockWithSources(ctx, id, c.ID, []string{"deep-magic"})
	if err == nil {
		t.Fatal("expected ErrNotFound when slug is not enabled")
	}

	// ListStatBlocks also applies the filter.
	// Use a uuid.Nil campaign with no Open5e list → row hidden.
	list, err := statSvc.ListStatBlocks(ctx, statblocklibrary.StatBlockFilter{})
	if err != nil {
		t.Fatalf("ListStatBlocks: %v", err)
	}
	for _, e := range list {
		if e.ID == id {
			t.Fatal("Open5e row should be hidden when no sources enabled")
		}
	}

	list, err = statSvc.ListStatBlocks(ctx, statblocklibrary.StatBlockFilter{
		CampaignID:           c.ID,
		EnabledOpen5eSources: enabled,
	})
	if err != nil {
		t.Fatalf("ListStatBlocks (enabled): %v", err)
	}
	found := false
	for _, e := range list {
		if e.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected Open5e row visible when tome-of-beasts enabled")
	}
}
