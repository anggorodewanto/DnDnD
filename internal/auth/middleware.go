package auth

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

type contextKey string

const discordUserIDKey contextKey = "discord_user_id"

// DiscordUserIDFromContext returns the Discord user ID from the request context.
func DiscordUserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(discordUserIDKey).(string)
	return id, ok
}

// ContextWithDiscordUserID returns a new context with the Discord user ID set.
// This is useful for testing and internal service calls.
func ContextWithDiscordUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, discordUserIDKey, userID)
}

// SessionMiddleware validates session cookies and injects the Discord user ID
// into the request context. It also handles transparent token refresh and TTL sliding.
func SessionMiddleware(sessions SessionRepository, refresher TokenRefresher, logger *slog.Logger, secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(CookieName)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			sessionID, err := uuid.Parse(cookie.Value)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			sess, err := sessions.GetByID(r.Context(), sessionID)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			// Transparent token refresh if token is expired
			if sess.TokenExpiresAt != nil && sess.TokenExpiresAt.Before(time.Now()) {
				if err := refreshToken(r.Context(), sess, sessions, refresher, logger); err != nil {
					// Refresh failed — delete session and force re-auth
					logger.Warn("token refresh failed, deleting session", "error", err, "session_id", sess.ID)
					_ = sessions.Delete(r.Context(), sessionID)
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
			}

			// Slide TTL
			if err := sessions.SlideTTL(r.Context(), sessionID); err != nil {
				logger.Error("failed to slide session TTL", "error", err, "session_id", sessionID)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			// Re-issue cookie with refreshed MaxAge so browser and DB stay in sync
			http.SetCookie(w, &http.Cookie{
				Name:     CookieName,
				Value:    sessionID.String(),
				Path:     "/",
				MaxAge:   int(SessionTTL.Seconds()),
				HttpOnly: true,
				Secure:   secure,
				SameSite: http.SameSiteLaxMode,
			})

			ctx := context.WithValue(r.Context(), discordUserIDKey, sess.DiscordUserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func refreshToken(ctx context.Context, sess *Session, sessions SessionRepository, refresher TokenRefresher, logger *slog.Logger) error {
	oldToken := &oauth2.Token{
		AccessToken:  sess.AccessToken,
		RefreshToken: sess.RefreshToken,
		Expiry:       *sess.TokenExpiresAt,
	}

	src := refresher.TokenSource(ctx, oldToken)
	newToken, err := src.Token()
	if err != nil {
		return err
	}

	if err := sessions.UpdateTokens(ctx, sess.ID, newToken.AccessToken, newToken.RefreshToken, &newToken.Expiry); err != nil {
		return err
	}

	logger.Info("token refreshed", "session_id", sess.ID)
	return nil
}
