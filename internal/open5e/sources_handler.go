package open5e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// CampaignSettingsStore is the read+write surface SourcesHandler needs to
// load and mutate the campaigns.settings JSONB column. refdata.Queries
// satisfies it directly.
type CampaignSettingsStore interface {
	GetCampaignByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)
	UpdateCampaignSettings(ctx context.Context, arg refdata.UpdateCampaignSettingsParams) (refdata.Campaign, error)
}

// SourcesHandler exposes the per-campaign Open5e source toggle API used by
// the Svelte DM dashboard (F-8 / Phase 111).
type SourcesHandler struct {
	store CampaignSettingsStore
}

// NewSourcesHandler constructs a SourcesHandler backed by the given store.
func NewSourcesHandler(store CampaignSettingsStore) *SourcesHandler {
	return &SourcesHandler{store: store}
}

// RegisterRoutes mounts the source-catalog read and the per-campaign
// read/write endpoints under /api/open5e. The catalog GET is always safe
// (read-only, no campaign data); the per-campaign endpoints expect to sit
// behind the DM auth middleware applied at the router level in main.go.
func (h *SourcesHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/open5e/sources", h.ListCatalog)
	r.Get("/api/open5e/campaigns/{id}/sources", h.GetCampaignSources)
	r.Put("/api/open5e/campaigns/{id}/sources", h.UpdateCampaignSources)
}

// ListCatalog returns the curated list of Open5e document slugs available
// to the dashboard. The frontend renders one checkbox per entry; the
// backend stays the single source of truth for the catalog.
func (h *SourcesHandler) ListCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"sources": Catalog()})
}

// campaignSourcesResponse is the wire shape for GET/PUT of per-campaign
// enabled slugs.
type campaignSourcesResponse struct {
	CampaignID string   `json:"campaign_id"`
	Enabled    []string `json:"enabled"`
}

// GetCampaignSources returns the campaign's currently enabled Open5e
// document slugs. Always returns a non-nil "enabled" array (possibly
// empty) so the frontend can render checkboxes deterministically.
func (h *SourcesHandler) GetCampaignSources(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseCampaignID(w, r)
	if !ok {
		return
	}
	enabled, err := h.readEnabled(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, campaignSourcesResponse{
		CampaignID: id.String(),
		Enabled:    enabled,
	})
}

// updateRequest is the wire shape for PUT.
type updateRequest struct {
	Enabled []string `json:"enabled"`
}

// UpdateCampaignSources replaces the campaign's open5e_sources list with
// the provided slugs. Every slug must appear in the curated Catalog;
// unknown slugs cause a 400 with the offending value in the error body.
// The update is a JSONB partial: other settings fields (turn_timeout_hours,
// channel_ids, etc.) are preserved verbatim.
func (h *SourcesHandler) UpdateCampaignSources(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseCampaignID(w, r)
	if !ok {
		return
	}

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	cleaned, err := validateSlugs(req.Enabled)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c, err := h.store.GetCampaignByID(r.Context(), id)
	if err != nil {
		http.Error(w, "campaign not found", http.StatusNotFound)
		return
	}

	merged, err := mergeOpen5eSources(c.Settings, cleaned)
	if err != nil {
		http.Error(w, fmt.Sprintf("merging settings: %v", err), http.StatusInternalServerError)
		return
	}

	if _, err := h.store.UpdateCampaignSettings(r.Context(), refdata.UpdateCampaignSettingsParams{
		ID:       id,
		Settings: merged,
	}); err != nil {
		http.Error(w, "failed to update campaign settings", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, campaignSourcesResponse{
		CampaignID: id.String(),
		Enabled:    cleaned,
	})
}

// --- internals ---

func (h *SourcesHandler) parseCampaignID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := chi.URLParam(r, "id")
	id, err := uuid.Parse(raw)
	if err != nil {
		http.Error(w, "invalid campaign id", http.StatusBadRequest)
		return uuid.Nil, false
	}
	return id, true
}

// readEnabled loads the campaign's open5e_sources list as a non-nil slice
// (callers can iterate without nil checks). Returns an error only when the
// campaign itself is unreachable.
func (h *SourcesHandler) readEnabled(ctx context.Context, id uuid.UUID) ([]string, error) {
	c, err := h.store.GetCampaignByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !c.Settings.Valid || len(c.Settings.RawMessage) == 0 {
		return []string{}, nil
	}
	var s settingsShape
	if err := json.Unmarshal(c.Settings.RawMessage, &s); err != nil {
		return []string{}, nil
	}
	if s.Open5eSources == nil {
		return []string{}, nil
	}
	return s.Open5eSources, nil
}

// validateSlugs trims duplicates / blanks and rejects any slug not in the
// curated catalog. Returns a fresh slice in input order.
func validateSlugs(in []string) ([]string, error) {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		if raw == "" {
			continue
		}
		if _, dup := seen[raw]; dup {
			continue
		}
		if !IsKnownSource(raw) {
			return nil, fmt.Errorf("unknown open5e source: %q", raw)
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
	}
	return out, nil
}

// mergeOpen5eSources rewrites the open5e_sources field of an existing
// JSONB settings blob with newSlugs, leaving every other field unchanged.
// An invalid/empty starting payload becomes a fresh object holding just
// open5e_sources.
func mergeOpen5eSources(existing pqtype.NullRawMessage, newSlugs []string) (pqtype.NullRawMessage, error) {
	merged := map[string]any{}
	if existing.Valid && len(existing.RawMessage) > 0 {
		if err := json.Unmarshal(existing.RawMessage, &merged); err != nil {
			// Preserving a malformed settings blob would silently overwrite
			// other DM-configured fields; signal upward instead.
			return pqtype.NullRawMessage{}, errors.New("existing campaign settings are not a JSON object")
		}
	}
	if len(newSlugs) == 0 {
		// Empty list disables every Open5e doc — keep the key explicit so
		// downstream code (campaign_lookup.go) sees "no sources enabled"
		// rather than "field missing" (both behave identically today, but
		// the explicit form is the canonical wire shape).
		merged["open5e_sources"] = []string{}
	} else {
		merged["open5e_sources"] = newSlugs
	}
	out, err := json.Marshal(merged)
	if err != nil {
		return pqtype.NullRawMessage{}, err
	}
	return pqtype.NullRawMessage{RawMessage: out, Valid: true}, nil
}
