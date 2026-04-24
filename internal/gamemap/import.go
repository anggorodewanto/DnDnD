package gamemap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
	ErrInfiniteMap       = errors.New("infinite maps are not supported")
	ErrNonOrthogonal     = errors.New("only orthogonal orientation is supported")
	ErrMapTooLarge       = errors.New("map dimensions exceed hard limit")
	ErrInvalidDimensions = errors.New("map dimensions must be positive")
	ErrInvalidTiledJSON  = errors.New("invalid Tiled JSON")
)

// ImportResult is the outcome of a successful Tiled import. The TiledJSON has
// been cleaned of all unsupported features; each removed class is recorded in
// Skipped so the DM can see what was stripped.
type ImportResult struct {
	TiledJSON json.RawMessage
	Width     int
	Height    int
	Skipped   []SkippedFeature
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
	if layers, ok := doc["layers"].([]any); ok {
		doc["layers"] = sanitizeLayers(layers, skipped)
	}
	if tilesets, ok := doc["tilesets"].([]any); ok {
		doc["tilesets"] = sanitizeTilesets(tilesets, skipped)
	}

	cleaned, err := json.Marshal(doc)
	if err != nil {
		return ImportResult{}, fmt.Errorf("re-marshaling tiled json: %w", err)
	}

	return ImportResult{
		TiledJSON: cleaned,
		Width:     intField(doc, "width"),
		Height:    intField(doc, "height"),
		Skipped:   skipped.list(),
	}, nil
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
	width, height := intField(doc, "width"), intField(doc, "height")
	if width < 1 || height < 1 {
		return fmt.Errorf("%w: got %dx%d", ErrInvalidDimensions, width, height)
	}
	if width > HardLimitDimension || height > HardLimitDimension {
		return fmt.Errorf("%w: got %dx%d, max %d", ErrMapTooLarge, width, height, HardLimitDimension)
	}
	return nil
}

// sanitizeLayers walks the layer tree, flattening groups and stripping
// unsupported layer kinds and unsupported per-layer fields.
func sanitizeLayers(layers []any, skipped *skippedTracker) []any {
	cleaned := make([]any, 0, len(layers))
	for _, raw := range layers {
		layer, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		layerType, _ := layer["type"].(string)
		switch layerType {
		case "group":
			skipped.add(SkippedGroupLayer, "flattened into root layer list")
			children, _ := layer["layers"].([]any)
			cleaned = append(cleaned, sanitizeLayers(children, skipped)...)
		case "imagelayer":
			skipped.add(SkippedImageLayer, "")
		default:
			stripParallax(layer, skipped)
			if layerType == "objectgroup" {
				layer["objects"] = sanitizeObjects(layer["objects"], skipped)
			}
			cleaned = append(cleaned, layer)
		}
	}
	return cleaned
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

// sanitizeTilesets removes wang sets and per-tile animations from each tileset.
func sanitizeTilesets(tilesets []any, skipped *skippedTracker) []any {
	for _, raw := range tilesets {
		ts, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, has := ts["wangsets"]; has {
			delete(ts, "wangsets")
			skipped.add(SkippedWangSet, "")
		}
		tiles, ok := ts["tiles"].([]any)
		if !ok {
			continue
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
	return tilesets
}

// intField extracts an integer-valued field from a parsed JSON map. JSON
// numbers come through as float64.
func intField(doc map[string]any, key string) int {
	v, _ := doc[key].(float64)
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
}

// ImportMap validates a `.tmj` payload, sanitizes it, and persists the map.
// Returns the created map, its size category, and the list of features that
// were stripped during import.
func (s *Service) ImportMap(ctx context.Context, input ImportMapInput) (refdata.Map, SizeCategory, []SkippedFeature, error) {
	result, err := ImportTiledJSON(input.TiledJSON)
	if err != nil {
		return refdata.Map{}, "", nil, err
	}

	m, cat, err := s.CreateMap(ctx, CreateMapInput{
		CampaignID:        input.CampaignID,
		Name:              input.Name,
		Width:             result.Width,
		Height:            result.Height,
		TiledJSON:         result.TiledJSON,
		BackgroundImageID: input.BackgroundImageID,
		TilesetRefs:       input.TilesetRefs,
	})
	if err != nil {
		return refdata.Map{}, "", nil, err
	}
	return m, cat, result.Skipped, nil
}
