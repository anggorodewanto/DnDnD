package renderer

import (
	"image/color"
	"testing"

	"github.com/fogleman/gg"
)

func TestDrawWalls(t *testing.T) {
	md := &MapData{
		Width:    3,
		Height:   3,
		TileSize: 48,
		Walls: []WallSegment{
			{X1: 0, Y1: 0, X2: 3, Y2: 0}, // top wall
			{X1: 1, Y1: 1, X2: 1, Y2: 3}, // vertical wall
		},
	}
	dc := gg.NewContext(3*48, 3*48)
	dc.SetColor(color.White)
	dc.Clear()

	DrawWalls(dc, md)

	img := dc.Image()
	// The top wall should draw a thick line at y=0.
	// Check that pixels along y=0 are not white.
	r, g, b, _ := img.At(72, 1).RGBA()
	wr, wg, wb, _ := color.White.RGBA()
	if r == wr && g == wg && b == wb {
		t.Error("expected wall at y=0 but found white pixel")
	}
}

func TestDrawWalls_Empty(t *testing.T) {
	md := &MapData{Width: 2, Height: 2, TileSize: 48}
	dc := gg.NewContext(96, 96)
	dc.SetColor(color.White)
	dc.Clear()

	// No walls - should not panic
	DrawWalls(dc, md)

	// All pixels should still be white (no walls drawn)
	img := dc.Image()
	r, g, b, _ := img.At(48, 48).RGBA()
	wr, wg, wb, _ := color.White.RGBA()
	if r != wr || g != wg || b != wb {
		t.Error("no walls should mean no marks on canvas")
	}
}
