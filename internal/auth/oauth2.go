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

	http.SetCookie(w, &http.Cookie{
		Name:     StateCookieName,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300, // 5 minutes
	})

	url := s.oauth.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// HandleCallback processes the OAuth2 callback from Discord.
func (s *OAuthService) HandleCallback(w http.ResponseWriter, r *http.Request) {
	// Validate state
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

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     StateCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	// Check for error from Discord
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

	// Exchange code for tokens
	token, err := s.oauth.Exchange(r.Context(), code)
	if err != nil {
		s.logger.Error("token exchange failed", "error", err)
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	// Fetch user info
	user, err := s.userFetcher.FetchUserInfo(r.Context(), token.AccessToken)
	if err != nil {
		s.logger.Error("failed to fetch user info", "error", err)
		http.Error(w, "failed to fetch user info", http.StatusInternalServerError)
		return
	}

	// Create session
	sess, err := s.sessions.Create(r.Context(), user.ID, token.AccessToken, token.RefreshToken, &token.Expiry)
	if err != nil {
		s.logger.Error("failed to create session", "error", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    sess.ID.String(),
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(SessionTTL.Seconds()),
	})

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// HandleLogout deletes the session and clears the cookie.
func (s *OAuthService) HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(CookieName)
	if err == nil {
		sessionID, parseErr := uuid.Parse(cookie.Value)
		if parseErr == nil {
			if delErr := s.sessions.Delete(r.Context(), sessionID); delErr != nil {
				s.logger.Error("failed to delete session", "error", delErr)
			}
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func defaultGenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
