package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/refdata"
)

type CampaignStore interface {
	CreateCampaign(ctx context.Context, arg refdata.CreateCampaignParams) (refdata.Campaign, error)
	ListCampaigns(ctx context.Context) ([]refdata.Campaign, error)
}

type CampaignsHandler struct {
	logger *slog.Logger
	store  CampaignStore
}

type campaignDTO struct {
	ID        string    `json:"id"`
	GuildID   string    `json:"guild_id"`
	DMUserID  string    `json:"dm_user_id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type campaignsResponse struct {
	Campaigns []campaignDTO `json:"campaigns"`
}

type createCampaignRequest struct {
	GuildID string `json:"guild_id"`
	Name    string `json:"name"`
}

func NewCampaignsHandler(logger *slog.Logger, store CampaignStore) *CampaignsHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &CampaignsHandler{logger: logger, store: store}
}

func (h *CampaignsHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.store == nil {
		http.Error(w, "campaign store unavailable", http.StatusInternalServerError)
		return
	}

	campaigns, err := h.store.ListCampaigns(r.Context())
	if err != nil {
		h.logger.Error("list campaigns failed", "error", err)
		http.Error(w, "failed to list campaigns", http.StatusInternalServerError)
		return
	}

	resp := campaignsResponse{Campaigns: []campaignDTO{}}
	for _, c := range campaigns {
		if c.DmUserID != userID {
			continue
		}
		resp.Campaigns = append(resp.Campaigns, campaignToDTO(c))
	}
	writeCampaignJSON(w, http.StatusOK, resp)
}

func (h *CampaignsHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.store == nil {
		http.Error(w, "campaign store unavailable", http.StatusInternalServerError)
		return
	}

	var req createCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	guildID := strings.TrimSpace(req.GuildID)
	name := strings.TrimSpace(req.Name)
	if guildID == "" {
		http.Error(w, "guild_id is required", http.StatusBadRequest)
		return
	}
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	settings := campaign.DefaultSettings()
	rawSettings, err := json.Marshal(settings)
	if err != nil {
		h.logger.Error("marshal default campaign settings failed", "error", err)
		http.Error(w, "failed to create campaign", http.StatusInternalServerError)
		return
	}

	created, err := h.store.CreateCampaign(r.Context(), refdata.CreateCampaignParams{
		GuildID:  guildID,
		DmUserID: userID,
		Name:     name,
		Settings: pqtype.NullRawMessage{RawMessage: rawSettings, Valid: true},
	})
	if err != nil {
		h.logger.Error("create campaign failed", "error", err)
		http.Error(w, fmt.Sprintf("failed to create campaign: %s", err), http.StatusConflict)
		return
	}

	writeCampaignJSON(w, http.StatusCreated, campaignToDTO(created))
}

func campaignToDTO(c refdata.Campaign) campaignDTO {
	return campaignDTO{
		ID:        c.ID.String(),
		GuildID:   c.GuildID,
		DMUserID:  c.DmUserID,
		Name:      c.Name,
		Status:    c.Status,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func writeCampaignJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func RegisterCampaignsRoutes(r chi.Router, h *CampaignsHandler, authMiddleware func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/api/campaigns", h.List)
		r.Post("/api/campaigns", h.Create)
	})
}
