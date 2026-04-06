package statblocklibrary

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler exposes the Stat Block Library HTTP API.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler wrapping the given service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts stat block library routes on the given Chi router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/statblocks", func(r chi.Router) {
		r.Get("/", h.List)
		r.Get("/{id}", h.Get)
	})
}

// List handles GET /api/statblocks with optional filters in the query string.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	filter, err := parseFilter(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	entries, err := h.svc.ListStatBlocks(r.Context(), filter)
	if err != nil {
		http.Error(w, "failed to list stat blocks", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

// Get handles GET /api/statblocks/{id}, enforcing campaign scoping for homebrew.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	campaignID, err := parseCampaignID(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	c, err := h.svc.GetStatBlock(r.Context(), id, campaignID)
	if errors.Is(err, ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "stat block not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to get stat block", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// parseFilter extracts StatBlockFilter fields from the query string.
func parseFilter(q url.Values) (StatBlockFilter, error) {
	filter := StatBlockFilter{
		Search: q.Get("search"),
		Types:  flattenMulti(q["type"]),
		Sizes:  flattenMulti(q["size"]),
	}

	campaignID, err := parseCampaignID(q)
	if err != nil {
		return filter, err
	}
	filter.CampaignID = campaignID

	if filter.CRMin, err = optionalFloat(q, "cr_min"); err != nil {
		return filter, err
	}
	if filter.CRMax, err = optionalFloat(q, "cr_max"); err != nil {
		return filter, err
	}

	source, err := parseSource(q.Get("source"))
	if err != nil {
		return filter, err
	}
	filter.Source = source

	if filter.Limit, err = optionalInt(q, "limit"); err != nil {
		return filter, err
	}
	if filter.Offset, err = optionalInt(q, "offset"); err != nil {
		return filter, err
	}

	return filter, nil
}

// parseCampaignID extracts and validates the campaign_id query field.
func parseCampaignID(q url.Values) (uuid.UUID, error) {
	v := q.Get("campaign_id")
	if v == "" {
		return uuid.Nil, nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid campaign_id")
	}
	return id, nil
}

// parseSource validates the source field (empty/srd/homebrew).
func parseSource(v string) (Source, error) {
	switch v {
	case "":
		return SourceAny, nil
	case "srd":
		return SourceSRD, nil
	case "homebrew":
		return SourceHomebrew, nil
	}
	return SourceAny, fmt.Errorf("invalid source")
}

// optionalFloat parses a float64 query field, returning nil when unset.
func optionalFloat(q url.Values, key string) (*float64, error) {
	v := q.Get(key)
	if v == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid %s", key)
	}
	return &parsed, nil
}

// optionalInt parses an int query field, returning 0 when unset.
func optionalInt(q url.Values, key string) (int, error) {
	v := q.Get(key)
	if v == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return parsed, nil
}

// flattenMulti accepts both repeated ?type=a&type=b and comma-separated ?type=a,b forms.
func flattenMulti(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			p := strings.TrimSpace(part)
			if p == "" {
				continue
			}
			out = append(out, p)
		}
	}
	return out
}

// writeJSON writes v as JSON with the given status. On encode failure, the
// response is already committed so we simply swallow the error.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
