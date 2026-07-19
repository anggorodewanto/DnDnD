package renderer

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"

	"github.com/fogleman/gg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestRenderMap_WallHiddenByUnexploredFog guards the wall/fog draw order:
// walls must be drawn BENEATH the fog overlay so a wall on an Unexplored tile
// (a corridor the party has not discovered) is fully hidden by the black fog,
// while a wall on a Visible tile is rendered in the bold slate wall color.
// Regression against a fog leak: if DrawWalls runs AFTER DrawFogOfWar the
// far wall would paint on top of the black overlay and reveal the maze layout.
func TestRenderMap_WallHiddenByUnexploredFog(t *testing.T) {
	ts := 48 // variable (not const) so float64(ts) coordinate math is a runtime conversion
	md := &MapData{
		Width:       12,
		Height:      3,
		TileSize:    ts,
		TerrainGrid: make([]TerrainType, 12*3),
		// Single vision source at (1,1), range 2 tiles.
		VisionSources: []VisionSource{
			{Col: 1, Row: 1, RangeTiles: 2},
		},
		Walls: []WallSegment{
			// Near wall: a short segment inside the source's own (always
			// Visible) tile — no fog covers it, so it renders in slate.
			{X1: 1.3, Y1: 1.2, X2: 1.3, Y2: 1.8},
			// Far wall: a segment on a tile far beyond vision range, so its
			// tile stays Unexplored (solid-black fog) and the wall must not leak.
			{X1: 9.3, Y1: 1.2, X2: 9.3, Y2: 1.8},
		},
	}

	data, err := RenderMap(md)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	img, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)

	// Sanity: fog computed as expected (near tile Visible, far tile Unexplored).
	require.NotNil(t, md.FogOfWar)
	require.Equal(t, Visible, md.FogOfWar.StateAt(1, 1), "source tile must be Visible")
	require.Equal(t, Unexplored, md.FogOfWar.StateAt(9, 1), "far tile must be Unexplored")

	margin := gridLabelMargin

	// Far wall centerline: must be fully black — covered by Unexplored fog.
	// This is the load-bearing assertion: it FAILS when walls draw after fog.
	farX := margin + int(9.3*float64(ts))
	farY := margin + int(1.5*float64(ts))
	fr, fg, fb := rgb8(img, farX, farY)
	assert.Equal(t, uint8(0), fr, "far wall must be hidden under Unexplored fog (fog leak)")
	assert.Equal(t, uint8(0), fg, "far wall must be hidden under Unexplored fog (fog leak)")
	assert.Equal(t, uint8(0), fb, "far wall must be hidden under Unexplored fog (fog leak)")

	// Near wall centerline: rendered in the bold slate wall color.
	nearX := margin + int(1.3*float64(ts))
	nearY := margin + int(1.5*float64(ts))
	nr, ng, nb := rgb8(img, nearX, nearY)
	// Not the cream floor and not black (covered).
	assert.False(t, nr == 0xF0 && ng == 0xF0 && nb == 0xE8, "near wall must not be the floor color")
	assert.False(t, nr == 0 && ng == 0 && nb == 0, "near wall must be drawn, not black/covered")
	// The new slate wall color (small tolerance for stroke anti-aliasing).
	assert.InDelta(t, 0x2C, int(nr), 8, "near wall red ~ slate")
	assert.InDelta(t, 0x38, int(ng), 8, "near wall green ~ slate")
	assert.InDelta(t, 0x4A, int(nb), 8, "near wall blue ~ slate")
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
