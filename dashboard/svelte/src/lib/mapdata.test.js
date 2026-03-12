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

  it('has two layers: terrain and walls', () => {
    const map = generateBlankMap(5, 5);
    expect(map.layers).toHaveLength(2);
    expect(map.layers[0].name).toBe('terrain');
    expect(map.layers[0].type).toBe('tilelayer');
    expect(map.layers[1].name).toBe('walls');
    expect(map.layers[1].type).toBe('objectgroup');
  });

  it('has empty walls layer', () => {
    const map = generateBlankMap(5, 5);
    expect(getWalls(map)).toHaveLength(0);
  });

  it('has a tileset with 5 terrain tiles', () => {
    const map = generateBlankMap(5, 5);
    expect(map.tilesets).toHaveLength(1);
    expect(map.tilesets[0].firstgid).toBe(1);
    expect(map.tilesets[0].tiles).toHaveLength(5);
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
