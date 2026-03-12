package renderer

import (
	"github.com/fogleman/gg"
)

// DrawTerrain renders the terrain layer onto the drawing context.
func DrawTerrain(dc *gg.Context, md *MapData) {
	ts := float64(md.TileSize)
	for row := 0; row < md.Height; row++ {
		for col := 0; col < md.Width; col++ {
			terrain := terrainAt(md, col, row)
			c := terrain.TerrainColor()
			x := float64(col) * ts
			y := float64(row) * ts

			dc.SetColor(c)
			dc.DrawRectangle(x, y, ts, ts)
			dc.Fill()

			if terrain == TerrainDifficultTerrain {
				drawHatching(dc, x, y, ts)
			}
		}
	}
}

// terrainAt returns the terrain type at (col, row), defaulting to open ground.
func terrainAt(md *MapData, col, row int) TerrainType {
	if md.TerrainGrid == nil {
		return TerrainOpenGround
	}
	idx := row*md.Width + col
	if idx < 0 || idx >= len(md.TerrainGrid) {
		return TerrainOpenGround
	}
	return md.TerrainGrid[idx]
}

// drawHatching draws a cross-hatching pattern for difficult terrain.
func drawHatching(dc *gg.Context, x, y, size float64) {
	dc.SetRGBA255(80, 60, 30, 120)
	dc.SetLineWidth(1)
	step := 8.0
	for offset := step; offset < size*2; offset += step {
		dc.DrawLine(x+offset, y, x, y+offset)
		dc.Stroke()
	}
}

// CollectNonStandardTerrain returns the set of terrain types present that are not open ground.
func CollectNonStandardTerrain(grid []TerrainType) map[TerrainType]bool {
	result := map[TerrainType]bool{}
	for _, t := range grid {
		if t != TerrainOpenGround {
			result[t] = true
		}
	}
	return result
}
