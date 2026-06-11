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
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/campaign"
	"github.com/ab/dndnd/internal/refdata"
)

type CampaignStore interface {
	CreateCampaign(ctx context.Context, arg refdata.CreateCampaignParams) (refdata.Campaign, error)
	ListCampaigns(ctx context.Context) ([]refdata.Campaign, error)
	SetActiveCampaign(ctx context.Context, arg refdata.SetActiveCampaignParams) error
}

type CampaignsHandler struct {
	logger *slog.Logger
	store  CampaignStore
	// Passthrough is true when dashboard OAuth is disabled and every request is
	// authenticated as the local-dev passthrough user. Campaigns created in
	// this mode are owned by DEV_DISCORD_USER_ID (default "local-dev"), so once
	// real OAuth is enabled the DM fails the IsDM ownership check and the
	// campaign 403s. Create logs a warning so the trap is visible in the logs.
	Passthrough bool
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

	if h.Passthrough {
		h.logger.Warn("campaign created in passthrough auth mode; it is owned by the local-dev passthrough user and will be inaccessible (403) once real OAuth is enabled — recreate it after configuring DISCORD_CLIENT_ID/SECRET",
			"campaign_id", created.ID, "dm_user_id", userID)
	}

	writeCampaignJSON(w, http.StatusCreated, campaignToDTO(created))
}

type setActiveCampaignResponse struct {
	CampaignID string `json:"campaign_id"`
	Status     string `json:"status"`
}

// SetActive records the DM's explicit active-campaign choice (T20 / Finding 12)
// so the dashboard stops silently following the most-recently-created campaign.
// The target must be owned by the authenticated DM and not archived.
func (h *CampaignsHandler) SetActive(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.store == nil {
		http.Error(w, "campaign store unavailable", http.StatusInternalServerError)
		return
	}

	campaignID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid campaign id", http.StatusBadRequest)
		return
	}

	campaigns, err := h.store.ListCampaigns(r.Context())
	if err != nil {
		h.logger.Error("list campaigns failed", "error", err)
		http.Error(w, "failed to set active campaign", http.StatusInternalServerError)
		return
	}

	var target *refdata.Campaign
	for i := range campaigns {
		if campaigns[i].ID == campaignID && campaigns[i].DmUserID == userID {
			target = &campaigns[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "campaign not found", http.StatusNotFound)
		return
	}
	if target.Status == "archived" {
		http.Error(w, "cannot activate an archived campaign", http.StatusConflict)
		return
	}

	if err := h.store.SetActiveCampaign(r.Context(), refdata.SetActiveCampaignParams{
		DmUserID:         userID,
		ActiveCampaignID: campaignID,
	}); err != nil {
		h.logger.Error("set active campaign failed", "error", err)
		http.Error(w, "failed to set active campaign", http.StatusInternalServerError)
		return
	}

	writeCampaignJSON(w, http.StatusOK, setActiveCampaignResponse{
		CampaignID: campaignID.String(),
		Status:     target.Status,
	})
}

// ResolveActiveCampaign picks the DM's active campaign from the full campaign
// list (as returned by ListCampaigns, ordered created_at DESC). It honors the
// DM's explicit selection (preferred) when that campaign is still owned by the
// DM and not archived; otherwise it falls back to the most-recently-created
// non-archived campaign the DM owns (the historical heuristic). Returns
// ("", "") when the DM owns no eligible campaign. (T20 / Finding 12)
func ResolveActiveCampaign(campaigns []refdata.Campaign, dmUserID string, preferred uuid.UUID) (id, status string) {
	if preferred != uuid.Nil {
		for _, c := range campaigns {
			if c.ID == preferred && c.DmUserID == dmUserID && c.Status != "archived" {
				return c.ID.String(), c.Status
			}
		}
	}
	for _, c := range campaigns {
		if c.DmUserID != dmUserID || c.Status == "archived" {
			continue
		}
		return c.ID.String(), c.Status
	}
	return "", ""
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
		r.Post("/api/campaigns/{id}/set-active", h.SetActive)
	})
}
