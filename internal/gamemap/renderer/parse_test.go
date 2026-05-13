package renderer

import (
	"encoding/json"
	"testing"
)

func TestParseTiledJSON_BasicTerrain(t *testing.T) {
	tiledJSON := `{
		"width": 3,
		"height": 2,
		"tilewidth": 48,
		"tileheight": 48,
		"layers": [
			{
				"name": "terrain",
				"type": "tilelayer",
				"width": 3,
				"height": 2,
				"data": [1, 2, 3, 4, 5, 1]
			}
		],
		"tilesets": [
			{
				"firstgid": 1,
				"name": "terrain",
				"tiles": [
					{"id": 0, "type": "open_ground"},
					{"id": 1, "type": "difficult_terrain"},
					{"id": 2, "type": "water"},
					{"id": 3, "type": "lava"},
					{"id": 4, "type": "pit"}
				]
			}
		]
	}`

	md, err := ParseTiledJSON(json.RawMessage(tiledJSON), nil, nil)
	if err != nil {
		t.Fatalf("ParseTiledJSON error: %v", err)
	}

	if md.Width != 3 || md.Height != 2 {
		t.Errorf("dimensions = %dx%d, want 3x2", md.Width, md.Height)
	}
	if md.TileSize != 48 {
		t.Errorf("tileSize = %d, want 48", md.TileSize)
	}
	if len(md.TerrainGrid) != 6 {
		t.Fatalf("terrain grid len = %d, want 6", len(md.TerrainGrid))
	}

	// GID 1 -> tile 0 (open_ground), GID 2 -> tile 1 (difficult), etc.
	expected := []TerrainType{
		TerrainOpenGround, TerrainDifficultTerrain, TerrainWater,
		TerrainLava, TerrainPit, TerrainOpenGround,
	}
	for i, want := range expected {
		if md.TerrainGrid[i] != want {
			t.Errorf("terrain[%d] = %v, want %v", i, md.TerrainGrid[i], want)
		}
	}
}

func TestParseTiledJSON_Walls(t *testing.T) {
	tiledJSON := `{
		"width": 5,
		"height": 5,
		"tilewidth": 48,
		"tileheight": 48,
		"layers": [
			{
				"name": "terrain",
				"type": "tilelayer",
				"width": 5,
				"height": 5,
				"data": [1,1,1,1,1, 1,1,1,1,1, 1,1,1,1,1, 1,1,1,1,1, 1,1,1,1,1]
			},
			{
				"name": "walls",
				"type": "objectgroup",
				"objects": [
					{"x": 0, "y": 0, "width": 240, "height": 0},
					{"x": 48, "y": 48, "width": 0, "height": 96}
				]
			}
		],
		"tilesets": [{"firstgid": 1, "name": "terrain", "tiles": [{"id":0,"type":"open_ground"}]}]
	}`

	md, err := ParseTiledJSON(json.RawMessage(tiledJSON), nil, nil)
	if err != nil {
		t.Fatalf("ParseTiledJSON error: %v", err)
	}

	if len(md.Walls) != 2 {
		t.Fatalf("walls count = %d, want 2", len(md.Walls))
	}

	// First wall: x=0,y=0 to x=240,y=0 -> in tile coords: 0,0 to 5,0
	w0 := md.Walls[0]
	if w0.X1 != 0 || w0.Y1 != 0 || w0.X2 != 5 || w0.Y2 != 0 {
		t.Errorf("wall[0] = %v, want (0,0)-(5,0)", w0)
	}

	// Second wall: x=48,y=48 to x=48,y=144 -> in tile coords: 1,1 to 1,3
	w1 := md.Walls[1]
	if w1.X1 != 1 || w1.Y1 != 1 || w1.X2 != 1 || w1.Y2 != 3 {
		t.Errorf("wall[1] = %v, want (1,1)-(1,3)", w1)
	}
}

func TestParseTiledJSON_WithCombatants(t *testing.T) {
	tiledJSON := `{
		"width": 3,
		"height": 3,
		"tilewidth": 48,
		"tileheight": 48,
		"layers": [
			{"name": "terrain", "type": "tilelayer", "width": 3, "height": 3, "data": [1,1,1,1,1,1,1,1,1]}
		],
		"tilesets": [{"firstgid": 1, "name": "terrain", "tiles": [{"id":0,"type":"open_ground"}]}]
	}`

	combatants := []Combatant{
		{ShortID: "G1", Col: 1, Row: 2, HPMax: 10, HPCurrent: 5},
	}

	md, err := ParseTiledJSON(json.RawMessage(tiledJSON), combatants, nil)
	if err != nil {
		t.Fatalf("ParseTiledJSON error: %v", err)
	}

	if len(md.Combatants) != 1 {
		t.Fatalf("combatants = %d, want 1", len(md.Combatants))
	}
	if md.Combatants[0].ShortID != "G1" {
		t.Errorf("combatant short_id = %q, want %q", md.Combatants[0].ShortID, "G1")
	}
}

func TestParseTiledJSON_InvalidJSON(t *testing.T) {
	_, err := ParseTiledJSON(json.RawMessage(`{invalid`), nil, nil)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseTiledJSON_NoTerrainLayer(t *testing.T) {
	tiledJSON := `{
		"width": 2, "height": 2, "tilewidth": 48, "tileheight": 48,
		"layers": [],
		"tilesets": [{"firstgid": 1, "name": "terrain", "tiles": [{"id":0,"type":"open_ground"}]}]
	}`

	md, err := ParseTiledJSON(json.RawMessage(tiledJSON), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should default to all open ground
	if len(md.TerrainGrid) != 4 {
		t.Errorf("terrain grid len = %d, want 4", len(md.TerrainGrid))
	}
	for i, g := range md.TerrainGrid {
		if g != TerrainOpenGround {
			t.Errorf("terrain[%d] = %v, want open ground", i, g)
		}
	}
}

// SR-029: lighting layer with magical_darkness tiles populates MagicalDarknessTiles.
// This variant exercises the "raw LIGHTING_TYPES.gid" path that the Svelte editor
// actually writes today (lighting layer data carries small canonical GIDs that do
// not match the lighting tileset's firstgid offset — SR-030 drift).
func TestParseTiledJSON_LightingMagicalDarkness_RawGID(t *testing.T) {
	// 3x3 lighting layer; (col=1,row=1) is magical_darkness (GID 3 in
	// LIGHTING_TYPES). Other tiles are normal (GID 0).
	tiledJSON := `{
		"width": 3, "height": 3, "tilewidth": 48, "tileheight": 48,
		"layers": [
			{"name": "terrain", "type": "tilelayer", "width": 3, "height": 3, "data": [1,1,1,1,1,1,1,1,1]},
			{"name": "lighting", "type": "tilelayer", "width": 3, "height": 3, "data": [0,0,0, 0,3,0, 0,0,0]}
		],
		"tilesets": [{"firstgid": 1, "name": "terrain", "tiles": [{"id":0,"type":"open_ground"}]}]
	}`

	md, err := ParseTiledJSON(json.RawMessage(tiledJSON), nil, nil)
	if err != nil {
		t.Fatalf("ParseTiledJSON error: %v", err)
	}

	if len(md.MagicalDarknessTiles) != 1 {
		t.Fatalf("MagicalDarknessTiles len = %d, want 1; got %v", len(md.MagicalDarknessTiles), md.MagicalDarknessTiles)
	}
	got := md.MagicalDarknessTiles[0]
	if got.Col != 1 || got.Row != 1 {
		t.Errorf("MagicalDarknessTiles[0] = %+v, want {Col:1 Row:1}", got)
	}
}

// SR-029: lighting layer resolves magical_darkness via tileset metadata
// (the Tiled-correct path: firstgid + tile.id with tile.type).
func TestParseTiledJSON_LightingMagicalDarkness_TilesetType(t *testing.T) {
	// Lighting tileset firstgid=7 — tile id=2 is magical_darkness, so its
	// resolved GID is 9. Place 9 at (2,0).
	tiledJSON := `{
		"width": 3, "height": 2, "tilewidth": 48, "tileheight": 48,
		"layers": [
			{"name": "terrain", "type": "tilelayer", "width": 3, "height": 2, "data": [1,1,1,1,1,1]},
			{"name": "lighting", "type": "tilelayer", "width": 3, "height": 2, "data": [0,0,9, 0,0,0]}
		],
		"tilesets": [
			{"firstgid": 1, "name": "terrain", "tiles": [{"id":0,"type":"open_ground"}]},
			{"firstgid": 7, "name": "lighting", "tiles": [
				{"id": 0, "type": "dim_light"},
				{"id": 1, "type": "darkness"},
				{"id": 2, "type": "magical_darkness"},
				{"id": 3, "type": "fog"},
				{"id": 4, "type": "light_obscurement"}
			]}
		]
	}`

	md, err := ParseTiledJSON(json.RawMessage(tiledJSON), nil, nil)
	if err != nil {
		t.Fatalf("ParseTiledJSON error: %v", err)
	}

	if len(md.MagicalDarknessTiles) != 1 {
		t.Fatalf("MagicalDarknessTiles len = %d, want 1", len(md.MagicalDarknessTiles))
	}
	got := md.MagicalDarknessTiles[0]
	if got.Col != 2 || got.Row != 0 {
		t.Errorf("MagicalDarknessTiles[0] = %+v, want {Col:2 Row:0}", got)
	}
}

// SR-029: elevation layer populates MapData.ElevationByTile in feet, row-major.
func TestParseTiledJSON_Elevation(t *testing.T) {
	tiledJSON := `{
		"width": 3, "height": 3, "tilewidth": 48, "tileheight": 48,
		"layers": [
			{"name": "terrain", "type": "tilelayer", "width": 3, "height": 3, "data": [1,1,1,1,1,1,1,1,1]},
			{"name": "elevation", "type": "tilelayer", "width": 3, "height": 3, "data": [0,0,3, 0,5,0, 0,0,0]}
		],
		"tilesets": [{"firstgid": 1, "name": "terrain", "tiles": [{"id":0,"type":"open_ground"}]}]
	}`

	md, err := ParseTiledJSON(json.RawMessage(tiledJSON), nil, nil)
	if err != nil {
		t.Fatalf("ParseTiledJSON error: %v", err)
	}

	want := []int{0, 0, 3, 0, 5, 0, 0, 0, 0}
	if len(md.ElevationByTile) != len(want) {
		t.Fatalf("ElevationByTile len = %d, want %d", len(md.ElevationByTile), len(want))
	}
	for i, w := range want {
		if md.ElevationByTile[i] != w {
			t.Errorf("ElevationByTile[%d] = %d, want %d", i, md.ElevationByTile[i], w)
		}
	}
}

// SR-029: spawn_zones objectgroup populates MapData.SpawnZones with tile-unit coords.
func TestParseTiledJSON_SpawnZones(t *testing.T) {
	// 5x5 map, tilewidth=48. Player zone at tile (0,0) 2x2 -> pixel (0,0,96,96).
	// Enemy zone at tile (3,3) 2x2 -> pixel (144,144,96,96).
	tiledJSON := `{
		"width": 5, "height": 5, "tilewidth": 48, "tileheight": 48,
		"layers": [
			{"name": "terrain", "type": "tilelayer", "width": 5, "height": 5, "data": [1,1,1,1,1, 1,1,1,1,1, 1,1,1,1,1, 1,1,1,1,1, 1,1,1,1,1]},
			{"name": "spawn_zones", "type": "objectgroup", "objects": [
				{"id": 1, "type": "player", "x": 0,   "y": 0,   "width": 96, "height": 96},
				{"id": 2, "type": "enemy",  "x": 144, "y": 144, "width": 96, "height": 96}
			]}
		],
		"tilesets": [{"firstgid": 1, "name": "terrain", "tiles": [{"id":0,"type":"open_ground"}]}]
	}`

	md, err := ParseTiledJSON(json.RawMessage(tiledJSON), nil, nil)
	if err != nil {
		t.Fatalf("ParseTiledJSON error: %v", err)
	}

	if len(md.SpawnZones) != 2 {
		t.Fatalf("SpawnZones len = %d, want 2", len(md.SpawnZones))
	}
	p := md.SpawnZones[0]
	if p.ZoneType != "player" || p.TileX != 0 || p.TileY != 0 || p.TileWidth != 2 || p.TileHeight != 2 {
		t.Errorf("SpawnZones[0] = %+v, want player at (0,0) 2x2", p)
	}
	e := md.SpawnZones[1]
	if e.ZoneType != "enemy" || e.TileX != 3 || e.TileY != 3 || e.TileWidth != 2 || e.TileHeight != 2 {
		t.Errorf("SpawnZones[1] = %+v, want enemy at (3,3) 2x2", e)
	}
}

// SR-029 acceptance criterion: a map with a magical-darkness tile painted in
// the lighting layer demotes FoW correctly when parsed. End-to-end check that
// MagicalDarknessTiles flows from ParseTiledJSON -> ComputeVisibilityWithZones.
func TestParseTiledJSON_MagicalDarkness_DemotesFoW(t *testing.T) {
	// 5x1 corridor: viewer at col 0 with darkvision range 4. Magical darkness
	// at col 3. Darkvision should NOT reach col 3 (demoted to Unexplored).
	tiledJSON := `{
		"width": 5, "height": 1, "tilewidth": 48, "tileheight": 48,
		"layers": [
			{"name": "terrain", "type": "tilelayer", "width": 5, "height": 1, "data": [1,1,1,1,1]},
			{"name": "lighting", "type": "tilelayer", "width": 5, "height": 1, "data": [0,0,0,3,0]}
		],
		"tilesets": [{"firstgid": 1, "name": "terrain", "tiles": [{"id":0,"type":"open_ground"}]}]
	}`

	md, err := ParseTiledJSON(json.RawMessage(tiledJSON), nil, nil)
	if err != nil {
		t.Fatalf("ParseTiledJSON error: %v", err)
	}

	// Viewer with darkvision only (no Devil's Sight) → magical darkness blocks it.
	sources := []VisionSource{
		{Col: 0, Row: 0, RangeTiles: 0, DarkvisionTiles: 4},
	}
	fow := ComputeVisibilityWithZones(sources, nil, md.Walls, md.MagicalDarknessTiles, md.Width, md.Height)

	if got := fow.StateAt(2, 0); got != Visible {
		t.Errorf("tile col=2 = %v, want Visible (in darkvision range, no darkness)", got)
	}
	if got := fow.StateAt(3, 0); got != Unexplored {
		t.Errorf("tile col=3 (magical darkness) = %v, want Unexplored (darkvision demoted)", got)
	}
}

func TestParseTiledJSON_WithActiveEffects(t *testing.T) {
	tiledJSON := `{
		"width": 3, "height": 3, "tilewidth": 48, "tileheight": 48,
		"layers": [{"name": "terrain", "type": "tilelayer", "width": 3, "height": 3, "data": [1,1,1,1,1,1,1,1,1]}],
		"tilesets": [{"firstgid": 1, "name": "terrain", "tiles": [{"id":0,"type":"open_ground"}]}]
	}`

	effects := []ActiveEffect{
		{Symbol: "\u2601", Name: "Fog Cloud", CasterName: "Kael", Area: "20ft radius", Rounds: 8},
	}

	md, err := ParseTiledJSON(json.RawMessage(tiledJSON), nil, effects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(md.ActiveEffects) != 1 {
		t.Errorf("effects = %d, want 1", len(md.ActiveEffects))
	}
}
