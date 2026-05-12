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

	// Compute fog of war if vision sources or light sources are provided but fog is not pre-computed
	if (len(md.VisionSources) > 0 || len(md.LightSources) > 0) && md.FogOfWar == nil {
		md.FogOfWar = ComputeVisibilityWithZones(md.VisionSources, md.LightSources, md.Walls, md.MagicalDarknessTiles, md.Width, md.Height)
	}
	// Propagate DMSeesAll into the FoW so downstream draw helpers see it
	// regardless of whether the caller pre-computed the fog or asked for
	// auto-compute. DMSeesAll on the FoW is the load-bearing flag.
	if md.FogOfWar != nil && md.DMSeesAll {
		md.FogOfWar.DMSeesAll = true
	}

	// Filter combatants based on fog visibility
	originalCombatants := md.Combatants
	if md.FogOfWar != nil {
		md.Combatants = filterCombatantsForFog(md.Combatants, md.FogOfWar)
	}

	// Push offset for the map area (after label margin)
	dc.Push()
	dc.Translate(float64(margin), float64(margin))

	// 1. Terrain layer
	DrawTerrain(dc, md)

	// 1.5. Zone overlays (between terrain and grid so grid lines show over overlays)
	DrawZoneOverlays(dc, md)

	// 1.8. Fog of war overlay (between zones and grid)
	DrawFogOfWar(dc, md)

	// 2. Grid lines
	DrawGrid(dc, md)

	// 3. Walls
	DrawWalls(dc, md)

	// 4. Tokens
	DrawTokens(dc, md)

	dc.Pop()

	// Restore original combatants list
	md.Combatants = originalCombatants

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
