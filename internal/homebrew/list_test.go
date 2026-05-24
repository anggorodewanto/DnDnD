package homebrew

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// =============================================================================
// Service-level list filtering
// =============================================================================

// TestService_ListCreatures_FiltersByCampaignAndHomebrew verifies that a list
// returns only rows that are flagged homebrew AND owned by the requesting
// campaign — never SRD rows or another campaign's homebrew.
func TestService_ListCreatures_FiltersByCampaignAndHomebrew(t *testing.T) {
	store := newFakeStore()
	svc := newSvc(store)
	ctx := context.Background()
	campA, campB := uuid.New(), uuid.New()

	_, err := svc.CreateHomebrewCreature(ctx, campA, refdata.UpsertCreatureParams{Name: "A-Goblin"})
	require.NoError(t, err)
	_, err = svc.CreateHomebrewCreature(ctx, campA, refdata.UpsertCreatureParams{Name: "A-Orc"})
	require.NoError(t, err)
	_, err = svc.CreateHomebrewCreature(ctx, campB, refdata.UpsertCreatureParams{Name: "B-Kobold"})
	require.NoError(t, err)
	seedSRDCreature(store, "srd-dragon")

	got, err := svc.ListHomebrewCreatures(ctx, campA)
	require.NoError(t, err)
	require.Len(t, got, 2)
	for _, c := range got {
		assert.True(t, c.Homebrew.Bool, "listed entry must be homebrew")
		assert.Equal(t, campA, c.CampaignID.UUID, "listed entry must belong to campA")
	}
}

// TestService_ListCreatures_EmptyForUnknownCampaign returns a non-nil empty
// slice when the campaign owns no homebrew.
func TestService_ListCreatures_EmptyForUnknownCampaign(t *testing.T) {
	svc := newSvc(newFakeStore())
	got, err := svc.ListHomebrewCreatures(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

// TestService_List_InvalidCampaign rejects the zero UUID with ErrInvalidInput.
func TestService_List_InvalidCampaign(t *testing.T) {
	svc := newSvc(newFakeStore())
	_, err := svc.ListHomebrewSpells(context.Background(), uuid.Nil)
	require.ErrorIs(t, err, ErrInvalidInput)
}

// TestService_List_StoreError propagates the store's error.
func TestService_List_StoreError(t *testing.T) {
	store := newFakeStore()
	store.listErr = errors.New("boom")
	svc := newSvc(store)
	_, err := svc.ListHomebrewClasses(context.Background(), uuid.New())
	require.Error(t, err)
}

// TestService_List_AllTypes exercises each per-type List method once so a row
// created for the campaign comes back through every list path.
func TestService_List_AllTypes(t *testing.T) {
	store := newFakeStore()
	svc := newSvc(store)
	ctx := context.Background()
	cid := uuid.New()

	_, err := svc.CreateHomebrewCreature(ctx, cid, refdata.UpsertCreatureParams{Name: "c"})
	require.NoError(t, err)
	_, err = svc.CreateHomebrewSpell(ctx, cid, refdata.UpsertSpellParams{Name: "s"})
	require.NoError(t, err)
	_, err = svc.CreateHomebrewWeapon(ctx, cid, refdata.UpsertWeaponParams{Name: "w"})
	require.NoError(t, err)
	_, err = svc.CreateHomebrewMagicItem(ctx, cid, refdata.UpsertMagicItemParams{Name: "m"})
	require.NoError(t, err)
	_, err = svc.CreateHomebrewRace(ctx, cid, refdata.UpsertRaceParams{Name: "r"})
	require.NoError(t, err)
	_, err = svc.CreateHomebrewFeat(ctx, cid, refdata.UpsertFeatParams{Name: "f"})
	require.NoError(t, err)
	_, err = svc.CreateHomebrewClass(ctx, cid, refdata.UpsertClassParams{Name: "cl"})
	require.NoError(t, err)

	creatures, err := svc.ListHomebrewCreatures(ctx, cid)
	require.NoError(t, err)
	assert.Len(t, creatures, 1)
	spells, err := svc.ListHomebrewSpells(ctx, cid)
	require.NoError(t, err)
	assert.Len(t, spells, 1)
	weapons, err := svc.ListHomebrewWeapons(ctx, cid)
	require.NoError(t, err)
	assert.Len(t, weapons, 1)
	items, err := svc.ListHomebrewMagicItems(ctx, cid)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	races, err := svc.ListHomebrewRaces(ctx, cid)
	require.NoError(t, err)
	assert.Len(t, races, 1)
	feats, err := svc.ListHomebrewFeats(ctx, cid)
	require.NoError(t, err)
	assert.Len(t, feats, 1)
	classes, err := svc.ListHomebrewClasses(ctx, cid)
	require.NoError(t, err)
	assert.Len(t, classes, 1)
}

// =============================================================================
// Handler-level GET list routes
// =============================================================================

// idList decodes a JSON array of homebrew rows down to their ids.
func idList(t *testing.T, body []byte) []string {
	t.Helper()
	var rows []struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(body, &rows))
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids
}

// TestHandler_List_AllTypes creates one entry per type then lists it back over
// the GET route, asserting the created id is returned.
func TestHandler_List_AllTypes(t *testing.T) {
	routes := append([]typeRoute{{"creatures", "/api/homebrew/creatures"}}, allRoutes...)
	for _, tc := range routes {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore()
			r := newTestRouter(store)
			cid := uuid.New()

			rec := do(t, r, http.MethodPost, tc.path+"?campaign_id="+cid.String(),
				bodyForType(tc.name, "Listed"))
			require.Equal(t, http.StatusCreated, rec.Code, "create body=%s", rec.Body.String())
			var created struct {
				ID string `json:"id"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

			recL := do(t, r, http.MethodGet, tc.path+"?campaign_id="+cid.String(), nil)
			require.Equal(t, http.StatusOK, recL.Code, "list body=%s", recL.Body.String())
			assert.Contains(t, idList(t, recL.Body.Bytes()), created.ID)
		})
	}
}

// TestHandler_List_MissingCampaign rejects a list without campaign_id.
func TestHandler_List_MissingCampaign(t *testing.T) {
	r := newTestRouter(newFakeStore())
	rec := do(t, r, http.MethodGet, "/api/homebrew/spells", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestHandler_List_Empty returns 200 and a JSON array (not null) when the
// campaign owns nothing.
func TestHandler_List_Empty(t *testing.T) {
	r := newTestRouter(newFakeStore())
	rec := do(t, r, http.MethodGet, "/api/homebrew/weapons?campaign_id="+uuid.NewString(), nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, "[]", rec.Body.String())
}

// TestHandler_List_CampaignIsolation does not surface another campaign's
// homebrew.
func TestHandler_List_CampaignIsolation(t *testing.T) {
	store := newFakeStore()
	r := newTestRouter(store)
	campA, campB := uuid.New(), uuid.New()

	rec := do(t, r, http.MethodPost, "/api/homebrew/feats?campaign_id="+campA.String(),
		bodyForType("feats", "OnlyA"))
	require.Equal(t, http.StatusCreated, rec.Code)

	recB := do(t, r, http.MethodGet, "/api/homebrew/feats?campaign_id="+campB.String(), nil)
	require.Equal(t, http.StatusOK, recB.Code)
	assert.JSONEq(t, "[]", recB.Body.String())
}

// TestHandler_List_StoreError maps a store failure to 500.
func TestHandler_List_StoreError(t *testing.T) {
	store := newFakeStore()
	store.listErr = errors.New("boom")
	r := newTestRouter(store)
	rec := do(t, r, http.MethodGet, "/api/homebrew/races?campaign_id="+uuid.NewString(), nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
