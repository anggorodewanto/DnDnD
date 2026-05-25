package renderer

import (
	"bytes"
	"image"
	_ "image/jpeg"
	"image/png"

	"github.com/fogleman/gg"
)

// Tiled GID flip flags packed into the high bits of each sprite-layer cell.
const (
	flipHorizontal uint32 = 0x80000000
	flipVertical   uint32 = 0x40000000
	flipDiagonal   uint32 = 0x20000000
	gidMask        uint32 = 0x1FFFFFFF
)

// subImager is implemented by the concrete image types (NRGBA/RGBA/...) that
// image.Decode returns, letting us extract a tile-sized sub-image cheaply.
type subImager interface {
	SubImage(r image.Rectangle) image.Image
}

// decodeImage decodes raw encoded image bytes (PNG/JPEG/...) into an
// image.Image. Returns nil when the bytes are empty or undecodable.
func decodeImage(raw []byte) image.Image {
	if len(raw) == 0 {
		return nil
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err == nil {
		return img
	}
	img, _, err = image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	return img
}

// DrawSpriteLayers blits each sprite layer's tiles onto the drawing context,
// resolving GIDs against md.Tilesets. Layers are drawn in order (bottom to
// top). Tileset images are decoded once and cached for the duration of the
// call. No-op when there are no sprite layers.
func DrawSpriteLayers(dc *gg.Context, md *MapData) {
	if len(md.SpriteLayers) == 0 || len(md.Tilesets) == 0 {
		return
	}

	// Decode each tileset image once, keyed by FirstGID. Abstract tilesets
	// (nil/undecodable ImagePNG) are left out of the cache.
	cache := make(map[int]image.Image, len(md.Tilesets))
	for i := range md.Tilesets {
		ts := &md.Tilesets[i]
		if img := decodeImage(ts.ImagePNG); img != nil {
			cache[ts.FirstGID] = img
		}
	}
	if len(cache) == 0 {
		return
	}

	dest := float64(md.TileSize)
	for _, layer := range md.SpriteLayers {
		for i, raw := range layer.Data {
			id := raw & gidMask
			if id == 0 {
				continue
			}
			ts := resolveTileset(md.Tilesets, cache, int(id))
			if ts == nil {
				continue
			}
			img := cache[ts.FirstGID]
			drawSprite(dc, img, ts, int(id), i, md.Width, raw, dest)
		}
	}
}

// resolveTileset returns the tileset with the greatest FirstGID that is <= id
// and has a decoded image (present in cache). Returns nil when none match.
func resolveTileset(tilesets []RenderTileset, cache map[int]image.Image, id int) *RenderTileset {
	var best *RenderTileset
	for i := range tilesets {
		ts := &tilesets[i]
		if ts.FirstGID > id {
			continue
		}
		if _, ok := cache[ts.FirstGID]; !ok {
			continue
		}
		if best == nil || ts.FirstGID > best.FirstGID {
			best = ts
		}
	}
	return best
}

// drawSprite extracts the source tile for id from the tileset image and draws
// it scaled into the cell at layer index i, applying Tiled flip flags.
func drawSprite(dc *gg.Context, img image.Image, ts *RenderTileset, id, i, width int, raw uint32, dest float64) {
	si, ok := img.(subImager)
	if !ok {
		return
	}
	if ts.Columns <= 0 {
		return
	}

	localID := id - ts.FirstGID
	srcCol := localID % ts.Columns
	srcRow := localID / ts.Columns
	b := img.Bounds()
	sx := b.Min.X + ts.Margin + srcCol*(ts.TileWidth+ts.Spacing)
	sy := b.Min.Y + ts.Margin + srcRow*(ts.TileHeight+ts.Spacing)
	tile := si.SubImage(image.Rect(sx, sy, sx+ts.TileWidth, sy+ts.TileHeight))

	col := i % width
	row := i / width
	dx := float64(col) * dest
	dy := float64(row) * dest

	dc.Push()
	// Move to the cell origin, then apply flip transforms about the cell.
	dc.Translate(dx, dy)
	applyFlips(dc, raw, dest)

	// Scale the source tile up/down to the destination cell size, then draw
	// it at the (now transformed) origin.
	if ts.TileWidth > 0 && ts.TileHeight > 0 {
		dc.Scale(dest/float64(ts.TileWidth), dest/float64(ts.TileHeight))
	}
	// DrawImage preserves the sub-image's source position, so shift by the
	// tile's source origin to land it at the (transformed) cell origin.
	dc.DrawImage(tile, -sx, -sy)
	dc.Pop()
}

// applyFlips applies Tiled flip transforms within a context already translated
// to the cell origin. size is the destination cell size in pixels. The
// transforms keep the tile inside the same cell bounds.
func applyFlips(dc *gg.Context, raw uint32, size float64) {
	h := raw&flipHorizontal != 0
	v := raw&flipVertical != 0
	d := raw&flipDiagonal != 0

	// Tiled applies the flips to the source tile in this order: diagonal
	// (transpose) first, then horizontal, then vertical. gg transforms
	// post-multiply, so the call made LAST is applied to source coordinates
	// FIRST. We therefore emit them in reverse: vertical, horizontal,
	// diagonal — so the diagonal transpose acts on the raw source tile.
	if v {
		dc.Translate(0, size)
		dc.Scale(1, -1)
	}
	if h {
		dc.Translate(size, 0)
		dc.Scale(-1, 1)
	}
	// Diagonal flip is a transpose: reflect across the main diagonal so a
	// source pixel (x,y) is drawn at (y,x). Implemented as a 90° rotation
	// plus a vertical reflection, both kept within the cell bounds.
	if d {
		dc.Translate(size, 0)
		dc.Rotate(gg.Radians(90))
		dc.Translate(0, size)
		dc.Scale(1, -1)
	}
}

// DrawImageLayers composites each image layer onto the drawing context at its
// pixel offset, scaled from design-tile pixels to render-tile pixels. No-op
// when there are no image layers.
func DrawImageLayers(dc *gg.Context, md *MapData, scale float64) {
	for _, layer := range md.ImageLayers {
		img := decodeImage(layer.ImagePNG)
		if img == nil {
			continue
		}
		dc.Push()
		dc.Translate(float64(layer.OffsetX)*scale, float64(layer.OffsetY)*scale)
		dc.Scale(scale, scale)
		dc.DrawImage(img, 0, 0)
		dc.Pop()
	}
}
