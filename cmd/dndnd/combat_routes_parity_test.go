package main

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/combat"
)

// walkRouteSet returns the set of "METHOD path" entries registered on r.
func walkRouteSet(t *testing.T, r chi.Router) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		out[method+" "+route] = true
		return nil
	})
	require.NoError(t, err)
	return out
}

// The DM combat-dashboard routes live in TWO hand-synced places: the
// production mount mountCombatDashboardRoutes (what actually serves traffic)
// and combat.DMDashboardHandler.RegisterRoutes (what the combat package's
// own handler tests exercise). They drifted once — ISSUE-043's pending-saves
// routes were added to RegisterRoutes but not to the production mount, so the
// endpoints 404'd in the running app while the unit tests passed. This guards
// that the DM route surface stays identical in both, so adding a route to one
// without the other fails CI instead of in live play.
func TestCombatDashboardRouteParity(t *testing.T) {
	svc := combat.NewService(nil)

	// Production mount with a nil workspaceStore registers only the DM (dm.*)
	// routes — the same surface RegisterRoutes covers.
	prodRouter := chi.NewRouter()
	mountCombatDashboardRoutes(prodRouter, svc, nil, nil, nil)
	prod := walkRouteSet(t, prodRouter)

	regRouter := chi.NewRouter()
	combat.NewDMDashboardHandler(svc).RegisterRoutes(regRouter)
	reg := walkRouteSet(t, regRouter)

	assert.Equal(t, reg, prod,
		"DM combat-dashboard routes drifted between combat.DMDashboardHandler.RegisterRoutes "+
			"and mountCombatDashboardRoutes — add the route to BOTH")
}
