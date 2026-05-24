/**
 * Extracts flat weapon proficiencies granted by a race's traits.
 *
 * Races grant weapon proficiencies via trait `mechanical_effect` codes of the
 * form `proficiency_<weapon-id>` (e.g. Dwarf "Dwarven Combat Training" ->
 * `proficiency_battleaxe,proficiency_handaxe,...`). Codes may be comma-separated,
 * and `mechanical_effect` may use underscores while weapon ids use hyphens.
 *
 * The same trait surface also carries non-weapon proficiency codes (skills,
 * tools, armor categories, count-based tokens like `proficiency_4-weapons`), so
 * we can only surface real weapons by filtering against an allow-list of known
 * weapon ids supplied by the caller.
 */

import { parseTraits } from './race-perks.js';

/**
 * Extracts the deduped, first-seen-order list of weapon ids a race grants,
 * keeping only ids present in the supplied allow-list.
 * @param {*} traitsRaw - trait array or JSON string of {name, description, mechanical_effect}
 * @param {string[]|Set<string>} weaponIds - allow-list of valid weapon ids
 * @returns {string[]} weapon ids (e.g. ['battleaxe','handaxe']); [] for none/invalid
 */
export function raceGrantedWeaponProficiencies(traitsRaw, weaponIds) {
  const allow = toAllowSet(weaponIds);
  if (allow.size === 0) return [];

  // parseTraits drops mechanical_effect, so re-parse the raw blob ourselves to
  // reach the codes. Reuse parseTraits only to validate the array shape.
  if (parseTraits(traitsRaw).length === 0) return [];

  const parsed = typeof traitsRaw === 'string' ? safeParse(traitsRaw) : traitsRaw;
  if (!Array.isArray(parsed)) return [];

  const weapons = [];
  for (const trait of parsed) {
    if (!trait || typeof trait.mechanical_effect !== 'string') continue;
    for (const rawCode of trait.mechanical_effect.split(',')) {
      const code = rawCode.trim();
      const match = code.match(/^proficiency_(.+)$/);
      if (!match) continue;
      const weapon = match[1].replace(/_/g, '-').toLowerCase();
      if (!allow.has(weapon)) continue;
      if (weapons.includes(weapon)) continue;
      weapons.push(weapon);
    }
  }
  return weapons;
}

/**
 * Title-cases a weapon id for display.
 *   'light-hammer' -> 'Light Hammer'
 *   'battleaxe' -> 'Battleaxe'
 * @param {string} weaponId
 * @returns {string} display label, or '' for empty/null/undefined
 */
export function weaponProficiencyLabel(weaponId) {
  if (!weaponId) return '';
  return weaponId
    .split('-')
    .map((word) => (word ? word[0].toUpperCase() + word.slice(1) : word))
    .join(' ');
}

/**
 * Normalises an Array or Set allow-list into a Set for lookup.
 * @param {string[]|Set<string>|null|undefined} weaponIds
 * @returns {Set<string>} empty set for null/undefined
 */
function toAllowSet(weaponIds) {
  if (!weaponIds) return new Set();
  if (weaponIds instanceof Set) return weaponIds;
  if (Array.isArray(weaponIds)) return new Set(weaponIds);
  return new Set();
}

/**
 * @param {string} raw
 * @returns {*} parsed value, or null on failure
 */
function safeParse(raw) {
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
}
