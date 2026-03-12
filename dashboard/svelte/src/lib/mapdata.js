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
 * Create a deep copy of a Tiled-compatible map.
 * @param {object} map - The map to clone.
 * @returns {object} A deep copy of the map.
 */
export function cloneMap(map) {
  return JSON.parse(JSON.stringify(map));
}

/**
 * Extract all layer data for a rectangular region of the map.
 * @param {object} map - The Tiled-compatible map object.
 * @param {number} x - Region start tile x.
 * @param {number} y - Region start tile y.
 * @param {number} width - Region width in tiles.
 * @param {number} height - Region height in tiles.
 * @returns {object} Region data with terrain, lighting, elevation, walls, spawn_zones.
 */
export function extractRegion(map, x, y, width, height) {
  // Clip to map bounds
  const clippedX = Math.max(0, Math.min(x, map.width));
  const clippedY = Math.max(0, Math.min(y, map.height));
  const clippedW = Math.max(0, Math.min(width, map.width - clippedX));
  const clippedH = Math.max(0, Math.min(height, map.height - clippedY));

  if (clippedW === 0 || clippedH === 0) {
    return { width: 0, height: 0, terrain: [], lighting: [], elevation: [], walls: [], spawn_zones: [] };
  }

  const terrainData = getTerrainData(map);
  const lightingData = getLightingData(map);
  const elevationData = getElevationData(map);

  const terrain = [];
  const lighting = [];
  const elevation = [];

  for (let ry = 0; ry < clippedH; ry++) {
    for (let rx = 0; rx < clippedW; rx++) {
      const idx = (clippedY + ry) * map.width + (clippedX + rx);
      terrain.push(terrainData[idx] || 0);
      lighting.push(lightingData[idx] || 0);
      elevation.push(elevationData[idx] || 0);
    }
  }

  // Extract walls within bounds (convert to relative pixel coords)
  const tileSize = map.tilewidth;
  const regionPxX = clippedX * tileSize;
  const regionPxY = clippedY * tileSize;
  const regionPxW = clippedW * tileSize;
  const regionPxH = clippedH * tileSize;

  const allWalls = getWalls(map);
  const walls = allWalls
    .filter(w => {
      const wx = w.x;
      const wy = w.y;
      const wRight = wx + (w.width || 0);
      const wBottom = wy + (w.height || 0);
      return wx >= regionPxX && wy >= regionPxY && wRight <= regionPxX + regionPxW && wBottom <= regionPxY + regionPxH;
    })
    .map(w => ({ ...w, x: w.x - regionPxX, y: w.y - regionPxY }));

  // Extract spawn zones within bounds (convert to relative pixel coords)
  const allZones = getSpawnZones(map);
  const spawn_zones = allZones
    .filter(z => {
      return z.x >= regionPxX && z.y >= regionPxY &&
        z.x + z.width <= regionPxX + regionPxW &&
        z.y + z.height <= regionPxY + regionPxH;
    })
    .map(z => ({ ...z, x: z.x - regionPxX, y: z.y - regionPxY }));

  return { width: clippedW, height: clippedH, terrain, lighting, elevation, walls, spawn_zones };
}

/**
 * Paste a region's data onto the map at the destination position, clipping to map bounds.
 * @param {object} map - The Tiled-compatible map object.
 * @param {object} region - Region data from extractRegion.
 * @param {number} destX - Destination tile x.
 * @param {number} destY - Destination tile y.
 * @returns {object} Updated map (mutates in place).
 */
export function pasteRegion(map, region, destX, destY) {
  if (region.width === 0 || region.height === 0) return map;

  const terrainLayer = findLayer(map, 'terrain', 'tilelayer');
  const lightingLayer = findLayer(map, 'lighting', 'tilelayer');
  const elevationLayer = findLayer(map, 'elevation', 'tilelayer');

  // Paste tile data, clipping to map bounds
  const pasteW = Math.min(region.width, map.width - destX);
  const pasteH = Math.min(region.height, map.height - destY);

  for (let ry = 0; ry < pasteH; ry++) {
    for (let rx = 0; rx < pasteW; rx++) {
      const srcIdx = ry * region.width + rx;
      const dstIdx = (destY + ry) * map.width + (destX + rx);
      if (terrainLayer && region.terrain[srcIdx] !== undefined) {
        terrainLayer.data[dstIdx] = region.terrain[srcIdx];
      }
      if (lightingLayer && region.lighting[srcIdx] !== undefined) {
        lightingLayer.data[dstIdx] = region.lighting[srcIdx];
      }
      if (elevationLayer && region.elevation[srcIdx] !== undefined) {
        elevationLayer.data[dstIdx] = region.elevation[srcIdx];
      }
    }
  }

  // Paste walls with offset
  const tileSize = map.tilewidth;
  const wallLayer = findLayer(map, 'walls', 'objectgroup');
  if (wallLayer && region.walls) {
    for (const w of region.walls) {
      wallLayer.objects.push({
        ...w,
        id: wallLayer.objects.length + 1,
        x: w.x + destX * tileSize,
        y: w.y + destY * tileSize,
      });
    }
  }

  // Paste spawn zones with offset
  const spawnLayer = findLayer(map, 'spawn_zones', 'objectgroup');
  if (spawnLayer && region.spawn_zones) {
    for (const z of region.spawn_zones) {
      spawnLayer.objects.push({
        ...z,
        id: spawnLayer.objects.length + 1,
        x: z.x + destX * tileSize,
        y: z.y + destY * tileSize,
      });
    }
  }

  return map;
}

/**
 * Create an independent deep copy of a map (alias for cloneMap).
 * @param {object} map - The map to duplicate.
 * @returns {object} A deep copy of the map.
 */
export function duplicateMap(map) {
  return cloneMap(map);
}

/**
 * Undo/redo stack for map editor operations.
 * Stores full map snapshots for simplicity and reliability.
 */
export class UndoStack {
  constructor(maxSize = 50) {
    this._undoStack = [];
    this._redoStack = [];
    this._maxSize = maxSize;
  }

  push(snapshot) {
    this._undoStack.push(snapshot);
    if (this._undoStack.length > this._maxSize) {
      this._undoStack.shift();
    }
    this._redoStack = [];
  }

  canUndo() {
    return this._undoStack.length > 0;
  }

  canRedo() {
    return this._redoStack.length > 0;
  }

  undo(currentMap) {
    if (!this.canUndo()) return null;
    const snapshot = this._undoStack.pop();
    this._redoStack.push(currentMap);
    return snapshot;
  }

  redo(currentMap) {
    if (!this.canRedo()) return null;
    const snapshot = this._redoStack.pop();
    this._undoStack.push(currentMap);
    return snapshot;
  }

  clear() {
    this._undoStack = [];
    this._redoStack = [];
  }
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
