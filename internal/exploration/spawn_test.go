package exploration_test

import (
	"encoding/json"
	"testing"

	"github.com/ab/dndnd/internal/exploration"
)

// buildTiledWithSpawnZones produces a Tiled-format map JSON with a spawn_zones
// object layer containing one or more zones of the given type.
//
// zones slice entries are [x, y, width, height, zoneType] where x/y/w/h are in
// pixel units (tileSize * tiles). We use tileSize=48.
func buildTiledWithSpawnZones(t *testing.T, tileSize int, width, height int, zones []spawnZone) json.RawMessage {
	t.Helper()

	objs := make([]map[string]any, 0, len(zones))
	for i, z := range zones {
		objs = append(objs, map[string]any{
			"id":     i + 1,
			"x":      float64(z.X * tileSize),
			"y":      float64(z.Y * tileSize),
			"width":  float64(z.W * tileSize),
			"height": float64(z.H * tileSize),
			"type":   z.ZoneType,
		})
	}

	m := map[string]any{
		"width":      width,
		"height":     height,
		"tilewidth":  tileSize,
		"tileheight": tileSize,
		"layers": []map[string]any{
			{
				"name":    "spawn_zones",
				"type":    "objectgroup",
				"objects": objs,
			},
		},
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

type spawnZone struct {
	X, Y, W, H int
	ZoneType   string
}

func TestParseSpawnZones_Empty(t *testing.T) {
	raw := json.RawMessage(`{"width":10,"height":10,"tilewidth":48,"tileheight":48,"layers":[]}`)
	zones, err := exploration.ParseSpawnZones(raw)
	if err != nil {
		t.Fatalf("ParseSpawnZones: %v", err)
	}
	if len(zones) != 0 {
		t.Fatalf("expected 0 zones, got %d", len(zones))
	}
}

func TestParseSpawnZones_PlayerZoneTiles(t *testing.T) {
	raw := buildTiledWithSpawnZones(t, 48, 10, 10, []spawnZone{
		{X: 2, Y: 3, W: 2, H: 2, ZoneType: "player"},
	})
	zones, err := exploration.ParseSpawnZones(raw)
	if err != nil {
		t.Fatalf("ParseSpawnZones: %v", err)
	}
	if len(zones) != 1 {
		t.Fatalf("expected 1 zone, got %d", len(zones))
	}
	z := zones[0]
	if z.ZoneType != "player" {
		t.Errorf("ZoneType: got %q want player", z.ZoneType)
	}
	if z.TileX != 2 || z.TileY != 3 || z.TileWidth != 2 || z.TileHeight != 2 {
		t.Errorf("tile bounds: %+v", z)
	}
	// Tiles covered: (2,3), (3,3), (2,4), (3,4)
	if len(z.Tiles) != 4 {
		t.Errorf("expected 4 tiles, got %d: %+v", len(z.Tiles), z.Tiles)
	}
}

func TestParseSpawnZones_IgnoresNonSpawnLayers(t *testing.T) {
	raw := json.RawMessage(`{
		"width": 10, "height": 10, "tilewidth": 48, "tileheight": 48,
		"layers": [
			{"name":"walls","type":"objectgroup","objects":[{"x":0,"y":0,"width":48,"height":48}]},
			{"name":"terrain","type":"tilelayer","data":[]}
		]
	}`)
	zones, err := exploration.ParseSpawnZones(raw)
	if err != nil {
		t.Fatalf("ParseSpawnZones: %v", err)
	}
	if len(zones) != 0 {
		t.Fatalf("expected 0 zones, got %d", len(zones))
	}
}

func TestParseSpawnZones_InvalidJSON(t *testing.T) {
	_, err := exploration.ParseSpawnZones(json.RawMessage(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestAssignPCsToSpawnZones_DeterministicOrder(t *testing.T) {
	raw := buildTiledWithSpawnZones(t, 48, 10, 10, []spawnZone{
		{X: 2, Y: 3, W: 2, H: 2, ZoneType: "player"}, // 4 tiles
	})
	zones, err := exploration.ParseSpawnZones(raw)
	if err != nil {
		t.Fatal(err)
	}
	pcIDs := []string{"alpha", "bravo", "charlie"}
	positions, err := exploration.AssignPCsToSpawnZones(zones, pcIDs)
	if err != nil {
		t.Fatalf("AssignPCsToSpawnZones: %v", err)
	}
	if len(positions) != 3 {
		t.Fatalf("got %d positions, want 3", len(positions))
	}
	// Deterministic: tiles are iterated row-major within each zone:
	// (2,3), (3,3), (2,4)
	want := map[string]exploration.TilePos{
		"alpha":   {Col: 2, Row: 3},
		"bravo":   {Col: 3, Row: 3},
		"charlie": {Col: 2, Row: 4},
	}
	for id, pos := range want {
		got, ok := positions[id]
		if !ok {
			t.Errorf("missing assignment for %q", id)
			continue
		}
		if got != pos {
			t.Errorf("%s: got %+v want %+v", id, got, pos)
		}
	}
}

func TestAssignPCsToSpawnZones_NoPlayerZones(t *testing.T) {
	raw := buildTiledWithSpawnZones(t, 48, 10, 10, []spawnZone{
		{X: 1, Y: 1, W: 1, H: 1, ZoneType: "enemy"},
	})
	zones, _ := exploration.ParseSpawnZones(raw)
	_, err := exploration.AssignPCsToSpawnZones(zones, []string{"alpha"})
	if err == nil {
		t.Fatal("expected error when no player zones available")
	}
}

func TestAssignPCsToSpawnZones_NotEnoughTiles(t *testing.T) {
	raw := buildTiledWithSpawnZones(t, 48, 10, 10, []spawnZone{
		{X: 1, Y: 1, W: 1, H: 1, ZoneType: "player"}, // 1 tile
	})
	zones, _ := exploration.ParseSpawnZones(raw)
	_, err := exploration.AssignPCsToSpawnZones(zones, []string{"a", "b"})
	if err == nil {
		t.Fatal("expected error when not enough spawn tiles")
	}
}

func TestAssignPCsToSpawnZones_NoPCs(t *testing.T) {
	raw := buildTiledWithSpawnZones(t, 48, 10, 10, []spawnZone{
		{X: 1, Y: 1, W: 2, H: 2, ZoneType: "player"},
	})
	zones, _ := exploration.ParseSpawnZones(raw)
	positions, err := exploration.AssignPCsToSpawnZones(zones, nil)
	if err != nil {
		t.Fatalf("AssignPCsToSpawnZones with no PCs: %v", err)
	}
	if len(positions) != 0 {
		t.Fatalf("got %d positions, want 0", len(positions))
	}
}
