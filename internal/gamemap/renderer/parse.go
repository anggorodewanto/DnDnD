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
