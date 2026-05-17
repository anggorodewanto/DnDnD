package renderer

import (
	"image/color"
)

// HealthTier represents the visible health state of a combatant.
type HealthTier int

const (
	TierUninjured HealthTier = iota
	TierScratched
	TierBloodied
	TierCritical
	TierDying
	TierDead
	TierStable
)

// String returns the display name of the health tier.
func (h HealthTier) String() string {
	switch h {
	case TierUninjured:
		return "Uninjured"
	case TierScratched:
		return "Scratched"
	case TierBloodied:
		return "Bloodied"
	case TierCritical:
		return "Critical"
	case TierDying:
		return "Dying"
	case TierDead:
		return "Dead"
	case TierStable:
		return "Stable"
	default:
		return "Unknown"
	}
}

// TierColor returns the color associated with a health tier.
func (h HealthTier) TierColor() color.RGBA {
	switch h {
	case TierUninjured:
		return color.RGBA{R: 0x22, G: 0xAA, B: 0x22, A: 0xFF} // green
	case TierScratched:
		return color.RGBA{R: 0xDD, G: 0xDD, B: 0x00, A: 0xFF} // yellow
	case TierBloodied:
		return color.RGBA{R: 0xEE, G: 0x88, B: 0x00, A: 0xFF} // orange
	case TierCritical:
		return color.RGBA{R: 0xDD, G: 0x22, B: 0x22, A: 0xFF} // red
	case TierDying:
		return color.RGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xAA} // dimmed
	case TierDead:
		return color.RGBA{R: 0x66, G: 0x66, B: 0x66, A: 0xAA} // greyed
	case TierStable:
		return color.RGBA{R: 0x88, G: 0x88, B: 0xAA, A: 0xBB} // dimmed blue-ish
	default:
		return color.RGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xFF}
	}
}

// TerrainType represents the terrain classification of a map tile.
type TerrainType int

const (
	TerrainOpenGround      TerrainType = 0
	TerrainDifficultTerrain TerrainType = 1
	TerrainWater           TerrainType = 2
	TerrainLava            TerrainType = 3
	TerrainPit             TerrainType = 4
)

// String returns the display name of a terrain type.
func (t TerrainType) String() string {
	switch t {
	case TerrainOpenGround:
		return "Open Ground"
	case TerrainDifficultTerrain:
		return "Difficult Terrain"
	case TerrainWater:
		return "Water"
	case TerrainLava:
		return "Lava"
	case TerrainPit:
		return "Pit"
	default:
		return "Unknown"
	}
}

// TerrainColor returns the fill color for a terrain type.
func (t TerrainType) TerrainColor() color.RGBA {
	switch t {
	case TerrainOpenGround:
		return color.RGBA{R: 0xF0, G: 0xF0, B: 0xE8, A: 0xFF} // light beige
	case TerrainDifficultTerrain:
		return color.RGBA{R: 0xBB, G: 0x99, B: 0x66, A: 0xFF} // brown
	case TerrainWater:
		return color.RGBA{R: 0x44, G: 0x88, B: 0xCC, A: 0xFF} // blue
	case TerrainLava:
		return color.RGBA{R: 0xEE, G: 0x66, B: 0x00, A: 0xFF} // orange
	case TerrainPit:
		return color.RGBA{R: 0x55, G: 0x55, B: 0x55, A: 0xFF} // dark grey
	default:
		return color.RGBA{R: 0xF0, G: 0xF0, B: 0xE8, A: 0xFF}
	}
}

// Combatant represents a creature on the map for rendering purposes.
type Combatant struct {
	ShortID     string
	DisplayName string
	Col         int // zero-based column index
	Row         int // zero-based row index
	AltitudeFt  int
	HPMax       int
	HPCurrent   int
	IsPlayer    bool
	IsDying     bool // 0 HP, making death saves
	IsStable    bool // 0 HP, stabilized
	IsVisible   bool // false when hidden (Hide action); excluded from player view
	InFog       bool // true when on an explored (dim) tile — rendered greyed out
}

// HealthTier computes the health tier for a combatant.
func (c Combatant) HealthTier() HealthTier {
	if c.HPCurrent <= 0 {
		if c.IsStable {
			return TierStable
		}
		if c.IsDying {
			return TierDying
		}
		return TierDead
	}
	if c.HPMax <= 0 {
		return TierUninjured
	}
	pct := float64(c.HPCurrent) / float64(c.HPMax) * 100
	if pct >= 100 {
		return TierUninjured
	}
	if pct >= 75 {
		return TierScratched
	}
	if pct >= 25 {
		return TierBloodied
	}
	return TierCritical
}

// WallSegment represents a wall line on the map.
type WallSegment struct {
	X1, Y1, X2, Y2 float64 // coordinates in tile units
}

// ActiveEffect represents an active spell/effect on the map.
type ActiveEffect struct {
	Symbol     string
	Name       string
	CasterName string
	Area       string // e.g. "20ft radius"
	Rounds     int    // remaining rounds
}

// GridPos represents a 0-based grid position (col, row).
type GridPos struct {
	Col int
	Row int
}

// ZoneOverlay represents a zone effect overlay to render on the map.
type ZoneOverlay struct {
	OriginCol     int
	OriginRow     int
	AffectedTiles []GridPos
	Color         color.RGBA
	MarkerIcon    string
}

// LightSource represents a light source on the map (torch, Light cantrip, etc.).
// Light sources contribute additional visible tiles, acting like vision sources
// without a specific viewer.
type LightSource struct {
	Col        int // zero-based column
	Row        int // zero-based row
	RangeTiles int // illumination range in tiles
}

// LightingType represents the painted lighting/obscurement classification of a
// map tile (Phase 21c "lighting brush"). Mirrors Svelte LIGHTING_TYPES.
type LightingType int

const (
	LightingNormal LightingType = iota
	LightingDimLight
	LightingDarkness
	LightingMagicalDarkness
	LightingFog
	LightingLightObscurement
)

// SpawnZoneRect is a rectangular spawn-zone region painted in the Map Editor,
// expressed in tile units. ZoneType is "player" or "enemy". Parallel to
// internal/exploration.SpawnZone but renderer-local so the renderer package
// does not depend on exploration.
type SpawnZoneRect struct {
	ZoneType   string // "player" or "enemy"
	TileX      int
	TileY      int
	TileWidth  int
	TileHeight int
}

// MapData holds all the data needed to render a map image.
type MapData struct {
	Width         int           // map width in tiles
	Height        int           // map height in tiles
	TileSize      int           // pixel size per tile
	TerrainGrid   []TerrainType // row-major terrain data, len = Width*Height
	Walls         []WallSegment
	Combatants    []Combatant
	ActiveEffects []ActiveEffect
	ZoneOverlays  []ZoneOverlay

	// Fog of War fields
	VisionSources []VisionSource // player vision sources; if non-empty, fog is computed
	LightSources  []LightSource  // light sources (torches, Light cantrip, etc.)
	FogOfWar      *FogOfWar      // pre-computed fog; if nil and VisionSources/LightSources set, computed automatically

	// MagicalDarknessTiles is the union of every magical-darkness zone's
	// affected-tile set. Used by ComputeVisibilityWithZones to demote
	// darkvision inside the zone (Devil's Sight still penetrates).
	// Populated both from runtime encounter_zones (SR-008) and from
	// statically-painted map lighting tiles (SR-029).
	MagicalDarknessTiles []GridPos

	// ElevationByTile is the row-major elevation grid in feet (len = Width*Height)
	// parsed from the Tiled "elevation" tilelayer (SR-029). Map-baked elevation
	// only sets ground elevation per tile; combatant altitude (`Combatant.AltitudeFt`)
	// still wins for token rendering. Downstream consumers (cliff cover, default
	// altitude) are out of scope for SR-029.
	ElevationByTile []int

	// SpawnZones lists the player/enemy spawn rectangles parsed from the Tiled
	// "spawn_zones" objectgroup (SR-029). Exploration code reads the same JSON
	// separately via internal/exploration.ParseSpawnZones — this field is the
	// renderer-side mirror for future DM-view overlays.
	SpawnZones []SpawnZoneRect

	// DMSeesAll, when true, propagates to the auto-computed FogOfWar and
	// instructs DrawFogOfWar + filterCombatantsForFog to bypass fog so the
	// DM-view PNG renders every tile at full brightness regardless of
	// player vision. Caller may also set FogOfWar.DMSeesAll directly.
	DMSeesAll bool

	// BackgroundImage is the raw PNG bytes of the map's background image
	// (looked up via AssetStore from maps.background_image_id). When non-nil,
	// the image is composited beneath the terrain layer at BackgroundOpacity.
	BackgroundImage []byte

	// BackgroundOpacity controls the opacity of the background image layer
	// (0.0 = fully transparent, 1.0 = fully opaque). Only used when
	// BackgroundImage is non-nil.
	BackgroundOpacity float64
}
