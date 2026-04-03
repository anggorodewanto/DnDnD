package shops

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts shop API routes on the given Chi router.
func RegisterRoutes(r chi.Router, h *Handler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/campaigns/{campaignID}/shops", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/", h.HandleListShops)
		r.Post("/", h.HandleCreateShop)
		r.Route("/{shopID}", func(r chi.Router) {
			r.Get("/", h.HandleGetShop)
			r.Put("/", h.HandleUpdateShop)
			r.Delete("/", h.HandleDeleteShop)
			r.Post("/post", h.HandlePostToDiscord)
			r.Route("/items", func(r chi.Router) {
				r.Post("/", h.HandleAddItem)
				r.Route("/{itemID}", func(r chi.Router) {
					r.Put("/", h.HandleUpdateItem)
					r.Delete("/", h.HandleRemoveItem)
				})
			})
		})
	})
}
