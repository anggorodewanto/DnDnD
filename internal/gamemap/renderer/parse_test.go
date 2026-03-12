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
