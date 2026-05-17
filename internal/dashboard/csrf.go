package dashboard

import (
	"net/http"
	"net/url"
	"strings"
)

// OriginCheckMiddleware rejects mutating requests (POST/PUT/PATCH/DELETE) whose
// Origin header doesn't match the expected host. This mitigates CSRF attacks
// that SameSite=Lax doesn't cover (top-level POST navigations).
// When allowedHost is empty, the check is skipped (dev mode).
func OriginCheckMiddleware(allowedHost string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if allowedHost == "" {
				next.ServeHTTP(w, r)
				return
			}
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			origin := r.Header.Get("Origin")
			if origin == "" {
				// No Origin header (same-origin requests from some browsers).
				// Fall back to Referer check.
				ref := r.Header.Get("Referer")
				if ref != "" {
					if u, err := url.Parse(ref); err == nil {
						origin = u.Scheme + "://" + u.Host
					}
				}
			}
			if origin == "" {
				// No Origin or Referer — allow (non-browser client).
				next.ServeHTTP(w, r)
				return
			}
			u, err := url.Parse(origin)
			if err != nil {
				http.Error(w, "invalid origin", http.StatusForbidden)
				return
			}
			if !strings.EqualFold(u.Host, allowedHost) {
				http.Error(w, "origin mismatch", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
