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
