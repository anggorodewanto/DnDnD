package renderer

import (
	"github.com/fogleman/gg"
)

// DrawZoneOverlays renders semi-transparent color overlays for each zone
// on the affected tiles, and draws a marker icon at the zone origin.
func DrawZoneOverlays(dc *gg.Context, md *MapData) {
	if len(md.ZoneOverlays) == 0 {
		return
	}

	ts := float64(md.TileSize)

	for _, zo := range md.ZoneOverlays {
		// Draw semi-transparent overlay on each affected tile
		dc.SetColor(zo.Color)
		for _, tile := range zo.AffectedTiles {
			if tile.Col < 0 || tile.Col >= md.Width || tile.Row < 0 || tile.Row >= md.Height {
				continue
			}
			x := float64(tile.Col) * ts
			y := float64(tile.Row) * ts
			dc.DrawRectangle(x, y, ts, ts)
			dc.Fill()
		}

		// Draw marker icon at origin
		if zo.MarkerIcon != "" && zo.OriginCol >= 0 && zo.OriginCol < md.Width && zo.OriginRow >= 0 && zo.OriginRow < md.Height {
			cx := float64(zo.OriginCol)*ts + ts/2
			cy := float64(zo.OriginRow)*ts + ts/2
			fontSize := ts * 0.5
			_ = dc.LoadFontFace("", fontSize)
			dc.SetRGBA(0, 0, 0, 0.8)
			dc.DrawStringAnchored(zo.MarkerIcon, cx, cy, 0.5, 0.5)
		}
	}
}
