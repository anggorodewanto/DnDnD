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
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "stat block not found", http.StatusNotFound)
			return
		}
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

	if v := q.Get("cr_min"); v != "" {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return filter, fmt.Errorf("invalid cr_min")
		}
		filter.CRMin = &parsed
	}
	if v := q.Get("cr_max"); v != "" {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return filter, fmt.Errorf("invalid cr_max")
		}
		filter.CRMax = &parsed
	}

	source, err := parseSource(q.Get("source"))
	if err != nil {
		return filter, err
	}
	filter.Source = source

	if v := q.Get("limit"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return filter, fmt.Errorf("invalid limit")
		}
		filter.Limit = parsed
	}
	if v := q.Get("offset"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return filter, fmt.Errorf("invalid offset")
		}
		filter.Offset = parsed
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
