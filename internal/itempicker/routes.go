package itempicker

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts item picker API routes on the given Chi router.
func RegisterRoutes(r chi.Router, h *Handler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/campaigns/{campaignID}/items", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/search", h.HandleSearch)
	})
	r.Route("/api/campaigns/{campaignID}/encounters/{encounterID}/creature-inventories", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/", h.HandleCreatureInventories)
	})
}
