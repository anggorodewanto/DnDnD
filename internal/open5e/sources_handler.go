package open5e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// slugPattern is the canonical Open5e document__slug shape: lowercase letters
// and digits in hyphen-separated groups (e.g. "tome-of-beasts"). It guards the
// admin add-source path so a fat-fingered slug can never reach the catalog.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// CampaignSettingsStore is the read+write surface SourcesHandler needs to
// load and mutate the campaigns.settings JSONB column. refdata.Queries
// satisfies it directly.
type CampaignSettingsStore interface {
	GetCampaignByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)
	UpdateCampaignSettings(ctx context.Context, arg refdata.UpdateCampaignSettingsParams) (refdata.Campaign, error)
}

// CustomSourceStore is the read+write surface for the admin-managed
// open5e_custom_sources table (extends the built-in catalog at runtime).
// refdata.Queries satisfies it directly.
type CustomSourceStore interface {
	ListOpen5eCustomSources(ctx context.Context) ([]refdata.Open5eCustomSource, error)
	UpsertOpen5eCustomSource(ctx context.Context, arg refdata.UpsertOpen5eCustomSourceParams) (refdata.Open5eCustomSource, error)
	DeleteOpen5eCustomSource(ctx context.Context, slug string) (int64, error)
}

// SourceStore is the combined store SourcesHandler depends on. refdata.Queries
// satisfies it (campaign settings + custom sources).
type SourceStore interface {
	CampaignSettingsStore
	CustomSourceStore
}

// SourcesHandler exposes the Open5e source catalog API used by the Svelte DM
// dashboard (F-8 / Phase 111): the global catalog (built-in + admin-added
// custom sources) and the per-campaign enable toggle.
type SourcesHandler struct {
	store SourceStore
}

// NewSourcesHandler constructs a SourcesHandler backed by the given store.
func NewSourcesHandler(store SourceStore) *SourcesHandler {
	return &SourcesHandler{store: store}
}

// RegisterRoutes mounts the catalog read/manage endpoints and the per-campaign
// read/write endpoints under /api/open5e. The catalog GET is always safe
// (read-only); the manage (POST/DELETE) and per-campaign endpoints expect to
// sit behind the DM auth middleware applied at the router level in main.go.
func (h *SourcesHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/open5e/sources", h.ListCatalog)
	r.Post("/api/open5e/sources", h.AddCustomSource)
	r.Delete("/api/open5e/sources/{slug}", h.DeleteCustomSource)
	r.Get("/api/open5e/campaigns/{id}/sources", h.GetCampaignSources)
	r.Put("/api/open5e/campaigns/{id}/sources", h.UpdateCampaignSources)
}

// ListCatalog returns the full Open5e catalog the dashboard renders: the
// built-in baseline followed by any admin-added custom sources. The frontend
// renders one checkbox per entry; the backend stays the single source of
// truth for the slug→title mapping.
func (h *SourcesHandler) ListCatalog(w http.ResponseWriter, r *http.Request) {
	all, err := h.fullCatalog(r.Context())
	if err != nil {
		http.Error(w, "failed to load source catalog", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": all})
}

// addSourceRequest is the wire shape for POST /api/open5e/sources.
type addSourceRequest struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Publisher   string `json:"publisher"`
	Description string `json:"description"`
}

// AddCustomSource registers (or updates) an admin-supplied Open5e document so
// it joins the catalog for every campaign. The slug must look like a real
// Open5e document slug and must not collide with a built-in entry; the title
// is required. Re-posting an existing custom slug edits it in place.
func (h *SourcesHandler) AddCustomSource(w http.ResponseWriter, r *http.Request) {
	var req addSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	slug := strings.TrimSpace(req.Slug)
	title := strings.TrimSpace(req.Title)
	if !slugPattern.MatchString(slug) {
		http.Error(w, "slug must be lowercase letters, digits, and single hyphens (e.g. tome-of-beasts)", http.StatusBadRequest)
		return
	}
	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if IsKnownSource(slug) {
		http.Error(w, fmt.Sprintf("%q is a built-in source and cannot be overridden", slug), http.StatusConflict)
		return
	}

	rec, err := h.store.UpsertOpen5eCustomSource(r.Context(), refdata.UpsertOpen5eCustomSourceParams{
		Slug:        slug,
		Title:       title,
		Publisher:   strings.TrimSpace(req.Publisher),
		Description: strings.TrimSpace(req.Description),
	})
	if err != nil {
		http.Error(w, "failed to save custom source", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, Source{
		Slug:        rec.Slug,
		Title:       rec.Title,
		Publisher:   rec.Publisher,
		Description: rec.Description,
		Builtin:     false,
	})
}

// DeleteCustomSource removes an admin-added custom source from the catalog.
// Built-in sources are protected (409). Campaigns that still list the slug in
// their open5e_sources keep the stale entry harmlessly — it matches no cached
// content until/unless the slug is re-added, and the per-campaign validator
// rejects re-saving it.
func (h *SourcesHandler) DeleteCustomSource(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(chi.URLParam(r, "slug"))
	if slug == "" {
		http.Error(w, "slug is required", http.StatusBadRequest)
		return
	}
	if IsKnownSource(slug) {
		http.Error(w, fmt.Sprintf("%q is a built-in source and cannot be deleted", slug), http.StatusConflict)
		return
	}
	n, err := h.store.DeleteOpen5eCustomSource(r.Context(), slug)
	if err != nil {
		http.Error(w, "failed to delete custom source", http.StatusInternalServerError)
		return
	}
	if n == 0 {
		http.Error(w, "custom source not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

// UpdateCampaignSources replaces the campaign's open5e_sources list with the
// provided slugs. Every slug must appear in the catalog (built-in or custom);
// unknown slugs cause a 400 with the offending value in the error body. The
// update is a JSONB partial: other settings fields (turn_timeout_hours,
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

	known, err := h.knownSlugSet(r.Context())
	if err != nil {
		http.Error(w, "failed to load source catalog", http.StatusInternalServerError)
		return
	}
	cleaned, err := validateSlugsAgainst(known, req.Enabled)
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

// fullCatalog returns the built-in baseline followed by admin-added custom
// sources, all flagged appropriately via Source.Builtin.
func (h *SourcesHandler) fullCatalog(ctx context.Context) ([]Source, error) {
	out := Catalog() // built-ins, Builtin=true
	custom, err := h.store.ListOpen5eCustomSources(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range custom {
		out = append(out, Source{
			Slug:        c.Slug,
			Title:       c.Title,
			Publisher:   c.Publisher,
			Description: c.Description,
			Builtin:     false,
		})
	}
	return out, nil
}

// knownSlugSet is the allow-list the per-campaign enable path validates
// against: exactly the slugs of the full catalog (built-in ∪ custom). Deriving
// it from fullCatalog keeps "listable" and "enableable" from ever drifting.
func (h *SourcesHandler) knownSlugSet(ctx context.Context) (map[string]struct{}, error) {
	all, err := h.fullCatalog(ctx)
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(all))
	for _, s := range all {
		set[s.Slug] = struct{}{}
	}
	return set, nil
}

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

// validateSlugsAgainst trims duplicates / blanks and rejects any slug not in
// the supplied allow-list. Returns a fresh slice in input order. Pure (no DB)
// so the catalog lookup stays an explicit step in the caller.
func validateSlugsAgainst(known map[string]struct{}, in []string) ([]string, error) {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		if raw == "" {
			continue
		}
		if _, dup := seen[raw]; dup {
			continue
		}
		if _, ok := known[raw]; !ok {
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
