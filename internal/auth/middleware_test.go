package auth_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ab/dndnd/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

type mockTokenSource struct {
	token *oauth2.Token
	err   error
}

func (m *mockTokenSource) Token() (*oauth2.Token, error) {
	return m.token, m.err
}

func TestSessionMiddleware_ValidSession(t *testing.T) {
	repo := newMockSessionRepo()
	tokenExp := time.Now().Add(time.Hour) // not expired
	sess, _ := repo.Create(context.Background(), "user42", "at", "rt", &tokenExp)

	tr := &mockTokenRefresher{}
	middleware := auth.SessionMiddleware(repo, tr, slog.Default(), false)

	var gotUserID string
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := auth.DiscordUserIDFromContext(r.Context())
		require.True(t, ok)
		gotUserID = id
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sess.ID.String()})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "user42", gotUserID)
}

func TestSessionMiddleware_NoCookie(t *testing.T) {
	middleware := auth.SessionMiddleware(newMockSessionRepo(), &mockTokenRefresher{}, slog.Default(), false)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionMiddleware_InvalidSessionID(t *testing.T) {
	middleware := auth.SessionMiddleware(newMockSessionRepo(), &mockTokenRefresher{}, slog.Default(), false)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: "not-a-uuid"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionMiddleware_ExpiredSession(t *testing.T) {
	repo := newMockSessionRepo()
	tokenExp := time.Now().Add(time.Hour)
	sess, _ := repo.Create(context.Background(), "user42", "at", "rt", &tokenExp)
	// Expire the session
	sess.ExpiresAt = time.Now().Add(-time.Hour)

	middleware := auth.SessionMiddleware(repo, &mockTokenRefresher{}, slog.Default(), false)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sess.ID.String()})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionMiddleware_TokenRefreshSuccess(t *testing.T) {
	repo := newMockSessionRepo()
	tokenExp := time.Now().Add(-time.Hour) // expired token
	sess, _ := repo.Create(context.Background(), "user42", "old_at", "old_rt", &tokenExp)

	newExpiry := time.Now().Add(7 * 24 * time.Hour)
	tr := &mockTokenRefresher{
		tokenSource: &mockTokenSource{
			token: &oauth2.Token{
				AccessToken:  "new_at",
				RefreshToken: "new_rt",
				Expiry:       newExpiry,
			},
		},
	}

	middleware := auth.SessionMiddleware(repo, tr, slog.Default(), false)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sess.ID.String()})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Tokens should be updated
	updated, _ := repo.GetByID(context.Background(), sess.ID)
	assert.Equal(t, "new_at", updated.AccessToken)
	assert.Equal(t, "new_rt", updated.RefreshToken)
}

func TestSessionMiddleware_TokenRefreshFails_DeletesSession(t *testing.T) {
	repo := newMockSessionRepo()
	tokenExp := time.Now().Add(-time.Hour) // expired token
	sess, _ := repo.Create(context.Background(), "user42", "old_at", "old_rt", &tokenExp)

	tr := &mockTokenRefresher{
		tokenSource: &mockTokenSource{
			err: fmt.Errorf("refresh failed"),
		},
	}

	middleware := auth.SessionMiddleware(repo, tr, slog.Default(), false)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sess.ID.String()})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// Session should be deleted
	_, err := repo.GetByID(context.Background(), sess.ID)
	assert.ErrorIs(t, err, auth.ErrSessionNotFound)
}

func TestSessionMiddleware_SlidesTTL(t *testing.T) {
	repo := newMockSessionRepo()
	tokenExp := time.Now().Add(time.Hour)
	sess, _ := repo.Create(context.Background(), "user42", "at", "rt", &tokenExp)
	// Set expires_at to something less than 30 days
	sess.ExpiresAt = time.Now().Add(15 * 24 * time.Hour)

	middleware := auth.SessionMiddleware(repo, &mockTokenRefresher{}, slog.Default(), false)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sess.ID.String()})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// TTL should have been slid to ~30 days
	updated, _ := repo.GetByID(context.Background(), sess.ID)
	assert.WithinDuration(t, time.Now().Add(30*24*time.Hour), updated.ExpiresAt, 5*time.Second)
}

func TestSessionMiddleware_NilTokenExpiresAt(t *testing.T) {
	repo := newMockSessionRepo()
	sess, _ := repo.Create(context.Background(), "user42", "at", "rt", nil)

	middleware := auth.SessionMiddleware(repo, &mockTokenRefresher{}, slog.Default(), false)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sess.ID.String()})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSessionMiddleware_SlidesCookieMaxAge(t *testing.T) {
	repo := newMockSessionRepo()
	tokenExp := time.Now().Add(time.Hour)
	sess, _ := repo.Create(context.Background(), "user42", "at", "rt", &tokenExp)

	middleware := auth.SessionMiddleware(repo, &mockTokenRefresher{}, slog.Default(), false)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sess.ID.String()})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// The response must contain a Set-Cookie header that refreshes the session cookie MaxAge
	cookies := rec.Result().Cookies()
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == auth.CookieName {
			found = c
			break
		}
	}
	require.NotNil(t, found, "session cookie must be re-issued on each authenticated request")
	assert.Equal(t, sess.ID.String(), found.Value)
	expectedMaxAge := int(auth.SessionTTL.Seconds())
	assert.Equal(t, expectedMaxAge, found.MaxAge)
	assert.True(t, found.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, found.SameSite)
}

func TestSessionMiddleware_SlidesCookieMaxAge_Secure(t *testing.T) {
	repo := newMockSessionRepo()
	tokenExp := time.Now().Add(time.Hour)
	sess, _ := repo.Create(context.Background(), "user42", "at", "rt", &tokenExp)

	middleware := auth.SessionMiddleware(repo, &mockTokenRefresher{}, slog.Default(), true)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sess.ID.String()})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cookies := rec.Result().Cookies()
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == auth.CookieName {
			found = c
			break
		}
	}
	require.NotNil(t, found)
	assert.True(t, found.Secure)
}

func TestDiscordUserIDFromContext_Missing(t *testing.T) {
	id, ok := auth.DiscordUserIDFromContext(context.Background())
	assert.False(t, ok)
	assert.Empty(t, id)
}

func TestContextWithDiscordUserID(t *testing.T) {
	ctx := auth.ContextWithDiscordUserID(context.Background(), "test-user-123")
	id, ok := auth.DiscordUserIDFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, "test-user-123", id)
}
