package dashboard

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ab/dndnd/internal/auth"
)

// Guild is a Discord server the bot is a member of, exposed to the dashboard so
// the campaign-create form can offer a dropdown instead of a free-text Guild ID
// field — which previously let a typo create an orphan campaign (T20 / Finding 12).
type Guild struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GuildLister returns the guilds the bot is currently in. Implemented in
// cmd/dndnd by reading the live discordgo session state.
type GuildLister interface {
	ListGuilds(ctx context.Context) ([]Guild, error)
}

type guildsResponse struct {
	Guilds []Guild `json:"guilds"`
}

// GuildsHandler serves GET /api/guilds.
type GuildsHandler struct {
	logger *slog.Logger
	lister GuildLister
}

// NewGuildsHandler constructs a GuildsHandler. lister may be nil (e.g. a deploy
// with no live Discord session) — the handler then returns an empty list with
// 200 so the form falls back to its free-text Guild ID input.
func NewGuildsHandler(logger *slog.Logger, lister GuildLister) *GuildsHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &GuildsHandler{logger: logger, lister: lister}
}

// List returns the guilds the bot is in as JSON.
func (h *GuildsHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	resp := guildsResponse{Guilds: []Guild{}}
	if h.lister != nil {
		guilds, err := h.lister.ListGuilds(r.Context())
		if err != nil {
			h.logger.Warn("/api/guilds list failed", "error", err)
		} else {
			resp.Guilds = guilds
		}
	}

	writeCampaignJSON(w, http.StatusOK, resp)
}

// RegisterGuildsRoute mounts GET /api/guilds behind authMiddleware.
func RegisterGuildsRoute(r chi.Router, h *GuildsHandler, authMiddleware func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/api/guilds", h.List)
	})
}
