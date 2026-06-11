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
	oauthSvc       *auth.OAuthService
	apiH           *APIHandler
	sheetH         *CharacterSheetHandler
	prepH          *PreparationHandler
	csrfMiddleware func(http.Handler) http.Handler
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

// WithCharacterSheet adds the character sheet handler.
func WithCharacterSheet(h *CharacterSheetHandler) RouteOption {
	return func(cfg *routeConfig) {
		cfg.sheetH = h
	}
}

// WithSpellPreparation adds the logged-in web spell-preparation page shell
// (/portal/character/{characterID}/prepare) and its JSON API endpoints
// (/portal/api/characters/{characterID}/preparation). Routes are only mounted
// when this option is provided, so go build stays green before wiring.
func WithSpellPreparation(h *PreparationHandler) RouteOption {
	return func(cfg *routeConfig) {
		cfg.prepH = h
	}
}

// WithCSRFMiddleware adds origin-check CSRF protection to mutating API routes.
func WithCSRFMiddleware(mw func(http.Handler) http.Handler) RouteOption {
	return func(cfg *routeConfig) {
		cfg.csrfMiddleware = mw
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

			// Character sheet route
			if cfg.sheetH != nil {
				r.Get("/character/{characterID}", cfg.sheetH.ServeCharacterSheet)
			}

			// Spell-preparation page shell (only when wired).
			if cfg.prepH != nil {
				r.Get("/character/{characterID}/prepare", h.ServePrepare)
			}

			// API routes
			if cfg.apiH != nil || cfg.prepH != nil {
				r.Route("/api", func(r chi.Router) {
					if cfg.csrfMiddleware != nil {
						r.Use(cfg.csrfMiddleware)
					}
					if cfg.apiH != nil {
						r.Get("/races", cfg.apiH.ListRaces)
						r.Get("/classes", cfg.apiH.ListClasses)
						r.Get("/spells", cfg.apiH.ListSpells)
						r.Get("/equipment", cfg.apiH.ListEquipment)
						r.Get("/starting-equipment", cfg.apiH.GetStartingEquipment)
						r.Get("/ability-methods", cfg.apiH.ListAbilityMethods)
						r.Get("/characters/draft", cfg.apiH.GetCharacterDraft)
						r.Post("/characters", cfg.apiH.SubmitCharacter)
						r.Post("/characters/preview", cfg.apiH.PreviewCharacter)
					}
					if cfg.prepH != nil {
						r.Get("/characters/{characterID}/preparation", cfg.prepH.GetPreparation)
						r.Post("/characters/{characterID}/preparation", cfg.prepH.PostPreparation)
					}
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
