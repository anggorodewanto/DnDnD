package open5e

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Handler exposes the Open5e integration HTTP API.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler wrapping the given Service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts the Open5e endpoints under /api/open5e.
// Deprecated: use RegisterPublicRoutes + RegisterProtectedRoutes for proper auth scoping.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/open5e", func(r chi.Router) {
		r.Get("/monsters", h.SearchMonsters)
		r.Post("/monsters/{slug}", h.CacheMonster)
		r.Get("/spells", h.SearchSpells)
		r.Post("/spells/{slug}", h.CacheSpell)
	})
}

// RegisterPublicRoutes mounts read-only Open5e search endpoints (no auth required).
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Get("/api/open5e/monsters", h.SearchMonsters)
	r.Get("/api/open5e/spells", h.SearchSpells)
}

// RegisterProtectedRoutes mounts mutating Open5e cache endpoints (DM auth required).
func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.Post("/api/open5e/monsters/{slug}", h.CacheMonster)
	r.Post("/api/open5e/spells/{slug}", h.CacheSpell)
}

// SearchMonsters GET /api/open5e/monsters?search=&document=&limit=&offset=.
func (h *Handler) SearchMonsters(w http.ResponseWriter, r *http.Request) {
	q, err := parseSearchQuery(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := h.svc.SearchMonsters(r.Context(), q)
	if err != nil {
		http.Error(w, "open5e upstream error", http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// CacheMonster POST /api/open5e/monsters/{slug}.
func (h *Handler) CacheMonster(w http.ResponseWriter, r *http.Request) {
	slug, err := extractSlug(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, err := h.svc.SearchAndCacheMonster(r.Context(), slug)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to cache monster: %v", err), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "slug": slug})
}

// SearchSpells GET /api/open5e/spells?search=&document=&limit=&offset=.
func (h *Handler) SearchSpells(w http.ResponseWriter, r *http.Request) {
	q, err := parseSearchQuery(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := h.svc.SearchSpells(r.Context(), q)
	if err != nil {
		http.Error(w, "open5e upstream error", http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// CacheSpell POST /api/open5e/spells/{slug}.
func (h *Handler) CacheSpell(w http.ResponseWriter, r *http.Request) {
	slug, err := extractSlug(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, err := h.svc.SearchAndCacheSpell(r.Context(), slug)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to cache spell: %v", err), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "slug": slug})
}

// --- helpers ---

// extractSlug pulls the {slug} URL param and rejects empty/whitespace.
func extractSlug(r *http.Request) (string, error) {
	raw := chi.URLParam(r, "slug")
	// chi URL params arrive already path-unescaped but we defensively
	// run the unescape again in case a future middleware double-encodes.
	slug, err := url.PathUnescape(raw)
	if err != nil {
		slug = raw
	}
	if strings.TrimSpace(slug) == "" {
		return "", errors.New("slug is required")
	}
	return slug, nil
}

// parseSearchQuery extracts SearchQuery from url.Values.
func parseSearchQuery(q url.Values) (SearchQuery, error) {
	sq := SearchQuery{
		Search:       strings.TrimSpace(q.Get("search")),
		DocumentSlug: strings.TrimSpace(q.Get("document")),
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return SearchQuery{}, errors.New("invalid limit")
		}
		sq.Limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return SearchQuery{}, errors.New("invalid offset")
		}
		sq.Offset = n
	}
	return sq, nil
}

// writeJSON encodes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
