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
// A nil verifier disables the gate (passthrough) — the local-dev wiring in
// main.go uses passthroughMiddleware for auth when DISCORD_CLIENT_ID is unset,
// and we want RequireDM to follow the same fall-back rather than locking the
// developer out.
func RequireDM(verifier DMVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if verifier == nil {
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

// writeDMForbidden emits the canonical 403 JSON payload. Centralised so the
// wire format stays consistent across every gated route.
func writeDMForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "forbidden: DM only"})
}
