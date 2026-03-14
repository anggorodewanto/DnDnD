package renderer

import (
	"image/color"
	"image/png"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TDD Cycle 16: DrawZoneOverlays renders overlays on affected tiles
func TestDrawZoneOverlays_BasicRender(t *testing.T) {
	md := &MapData{
		Width:    5,
		Height:   5,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 25)
			for i := range g {
				g[i] = TerrainOpenGround
			}
			return g
		}(),
		ZoneOverlays: []ZoneOverlay{
			{
				OriginCol: 2,
				OriginRow: 2,
				AffectedTiles: []GridPos{
					{Col: 1, Row: 1}, {Col: 1, Row: 2}, {Col: 1, Row: 3},
					{Col: 2, Row: 1}, {Col: 2, Row: 2}, {Col: 2, Row: 3},
					{Col: 3, Row: 1}, {Col: 3, Row: 2}, {Col: 3, Row: 3},
				},
				Color:      color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0x80},
				MarkerIcon: "\u2601",
			},
		},
	}

	data, err := RenderMap(md)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify valid PNG
	_, err = png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
}

// TDD Cycle 17: Map with zone overlays includes zones in legend
func TestRenderMap_ZoneOverlaysWithLegend(t *testing.T) {
	md := &MapData{
		Width:    5,
		Height:   5,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 25)
			for i := range g {
				g[i] = TerrainOpenGround
			}
			return g
		}(),
		ZoneOverlays: []ZoneOverlay{
			{
				OriginCol:     2,
				OriginRow:     2,
				AffectedTiles: []GridPos{{Col: 2, Row: 2}},
				Color:         color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0x80},
				MarkerIcon:    "\u2601",
			},
		},
		ActiveEffects: []ActiveEffect{
			{Symbol: "\u2601", Name: "Fog Cloud", CasterName: "Wizard", Area: "20ft radius", Rounds: 5},
		},
	}

	data, err := RenderMap(md)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	img, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)

	// Should have legend height (active effects present)
	legendH := LegendHeight(md)
	assert.Greater(t, legendH, 0)

	bounds := img.Bounds()
	expectedH := 5*48 + gridLabelMargin + legendH
	assert.Equal(t, expectedH, bounds.Dy())
}

// TDD Cycle 18: DrawZoneOverlays with no zones is a no-op
func TestDrawZoneOverlays_EmptyZones(t *testing.T) {
	md := &MapData{
		Width:    3,
		Height:   3,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 9)
			return g
		}(),
	}

	data, err := RenderMap(md)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

// TDD Cycle 19: Tiles outside map bounds are skipped
func TestDrawZoneOverlays_OutOfBoundsSkipped(t *testing.T) {
	md := &MapData{
		Width:    3,
		Height:   3,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 9)
			return g
		}(),
		ZoneOverlays: []ZoneOverlay{
			{
				OriginCol: 1,
				OriginRow: 1,
				AffectedTiles: []GridPos{
					{Col: -1, Row: -1}, // out of bounds
					{Col: 1, Row: 1},   // in bounds
					{Col: 10, Row: 10}, // out of bounds
				},
				Color:      color.RGBA{R: 0xFF, A: 0x80},
				MarkerIcon: "\U0001f525",
			},
		},
	}

	data, err := RenderMap(md)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

// TDD Cycle 20: Multiple zone overlays render without error
func TestDrawZoneOverlays_MultipleZones(t *testing.T) {
	md := &MapData{
		Width:    8,
		Height:   8,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 64)
			return g
		}(),
		ZoneOverlays: []ZoneOverlay{
			{
				OriginCol:     2,
				OriginRow:     2,
				AffectedTiles: []GridPos{{Col: 2, Row: 2}, {Col: 3, Row: 2}},
				Color:         color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0x60},
				MarkerIcon:    "\u2601",
			},
			{
				OriginCol:     5,
				OriginRow:     5,
				AffectedTiles: []GridPos{{Col: 5, Row: 5}, {Col: 5, Row: 6}},
				Color:         color.RGBA{R: 0xFF, G: 0x44, A: 0x60},
				MarkerIcon:    "\U0001f525",
			},
		},
	}

	data, err := RenderMap(md)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}
