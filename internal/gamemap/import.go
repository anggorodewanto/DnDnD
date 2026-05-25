package gamemap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// SkippedFeatureType identifies a class of Tiled feature that was stripped during import.
type SkippedFeatureType string

const (
	SkippedTileAnimation SkippedFeatureType = "tile_animation"
	SkippedImageLayer    SkippedFeatureType = "image_layer"
	SkippedParallax      SkippedFeatureType = "parallax_scrolling"
	SkippedGroupLayer    SkippedFeatureType = "group_layer"
	SkippedTextObject    SkippedFeatureType = "text_object"
	SkippedPointObject   SkippedFeatureType = "point_object"
	SkippedWangSet       SkippedFeatureType = "wang_set"
)

// SkippedFeature describes one class of feature that was stripped during import.
type SkippedFeature struct {
	Feature SkippedFeatureType `json:"feature"`
	Detail  string             `json:"detail,omitempty"`
}

// Hard-rejection sentinel errors for Tiled imports.
var (
	ErrInfiniteMap            = errors.New("infinite maps are not supported")
	ErrNonOrthogonal          = errors.New("only orthogonal orientation is supported")
	ErrMapTooLarge            = errors.New("map dimensions exceed hard limit")
	ErrInvalidDimensions      = errors.New("map dimensions must be positive")
	ErrInvalidTiledJSON       = errors.New("invalid Tiled JSON")
	ErrExternalTileset        = errors.New("external (.tsx) tilesets are not supported; embed the tileset in the map before exporting")
	ErrImageCollectionTileset = errors.New("image-collection tilesets are not supported; use a single-image tileset")
	// ErrMissingImages reports that the map references image files that were not
	// supplied in the upload. The wrapped message lists the missing basenames.
	ErrMissingImages = errors.New("missing image files for import")
)

// ImageUploader persists a tileset or image-layer backing image and returns the
// URL the stored map should reference. It is intentionally decoupled from the
// asset package so gamemap does not import it; the application wires a concrete
// adapter via Service.SetImageUploader.
type ImageUploader interface {
	UploadMapImage(ctx context.Context, campaignID uuid.UUID, isTileset bool, filename, mimeType string, content io.Reader) (assetURL string, err error)
}

// ImportImage is a single image file supplied alongside a `.tmj` during a
// multipart Tiled-project import. Basename matches the (normalized) image
// reference inside the map; IsTileset is informational and recomputed from the
// parsed map during import.
type ImportImage struct {
	Basename  string
	MimeType  string
	Content   []byte
	IsTileset bool
}

// ParsedTileset describes a single-image embedded tileset that the renderer
// must blit sprites from. Image is the basename of the tileset's image file
// (the caller resolves it to an uploaded asset).
type ParsedTileset struct {
	Name        string
	FirstGID    int
	Image       string
	ImageWidth  int
	ImageHeight int
	TileWidth   int
	TileHeight  int
	Columns     int
	Margin      int
	Spacing     int
	TileCount   int
}

// ParsedImageLayer describes an image layer whose backing image must be
// uploaded. Image is the basename of the referenced file.
type ParsedImageLayer struct {
	Name  string
	Image string
}

// ImportResult is the outcome of a successful Tiled import. The TiledJSON has
// been cleaned of unsupported features (each removed class recorded in Skipped)
// and every tileset/image-layer image path normalized to its basename. Tilesets
// and ImageLayers enumerate the image files the caller must supply.
type ImportResult struct {
	TiledJSON   json.RawMessage
	Width       int
	Height      int
	Skipped     []SkippedFeature
	Tilesets    []ParsedTileset
	ImageLayers []ParsedImageLayer
}

// RequiredImages returns the deduplicated set of image basenames the map
// references across its tilesets and image layers.
func (r ImportResult) RequiredImages() []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, ts := range r.Tilesets {
		add(ts.Image)
	}
	for _, il := range r.ImageLayers {
		add(il.Image)
	}
	return out
}

// ImportTiledJSON validates and sanitizes a `.tmj` payload. Hard-rejection
// rules return a typed error; soft-rejection rules strip the offending
// feature and append to the Skipped list.
func ImportTiledJSON(raw json.RawMessage) (ImportResult, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ImportResult{}, fmt.Errorf("%w: %v", ErrInvalidTiledJSON, err)
	}

	if err := checkHardRejections(doc); err != nil {
		return ImportResult{}, err
	}

	skipped := newSkippedTracker()
	var imageLayers []ParsedImageLayer
	if layers, ok := doc["layers"].([]any); ok {
		doc["layers"] = sanitizeLayers(layers, skipped, &imageLayers)
	}
	var tilesets []ParsedTileset
	if rawTilesets, ok := doc["tilesets"].([]any); ok {
		cleaned, parsed, err := sanitizeTilesets(rawTilesets, skipped)
		if err != nil {
			return ImportResult{}, err
		}
		doc["tilesets"] = cleaned
		tilesets = parsed
	}

	cleaned, err := json.Marshal(doc)
	if err != nil {
		return ImportResult{}, fmt.Errorf("re-marshaling tiled json: %w", err)
	}

	return ImportResult{
		TiledJSON:   cleaned,
		Width:       intFrom(doc, "width"),
		Height:      intFrom(doc, "height"),
		Skipped:     skipped.list(),
		Tilesets:    tilesets,
		ImageLayers: imageLayers,
	}, nil
}

// ApplyImageAssets rewrites every tileset and image-layer `image` field in a
// cleaned Tiled document, replacing the basename with the resolved asset URL
// from byBasename. Paths absent from the map are left untouched. Used after the
// importer's image files are uploaded so the stored map is self-contained.
func ApplyImageAssets(raw json.RawMessage, byBasename map[string]string) (json.RawMessage, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTiledJSON, err)
	}

	rewrite := func(m map[string]any) {
		img, _ := m["image"].(string)
		if url, ok := byBasename[img]; ok {
			m["image"] = url
		}
	}
	if layers, ok := doc["layers"].([]any); ok {
		for _, raw := range layers {
			if layer, ok := raw.(map[string]any); ok {
				rewrite(layer)
			}
		}
	}
	if tilesets, ok := doc["tilesets"].([]any); ok {
		for _, raw := range tilesets {
			if ts, ok := raw.(map[string]any); ok {
				rewrite(ts)
			}
		}
	}

	out, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("re-marshaling tiled json: %w", err)
	}
	return out, nil
}

// checkHardRejections returns a typed sentinel error when the doc has a
// feature the system cannot support.
func checkHardRejections(doc map[string]any) error {
	if infinite, _ := doc["infinite"].(bool); infinite {
		return ErrInfiniteMap
	}
	if orient, _ := doc["orientation"].(string); orient != "" && orient != "orthogonal" {
		return fmt.Errorf("%w: got %q", ErrNonOrthogonal, orient)
	}
	width, height := intFrom(doc, "width"), intFrom(doc, "height")
	if width < 1 || height < 1 {
		return fmt.Errorf("%w: got %dx%d", ErrInvalidDimensions, width, height)
	}
	if width > HardLimitDimension || height > HardLimitDimension {
		return fmt.Errorf("%w: got %dx%d, max %d", ErrMapTooLarge, width, height, HardLimitDimension)
	}
	return nil
}

// sanitizeLayers walks the layer tree, flattening groups, stripping
// unsupported per-layer fields, and collecting the images that image layers
// reference. Image layers are retained (their backing image is uploaded
// separately and composited at render time).
func sanitizeLayers(layers []any, skipped *skippedTracker, imageLayers *[]ParsedImageLayer) []any {
	cleaned := make([]any, 0, len(layers))
	for _, raw := range layers {
		layer, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		layerType, _ := layer["type"].(string)
		if layerType == "group" {
			skipped.add(SkippedGroupLayer, "flattened into root layer list")
			children, _ := layer["layers"].([]any)
			cleaned = append(cleaned, sanitizeLayers(children, skipped, imageLayers)...)
			continue
		}

		stripParallax(layer, skipped)
		switch layerType {
		case "imagelayer":
			collectImageLayer(layer, imageLayers)
		case "objectgroup":
			layer["objects"] = sanitizeObjects(layer["objects"], skipped)
		}
		cleaned = append(cleaned, layer)
	}
	return cleaned
}

// collectImageLayer normalizes an image layer's image path to its basename and
// records the requirement.
func collectImageLayer(layer map[string]any, imageLayers *[]ParsedImageLayer) {
	img, _ := layer["image"].(string)
	if img == "" {
		return
	}
	base := baseName(img)
	layer["image"] = base
	name, _ := layer["name"].(string)
	*imageLayers = append(*imageLayers, ParsedImageLayer{Name: name, Image: base})
}

// stripParallax removes parallax fields from a layer and records the skip.
func stripParallax(layer map[string]any, skipped *skippedTracker) {
	_, hasX := layer["parallaxx"]
	_, hasY := layer["parallaxy"]
	if !hasX && !hasY {
		return
	}
	delete(layer, "parallaxx")
	delete(layer, "parallaxy")
	skipped.add(SkippedParallax, "")
}

// sanitizeObjects drops text and point objects from an objectgroup's object list.
func sanitizeObjects(raw any, skipped *skippedTracker) []any {
	objs, ok := raw.([]any)
	if !ok {
		return []any{}
	}
	cleaned := make([]any, 0, len(objs))
	for _, o := range objs {
		obj, ok := o.(map[string]any)
		if !ok {
			continue
		}
		if _, isText := obj["text"]; isText {
			skipped.add(SkippedTextObject, "")
			continue
		}
		if pt, _ := obj["point"].(bool); pt {
			skipped.add(SkippedPointObject, "")
			continue
		}
		cleaned = append(cleaned, obj)
	}
	return cleaned
}

// sanitizeTilesets strips wang sets and per-tile animations from each tileset,
// rejects unsupported tileset kinds (external .tsx, image collections), and
// returns the parsed single-image tilesets the renderer needs. Abstract
// tilesets (semantic tiles with a type but no image) are kept and produce no
// image requirement.
func sanitizeTilesets(tilesets []any, skipped *skippedTracker) ([]any, []ParsedTileset, error) {
	var parsed []ParsedTileset
	for _, raw := range tilesets {
		ts, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if src, _ := ts["source"].(string); src != "" {
			return nil, nil, fmt.Errorf("%w: %q", ErrExternalTileset, src)
		}
		if _, has := ts["wangsets"]; has {
			delete(ts, "wangsets")
			skipped.add(SkippedWangSet, "")
		}
		stripTileAnimations(ts, skipped)

		if img, _ := ts["image"].(string); img != "" {
			base := baseName(img)
			ts["image"] = base
			parsed = append(parsed, parseTileset(ts, base))
			continue
		}
		if tilesetHasTileImages(ts) {
			name, _ := ts["name"].(string)
			return nil, nil, fmt.Errorf("%w: %q", ErrImageCollectionTileset, name)
		}
		// No image anywhere: an abstract semantic tileset — keep as-is.
	}
	return tilesets, parsed, nil
}

// parseTileset reads the grid metadata from a single-image tileset.
func parseTileset(ts map[string]any, image string) ParsedTileset {
	name, _ := ts["name"].(string)
	return ParsedTileset{
		Name:        name,
		FirstGID:    intFrom(ts, "firstgid"),
		Image:       image,
		ImageWidth:  intFrom(ts, "imagewidth"),
		ImageHeight: intFrom(ts, "imageheight"),
		TileWidth:   intFrom(ts, "tilewidth"),
		TileHeight:  intFrom(ts, "tileheight"),
		Columns:     intFrom(ts, "columns"),
		Margin:      intFrom(ts, "margin"),
		Spacing:     intFrom(ts, "spacing"),
		TileCount:   intFrom(ts, "tilecount"),
	}
}

// stripTileAnimations removes per-tile animation frames from a tileset.
func stripTileAnimations(ts map[string]any, skipped *skippedTracker) {
	tiles, ok := ts["tiles"].([]any)
	if !ok {
		return
	}
	for _, t := range tiles {
		tile, ok := t.(map[string]any)
		if !ok {
			continue
		}
		if _, has := tile["animation"]; has {
			delete(tile, "animation")
			skipped.add(SkippedTileAnimation, "")
		}
	}
}

// tilesetHasTileImages reports whether any tile carries its own image, the
// signature of a "collection of images" tileset.
func tilesetHasTileImages(ts map[string]any) bool {
	tiles, ok := ts["tiles"].([]any)
	if !ok {
		return false
	}
	for _, t := range tiles {
		tile, ok := t.(map[string]any)
		if !ok {
			continue
		}
		if img, _ := tile["image"].(string); img != "" {
			return true
		}
	}
	return false
}

// baseName returns the final path element, tolerating both / and \ separators.
func baseName(p string) string {
	return path.Base(strings.ReplaceAll(p, "\\", "/"))
}

// intFrom extracts an integer-valued field from a parsed JSON object. JSON
// numbers decode as float64.
func intFrom(m map[string]any, key string) int {
	v, _ := m[key].(float64)
	return int(v)
}


// skippedTracker collects unique skipped features in insertion order.
type skippedTracker struct {
	seen  map[SkippedFeatureType]struct{}
	items []SkippedFeature
}

func newSkippedTracker() *skippedTracker {
	return &skippedTracker{seen: map[SkippedFeatureType]struct{}{}}
}

func (s *skippedTracker) add(feature SkippedFeatureType, detail string) {
	if _, ok := s.seen[feature]; ok {
		return
	}
	s.seen[feature] = struct{}{}
	s.items = append(s.items, SkippedFeature{Feature: feature, Detail: detail})
}

func (s *skippedTracker) list() []SkippedFeature { return s.items }

// ImportMapInput holds parameters for importing a Tiled map.
type ImportMapInput struct {
	CampaignID        uuid.UUID
	Name              string
	TiledJSON         json.RawMessage
	BackgroundImageID uuid.NullUUID
	TilesetRefs       []TilesetRef
	// Images are the tileset/image-layer backing files supplied alongside the
	// `.tmj`, matched to the map's references by Basename.
	Images []ImportImage
}

// ImportMap validates a `.tmj` payload, sanitizes it, uploads any referenced
// tileset/image-layer images, rewrites the map to point at the stored assets,
// and persists it. Returns the created map, its size category, and the list of
// features that were stripped during import.
func (s *Service) ImportMap(ctx context.Context, input ImportMapInput) (refdata.Map, SizeCategory, []SkippedFeature, error) {
	result, err := ImportTiledJSON(input.TiledJSON)
	if err != nil {
		return refdata.Map{}, "", nil, err
	}

	tiledJSON, err := s.resolveImages(ctx, input.CampaignID, result, input.Images)
	if err != nil {
		return refdata.Map{}, "", nil, err
	}

	m, cat, err := s.CreateMap(ctx, CreateMapInput{
		CampaignID:        input.CampaignID,
		Name:              input.Name,
		Width:             result.Width,
		Height:            result.Height,
		TiledJSON:         tiledJSON,
		BackgroundImageID: input.BackgroundImageID,
		TilesetRefs:       input.TilesetRefs,
	})
	if err != nil {
		return refdata.Map{}, "", nil, err
	}
	return m, cat, result.Skipped, nil
}

// resolveImages uploads every image the map requires and returns the Tiled JSON
// rewritten to reference the stored asset URLs. When the map references no
// images it returns the cleaned JSON unchanged.
func (s *Service) resolveImages(ctx context.Context, campaignID uuid.UUID, result ImportResult, images []ImportImage) (json.RawMessage, error) {
	required := result.RequiredImages()
	if len(required) == 0 {
		return result.TiledJSON, nil
	}
	if s.uploader == nil {
		return nil, fmt.Errorf("image uploader not configured but map references %d image(s)", len(required))
	}

	supplied := make(map[string]ImportImage, len(images))
	for _, img := range images {
		supplied[img.Basename] = img
	}

	tilesetImages := tilesetImageSet(result)

	var missing []string
	byBasename := make(map[string]string, len(required))
	for _, name := range required {
		img, ok := supplied[name]
		if !ok {
			missing = append(missing, name)
			continue
		}
		_, isTileset := tilesetImages[name]
		url, err := s.uploader.UploadMapImage(ctx, campaignID, isTileset, name, img.MimeType, bytes.NewReader(img.Content))
		if err != nil {
			return nil, fmt.Errorf("uploading image %q: %w", name, err)
		}
		byBasename[name] = url
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("%w: %s", ErrMissingImages, strings.Join(missing, ", "))
	}

	rewritten, err := ApplyImageAssets(result.TiledJSON, byBasename)
	if err != nil {
		return nil, err
	}
	return rewritten, nil
}

// tilesetImageSet returns the set of basenames referenced by tilesets (as
// opposed to image layers), so each upload can be tagged as a tileset image.
func tilesetImageSet(result ImportResult) map[string]struct{} {
	set := make(map[string]struct{}, len(result.Tilesets))
	for _, ts := range result.Tilesets {
		if ts.Image != "" {
			set[ts.Image] = struct{}{}
		}
	}
	return set
}
