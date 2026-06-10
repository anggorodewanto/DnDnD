package renderer

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/fogleman/gg"
)

// makeTilesheetPNG builds an in-memory tilesheet of `cols` tiles across × 1
// row, each tile `tilePx`×`tilePx`, filled with the supplied colors (one per
// tile, left to right). Margin and spacing default to 0. Returns the encoded
// PNG bytes.
func makeTilesheetPNG(t *testing.T, tilePx, margin, spacing int, colors []color.RGBA) []byte {
	t.Helper()
	cols := len(colors)
	w := margin*2 + cols*tilePx + (cols-1)*spacing
	h := margin*2 + tilePx
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i, c := range colors {
		ox := margin + i*(tilePx+spacing)
		oy := margin
		for y := range tilePx {
			for x := range tilePx {
				img.SetNRGBA(ox+x, oy+y, color.NRGBA{R: c.R, G: c.G, B: c.B, A: 0xFF})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode tilesheet: %v", err)
	}
	return buf.Bytes()
}

// rgb8 reads the pixel at (x,y) and returns 8-bit R,G,B.
func rgb8(img image.Image, x, y int) (uint8, uint8, uint8) {
	r, g, b, _ := img.At(x, y).RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

// cellCenter returns the output-PNG pixel coordinates of the center of cell
// (col,row) given the map-area offset (gridLabelMargin) and tile size.
func cellCenter(col, row, ts int) (int, int) {
	return gridLabelMargin + col*ts + ts/2, gridLabelMargin + row*ts + ts/2
}

var (
	spriteRed  = color.RGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF}
	spriteBlue = color.RGBA{R: 0x00, G: 0x00, B: 0xFF, A: 0xFF}
)

// makeQuadrantTilePNG builds a single tilePx×tilePx tile whose four quadrants
// are distinct colors (top-left, top-right, bottom-left, bottom-right). This
// lets flip transforms be checked by which quadrant ends up where.
func makeQuadrantTilePNG(t *testing.T, tilePx int, tl, tr, bl, br color.RGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, tilePx, tilePx))
	half := tilePx / 2
	for y := range tilePx {
		for x := range tilePx {
			c := tl
			if x >= half && y < half {
				c = tr
			} else if x < half && y >= half {
				c = bl
			} else if x >= half && y >= half {
				c = br
			}
			img.SetNRGBA(x, y, color.NRGBA{R: c.R, G: c.G, B: c.B, A: 0xFF})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode quadrant tile: %v", err)
	}
	return buf.Bytes()
}

// renderQuadrantFlip renders a single quadrant tile with the given flip flags
// and returns the four sampled quadrant colors (tl, tr, bl, br) of the cell.
func renderQuadrantFlip(t *testing.T, flags uint32) (tl, tr, bl, br [3]uint8) {
	t.Helper()
	ts := 16
	red := color.RGBA{R: 0xFF}
	green := color.RGBA{G: 0xFF}
	blue := color.RGBA{B: 0xFF}
	yellow := color.RGBA{R: 0xFF, G: 0xFF}
	sheet := makeQuadrantTilePNG(t, 8, red, green, blue, yellow)
	md := &MapData{
		Width:    1,
		Height:   1,
		TileSize: ts,
		Tilesets: []RenderTileset{{
			FirstGID:   1,
			Columns:    1,
			TileWidth:  8,
			TileHeight: 8,
			ImagePNG:   sheet,
		}},
		SpriteLayers: []SpriteLayer{{Name: "art", Data: []uint32{flags | 1}}},
	}
	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	q := ts / 4
	x0, y0 := gridLabelMargin, gridLabelMargin
	read := func(cx, cy int) [3]uint8 {
		r, g, b := rgb8(img, cx, cy)
		return [3]uint8{r, g, b}
	}
	tl = read(x0+q, y0+q)
	tr = read(x0+3*q, y0+q)
	bl = read(x0+q, y0+3*q)
	br = read(x0+3*q, y0+3*q)
	return tl, tr, bl, br
}

func dominant(p [3]uint8) string {
	r, g, b := p[0], p[1], p[2]
	if r > 180 && g > 180 {
		return "yellow"
	}
	if r > 180 {
		return "red"
	}
	if g > 180 {
		return "green"
	}
	if b > 180 {
		return "blue"
	}
	return "other"
}

func TestDrawSpriteLayers_FlipDirections(t *testing.T) {
	const (
		flipH = 0x80000000
		flipV = 0x40000000
		flipD = 0x20000000
	)
	// Source quadrants: TL=red TR=green BL=blue BR=yellow.
	tests := []struct {
		name           string
		flags          uint32
		tl, tr, bl, br string
	}{
		{"none", 0, "red", "green", "blue", "yellow"},
		{"horizontal", flipH, "green", "red", "yellow", "blue"},
		{"vertical", flipV, "blue", "yellow", "red", "green"},
		// Diagonal flip transposes across the main diagonal: TR<->BL swap.
		{"diagonal", flipD, "red", "blue", "green", "yellow"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tl, tr, bl, br := renderQuadrantFlip(t, tc.flags)
			if dominant(tl) != tc.tl || dominant(tr) != tc.tr || dominant(bl) != tc.bl || dominant(br) != tc.br {
				t.Errorf("%s flip: got TL=%s TR=%s BL=%s BR=%s; want TL=%s TR=%s BL=%s BR=%s",
					tc.name, dominant(tl), dominant(tr), dominant(bl), dominant(br),
					tc.tl, tc.tr, tc.bl, tc.br)
			}
		})
	}
}

func TestDrawSpriteLayers_BasicPlacement(t *testing.T) {
	ts := 8
	sheet := makeTilesheetPNG(t, 4, 0, 0, []color.RGBA{spriteRed, spriteBlue})
	md := &MapData{
		Width:    2,
		Height:   1,
		TileSize: ts,
		Tilesets: []RenderTileset{{
			FirstGID:   1,
			Columns:    2,
			TileWidth:  4,
			TileHeight: 4,
			ImagePNG:   sheet,
		}},
		SpriteLayers: []SpriteLayer{{
			Name: "art",
			Data: []uint32{1, 2}, // cell0 -> red tile, cell1 -> blue tile
		}},
	}

	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}

	x0, y0 := cellCenter(0, 0, ts)
	r, g, b := rgb8(img, x0, y0)
	if r < 200 || g > 80 || b > 80 {
		t.Errorf("cell0 center (%d,%d): want ~red, got (%d,%d,%d)", x0, y0, r, g, b)
	}

	x1, y1 := cellCenter(1, 0, ts)
	r, g, b = rgb8(img, x1, y1)
	if b < 200 || r > 80 || g > 80 {
		t.Errorf("cell1 center (%d,%d): want ~blue, got (%d,%d,%d)", x1, y1, r, g, b)
	}
}

func TestDrawSpriteLayers_GIDZeroLeavesTerrain(t *testing.T) {
	ts := 8
	sheet := makeTilesheetPNG(t, 4, 0, 0, []color.RGBA{spriteRed, spriteBlue})
	md := &MapData{
		Width:    2,
		Height:   1,
		TileSize: ts,
		Tilesets: []RenderTileset{{
			FirstGID:   1,
			Columns:    2,
			TileWidth:  4,
			TileHeight: 4,
			ImagePNG:   sheet,
		}},
		SpriteLayers: []SpriteLayer{{
			Name: "art",
			Data: []uint32{0, 2}, // cell0 empty, cell1 blue
		}},
	}

	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}

	// cell0 has no sprite: should show terrain open-ground tint over white
	// background (i.e. not red/blue).
	x0, y0 := cellCenter(0, 0, ts)
	r, g, b := rgb8(img, x0, y0)
	if r > 200 && g < 80 && b < 80 {
		t.Errorf("cell0 should be empty (no red sprite), got (%d,%d,%d)", r, g, b)
	}
	if b > 200 && r < 80 && g < 80 {
		t.Errorf("cell0 should be empty (no blue sprite), got (%d,%d,%d)", r, g, b)
	}

	// cell1 still blue.
	x1, y1 := cellCenter(1, 0, ts)
	_, _, b = rgb8(img, x1, y1)
	if b < 200 {
		t.Errorf("cell1 should be blue, got blue=%d", b)
	}
}

func TestDrawSpriteLayers_FlippedTileRenders(t *testing.T) {
	const flipH = 0x80000000
	ts := 8
	sheet := makeTilesheetPNG(t, 4, 0, 0, []color.RGBA{spriteRed, spriteBlue})
	md := &MapData{
		Width:    1,
		Height:   1,
		TileSize: ts,
		Tilesets: []RenderTileset{{
			FirstGID:   1,
			Columns:    2,
			TileWidth:  4,
			TileHeight: 4,
			ImagePNG:   sheet,
		}},
		SpriteLayers: []SpriteLayer{{
			Name: "art",
			Data: []uint32{flipH | 1}, // horizontally flipped red tile
		}},
	}

	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}

	// A flipped solid-red tile still fills the cell with red.
	x0, y0 := cellCenter(0, 0, ts)
	r, g, b := rgb8(img, x0, y0)
	if r < 200 || g > 80 || b > 80 {
		t.Errorf("flipped cell center (%d,%d): want ~red, got (%d,%d,%d)", x0, y0, r, g, b)
	}
}

func TestDrawSpriteLayers_CombinedFlip(t *testing.T) {
	const (
		flipH = 0x80000000
		flipD = 0x20000000
	)
	// Diagonal + horizontal together is a 90° clockwise rotation in Tiled.
	// Source TL=red TR=green BL=blue BR=yellow -> rotate 90° CW:
	// TL=blue, TR=red, BL=yellow, BR=green.
	tl, tr, bl, br := renderQuadrantFlip(t, flipD|flipH)
	if dominant(tl) != "blue" || dominant(tr) != "red" || dominant(bl) != "yellow" || dominant(br) != "green" {
		t.Errorf("D|H (90° CW): got TL=%s TR=%s BL=%s BR=%s; want TL=blue TR=red BL=yellow BR=green",
			dominant(tl), dominant(tr), dominant(bl), dominant(br))
	}
}

func TestDrawSpriteLayers_MultipleTilesetsResolveByFirstGID(t *testing.T) {
	ts := 8
	sheetA := makeTilesheetPNG(t, 4, 0, 0, []color.RGBA{spriteRed})  // FirstGID 1 -> gid 1
	sheetB := makeTilesheetPNG(t, 4, 0, 0, []color.RGBA{spriteBlue}) // FirstGID 2 -> gid 2
	md := &MapData{
		Width:    2,
		Height:   1,
		TileSize: ts,
		Tilesets: []RenderTileset{
			{FirstGID: 1, Columns: 1, TileWidth: 4, TileHeight: 4, ImagePNG: sheetA},
			{FirstGID: 2, Columns: 1, TileWidth: 4, TileHeight: 4, ImagePNG: sheetB},
		},
		SpriteLayers: []SpriteLayer{{Name: "art", Data: []uint32{1, 2}}},
	}
	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	x0, y0 := cellCenter(0, 0, ts)
	if r, _, _ := rgb8(img, x0, y0); r < 200 {
		t.Errorf("gid 1 should resolve to tileset A (red), got red=%d", r)
	}
	x1, y1 := cellCenter(1, 0, ts)
	if _, _, b := rgb8(img, x1, y1); b < 200 {
		t.Errorf("gid 2 should resolve to tileset B (blue), got blue=%d", b)
	}
}

func TestDrawSpriteLayers_AbstractTilesetNoSprite(t *testing.T) {
	ts := 8
	md := &MapData{
		Width:    1,
		Height:   1,
		TileSize: ts,
		Tilesets: []RenderTileset{{
			FirstGID: 1,
			Columns:  2,
			ImagePNG: nil, // abstract: contributes no sprite
		}},
		SpriteLayers: []SpriteLayer{{
			Name: "art",
			Data: []uint32{1},
		}},
	}

	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}

	// No sprite drawn — cell shows terrain/background, not red/blue.
	x0, y0 := cellCenter(0, 0, ts)
	r, g, b := rgb8(img, x0, y0)
	if r > 200 && g < 80 && b < 80 {
		t.Errorf("abstract tileset should draw no sprite, got red (%d,%d,%d)", r, g, b)
	}
}

func TestDrawImageLayers_DrawsAtScaledOffset(t *testing.T) {
	ts := 16
	// A 4x4 solid-blue image placed at offset (4,0) in design-tile pixels.
	// origTileSize == TileSize here so scale == 1.
	imgPNG := makeTilesheetPNG(t, 4, 0, 0, []color.RGBA{spriteBlue})
	md := &MapData{
		Width:    1,
		Height:   1,
		TileSize: ts,
		ImageLayers: []RenderImageLayer{{
			Name:     "overlay",
			ImagePNG: imgPNG,
			OffsetX:  4,
			OffsetY:  4,
		}},
	}

	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}

	// The 4x4 blue block sits at map-pixel (4,4)..(8,8); add the margin.
	px, py := gridLabelMargin+5, gridLabelMargin+5
	_, _, b := rgb8(img, px, py)
	if b < 200 {
		t.Errorf("image layer should be blue at (%d,%d), got blue=%d", px, py, b)
	}
}

func TestDrawTerrain_TranslucentWhenSpriteArt(t *testing.T) {
	ts := 8
	sheet := makeTilesheetPNG(t, 4, 0, 0, []color.RGBA{spriteRed, spriteBlue})
	// Water terrain underneath a red sprite. With sprite art present, the
	// terrain must be a translucent tint, so the red sprite shows through.
	md := &MapData{
		Width:       1,
		Height:      1,
		TileSize:    ts,
		TerrainGrid: []TerrainType{TerrainWater},
		Tilesets: []RenderTileset{{
			FirstGID:   1,
			Columns:    2,
			TileWidth:  4,
			TileHeight: 4,
			ImagePNG:   sheet,
		}},
		SpriteLayers: []SpriteLayer{{
			Name: "art",
			Data: []uint32{1}, // red tile
		}},
	}

	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}

	x0, y0 := cellCenter(0, 0, ts)
	r, _, b := rgb8(img, x0, y0)
	// Opaque water (0x44,0x88,0xCC) would have b > r. With a translucent
	// tint over the red sprite, red dominates.
	if r <= b {
		t.Errorf("water tint should let red sprite show through (r>b): got r=%d b=%d", r, b)
	}
}

func TestDrawTerrain_OpaqueWhenNoSpriteArt(t *testing.T) {
	// Regression guard: with no sprite art, water stays fully opaque.
	md := &MapData{
		Width:       1,
		Height:      1,
		TileSize:    8,
		TerrainGrid: []TerrainType{TerrainWater},
	}
	dc := gg.NewContext(8, 8)
	DrawTerrain(dc, md)
	img := dc.Image()
	r, g, b := rgb8(img, 4, 4)
	want := TerrainWater.TerrainColor()
	if r != want.R || g != want.G || b != want.B {
		t.Errorf("opaque water without sprite art: got (%d,%d,%d), want (%d,%d,%d)", r, g, b, want.R, want.G, want.B)
	}
}
