package gamemap

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// importMaxMemory bounds the in-memory portion of a multipart import upload;
// the rest spills to temp files. Tiled images are typically a few MB each.
const importMaxMemory = 32 << 20

// importMapResponse is the response for a successful import.
type importMapResponse struct {
	Map     mapResponse      `json:"map"`
	Skipped []SkippedFeature `json:"skipped"`
}

// ImportMap handles POST /api/maps/import. It accepts a multipart/form-data
// upload carrying the `.tmj` document (`tmj` file part) plus any referenced
// tileset/image-layer files (`images` file parts, repeated), validates the
// payload, strips unsupported features, uploads the images, persists the map,
// and returns a summary listing every class of feature that was stripped.
func (h *Handler) ImportMap(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(importMaxMemory); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}

	campaignID, err := uuid.Parse(r.FormValue("campaign_id"))
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}

	tmj, err := readTMJPart(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	images, err := readImageParts(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	m, _, skipped, err := h.svc.ImportMap(r.Context(), ImportMapInput{
		CampaignID: campaignID,
		Name:       r.FormValue("name"),
		TiledJSON:  tmj,
		Images:     images,
	})
	if err != nil {
		handleImportError(w, err)
		return
	}

	if skipped == nil {
		skipped = []SkippedFeature{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(importMapResponse{
		Map:     newMapResponse(m),
		Skipped: skipped,
	})
}

// ReimportMap handles PUT /api/maps/{id}/import. Same multipart shape as
// ImportMap (form fields campaign_id, name; file part tmj; repeated file parts
// images) but overwrites the existing map identified by {id} instead of
// creating a new one. Responds 200 on success.
func (h *Handler) ReimportMap(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid map id", http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(importMaxMemory); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}

	campaignID, err := uuid.Parse(r.FormValue("campaign_id"))
	if err != nil {
		http.Error(w, "invalid campaign_id", http.StatusBadRequest)
		return
	}

	tmj, err := readTMJPart(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	images, err := readImageParts(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	m, _, skipped, err := h.svc.ReimportMap(r.Context(), ReimportMapInput{
		ID:         id,
		CampaignID: campaignID,
		Name:       r.FormValue("name"),
		TiledJSON:  tmj,
		Images:     images,
	})
	if err != nil {
		if err.Error() == errNotFound.Error() {
			http.Error(w, "map not found", http.StatusNotFound)
			return
		}
		handleImportError(w, err)
		return
	}

	if skipped == nil {
		skipped = []SkippedFeature{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(importMapResponse{
		Map:     newMapResponse(m),
		Skipped: skipped,
	})
}

// readTMJPart reads the `tmj` file part into raw JSON bytes.
func readTMJPart(r *http.Request) (json.RawMessage, error) {
	file, _, err := r.FormFile("tmj")
	if err != nil {
		return nil, errors.New("tmj file is required")
	}
	defer file.Close()

	raw, err := io.ReadAll(file)
	if err != nil {
		return nil, errors.New("reading tmj file")
	}
	if len(raw) == 0 {
		return nil, errors.New("tmj must not be empty")
	}
	return json.RawMessage(raw), nil
}

// readImageParts reads every `images` file part into an ImportImage, deriving
// the basename from the upload filename and the MIME type from the part header
// (falling back to content sniffing).
func readImageParts(r *http.Request) ([]ImportImage, error) {
	if r.MultipartForm == nil {
		return nil, nil
	}
	headers := r.MultipartForm.File["images"]
	images := make([]ImportImage, 0, len(headers))
	for _, fh := range headers {
		img, err := readImagePart(fh)
		if err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, nil
}

// readImagePart reads a single uploaded image file header into an ImportImage.
func readImagePart(fh *multipart.FileHeader) (ImportImage, error) {
	f, err := fh.Open()
	if err != nil {
		return ImportImage{}, fmt.Errorf("opening image %q", fh.Filename)
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return ImportImage{}, fmt.Errorf("reading image %q", fh.Filename)
	}

	mimeType := fh.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = http.DetectContentType(content)
	}

	return ImportImage{
		Basename: path.Base(fh.Filename),
		MimeType: mimeType,
		Content:  content,
	}, nil
}

// handleImportError maps importer errors to HTTP responses, surfacing
// hard-rejection sentinels and missing-image errors as 400s and falling back to
// the shared service error handler for everything else.
func handleImportError(w http.ResponseWriter, err error) {
	for _, sentinel := range []error{
		ErrInfiniteMap,
		ErrNonOrthogonal,
		ErrMapTooLarge,
		ErrInvalidDimensions,
		ErrInvalidTiledJSON,
		ErrExternalTileset,
		ErrImageCollectionTileset,
		ErrMissingImages,
	} {
		if errors.Is(err, sentinel) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	handleServiceError(w, err)
}
