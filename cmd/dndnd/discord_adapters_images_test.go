package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/refdata"
)

// fakeImageFetcher serves canned bytes per asset id and records lookups.
type fakeImageFetcher struct {
	bytesByID map[uuid.UUID][]byte
	err       error
	calls     []uuid.UUID
}

func (f *fakeImageFetcher) OpenFile(_ context.Context, id uuid.UUID) (refdata.Asset, io.ReadCloser, error) {
	f.calls = append(f.calls, id)
	if f.err != nil {
		return refdata.Asset{}, nil, f.err
	}
	b, ok := f.bytesByID[id]
	if !ok {
		return refdata.Asset{}, nil, errors.New("not found")
	}
	return refdata.Asset{ID: id}, io.NopCloser(bytes.NewReader(b)), nil
}

func TestAssetIDFromRef(t *testing.T) {
	id := uuid.New()
	assert.Equal(t, id, assetIDFromRef("/api/assets/"+id.String()))
	assert.Equal(t, uuid.Nil, assetIDFromRef(""))
	assert.Equal(t, uuid.Nil, assetIDFromRef("/api/assets/not-a-uuid"))
}

func TestResolveMapImages_FillsBytesFromRefsAndBackground(t *testing.T) {
	tilesetID := uuid.New()
	imgLayerID := uuid.New()
	bgID := uuid.New()
	fetcher := &fakeImageFetcher{bytesByID: map[uuid.UUID][]byte{
		tilesetID:  []byte("TILESET"),
		imgLayerID: []byte("IMGLAYER"),
		bgID:       []byte("BG"),
	}}
	a := &mapRegeneratorAdapter{}
	a.withImageFetcher(fetcher)

	md := &renderer.MapData{
		Tilesets:    []renderer.RenderTileset{{ImageRef: "/api/assets/" + tilesetID.String()}},
		ImageLayers: []renderer.RenderImageLayer{{ImageRef: "/api/assets/" + imgLayerID.String()}},
	}
	a.resolveMapImages(context.Background(), md, uuid.NullUUID{UUID: bgID, Valid: true})

	assert.Equal(t, []byte("TILESET"), md.Tilesets[0].ImagePNG)
	assert.Equal(t, []byte("IMGLAYER"), md.ImageLayers[0].ImagePNG)
	assert.Equal(t, []byte("BG"), md.BackgroundImage)
	assert.Equal(t, 1.0, md.BackgroundOpacity, "background defaults to full opacity")
}

func TestResolveMapImages_NoFetcherIsNoop(t *testing.T) {
	a := &mapRegeneratorAdapter{}
	md := &renderer.MapData{Tilesets: []renderer.RenderTileset{{ImageRef: "/api/assets/" + uuid.New().String()}}}
	a.resolveMapImages(context.Background(), md, uuid.NullUUID{})
	assert.Nil(t, md.Tilesets[0].ImagePNG)
}

func TestResolveMapImages_MissingAssetLeavesNilNotFatal(t *testing.T) {
	fetcher := &fakeImageFetcher{err: errors.New("store down")}
	a := (&mapRegeneratorAdapter{}).withImageFetcher(fetcher)
	md := &renderer.MapData{Tilesets: []renderer.RenderTileset{{ImageRef: "/api/assets/" + uuid.New().String()}}}
	require.NotPanics(t, func() {
		a.resolveMapImages(context.Background(), md, uuid.NullUUID{})
	})
	assert.Nil(t, md.Tilesets[0].ImagePNG)
}

// TestCombatantsToRendererForm_PropagatesIsVisible guards the player-view fog
// filter: filterCombatantsForFog drops any non-player token whose IsVisible is
// false BEFORE it checks line-of-sight, so the conversion must carry the
// combatant's is_visible flag through. Without it every enemy renders as
// "hidden" and never appears on the player-facing #combat-map even when a PC
// can plainly see it.
func TestCombatantsToRendererForm_PropagatesIsVisible(t *testing.T) {
	in := []refdata.Combatant{
		{ShortID: "G1", DisplayName: "Ghoul", PositionCol: "C", PositionRow: 7, IsNpc: true, IsVisible: true, HpMax: 22, HpCurrent: 22},
		{ShortID: "HID", DisplayName: "Lurker", PositionCol: "D", PositionRow: 8, IsNpc: true, IsVisible: false, HpMax: 10, HpCurrent: 10},
		{ShortID: "VA", DisplayName: "Vale", PositionCol: "K", PositionRow: 6, IsNpc: false, IsVisible: true, HpMax: 24, HpCurrent: 24},
	}

	out := combatantsToRendererForm(in)
	require.Len(t, out, 3)

	byID := map[string]renderer.Combatant{}
	for _, c := range out {
		byID[c.ShortID] = c
	}
	assert.True(t, byID["G1"].IsVisible, "a visible enemy must carry IsVisible=true so fog can show it when seen")
	assert.False(t, byID["HID"].IsVisible, "a hidden enemy must stay IsVisible=false")
	assert.True(t, byID["VA"].IsVisible, "a visible PC must carry IsVisible=true")
}
