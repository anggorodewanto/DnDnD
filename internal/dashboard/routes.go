package dashboard

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ab/dndnd/internal/inventory"
)

// RegisterRoutes mounts dashboard routes on the given Chi router.
// The authMiddleware should validate the session and inject the discord user ID.
func RegisterRoutes(r chi.Router, h *Handler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/dashboard", func(r chi.Router) {
		r.Use(authMiddleware)

		r.Get("/", h.ServeDashboard)
		r.Get("/ws", h.ServeWebSocket)

		// Serve the Svelte SPA stub from embedded assets
		assetsFS, err := fs.Sub(Assets, "assets")
		if err != nil {
			panic("dashboard: failed to create assets sub-filesystem: " + err.Error())
		}
		fileServer := http.FileServer(http.FS(assetsFS))
		r.Get("/app/*", http.StripPrefix("/dashboard/app", fileServer).ServeHTTP)
	})
}

// RegisterInventoryAPI mounts the DM inventory management API endpoints.
func RegisterInventoryAPI(r chi.Router, invHandler *inventory.APIHandler, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/inventory", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Post("/add", invHandler.HandleAddItem)
		r.Post("/remove", invHandler.HandleRemoveItem)
		r.Post("/transfer", invHandler.HandleTransferItem)
		r.Post("/gold", invHandler.HandleSetGold)
	})
}

// MountDashboard is a convenience function that creates a Handler, sets up the hub,
// and registers all dashboard routes on the given router.
func MountDashboard(r chi.Router, logger *slog.Logger, hub *Hub, authMiddleware func(http.Handler) http.Handler) *Handler {
	h := NewHandler(logger, hub)
	RegisterRoutes(r, h, authMiddleware)
	return h
}
