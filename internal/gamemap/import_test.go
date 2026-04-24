package gamemap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// validTmj returns a minimal valid orthogonal Tiled JSON map.
func validTmj(width, height int) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{
		"version":"1.10",
		"tiledversion":"1.10.0",
		"type":"map",
		"orientation":"orthogonal",
		"renderorder":"right-down",
		"width":%d,
		"height":%d,
		"tilewidth":48,
		"tileheight":48,
		"infinite":false,
		"layers":[
			{"type":"tilelayer","name":"terrain","width":%d,"height":%d,"data":[1]}
		],
		"tilesets":[]
	}`, width, height, width, height))
}

// --- TDD Cycle 1: ImportTiledJSON parses valid orthogonal map ---

func TestImportTiledJSON_ValidOrthogonal(t *testing.T) {
	raw := validTmj(20, 15)
	result, err := ImportTiledJSON(raw)
	require.NoError(t, err)
	assert.Equal(t, 20, result.Width)
	assert.Equal(t, 15, result.Height)
	assert.NotEmpty(t, result.TiledJSON)
	assert.Empty(t, result.Skipped, "no features stripped on a clean map")
}

// --- TDD Cycle 2: Hard rejection — infinite maps ---

func TestImportTiledJSON_RejectInfinite(t *testing.T) {
	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":true,
		"layers":[]
	}`)
	_, err := ImportTiledJSON(raw)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInfiniteMap), "expected ErrInfiniteMap, got %v", err)
}

// --- TDD Cycle 3: Hard rejection — non-orthogonal orientations ---

func TestImportTiledJSON_RejectNonOrthogonal(t *testing.T) {
	cases := []string{"isometric", "staggered", "hexagonal"}
	for _, orient := range cases {
		t.Run(orient, func(t *testing.T) {
			raw := json.RawMessage(`{
				"orientation":"` + orient + `",
				"width":10,"height":10,"tilewidth":48,"tileheight":48,
				"infinite":false,
				"layers":[]
			}`)
			_, err := ImportTiledJSON(raw)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrNonOrthogonal), "expected ErrNonOrthogonal for %s, got %v", orient, err)
		})
	}
}

// --- TDD Cycle 4: Hard rejection — too large ---

func TestImportTiledJSON_RejectTooLarge(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		height int
	}{
		{"width", 201, 100},
		{"height", 100, 201},
		{"both", 250, 250},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := validTmj(tc.width, tc.height)
			_, err := ImportTiledJSON(raw)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrMapTooLarge), "expected ErrMapTooLarge, got %v", err)
		})
	}
}

// --- TDD Cycle 5: Hard rejection — invalid JSON ---

func TestImportTiledJSON_InvalidJSON(t *testing.T) {
	_, err := ImportTiledJSON(json.RawMessage(`not json`))
	require.Error(t, err)
}

// --- TDD Cycle 6: Hard rejection — non-positive dimensions ---

func TestImportTiledJSON_RejectZeroDimensions(t *testing.T) {
	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":0,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[]
	}`)
	_, err := ImportTiledJSON(raw)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidDimensions), "expected ErrInvalidDimensions, got %v", err)
}

// --- TDD Cycle 7: Skip — tile animations ---

func TestImportTiledJSON_StripTileAnimations(t *testing.T) {
	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[
			{"type":"tilelayer","name":"terrain","width":10,"height":10,"data":[1]}
		],
		"tilesets":[
			{"firstgid":1,"name":"t","tiles":[
				{"id":0,"type":"open_ground","animation":[{"tileid":0,"duration":100}]},
				{"id":1,"type":"water"}
			]}
		]
	}`)
	result, err := ImportTiledJSON(raw)
	require.NoError(t, err)
	assert.True(t, hasSkipped(result.Skipped, SkippedTileAnimation), "expected animation in skipped list, got %v", result.Skipped)

	// Assert the animation was actually removed from the JSON
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result.TiledJSON, &parsed))
	tilesets := parsed["tilesets"].([]any)
	tiles := tilesets[0].(map[string]any)["tiles"].([]any)
	for _, tile := range tiles {
		_, has := tile.(map[string]any)["animation"]
		assert.False(t, has, "animation should be stripped")
	}
}

// --- TDD Cycle 8: Skip — image layers ---

func TestImportTiledJSON_StripImageLayers(t *testing.T) {
	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[
			{"type":"tilelayer","name":"terrain","width":10,"height":10,"data":[1]},
			{"type":"imagelayer","name":"bg","image":"bg.png"}
		]
	}`)
	result, err := ImportTiledJSON(raw)
	require.NoError(t, err)
	assert.True(t, hasSkipped(result.Skipped, SkippedImageLayer))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result.TiledJSON, &parsed))
	layers := parsed["layers"].([]any)
	for _, l := range layers {
		assert.NotEqual(t, "imagelayer", l.(map[string]any)["type"])
	}
	assert.Len(t, layers, 1)
}

// --- TDD Cycle 9: Skip — parallax scrolling factors ---

func TestImportTiledJSON_StripParallax(t *testing.T) {
	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[
			{"type":"tilelayer","name":"terrain","width":10,"height":10,"data":[1],"parallaxx":0.5,"parallaxy":0.5}
		]
	}`)
	result, err := ImportTiledJSON(raw)
	require.NoError(t, err)
	assert.True(t, hasSkipped(result.Skipped, SkippedParallax))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result.TiledJSON, &parsed))
	layer := parsed["layers"].([]any)[0].(map[string]any)
	_, hasX := layer["parallaxx"]
	_, hasY := layer["parallaxy"]
	assert.False(t, hasX)
	assert.False(t, hasY)
}

// --- TDD Cycle 10: Skip — group layers flattened ---

func TestImportTiledJSON_FlattenGroupLayers(t *testing.T) {
	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[
			{"type":"tilelayer","name":"terrain","width":10,"height":10,"data":[1]},
			{"type":"group","name":"group1","layers":[
				{"type":"objectgroup","name":"walls","objects":[]},
				{"type":"tilelayer","name":"decor","width":10,"height":10,"data":[0]}
			]}
		]
	}`)
	result, err := ImportTiledJSON(raw)
	require.NoError(t, err)
	assert.True(t, hasSkipped(result.Skipped, SkippedGroupLayer))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result.TiledJSON, &parsed))
	layers := parsed["layers"].([]any)
	// terrain + walls + decor = 3, no group remaining
	assert.Len(t, layers, 3)
	for _, l := range layers {
		assert.NotEqual(t, "group", l.(map[string]any)["type"])
	}
}

// --- TDD Cycle 11: Skip — text and point objects ---

func TestImportTiledJSON_StripUnsupportedObjects(t *testing.T) {
	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[
			{"type":"tilelayer","name":"terrain","width":10,"height":10,"data":[1]},
			{"type":"objectgroup","name":"walls","objects":[
				{"id":1,"name":"a","x":0,"y":0,"width":48,"height":48,"type":"wall"},
				{"id":2,"name":"label","x":10,"y":10,"text":{"text":"hello","wrap":true}},
				{"id":3,"name":"point","x":20,"y":20,"point":true}
			]}
		]
	}`)
	result, err := ImportTiledJSON(raw)
	require.NoError(t, err)
	assert.True(t, hasSkipped(result.Skipped, SkippedTextObject))
	assert.True(t, hasSkipped(result.Skipped, SkippedPointObject))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result.TiledJSON, &parsed))
	layers := parsed["layers"].([]any)
	walls := layers[1].(map[string]any)
	objects := walls["objects"].([]any)
	assert.Len(t, objects, 1, "only the wall object should remain")
}

// --- TDD Cycle 12: Skip — wang sets ---

func TestImportTiledJSON_StripWangSets(t *testing.T) {
	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[
			{"type":"tilelayer","name":"terrain","width":10,"height":10,"data":[1]}
		],
		"tilesets":[
			{"firstgid":1,"name":"t","wangsets":[{"name":"corners","type":"corner"}]}
		]
	}`)
	result, err := ImportTiledJSON(raw)
	require.NoError(t, err)
	assert.True(t, hasSkipped(result.Skipped, SkippedWangSet))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result.TiledJSON, &parsed))
	ts := parsed["tilesets"].([]any)[0].(map[string]any)
	_, has := ts["wangsets"]
	assert.False(t, has)
}

// --- TDD Cycle 13: Multiple skipped features deduplicated ---

func TestImportTiledJSON_MultipleSkippedDeduplicated(t *testing.T) {
	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[
			{"type":"tilelayer","name":"a","width":10,"height":10,"data":[1],"parallaxx":0.5},
			{"type":"tilelayer","name":"b","width":10,"height":10,"data":[1],"parallaxy":0.5},
			{"type":"imagelayer","name":"i1","image":"a.png"},
			{"type":"imagelayer","name":"i2","image":"b.png"}
		]
	}`)
	result, err := ImportTiledJSON(raw)
	require.NoError(t, err)
	count := 0
	for _, s := range result.Skipped {
		if s.Feature == SkippedParallax || s.Feature == SkippedImageLayer {
			count++
		}
	}
	assert.Equal(t, 2, count, "each feature should appear once")
}

// --- TDD Cycle 14: Service.ImportMap wires importer + CreateMap ---

func TestService_ImportMap_Success(t *testing.T) {
	campaignID := uuid.New()
	store := successStore(campaignID)
	svc := NewService(store)

	raw := validTmj(10, 10)
	m, cat, skipped, err := svc.ImportMap(context.Background(), ImportMapInput{
		CampaignID: campaignID,
		Name:       "Imported",
		TiledJSON:  raw,
	})
	require.NoError(t, err)
	assert.Equal(t, "Imported", m.Name)
	assert.Equal(t, int32(10), m.WidthSquares)
	assert.Equal(t, int32(10), m.HeightSquares)
	assert.Equal(t, SizeCategoryStandard, cat)
	assert.Empty(t, skipped)
}

func TestService_ImportMap_HardRejection(t *testing.T) {
	campaignID := uuid.New()
	store := successStore(campaignID)
	svc := NewService(store)

	raw := json.RawMessage(`{"orientation":"isometric","width":10,"height":10,"tilewidth":48,"tileheight":48,"infinite":false,"layers":[]}`)
	_, _, _, err := svc.ImportMap(context.Background(), ImportMapInput{
		CampaignID: campaignID,
		Name:       "Iso",
		TiledJSON:  raw,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNonOrthogonal))
}

func TestService_ImportMap_PassesSkippedThrough(t *testing.T) {
	campaignID := uuid.New()
	svc := NewService(successStore(campaignID))

	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[
			{"type":"tilelayer","name":"terrain","width":10,"height":10,"data":[1]},
			{"type":"imagelayer","name":"bg","image":"bg.png"}
		]
	}`)
	_, _, skipped, err := svc.ImportMap(context.Background(), ImportMapInput{
		CampaignID: campaignID,
		Name:       "Has Image",
		TiledJSON:  raw,
	})
	require.NoError(t, err)
	assert.True(t, hasSkipped(skipped, SkippedImageLayer))
}

func TestService_ImportMap_EmptyName(t *testing.T) {
	campaignID := uuid.New()
	svc := NewService(successStore(campaignID))
	_, _, _, err := svc.ImportMap(context.Background(), ImportMapInput{
		CampaignID: campaignID,
		Name:       "",
		TiledJSON:  validTmj(10, 10),
	})
	require.Error(t, err)
}

func TestService_ImportMap_StoreError(t *testing.T) {
	campaignID := uuid.New()
	store := &mockStore{
		createMapFn: func(ctx context.Context, arg refdata.CreateMapParams) (refdata.Map, error) {
			return refdata.Map{}, errors.New("db error")
		},
	}
	svc := NewService(store)
	_, _, _, err := svc.ImportMap(context.Background(), ImportMapInput{
		CampaignID: campaignID,
		Name:       "X",
		TiledJSON:  validTmj(10, 10),
	})
	require.Error(t, err)
}

// hasSkipped checks if a feature appears in the skipped list.
func hasSkipped(skipped []SkippedFeature, feature SkippedFeatureType) bool {
	for _, s := range skipped {
		if s.Feature == feature {
			return true
		}
	}
	return false
}

// --- TDD Cycle 20: Defensive parsing — non-map/string types are tolerated ---

func TestImportTiledJSON_TolerateMalformedShapes(t *testing.T) {
	// Layers, layer entries, objects, tilesets and tiles aren't always the expected shape.
	// The importer should drop them silently rather than panicking.
	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[
			"not-a-layer",
			{"type":"objectgroup","name":"objs","objects":["not-an-object", null, {"id":1,"x":0,"y":0,"width":10,"height":10}]},
			{"type":"group","layers":["bad", {"type":"tilelayer","name":"keep","width":10,"height":10,"data":[1]}]}
		],
		"tilesets":[
			"not-a-tileset",
			{"firstgid":1,"name":"t","tiles":["not-a-tile", null, {"id":0,"type":"open_ground"}]}
		]
	}`)
	result, err := ImportTiledJSON(raw)
	require.NoError(t, err)
	// Only the well-formed objectgroup, the kept child of group, and the keep tilelayer survive.
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result.TiledJSON, &parsed))
	layers := parsed["layers"].([]any)
	// Real objectgroup + flattened tilelayer = 2
	assert.Len(t, layers, 2)
}

// --- TDD Cycle 21: orientation field absent is treated as orthogonal ---

func TestImportTiledJSON_OrientationOmitted(t *testing.T) {
	raw := json.RawMessage(`{
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[]
	}`)
	_, err := ImportTiledJSON(raw)
	require.NoError(t, err, "missing orientation defaults to orthogonal")
}
