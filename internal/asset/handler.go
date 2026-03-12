package asset

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler serves asset files over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler creates a new asset Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts asset API routes on the given Chi router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/api/assets/upload", h.UploadAsset)
	r.Get("/api/assets/{id}", h.ServeAsset)
}

// maxUploadSize limits the size of uploaded files (10 MB).
const maxUploadSize = 10 << 20

// UploadAsset handles POST /api/assets/upload with multipart form data.
// Required form fields: campaign_id, type, file.
func (h *Handler) UploadAsset(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "file too large or invalid multipart form", http.StatusBadRequest)
		return
	}

	campaignIDStr := r.FormValue("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}

	assetType := AssetType(r.FormValue("type"))

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file field required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	asset, err := h.svc.Upload(r.Context(), UploadInput{
		CampaignID:   campaignID,
		Type:         assetType,
		OriginalName: header.Filename,
		MimeType:     mimeType,
		Content:      file,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := map[string]string{
		"id":  asset.ID.String(),
		"url": h.svc.URL(asset.ID),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// ServeAsset streams the asset file identified by {id} in the URL.
func (h *Handler) ServeAsset(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid asset id", http.StatusBadRequest)
		return
	}

	asset, rc, err := h.svc.OpenFile(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "asset not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", asset.MimeType)
	if asset.ByteSize > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(asset.ByteSize, 10))
	}

	// Response already started; best-effort copy.
	_, _ = io.Copy(w, rc)
}
