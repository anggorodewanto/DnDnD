package homebrew

import (
	"context"
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

// serviceFuncs bundles the three Service methods (create/update/delete)
// for a single refdata type so routes can be registered via a table.
type serviceFuncs[P any, R any] struct {
	create func(ctx context.Context, campaignID uuid.UUID, params P) (R, error)
	update func(ctx context.Context, campaignID uuid.UUID, id string, params P) (R, error)
	delete func(ctx context.Context, campaignID uuid.UUID, id string) error
}

// mount registers the create/update/delete routes for one homebrew type
// under the given collection path.
func mount[P any, R any](r chi.Router, path string, fns serviceFuncs[P, R]) {
	r.Post(path, makeCreate(fns.create))
	r.Put(path+"/{id}", makeUpdate(fns.update))
	r.Delete(path+"/{id}", makeDelete(fns.delete))
}

// RegisterRoutes mounts homebrew routes on the given Chi router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/homebrew", func(r chi.Router) {
		mount(r, "/creatures", serviceFuncs[refdata.UpsertCreatureParams, refdata.Creature]{
			create: h.svc.CreateHomebrewCreature,
			update: h.svc.UpdateHomebrewCreature,
			delete: h.svc.DeleteHomebrewCreature,
		})
		mount(r, "/spells", serviceFuncs[refdata.UpsertSpellParams, refdata.Spell]{
			create: h.svc.CreateHomebrewSpell,
			update: h.svc.UpdateHomebrewSpell,
			delete: h.svc.DeleteHomebrewSpell,
		})
		mount(r, "/weapons", serviceFuncs[refdata.UpsertWeaponParams, refdata.Weapon]{
			create: h.svc.CreateHomebrewWeapon,
			update: h.svc.UpdateHomebrewWeapon,
			delete: h.svc.DeleteHomebrewWeapon,
		})
		mount(r, "/magic-items", serviceFuncs[refdata.UpsertMagicItemParams, refdata.MagicItem]{
			create: h.svc.CreateHomebrewMagicItem,
			update: h.svc.UpdateHomebrewMagicItem,
			delete: h.svc.DeleteHomebrewMagicItem,
		})
		mount(r, "/races", serviceFuncs[refdata.UpsertRaceParams, refdata.Race]{
			create: h.svc.CreateHomebrewRace,
			update: h.svc.UpdateHomebrewRace,
			delete: h.svc.DeleteHomebrewRace,
		})
		mount(r, "/feats", serviceFuncs[refdata.UpsertFeatParams, refdata.Feat]{
			create: h.svc.CreateHomebrewFeat,
			update: h.svc.UpdateHomebrewFeat,
			delete: h.svc.DeleteHomebrewFeat,
		})
		mount(r, "/classes", serviceFuncs[refdata.UpsertClassParams, refdata.Class]{
			create: h.svc.CreateHomebrewClass,
			update: h.svc.UpdateHomebrewClass,
			delete: h.svc.DeleteHomebrewClass,
		})
	})
}

// makeCreate returns a POST handler that decodes a JSON body into P and
// forwards to the given Service.Create method.
func makeCreate[P any, R any](create func(ctx context.Context, campaignID uuid.UUID, params P) (R, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cid, ok := readCampaignOnly(w, r)
		if !ok {
			return
		}
		var body P
		if err := decodeBody(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		got, err := create(r.Context(), cid, body)
		if err != nil {
			translateServiceErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, got)
	}
}

// makeUpdate returns a PUT handler that decodes a JSON body into P and
// forwards to the given Service.Update method.
func makeUpdate[P any, R any](update func(ctx context.Context, campaignID uuid.UUID, id string, params P) (R, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cid, id, ok := readCampaignAndID(w, r)
		if !ok {
			return
		}
		var body P
		if err := decodeBody(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		got, err := update(r.Context(), cid, id, body)
		if err != nil {
			translateServiceErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, got)
	}
}

// makeDelete returns a DELETE handler that forwards to the given
// Service.Delete method.
func makeDelete(del func(ctx context.Context, campaignID uuid.UUID, id string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cid, id, ok := readCampaignAndID(w, r)
		if !ok {
			return
		}
		if err := del(r.Context(), cid, id); err != nil {
			translateServiceErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
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
