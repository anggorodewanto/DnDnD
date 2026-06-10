package gamemap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// fakeUploader is a deterministic ImageUploader for tests. It records each
// upload and returns a stable /api/assets/{basename} URL.
type fakeUploader struct {
	calls     []fakeUploadCall
	uploadErr error
}

type fakeUploadCall struct {
	campaignID uuid.UUID
	isTileset  bool
	filename   string
	mimeType   string
	content    []byte
}

func (f *fakeUploader) UploadMapImage(_ context.Context, campaignID uuid.UUID, isTileset bool, filename, mimeType string, content io.Reader) (string, error) {
	if f.uploadErr != nil {
		return "", f.uploadErr
	}
	b, err := io.ReadAll(content)
	if err != nil {
		return "", err
	}
	f.calls = append(f.calls, fakeUploadCall{
		campaignID: campaignID,
		isTileset:  isTileset,
		filename:   filename,
		mimeType:   mimeType,
		content:    b,
	})
	return "/api/assets/" + filename, nil
}

// captureStore records the TiledJson handed to CreateMap so tests can assert on
// the rewritten image references.
func captureStore(_ uuid.UUID, captured *json.RawMessage) *mockStore {
	return &mockStore{
		createMapFn: func(_ context.Context, arg refdata.CreateMapParams) (refdata.Map, error) {
			*captured = arg.TiledJson
			return refdata.Map{
				ID:            uuid.New(),
				CampaignID:    arg.CampaignID,
				Name:          arg.Name,
				WidthSquares:  arg.WidthSquares,
				HeightSquares: arg.HeightSquares,
				TiledJson:     arg.TiledJson,
			}, nil
		},
	}
}

// tmjWithTileset returns a minimal valid map whose single tileset references an
// embedded single-image file by basename.
func tmjWithTileset(imageName string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{
		"version":"1.10","tiledversion":"1.10.0","type":"map",
		"orientation":"orthogonal","renderorder":"right-down",
		"width":10,"height":10,"tilewidth":48,"tileheight":48,
		"infinite":false,
		"layers":[{"type":"tilelayer","name":"terrain","width":10,"height":10,"data":[1]}],
		"tilesets":[{"firstgid":1,"name":"t","image":%q,"imagewidth":480,"imageheight":480,"tilewidth":48,"tileheight":48,"columns":10,"tilecount":100}]
	}`, imageName))
}

// --- ImportMap with embedded tileset image: success, rewritten to asset URL ---

func TestImportMap_TilesetImage_Success(t *testing.T) {
	campaignID := uuid.New()
	var captured json.RawMessage
	svc := NewService(captureStore(campaignID, &captured))
	up := &fakeUploader{}
	require.Same(t, svc, svc.SetImageUploader(up))

	m, _, skipped, err := svc.ImportMap(context.Background(), ImportMapInput{
		CampaignID: campaignID,
		Name:       "Tileset Map",
		TiledJSON:  tmjWithTileset("dungeon.png"),
		Images: []ImportImage{
			{Basename: "dungeon.png", MimeType: "image/png", Content: []byte("PNGDATA"), IsTileset: true},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "Tileset Map", m.Name)
	assert.Empty(t, skipped)

	// The uploader was invoked once for the tileset image.
	require.Len(t, up.calls, 1)
	assert.True(t, up.calls[0].isTileset)
	assert.Equal(t, "dungeon.png", up.calls[0].filename)
	assert.Equal(t, "image/png", up.calls[0].mimeType)
	assert.Equal(t, []byte("PNGDATA"), up.calls[0].content)

	// The stored tiled_json tileset image is rewritten to the asset URL.
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(captured, &parsed))
	tilesets := parsed["tilesets"].([]any)
	ts := tilesets[0].(map[string]any)
	assert.Equal(t, "/api/assets/dungeon.png", ts["image"])
}

// --- ImportMap reports missing images via ErrMissingImages ---

func TestImportMap_MissingImage(t *testing.T) {
	campaignID := uuid.New()
	svc := NewService(successStore(campaignID)).SetImageUploader(&fakeUploader{})

	_, _, _, err := svc.ImportMap(context.Background(), ImportMapInput{
		CampaignID: campaignID,
		Name:       "Missing",
		TiledJSON:  tmjWithTileset("dungeon.png"),
		// No images supplied.
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrMissingImages))
	assert.Contains(t, err.Error(), "dungeon.png")
}

// --- ImportMap errors when images are required but no uploader is configured ---

func TestImportMap_RequiredImages_NoUploader(t *testing.T) {
	campaignID := uuid.New()
	svc := NewService(successStore(campaignID)) // no uploader

	_, _, _, err := svc.ImportMap(context.Background(), ImportMapInput{
		CampaignID: campaignID,
		Name:       "NoUploader",
		TiledJSON:  tmjWithTileset("dungeon.png"),
		Images: []ImportImage{
			{Basename: "dungeon.png", MimeType: "image/png", Content: []byte("X"), IsTileset: true},
		},
	})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "uploader")
}

// --- ImportMap with no required images behaves as before (no uploads) ---

func TestImportMap_AbstractMap_NoUploads(t *testing.T) {
	campaignID := uuid.New()
	svc := NewService(successStore(campaignID))
	up := &fakeUploader{}
	svc.SetImageUploader(up)

	m, _, skipped, err := svc.ImportMap(context.Background(), ImportMapInput{
		CampaignID: campaignID,
		Name:       "Abstract",
		TiledJSON:  validTmj(10, 10),
	})
	require.NoError(t, err)
	assert.Equal(t, "Abstract", m.Name)
	assert.Empty(t, skipped)
	assert.Empty(t, up.calls, "abstract maps require no image uploads")
}

// --- ImportMap surfaces uploader errors ---

func TestImportMap_UploaderError(t *testing.T) {
	campaignID := uuid.New()
	svc := NewService(successStore(campaignID)).SetImageUploader(&fakeUploader{uploadErr: errors.New("boom")})

	_, _, _, err := svc.ImportMap(context.Background(), ImportMapInput{
		CampaignID: campaignID,
		Name:       "Boom",
		TiledJSON:  tmjWithTileset("dungeon.png"),
		Images: []ImportImage{
			{Basename: "dungeon.png", MimeType: "image/png", Content: []byte("X"), IsTileset: true},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}
