package portal

import (
	"io/fs"
	"net/http"

	"github.com/ab/dndnd/internal/auth"
	"github.com/go-chi/chi/v5"
)

// RouteOption configures optional features of portal routes.
type RouteOption func(*routeConfig)

type routeConfig struct {
	oauthSvc *auth.OAuthService
	apiH     *APIHandler
}

// WithOAuth adds OAuth2 login/callback/logout routes to the portal.
func WithOAuth(svc *auth.OAuthService) RouteOption {
	return func(cfg *routeConfig) {
		cfg.oauthSvc = svc
	}
}

// WithAPI adds the portal API handler for reference data and character submission.
func WithAPI(h *APIHandler) RouteOption {
	return func(cfg *routeConfig) {
		cfg.apiH = h
	}
}

// RegisterRoutes mounts portal routes on the given Chi router.
// The authMiddleware should validate the session and inject the discord user ID.
func RegisterRoutes(r chi.Router, h *Handler, authMiddleware func(handler http.Handler) http.Handler, opts ...RouteOption) {
	cfg := &routeConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	r.Route("/portal", func(r chi.Router) {
		// OAuth routes (no auth required)
		if cfg.oauthSvc != nil {
			r.Get("/auth/login", cfg.oauthSvc.HandleLogin)
			r.Get("/auth/callback", cfg.oauthSvc.HandleCallback)
			r.Post("/auth/logout", cfg.oauthSvc.HandleLogout)
		}

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)
			r.Get("/", h.ServeLanding)
			r.Get("/create", h.ServeCreate)

			// API routes
			if cfg.apiH != nil {
				r.Route("/api", func(r chi.Router) {
					r.Get("/races", cfg.apiH.ListRaces)
					r.Get("/classes", cfg.apiH.ListClasses)
					r.Get("/spells", cfg.apiH.ListSpells)
					r.Get("/equipment", cfg.apiH.ListEquipment)
					r.Get("/starting-equipment", cfg.apiH.GetStartingEquipment)
					r.Post("/characters", cfg.apiH.SubmitCharacter)
				})
			}
		})

		// Serve built Svelte assets (no auth required for static files)
		assetsFS, err := fs.Sub(Assets, "assets/assets")
		if err == nil {
			r.Handle("/app/assets/*", http.StripPrefix("/portal/app/assets/", http.FileServer(http.FS(assetsFS))))
		}
	})
}
