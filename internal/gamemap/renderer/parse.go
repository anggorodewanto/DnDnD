package renderer

import (
	"encoding/json"
	"fmt"
)

// tiledMap is the top-level Tiled JSON structure.
type tiledMap struct {
	Width      int           `json:"width"`
	Height     int           `json:"height"`
	TileWidth  int           `json:"tilewidth"`
	TileHeight int           `json:"tileheight"`
	Layers     []tiledLayer  `json:"layers"`
	Tilesets   []tiledTileset `json:"tilesets"`
}

type tiledLayer struct {
	Name    string        `json:"name"`
	Type    string        `json:"type"`
	Width   int           `json:"width"`
	Height  int           `json:"height"`
	Data    []int         `json:"data"`
	Objects []tiledObject `json:"objects"`
}

type tiledObject struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Type   string  `json:"type"`
}

type tiledTileset struct {
	FirstGID int         `json:"firstgid"`
	Name     string      `json:"name"`
	Tiles    []tiledTile `json:"tiles"`
}

type tiledTile struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
}

// ParseTiledJSON converts Tiled-format JSON into a MapData structure for rendering.
func ParseTiledJSON(raw json.RawMessage, combatants []Combatant, effects []ActiveEffect) (*MapData, error) {
	var tm tiledMap
	if err := json.Unmarshal(raw, &tm); err != nil {
		return nil, fmt.Errorf("parsing tiled JSON: %w", err)
	}

	md := &MapData{
		Width:         tm.Width,
		Height:        tm.Height,
		TileSize:      tm.TileWidth,
		Combatants:    combatants,
		ActiveEffects: effects,
	}

	// Build GID -> TerrainType mapping
	gidMap := buildGIDMap(tm.Tilesets)

	// Parse terrain layer
	md.TerrainGrid = parseTerrainLayer(tm, gidMap)

	// Parse walls layer
	md.Walls = parseWallsLayer(tm)

	// SR-029: parse lighting/elevation/spawn_zones (write-only on server before).
	lightingGIDs := buildLightingGIDMap(tm.Tilesets)
	md.MagicalDarknessTiles = parseLightingLayer(tm, lightingGIDs)
	md.ElevationByTile = parseElevationLayer(tm)
	md.SpawnZones = parseSpawnZonesLayer(tm)

	return md, nil
}

// buildGIDMap builds a mapping from GID to TerrainType using tileset definitions.
func buildGIDMap(tilesets []tiledTileset) map[int]TerrainType {
	gidMap := map[int]TerrainType{}
	for _, ts := range tilesets {
		for _, tile := range ts.Tiles {
			gid := ts.FirstGID + tile.ID
			switch tile.Type {
			case "open_ground":
				gidMap[gid] = TerrainOpenGround
			case "difficult_terrain":
				gidMap[gid] = TerrainDifficultTerrain
			case "water":
				gidMap[gid] = TerrainWater
			case "lava":
				gidMap[gid] = TerrainLava
			case "pit":
				gidMap[gid] = TerrainPit
			}
		}
	}
	return gidMap
}

// parseTerrainLayer extracts the terrain grid from the tiled map data.
func parseTerrainLayer(tm tiledMap, gidMap map[int]TerrainType) []TerrainType {
	for _, layer := range tm.Layers {
		if layer.Name != "terrain" || layer.Type != "tilelayer" {
			continue
		}
		grid := make([]TerrainType, len(layer.Data))
		for i, gid := range layer.Data {
			// Zero value (TerrainOpenGround) is already set by make
			if t, ok := gidMap[gid]; ok {
				grid[i] = t
			}
		}
		return grid
	}

	// No terrain layer found: default to open ground (zero value is TerrainOpenGround)
	return make([]TerrainType, tm.Width*tm.Height)
}

// rawLightingGID is the canonical fallback GID -> LightingType map matching the
// Svelte editor's LIGHTING_TYPES.gid scheme. The current editor writes these
// small GIDs directly into the lighting tilelayer (instead of the tileset's
// firstgid+id offset). Keeping this fallback lets us honor map-baked magical
// darkness today without depending on SR-030 (default Tiled JSON drift).
var rawLightingGID = map[int]LightingType{
	1: LightingDimLight,
	2: LightingDarkness,
	3: LightingMagicalDarkness,
	4: LightingFog,
	5: LightingLightObscurement,
}

// buildLightingGIDMap resolves lighting-type GIDs via tileset metadata
// (firstgid + tile.id where tile.type names the lighting kind). The Tiled-correct
// path. Maps that follow the canonical LIGHTING_TYPES.gid scheme without
// matching tileset entries fall back through rawLightingGID at parse time.
func buildLightingGIDMap(tilesets []tiledTileset) map[int]LightingType {
	out := map[int]LightingType{}
	for _, ts := range tilesets {
		for _, tile := range ts.Tiles {
			gid := ts.FirstGID + tile.ID
			switch tile.Type {
			case "dim_light":
				out[gid] = LightingDimLight
			case "darkness":
				out[gid] = LightingDarkness
			case "magical_darkness":
				out[gid] = LightingMagicalDarkness
			case "fog":
				out[gid] = LightingFog
			case "light_obscurement":
				out[gid] = LightingLightObscurement
			}
		}
	}
	return out
}

// resolveLighting maps a raw lighting layer GID to a LightingType. Tileset
// metadata (Tiled-correct) wins; otherwise we fall back to LIGHTING_TYPES.gid.
// GID 0 always means "no override" → LightingNormal.
func resolveLighting(gid int, tilesetMap map[int]LightingType) LightingType {
	if gid == 0 {
		return LightingNormal
	}
	if t, ok := tilesetMap[gid]; ok {
		return t
	}
	if t, ok := rawLightingGID[gid]; ok {
		return t
	}
	return LightingNormal
}

// parseLightingLayer scans the "lighting" tilelayer and returns the union of
// magical-darkness tiles. Other lighting kinds (dim/darkness/fog/light
// obscurement) are recognized but currently consumed elsewhere (SR-008 +
// Batch 10 obscurement gap) — this parse path is the static-map feed only.
func parseLightingLayer(tm tiledMap, tilesetMap map[int]LightingType) []GridPos {
	for _, layer := range tm.Layers {
		if layer.Name != "lighting" || layer.Type != "tilelayer" {
			continue
		}
		var dark []GridPos
		w := tm.Width
		for i, gid := range layer.Data {
			lt := resolveLighting(gid, tilesetMap)
			if lt != LightingMagicalDarkness {
				continue
			}
			if w <= 0 {
				continue
			}
			dark = append(dark, GridPos{Col: i % w, Row: i / w})
		}
		return dark
	}
	return nil
}

// parseElevationLayer extracts the elevation grid (feet, row-major) from the
// "elevation" tilelayer. Returns nil if the layer is absent.
func parseElevationLayer(tm tiledMap) []int {
	for _, layer := range tm.Layers {
		if layer.Name != "elevation" || layer.Type != "tilelayer" {
			continue
		}
		grid := make([]int, len(layer.Data))
		copy(grid, layer.Data)
		return grid
	}
	return nil
}

// parseSpawnZonesLayer scans the "spawn_zones" objectgroup and returns the
// rectangles in tile units. Mirrors internal/exploration.ParseSpawnZones but
// renderer-local so the renderer package has no exploration dependency.
func parseSpawnZonesLayer(tm tiledMap) []SpawnZoneRect {
	ts := float64(tm.TileWidth)
	if ts <= 0 {
		return nil
	}
	var zones []SpawnZoneRect
	for _, layer := range tm.Layers {
		if layer.Name != "spawn_zones" || layer.Type != "objectgroup" {
			continue
		}
		for _, obj := range layer.Objects {
			zones = append(zones, SpawnZoneRect{
				ZoneType:   obj.Type,
				TileX:      int(obj.X / ts),
				TileY:      int(obj.Y / ts),
				TileWidth:  int(obj.Width / ts),
				TileHeight: int(obj.Height / ts),
			})
		}
	}
	return zones
}

// parseWallsLayer extracts wall segments from the tiled map object layer.
func parseWallsLayer(tm tiledMap) []WallSegment {
	var walls []WallSegment
	ts := float64(tm.TileWidth)
	if ts <= 0 {
		return walls
	}

	for _, layer := range tm.Layers {
		if layer.Name != "walls" || layer.Type != "objectgroup" {
			continue
		}
		for _, obj := range layer.Objects {
			walls = append(walls, WallSegment{
				X1: obj.X / ts,
				Y1: obj.Y / ts,
				X2: (obj.X + obj.Width) / ts,
				Y2: (obj.Y + obj.Height) / ts,
			})
		}
	}
	return walls
}
