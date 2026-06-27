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
 * Adds one or more ability-bonus sets to a base score map, returning a fresh
 * object with exactly the six abilities. Used to fold both the race's and the
 * chosen subrace's ability_bonuses into the character's final scores — picking
 * a subrace (e.g. High Elf's +1 INT) must actually raise the stat, not just be
 * advertised. Null/non-object sets are skipped and non-ability keys (e.g. a
 * `choose` blob) are ignored. Base scores that are missing or non-numeric
 * coerce to 0.
 * @param {object} scores - base scores keyed str/dex/con/int/wis/cha
 * @param {...(object|null|undefined)} bonusSets - ability_bonuses blobs to add
 * @returns {{str:number,dex:number,con:number,int:number,wis:number,cha:number}}
 */
export function applyAbilityBonuses(scores = {}, ...bonusSets) {
  const out = {};
  for (const ability of ABILITY_ORDER) out[ability] = Number(scores?.[ability]) || 0;
  for (const bonuses of bonusSets) {
    if (!bonuses || typeof bonuses !== 'object') continue;
    for (const ability of ABILITY_ORDER) out[ability] += Number(bonuses[ability]) || 0;
  }
  return out;
}

/**
 * Inverse of applyAbilityBonuses: recovers base point-buy scores by subtracting
 * the given ability-bonus blobs from post-racial scores. Edit-mode prefill needs
 * this because the saved character stores post-racial scores, but the point-buy
 * widget edits base scores — seeding it with post-racial values would re-apply
 * the racials on save (e.g. a Tiefling's CHA 16 → 18 on every edit). Floors at 1
 * so a malformed/oversized bonus can never produce a negative base.
 * @param {object} scores - post-racial scores keyed str/dex/con/int/wis/cha
 * @param {...(object|null|undefined)} bonusSets - ability_bonuses blobs to remove
 * @returns {{str:number,dex:number,con:number,int:number,wis:number,cha:number}}
 */
export function removeAbilityBonuses(scores = {}, ...bonusSets) {
  const out = {};
  for (const ability of ABILITY_ORDER) out[ability] = Number(scores?.[ability]) || 0;
  for (const bonuses of bonusSets) {
    if (!bonuses || typeof bonuses !== 'object') continue;
    for (const ability of ABILITY_ORDER) out[ability] -= Number(bonuses[ability]) || 0;
  }
  for (const ability of ABILITY_ORDER) out[ability] = Math.max(1, out[ability]);
  return out;
}

/**
 * Returns a race's fixed ability_bonuses map ({} when absent/unparseable),
 * mirroring the racialBonuses derivation used for display. Lets edit-mode
 * prefill strip racials without depending on Svelte's reactive race lookup.
 * @param {object} race - race with `ability_bonuses` (object or JSON string)
 * @returns {object}
 */
export function raceAbilityBonuses(race) {
  if (!race) return {};
  const parsed = parseJSONField(race.ability_bonuses);
  return parsed && typeof parsed === 'object' ? parsed : {};
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
