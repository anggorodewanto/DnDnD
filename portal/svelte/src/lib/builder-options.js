/**
 * Helpers for parsing race/class catalog data and managing the
 * multiclass row list inside the character builder.
 *
 * The portal API returns subraces and subclasses as JSON-encoded blobs
 * (the storage layer keeps them as JSONB). These helpers normalise the
 * raw shapes from the backend into simple [{ id, name }] option arrays.
 */

/**
 * Normalises a race's subrace blob into a list of options.
 * Backend shape: array of { id, name, ... }.
 * Returns [] if the race has no subraces.
 * @param {object} race
 * @returns {{id:string,name:string}[]}
 */
export function subraceOptions(race) {
  if (!race || !race.subraces) return [];
  let raw = race.subraces;
  if (typeof raw === 'string') {
    try { raw = JSON.parse(raw); } catch { return []; }
  }
  if (!Array.isArray(raw)) return [];
  return raw
    .filter(s => s && (s.id || s.name))
    .map(s => ({ id: s.id || s.name, name: s.name || s.id }));
}

/**
 * Normalises a class's subclass blob into a list of options.
 * Backend shape: object keyed by subclass id with { name, ... } values.
 * Returns [] if the class has no subclasses.
 * @param {object} cls
 * @returns {{id:string,name:string}[]}
 */
export function subclassOptions(cls) {
  if (!cls || !cls.subclasses) return [];
  let raw = cls.subclasses;
  if (typeof raw === 'string') {
    try { raw = JSON.parse(raw); } catch { return []; }
  }
  if (!raw || typeof raw !== 'object') return [];
  return Object.entries(raw).map(([id, val]) => ({
    id,
    name: (val && val.name) || id,
  }));
}

/**
 * Whether the chosen class has reached / passed its subclass level
 * threshold. Sorcerer/Cleric/Warlock pick at level 1; most pick at 3.
 * @param {object} cls - ClassInfo with subclass_level
 * @param {number} level - the character's level in this class
 */
export function isSubclassEligible(cls, level) {
  if (!cls) return false;
  const threshold = cls.subclass_level || 0;
  if (threshold <= 0) return false;
  return level >= threshold;
}

/**
 * Creates an empty multiclass row.
 * @returns {{class:string, level:number, subclass:string}}
 */
export function emptyClassRow() {
  return { class: '', level: 1, subclass: '' };
}

/**
 * Returns a new array with an appended class row.
 * @param {Array} rows
 */
export function addClassRow(rows) {
  return [...(rows || []), emptyClassRow()];
}

/**
 * Returns a new array without the row at `idx`. The first row cannot
 * be removed — characters always have at least one class.
 * @param {Array} rows
 * @param {number} idx
 */
export function removeClassRow(rows, idx) {
  if (!Array.isArray(rows) || rows.length <= 1) return rows || [];
  if (idx <= 0 || idx >= rows.length) return rows;
  return rows.filter((_, i) => i !== idx);
}

/**
 * Returns a new array with the row at `idx` updated with the partial patch.
 * @param {Array} rows
 * @param {number} idx
 * @param {object} patch
 */
export function updateClassRow(rows, idx, patch) {
  if (!Array.isArray(rows) || idx < 0 || idx >= rows.length) return rows;
  return rows.map((r, i) => (i === idx ? { ...r, ...patch } : r));
}

/**
 * Returns the total level across all multiclass entries (capped at 20).
 * @param {Array} rows
 */
export function totalLevel(rows) {
  if (!Array.isArray(rows)) return 0;
  let sum = 0;
  for (const r of rows) sum += Number(r?.level) || 0;
  return Math.min(sum, 20);
}
