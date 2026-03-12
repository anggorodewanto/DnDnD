/**
 * Terrain type GID mapping (1-indexed for Tiled compatibility).
 * GID 0 means "no tile" in Tiled format.
 */
export const TERRAIN_TYPES = {
  open_ground: { gid: 1, label: 'Open Ground', color: '#c8b88a' },
  difficult_terrain: { gid: 2, label: 'Difficult Terrain', color: '#8b7355' },
  water: { gid: 3, label: 'Water', color: '#4a90d9' },
  lava: { gid: 4, label: 'Lava', color: '#ff4500' },
  pit: { gid: 5, label: 'Pit', color: '#2d2d2d' },
};

/** Get terrain info by GID. */
export function terrainByGid(gid) {
  for (const [key, val] of Object.entries(TERRAIN_TYPES)) {
    if (val.gid === gid) {
      return { key, ...val };
    }
  }
  return { key: 'open_ground', ...TERRAIN_TYPES.open_ground };
}

/** Standard tile size in pixels. */
export const STANDARD_TILE_SIZE = 48;
/** Large map tile size in pixels. */
export const LARGE_TILE_SIZE = 32;
/** Soft limit for map dimensions. */
export const SOFT_LIMIT = 100;
/** Hard limit for map dimensions. */
export const HARD_LIMIT = 200;

/**
 * Generate a blank Tiled-compatible JSON map.
 * @param {number} width - Width in squares.
 * @param {number} height - Height in squares.
 * @returns {object} Tiled-compatible map object.
 */
export function generateBlankMap(width, height) {
  const tileSize = (width > SOFT_LIMIT || height > SOFT_LIMIT) ? LARGE_TILE_SIZE : STANDARD_TILE_SIZE;
  const data = new Array(width * height).fill(1); // All open ground

  return {
    width,
    height,
    tilewidth: tileSize,
    tileheight: tileSize,
    orientation: 'orthogonal',
    renderorder: 'right-down',
    layers: [
      {
        name: 'terrain',
        type: 'tilelayer',
        width,
        height,
        data,
        visible: true,
        opacity: 1,
      },
      {
        name: 'walls',
        type: 'objectgroup',
        objects: [],
        visible: true,
        opacity: 1,
      },
    ],
    tilesets: [
      {
        firstgid: 1,
        name: 'terrain',
        tilecount: 6,
        tiles: [
          { id: 0, type: 'open_ground' },
          { id: 1, type: 'difficult_terrain' },
          { id: 2, type: 'water' },
          { id: 3, type: 'lava' },
          { id: 4, type: 'pit' },
        ],
      },
    ],
  };
}

/**
 * Get the terrain layer data array from a Tiled map.
 * @param {object} tiledMap - The Tiled-compatible map object.
 * @returns {number[]} The terrain data array (GIDs).
 */
export function getTerrainData(tiledMap) {
  const layer = tiledMap.layers?.find(l => l.name === 'terrain' && l.type === 'tilelayer');
  return layer?.data || [];
}

/**
 * Set terrain at a specific tile position.
 * @param {object} tiledMap - The Tiled-compatible map object.
 * @param {number} x - Tile x coordinate.
 * @param {number} y - Tile y coordinate.
 * @param {number} gid - Terrain GID to set.
 * @returns {object} Updated map (mutates in place for performance).
 */
export function setTerrain(tiledMap, x, y, gid) {
  const layer = tiledMap.layers?.find(l => l.name === 'terrain' && l.type === 'tilelayer');
  if (!layer) return tiledMap;

  const idx = y * tiledMap.width + x;
  if (idx >= 0 && idx < layer.data.length) {
    layer.data[idx] = gid;
  }
  return tiledMap;
}

/**
 * Get the walls object layer from a Tiled map.
 * @param {object} tiledMap - The Tiled-compatible map object.
 * @returns {object[]} The wall objects array.
 */
export function getWalls(tiledMap) {
  const layer = tiledMap.layers?.find(l => l.name === 'walls' && l.type === 'objectgroup');
  return layer?.objects || [];
}

/**
 * Add a wall to the map.
 * Walls are placed along tile edges.
 * @param {object} tiledMap - The Tiled-compatible map object.
 * @param {number} x - Pixel x position of the wall.
 * @param {number} y - Pixel y position of the wall.
 * @param {'horizontal'|'vertical'} orientation - Wall orientation.
 * @returns {object} Updated map.
 */
export function addWall(tiledMap, x, y, orientation) {
  const layer = tiledMap.layers?.find(l => l.name === 'walls' && l.type === 'objectgroup');
  if (!layer) return tiledMap;

  const tileSize = tiledMap.tilewidth;
  const wall = {
    id: layer.objects.length + 1,
    type: 'wall',
    x,
    y,
    width: orientation === 'horizontal' ? tileSize : 0,
    height: orientation === 'vertical' ? tileSize : 0,
    properties: {
      edge: orientation === 'horizontal' ? 'top' : 'left',
    },
  };

  layer.objects.push(wall);
  return tiledMap;
}

/**
 * Remove a wall from the map by checking proximity to a click position.
 * @param {object} tiledMap - The Tiled-compatible map object.
 * @param {number} px - Pixel x of click.
 * @param {number} py - Pixel y of click.
 * @param {number} threshold - Max distance in pixels to match.
 * @returns {object} Updated map.
 */
export function removeWall(tiledMap, px, py, threshold) {
  const layer = tiledMap.layers?.find(l => l.name === 'walls' && l.type === 'objectgroup');
  if (!layer) return tiledMap;

  layer.objects = layer.objects.filter(wall => {
    const wx = wall.x;
    const wy = wall.y;
    const ww = wall.width || 0;
    const wh = wall.height || 0;

    // Distance from point to line segment
    if (ww > 0) {
      // Horizontal wall
      if (px >= wx && px <= wx + ww && Math.abs(py - wy) <= threshold) {
        return false;
      }
    } else if (wh > 0) {
      // Vertical wall
      if (py >= wy && py <= wy + wh && Math.abs(px - wx) <= threshold) {
        return false;
      }
    }
    return true;
  });

  return tiledMap;
}

/**
 * Validate map dimensions.
 * @param {number} width
 * @param {number} height
 * @returns {string|null} Error message or null if valid.
 */
export function validateDimensions(width, height) {
  if (!Number.isInteger(width) || !Number.isInteger(height)) {
    return 'Dimensions must be integers';
  }
  if (width < 1 || height < 1) {
    return 'Dimensions must be positive';
  }
  if (width > HARD_LIMIT || height > HARD_LIMIT) {
    return `Dimensions exceed hard limit of ${HARD_LIMIT}x${HARD_LIMIT}`;
  }
  return null;
}
