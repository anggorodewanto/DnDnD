package exploration_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/database"
	"github.com/ab/dndnd/internal/exploration"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/testutil"
)

// TestIntegration_EncounterModeColumn verifies the Phase 110 migration adds the
// mode column with correct default and CHECK constraint, and that the new
// CreateExplorationEncounter and UpdateEncounterMode queries behave correctly.
func TestIntegration_EncounterModeColumn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)
	require.NoError(t, database.MigrateUp(db, dbfs.Migrations))

	q := refdata.New(db)
	ctx := context.Background()

	campaignID := uuid.New()
	_, err := db.Exec(`INSERT INTO campaigns (id, guild_id, dm_user_id, name, status) VALUES ($1, $2, $3, $4, $5)`,
		campaignID, "g-mode", "dm", "Mode Campaign", "active")
	require.NoError(t, err)

	// Existing combat encounter: mode should default to 'combat'.
	combatEnc, err := q.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID:  campaignID,
		Name:        "combat encounter",
		Status:      "preparing",
		RoundNumber: 0,
	})
	require.NoError(t, err)
	require.Equal(t, "combat", combatEnc.Mode)

	// CreateExplorationEncounter: mode = 'exploration', status = 'active'.
	mapID := uuid.New()
	_, err = db.Exec(`INSERT INTO maps (id, campaign_id, name, width_squares, height_squares, tiled_json, tileset_refs) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		mapID, campaignID, "map1", 10, 10, json.RawMessage(`{"width":10,"height":10,"tilewidth":48,"tileheight":48,"layers":[]}`), json.RawMessage(`[]`))
	require.NoError(t, err)

	expEnc, err := q.CreateExplorationEncounter(ctx, refdata.CreateExplorationEncounterParams{
		CampaignID: campaignID,
		MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
		Name:       "exploration encounter",
	})
	require.NoError(t, err)
	require.Equal(t, "exploration", expEnc.Mode)
	require.Equal(t, "active", expEnc.Status)
	require.Equal(t, int32(0), expEnc.RoundNumber)

	// UpdateEncounterMode: flip exploration -> combat (used during transition).
	flipped, err := q.UpdateEncounterMode(ctx, refdata.UpdateEncounterModeParams{
		ID:   expEnc.ID,
		Mode: "combat",
	})
	require.NoError(t, err)
	require.Equal(t, "combat", flipped.Mode)

	// CHECK constraint rejects invalid mode.
	_, err = q.UpdateEncounterMode(ctx, refdata.UpdateEncounterModeParams{
		ID:   expEnc.ID,
		Mode: "nonsense",
	})
	require.Error(t, err, "expected CHECK constraint to reject invalid mode")
}

func TestIntegration_DashboardTransitionToCombatFlipsModeInDB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)
	require.NoError(t, database.MigrateUp(db, dbfs.Migrations))

	q := refdata.New(db)
	ctx := context.Background()

	campaign := testutil.NewTestCampaign(t, q, "g-transition")
	gameMap := testutil.NewTestMap(t, q, campaign.ID)
	character := testutil.NewTestCharacter(t, q, campaign.ID, "Clara", 1)
	encounter, err := q.CreateExplorationEncounter(ctx, refdata.CreateExplorationEncounterParams{
		CampaignID: campaign.ID,
		MapID:      uuid.NullUUID{UUID: gameMap.ID, Valid: true},
		Name:       "Exploring",
	})
	require.NoError(t, err)
	_ = testutil.NewTestCombatant(t, q, encounter.ID, character.ID)

	svc := exploration.NewService(q)
	h := exploration.NewDashboardHandler(svc, q)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	form := url.Values{}
	form.Set("encounter_id", encounter.ID.String())
	resp, err := http.PostForm(srv.URL+"/dashboard/exploration/transition-to-combat", form)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))

	var out struct {
		Mode string `json:"mode"`
	}
	require.NoError(t, json.Unmarshal(body, &out))
	require.Equal(t, "combat", out.Mode)

	stored, err := q.GetEncounter(ctx, encounter.ID)
	require.NoError(t, err)
	require.Equal(t, "combat", stored.Mode)
}
