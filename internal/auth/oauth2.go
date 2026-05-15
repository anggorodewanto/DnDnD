package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

const (
	// CookieName is the name of the session cookie.
	CookieName = "dndnd_session"
	// StateCookieName is the name of the OAuth2 state cookie.
	StateCookieName = "dndnd_oauth_state"
	// SessionTTL is the default session duration.
	SessionTTL = 30 * 24 * time.Hour
)

// DiscordUser represents the user info returned by Discord's /users/@me endpoint.
type DiscordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// TokenRefresher can exchange and refresh OAuth2 tokens.
type TokenRefresher interface {
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	TokenSource(ctx context.Context, token *oauth2.Token) oauth2.TokenSource
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
}

// UserInfoFetcher fetches user info from Discord.
type UserInfoFetcher interface {
	FetchUserInfo(ctx context.Context, accessToken string) (*DiscordUser, error)
}

// SessionRepository defines the session persistence interface.
type SessionRepository interface {
	Create(ctx context.Context, discordUserID, accessToken, refreshToken string, tokenExpiresAt *time.Time) (*Session, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Session, error)
	Delete(ctx context.Context, id uuid.UUID) error
	SlideTTL(ctx context.Context, id uuid.UUID) error
	UpdateTokens(ctx context.Context, id uuid.UUID, accessToken, refreshToken string, tokenExpiresAt *time.Time) error
}

// DiscordUserInfoFetcher fetches user info from Discord's API.
type DiscordUserInfoFetcher struct {
	Client  *http.Client
	BaseURL string
}

// FetchUserInfo calls Discord's /users/@me endpoint.
func (f *DiscordUserInfoFetcher) FetchUserInfo(ctx context.Context, accessToken string) (*DiscordUser, error) {
	baseURL := f.BaseURL
	if baseURL == "" {
		baseURL = "https://discord.com"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/users/@me", nil)
	if err != nil {
		return nil, fmt.Errorf("creating user info request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var user DiscordUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decoding user info: %w", err)
	}
	return &user, nil
}

// OAuthService handles Discord OAuth2 authentication.
type OAuthService struct {
	oauth         TokenRefresher
	sessions      SessionRepository
	userFetcher   UserInfoFetcher
	logger        *slog.Logger
	secure        bool // whether cookies should be Secure
	generateState func() (string, error)
}

// NewOAuthService creates a new OAuthService.
func NewOAuthService(oauth TokenRefresher, sessions SessionRepository, userFetcher UserInfoFetcher, logger *slog.Logger, secure bool) *OAuthService {
	return &OAuthService{
		oauth:         oauth,
		sessions:      sessions,
		userFetcher:   userFetcher,
		logger:        logger,
		secure:        secure,
		generateState: defaultGenerateState,
	}
}

// SetGenerateState overrides the state generation function (for testing).
func (s *OAuthService) SetGenerateState(fn func() (string, error)) {
	s.generateState = fn
}

// HandleLogin redirects the user to Discord's OAuth2 authorization page.
func (s *OAuthService) HandleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := s.generateState()
	if err != nil {
		s.logger.Error("failed to generate state", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, s.cookie(StateCookieName, state, 300))
	http.Redirect(w, r, s.oauth.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

// HandleCallback processes the OAuth2 callback from Discord.
func (s *OAuthService) HandleCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie(StateCookieName)
	if err != nil {
		http.Error(w, "missing state cookie", http.StatusBadRequest)
		return
	}

	queryState := r.URL.Query().Get("state")
	if queryState == "" || queryState != stateCookie.Value {
		http.Error(w, "invalid state parameter", http.StatusBadRequest)
		return
	}

	http.SetCookie(w, s.cookie(StateCookieName, "", -1))

	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		s.logger.Warn("OAuth2 error from Discord", "error", errMsg)
		http.Error(w, "authorization denied", http.StatusForbidden)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	token, err := s.oauth.Exchange(r.Context(), code)
	if err != nil {
		s.logger.Error("token exchange failed", "error", err)
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	user, err := s.userFetcher.FetchUserInfo(r.Context(), token.AccessToken)
	if err != nil {
		s.logger.Error("failed to fetch user info", "error", err)
		http.Error(w, "failed to fetch user info", http.StatusInternalServerError)
		return
	}

	if user.ID == "" {
		http.Error(w, "invalid user", http.StatusForbidden)
		return
	}

	sess, err := s.sessions.Create(r.Context(), user.ID, token.AccessToken, token.RefreshToken, &token.Expiry)
	if err != nil {
		s.logger.Error("failed to create session", "error", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, s.cookie(CookieName, sess.ID.String(), int(SessionTTL.Seconds())))
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// HandleLogout deletes the session and clears the cookie.
func (s *OAuthService) HandleLogout(w http.ResponseWriter, r *http.Request) {
	s.deleteSessionFromCookie(r)
	http.SetCookie(w, s.cookie(CookieName, "", -1))
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// cookie builds an http.Cookie with the service's shared defaults.
func (s *OAuthService) cookie(name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
}

// deleteSessionFromCookie attempts to parse and delete the session referenced
// by the request's session cookie. Errors are logged but not propagated since
// logout must always succeed.
func (s *OAuthService) deleteSessionFromCookie(r *http.Request) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return
	}
	sessionID, err := uuid.Parse(cookie.Value)
	if err != nil {
		return
	}
	if err := s.sessions.Delete(r.Context(), sessionID); err != nil {
		s.logger.Error("failed to delete session", "error", err)
	}
}

func defaultGenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
