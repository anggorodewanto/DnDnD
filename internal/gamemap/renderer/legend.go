package renderer

import (
	"fmt"
	"image/color"
	"sort"

	"github.com/fogleman/gg"
)

const (
	legendRowHeight  = 24
	legendPadding    = 12
	legendSwatchSize = 16
)

// NeedsLegend returns true if the map requires a legend panel.
func NeedsLegend(md *MapData) bool {
	return len(md.ActiveEffects) > 0 || len(CollectNonStandardTerrain(md.TerrainGrid)) > 0
}

// LegendHeight returns the height in pixels needed for the legend panel.
// Returns 0 if no legend is needed.
func LegendHeight(md *MapData) int {
	if !NeedsLegend(md) {
		return 0
	}
	nonStandard := CollectNonStandardTerrain(md.TerrainGrid)
	rows := 0

	if len(nonStandard) > 0 {
		rows++ // "Terrain" header
		rows += len(nonStandard)
	}
	if len(md.ActiveEffects) > 0 {
		rows++ // "Active Effects" header
		rows += len(md.ActiveEffects)
	}

	return rows*legendRowHeight + legendPadding*2
}

// DrawLegend renders the legend panel on the context at the given y offset.
func DrawLegend(dc *gg.Context, md *MapData, yOffset int) {
	if !NeedsLegend(md) {
		return
	}

	nonStandard := CollectNonStandardTerrain(md.TerrainGrid)
	y := float64(yOffset) + float64(legendPadding)
	x := float64(legendPadding)
	fontSize := 11.0

	_ = dc.LoadFontFace("", fontSize)

	// Terrain key section
	if len(nonStandard) > 0 {
		dc.SetColor(color.RGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xFF})
		dc.DrawStringAnchored("Terrain", x, y, 0, 0.5)
		y += legendRowHeight

		// Sort terrain types for consistent ordering
		terrains := make([]TerrainType, 0, len(nonStandard))
		for t := range nonStandard {
			terrains = append(terrains, t)
		}
		sort.Slice(terrains, func(i, j int) bool { return int(terrains[i]) < int(terrains[j]) })

		for _, t := range terrains {
			// Draw swatch
			tc := t.TerrainColor()
			dc.SetColor(tc)
			dc.DrawRectangle(x, y-legendSwatchSize/2, legendSwatchSize, legendSwatchSize)
			dc.Fill()

			// Swatch border
			dc.SetColor(color.RGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xFF})
			dc.SetLineWidth(1)
			dc.DrawRectangle(x, y-legendSwatchSize/2, legendSwatchSize, legendSwatchSize)
			dc.Stroke()

			// Hatching for difficult terrain
			if t == TerrainDifficultTerrain {
				drawHatching(dc, x, y-legendSwatchSize/2, legendSwatchSize)
			}

			// Label
			dc.SetColor(color.RGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xFF})
			dc.DrawStringAnchored(t.String(), x+legendSwatchSize+8, y, 0, 0.5)
			y += legendRowHeight
		}
	}

	// Active effects section
	if len(md.ActiveEffects) > 0 {
		dc.SetColor(color.RGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xFF})
		dc.DrawStringAnchored("Active Effects", x, y, 0, 0.5)
		y += legendRowHeight

		for _, e := range md.ActiveEffects {
			text := fmt.Sprintf("%s %s (%s) \u2014 %s \u2014 %d rounds remaining", e.Symbol, e.Name, e.CasterName, e.Area, e.Rounds)
			dc.SetColor(color.RGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF})
			dc.DrawStringAnchored(text, x, y, 0, 0.5)
			y += legendRowHeight
		}
	}
}
