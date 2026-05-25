package renderer

import (
	"github.com/fogleman/gg"
)

// spriteArtTintAlpha is the alpha used to tint special terrain when sprite art
// is present, so the underlying art remains visible.
const spriteArtTintAlpha = 100

// DrawTerrain renders the terrain layer onto the drawing context.
//
// When the map carries sprite/image art (md.HasSpriteArt()), terrain is drawn
// as a translucent tint over the art: open ground is left untouched and
// special terrain (water/lava/pit/difficult) gets a semi-transparent color
// wash. Otherwise terrain is painted with opaque fills.
func DrawTerrain(dc *gg.Context, md *MapData) {
	spriteArt := md.HasSpriteArt()
	ts := float64(md.TileSize)
	for row := 0; row < md.Height; row++ {
		for col := 0; col < md.Width; col++ {
			terrain := terrainAt(md, col, row)
			x := float64(col) * ts
			y := float64(row) * ts

			drawTerrainCell(dc, terrain, x, y, ts, spriteArt)

			if terrain == TerrainDifficultTerrain {
				drawHatching(dc, x, y, ts)
			}
		}
	}
}

// drawTerrainCell paints one terrain cell, either as an opaque fill or, when
// sprite art is present, as a translucent tint (open ground draws nothing).
func drawTerrainCell(dc *gg.Context, terrain TerrainType, x, y, ts float64, spriteArt bool) {
	c := terrain.TerrainColor()

	if !spriteArt {
		dc.SetColor(c)
		dc.DrawRectangle(x, y, ts, ts)
		dc.Fill()
		return
	}

	if terrain == TerrainOpenGround {
		return // let the art show through unchanged
	}

	dc.SetRGBA255(int(c.R), int(c.G), int(c.B), spriteArtTintAlpha)
	dc.DrawRectangle(x, y, ts, ts)
	dc.Fill()
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
