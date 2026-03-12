package gamemap

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// mockStore implements Store for unit tests.
type mockStore struct {
	createMapFn            func(ctx context.Context, arg refdata.CreateMapParams) (refdata.Map, error)
	getMapByIDFn           func(ctx context.Context, id uuid.UUID) (refdata.Map, error)
	listMapsByCampaignIDFn func(ctx context.Context, campaignID uuid.UUID) ([]refdata.Map, error)
	updateMapFn            func(ctx context.Context, arg refdata.UpdateMapParams) (refdata.Map, error)
	deleteMapFn            func(ctx context.Context, id uuid.UUID) error
}

func (m *mockStore) CreateMap(ctx context.Context, arg refdata.CreateMapParams) (refdata.Map, error) {
	return m.createMapFn(ctx, arg)
}
func (m *mockStore) GetMapByID(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
	return m.getMapByIDFn(ctx, id)
}
func (m *mockStore) ListMapsByCampaignID(ctx context.Context, campaignID uuid.UUID) ([]refdata.Map, error) {
	return m.listMapsByCampaignIDFn(ctx, campaignID)
}
func (m *mockStore) UpdateMap(ctx context.Context, arg refdata.UpdateMapParams) (refdata.Map, error) {
	return m.updateMapFn(ctx, arg)
}
func (m *mockStore) DeleteMap(ctx context.Context, id uuid.UUID) error {
	return m.deleteMapFn(ctx, id)
}

func minimalTiledJSON() json.RawMessage {
	return json.RawMessage(`{"version":"1.10","tiledversion":"1.10.0","type":"map","orientation":"orthogonal","width":10,"height":10,"tilewidth":48,"tileheight":48,"layers":[]}`)
}

func successStore(campaignID uuid.UUID) *mockStore {
	return &mockStore{
		createMapFn: func(ctx context.Context, arg refdata.CreateMapParams) (refdata.Map, error) {
			return refdata.Map{
				ID:            uuid.New(),
				CampaignID:    arg.CampaignID,
				Name:          arg.Name,
				WidthSquares:  arg.WidthSquares,
				HeightSquares: arg.HeightSquares,
				TiledJson:     arg.TiledJson,
				TilesetRefs:   arg.TilesetRefs,
			}, nil
		},
		updateMapFn: func(ctx context.Context, arg refdata.UpdateMapParams) (refdata.Map, error) {
			return refdata.Map{
				ID:            arg.ID,
				CampaignID:    campaignID,
				Name:          arg.Name,
				WidthSquares:  arg.WidthSquares,
				HeightSquares: arg.HeightSquares,
				TiledJson:     arg.TiledJson,
				TilesetRefs:   arg.TilesetRefs,
			}, nil
		},
	}
}

// --- TDD Cycle 1: Reject maps with dimensions > 200 (hard limit) ---

func TestCreateMap_RejectHardLimit(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		height int
	}{
		{"width exceeds", 201, 100},
		{"height exceeds", 100, 201},
		{"both exceed", 201, 201},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService(&mockStore{})
			_, _, err := svc.CreateMap(context.Background(), CreateMapInput{
				CampaignID: uuid.New(),
				Name:       "Test Map",
				Width:      tc.width,
				Height:     tc.height,
				TiledJSON:  minimalTiledJSON(),
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "exceeds hard limit")
		})
	}
}

// --- TDD Cycle 2: Reject maps with non-positive dimensions ---

func TestCreateMap_RejectNonPositiveDimensions(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		height int
	}{
		{"zero width", 0, 10},
		{"negative height", 10, -5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService(&mockStore{})
			_, _, err := svc.CreateMap(context.Background(), CreateMapInput{
				CampaignID: uuid.New(),
				Name:       "Test Map",
				Width:      tc.width,
				Height:     tc.height,
				TiledJSON:  minimalTiledJSON(),
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "must be positive")
		})
	}
}

// --- TDD Cycle 3: Standard size category for maps <= 100x100 ---

func TestCreateMap_StandardSize(t *testing.T) {
	campaignID := uuid.New()
	svc := NewService(successStore(campaignID))
	m, cat, err := svc.CreateMap(context.Background(), CreateMapInput{
		CampaignID: campaignID,
		Name:       "Standard Map",
		Width:      50,
		Height:     50,
		TiledJSON:  minimalTiledJSON(),
	})
	require.NoError(t, err)
	assert.Equal(t, SizeCategoryStandard, cat)
	assert.Equal(t, "Standard Map", m.Name)
	assert.Equal(t, int32(50), m.WidthSquares)
	assert.Equal(t, int32(50), m.HeightSquares)
}

func TestCreateMap_Standard100x100(t *testing.T) {
	campaignID := uuid.New()
	svc := NewService(successStore(campaignID))
	_, cat, err := svc.CreateMap(context.Background(), CreateMapInput{
		CampaignID: campaignID,
		Name:       "Boundary Map",
		Width:      100,
		Height:     100,
		TiledJSON:  minimalTiledJSON(),
	})
	require.NoError(t, err)
	assert.Equal(t, SizeCategoryStandard, cat)
}

// --- TDD Cycle 4: Large size category for maps 101-200 ---

func TestCreateMap_LargeWidth101(t *testing.T) {
	campaignID := uuid.New()
	svc := NewService(successStore(campaignID))
	_, cat, err := svc.CreateMap(context.Background(), CreateMapInput{
		CampaignID: campaignID,
		Name:       "Large Map",
		Width:      101,
		Height:     100,
		TiledJSON:  minimalTiledJSON(),
	})
	require.NoError(t, err)
	assert.Equal(t, SizeCategoryLarge, cat)
}

func TestCreateMap_LargeHeight101(t *testing.T) {
	campaignID := uuid.New()
	svc := NewService(successStore(campaignID))
	_, cat, err := svc.CreateMap(context.Background(), CreateMapInput{
		CampaignID: campaignID,
		Name:       "Large Map",
		Width:      100,
		Height:     101,
		TiledJSON:  minimalTiledJSON(),
	})
	require.NoError(t, err)
	assert.Equal(t, SizeCategoryLarge, cat)
}

func TestCreateMap_Large200x200(t *testing.T) {
	campaignID := uuid.New()
	svc := NewService(successStore(campaignID))
	_, cat, err := svc.CreateMap(context.Background(), CreateMapInput{
		CampaignID: campaignID,
		Name:       "Max Map",
		Width:      200,
		Height:     200,
		TiledJSON:  minimalTiledJSON(),
	})
	require.NoError(t, err)
	assert.Equal(t, SizeCategoryLarge, cat)
}

// --- TDD Cycle 5: Name validation ---

func TestCreateMap_EmptyName(t *testing.T) {
	svc := NewService(&mockStore{})
	_, _, err := svc.CreateMap(context.Background(), CreateMapInput{
		CampaignID: uuid.New(),
		Name:       "",
		Width:      10,
		Height:     10,
		TiledJSON:  minimalTiledJSON(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name must not be empty")
}

// --- TDD Cycle 6: TiledJSON validation ---

func TestCreateMap_NilTiledJSON(t *testing.T) {
	svc := NewService(&mockStore{})
	_, _, err := svc.CreateMap(context.Background(), CreateMapInput{
		CampaignID: uuid.New(),
		Name:       "Test",
		Width:      10,
		Height:     10,
		TiledJSON:  nil,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tiled_json must not be empty")
}

func TestCreateMap_EmptyTiledJSON(t *testing.T) {
	svc := NewService(&mockStore{})
	_, _, err := svc.CreateMap(context.Background(), CreateMapInput{
		CampaignID: uuid.New(),
		Name:       "Test",
		Width:      10,
		Height:     10,
		TiledJSON:  json.RawMessage{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tiled_json must not be empty")
}

// --- TDD Cycle 7: Successful creation with tileset refs ---

func TestCreateMap_WithTilesetRefs(t *testing.T) {
	campaignID := uuid.New()
	var capturedParams refdata.CreateMapParams
	store := &mockStore{
		createMapFn: func(ctx context.Context, arg refdata.CreateMapParams) (refdata.Map, error) {
			capturedParams = arg
			return refdata.Map{
				ID:           uuid.New(),
				CampaignID:   arg.CampaignID,
				Name:         arg.Name,
				WidthSquares: arg.WidthSquares,
				TilesetRefs:  arg.TilesetRefs,
			}, nil
		},
	}
	svc := NewService(store)

	refs := []TilesetRef{
		{Name: "terrain", SourceURL: "terrain.tsj", FirstGID: 1},
		{Name: "objects", SourceURL: "objects.tsj", FirstGID: 100},
	}
	_, _, err := svc.CreateMap(context.Background(), CreateMapInput{
		CampaignID:  campaignID,
		Name:        "Ref Map",
		Width:       10,
		Height:      10,
		TiledJSON:   minimalTiledJSON(),
		TilesetRefs: refs,
	})
	require.NoError(t, err)
	assert.True(t, capturedParams.TilesetRefs.Valid)

	var parsed []TilesetRef
	err = json.Unmarshal(capturedParams.TilesetRefs.RawMessage, &parsed)
	require.NoError(t, err)
	assert.Len(t, parsed, 2)
	assert.Equal(t, "terrain", parsed[0].Name)
	assert.Equal(t, 100, parsed[1].FirstGID)
}

func TestCreateMap_NilTilesetRefs(t *testing.T) {
	campaignID := uuid.New()
	var capturedParams refdata.CreateMapParams
	store := &mockStore{
		createMapFn: func(ctx context.Context, arg refdata.CreateMapParams) (refdata.Map, error) {
			capturedParams = arg
			return refdata.Map{ID: uuid.New(), CampaignID: arg.CampaignID}, nil
		},
	}
	svc := NewService(store)
	_, _, err := svc.CreateMap(context.Background(), CreateMapInput{
		CampaignID:  campaignID,
		Name:        "No Refs",
		Width:       10,
		Height:      10,
		TiledJSON:   minimalTiledJSON(),
		TilesetRefs: nil,
	})
	require.NoError(t, err)
	assert.False(t, capturedParams.TilesetRefs.Valid)
}

// --- TDD Cycle 8: Store error on create ---

func TestCreateMap_StoreError(t *testing.T) {
	store := &mockStore{
		createMapFn: func(ctx context.Context, arg refdata.CreateMapParams) (refdata.Map, error) {
			return refdata.Map{}, errors.New("db error")
		},
	}
	svc := NewService(store)
	_, _, err := svc.CreateMap(context.Background(), CreateMapInput{
		CampaignID: uuid.New(),
		Name:       "Test",
		Width:      10,
		Height:     10,
		TiledJSON:  minimalTiledJSON(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating map")
}

// --- TDD Cycle 9: GetByID ---

func TestGetByID_Success(t *testing.T) {
	id := uuid.New()
	expected := refdata.Map{ID: id, Name: "Test Map", WidthSquares: 10, HeightSquares: 10}
	store := &mockStore{
		getMapByIDFn: func(ctx context.Context, mid uuid.UUID) (refdata.Map, error) {
			assert.Equal(t, id, mid)
			return expected, nil
		},
	}
	svc := NewService(store)
	m, err := svc.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, m.ID)
	assert.Equal(t, "Test Map", m.Name)
}

func TestGetByID_NotFound(t *testing.T) {
	store := &mockStore{
		getMapByIDFn: func(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
			return refdata.Map{}, errors.New("not found")
		},
	}
	svc := NewService(store)
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.Error(t, err)
}

// --- TDD Cycle 10: ListByCampaignID ---

func TestListByCampaignID_Success(t *testing.T) {
	campaignID := uuid.New()
	expected := []refdata.Map{
		{ID: uuid.New(), CampaignID: campaignID, Name: "Map 1"},
		{ID: uuid.New(), CampaignID: campaignID, Name: "Map 2"},
	}
	store := &mockStore{
		listMapsByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Map, error) {
			assert.Equal(t, campaignID, cid)
			return expected, nil
		},
	}
	svc := NewService(store)
	maps, err := svc.ListByCampaignID(context.Background(), campaignID)
	require.NoError(t, err)
	assert.Len(t, maps, 2)
}

func TestListByCampaignID_Empty(t *testing.T) {
	store := &mockStore{
		listMapsByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Map, error) {
			return []refdata.Map{}, nil
		},
	}
	svc := NewService(store)
	maps, err := svc.ListByCampaignID(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, maps)
}

func TestListByCampaignID_StoreError(t *testing.T) {
	store := &mockStore{
		listMapsByCampaignIDFn: func(ctx context.Context, cid uuid.UUID) ([]refdata.Map, error) {
			return nil, errors.New("db error")
		},
	}
	svc := NewService(store)
	_, err := svc.ListByCampaignID(context.Background(), uuid.New())
	require.Error(t, err)
}

// --- TDD Cycle 11: UpdateMap ---

func TestUpdateMap_Success(t *testing.T) {
	campaignID := uuid.New()
	mapID := uuid.New()
	svc := NewService(successStore(campaignID))
	m, cat, err := svc.UpdateMap(context.Background(), UpdateMapInput{
		ID:        mapID,
		Name:      "Updated Map",
		Width:     50,
		Height:    50,
		TiledJSON: minimalTiledJSON(),
	})
	require.NoError(t, err)
	assert.Equal(t, SizeCategoryStandard, cat)
	assert.Equal(t, mapID, m.ID)
	assert.Equal(t, "Updated Map", m.Name)
}

func TestUpdateMap_RejectWidth201(t *testing.T) {
	svc := NewService(&mockStore{})
	_, _, err := svc.UpdateMap(context.Background(), UpdateMapInput{
		ID:        uuid.New(),
		Name:      "Test",
		Width:     201,
		Height:    100,
		TiledJSON: minimalTiledJSON(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds hard limit")
}

func TestUpdateMap_RejectZeroHeight(t *testing.T) {
	svc := NewService(&mockStore{})
	_, _, err := svc.UpdateMap(context.Background(), UpdateMapInput{
		ID:        uuid.New(),
		Name:      "Test",
		Width:     10,
		Height:    0,
		TiledJSON: minimalTiledJSON(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be positive")
}

func TestUpdateMap_EmptyName(t *testing.T) {
	svc := NewService(&mockStore{})
	_, _, err := svc.UpdateMap(context.Background(), UpdateMapInput{
		ID:        uuid.New(),
		Name:      "",
		Width:     10,
		Height:    10,
		TiledJSON: minimalTiledJSON(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name must not be empty")
}

func TestUpdateMap_EmptyTiledJSON(t *testing.T) {
	svc := NewService(&mockStore{})
	_, _, err := svc.UpdateMap(context.Background(), UpdateMapInput{
		ID:        uuid.New(),
		Name:      "Test",
		Width:     10,
		Height:    10,
		TiledJSON: nil,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tiled_json must not be empty")
}

func TestUpdateMap_LargeCategory(t *testing.T) {
	campaignID := uuid.New()
	svc := NewService(successStore(campaignID))
	_, cat, err := svc.UpdateMap(context.Background(), UpdateMapInput{
		ID:        uuid.New(),
		Name:      "Large Update",
		Width:     150,
		Height:    150,
		TiledJSON: minimalTiledJSON(),
	})
	require.NoError(t, err)
	assert.Equal(t, SizeCategoryLarge, cat)
}

func TestUpdateMap_StoreError(t *testing.T) {
	store := &mockStore{
		updateMapFn: func(ctx context.Context, arg refdata.UpdateMapParams) (refdata.Map, error) {
			return refdata.Map{}, errors.New("db error")
		},
	}
	svc := NewService(store)
	_, _, err := svc.UpdateMap(context.Background(), UpdateMapInput{
		ID:        uuid.New(),
		Name:      "Test",
		Width:     10,
		Height:    10,
		TiledJSON: minimalTiledJSON(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating map")
}

func TestUpdateMap_WithTilesetRefs(t *testing.T) {
	campaignID := uuid.New()
	var capturedParams refdata.UpdateMapParams
	store := &mockStore{
		updateMapFn: func(ctx context.Context, arg refdata.UpdateMapParams) (refdata.Map, error) {
			capturedParams = arg
			return refdata.Map{
				ID:           arg.ID,
				CampaignID:   campaignID,
				Name:         arg.Name,
				WidthSquares: arg.WidthSquares,
				TilesetRefs:  arg.TilesetRefs,
			}, nil
		},
	}
	svc := NewService(store)
	refs := []TilesetRef{{Name: "terrain", SourceURL: "terrain.tsj", FirstGID: 1}}
	_, _, err := svc.UpdateMap(context.Background(), UpdateMapInput{
		ID:          uuid.New(),
		Name:        "Updated",
		Width:       10,
		Height:      10,
		TiledJSON:   minimalTiledJSON(),
		TilesetRefs: refs,
	})
	require.NoError(t, err)
	assert.True(t, capturedParams.TilesetRefs.Valid)
}

// --- TDD Cycle 12: DeleteMap ---

func TestDeleteMap_Success(t *testing.T) {
	id := uuid.New()
	var deletedID uuid.UUID
	store := &mockStore{
		deleteMapFn: func(ctx context.Context, mid uuid.UUID) error {
			deletedID = mid
			return nil
		},
	}
	svc := NewService(store)
	err := svc.DeleteMap(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, deletedID)
}

func TestDeleteMap_StoreError(t *testing.T) {
	store := &mockStore{
		deleteMapFn: func(ctx context.Context, id uuid.UUID) error {
			return errors.New("db error")
		},
	}
	svc := NewService(store)
	err := svc.DeleteMap(context.Background(), uuid.New())
	require.Error(t, err)
}

// --- TDD Cycle 13: TileSizeForCategory ---

func TestTileSizeForCategory_Standard(t *testing.T) {
	assert.Equal(t, StandardTileSize, TileSizeForCategory(SizeCategoryStandard))
}

func TestTileSizeForCategory_Large(t *testing.T) {
	assert.Equal(t, LargeTileSize, TileSizeForCategory(SizeCategoryLarge))
}

// --- TDD Cycle 14: classifySize internal function ---

func TestClassifySize_Standard(t *testing.T) {
	assert.Equal(t, SizeCategoryStandard, classifySize(1, 1))
	assert.Equal(t, SizeCategoryStandard, classifySize(50, 50))
	assert.Equal(t, SizeCategoryStandard, classifySize(100, 100))
}

func TestClassifySize_Large(t *testing.T) {
	assert.Equal(t, SizeCategoryLarge, classifySize(101, 100))
	assert.Equal(t, SizeCategoryLarge, classifySize(100, 101))
	assert.Equal(t, SizeCategoryLarge, classifySize(200, 200))
}

// --- TDD Cycle 15: validateDimensions internal function ---

func TestValidateDimensions_Valid(t *testing.T) {
	assert.NoError(t, validateDimensions(1, 1))
	assert.NoError(t, validateDimensions(100, 100))
	assert.NoError(t, validateDimensions(200, 200))
}

func TestValidateDimensions_Invalid(t *testing.T) {
	assert.Error(t, validateDimensions(0, 10))
	assert.Error(t, validateDimensions(10, 0))
	assert.Error(t, validateDimensions(-1, 10))
	assert.Error(t, validateDimensions(10, -1))
	assert.Error(t, validateDimensions(201, 10))
	assert.Error(t, validateDimensions(10, 201))
}

// --- TDD Cycle 16: marshalTilesetRefs ---

func TestMarshalTilesetRefs_Nil(t *testing.T) {
	result, err := marshalTilesetRefs(nil)
	require.NoError(t, err)
	assert.False(t, result.Valid)
}

func TestMarshalTilesetRefs_WithRefs(t *testing.T) {
	refs := []TilesetRef{{Name: "test", SourceURL: "test.tsj", FirstGID: 1}}
	result, err := marshalTilesetRefs(refs)
	require.NoError(t, err)
	assert.True(t, result.Valid)

	var parsed []TilesetRef
	err = json.Unmarshal(result.RawMessage, &parsed)
	require.NoError(t, err)
	assert.Len(t, parsed, 1)
	assert.Equal(t, "test", parsed[0].Name)
}

func TestMarshalTilesetRefs_EmptySlice(t *testing.T) {
	refs := []TilesetRef{}
	result, err := marshalTilesetRefs(refs)
	require.NoError(t, err)
	assert.True(t, result.Valid)

	var parsed []TilesetRef
	err = json.Unmarshal(result.RawMessage, &parsed)
	require.NoError(t, err)
	assert.Empty(t, parsed)
}

// --- TDD Cycle 17: CreateMap with background image ID ---

func TestCreateMap_WithBackgroundImageID(t *testing.T) {
	campaignID := uuid.New()
	bgID := uuid.New()
	var capturedParams refdata.CreateMapParams
	store := &mockStore{
		createMapFn: func(ctx context.Context, arg refdata.CreateMapParams) (refdata.Map, error) {
			capturedParams = arg
			return refdata.Map{
				ID:                uuid.New(),
				CampaignID:        arg.CampaignID,
				Name:              arg.Name,
				BackgroundImageID: arg.BackgroundImageID,
			}, nil
		},
	}
	svc := NewService(store)
	_, _, err := svc.CreateMap(context.Background(), CreateMapInput{
		CampaignID:        campaignID,
		Name:              "BG Map",
		Width:             10,
		Height:            10,
		TiledJSON:         minimalTiledJSON(),
		BackgroundImageID: uuid.NullUUID{UUID: bgID, Valid: true},
	})
	require.NoError(t, err)
	assert.True(t, capturedParams.BackgroundImageID.Valid)
	assert.Equal(t, bgID, capturedParams.BackgroundImageID.UUID)
}

// --- TDD Cycle 18: 1x1 minimum dimension ---

func TestCreateMap_1x1(t *testing.T) {
	campaignID := uuid.New()
	svc := NewService(successStore(campaignID))
	m, cat, err := svc.CreateMap(context.Background(), CreateMapInput{
		CampaignID: campaignID,
		Name:       "Tiny Map",
		Width:      1,
		Height:     1,
		TiledJSON:  minimalTiledJSON(),
	})
	require.NoError(t, err)
	assert.Equal(t, SizeCategoryStandard, cat)
	assert.Equal(t, int32(1), m.WidthSquares)
	assert.Equal(t, int32(1), m.HeightSquares)
}

// --- TDD Cycle 19: Constants ---

func TestConstants(t *testing.T) {
	assert.Equal(t, 100, SoftLimitDimension)
	assert.Equal(t, 200, HardLimitDimension)
	assert.Equal(t, 48, StandardTileSize)
	assert.Equal(t, 32, LargeTileSize)
}

// --- TDD Cycle 20: TiledJSON round-trip with complex structure ---

func TestCreateMap_ComplexTiledJSON(t *testing.T) {
	campaignID := uuid.New()
	tiledJSON := json.RawMessage(`{
		"version":"1.10",
		"tiledversion":"1.10.0",
		"type":"map",
		"orientation":"orthogonal",
		"width":10,
		"height":10,
		"tilewidth":48,
		"tileheight":48,
		"layers":[
			{"type":"tilelayer","name":"terrain","data":[1,2,3],"width":10,"height":10},
			{"type":"objectgroup","name":"objects","objects":[
				{"type":"wall","x":0,"y":0,"width":48,"height":480,"properties":{"blocks_los":true}},
				{"type":"door","x":48,"y":96,"width":48,"height":48,"properties":{"locked":false}}
			]}
		],
		"tilesets":[
			{"firstgid":1,"source":"terrain.tsj"}
		],
		"properties":{"map_level":"underground"}
	}`)
	var capturedJSON json.RawMessage
	store := &mockStore{
		createMapFn: func(ctx context.Context, arg refdata.CreateMapParams) (refdata.Map, error) {
			capturedJSON = arg.TiledJson
			return refdata.Map{
				ID:         uuid.New(),
				CampaignID: arg.CampaignID,
				Name:       arg.Name,
				TiledJson:  arg.TiledJson,
			}, nil
		},
	}
	svc := NewService(store)
	_, _, err := svc.CreateMap(context.Background(), CreateMapInput{
		CampaignID: campaignID,
		Name:       "Complex Map",
		Width:      10,
		Height:     10,
		TiledJSON:  tiledJSON,
	})
	require.NoError(t, err)

	// Verify the JSON was passed through untouched
	var parsed map[string]interface{}
	err = json.Unmarshal(capturedJSON, &parsed)
	require.NoError(t, err)
	assert.Equal(t, "orthogonal", parsed["orientation"])
	layers, ok := parsed["layers"].([]interface{})
	require.True(t, ok)
	assert.Len(t, layers, 2)
}

// --- TDD Cycle 21: TilesetRef JSON round-trip ---

func TestTilesetRef_JSONRoundTrip(t *testing.T) {
	ref := TilesetRef{
		Name:      "dungeon_tiles",
		SourceURL: "https://example.com/dungeon.tsj",
		FirstGID:  42,
	}
	data, err := json.Marshal(ref)
	require.NoError(t, err)

	var parsed TilesetRef
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)
	assert.Equal(t, ref, parsed)
}



