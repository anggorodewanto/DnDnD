// Package spawnzone parses player/enemy spawn rectangles from a Tiled map and
// deterministically seats player characters into them. It is shared by
// exploration mode (StartExploration) and combat mode (StartCombat) so both
// place PCs the same way; it depends on neither package, avoiding an import
// cycle between them.
package spawnzone

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNoPlayerSpawnZones is returned when a map has no "player" spawn zones
// but the caller asked to place PCs.
var ErrNoPlayerSpawnZones = errors.New("map has no player spawn zones")

// ErrNotEnoughSpawnTiles is returned when the player spawn zones do not have
// enough tiles to seat every PC.
var ErrNotEnoughSpawnTiles = errors.New("not enough player spawn tiles")

// TilePos is a 0-indexed tile coordinate on the map grid.
// Col is the X tile index; Row is the Y tile index.
type TilePos struct {
	Col int `json:"col"`
	Row int `json:"row"`
}

// SpawnZone is a rectangular region on the map flagged as a PC or enemy spawn area.
// TileX/TileY/TileWidth/TileHeight are in tile units (not pixels).
// Tiles is the expanded list of tile coordinates covered by the zone, iterated
// row-major (left-to-right, then top-to-bottom) for deterministic PC assignment.
type SpawnZone struct {
	ZoneType   string    `json:"zone_type"` // "player" or "enemy"
	TileX      int       `json:"tile_x"`
	TileY      int       `json:"tile_y"`
	TileWidth  int       `json:"tile_width"`
	TileHeight int       `json:"tile_height"`
	Tiles      []TilePos `json:"tiles"`
}

// tiledForSpawn is a minimal Tiled structure for spawn_zones extraction.
type tiledForSpawn struct {
	TileWidth  int             `json:"tilewidth"`
	TileHeight int             `json:"tileheight"`
	Layers     []tiledSpawnLyr `json:"layers"`
}

type tiledSpawnLyr struct {
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Objects []tiledSpawnObj `json:"objects"`
}

type tiledSpawnObj struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Type   string  `json:"type"` // "player" or "enemy"
}

// ParseSpawnZones extracts spawn zones from a Tiled-format map JSON.
// Zones on non-spawn_zones layers are ignored. Pixel coordinates are converted
// to tile coordinates using the map's tilewidth.
func ParseSpawnZones(raw json.RawMessage) ([]SpawnZone, error) {
	var tm tiledForSpawn
	if err := json.Unmarshal(raw, &tm); err != nil {
		return nil, fmt.Errorf("parsing tiled JSON for spawn zones: %w", err)
	}
	if tm.TileWidth <= 0 {
		return nil, nil
	}
	ts := float64(tm.TileWidth)

	var zones []SpawnZone
	for _, layer := range tm.Layers {
		if layer.Name != "spawn_zones" || layer.Type != "objectgroup" {
			continue
		}
		for _, obj := range layer.Objects {
			zone := SpawnZone{
				ZoneType:   obj.Type,
				TileX:      int(obj.X / ts),
				TileY:      int(obj.Y / ts),
				TileWidth:  int(obj.Width / ts),
				TileHeight: int(obj.Height / ts),
			}
			zone.Tiles = expandTiles(zone)
			zones = append(zones, zone)
		}
	}
	return zones, nil
}

// expandTiles returns the row-major list of tiles covered by a zone.
func expandTiles(z SpawnZone) []TilePos {
	if z.TileWidth <= 0 || z.TileHeight <= 0 {
		return nil
	}
	tiles := make([]TilePos, 0, z.TileWidth*z.TileHeight)
	for dy := 0; dy < z.TileHeight; dy++ {
		for dx := 0; dx < z.TileWidth; dx++ {
			tiles = append(tiles, TilePos{Col: z.TileX + dx, Row: z.TileY + dy})
		}
	}
	return tiles
}

// AssignPCsToSpawnZones deterministically assigns each PC ID to a unique
// player spawn tile. PCs are assigned in input order; tiles are consumed
// row-major from the first player zone, then the second, etc.
// Returns ErrNoPlayerSpawnZones if no player zones exist.
// Returns ErrNotEnoughSpawnTiles if there are fewer player tiles than PCs.
func AssignPCsToSpawnZones(zones []SpawnZone, pcIDs []string) (map[string]TilePos, error) {
	positions := make(map[string]TilePos, len(pcIDs))
	if len(pcIDs) == 0 {
		return positions, nil
	}

	var available []TilePos
	for _, z := range zones {
		if z.ZoneType != "player" {
			continue
		}
		available = append(available, z.Tiles...)
	}
	if len(available) == 0 {
		return nil, ErrNoPlayerSpawnZones
	}
	if len(available) < len(pcIDs) {
		return nil, fmt.Errorf("%w: have %d tiles, need %d", ErrNotEnoughSpawnTiles, len(available), len(pcIDs))
	}

	for i, id := range pcIDs {
		positions[id] = available[i]
	}
	return positions, nil
}

// AssignPCsToOpenTiles is the fallback seater used when a map has no authored
// player spawn zones (e.g. a blank dashboard "New Map"). It seats each PC, in
// input order, into the first open in-bounds tile scanned row-major
// (left-to-right, then top-to-bottom), skipping any tile in occupied — typically
// the monster positions already placed from the encounter template. This
// guarantees every PC gets a valid, distinct grid position so combat tokens
// never fall to the zero-value position (col "", row 0), which the map renderer
// skips, leaving the token invisible.
//
// Returns an error if width or height is non-positive, and ErrNotEnoughSpawnTiles
// if there are fewer open tiles than PCs.
func AssignPCsToOpenTiles(width, height int, pcIDs []string, occupied []TilePos) (map[string]TilePos, error) {
	positions := make(map[string]TilePos, len(pcIDs))
	if len(pcIDs) == 0 {
		return positions, nil
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid map dimensions: %dx%d", width, height)
	}

	blocked := make(map[TilePos]bool, len(occupied))
	for _, t := range occupied {
		blocked[t] = true
	}

	i := 0
	for row := 0; row < height && i < len(pcIDs); row++ {
		for col := 0; col < width && i < len(pcIDs); col++ {
			tile := TilePos{Col: col, Row: row}
			if blocked[tile] {
				continue
			}
			positions[pcIDs[i]] = tile
			i++
		}
	}
	if i < len(pcIDs) {
		return nil, fmt.Errorf("%w: seated %d of %d PCs in open tiles", ErrNotEnoughSpawnTiles, i, len(pcIDs))
	}
	return positions, nil
}
