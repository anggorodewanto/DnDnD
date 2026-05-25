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

// --- TDD Cycle 8: Image layers are kept and their image refs collected ---

func TestImportTiledJSON_KeepImageLayers(t *testing.T) {
	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[
			{"type":"tilelayer","name":"terrain","width":10,"height":10,"data":[1]},
			{"type":"imagelayer","name":"bg","image":"art/bg.png"}
		]
	}`)
	result, err := ImportTiledJSON(raw)
	require.NoError(t, err)
	assert.False(t, hasSkipped(result.Skipped, SkippedImageLayer), "image layers are no longer stripped")

	// Image layer is retained in the output JSON, image normalized to basename.
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result.TiledJSON, &parsed))
	layers := parsed["layers"].([]any)
	require.Len(t, layers, 2)
	imgLayer := layers[1].(map[string]any)
	assert.Equal(t, "imagelayer", imgLayer["type"])
	assert.Equal(t, "bg.png", imgLayer["image"], "image path normalized to basename")

	// The required image is reported so the importer's caller can match an upload.
	require.Len(t, result.ImageLayers, 1)
	assert.Equal(t, "bg.png", result.ImageLayers[0].Image)
	assert.Contains(t, result.RequiredImages(), "bg.png")
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
			{"type":"tilelayer","name":"b","width":10,"height":10,"data":[1],"parallaxy":0.5}
		],
		"tilesets":[
			{"firstgid":1,"name":"t","tiles":[
				{"id":0,"type":"open_ground","animation":[{"tileid":0,"duration":100}]},
				{"id":1,"type":"water","animation":[{"tileid":1,"duration":100}]}
			]}
		]
	}`)
	result, err := ImportTiledJSON(raw)
	require.NoError(t, err)
	count := 0
	for _, s := range result.Skipped {
		if s.Feature == SkippedParallax || s.Feature == SkippedTileAnimation {
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
			{"type":"tilelayer","name":"terrain","width":10,"height":10,"data":[1]}
		],
		"tilesets":[
			{"firstgid":1,"name":"t","wangsets":[{"name":"corners","type":"corner"}]}
		]
	}`)
	_, _, skipped, err := svc.ImportMap(context.Background(), ImportMapInput{
		CampaignID: campaignID,
		Name:       "Has WangSet",
		TiledJSON:  raw,
	})
	require.NoError(t, err)
	assert.True(t, hasSkipped(skipped, SkippedWangSet))
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

// --- Tileset image parsing (full-tileset import) ---

func TestImportTiledJSON_ParseEmbeddedTileset(t *testing.T) {
	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[
			{"type":"tilelayer","name":"floor","width":10,"height":10,"data":[1]}
		],
		"tilesets":[
			{"firstgid":1,"name":"dungeon","image":"tiles/dungeon.png",
			 "imagewidth":256,"imageheight":512,"tilewidth":48,"tileheight":48,
			 "columns":5,"margin":1,"spacing":2,"tilecount":50}
		]
	}`)
	result, err := ImportTiledJSON(raw)
	require.NoError(t, err)
	require.Len(t, result.Tilesets, 1)
	ts := result.Tilesets[0]
	assert.Equal(t, "dungeon", ts.Name)
	assert.Equal(t, 1, ts.FirstGID)
	assert.Equal(t, "dungeon.png", ts.Image, "image normalized to basename")
	assert.Equal(t, 256, ts.ImageWidth)
	assert.Equal(t, 512, ts.ImageHeight)
	assert.Equal(t, 48, ts.TileWidth)
	assert.Equal(t, 48, ts.TileHeight)
	assert.Equal(t, 5, ts.Columns)
	assert.Equal(t, 1, ts.Margin)
	assert.Equal(t, 2, ts.Spacing)
	assert.Equal(t, 50, ts.TileCount)
	assert.Contains(t, result.RequiredImages(), "dungeon.png")

	// The basename is written back into the stored JSON.
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result.TiledJSON, &parsed))
	stored := parsed["tilesets"].([]any)[0].(map[string]any)
	assert.Equal(t, "dungeon.png", stored["image"])
}

func TestImportTiledJSON_RejectExternalTileset(t *testing.T) {
	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[{"type":"tilelayer","name":"floor","width":10,"height":10,"data":[1]}],
		"tilesets":[{"firstgid":1,"source":"dungeon.tsx"}]
	}`)
	_, err := ImportTiledJSON(raw)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrExternalTileset), "expected ErrExternalTileset, got %v", err)
}

func TestImportTiledJSON_RejectImageCollectionTileset(t *testing.T) {
	// A "collection of images" tileset has no top-level image; each tile carries its own.
	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[{"type":"tilelayer","name":"floor","width":10,"height":10,"data":[1]}],
		"tilesets":[{"firstgid":1,"name":"props","tiles":[{"id":0,"image":"chair.png"}]}]
	}`)
	_, err := ImportTiledJSON(raw)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrImageCollectionTileset), "expected ErrImageCollectionTileset, got %v", err)
}

func TestImportTiledJSON_AbstractTilesetKept(t *testing.T) {
	// The semantic terrain tileset (tiles carry a type but no image) must still
	// import cleanly and produce no image requirement.
	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[{"type":"tilelayer","name":"terrain","width":10,"height":10,"data":[1]}],
		"tilesets":[{"firstgid":1,"name":"semantic","tiles":[
			{"id":0,"type":"open_ground"},
			{"id":1,"type":"water"}
		]}]
	}`)
	result, err := ImportTiledJSON(raw)
	require.NoError(t, err)
	assert.Empty(t, result.Tilesets, "abstract tileset needs no image")
	assert.Empty(t, result.RequiredImages())
}

func TestApplyImageAssets_RewritesTilesetAndImageLayerImages(t *testing.T) {
	raw := json.RawMessage(`{
		"orientation":"orthogonal",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[
			{"type":"tilelayer","name":"floor","width":10,"height":10,"data":[1]},
			{"type":"imagelayer","name":"bg","image":"backdrop.png"}
		],
		"tilesets":[
			{"firstgid":1,"name":"dungeon","image":"dungeon.png","imagewidth":48,"imageheight":48,"tilewidth":48,"tileheight":48,"columns":1}
		]
	}`)
	result, err := ImportTiledJSON(raw)
	require.NoError(t, err)

	rewritten, err := ApplyImageAssets(result.TiledJSON, map[string]string{
		"dungeon.png":  "/api/assets/aaaa",
		"backdrop.png": "/api/assets/bbbb",
	})
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(rewritten, &parsed))
	assert.Equal(t, "/api/assets/aaaa", parsed["tilesets"].([]any)[0].(map[string]any)["image"])
	assert.Equal(t, "/api/assets/bbbb", parsed["layers"].([]any)[1].(map[string]any)["image"])
}

// --- Service.ReimportMap wires importer + UpdateMap in place ---

func TestService_ReimportMap_Success(t *testing.T) {
	campaignID := uuid.New()
	mapID := uuid.New()
	bgID := uuid.New()
	var captured refdata.UpdateMapParams
	store := &mockStore{
		getMapByIDFn: func(_ context.Context, arg refdata.GetMapByIDParams) (refdata.Map, error) {
			return refdata.Map{
				ID:                arg.ID,
				CampaignID:        arg.CampaignID,
				Name:              "Old Name",
				WidthSquares:      10,
				HeightSquares:     10,
				TiledJson:         minimalTiledJSON(),
				BackgroundImageID: uuid.NullUUID{UUID: bgID, Valid: true},
			}, nil
		},
		updateMapFn: func(_ context.Context, arg refdata.UpdateMapParams) (refdata.Map, error) {
			captured = arg
			return refdata.Map{
				ID:                arg.ID,
				CampaignID:        arg.CampaignID,
				Name:              arg.Name,
				WidthSquares:      arg.WidthSquares,
				HeightSquares:     arg.HeightSquares,
				TiledJson:         arg.TiledJson,
				BackgroundImageID: arg.BackgroundImageID,
			}, nil
		},
	}
	svc := NewService(store)

	m, cat, skipped, err := svc.ReimportMap(context.Background(), ReimportMapInput{
		ID:         mapID,
		CampaignID: campaignID,
		Name:       "New Name",
		TiledJSON:  validTmj(20, 18),
	})
	require.NoError(t, err)
	// Same ID -> encounters stay linked.
	assert.Equal(t, mapID, captured.ID)
	assert.Equal(t, mapID, m.ID)
	// New dimensions taken from the reimported tmj.
	assert.Equal(t, int32(20), captured.WidthSquares)
	assert.Equal(t, int32(18), captured.HeightSquares)
	assert.Equal(t, int32(20), m.WidthSquares)
	assert.Equal(t, int32(18), m.HeightSquares)
	assert.Equal(t, SizeCategoryStandard, cat)
	// Background image preserved from the existing map.
	assert.True(t, captured.BackgroundImageID.Valid)
	assert.Equal(t, bgID, captured.BackgroundImageID.UUID)
	// Provided name overrides the old one.
	assert.Equal(t, "New Name", captured.Name)
	assert.Equal(t, "New Name", m.Name)
	assert.Empty(t, skipped)
}

func TestService_ReimportMap_PreservesNameWhenEmpty(t *testing.T) {
	campaignID := uuid.New()
	var captured refdata.UpdateMapParams
	store := &mockStore{
		getMapByIDFn: func(_ context.Context, arg refdata.GetMapByIDParams) (refdata.Map, error) {
			return refdata.Map{
				ID:            arg.ID,
				CampaignID:    arg.CampaignID,
				Name:          "Existing Name",
				WidthSquares:  10,
				HeightSquares: 10,
				TiledJson:     minimalTiledJSON(),
			}, nil
		},
		updateMapFn: func(_ context.Context, arg refdata.UpdateMapParams) (refdata.Map, error) {
			captured = arg
			return refdata.Map{ID: arg.ID, CampaignID: arg.CampaignID, Name: arg.Name}, nil
		},
	}
	svc := NewService(store)

	_, _, _, err := svc.ReimportMap(context.Background(), ReimportMapInput{
		ID:         uuid.New(),
		CampaignID: campaignID,
		Name:       "",
		TiledJSON:  validTmj(10, 10),
	})
	require.NoError(t, err)
	assert.Equal(t, "Existing Name", captured.Name, "empty name preserves the existing map's name")
}

func TestService_ReimportMap_PassesSkippedThrough(t *testing.T) {
	campaignID := uuid.New()
	store := &mockStore{
		getMapByIDFn: func(_ context.Context, arg refdata.GetMapByIDParams) (refdata.Map, error) {
			return refdata.Map{ID: arg.ID, CampaignID: arg.CampaignID, Name: "Old", WidthSquares: 10, HeightSquares: 10, TiledJson: minimalTiledJSON()}, nil
		},
		updateMapFn: func(_ context.Context, arg refdata.UpdateMapParams) (refdata.Map, error) {
			return refdata.Map{ID: arg.ID, CampaignID: arg.CampaignID, Name: arg.Name, WidthSquares: arg.WidthSquares, HeightSquares: arg.HeightSquares, TiledJson: arg.TiledJson}, nil
		},
	}
	svc := NewService(store)

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
	_, _, skipped, err := svc.ReimportMap(context.Background(), ReimportMapInput{
		ID:         uuid.New(),
		CampaignID: campaignID,
		TiledJSON:  raw,
	})
	require.NoError(t, err)
	assert.True(t, hasSkipped(skipped, SkippedWangSet))
}

func TestService_ReimportMap_GetByIDError(t *testing.T) {
	updateCalled := false
	store := &mockStore{
		getMapByIDFn: func(_ context.Context, _ refdata.GetMapByIDParams) (refdata.Map, error) {
			return refdata.Map{}, errors.New("not found")
		},
		updateMapFn: func(_ context.Context, arg refdata.UpdateMapParams) (refdata.Map, error) {
			updateCalled = true
			return refdata.Map{ID: arg.ID}, nil
		},
	}
	svc := NewService(store)

	_, _, _, err := svc.ReimportMap(context.Background(), ReimportMapInput{
		ID:         uuid.New(),
		CampaignID: uuid.New(),
		TiledJSON:  validTmj(10, 10),
	})
	require.Error(t, err)
	assert.False(t, updateCalled, "UpdateMap must not run when the map can't be fetched")
}

func TestService_ReimportMap_HardRejection(t *testing.T) {
	getCalled := false
	updateCalled := false
	store := &mockStore{
		getMapByIDFn: func(_ context.Context, arg refdata.GetMapByIDParams) (refdata.Map, error) {
			getCalled = true
			return refdata.Map{ID: arg.ID, CampaignID: arg.CampaignID, Name: "Old", WidthSquares: 10, HeightSquares: 10, TiledJson: minimalTiledJSON()}, nil
		},
		updateMapFn: func(_ context.Context, arg refdata.UpdateMapParams) (refdata.Map, error) {
			updateCalled = true
			return refdata.Map{ID: arg.ID}, nil
		},
	}
	svc := NewService(store)

	raw := json.RawMessage(`{"orientation":"orthogonal","width":10,"height":10,"tilewidth":48,"tileheight":48,"infinite":true,"layers":[]}`)
	_, _, _, err := svc.ReimportMap(context.Background(), ReimportMapInput{
		ID:         uuid.New(),
		CampaignID: uuid.New(),
		TiledJSON:  raw,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInfiniteMap))
	assert.True(t, getCalled, "the existing map is fetched before parsing")
	assert.False(t, updateCalled, "no update on a hard-rejected payload")
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
