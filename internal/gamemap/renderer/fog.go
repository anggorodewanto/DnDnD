package renderer

import (
	"image/color"

	"github.com/fogleman/gg"
)

// DrawFogOfWar renders the fog overlay on non-visible tiles.
// Unexplored tiles get a solid black overlay. Explored tiles get a dim overlay.
// Visible tiles have no overlay.
// When FogOfWar.DMSeesAll is true the fog overlay is bypassed entirely so the
// DM-view PNG shows every tile at full brightness regardless of player vision.
func DrawFogOfWar(dc *gg.Context, md *MapData) {
	if md.FogOfWar == nil {
		return
	}
	if md.FogOfWar.DMSeesAll {
		return
	}

	ts := float64(md.TileSize)
	fow := md.FogOfWar

	for row := 0; row < md.Height; row++ {
		for col := 0; col < md.Width; col++ {
			state := fow.StateAt(col, row)
			if state == Visible {
				continue
			}

			x := float64(col) * ts
			y := float64(row) * ts

			switch state {
			case Unexplored:
				dc.SetColor(color.RGBA{R: 0, G: 0, B: 0, A: 0xFF})
			case Explored:
				dc.SetColor(color.RGBA{R: 0, G: 0, B: 0, A: 0xAA})
			}
			dc.DrawRectangle(x, y, ts, ts)
			dc.Fill()
		}
	}
}

// filterCombatantsForFog returns combatants that should be rendered given the fog state.
// Player tokens are always rendered. Enemy tokens on unexplored tiles are hidden.
// Enemy tokens on explored (dim) tiles are included but marked with InFog=true
// so they render greyed out. If fog is nil OR FogOfWar.DMSeesAll is true,
// all combatants are returned unmodified (DM mode).
func filterCombatantsForFog(combatants []Combatant, fow *FogOfWar) []Combatant {
	if fow == nil {
		return combatants
	}
	if fow.DMSeesAll {
		return combatants
	}

	result := make([]Combatant, 0, len(combatants))
	for _, c := range combatants {
		if c.IsPlayer {
			result = append(result, c)
			continue
		}

		if !c.IsVisible {
			continue
		}

		state := fow.StateAt(c.Col, c.Row)
		if state == Unexplored {
			continue
		}

		if state == Explored {
			c.InFog = true
		}
		result = append(result, c)
	}
	return result
}
