package renderer

// VisibilityState represents the fog of war state of a tile.
type VisibilityState int

const (
	// Unexplored means the tile has never been seen (solid black).
	Unexplored VisibilityState = iota
	// Explored means the tile was previously visible but not currently (dim overlay).
	Explored
	// Visible means the tile is currently visible (no overlay).
	Visible
)

// devilsSightTileRange is the max range of Devil's Sight in tiles (120ft / 5ft per tile).
// Mirrors `devilsSightRange` in internal/combat/obscurement.go but in tile units so the
// renderer package stays free of a combat-package import.
const devilsSightTileRange = 24

// VisionSource represents a source of vision on the map (typically a player token).
type VisionSource struct {
	Col             int
	Row             int
	RangeTiles      int  // base vision range in tiles
	DarkvisionTiles int  // darkvision range in tiles (0 = none)
	BlindsightTiles int  // blindsight range in tiles (0 = none)
	TruesightTiles  int  // truesight range in tiles (0 = none)
	HasDevilsSight  bool // Devil's Sight: pierces magical darkness up to 120ft (24 tiles)
}

// FogOfWar holds the computed visibility state for each tile on the map.
type FogOfWar struct {
	Width  int
	Height int
	States []VisibilityState // row-major, len = Width*Height
	// DMSeesAll, when true, instructs DrawFogOfWar and filterCombatantsForFog
	// to bypass the fog entirely (DM-view rendering). The States slice is left
	// intact so the same FogOfWar can be reused for both player and DM views.
	DMSeesAll bool
}

// StateAt returns the visibility state at (col, row). Out-of-bounds returns Unexplored.
func (f *FogOfWar) StateAt(col, row int) VisibilityState {
	if col < 0 || col >= f.Width || row < 0 || row >= f.Height {
		return Unexplored
	}
	return f.States[row*f.Width+col]
}

// ComputeVisibility computes the fog of war visibility for the map by unioning
// the visible tiles from all vision sources.
func ComputeVisibility(sources []VisionSource, walls []WallSegment, width, height int) *FogOfWar {
	return ComputeVisibilityWithLights(sources, nil, walls, width, height)
}

// ComputeVisibilityWithLights computes the fog of war visibility by unioning
// visible tiles from all vision sources and light sources.
func ComputeVisibilityWithLights(sources []VisionSource, lights []LightSource, walls []WallSegment, width, height int) *FogOfWar {
	return ComputeVisibilityWithZones(sources, lights, walls, nil, width, height)
}

// ComputeVisibilityWithZones is the full-featured visibility computation:
// it unions vision and light sources but ALSO demotes darkvision through
// magical-darkness tiles. A darkvision-only viewer cannot see Visible into a
// magical-darkness tile; a viewer with Devil's Sight can (within 24 tiles).
// magicalDarknessTiles is the union of every magical-darkness zone's
// affected-tile set; pass nil when no such zones exist.
func ComputeVisibilityWithZones(sources []VisionSource, lights []LightSource, walls []WallSegment, magicalDarknessTiles []GridPos, width, height int) *FogOfWar {
	fow := &FogOfWar{
		Width:  width,
		Height: height,
		States: make([]VisibilityState, width*height),
	}

	dark := gridPosSet(magicalDarknessTiles)

	for _, src := range sources {
		applyVisionSource(fow, src, walls, dark, width, height)
	}

	for _, light := range lights {
		effectiveRange := light.RangeTiles
		if light.DimRangeTiles > effectiveRange {
			effectiveRange = light.DimRangeTiles
		}
		visible := shadowcast(light.Col, light.Row, effectiveRange, walls, width, height)
		for pos := range visible {
			idx := pos.Row*width + pos.Col
			if idx < 0 || idx >= len(fow.States) {
				continue
			}
			dist := chebyshevDistance(light.Col, light.Row, pos.Col, pos.Row)
			if dist <= light.RangeTiles {
				fow.States[idx] = Visible
			} else if light.DimRangeTiles > 0 && dist <= light.DimRangeTiles && fow.States[idx] < Explored {
				fow.States[idx] = Explored
			}
		}
	}

	return fow
}

// applyVisionSource unions one VisionSource's visible tiles into fow,
// respecting magical-darkness demotion of ordinary vision (Devil's Sight,
// blindsight, and truesight still penetrate within their ranges).
func applyVisionSource(fow *FogOfWar, src VisionSource, walls []WallSegment, magicalDark map[GridPos]bool, width, height int) {
	effectiveRange := src.RangeTiles
	if src.DarkvisionTiles > effectiveRange {
		effectiveRange = src.DarkvisionTiles
	}
	if src.BlindsightTiles > effectiveRange {
		effectiveRange = src.BlindsightTiles
	}
	if src.TruesightTiles > effectiveRange {
		effectiveRange = src.TruesightTiles
	}
	if effectiveRange <= 0 {
		// Origin tile is always visible if the source exists at all.
		idx := src.Row*width + src.Col
		if idx >= 0 && idx < len(fow.States) {
			fow.States[idx] = Visible
		}
		return
	}

	visible := shadowcast(src.Col, src.Row, effectiveRange, walls, width, height)
	for pos := range visible {
		if !tileVisibleFromSource(src, pos, magicalDark) {
			continue
		}
		idx := pos.Row*width + pos.Col
		if idx < 0 || idx >= len(fow.States) {
			continue
		}
		fow.States[idx] = Visible
	}
}

// tileVisibleFromSource decides whether a candidate tile (already in geometric
// LOS of the source) should be marked Visible given magical-darkness zones.
// Magical darkness is heavily obscured for ALL ordinary vision (base sight and
// darkvision alike); only blindsight, truesight, and Devil's Sight (≤120ft)
// pierce it. Mirrors EffectiveObscurement semantics in internal/combat.
func tileVisibleFromSource(src VisionSource, pos GridPos, magicalDark map[GridPos]bool) bool {
	if !magicalDark[pos] {
		return true
	}
	// Origin tile of the source is always visible.
	if pos.Col == src.Col && pos.Row == src.Row {
		return true
	}
	dist := chebyshevDistance(src.Col, src.Row, pos.Col, pos.Row)
	if src.BlindsightTiles > 0 && dist <= src.BlindsightTiles {
		return true
	}
	if src.TruesightTiles > 0 && dist <= src.TruesightTiles {
		return true
	}
	if src.HasDevilsSight && dist <= devilsSightTileRange {
		return true
	}
	return false
}

// chebyshevDistance returns the max-axis distance between two grid cells in tiles.
// FoW uses tile-unit distances; chebyshev matches the renderer's vision range
// semantics (a 1-tile diagonal is "1 tile away").
func chebyshevDistance(c1, r1, c2, r2 int) int {
	dc := c1 - c2
	if dc < 0 {
		dc = -dc
	}
	dr := r1 - r2
	if dr < 0 {
		dr = -dr
	}
	if dc > dr {
		return dc
	}
	return dr
}

// gridPosSet builds a lookup map from a tile slice. Nil-safe.
func gridPosSet(tiles []GridPos) map[GridPos]bool {
	if len(tiles) == 0 {
		return nil
	}
	m := make(map[GridPos]bool, len(tiles))
	for _, t := range tiles {
		m[t] = true
	}
	return m
}
