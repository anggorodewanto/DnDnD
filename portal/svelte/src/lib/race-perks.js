/**
 * Helpers for decorating race / subrace catalog data in the character builder.
 *
 * The portal API returns several fields as JSON-encoded blobs (the storage
 * layer keeps them as JSONB), so each may arrive as a parsed object/array OR
 * a JSON string. These helpers normalise those shapes into display-ready
 * strings and trait lists.
 */

/** Canonical D&D ability order for sorting fixed bonuses. */
const ABILITY_ORDER = ['str', 'dex', 'con', 'int', 'wis', 'cha'];

/**
 * Parses a field that may be a JS object/array, a JSON string, or null.
 * @param {*} raw
 * @returns {*} parsed value, or null on parse failure / empty input
 */
export function parseJSONField(raw) {
  if (raw === null || raw === undefined) return null;
  if (typeof raw !== 'string') return raw;
  if (raw === '') return null;
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

/**
 * Formats a race/subrace ability_bonuses blob into a human string.
 *   {"con":2} -> "+2 CON"
 *   all six at +1 -> "+1 to all abilities"
 *   {"cha":2,"choose":{count,amount,from}} -> "+2 CHA, +1 to 2 abilities of your choice"
 * Fixed bonuses are sorted in canonical order; ability codes uppercased.
 * @param {*} raw - object or JSON string
 * @returns {string} human-readable bonus string, or '' if none
 */
export function formatAbilityBonuses(raw) {
  const parsed = parseJSONField(raw);
  if (!parsed || typeof parsed !== 'object') return '';

  const { choose, ...fixed } = parsed;
  const entries = ABILITY_ORDER.filter((a) => typeof fixed[a] === 'number').map((a) => [
    a,
    fixed[a],
  ]);

  const allSixPlusOne =
    entries.length === ABILITY_ORDER.length && entries.every(([, v]) => v === 1);

  const parts = [];
  if (allSixPlusOne) {
    parts.push('+1 to all abilities');
  } else {
    for (const [ability, amount] of entries) {
      parts.push(`+${amount} ${ability.toUpperCase()}`);
    }
  }

  if (choose && typeof choose === 'object') {
    const count = choose.count || 0;
    const amount = choose.amount || 0;
    parts.push(`+${amount} to ${count} abilities of your choice`);
  }

  return parts.join(', ');
}

/**
 * Parses a traits blob into display rows, dropping mechanical_effect.
 * @param {*} raw - array of {name, description, mechanical_effect} or JSON string
 * @returns {{name:string, description:string}[]} [] if none/invalid
 */
export function parseTraits(raw) {
  const parsed = parseJSONField(raw);
  if (!Array.isArray(parsed)) return [];
  return parsed
    .filter((t) => t && t.name)
    .map((t) => ({ name: t.name, description: t.description || '' }));
}

/**
 * Formats a darkvision range in feet.
 * @param {number} ft
 * @returns {string} '' when falsy/0, else `${ft} ft`
 */
export function formatDarkvision(ft) {
  if (!ft) return '';
  return `${ft} ft`;
}

/**
 * Looks up a subrace within a race and returns its perks.
 * @param {object} race - race with `subraces` (array or JSON string)
 * @param {string} subraceId
 * @returns {{abilityBonuses: (object|null), traits: {name:string,description:string}[]} | null}
 *   null if the race has no subraces or the id is not found.
 */
export function subracePerks(race, subraceId) {
  if (!race) return null;
  const subraces = parseJSONField(race.subraces);
  if (!Array.isArray(subraces)) return null;

  const match = subraces.find((s) => s && s.id === subraceId);
  if (!match) return null;

  return {
    abilityBonuses: parseJSONField(match.ability_bonuses),
    traits: parseTraits(match.traits),
  };
}
