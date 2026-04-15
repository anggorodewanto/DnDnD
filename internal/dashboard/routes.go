package dashboard

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ab/dndnd/internal/dmqueue"
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

// RegisterDMQueueRoutes mounts the dm-queue resolver pages.
// Phase 106a: minimal viable — list pending items, view one, mark it resolved.
func RegisterDMQueueRoutes(r chi.Router, logger *slog.Logger, notifier dmqueue.Notifier, authMiddleware func(http.Handler) http.Handler) {
	h := NewDMQueueHandler(logger, notifier)
	r.Route("/dashboard/queue", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/{itemID}", h.ServeItem)
		r.Post("/{itemID}/resolve", h.HandleResolve)
		r.Post("/{itemID}/reply", h.HandleWhisperReply)
		r.Post("/{itemID}/narrate", h.HandleSkillCheckNarration)
	})
}

// ExplorationHandler is the narrow surface of the exploration dashboard
// handler consumed by RegisterExplorationRoutes. Keeping the dependency a
// pair of http.Handler methods avoids a circular import on exploration.
type ExplorationHandler interface {
	ServePage(w http.ResponseWriter, r *http.Request)
	HandleStart(w http.ResponseWriter, r *http.Request)
}

// RegisterExplorationRoutes mounts the Phase 110 exploration dashboard pages
// (Q4a: reachable from DM dashboard) behind authMiddleware so they are not
// publicly reachable.
func RegisterExplorationRoutes(r chi.Router, h ExplorationHandler, authMiddleware func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/dashboard/exploration", h.ServePage)
		r.Post("/dashboard/exploration/start", h.HandleStart)
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
		r.Post("/identify", invHandler.HandleIdentifyItem)
	})
}

// MountDashboard is a convenience function that creates a Handler, sets up the hub,
// and registers all dashboard routes on the given router.
func MountDashboard(r chi.Router, logger *slog.Logger, hub *Hub, authMiddleware func(http.Handler) http.Handler) *Handler {
	h := NewHandler(logger, hub)
	RegisterRoutes(r, h, authMiddleware)
	return h
}
