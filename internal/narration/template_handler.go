package narration

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TemplateHandler exposes the narration template REST API.
type TemplateHandler struct {
	svc *TemplateService
}

// NewTemplateHandler constructs a TemplateHandler.
func NewTemplateHandler(svc *TemplateService) *TemplateHandler {
	return &TemplateHandler{svc: svc}
}

// RegisterRoutes mounts the template routes under /api/narration/templates.
func (h *TemplateHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/narration/templates", func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/{id}", h.Get)
		r.Put("/{id}", h.Update)
		r.Delete("/{id}", h.Delete)
		r.Post("/{id}/duplicate", h.Duplicate)
		r.Post("/{id}/apply", h.Apply)
	})
}

type templateCreateRequest struct {
	CampaignID string `json:"campaign_id"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	Body       string `json:"body"`
}

type templateUpdateRequest struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Body     string `json:"body"`
}

type templateApplyRequest struct {
	Values map[string]string `json:"values"`
}

type templateApplyResponse struct {
	Body         string   `json:"body"`
	Placeholders []string `json:"placeholders"`
}

// Create handles POST /api/narration/templates.
func (h *TemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req templateCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	campID, err := uuid.Parse(req.CampaignID)
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}
	tpl, err := h.svc.Create(r.Context(), CreateTemplateInput{
		CampaignID: campID,
		Name:       req.Name,
		Category:   req.Category,
		Body:       req.Body,
	})
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tpl)
}

// List handles GET /api/narration/templates?campaign_id=...&category=...&q=...
func (h *TemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	campStr := r.URL.Query().Get("campaign_id")
	if campStr == "" {
		http.Error(w, "campaign_id required", http.StatusBadRequest)
		return
	}
	campID, err := uuid.Parse(campStr)
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}
	templates, err := h.svc.List(r.Context(), TemplateFilter{
		CampaignID: campID,
		Category:   r.URL.Query().Get("category"),
		Search:     r.URL.Query().Get("q"),
	})
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	if templates == nil {
		templates = []Template{}
	}
	writeJSON(w, http.StatusOK, templates)
}

// Get handles GET /api/narration/templates/{id}?campaign_id=...
func (h *TemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	campID, ok := parseCampaignIDParam(w, r)
	if !ok {
		return
	}
	tpl, err := h.svc.Get(r.Context(), id, campID)
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tpl)
}

// Update handles PUT /api/narration/templates/{id}?campaign_id=...
func (h *TemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	campID, ok := parseCampaignIDParam(w, r)
	if !ok {
		return
	}
	var req templateUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	tpl, err := h.svc.Update(r.Context(), id, campID, UpdateTemplateInput{
		Name:     req.Name,
		Category: req.Category,
		Body:     req.Body,
	})
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tpl)
}

// Delete handles DELETE /api/narration/templates/{id}?campaign_id=...
func (h *TemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	campID, ok := parseCampaignIDParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), id, campID); err != nil {
		writeTemplateError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Duplicate handles POST /api/narration/templates/{id}/duplicate?campaign_id=...
func (h *TemplateHandler) Duplicate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	campID, ok := parseCampaignIDParam(w, r)
	if !ok {
		return
	}
	tpl, err := h.svc.Duplicate(r.Context(), id, campID)
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tpl)
}

// Apply handles POST /api/narration/templates/{id}/apply?campaign_id=...
func (h *TemplateHandler) Apply(w http.ResponseWriter, r *http.Request) {
	id, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	campID, ok := parseCampaignIDParam(w, r)
	if !ok {
		return
	}
	var req templateApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	body, err := h.svc.Apply(r.Context(), id, campID, req.Values)
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, templateApplyResponse{
		Body:         body,
		Placeholders: ExtractPlaceholders(body),
	})
}

// parseTemplateID extracts and validates the {id} URL param.
func parseTemplateID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return uuid.Nil, false
	}
	return id, true
}

// parseCampaignIDParam extracts and validates the campaign_id query parameter.
func parseCampaignIDParam(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	campStr := r.URL.Query().Get("campaign_id")
	if campStr == "" {
		http.Error(w, "campaign_id required", http.StatusBadRequest)
		return uuid.Nil, false
	}
	campID, err := uuid.Parse(campStr)
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return uuid.Nil, false
	}
	return campID, true
}

// writeTemplateError maps known errors to status codes.
func writeTemplateError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrTemplateNotFound) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if errors.Is(err, ErrInvalidInput) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
