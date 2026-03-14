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

// VisionSource represents a source of vision on the map (typically a player token).
type VisionSource struct {
	Col             int
	Row             int
	RangeTiles      int  // base vision range in tiles
	DarkvisionTiles int  // darkvision range in tiles (0 = none)
	BlindsightTiles int  // blindsight range in tiles (0 = none)
	TruesightTiles  int  // truesight range in tiles (0 = none)
	HasDevilsSight  bool // can see in magical darkness
}

// FogOfWar holds the computed visibility state for each tile on the map.
type FogOfWar struct {
	Width  int
	Height int
	States []VisibilityState // row-major, len = Width*Height
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
	fow := &FogOfWar{
		Width:  width,
		Height: height,
		States: make([]VisibilityState, width*height),
	}

	for _, src := range sources {
		// Effective range is max of all vision ranges
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

		visible := shadowcast(src.Col, src.Row, effectiveRange, walls, width, height)
		for pos := range visible {
			idx := pos.Row*width + pos.Col
			if idx >= 0 && idx < len(fow.States) {
				fow.States[idx] = Visible
			}
		}
	}

	return fow
}
