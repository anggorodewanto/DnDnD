package auth

import (
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts OAuth2 routes on the given Chi router.
// Routes: GET /auth/login, GET /auth/callback, POST /auth/logout
func RegisterRoutes(r chi.Router, svc *OAuthService) {
	r.Route("/auth", func(r chi.Router) {
		r.Get("/login", svc.HandleLogin)
		r.Get("/callback", svc.HandleCallback)
		r.Post("/logout", svc.HandleLogout)
	})
}
