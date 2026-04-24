package gamemap

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

// importMapRequest is the JSON body for POST /api/maps/import.
type importMapRequest struct {
	CampaignID        string          `json:"campaign_id"`
	Name              string          `json:"name"`
	TMJ               json.RawMessage `json:"tmj"`
	BackgroundImageID *string         `json:"background_image_id,omitempty"`
}

// importMapResponse is the response for a successful import.
type importMapResponse struct {
	Map     mapResponse      `json:"map"`
	Skipped []SkippedFeature `json:"skipped"`
}

// ImportMap handles POST /api/maps/import. It validates the .tmj payload,
// strips unsupported features, persists the map, and returns a summary
// listing every class of feature that was stripped.
func (h *Handler) ImportMap(w http.ResponseWriter, r *http.Request) {
	var req importMapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	campaignID, err := uuid.Parse(req.CampaignID)
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}

	if len(req.TMJ) == 0 {
		http.Error(w, "tmj must not be empty", http.StatusBadRequest)
		return
	}

	bgImageID, err := parseOptionalUUID(req.BackgroundImageID)
	if err != nil {
		http.Error(w, "invalid background_image_id", http.StatusBadRequest)
		return
	}

	m, _, skipped, err := h.svc.ImportMap(r.Context(), ImportMapInput{
		CampaignID:        campaignID,
		Name:              req.Name,
		TiledJSON:         req.TMJ,
		BackgroundImageID: bgImageID,
	})
	if err != nil {
		handleImportError(w, err)
		return
	}

	if skipped == nil {
		skipped = []SkippedFeature{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(importMapResponse{
		Map:     newMapResponse(m),
		Skipped: skipped,
	})
}

// handleImportError maps importer errors to HTTP responses, surfacing
// hard-rejection sentinels as 400s and falling back to the shared service
// error handler for everything else.
func handleImportError(w http.ResponseWriter, err error) {
	for _, sentinel := range []error{
		ErrInfiniteMap,
		ErrNonOrthogonal,
		ErrMapTooLarge,
		ErrInvalidDimensions,
		ErrInvalidTiledJSON,
	} {
		if errors.Is(err, sentinel) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	handleServiceError(w, err)
}
