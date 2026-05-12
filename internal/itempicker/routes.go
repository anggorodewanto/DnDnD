package itempicker

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts item picker API routes on the given Chi router.
// F-86: the picker now exposes a `custom` endpoint so DMs can register a
// freeform name/description/quantity/gold entry that the SRD reference
// tables don't carry.
func RegisterRoutes(r chi.Router, h *Handler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/campaigns/{campaignID}/items", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/search", h.HandleSearch)
		r.Post("/custom", h.HandleCustomEntry)
	})
	r.Route("/api/campaigns/{campaignID}/encounters/{encounterID}/creature-inventories", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/", h.HandleCreatureInventories)
	})
}
