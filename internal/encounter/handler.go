package encounter

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// Handler serves encounter template CRUD endpoints over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler creates a new encounter Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts encounter API routes on the given Chi router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/encounters", func(r chi.Router) {
		r.Post("/", h.CreateEncounter)
		r.Get("/", h.ListEncounters)
		r.Get("/{id}", h.GetEncounter)
		r.Put("/{id}", h.UpdateEncounter)
		r.Delete("/{id}", h.DeleteEncounter)
		r.Post("/{id}/duplicate", h.DuplicateEncounter)
	})
	r.Get("/api/creatures", h.ListCreatures)
}

// encounterResponse is the JSON response for a single encounter template.
type encounterResponse struct {
	ID            string          `json:"id"`
	CampaignID    string          `json:"campaign_id"`
	MapID         *string         `json:"map_id"`
	Name          string          `json:"name"`
	DisplayName   *string         `json:"display_name"`
	Creatures     json.RawMessage `json:"creatures"`
	CreatureCount int             `json:"creature_count"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
}

// creatureEntry is used to count creatures in the JSON array.
type creatureEntry struct {
	CreatureRefID string `json:"creature_ref_id"`
}

// newEncounterResponse converts a refdata.EncounterTemplate to an encounterResponse.
func newEncounterResponse(et refdata.EncounterTemplate) encounterResponse {
	resp := encounterResponse{
		ID:         et.ID.String(),
		CampaignID: et.CampaignID.String(),
		Name:       et.Name,
		Creatures:  et.Creatures,
		CreatedAt:  et.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:  et.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if et.MapID.Valid {
		s := et.MapID.UUID.String()
		resp.MapID = &s
	}
	if et.DisplayName.Valid {
		resp.DisplayName = &et.DisplayName.String
	}

	// Count creatures
	var creatures []creatureEntry
	if json.Unmarshal(et.Creatures, &creatures) == nil {
		resp.CreatureCount = len(creatures)
	}

	return resp
}

// createEncounterRequest is the JSON request body for creating an encounter.
type createEncounterRequest struct {
	CampaignID  string          `json:"campaign_id"`
	MapID       *string         `json:"map_id,omitempty"`
	Name        string          `json:"name"`
	DisplayName string          `json:"display_name,omitempty"`
	Creatures   json.RawMessage `json:"creatures,omitempty"`
}

// updateEncounterRequest is the JSON request body for updating an encounter.
type updateEncounterRequest struct {
	MapID       *string         `json:"map_id,omitempty"`
	Name        string          `json:"name"`
	DisplayName string          `json:"display_name,omitempty"`
	Creatures   json.RawMessage `json:"creatures,omitempty"`
}

// parseOptionalUUID parses an optional UUID string pointer into a uuid.NullUUID.
func parseOptionalUUID(s *string) (uuid.NullUUID, error) {
	if s == nil || *s == "" {
		return uuid.NullUUID{}, nil
	}
	parsed, err := uuid.Parse(*s)
	if err != nil {
		return uuid.NullUUID{}, err
	}
	return uuid.NullUUID{UUID: parsed, Valid: true}, nil
}

// CreateEncounter handles POST /api/encounters.
func (h *Handler) CreateEncounter(w http.ResponseWriter, r *http.Request) {
	var req createEncounterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	campaignID, err := uuid.Parse(req.CampaignID)
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}

	mapID, err := parseOptionalUUID(req.MapID)
	if err != nil {
		http.Error(w, "invalid map_id", http.StatusBadRequest)
		return
	}

	et, err := h.svc.Create(r.Context(), CreateInput{
		CampaignID:  campaignID,
		MapID:       mapID,
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Creatures:   req.Creatures,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newEncounterResponse(et))
}

// GetEncounter handles GET /api/encounters/{id}.
func (h *Handler) GetEncounter(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid encounter id", http.StatusBadRequest)
		return
	}

	et, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "encounter not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newEncounterResponse(et))
}

// ListEncounters handles GET /api/encounters?campaign_id=X.
func (h *Handler) ListEncounters(w http.ResponseWriter, r *http.Request) {
	campaignIDStr := r.URL.Query().Get("campaign_id")
	if campaignIDStr == "" {
		http.Error(w, "campaign_id query parameter required", http.StatusBadRequest)
		return
	}

	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}

	encounters, err := h.svc.ListByCampaignID(r.Context(), campaignID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := make([]encounterResponse, len(encounters))
	for i, et := range encounters {
		resp[i] = newEncounterResponse(et)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// UpdateEncounter handles PUT /api/encounters/{id}.
func (h *Handler) UpdateEncounter(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid encounter id", http.StatusBadRequest)
		return
	}

	var req updateEncounterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	mapID, err := parseOptionalUUID(req.MapID)
	if err != nil {
		http.Error(w, "invalid map_id", http.StatusBadRequest)
		return
	}

	et, err := h.svc.Update(r.Context(), UpdateInput{
		ID:          id,
		MapID:       mapID,
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Creatures:   req.Creatures,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newEncounterResponse(et))
}

// DeleteEncounter handles DELETE /api/encounters/{id}.
func (h *Handler) DeleteEncounter(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid encounter id", http.StatusBadRequest)
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DuplicateEncounter handles POST /api/encounters/{id}/duplicate.
func (h *Handler) DuplicateEncounter(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid encounter id", http.StatusBadRequest)
		return
	}

	et, err := h.svc.Duplicate(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newEncounterResponse(et))
}

// creatureListItem is a simplified creature response for the encounter builder.
type creatureListItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	CR   string `json:"cr"`
	Size string `json:"size"`
	Type string `json:"type"`
	AC   int    `json:"ac"`
	HP   int    `json:"hp_average"`
}

// ListCreatures handles GET /api/creatures.
func (h *Handler) ListCreatures(w http.ResponseWriter, r *http.Request) {
	creatures, err := h.svc.ListCreatures(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := make([]creatureListItem, len(creatures))
	for i, c := range creatures {
		resp[i] = creatureListItem{
			ID:   c.ID,
			Name: c.Name,
			CR:   c.Cr,
			Size: c.Size,
			Type: c.Type,
			AC:   int(c.Ac),
			HP:   int(c.HpAverage),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleServiceError maps service errors to HTTP status codes.
func handleServiceError(w http.ResponseWriter, err error) {
	msg := err.Error()
	if strings.Contains(msg, "must not be empty") {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	http.Error(w, "internal error", http.StatusInternalServerError)
}
