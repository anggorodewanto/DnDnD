package dashboard

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ab/dndnd/internal/auth"
)

// DMVerifier resolves whether the authenticated Discord user owns at least
// one non-archived campaign as the DM. The simplest production implementation
// (see cmd/dndnd/main.go dashboardCampaignLookup) scans campaigns where
// dm_user_id matches the user id and returns true when any row remains after
// the "archived" filter. Implementations should treat a missing/unknown user
// as "not the DM" and either return (false, nil) or an error.
//
// F-2: per-request DM verification per docs/dnd-async-discord-spec.md line
// 65 — "System verifies the authenticated Discord user ID matches the
// campaign's designated DM."
type DMVerifier interface {
	IsDM(ctx context.Context, discordUserID string) (bool, error)
}

// RequireDM returns an http middleware that rejects requests whose Discord
// user (as injected by auth.SessionMiddleware) is not a DM. It MUST sit
// downstream of SessionMiddleware so the user-id context key is populated.
//
// On reject (no session, verifier error, or "not a DM") the middleware emits
// a 403 with body {"error": "forbidden: DM only"} and DOES NOT invoke next.
//
// devPassthrough is a marker interface implemented by DevDMVerifier to signal
// that RequireDM should skip user-ID checks (local dev without OAuth).
type devPassthrough interface {
	isDevPassthrough()
}

// A nil verifier rejects all requests (returns 403). Production deploys must
// always supply a verifier; local-dev wiring should use DevDMVerifier (which
// approves all requests) so developers are never locked out while still
// ensuring that a misconfigured production deploy cannot silently skip DM
// authorization.
func RequireDM(verifier DMVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if verifier == nil {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeDMForbidden(w)
			})
		}
		// DevDMVerifier: full passthrough for local dev (no user ID required).
		if _, ok := verifier.(devPassthrough); ok {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := auth.DiscordUserIDFromContext(r.Context())
			if !ok || userID == "" {
				writeDMForbidden(w)
				return
			}

			isDM, err := verifier.IsDM(r.Context(), userID)
			if err != nil || !isDM {
				writeDMForbidden(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// DevDMVerifier is a DMVerifier that always returns true. It is used in local
// dev mode (when DISCORD_CLIENT_ID is unset) so the developer is never locked
// out of DM-only routes while still ensuring RequireDM(nil) rejects in
// production if a verifier is accidentally omitted.
type DevDMVerifier struct{}

func (DevDMVerifier) IsDM(_ context.Context, _ string) (bool, error) { return true, nil }

// isDevPassthrough is a marker method that RequireDM checks to skip the
// user-ID-in-context requirement. In local dev, passthroughMiddleware does
// not inject a Discord user ID, so RequireDM must not require one.
func (DevDMVerifier) isDevPassthrough() {}

// writeDMForbidden emits the canonical 403 JSON payload. Centralised so the
// wire format stays consistent across every gated route.
func writeDMForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "forbidden: DM only"})
}
