package auth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ab/dndnd/internal/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// --- Mocks ---

type mockTokenRefresher struct {
	authCodeURL string
	token       *oauth2.Token
	exchangeErr error
	tokenSource oauth2.TokenSource
}

func (m *mockTokenRefresher) Exchange(_ context.Context, _ string, _ ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return m.token, m.exchangeErr
}

func (m *mockTokenRefresher) TokenSource(_ context.Context, _ *oauth2.Token) oauth2.TokenSource {
	return m.tokenSource
}

func (m *mockTokenRefresher) AuthCodeURL(state string, _ ...oauth2.AuthCodeOption) string {
	return m.authCodeURL + "?state=" + state
}

type mockUserInfoFetcher struct {
	user *auth.DiscordUser
	err  error
}

func (m *mockUserInfoFetcher) FetchUserInfo(_ context.Context, _ string) (*auth.DiscordUser, error) {
	return m.user, m.err
}

type mockSessionRepo struct {
	sessions map[uuid.UUID]*auth.Session
	createFn func(ctx context.Context, discordUserID, accessToken, refreshToken string, tokenExpiresAt *time.Time) (*auth.Session, error)
}

func newMockSessionRepo() *mockSessionRepo {
	return &mockSessionRepo{sessions: make(map[uuid.UUID]*auth.Session)}
}

func (m *mockSessionRepo) Create(_ context.Context, discordUserID, accessToken, refreshToken string, tokenExpiresAt *time.Time) (*auth.Session, error) {
	if m.createFn != nil {
		return m.createFn(context.Background(), discordUserID, accessToken, refreshToken, tokenExpiresAt)
	}
	id := uuid.New()
	sess := &auth.Session{
		ID:             id,
		DiscordUserID:  discordUserID,
		AccessToken:    accessToken,
		RefreshToken:   refreshToken,
		TokenExpiresAt: tokenExpiresAt,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(30 * 24 * time.Hour),
	}
	m.sessions[id] = sess
	return sess, nil
}

func (m *mockSessionRepo) GetByID(_ context.Context, id uuid.UUID) (*auth.Session, error) {
	sess, ok := m.sessions[id]
	if !ok {
		return nil, auth.ErrSessionNotFound
	}
	if sess.ExpiresAt.Before(time.Now()) {
		return nil, auth.ErrSessionNotFound
	}
	return sess, nil
}

func (m *mockSessionRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.sessions, id)
	return nil
}

func (m *mockSessionRepo) SlideTTL(_ context.Context, id uuid.UUID) error {
	if sess, ok := m.sessions[id]; ok {
		sess.ExpiresAt = time.Now().Add(30 * 24 * time.Hour)
		sess.UpdatedAt = time.Now()
	}
	return nil
}

func (m *mockSessionRepo) UpdateTokens(_ context.Context, id uuid.UUID, accessToken, refreshToken string, tokenExpiresAt *time.Time) error {
	if sess, ok := m.sessions[id]; ok {
		sess.AccessToken = accessToken
		sess.RefreshToken = refreshToken
		sess.TokenExpiresAt = tokenExpiresAt
	}
	return nil
}

// --- Tests ---

func TestHandleLogin_RedirectsToDiscord(t *testing.T) {
	tr := &mockTokenRefresher{authCodeURL: "https://discord.com/oauth2/authorize"}
	svc := auth.NewOAuthService(tr, newMockSessionRepo(), &mockUserInfoFetcher{}, slog.Default(), false)

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()

	svc.HandleLogin(rec, req)

	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	location := rec.Header().Get("Location")
	assert.Contains(t, location, "https://discord.com/oauth2/authorize")
	assert.Contains(t, location, "state=")

	// Should set state cookie
	cookies := rec.Result().Cookies()
	var stateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == auth.StateCookieName {
			stateCookie = c
		}
	}
	require.NotNil(t, stateCookie)
	assert.True(t, stateCookie.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, stateCookie.SameSite)
}

func TestHandleCallback_Success(t *testing.T) {
	tokenExpiry := time.Now().Add(7 * 24 * time.Hour)
	tr := &mockTokenRefresher{
		token: &oauth2.Token{
			AccessToken:  "discord_at",
			RefreshToken: "discord_rt",
			Expiry:       tokenExpiry,
		},
	}
	fetcher := &mockUserInfoFetcher{
		user: &auth.DiscordUser{ID: "12345", Username: "testuser"},
	}
	repo := newMockSessionRepo()
	svc := auth.NewOAuthService(tr, repo, fetcher, slog.Default(), false)

	stateValue := "test_state_value"
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=authcode&state="+stateValue, nil)
	req.AddCookie(&http.Cookie{Name: auth.StateCookieName, Value: stateValue})
	rec := httptest.NewRecorder()

	svc.HandleCallback(rec, req)

	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)

	// Should set session cookie
	cookies := rec.Result().Cookies()
	var sessCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == auth.CookieName {
			sessCookie = c
		}
	}
	require.NotNil(t, sessCookie)
	assert.True(t, sessCookie.HttpOnly)

	// Session should be in repo
	sessID, err := uuid.Parse(sessCookie.Value)
	require.NoError(t, err)
	sess, err := repo.GetByID(context.Background(), sessID)
	require.NoError(t, err)
	assert.Equal(t, "12345", sess.DiscordUserID)
}

func TestHandleCallback_MissingState(t *testing.T) {
	svc := auth.NewOAuthService(&mockTokenRefresher{}, newMockSessionRepo(), &mockUserInfoFetcher{}, slog.Default(), false)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=authcode&state=abc", nil)
	rec := httptest.NewRecorder()

	svc.HandleCallback(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCallback_StateMismatch(t *testing.T) {
	svc := auth.NewOAuthService(&mockTokenRefresher{}, newMockSessionRepo(), &mockUserInfoFetcher{}, slog.Default(), false)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=authcode&state=wrong", nil)
	req.AddCookie(&http.Cookie{Name: auth.StateCookieName, Value: "correct"})
	rec := httptest.NewRecorder()

	svc.HandleCallback(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCallback_MissingCode(t *testing.T) {
	state := "valid_state"
	svc := auth.NewOAuthService(&mockTokenRefresher{}, newMockSessionRepo(), &mockUserInfoFetcher{}, slog.Default(), false)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state, nil)
	req.AddCookie(&http.Cookie{Name: auth.StateCookieName, Value: state})
	rec := httptest.NewRecorder()

	svc.HandleCallback(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCallback_DiscordError(t *testing.T) {
	state := "valid_state"
	svc := auth.NewOAuthService(&mockTokenRefresher{}, newMockSessionRepo(), &mockUserInfoFetcher{}, slog.Default(), false)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&error=access_denied", nil)
	req.AddCookie(&http.Cookie{Name: auth.StateCookieName, Value: state})
	rec := httptest.NewRecorder()

	svc.HandleCallback(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandleLogout(t *testing.T) {
	repo := newMockSessionRepo()
	sess, _ := repo.Create(context.Background(), "user1", "at", "rt", nil)
	svc := auth.NewOAuthService(&mockTokenRefresher{}, repo, &mockUserInfoFetcher{}, slog.Default(), false)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: sess.ID.String()})
	rec := httptest.NewRecorder()

	svc.HandleLogout(rec, req)

	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)

	// Session should be deleted
	_, err := repo.GetByID(context.Background(), sess.ID)
	assert.ErrorIs(t, err, auth.ErrSessionNotFound)

	// Cookie should be cleared
	cookies := rec.Result().Cookies()
	var sessCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == auth.CookieName {
			sessCookie = c
		}
	}
	require.NotNil(t, sessCookie)
	assert.Equal(t, -1, sessCookie.MaxAge)
}

func TestHandleLogout_NoCookie(t *testing.T) {
	svc := auth.NewOAuthService(&mockTokenRefresher{}, newMockSessionRepo(), &mockUserInfoFetcher{}, slog.Default(), false)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rec := httptest.NewRecorder()

	svc.HandleLogout(rec, req)

	// Should still redirect even without cookie
	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
}

func TestDiscordUserInfoFetcher(t *testing.T) {
	expectedUser := auth.DiscordUser{ID: "999", Username: "testbot"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test_token", r.Header.Get("Authorization"))
		assert.Equal(t, "/api/users/@me", r.URL.Path)
		json.NewEncoder(w).Encode(expectedUser)
	}))
	defer server.Close()

	fetcher := &auth.DiscordUserInfoFetcher{BaseURL: server.URL}
	user, err := fetcher.FetchUserInfo(context.Background(), "test_token")
	require.NoError(t, err)
	assert.Equal(t, "999", user.ID)
	assert.Equal(t, "testbot", user.Username)
}

func TestHandleCallback_ExchangeError(t *testing.T) {
	tr := &mockTokenRefresher{
		exchangeErr: fmt.Errorf("exchange failed"),
	}
	state := "valid_state"
	svc := auth.NewOAuthService(tr, newMockSessionRepo(), &mockUserInfoFetcher{}, slog.Default(), false)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=authcode&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: auth.StateCookieName, Value: state})
	rec := httptest.NewRecorder()

	svc.HandleCallback(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleCallback_FetchUserInfoError(t *testing.T) {
	tokenExpiry := time.Now().Add(7 * 24 * time.Hour)
	tr := &mockTokenRefresher{
		token: &oauth2.Token{
			AccessToken:  "discord_at",
			RefreshToken: "discord_rt",
			Expiry:       tokenExpiry,
		},
	}
	fetcher := &mockUserInfoFetcher{
		err: fmt.Errorf("user info fetch failed"),
	}
	state := "valid_state"
	svc := auth.NewOAuthService(tr, newMockSessionRepo(), fetcher, slog.Default(), false)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=authcode&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: auth.StateCookieName, Value: state})
	rec := httptest.NewRecorder()

	svc.HandleCallback(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleCallback_CreateSessionError(t *testing.T) {
	tokenExpiry := time.Now().Add(7 * 24 * time.Hour)
	tr := &mockTokenRefresher{
		token: &oauth2.Token{
			AccessToken:  "discord_at",
			RefreshToken: "discord_rt",
			Expiry:       tokenExpiry,
		},
	}
	fetcher := &mockUserInfoFetcher{
		user: &auth.DiscordUser{ID: "12345", Username: "testuser"},
	}
	repo := newMockSessionRepo()
	repo.createFn = func(_ context.Context, _, _, _ string, _ *time.Time) (*auth.Session, error) {
		return nil, fmt.Errorf("db error")
	}
	state := "valid_state"
	svc := auth.NewOAuthService(tr, repo, fetcher, slog.Default(), false)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=authcode&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: auth.StateCookieName, Value: state})
	rec := httptest.NewRecorder()

	svc.HandleCallback(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleLogin_SecureCookie(t *testing.T) {
	tr := &mockTokenRefresher{authCodeURL: "https://discord.com/oauth2/authorize"}
	svc := auth.NewOAuthService(tr, newMockSessionRepo(), &mockUserInfoFetcher{}, slog.Default(), true)

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()

	svc.HandleLogin(rec, req)

	cookies := rec.Result().Cookies()
	var stateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == auth.StateCookieName {
			stateCookie = c
		}
	}
	require.NotNil(t, stateCookie)
	assert.True(t, stateCookie.Secure)
}

func TestHandleLogin_GenerateStateError(t *testing.T) {
	tr := &mockTokenRefresher{authCodeURL: "https://discord.com/oauth2/authorize"}
	svc := auth.NewOAuthService(tr, newMockSessionRepo(), &mockUserInfoFetcher{}, slog.Default(), false)
	svc.SetGenerateState(func() (string, error) {
		return "", fmt.Errorf("rand error")
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()

	svc.HandleLogin(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleLogout_InvalidUUID(t *testing.T) {
	svc := auth.NewOAuthService(&mockTokenRefresher{}, newMockSessionRepo(), &mockUserInfoFetcher{}, slog.Default(), false)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: "not-a-uuid"})
	rec := httptest.NewRecorder()

	svc.HandleLogout(rec, req)
	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
}

func TestDiscordUserInfoFetcher_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"401: Unauthorized"}`))
	}))
	defer server.Close()

	fetcher := &auth.DiscordUserInfoFetcher{BaseURL: server.URL}
	user, err := fetcher.FetchUserInfo(context.Background(), "bad_token")
	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "401")
}
