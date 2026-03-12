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

/**
 * Lighting type GID mapping.
 * GID 0 means "normal" (no lighting override).
 */
export const LIGHTING_TYPES = {
  normal:             { gid: 0, label: 'Normal',             color: '#000000' },
  dim_light:          { gid: 1, label: 'Dim Light',          color: '#ffdd57' },
  darkness:           { gid: 2, label: 'Darkness',           color: '#333333' },
  magical_darkness:   { gid: 3, label: 'Magical Darkness',   color: '#1a0033' },
  fog:                { gid: 4, label: 'Fog / Obscurement',  color: '#aabbcc' },
  light_obscurement:  { gid: 5, label: 'Light Obscurement',  color: '#ccddaa' },
};

/**
 * Look up an entry by GID in a type map, returning the matching key and value.
 * Falls back to the given default key if no match is found.
 */
function lookupByGid(typeMap, gid, defaultKey) {
  for (const [key, val] of Object.entries(typeMap)) {
    if (val.gid === gid) {
      return { key, ...val };
    }
  }
  return { key: defaultKey, ...typeMap[defaultKey] };
}

/** Get lighting info by GID. */
export function lightingByGid(gid) {
  return lookupByGid(LIGHTING_TYPES, gid, 'normal');
}

/** Get terrain info by GID. */
export function terrainByGid(gid) {
  return lookupByGid(TERRAIN_TYPES, gid, 'open_ground');
}

/** Find a layer by name and type in a Tiled map. */
function findLayer(tiledMap, name, type) {
  return tiledMap.layers?.find(l => l.name === name && l.type === type);
}

/** Maximum elevation level. */
export const ELEVATION_MAX = 10;

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
  const tileCount = width * height;
  const data = new Array(tileCount).fill(1); // All open ground
  const lightingData = new Array(tileCount).fill(0); // No lighting override
  const elevationData = new Array(tileCount).fill(0); // Ground level

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
      {
        name: 'lighting',
        type: 'tilelayer',
        width,
        height,
        data: lightingData,
        visible: true,
        opacity: 1,
      },
      {
        name: 'elevation',
        type: 'tilelayer',
        width,
        height,
        data: elevationData,
        visible: true,
        opacity: 1,
      },
      {
        name: 'spawn_zones',
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
      {
        firstgid: 7,
        name: 'lighting',
        tilecount: 6,
        tiles: [
          { id: 0, type: 'dim_light' },
          { id: 1, type: 'darkness' },
          { id: 2, type: 'magical_darkness' },
          { id: 3, type: 'fog' },
          { id: 4, type: 'light_obscurement' },
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
  return findLayer(tiledMap, 'terrain', 'tilelayer')?.data || [];
}

/**
 * Set a value in a tile layer at a specific position.
 * Shared implementation for terrain, lighting, and elevation setters.
 */
function setTileValue(tiledMap, layerName, x, y, value) {
  const layer = findLayer(tiledMap, layerName, 'tilelayer');
  if (!layer) return tiledMap;

  const idx = y * tiledMap.width + x;
  if (idx >= 0 && idx < layer.data.length) {
    layer.data[idx] = value;
  }
  return tiledMap;
}

/**
 * Get the objects array from a named object group layer.
 */
function getObjectGroupObjects(tiledMap, layerName) {
  return findLayer(tiledMap, layerName, 'objectgroup')?.objects || [];
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
  return setTileValue(tiledMap, 'terrain', x, y, gid);
}

/**
 * Get the walls object layer from a Tiled map.
 * @param {object} tiledMap - The Tiled-compatible map object.
 * @returns {object[]} The wall objects array.
 */
export function getWalls(tiledMap) {
  return getObjectGroupObjects(tiledMap, 'walls');
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
  const layer = findLayer(tiledMap, 'walls', 'objectgroup');
  if (!layer) return tiledMap;

  const tileSize = tiledMap.tilewidth;
  const isHorizontal = orientation === 'horizontal';
  const wall = {
    id: layer.objects.length + 1,
    type: 'wall',
    x,
    y,
    width: isHorizontal ? tileSize : 0,
    height: isHorizontal ? 0 : tileSize,
    properties: {
      edge: isHorizontal ? 'top' : 'left',
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
  const layer = findLayer(tiledMap, 'walls', 'objectgroup');
  if (!layer) return tiledMap;

  layer.objects = layer.objects.filter(wall => {
    const ww = wall.width || 0;
    const wh = wall.height || 0;

    if (ww > 0) {
      if (px >= wall.x && px <= wall.x + ww && Math.abs(py - wall.y) <= threshold) {
        return false;
      }
    } else if (wh > 0) {
      if (py >= wall.y && py <= wall.y + wh && Math.abs(px - wall.x) <= threshold) {
        return false;
      }
    }
    return true;
  });

  return tiledMap;
}

/**
 * Get the lighting layer data array from a Tiled map.
 * @param {object} tiledMap - The Tiled-compatible map object.
 * @returns {number[]} The lighting data array (GIDs).
 */
export function getLightingData(tiledMap) {
  return findLayer(tiledMap, 'lighting', 'tilelayer')?.data || [];
}

/**
 * Set lighting at a specific tile position.
 * @param {object} tiledMap - The Tiled-compatible map object.
 * @param {number} x - Tile x coordinate.
 * @param {number} y - Tile y coordinate.
 * @param {number} gid - Lighting GID to set (0 = normal).
 * @returns {object} Updated map (mutates in place).
 */
export function setLighting(tiledMap, x, y, gid) {
  return setTileValue(tiledMap, 'lighting', x, y, gid);
}

/**
 * Get the elevation layer data array from a Tiled map.
 * @param {object} tiledMap - The Tiled-compatible map object.
 * @returns {number[]} The elevation data array.
 */
export function getElevationData(tiledMap) {
  return findLayer(tiledMap, 'elevation', 'tilelayer')?.data || [];
}

/**
 * Set elevation at a specific tile position.
 * @param {object} tiledMap - The Tiled-compatible map object.
 * @param {number} x - Tile x coordinate.
 * @param {number} y - Tile y coordinate.
 * @param {number} level - Elevation level (0-ELEVATION_MAX).
 * @returns {object} Updated map (mutates in place).
 */
export function setElevation(tiledMap, x, y, level) {
  const clamped = Math.max(0, Math.min(ELEVATION_MAX, level));
  return setTileValue(tiledMap, 'elevation', x, y, clamped);
}

/**
 * Get spawn zone objects from the map.
 * @param {object} tiledMap - The Tiled-compatible map object.
 * @returns {object[]} The spawn zone objects array.
 */
export function getSpawnZones(tiledMap) {
  return getObjectGroupObjects(tiledMap, 'spawn_zones');
}

/**
 * Add a spawn zone rectangle to the map.
 * @param {object} tiledMap - The Tiled-compatible map object.
 * @param {number} x - Tile x coordinate.
 * @param {number} y - Tile y coordinate.
 * @param {number} width - Width in tiles.
 * @param {number} height - Height in tiles.
 * @param {'player'|'enemy'} zoneType - Spawn zone type.
 * @returns {object} Updated map.
 */
export function addSpawnZone(tiledMap, x, y, width, height, zoneType) {
  const layer = findLayer(tiledMap, 'spawn_zones', 'objectgroup');
  if (!layer) return tiledMap;
  if (zoneType !== 'player' && zoneType !== 'enemy') return tiledMap;

  const tileSize = tiledMap.tilewidth;
  const zone = {
    id: layer.objects.length + 1,
    type: zoneType,
    x: x * tileSize,
    y: y * tileSize,
    width: width * tileSize,
    height: height * tileSize,
  };

  layer.objects.push(zone);
  return tiledMap;
}

/**
 * Remove a spawn zone by ID.
 * @param {object} tiledMap - The Tiled-compatible map object.
 * @param {number} zoneId - The zone ID to remove.
 * @returns {object} Updated map.
 */
export function removeSpawnZone(tiledMap, zoneId) {
  const layer = findLayer(tiledMap, 'spawn_zones', 'objectgroup');
  if (!layer) return tiledMap;

  layer.objects = layer.objects.filter(z => z.id !== zoneId);
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
