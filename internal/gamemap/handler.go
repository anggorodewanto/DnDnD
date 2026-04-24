package gamemap

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// Handler serves map CRUD endpoints over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler creates a new map Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts map API routes on the given Chi router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/maps", func(r chi.Router) {
		r.Post("/", h.CreateMap)
		r.Post("/import", h.ImportMap)
		r.Get("/", h.ListMaps)
		r.Get("/{id}", h.GetMap)
		r.Put("/{id}", h.UpdateMap)
		r.Delete("/{id}", h.DeleteMap)
	})
}

// mapResponse is the JSON response for a single map.
type mapResponse struct {
	ID                string          `json:"id"`
	Campaign          string          `json:"campaign_id"`
	Name              string          `json:"name"`
	Width             int             `json:"width"`
	Height            int             `json:"height"`
	TiledJSON         json.RawMessage `json:"tiled_json"`
	BackgroundImageID *string         `json:"background_image_id"`
}

// newMapResponse converts a refdata.Map to a mapResponse.
func newMapResponse(m refdata.Map) mapResponse {
	resp := mapResponse{
		ID:        m.ID.String(),
		Campaign:  m.CampaignID.String(),
		Name:      m.Name,
		Width:     int(m.WidthSquares),
		Height:    int(m.HeightSquares),
		TiledJSON: m.TiledJson,
	}
	if m.BackgroundImageID.Valid {
		s := m.BackgroundImageID.UUID.String()
		resp.BackgroundImageID = &s
	}
	return resp
}

// createMapRequest is the JSON request body for creating a map.
type createMapRequest struct {
	CampaignID        string          `json:"campaign_id"`
	Name              string          `json:"name"`
	Width             int             `json:"width"`
	Height            int             `json:"height"`
	TiledJSON         json.RawMessage `json:"tiled_json,omitempty"`
	BackgroundImageID *string         `json:"background_image_id,omitempty"`
}

// updateMapRequest is the JSON request body for updating a map.
type updateMapRequest struct {
	Name              string          `json:"name"`
	Width             int             `json:"width"`
	Height            int             `json:"height"`
	TiledJSON         json.RawMessage `json:"tiled_json"`
	BackgroundImageID *string         `json:"background_image_id,omitempty"`
}

// generateDefaultTiledJSON creates a default Tiled-compatible JSON for a blank map.
func generateDefaultTiledJSON(width, height, tileSize int) json.RawMessage {
	// Create terrain data array filled with 1 (open ground)
	data := make([]int, width*height)
	for i := range data {
		data[i] = 1
	}

	tiledMap := map[string]interface{}{
		"width":       width,
		"height":      height,
		"tilewidth":   tileSize,
		"tileheight":  tileSize,
		"orientation": "orthogonal",
		"renderorder": "right-down",
		"layers": []map[string]interface{}{
			{
				"name":    "terrain",
				"type":    "tilelayer",
				"width":   width,
				"height":  height,
				"data":    data,
				"visible": true,
				"opacity": 1,
			},
			{
				"name":    "walls",
				"type":    "objectgroup",
				"objects": []interface{}{},
				"visible": true,
				"opacity": 1,
			},
		},
		"tilesets": []map[string]interface{}{
			{
				"firstgid":  1,
				"name":      "terrain",
				"tilecount": 6,
				"tiles": []map[string]interface{}{
					{"id": 0, "type": "open_ground"},
					{"id": 1, "type": "difficult_terrain"},
					{"id": 2, "type": "water"},
					{"id": 3, "type": "lava"},
					{"id": 4, "type": "pit"},
				},
			},
		},
	}

	b, _ := json.Marshal(tiledMap)
	return b
}

// CreateMap handles POST /api/maps.
func (h *Handler) CreateMap(w http.ResponseWriter, r *http.Request) {
	var req createMapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	campaignID, err := uuid.Parse(req.CampaignID)
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}

	tiledJSON := req.TiledJSON
	if len(tiledJSON) == 0 {
		category := classifySize(req.Width, req.Height)
		tileSize := TileSizeForCategory(category)
		tiledJSON = generateDefaultTiledJSON(req.Width, req.Height, tileSize)
	}

	bgImageID, err := parseOptionalUUID(req.BackgroundImageID)
	if err != nil {
		http.Error(w, "invalid background_image_id", http.StatusBadRequest)
		return
	}

	m, _, err := h.svc.CreateMap(r.Context(), CreateMapInput{
		CampaignID:        campaignID,
		Name:              req.Name,
		Width:             req.Width,
		Height:            req.Height,
		TiledJSON:         tiledJSON,
		BackgroundImageID: bgImageID,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newMapResponse(m))
}

// GetMap handles GET /api/maps/{id}.
func (h *Handler) GetMap(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid map id", http.StatusBadRequest)
		return
	}

	m, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if err.Error() == errNotFound.Error() {
			http.Error(w, "map not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newMapResponse(m))
}

// ListMaps handles GET /api/maps?campaign_id=X.
func (h *Handler) ListMaps(w http.ResponseWriter, r *http.Request) {
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

	maps, err := h.svc.ListByCampaignID(r.Context(), campaignID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := make([]mapResponse, len(maps))
	for i, m := range maps {
		resp[i] = newMapResponse(m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// UpdateMap handles PUT /api/maps/{id}.
func (h *Handler) UpdateMap(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid map id", http.StatusBadRequest)
		return
	}

	var req updateMapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	bgImageID, err := parseOptionalUUID(req.BackgroundImageID)
	if err != nil {
		http.Error(w, "invalid background_image_id", http.StatusBadRequest)
		return
	}

	m, _, err := h.svc.UpdateMap(r.Context(), UpdateMapInput{
		ID:                id,
		Name:              req.Name,
		Width:             req.Width,
		Height:            req.Height,
		TiledJSON:         req.TiledJSON,
		BackgroundImageID: bgImageID,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newMapResponse(m))
}

// DeleteMap handles DELETE /api/maps/{id}.
func (h *Handler) DeleteMap(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid map id", http.StatusBadRequest)
		return
	}

	if err := h.svc.DeleteMap(r.Context(), id); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// errNotFound is a sentinel error for not found.
var errNotFound = errors.New("not found")

// parseOptionalUUID parses an optional UUID string pointer into a uuid.NullUUID.
// Returns an error if the string is present but not a valid UUID.
func parseOptionalUUID(s *string) (uuid.NullUUID, error) {
	if s == nil {
		return uuid.NullUUID{}, nil
	}
	parsed, err := uuid.Parse(*s)
	if err != nil {
		return uuid.NullUUID{}, err
	}
	return uuid.NullUUID{UUID: parsed, Valid: true}, nil
}

// handleServiceError maps service errors to HTTP status codes.
func handleServiceError(w http.ResponseWriter, err error) {
	msg := err.Error()
	if containsAny(msg, "must be positive", "exceeds hard limit", "must not be empty") {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	http.Error(w, "internal error", http.StatusInternalServerError)
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
