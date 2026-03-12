import { describe, it, expect } from 'vitest';
import {
  TERRAIN_TYPES,
  terrainByGid,
  generateBlankMap,
  getTerrainData,
  setTerrain,
  getWalls,
  addWall,
  removeWall,
  validateDimensions,
  STANDARD_TILE_SIZE,
  LARGE_TILE_SIZE,
  SOFT_LIMIT,
  HARD_LIMIT,
  LIGHTING_TYPES,
  lightingByGid,
  ELEVATION_MAX,
  setLighting,
  getLightingData,
  setElevation,
  getElevationData,
  addSpawnZone,
  getSpawnZones,
  removeSpawnZone,
  cloneMap,
  extractRegion,
  pasteRegion,
  duplicateMap,
  UndoStack,
} from './mapdata.js';

describe('TERRAIN_TYPES', () => {
  it('has 5 terrain types', () => {
    expect(Object.keys(TERRAIN_TYPES)).toHaveLength(5);
  });

  it('maps GIDs 1-5', () => {
    expect(TERRAIN_TYPES.open_ground.gid).toBe(1);
    expect(TERRAIN_TYPES.difficult_terrain.gid).toBe(2);
    expect(TERRAIN_TYPES.water.gid).toBe(3);
    expect(TERRAIN_TYPES.lava.gid).toBe(4);
    expect(TERRAIN_TYPES.pit.gid).toBe(5);
  });

  it('each has a label and color', () => {
    for (const [key, val] of Object.entries(TERRAIN_TYPES)) {
      expect(val.label).toBeTruthy();
      expect(val.color).toMatch(/^#[0-9a-f]{6}$/i);
    }
  });
});

describe('terrainByGid', () => {
  it('returns terrain info for valid GIDs', () => {
    expect(terrainByGid(1).key).toBe('open_ground');
    expect(terrainByGid(3).key).toBe('water');
    expect(terrainByGid(5).key).toBe('pit');
  });

  it('returns open_ground for unknown GIDs', () => {
    expect(terrainByGid(0).key).toBe('open_ground');
    expect(terrainByGid(99).key).toBe('open_ground');
  });
});

describe('generateBlankMap', () => {
  it('creates a map with correct dimensions', () => {
    const map = generateBlankMap(10, 8);
    expect(map.width).toBe(10);
    expect(map.height).toBe(8);
  });

  it('uses standard tile size for small maps', () => {
    const map = generateBlankMap(50, 50);
    expect(map.tilewidth).toBe(STANDARD_TILE_SIZE);
    expect(map.tileheight).toBe(STANDARD_TILE_SIZE);
  });

  it('uses large tile size for maps exceeding soft limit', () => {
    const map = generateBlankMap(101, 50);
    expect(map.tilewidth).toBe(LARGE_TILE_SIZE);
  });

  it('fills terrain with open ground (GID 1)', () => {
    const map = generateBlankMap(3, 2);
    const data = getTerrainData(map);
    expect(data).toHaveLength(6);
    expect(data.every(v => v === 1)).toBe(true);
  });

  it('has five layers: terrain, walls, lighting, elevation, spawn_zones', () => {
    const map = generateBlankMap(5, 5);
    expect(map.layers).toHaveLength(5);
    expect(map.layers[0].name).toBe('terrain');
    expect(map.layers[0].type).toBe('tilelayer');
    expect(map.layers[1].name).toBe('walls');
    expect(map.layers[1].type).toBe('objectgroup');
    expect(map.layers[2].name).toBe('lighting');
    expect(map.layers[2].type).toBe('tilelayer');
    expect(map.layers[3].name).toBe('elevation');
    expect(map.layers[3].type).toBe('tilelayer');
    expect(map.layers[4].name).toBe('spawn_zones');
    expect(map.layers[4].type).toBe('objectgroup');
  });

  it('has lighting layer filled with 0 (normal)', () => {
    const map = generateBlankMap(3, 2);
    const data = getLightingData(map);
    expect(data).toHaveLength(6);
    expect(data.every(v => v === 0)).toBe(true);
  });

  it('has elevation layer filled with 0 (ground level)', () => {
    const map = generateBlankMap(3, 2);
    const data = getElevationData(map);
    expect(data).toHaveLength(6);
    expect(data.every(v => v === 0)).toBe(true);
  });

  it('has empty spawn_zones layer', () => {
    const map = generateBlankMap(5, 5);
    expect(getSpawnZones(map)).toHaveLength(0);
  });

  it('has a lighting tileset', () => {
    const map = generateBlankMap(5, 5);
    const lightingTileset = map.tilesets.find(ts => ts.name === 'lighting');
    expect(lightingTileset).toBeTruthy();
    expect(lightingTileset.tiles).toHaveLength(5);
  });

  it('has empty walls layer', () => {
    const map = generateBlankMap(5, 5);
    expect(getWalls(map)).toHaveLength(0);
  });

  it('has tilesets for terrain and lighting', () => {
    const map = generateBlankMap(5, 5);
    expect(map.tilesets).toHaveLength(2);
    expect(map.tilesets[0].firstgid).toBe(1);
    expect(map.tilesets[0].name).toBe('terrain');
    expect(map.tilesets[0].tiles).toHaveLength(5);
    expect(map.tilesets[1].name).toBe('lighting');
    expect(map.tilesets[1].tiles).toHaveLength(5);
  });

  it('uses orthogonal orientation', () => {
    const map = generateBlankMap(5, 5);
    expect(map.orientation).toBe('orthogonal');
    expect(map.renderorder).toBe('right-down');
  });
});

describe('setTerrain', () => {
  it('sets terrain at specified position', () => {
    const map = generateBlankMap(5, 5);
    setTerrain(map, 2, 3, 3); // water at (2,3)
    const data = getTerrainData(map);
    expect(data[3 * 5 + 2]).toBe(3);
  });

  it('does not affect other tiles', () => {
    const map = generateBlankMap(3, 3);
    setTerrain(map, 1, 1, 4); // lava at center
    const data = getTerrainData(map);
    expect(data[0]).toBe(1); // top-left still open ground
    expect(data[4]).toBe(4); // center is lava
    expect(data[8]).toBe(1); // bottom-right still open ground
  });

  it('handles out-of-bounds gracefully', () => {
    const map = generateBlankMap(3, 3);
    setTerrain(map, -1, 0, 2);
    setTerrain(map, 0, 99, 2);
    // Should not throw, data should be unchanged
    expect(getTerrainData(map).every(v => v === 1)).toBe(true);
  });

  it('handles missing terrain layer', () => {
    const map = { width: 3, height: 3, layers: [] };
    const result = setTerrain(map, 1, 1, 2);
    expect(result).toBe(map); // returns same object
  });
});

describe('addWall', () => {
  it('adds a horizontal wall', () => {
    const map = generateBlankMap(5, 5);
    addWall(map, 48, 0, 'horizontal');
    const walls = getWalls(map);
    expect(walls).toHaveLength(1);
    expect(walls[0].type).toBe('wall');
    expect(walls[0].width).toBe(48);
    expect(walls[0].height).toBe(0);
  });

  it('adds a vertical wall', () => {
    const map = generateBlankMap(5, 5);
    addWall(map, 0, 48, 'vertical');
    const walls = getWalls(map);
    expect(walls).toHaveLength(1);
    expect(walls[0].width).toBe(0);
    expect(walls[0].height).toBe(48);
  });

  it('can add multiple walls', () => {
    const map = generateBlankMap(5, 5);
    addWall(map, 0, 0, 'horizontal');
    addWall(map, 48, 0, 'vertical');
    addWall(map, 96, 96, 'horizontal');
    expect(getWalls(map)).toHaveLength(3);
  });
});

describe('removeWall', () => {
  it('removes a horizontal wall near click', () => {
    const map = generateBlankMap(5, 5);
    addWall(map, 0, 48, 'horizontal');
    expect(getWalls(map)).toHaveLength(1);
    removeWall(map, 24, 48, 10);
    expect(getWalls(map)).toHaveLength(0);
  });

  it('removes a vertical wall near click', () => {
    const map = generateBlankMap(5, 5);
    addWall(map, 48, 0, 'vertical');
    expect(getWalls(map)).toHaveLength(1);
    removeWall(map, 48, 24, 10);
    expect(getWalls(map)).toHaveLength(0);
  });

  it('does not remove walls far from click', () => {
    const map = generateBlankMap(5, 5);
    addWall(map, 0, 0, 'horizontal');
    removeWall(map, 200, 200, 10);
    expect(getWalls(map)).toHaveLength(1);
  });
});

describe('validateDimensions', () => {
  it('returns null for valid dimensions', () => {
    expect(validateDimensions(1, 1)).toBeNull();
    expect(validateDimensions(100, 100)).toBeNull();
    expect(validateDimensions(200, 200)).toBeNull();
  });

  it('rejects non-positive dimensions', () => {
    expect(validateDimensions(0, 10)).toContain('positive');
    expect(validateDimensions(10, -1)).toContain('positive');
  });

  it('rejects dimensions exceeding hard limit', () => {
    expect(validateDimensions(201, 10)).toContain('hard limit');
    expect(validateDimensions(10, 201)).toContain('hard limit');
  });

  it('rejects non-integer dimensions', () => {
    expect(validateDimensions(1.5, 10)).toContain('integers');
    expect(validateDimensions(10, 2.5)).toContain('integers');
  });
});

describe('constants', () => {
  it('has correct values', () => {
    expect(STANDARD_TILE_SIZE).toBe(48);
    expect(LARGE_TILE_SIZE).toBe(32);
    expect(SOFT_LIMIT).toBe(100);
    expect(HARD_LIMIT).toBe(200);
  });
});

describe('LIGHTING_TYPES', () => {
  it('has 6 lighting types including normal', () => {
    expect(Object.keys(LIGHTING_TYPES)).toHaveLength(6);
  });

  it('has normal type with gid 0', () => {
    expect(LIGHTING_TYPES.normal.gid).toBe(0);
  });

  it('maps GIDs 0-5 for all types', () => {
    const gids = Object.values(LIGHTING_TYPES).map(t => t.gid).sort((a, b) => a - b);
    expect(gids).toEqual([0, 1, 2, 3, 4, 5]);
  });

  it('each has a label and color', () => {
    for (const [key, val] of Object.entries(LIGHTING_TYPES)) {
      expect(val.label).toBeTruthy();
      expect(val.color).toMatch(/^#[0-9a-f]{6}$/i);
    }
  });
});

describe('lightingByGid', () => {
  it('returns lighting info for valid GIDs', () => {
    expect(lightingByGid(0).key).toBe('normal');
    expect(lightingByGid(1).key).toBe('dim_light');
    expect(lightingByGid(3).key).toBe('magical_darkness');
    expect(lightingByGid(5).key).toBe('light_obscurement');
  });

  it('returns normal for unknown GIDs', () => {
    expect(lightingByGid(99).key).toBe('normal');
    expect(lightingByGid(-1).key).toBe('normal');
  });
});

describe('ELEVATION_MAX', () => {
  it('is 10', () => {
    expect(ELEVATION_MAX).toBe(10);
  });
});

describe('setLighting / getLightingData', () => {
  it('sets lighting at specified position', () => {
    const map = generateBlankMap(5, 5);
    setLighting(map, 2, 3, 1); // dim light at (2,3)
    const data = getLightingData(map);
    expect(data[3 * 5 + 2]).toBe(1);
  });

  it('does not affect other tiles', () => {
    const map = generateBlankMap(3, 3);
    setLighting(map, 1, 1, 3); // magical darkness at center
    const data = getLightingData(map);
    expect(data[0]).toBe(0); // top-left still normal
    expect(data[4]).toBe(3); // center is magical darkness
    expect(data[8]).toBe(0); // bottom-right still normal
  });

  it('handles out-of-bounds gracefully', () => {
    const map = generateBlankMap(3, 3);
    setLighting(map, -1, 0, 2);
    setLighting(map, 0, 99, 2);
    expect(getLightingData(map).every(v => v === 0)).toBe(true);
  });

  it('handles missing lighting layer', () => {
    const map = { width: 3, height: 3, layers: [] };
    const result = setLighting(map, 1, 1, 2);
    expect(result).toBe(map);
  });

  it('returns empty array for missing layer', () => {
    const map = { layers: [] };
    expect(getLightingData(map)).toEqual([]);
  });
});

describe('setElevation / getElevationData', () => {
  it('sets elevation at specified position', () => {
    const map = generateBlankMap(5, 5);
    setElevation(map, 2, 3, 5);
    const data = getElevationData(map);
    expect(data[3 * 5 + 2]).toBe(5);
  });

  it('clamps elevation to 0-ELEVATION_MAX', () => {
    const map = generateBlankMap(3, 3);
    setElevation(map, 0, 0, -5);
    setElevation(map, 1, 0, 15);
    const data = getElevationData(map);
    expect(data[0]).toBe(0);
    expect(data[1]).toBe(10);
  });

  it('does not affect other tiles', () => {
    const map = generateBlankMap(3, 3);
    setElevation(map, 1, 1, 7);
    const data = getElevationData(map);
    expect(data[0]).toBe(0);
    expect(data[4]).toBe(7);
    expect(data[8]).toBe(0);
  });

  it('handles out-of-bounds gracefully', () => {
    const map = generateBlankMap(3, 3);
    setElevation(map, -1, 0, 5);
    setElevation(map, 0, 99, 5);
    expect(getElevationData(map).every(v => v === 0)).toBe(true);
  });

  it('handles missing elevation layer', () => {
    const map = { width: 3, height: 3, layers: [] };
    const result = setElevation(map, 1, 1, 5);
    expect(result).toBe(map);
  });

  it('returns empty array for missing layer', () => {
    const map = { layers: [] };
    expect(getElevationData(map)).toEqual([]);
  });
});

describe('addSpawnZone / getSpawnZones / removeSpawnZone', () => {
  it('adds a player spawn zone', () => {
    const map = generateBlankMap(10, 10);
    addSpawnZone(map, 1, 2, 3, 4, 'player');
    const zones = getSpawnZones(map);
    expect(zones).toHaveLength(1);
    expect(zones[0].type).toBe('player');
    expect(zones[0].x).toBe(1 * 48);
    expect(zones[0].y).toBe(2 * 48);
    expect(zones[0].width).toBe(3 * 48);
    expect(zones[0].height).toBe(4 * 48);
  });

  it('adds an enemy spawn zone', () => {
    const map = generateBlankMap(10, 10);
    addSpawnZone(map, 5, 5, 2, 2, 'enemy');
    const zones = getSpawnZones(map);
    expect(zones).toHaveLength(1);
    expect(zones[0].type).toBe('enemy');
  });

  it('rejects invalid zone types', () => {
    const map = generateBlankMap(10, 10);
    addSpawnZone(map, 0, 0, 1, 1, 'invalid');
    expect(getSpawnZones(map)).toHaveLength(0);
  });

  it('can add multiple spawn zones', () => {
    const map = generateBlankMap(10, 10);
    addSpawnZone(map, 0, 0, 2, 2, 'player');
    addSpawnZone(map, 5, 5, 3, 3, 'enemy');
    expect(getSpawnZones(map)).toHaveLength(2);
  });

  it('removes a spawn zone by ID', () => {
    const map = generateBlankMap(10, 10);
    addSpawnZone(map, 0, 0, 2, 2, 'player');
    addSpawnZone(map, 5, 5, 3, 3, 'enemy');
    const zones = getSpawnZones(map);
    const idToRemove = zones[0].id;
    removeSpawnZone(map, idToRemove);
    expect(getSpawnZones(map)).toHaveLength(1);
    expect(getSpawnZones(map)[0].type).toBe('enemy');
  });

  it('does nothing when removing non-existent zone', () => {
    const map = generateBlankMap(10, 10);
    addSpawnZone(map, 0, 0, 2, 2, 'player');
    removeSpawnZone(map, 999);
    expect(getSpawnZones(map)).toHaveLength(1);
  });

  it('handles missing spawn_zones layer', () => {
    const map = { width: 5, height: 5, layers: [], tilewidth: 48 };
    const result = addSpawnZone(map, 0, 0, 1, 1, 'player');
    expect(result).toBe(map);
  });

  it('returns empty array for missing layer', () => {
    const map = { layers: [] };
    expect(getSpawnZones(map)).toEqual([]);
  });

  it('handles removeSpawnZone with missing layer', () => {
    const map = { layers: [] };
    const result = removeSpawnZone(map, 1);
    expect(result).toBe(map);
  });
});

describe('cloneMap', () => {
  it('creates a deep copy of a map', () => {
    const map = generateBlankMap(3, 3);
    setTerrain(map, 1, 1, 3);
    addWall(map, 48, 0, 'horizontal');
    const clone = cloneMap(map);
    expect(clone).toEqual(map);
    expect(clone).not.toBe(map);
  });

  it('deep copies terrain data independently', () => {
    const map = generateBlankMap(3, 3);
    const clone = cloneMap(map);
    setTerrain(clone, 0, 0, 5);
    expect(getTerrainData(map)[0]).toBe(1);
    expect(getTerrainData(clone)[0]).toBe(5);
  });

  it('deep copies walls independently', () => {
    const map = generateBlankMap(3, 3);
    addWall(map, 0, 0, 'horizontal');
    const clone = cloneMap(map);
    addWall(clone, 48, 0, 'vertical');
    expect(getWalls(map)).toHaveLength(1);
    expect(getWalls(clone)).toHaveLength(2);
  });

  it('deep copies lighting data independently', () => {
    const map = generateBlankMap(3, 3);
    setLighting(map, 0, 0, 2);
    const clone = cloneMap(map);
    setLighting(clone, 0, 0, 0);
    expect(getLightingData(map)[0]).toBe(2);
    expect(getLightingData(clone)[0]).toBe(0);
  });

  it('deep copies elevation data independently', () => {
    const map = generateBlankMap(3, 3);
    setElevation(map, 0, 0, 5);
    const clone = cloneMap(map);
    setElevation(clone, 0, 0, 0);
    expect(getElevationData(map)[0]).toBe(5);
    expect(getElevationData(clone)[0]).toBe(0);
  });

  it('deep copies spawn zones independently', () => {
    const map = generateBlankMap(5, 5);
    addSpawnZone(map, 0, 0, 2, 2, 'player');
    const clone = cloneMap(map);
    addSpawnZone(clone, 3, 3, 1, 1, 'enemy');
    expect(getSpawnZones(map)).toHaveLength(1);
    expect(getSpawnZones(clone)).toHaveLength(2);
  });
});

describe('extractRegion', () => {
  it('extracts terrain data for a rectangular region', () => {
    const map = generateBlankMap(5, 5);
    setTerrain(map, 1, 1, 3);
    setTerrain(map, 2, 2, 4);
    const region = extractRegion(map, 1, 1, 2, 2);
    expect(region.width).toBe(2);
    expect(region.height).toBe(2);
    expect(region.terrain).toEqual([3, 1, 1, 4]);
  });

  it('extracts lighting data for a region', () => {
    const map = generateBlankMap(5, 5);
    setLighting(map, 1, 1, 2);
    const region = extractRegion(map, 1, 1, 2, 2);
    expect(region.lighting).toEqual([2, 0, 0, 0]);
  });

  it('extracts elevation data for a region', () => {
    const map = generateBlankMap(5, 5);
    setElevation(map, 2, 2, 7);
    const region = extractRegion(map, 1, 1, 3, 3);
    expect(region.elevation[4]).toBe(7);
  });

  it('extracts walls within the region bounds', () => {
    const map = generateBlankMap(5, 5);
    const ts = map.tilewidth;
    addWall(map, ts, ts, 'horizontal');
    addWall(map, 0, 0, 'horizontal');
    const region = extractRegion(map, 1, 1, 3, 3);
    expect(region.walls).toHaveLength(1);
    expect(region.walls[0].x).toBe(0);
    expect(region.walls[0].y).toBe(0);
  });

  it('extracts spawn zones within the region bounds', () => {
    const map = generateBlankMap(10, 10);
    addSpawnZone(map, 2, 2, 2, 2, 'player');
    addSpawnZone(map, 8, 8, 1, 1, 'enemy');
    const region = extractRegion(map, 1, 1, 5, 5);
    expect(region.spawn_zones).toHaveLength(1);
    expect(region.spawn_zones[0].type).toBe('player');
  });

  it('clips to map bounds when region extends beyond', () => {
    const map = generateBlankMap(3, 3);
    const region = extractRegion(map, 2, 2, 5, 5);
    expect(region.width).toBe(1);
    expect(region.height).toBe(1);
    expect(region.terrain).toHaveLength(1);
  });

  it('returns empty region for out-of-bounds coordinates', () => {
    const map = generateBlankMap(3, 3);
    const region = extractRegion(map, 10, 10, 2, 2);
    expect(region.width).toBe(0);
    expect(region.height).toBe(0);
    expect(region.terrain).toEqual([]);
  });
});

describe('pasteRegion', () => {
  it('pastes terrain data at destination', () => {
    const map = generateBlankMap(5, 5);
    const region = { width: 2, height: 2, terrain: [3, 4, 5, 2], lighting: [0, 0, 0, 0], elevation: [0, 0, 0, 0], walls: [], spawn_zones: [] };
    pasteRegion(map, region, 1, 1);
    const data = getTerrainData(map);
    expect(data[1 * 5 + 1]).toBe(3);
    expect(data[1 * 5 + 2]).toBe(4);
    expect(data[2 * 5 + 1]).toBe(5);
    expect(data[2 * 5 + 2]).toBe(2);
  });

  it('pastes lighting data at destination', () => {
    const map = generateBlankMap(5, 5);
    const region = { width: 2, height: 1, terrain: [1, 1], lighting: [2, 3], elevation: [0, 0], walls: [], spawn_zones: [] };
    pasteRegion(map, region, 0, 0);
    const data = getLightingData(map);
    expect(data[0]).toBe(2);
    expect(data[1]).toBe(3);
  });

  it('pastes elevation data at destination', () => {
    const map = generateBlankMap(5, 5);
    const region = { width: 1, height: 1, terrain: [1], lighting: [0], elevation: [7], walls: [], spawn_zones: [] };
    pasteRegion(map, region, 3, 3);
    expect(getElevationData(map)[3 * 5 + 3]).toBe(7);
  });

  it('clips paste to map bounds', () => {
    const map = generateBlankMap(3, 3);
    const region = { width: 2, height: 2, terrain: [3, 4, 5, 2], lighting: [0, 0, 0, 0], elevation: [0, 0, 0, 0], walls: [], spawn_zones: [] };
    pasteRegion(map, region, 2, 2);
    const data = getTerrainData(map);
    expect(data[2 * 3 + 2]).toBe(3); // only top-left of region fits
    expect(data[0]).toBe(1); // untouched
  });

  it('pastes walls with offset', () => {
    const map = generateBlankMap(5, 5);
    const ts = map.tilewidth;
    const region = { width: 2, height: 2, terrain: [1, 1, 1, 1], lighting: [0, 0, 0, 0], elevation: [0, 0, 0, 0], walls: [{ id: 1, type: 'wall', x: 0, y: 0, width: ts, height: 0, properties: { edge: 'top' } }], spawn_zones: [] };
    pasteRegion(map, region, 2, 3);
    const walls = getWalls(map);
    expect(walls).toHaveLength(1);
    expect(walls[0].x).toBe(2 * ts);
    expect(walls[0].y).toBe(3 * ts);
  });

  it('pastes spawn zones with offset', () => {
    const map = generateBlankMap(10, 10);
    const ts = map.tilewidth;
    const region = { width: 3, height: 3, terrain: new Array(9).fill(1), lighting: new Array(9).fill(0), elevation: new Array(9).fill(0), walls: [], spawn_zones: [{ id: 1, type: 'player', x: 0, y: 0, width: ts * 2, height: ts * 2 }] };
    pasteRegion(map, region, 4, 4);
    const zones = getSpawnZones(map);
    expect(zones).toHaveLength(1);
    expect(zones[0].x).toBe(4 * ts);
    expect(zones[0].y).toBe(4 * ts);
  });

  it('handles empty region gracefully', () => {
    const map = generateBlankMap(3, 3);
    const region = { width: 0, height: 0, terrain: [], lighting: [], elevation: [], walls: [], spawn_zones: [] };
    const result = pasteRegion(map, region, 0, 0);
    expect(result).toBe(map);
  });
});

describe('duplicateMap', () => {
  it('creates an independent deep copy', () => {
    const map = generateBlankMap(3, 3);
    setTerrain(map, 0, 0, 5);
    const dup = duplicateMap(map);
    expect(dup).toEqual(map);
    expect(dup).not.toBe(map);
    setTerrain(dup, 0, 0, 1);
    expect(getTerrainData(map)[0]).toBe(5);
  });
});

describe('UndoStack', () => {
  it('starts empty with no undo/redo', () => {
    const stack = new UndoStack();
    expect(stack.canUndo()).toBe(false);
    expect(stack.canRedo()).toBe(false);
  });

  it('push adds a snapshot that can be undone', () => {
    const stack = new UndoStack();
    stack.push({ state: 1 });
    expect(stack.canUndo()).toBe(true);
    expect(stack.canRedo()).toBe(false);
  });

  it('undo returns previous state and enables redo', () => {
    const stack = new UndoStack();
    const snap1 = { state: 1 };
    stack.push(snap1);
    const current = { state: 2 };
    const result = stack.undo(current);
    expect(result).toEqual(snap1);
    expect(stack.canUndo()).toBe(false);
    expect(stack.canRedo()).toBe(true);
  });

  it('redo returns the next state', () => {
    const stack = new UndoStack();
    stack.push({ state: 1 });
    const current = { state: 2 };
    stack.undo(current);
    const result = stack.redo({ state: 1 });
    expect(result).toEqual(current);
    expect(stack.canUndo()).toBe(true);
    expect(stack.canRedo()).toBe(false);
  });

  it('undo returns null when nothing to undo', () => {
    const stack = new UndoStack();
    expect(stack.undo({ state: 1 })).toBeNull();
  });

  it('redo returns null when nothing to redo', () => {
    const stack = new UndoStack();
    expect(stack.redo({ state: 1 })).toBeNull();
  });

  it('push clears the redo stack', () => {
    const stack = new UndoStack();
    stack.push({ state: 1 });
    stack.undo({ state: 2 });
    expect(stack.canRedo()).toBe(true);
    stack.push({ state: 3 });
    expect(stack.canRedo()).toBe(false);
  });

  it('respects max stack size and drops oldest', () => {
    const stack = new UndoStack(3);
    stack.push({ s: 1 });
    stack.push({ s: 2 });
    stack.push({ s: 3 });
    stack.push({ s: 4 }); // should drop { s: 1 }
    expect(stack.canUndo()).toBe(true);
    // Can undo 3 times, not 4
    stack.undo({ s: 5 }); // returns { s: 4 }
    stack.undo({ s: 4 }); // returns { s: 3 }
    stack.undo({ s: 3 }); // returns { s: 2 }
    expect(stack.canUndo()).toBe(false);
  });

  it('clear empties both stacks', () => {
    const stack = new UndoStack();
    stack.push({ s: 1 });
    stack.push({ s: 2 });
    stack.undo({ s: 3 });
    stack.clear();
    expect(stack.canUndo()).toBe(false);
    expect(stack.canRedo()).toBe(false);
  });

  it('multiple undo/redo cycles work correctly', () => {
    const stack = new UndoStack();
    stack.push({ s: 'a' });
    stack.push({ s: 'b' });
    stack.push({ s: 'c' });

    const r1 = stack.undo({ s: 'd' }); // back to c
    expect(r1).toEqual({ s: 'c' });

    const r2 = stack.undo({ s: 'c' }); // back to b
    expect(r2).toEqual({ s: 'b' });

    const r3 = stack.redo({ s: 'b' }); // forward to c
    expect(r3).toEqual({ s: 'c' });

    const r4 = stack.redo({ s: 'c' }); // forward to d
    expect(r4).toEqual({ s: 'd' });

    expect(stack.canRedo()).toBe(false);
  });

  it('default max size is 50', () => {
    const stack = new UndoStack();
    for (let i = 0; i < 60; i++) {
      stack.push({ i });
    }
    // Should only be able to undo 50 times
    let count = 0;
    while (stack.canUndo()) {
      stack.undo({ current: true });
      count++;
    }
    expect(count).toBe(50);
  });
});

describe('extractRegion + pasteRegion roundtrip', () => {
  it('extract then paste preserves data', () => {
    const map = generateBlankMap(5, 5);
    setTerrain(map, 1, 1, 3);
    setTerrain(map, 2, 1, 4);
    setLighting(map, 1, 1, 2);
    setElevation(map, 2, 2, 5);
    const region = extractRegion(map, 1, 1, 2, 2);

    const target = generateBlankMap(5, 5);
    pasteRegion(target, region, 3, 3);
    expect(getTerrainData(target)[3 * 5 + 3]).toBe(3);
    expect(getTerrainData(target)[3 * 5 + 4]).toBe(4);
    expect(getLightingData(target)[3 * 5 + 3]).toBe(2);
    expect(getElevationData(target)[4 * 5 + 4]).toBe(5);
  });

  it('extractRegion with negative start gets clipped to 0', () => {
    const map = generateBlankMap(3, 3);
    const region = extractRegion(map, -1, -1, 2, 2);
    // clippedX=0, clippedY=0, width stays 2, height stays 2
    expect(region.width).toBe(2);
    expect(region.height).toBe(2);
  });

  it('pasteRegion with missing layers does not crash', () => {
    const map = { width: 3, height: 3, layers: [], tilewidth: 48 };
    const region = { width: 1, height: 1, terrain: [3], lighting: [0], elevation: [0], walls: [], spawn_zones: [] };
    const result = pasteRegion(map, region, 0, 0);
    expect(result).toBe(map);
  });
});
