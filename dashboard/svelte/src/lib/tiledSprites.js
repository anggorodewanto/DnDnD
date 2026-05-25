/**
 * Pure helpers for rendering Tiled visual tile layers from embedded
 * single-image tilesets. These mirror the Go renderer's sprite math
 * (internal/gamemap/renderer) so the dashboard canvas and the server-side
 * PNG renderer agree pixel-for-pixel.
 *
 * Tiled packs three flip flags into the high bits of each raw GID and stores
 * the actual global tile id in the low 29 bits.
 */

// Tiled flip flags (high bits of a raw GID).
const FLIP_H = 0x80000000;
const FLIP_V = 0x40000000;
const FLIP_D = 0x20000000;
// Mask for the low 29 bits that hold the actual global tile id.
const GID_MASK = 0x1fffffff;

/**
 * Decode a raw Tiled GID into its tile id and flip flags.
 *
 * @param {number} rawGid - A raw GID from a tilelayer `data` array.
 * @returns {{id: number, flipH: boolean, flipV: boolean, flipD: boolean}}
 *   `id` is the low-29-bit global tile id (0 = empty tile).
 */
export function decodeGID(rawGid) {
  if (!Number.isFinite(rawGid)) {
    return { id: 0, flipH: false, flipV: false, flipD: false };
  }
  // Coerce to an unsigned 32-bit integer so the sign bit (0x80000000) reads
  // as a flag rather than a negative number.
  const raw = rawGid >>> 0;
  return {
    id: raw & GID_MASK,
    flipH: (raw & FLIP_H) !== 0,
    flipV: (raw & FLIP_V) !== 0,
    flipD: (raw & FLIP_D) !== 0,
  };
}

/**
 * Select the tileset that owns a given (flip-masked) tile id: the one with the
 * greatest `firstgid` that is still <= `id`.
 *
 * @param {Array<{firstgid: number}>} tilesets - Embedded tilesets.
 * @param {number} id - A flip-masked global tile id (see decodeGID).
 * @returns {object|null} The owning tileset, or null for the empty tile / no match.
 */
export function tilesetForGID(tilesets, id) {
  if (!Array.isArray(tilesets) || tilesets.length === 0) return null;
  if (!Number.isFinite(id) || id <= 0) return null;

  let best = null;
  for (const ts of tilesets) {
    const first = ts?.firstgid;
    if (!Number.isFinite(first) || first > id) continue;
    if (best === null || first > best.firstgid) best = ts;
  }
  return best;
}

/**
 * Compute the source rectangle (in the tileset image's pixel space) for a tile
 * id within a single-image tileset, accounting for columns/margin/spacing.
 *
 * @param {object} tileset - Embedded single-image tileset (firstgid, columns,
 *   tilewidth, tileheight, margin, spacing).
 * @param {number} id - A flip-masked global tile id owned by this tileset.
 * @returns {{sx: number, sy: number, sw: number, sh: number}} Source rect.
 */
export function tileSrcRect(tileset, id) {
  const tw = tileset.tilewidth;
  const th = tileset.tileheight;
  const margin = tileset.margin || 0;
  const spacing = tileset.spacing || 0;
  const columns = tileset.columns > 0 ? tileset.columns : 1;

  const local = id - tileset.firstgid;
  const col = local % columns;
  const row = Math.floor(local / columns);

  return {
    sx: margin + col * (tw + spacing),
    sy: margin + row * (th + spacing),
    sw: tw,
    sh: th,
  };
}
