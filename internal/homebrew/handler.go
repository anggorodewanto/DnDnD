package homebrew

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// Handler exposes the homebrew CRUD API.
type Handler struct {
	svc *Service
}

// NewHandler builds a Handler over the given Service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts homebrew routes on the given Chi router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/homebrew", func(r chi.Router) {
		r.Post("/creatures", h.CreateCreature)
		r.Put("/creatures/{id}", h.UpdateCreature)
		r.Delete("/creatures/{id}", h.DeleteCreature)

		r.Post("/spells", h.CreateSpell)
		r.Put("/spells/{id}", h.UpdateSpell)
		r.Delete("/spells/{id}", h.DeleteSpell)

		r.Post("/weapons", h.CreateWeapon)
		r.Put("/weapons/{id}", h.UpdateWeapon)
		r.Delete("/weapons/{id}", h.DeleteWeapon)

		r.Post("/magic-items", h.CreateMagicItem)
		r.Put("/magic-items/{id}", h.UpdateMagicItem)
		r.Delete("/magic-items/{id}", h.DeleteMagicItem)

		r.Post("/races", h.CreateRace)
		r.Put("/races/{id}", h.UpdateRace)
		r.Delete("/races/{id}", h.DeleteRace)

		r.Post("/feats", h.CreateFeat)
		r.Put("/feats/{id}", h.UpdateFeat)
		r.Delete("/feats/{id}", h.DeleteFeat)

		r.Post("/classes", h.CreateClass)
		r.Put("/classes/{id}", h.UpdateClass)
		r.Delete("/classes/{id}", h.DeleteClass)
	})
}

// --- shared helpers ---

// parseCampaignID extracts and validates the required campaign_id query
// param. Empty/invalid → 400.
func parseCampaignID(q url.Values) (uuid.UUID, error) {
	v := q.Get("campaign_id")
	if v == "" {
		return uuid.Nil, fmt.Errorf("campaign_id required")
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid campaign_id")
	}
	return id, nil
}

// decodeBody parses the JSON body into dst. Returns an error suitable for 400.
func decodeBody(r *http.Request, dst any) error {
	if r.Body == nil {
		return fmt.Errorf("missing request body")
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	return nil
}

// writeJSON writes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr writes a JSON error envelope.
func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// translateServiceErr maps service errors to HTTP status codes and writes
// the response.
func translateServiceErr(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrInvalidInput) {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeErr(w, http.StatusInternalServerError, "internal server error")
}

// readCampaignAndID extracts campaign_id (query) and id (URL param), or
// writes a 400 and returns false on error.
func readCampaignAndID(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, bool) {
	cid, err := parseCampaignID(r.URL.Query())
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return uuid.Nil, "", false
	}
	return cid, chi.URLParam(r, "id"), true
}

// readCampaignOnly extracts campaign_id, returning false on error.
func readCampaignOnly(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	cid, err := parseCampaignID(r.URL.Query())
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return uuid.Nil, false
	}
	return cid, true
}

// =============================================================================
// CREATURES
// =============================================================================

// CreateCreature handles POST /api/homebrew/creatures.
func (h *Handler) CreateCreature(w http.ResponseWriter, r *http.Request) {
	cid, ok := readCampaignOnly(w, r)
	if !ok {
		return
	}
	var body refdata.UpsertCreatureParams
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	got, err := h.svc.CreateHomebrewCreature(r.Context(), cid, body)
	if err != nil {
		translateServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, got)
}

// UpdateCreature handles PUT /api/homebrew/creatures/{id}.
func (h *Handler) UpdateCreature(w http.ResponseWriter, r *http.Request) {
	cid, id, ok := readCampaignAndID(w, r)
	if !ok {
		return
	}
	var body refdata.UpsertCreatureParams
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	got, err := h.svc.UpdateHomebrewCreature(r.Context(), cid, id, body)
	if err != nil {
		translateServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, got)
}

// DeleteCreature handles DELETE /api/homebrew/creatures/{id}.
func (h *Handler) DeleteCreature(w http.ResponseWriter, r *http.Request) {
	cid, id, ok := readCampaignAndID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteHomebrewCreature(r.Context(), cid, id); err != nil {
		translateServiceErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// =============================================================================
// SPELLS
// =============================================================================

func (h *Handler) CreateSpell(w http.ResponseWriter, r *http.Request) {
	cid, ok := readCampaignOnly(w, r)
	if !ok {
		return
	}
	var body refdata.UpsertSpellParams
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	got, err := h.svc.CreateHomebrewSpell(r.Context(), cid, body)
	if err != nil {
		translateServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, got)
}

func (h *Handler) UpdateSpell(w http.ResponseWriter, r *http.Request) {
	cid, id, ok := readCampaignAndID(w, r)
	if !ok {
		return
	}
	var body refdata.UpsertSpellParams
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	got, err := h.svc.UpdateHomebrewSpell(r.Context(), cid, id, body)
	if err != nil {
		translateServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func (h *Handler) DeleteSpell(w http.ResponseWriter, r *http.Request) {
	cid, id, ok := readCampaignAndID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteHomebrewSpell(r.Context(), cid, id); err != nil {
		translateServiceErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// =============================================================================
// WEAPONS
// =============================================================================

func (h *Handler) CreateWeapon(w http.ResponseWriter, r *http.Request) {
	cid, ok := readCampaignOnly(w, r)
	if !ok {
		return
	}
	var body refdata.UpsertWeaponParams
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	got, err := h.svc.CreateHomebrewWeapon(r.Context(), cid, body)
	if err != nil {
		translateServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, got)
}

func (h *Handler) UpdateWeapon(w http.ResponseWriter, r *http.Request) {
	cid, id, ok := readCampaignAndID(w, r)
	if !ok {
		return
	}
	var body refdata.UpsertWeaponParams
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	got, err := h.svc.UpdateHomebrewWeapon(r.Context(), cid, id, body)
	if err != nil {
		translateServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func (h *Handler) DeleteWeapon(w http.ResponseWriter, r *http.Request) {
	cid, id, ok := readCampaignAndID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteHomebrewWeapon(r.Context(), cid, id); err != nil {
		translateServiceErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// =============================================================================
// MAGIC ITEMS
// =============================================================================

func (h *Handler) CreateMagicItem(w http.ResponseWriter, r *http.Request) {
	cid, ok := readCampaignOnly(w, r)
	if !ok {
		return
	}
	var body refdata.UpsertMagicItemParams
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	got, err := h.svc.CreateHomebrewMagicItem(r.Context(), cid, body)
	if err != nil {
		translateServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, got)
}

func (h *Handler) UpdateMagicItem(w http.ResponseWriter, r *http.Request) {
	cid, id, ok := readCampaignAndID(w, r)
	if !ok {
		return
	}
	var body refdata.UpsertMagicItemParams
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	got, err := h.svc.UpdateHomebrewMagicItem(r.Context(), cid, id, body)
	if err != nil {
		translateServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func (h *Handler) DeleteMagicItem(w http.ResponseWriter, r *http.Request) {
	cid, id, ok := readCampaignAndID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteHomebrewMagicItem(r.Context(), cid, id); err != nil {
		translateServiceErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// =============================================================================
// RACES
// =============================================================================

func (h *Handler) CreateRace(w http.ResponseWriter, r *http.Request) {
	cid, ok := readCampaignOnly(w, r)
	if !ok {
		return
	}
	var body refdata.UpsertRaceParams
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	got, err := h.svc.CreateHomebrewRace(r.Context(), cid, body)
	if err != nil {
		translateServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, got)
}

func (h *Handler) UpdateRace(w http.ResponseWriter, r *http.Request) {
	cid, id, ok := readCampaignAndID(w, r)
	if !ok {
		return
	}
	var body refdata.UpsertRaceParams
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	got, err := h.svc.UpdateHomebrewRace(r.Context(), cid, id, body)
	if err != nil {
		translateServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func (h *Handler) DeleteRace(w http.ResponseWriter, r *http.Request) {
	cid, id, ok := readCampaignAndID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteHomebrewRace(r.Context(), cid, id); err != nil {
		translateServiceErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// =============================================================================
// FEATS
// =============================================================================

func (h *Handler) CreateFeat(w http.ResponseWriter, r *http.Request) {
	cid, ok := readCampaignOnly(w, r)
	if !ok {
		return
	}
	var body refdata.UpsertFeatParams
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	got, err := h.svc.CreateHomebrewFeat(r.Context(), cid, body)
	if err != nil {
		translateServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, got)
}

func (h *Handler) UpdateFeat(w http.ResponseWriter, r *http.Request) {
	cid, id, ok := readCampaignAndID(w, r)
	if !ok {
		return
	}
	var body refdata.UpsertFeatParams
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	got, err := h.svc.UpdateHomebrewFeat(r.Context(), cid, id, body)
	if err != nil {
		translateServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func (h *Handler) DeleteFeat(w http.ResponseWriter, r *http.Request) {
	cid, id, ok := readCampaignAndID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteHomebrewFeat(r.Context(), cid, id); err != nil {
		translateServiceErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// =============================================================================
// CLASSES
// =============================================================================

func (h *Handler) CreateClass(w http.ResponseWriter, r *http.Request) {
	cid, ok := readCampaignOnly(w, r)
	if !ok {
		return
	}
	var body refdata.UpsertClassParams
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	got, err := h.svc.CreateHomebrewClass(r.Context(), cid, body)
	if err != nil {
		translateServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, got)
}

func (h *Handler) UpdateClass(w http.ResponseWriter, r *http.Request) {
	cid, id, ok := readCampaignAndID(w, r)
	if !ok {
		return
	}
	var body refdata.UpsertClassParams
	if err := decodeBody(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	got, err := h.svc.UpdateHomebrewClass(r.Context(), cid, id, body)
	if err != nil {
		translateServiceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func (h *Handler) DeleteClass(w http.ResponseWriter, r *http.Request) {
	cid, id, ok := readCampaignAndID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteHomebrewClass(r.Context(), cid, id); err != nil {
		translateServiceErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
