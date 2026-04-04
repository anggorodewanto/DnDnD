package portal_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/portal"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

// fakeAuthMiddleware injects a discord user ID for testing.
func fakeAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := auth.ContextWithDiscordUserID(r.Context(), "test-user")
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func TestRegisterRoutes_Landing(t *testing.T) {
	r := chi.NewRouter()
	validator := &mockTokenValidator{
		token: &portal.PortalToken{
			ID:            uuid.New(),
			Token:         "tok",
			CampaignID:    uuid.New(),
			DiscordUserID: "test-user",
			Purpose:       "create_character",
			ExpiresAt:     time.Now().Add(24 * time.Hour),
		},
	}
	h := portal.NewHandler(slog.Default(), validator)
	portal.RegisterRoutes(r, h, fakeAuthMiddleware)

	req := httptest.NewRequest(http.MethodGet, "/portal/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "DnDnD")
}

func TestRegisterRoutes_Create(t *testing.T) {
	r := chi.NewRouter()
	validator := &mockTokenValidator{
		token: &portal.PortalToken{
			ID:            uuid.New(),
			Token:         "tok",
			CampaignID:    uuid.New(),
			DiscordUserID: "test-user",
			Purpose:       "create_character",
			ExpiresAt:     time.Now().Add(24 * time.Hour),
		},
	}
	h := portal.NewHandler(slog.Default(), validator)
	portal.RegisterRoutes(r, h, fakeAuthMiddleware)

	req := httptest.NewRequest(http.MethodGet, "/portal/create?token=tok", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Character Builder")
}

func TestRegisterRoutes_CreateWithoutAuth(t *testing.T) {
	r := chi.NewRouter()
	noAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
	h := portal.NewHandler(slog.Default(), nil)
	portal.RegisterRoutes(r, h, noAuth)

	req := httptest.NewRequest(http.MethodGet, "/portal/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// Test that portal OAuth routes are mounted
func TestRegisterRoutes_OAuthLogin(t *testing.T) {
	r := chi.NewRouter()

	mockRefresher := &portalMockRefresher{authCodeURL: "https://discord.com/oauth2/authorize"}
	oauthSvc := auth.NewOAuthService(mockRefresher, &portalMockSessionRepo{}, &portalMockUserFetcher{}, slog.Default(), false)
	h := portal.NewHandler(slog.Default(), nil)
	portal.RegisterRoutes(r, h, fakeAuthMiddleware, portal.WithOAuth(oauthSvc))

	req := httptest.NewRequest(http.MethodGet, "/portal/auth/login", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
}

// Mock implementations for auth interfaces
type portalMockRefresher struct {
	authCodeURL string
}

func (m *portalMockRefresher) Exchange(_ context.Context, _ string, _ ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return &oauth2.Token{}, nil
}

func (m *portalMockRefresher) TokenSource(_ context.Context, _ *oauth2.Token) oauth2.TokenSource {
	return nil
}

func (m *portalMockRefresher) AuthCodeURL(state string, _ ...oauth2.AuthCodeOption) string {
	return m.authCodeURL + "?state=" + state
}

type portalMockSessionRepo struct{}

func (m *portalMockSessionRepo) Create(_ context.Context, _, _, _ string, _ *time.Time) (*auth.Session, error) {
	return &auth.Session{ID: uuid.New()}, nil
}
func (m *portalMockSessionRepo) GetByID(_ context.Context, _ uuid.UUID) (*auth.Session, error) {
	return nil, auth.ErrSessionNotFound
}
func (m *portalMockSessionRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }
func (m *portalMockSessionRepo) SlideTTL(_ context.Context, _ uuid.UUID) error { return nil }
func (m *portalMockSessionRepo) UpdateTokens(_ context.Context, _ uuid.UUID, _, _ string, _ *time.Time) error {
	return nil
}

type portalMockUserFetcher struct{}

func (m *portalMockUserFetcher) FetchUserInfo(_ context.Context, _ string) (*auth.DiscordUser, error) {
	return &auth.DiscordUser{ID: "user-123", Username: "testuser"}, nil
}
