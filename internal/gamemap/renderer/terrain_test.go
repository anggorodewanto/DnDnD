package renderer

import (
	"image"
	"image/color"
	"testing"

	"github.com/fogleman/gg"
)

func TestDrawTerrain_FillsCorrectColors(t *testing.T) {
	// 2x2 map: open, water, lava, pit
	md := &MapData{
		Width:    2,
		Height:   2,
		TileSize: 48,
		TerrainGrid: []TerrainType{
			TerrainOpenGround, TerrainWater,
			TerrainLava, TerrainPit,
		},
	}
	dc := gg.NewContext(96, 96)
	DrawTerrain(dc, md)

	img := dc.Image()

	// Check center pixel of each tile
	checks := []struct {
		x, y    int
		terrain TerrainType
	}{
		{24, 24, TerrainOpenGround},
		{72, 24, TerrainWater},
		{24, 72, TerrainLava},
		{72, 72, TerrainPit},
	}
	for _, c := range checks {
		got := img.At(c.x, c.y)
		want := c.terrain.TerrainColor()
		r1, g1, b1, _ := got.RGBA()
		r2, g2, b2, _ := want.RGBA()
		if r1 != r2 || g1 != g2 || b1 != b2 {
			t.Errorf("pixel (%d,%d) for %v: got %v, want %v", c.x, c.y, c.terrain, got, want)
		}
	}
}

func TestDrawTerrain_DifficultTerrainHasPattern(t *testing.T) {
	// Difficult terrain should have hatching pattern (not uniform color)
	md := &MapData{
		Width:       1,
		Height:      1,
		TileSize:    48,
		TerrainGrid: []TerrainType{TerrainDifficultTerrain},
	}
	dc := gg.NewContext(48, 48)
	DrawTerrain(dc, md)

	img := dc.Image()

	// Sample multiple pixels - should find at least 2 distinct colors (base + hatching)
	colors := map[color.RGBA]bool{}
	for y := 0; y < 48; y += 4 {
		for x := 0; x < 48; x += 4 {
			r, g, b, a := img.At(x, y).RGBA()
			colors[color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}] = true
		}
	}
	if len(colors) < 2 {
		t.Error("difficult terrain should have hatching pattern with multiple colors")
	}
}

func TestDrawTerrain_EmptyGrid(t *testing.T) {
	md := &MapData{
		Width:       2,
		Height:      2,
		TileSize:    48,
		TerrainGrid: nil,
	}
	dc := gg.NewContext(96, 96)
	// Should not panic
	DrawTerrain(dc, md)

	// With nil grid, should use default (open ground) fill
	img := dc.Image()
	got := img.At(24, 24)
	r, g, b, _ := got.RGBA()
	want := TerrainOpenGround.TerrainColor()
	wr, wg, wb, _ := want.RGBA()
	if r != wr || g != wg || b != wb {
		t.Errorf("nil grid should default to open ground, got %v", got)
	}
}

func TestCollectNonStandardTerrain(t *testing.T) {
	grid := []TerrainType{
		TerrainOpenGround, TerrainWater,
		TerrainLava, TerrainOpenGround,
	}
	result := CollectNonStandardTerrain(grid)
	if _, ok := result[TerrainOpenGround]; ok {
		t.Error("should not include open ground")
	}
	if _, ok := result[TerrainWater]; !ok {
		t.Error("should include water")
	}
	if _, ok := result[TerrainLava]; !ok {
		t.Error("should include lava")
	}
	if len(result) != 2 {
		t.Errorf("expected 2 terrain types, got %d", len(result))
	}
}

func TestCollectNonStandardTerrain_AllOpenGround(t *testing.T) {
	grid := []TerrainType{TerrainOpenGround, TerrainOpenGround}
	result := CollectNonStandardTerrain(grid)
	if len(result) != 0 {
		t.Errorf("expected 0 terrain types for all open ground, got %d", len(result))
	}
}

// helper to check if a point has a specific color within tolerance
func colorMatch(img image.Image, x, y int, want color.RGBA, tolerance uint8) bool {
	got := img.At(x, y)
	r1, g1, b1, _ := got.RGBA()
	r2, g2, b2, _ := want.RGBA()
	dr := absDiff(uint8(r1>>8), uint8(r2>>8))
	dg := absDiff(uint8(g1>>8), uint8(g2>>8))
	db := absDiff(uint8(b1>>8), uint8(b2>>8))
	return dr <= tolerance && dg <= tolerance && db <= tolerance
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}
