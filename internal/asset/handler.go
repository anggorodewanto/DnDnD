package asset

import (
	"database/sql"
	"errors"
	"fmt"
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

	if _, err := io.Copy(w, rc); err != nil {
		// Response already started, can't change status code.
		// Just log if we had a logger. For now, best-effort.
		_ = fmt.Errorf("streaming asset %s: %w", id, err)
	}
}
