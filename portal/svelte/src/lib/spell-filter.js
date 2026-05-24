/**
 * Pure logic for the character builder's spell-selection step: filtering
 * spells by name/level and grouping them by magic school.
 *
 * Operates on the portal API `SpellInfo` shape:
 *   { id, name, level (0-9, 0 = cantrip), school (lowercase slug),
 *     casting_time?, duration?, description?, classes[] }
 *
 * All functions are non-mutating: inputs (arrays and spell objects) are
 * never modified.
 */

/**
 * The 8 D&D 5e magic schools in canonical display order.
 * @type {string[]}
 */
export const SCHOOL_ORDER = [
  'abjuration',
  'conjuration',
  'divination',
  'enchantment',
  'evocation',
  'illusion',
  'necromancy',
  'transmutation',
];

/**
 * Title-cases a school slug for display. Empty/null/undefined → 'Other'.
 * @param {string} school - school slug, e.g. 'evocation'
 * @returns {string} display label, e.g. 'Evocation' (or 'Other')
 */
export function schoolLabel(school) {
  if (!school) return 'Other';
  return school.charAt(0).toUpperCase() + school.slice(1);
}

/**
 * Returns true when `spell` satisfies every active filter (logical AND).
 *
 * `filters` is `{ query, level }`; either key may be missing/empty.
 *   - query: case-insensitive substring match against `spell.name`
 *     (trimmed first). Empty/null/undefined matches everything.
 *   - level: '', null or undefined imposes no constraint; otherwise
 *     `Number(spell.level) === Number(level)` (level may be number or
 *     numeric string).
 * @param {object} spell - a SpellInfo object
 * @param {{query?: string, level?: (number|string)}} [filters={}]
 * @returns {boolean}
 */
export function spellMatchesFilters(spell, filters = {}) {
  const { query, level } = filters || {};

  if (!matchesQuery(spell, query)) return false;
  if (!matchesLevel(spell, level)) return false;
  return true;
}

/**
 * Returns true when the spell name contains the (trimmed) query, case
 * insensitively. An empty/null/undefined query always matches.
 * @param {object} spell
 * @param {string} [query]
 * @returns {boolean}
 */
function matchesQuery(spell, query) {
  if (query == null) return true;
  const needle = String(query).trim().toLowerCase();
  if (needle === '') return true;
  const name = String(spell?.name ?? '').toLowerCase();
  return name.includes(needle);
}

/**
 * Returns true when the spell level equals `level` numerically. An empty
 * string, null or undefined level always matches.
 * @param {object} spell
 * @param {(number|string)} [level]
 * @returns {boolean}
 */
function matchesLevel(spell, level) {
  if (level == null || level === '') return true;
  return Number(spell?.level) === Number(level);
}

/**
 * Returns a new array of spells passing `spellMatchesFilters`. Does not
 * mutate the input. Null/undefined input → [].
 * @param {object[]} spells - array of SpellInfo objects
 * @param {{query?: string, level?: (number|string)}} [filters={}]
 * @returns {object[]}
 */
export function filterSpells(spells, filters = {}) {
  if (!spells) return [];
  return spells.filter((spell) => spellMatchesFilters(spell, filters));
}

/**
 * Groups spells into ordered school buckets:
 *   [{ school, label, spells }]
 *
 * Ordering: known schools first (per SCHOOL_ORDER), then unknown non-empty
 * slugs alphabetically, then a single trailing 'Other' group (school '')
 * for spells with an empty/missing school. Empty schools are not emitted.
 * Within each group spells are sorted by level ascending, then name
 * ascending. Does not mutate the input array or its objects.
 * Null/undefined input → [].
 * @param {object[]} spells - array of SpellInfo objects
 * @returns {{school: string, label: string, spells: object[]}[]}
 */
export function groupSpellsBySchool(spells) {
  if (!spells) return [];

  const buckets = new Map();
  for (const spell of spells) {
    const key = SCHOOL_ORDER.includes(spell?.school) ? spell.school : spell?.school || '';
    if (!buckets.has(key)) buckets.set(key, []);
    buckets.get(key).push(spell);
  }

  const known = SCHOOL_ORDER.filter((school) => buckets.has(school));
  const unknown = [...buckets.keys()]
    .filter((key) => key !== '' && !SCHOOL_ORDER.includes(key))
    .sort((a, b) => a.localeCompare(b));
  const other = buckets.has('') ? [''] : [];

  const orderedKeys = [...known, ...unknown, ...other];
  return orderedKeys.map((school) => ({
    school,
    label: schoolLabel(school),
    spells: sortSpells(buckets.get(school)),
  }));
}

/**
 * Returns a new array sorted by level ascending, then name ascending.
 * @param {object[]} spells
 * @returns {object[]}
 */
function sortSpells(spells) {
  return [...spells].sort((a, b) => {
    const levelDiff = Number(a?.level) - Number(b?.level);
    if (levelDiff !== 0) return levelDiff;
    return String(a?.name ?? '').localeCompare(String(b?.name ?? ''));
  });
}

/**
 * Returns the sorted (ascending, numeric) unique set of `level` values
 * across `spells`. Null/undefined input → [].
 * @param {object[]} spells - array of SpellInfo objects
 * @returns {number[]}
 */
export function availableLevels(spells) {
  if (!spells) return [];
  const levels = new Set(spells.map((spell) => Number(spell?.level)));
  return [...levels].sort((a, b) => a - b);
}
