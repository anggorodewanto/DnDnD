package renderer

import (
	"bytes"
	"image/png"

	"github.com/fogleman/gg"
)

// RenderMap generates a PNG image from the given map data.
// Returns the PNG bytes or an error.
func RenderMap(md *MapData) ([]byte, error) {
	// Auto-reduce tile size for large maps (>100 in either dimension)
	if md.Width > 100 || md.Height > 100 {
		md.TileSize = 32
	}

	margin := gridLabelMargin
	mapW := md.Width * md.TileSize
	mapH := md.Height * md.TileSize
	legendH := LegendHeight(md)

	totalW := mapW + margin
	totalH := mapH + margin + legendH

	dc := gg.NewContext(totalW, totalH)

	// White background
	dc.SetRGB(1, 1, 1)
	dc.Clear()

	// Push offset for the map area (after label margin)
	dc.Push()
	dc.Translate(float64(margin), float64(margin))

	// 1. Terrain layer
	DrawTerrain(dc, md)

	// 1.5. Zone overlays (between terrain and grid so grid lines show over overlays)
	DrawZoneOverlays(dc, md)

	// 2. Grid lines
	DrawGrid(dc, md)

	// 3. Walls
	DrawWalls(dc, md)

	// 4. Tokens
	DrawTokens(dc, md)

	dc.Pop()

	// 5. Coordinate labels (drawn in full context with margin)
	DrawCoordinateLabels(dc, md)

	// 6. Legend (below the map)
	if legendH > 0 {
		DrawLegend(dc, md, mapH+margin)
	}

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
