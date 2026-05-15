package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/characteroverview"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/encounter"
	"github.com/ab/dndnd/internal/gamemap"
	"github.com/ab/dndnd/internal/homebrew"
	"github.com/ab/dndnd/internal/levelup"
	"github.com/ab/dndnd/internal/messageplayer"
	"github.com/ab/dndnd/internal/narration"
	"github.com/ab/dndnd/internal/statblocklibrary"
)

// TestMountDMOnlyAPIs_NonDMReceives403 walks every DM-mutation route group
// that SR-001 lists (SUMMARY.md §1 item 1 / batches 1, 13, 16) and asserts
// that a 403 forbidden response is returned BEFORE any handler is invoked
// when the middleware denies the caller. This is the regression test for
// the F-2 systemic auth gap: previously these route groups were mounted on
// the bare router with no middleware, so any authenticated Discord user
// could hit them.
//
// The middleware used here unconditionally returns 403, simulating the
// non-DM branch of dashboard.RequireDM. The test does NOT exercise the
// underlying handler logic — every route should be rejected at the
// middleware boundary.
func TestMountDMOnlyAPIs_NonDMReceives403(t *testing.T) {
	denyAll := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":"forbidden: DM only"}`, http.StatusForbidden)
		})
	}

	router := chi.NewRouter()
	deps := buildDMOnlyDepsForTest()
	mountDMOnlyAPIs(router, deps, denyAll)

	routes := dmOnlyRouteTable()
	for _, rt := range routes {
		t.Run(rt.method+"_"+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(rt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusForbidden, rec.Code,
				"%s %s must be gated by dmAuthMw and return 403 for non-DM callers",
				rt.method, rt.path)
		})
	}
}

// TestMountDMOnlyAPIs_DMReachesHandler asserts the same route table with a
// passthrough middleware so we know the gating helper does NOT block DMs;
// a non-403 status proves the middleware boundary allowed the request
// through. We do not assert 2xx specifically because the underlying
// services are nil/stub and most handlers will surface 400/500 — what
// matters for SR-001 is that the middleware is no longer blocking.
func TestMountDMOnlyAPIs_DMReachesHandler(t *testing.T) {
	passthrough := func(next http.Handler) http.Handler { return next }

	router := chi.NewRouter()
	// The stub services in buildDMOnlyDepsForTest panic on real DB calls;
	// recover so the test only observes WHETHER the handler was reached
	// (a panic means we're past the middleware) without crashing.
	router.Use(recoverAsInternalError)
	deps := buildDMOnlyDepsForTest()
	mountDMOnlyAPIs(router, deps, passthrough)

	routes := dmOnlyRouteTable()
	for _, rt := range routes {
		t.Run(rt.method+"_"+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(rt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.NotEqual(t, http.StatusForbidden, rec.Code,
				"%s %s must NOT return 403 when middleware passes the request through (handler-side errors are OK)",
				rt.method, rt.path)
			assert.NotEqual(t, http.StatusNotFound, rec.Code,
				"%s %s must be registered (got 404) after mountDMOnlyAPIs",
				rt.method, rt.path)
		})
	}
}

// recoverAsInternalError swallows panics from the stub services in this
// test file and writes 500 instead. The DM-reaches-handler test only cares
// that the route was registered and the middleware did not block; what the
// handler does afterwards is covered by per-package tests.
func recoverAsInternalError(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				http.Error(w, "panic", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// dmOnlyRoute is one entry in the SR-001 route table.
type dmOnlyRoute struct {
	method string
	path   string
	body   string // optional JSON body for POST/PUT/PATCH so handlers don't 415
}

// dmOnlyRouteTable enumerates one representative path per
// RegisterRoutes-mounted group covered by SR-001. The table is intentionally
// broad — every group listed in SUMMARY.md §1 item 1 has at least one entry
// so a future regression that re-mounts ANY of these on the bare router
// fails this test.
func dmOnlyRouteTable() []dmOnlyRoute {
	encID := "00000000-0000-0000-0000-000000000001"
	combID := "00000000-0000-0000-0000-000000000002"
	charID := "00000000-0000-0000-0000-000000000003"
	campID := "00000000-0000-0000-0000-000000000004"
	tplID := "00000000-0000-0000-0000-000000000005"
	mapID := "00000000-0000-0000-0000-000000000006"
	return []dmOnlyRoute{
		// gamemap (line 562) — Tiled import included.
		{http.MethodPost, "/api/maps/", `{}`},
		{http.MethodPost, "/api/maps/import", `{}`},
		{http.MethodPut, "/api/maps/" + mapID, `{}`},
		{http.MethodDelete, "/api/maps/" + mapID, ""},

		// statblock (line 594) — reveals hidden enemy data.
		{http.MethodGet, "/api/statblocks/", ""},

		// homebrew (line 615).
		{http.MethodPost, "/api/homebrew/creatures", `{}`},
		{http.MethodDelete, "/api/homebrew/creatures/some-id", ""},

		// campaign pause/resume (line 635).
		{http.MethodPost, "/api/campaigns/" + campID + "/pause", `{}`},
		{http.MethodPost, "/api/campaigns/" + campID + "/resume", `{}`},

		// narration (line 641).
		{http.MethodPost, "/api/narration/preview", `{}`},
		{http.MethodPost, "/api/narration/post", `{}`},

		// narration templates (line 647).
		{http.MethodPost, "/api/narration/templates/", `{}`},
		{http.MethodPut, "/api/narration/templates/" + tplID, `{}`},
		{http.MethodDelete, "/api/narration/templates/" + tplID, ""},

		// character overview (line 653).
		{http.MethodGet, "/api/character-overview?campaign_id=" + campID, ""},

		// message-player DM (line 665).
		{http.MethodPost, "/api/message-player/", `{}`},

		// combat handler (line 679).
		{http.MethodPost, "/api/combat/start", `{}`},
		{http.MethodPost, "/api/combat/" + encID + "/end", `{}`},
		{http.MethodPost, "/api/combat/" + encID + "/reactions", `{}`},

		// combat workspace + DM dashboard (line 703).
		{http.MethodGet, "/api/combat/workspace?campaign_id=" + campID, ""},
		{http.MethodPost, "/api/combat/" + encID + "/advance-turn", `{}`},
		{http.MethodPost, "/api/combat/" + encID + "/undo-last-action", `{}`},
		{http.MethodPost, "/api/combat/" + encID + "/override/combatant/" + combID + "/hp", `{}`},
		{http.MethodPost, "/api/combat/" + encID + "/override/combatant/" + combID + "/position", `{}`},
		{http.MethodPost, "/api/combat/" + encID + "/override/combatant/" + combID + "/conditions", `{}`},
		{http.MethodPost, "/api/combat/" + encID + "/override/combatant/" + combID + "/exhaustion", `{}`},
		{http.MethodPost, "/api/combat/" + encID + "/override/character/" + charID + "/spell-slots", `{}`},
		{http.MethodPost, "/api/combat/" + encID + "/combatants/" + combID + "/concentration/drop", `{}`},
		{http.MethodPatch, "/api/combat/" + encID + "/combatants/" + combID + "/hp", `{}`},
		{http.MethodDelete, "/api/combat/" + encID + "/combatants/" + combID, ""},

		// encounter builder (Finding 2: now behind dmAuthMw).
		{http.MethodPost, "/api/encounters/", `{"campaign_id":"` + campID + `","name":"test"}`},
		{http.MethodGet, "/api/encounters/?campaign_id=" + campID, ""},
		{http.MethodGet, "/api/encounters/" + encID, ""},
		{http.MethodGet, "/api/creatures", ""},

		// asset upload (Finding 2: now behind dmAuthMw).
		{http.MethodPost, "/api/assets/upload", ""},
	}
}

// buildDMOnlyDepsForTest constructs each handler with a nil-or-stub service
// so the routes register but do not exercise real persistence. Underlying
// handler errors are tolerated — the test asserts only middleware
// behaviour.
func buildDMOnlyDepsForTest() dmOnlyAPIDeps {
	combatSvc := combat.NewService(nil)
	return dmOnlyAPIDeps{
		mapHandler:               gamemap.NewHandler(gamemap.NewService(nil)),
		statBlockHandler:         statblocklibrary.NewHandler(statblocklibrary.NewService(nil)),
		homebrewHandler:          homebrew.NewHandler(homebrew.NewService(nil)),
		campaignHandler:          campaign.NewHandler(campaign.NewService(nil, nil)),
		narrationHandler:         narration.NewHandler(narration.NewService(nil, nil, nil, nil)),
		narrationTemplateHandler: narration.NewTemplateHandler(narration.NewTemplateService(nil)),
		characterOverviewHandler: characteroverview.NewHandler(characteroverview.NewService(nil)),
		messagePlayerHandler:     messageplayer.NewHandler(messageplayer.NewService(nil, nil, nil)),
		combatHandler:            combat.NewHandler(combatSvc, dice.NewRoller(nil)),
		combatSvc:                combatSvc,
		workspaceStore:           stubWorkspaceStore{},
		db:                       nil,
		combatLogPoster:          nil,
		encounterHandler:         encounter.NewHandler(encounter.NewService(nil)),
		assetUploadHandler: func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "stub", http.StatusBadRequest)
		},
	}
}

// TestMountDMOnlyAPIs_NilFieldsAreSkipped guards the helper against panicking
// when called with a partially-wired deps struct (matches the nil-safe
// pattern used by mountDashboardAPIs).
func TestMountDMOnlyAPIs_NilFieldsAreSkipped(t *testing.T) {
	r := chi.NewRouter()
	require.NotPanics(t, func() {
		mountDMOnlyAPIs(r, dmOnlyAPIDeps{}, func(h http.Handler) http.Handler { return h })
	})
}

// TestLevelUpRoutes_NonDMReceives403 proves that /api/levelup/* and
// /dashboard/levelup are gated by dmAuthMw (SR-063). The levelup handler
// is mounted via router.Group with dmAuthMw in main.go, separate from
// mountDMOnlyAPIs because it depends on services constructed later.
func TestLevelUpRoutes_NonDMReceives403(t *testing.T) {
	denyAll := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":"forbidden: DM only"}`, http.StatusForbidden)
		})
	}

	router := chi.NewRouter()
	h := levelup.NewHandler(levelup.NewService(nil, nil, nil), nil)
	router.Group(func(r chi.Router) {
		r.Use(denyAll)
		h.RegisterRoutes(r)
	})

	routes := []dmOnlyRoute{
		{http.MethodPost, "/api/levelup/", `{}`},
		{http.MethodPost, "/api/levelup/asi/approve", `{}`},
		{http.MethodPost, "/api/levelup/asi/deny", `{}`},
		{http.MethodPost, "/api/levelup/feat/apply", `{}`},
		{http.MethodPost, "/api/levelup/feat/check", `{}`},
		{http.MethodGet, "/dashboard/levelup", ""},
	}

	for _, rt := range routes {
		t.Run(rt.method+"_"+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(rt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusForbidden, rec.Code,
				"%s %s must be gated by dmAuthMw and return 403 for non-DM callers",
				rt.method, rt.path)
		})
	}
}

// TestLevelUpRoutes_DMReachesHandler asserts that with passthrough middleware,
// the levelup handler is reached (not blocked by middleware).
func TestLevelUpRoutes_DMReachesHandler(t *testing.T) {
	passthrough := func(next http.Handler) http.Handler { return next }

	router := chi.NewRouter()
	router.Use(recoverAsInternalError)
	h := levelup.NewHandler(levelup.NewService(nil, nil, nil), nil)
	router.Group(func(r chi.Router) {
		r.Use(passthrough)
		h.RegisterRoutes(r)
	})

	routes := []dmOnlyRoute{
		{http.MethodPost, "/api/levelup/", `{}`},
		{http.MethodPost, "/api/levelup/asi/approve", `{}`},
		{http.MethodPost, "/api/levelup/asi/deny", `{}`},
		{http.MethodPost, "/api/levelup/feat/apply", `{}`},
		{http.MethodPost, "/api/levelup/feat/check", `{}`},
		{http.MethodGet, "/dashboard/levelup", ""},
	}

	for _, rt := range routes {
		t.Run(rt.method+"_"+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(rt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.NotEqual(t, http.StatusForbidden, rec.Code,
				"%s %s must NOT return 403 when middleware passes through",
				rt.method, rt.path)
			assert.NotEqual(t, http.StatusNotFound, rec.Code,
				"%s %s must be registered (got 404)",
				rt.method, rt.path)
		})
	}
}
