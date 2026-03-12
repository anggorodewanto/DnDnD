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
	dc.SetColor(color.RGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xFF})
	dc.SetLineWidth(3)
	for _, w := range md.Walls {
		dc.DrawLine(w.X1*ts, w.Y1*ts, w.X2*ts, w.Y2*ts)
		dc.Stroke()
	}
}
