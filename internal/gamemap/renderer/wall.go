package renderer

import (
	"image/color"

	"github.com/fogleman/gg"
)

// DrawWalls renders wall segments on the map.
func DrawWalls(dc *gg.Context, md *MapData) {
	if len(md.Walls) == 0 {
		return
	}
	ts := float64(md.TileSize)
	// Dark slate — boldly distinct from the cream floor, the tan/brown
	// difficult-terrain fill, the light-grey grid, and the green tokens, so a
	// DM can read wall segments at a glance. Thick stroke with round caps keeps
	// corners of connected segments from leaving gaps.
	dc.SetColor(color.RGBA{R: 0x2C, G: 0x38, B: 0x4A, A: 0xFF})
	dc.SetLineWidth(6)
	dc.SetLineCap(gg.LineCapRound)
	for _, w := range md.Walls {
		dc.DrawLine(w.X1*ts, w.Y1*ts, w.X2*ts, w.Y2*ts)
		dc.Stroke()
	}
}
