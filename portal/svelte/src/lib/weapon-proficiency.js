/**
 * Computes which weapons a character is proficient with, and the subset
 * eligible for "weapon mastery" selection in the character builder.
 *
 * Weapon proficiency comes from two sources:
 *   - the class's `weapon_proficiencies`, which may be broad category tokens
 *     ('simple' / 'martial', optionally suffixed ' weapons') or specific weapon
 *     names/ids ('longsword', 'hand crossbow'). These are not normalised, so we
 *     are lenient: any token containing the word 'simple'/'martial' grants that
 *     category; anything else is treated as a specific weapon id (lowercased,
 *     spaces -> hyphens).
 *   - race-granted specific weapon ids (already normalised, lowercased here for
 *     safety).
 *
 * Mastery eligibility is the intersection of proficient weapons and weapons
 * that actually carry a non-empty `mastery` slug (a net, for example, has none).
 */

/**
 * Returns the deduped, weapon-order list of weapon ids the character is
 * proficient with, from class category/specific grants plus race-granted ids.
 * @param {Array<{id:string,weapon_type:string}>|null|undefined} weapons - weapon catalogue
 * @param {string[]|null|undefined} classProficiencies - raw class weapon_proficiencies tokens
 * @param {string[]|null|undefined} raceWeaponIds - normalised race-granted weapon ids
 * @returns {string[]} proficient weapon ids in `weapons` order; [] for null/undefined weapons
 */
export function proficientWeaponIds(weapons, classProficiencies, raceWeaponIds) {
  if (!Array.isArray(weapons)) return [];

  let simpleGranted = false;
  let martialGranted = false;
  const specificIds = new Set();

  for (const raw of toArray(classProficiencies)) {
    if (typeof raw !== 'string') continue;
    const token = raw.trim().toLowerCase();
    if (!token) continue;
    if (token.includes('simple')) {
      simpleGranted = true;
      continue;
    }
    if (token.includes('martial')) {
      martialGranted = true;
      continue;
    }
    specificIds.add(token.replace(/ /g, '-'));
  }

  for (const raw of toArray(raceWeaponIds)) {
    if (typeof raw !== 'string') continue;
    const id = raw.trim().toLowerCase();
    if (!id) continue;
    specificIds.add(id);
  }

  const ids = [];
  for (const weapon of weapons) {
    if (!weapon || typeof weapon.id !== 'string') continue;
    if (!isProficient(weapon, simpleGranted, martialGranted, specificIds)) continue;
    if (ids.includes(weapon.id)) continue;
    ids.push(weapon.id);
  }
  return ids;
}

/**
 * Returns the proficient weapon objects that carry a non-empty mastery slug —
 * i.e. the weapons a player may choose masteries from. Input is not mutated.
 * @param {Array<{id:string,mastery?:string}>|null|undefined} weapons - weapon catalogue
 * @param {string[]|Set<string>|null|undefined} proficientIds - ids the character is proficient with
 * @returns {Array<object>} matching weapon objects in `weapons` order; [] for null/undefined inputs
 */
export function masteryEligibleWeapons(weapons, proficientIds) {
  if (!Array.isArray(weapons)) return [];
  if (!proficientIds) return [];

  const proficient = toSet(proficientIds);
  const eligible = [];
  for (const weapon of weapons) {
    if (!weapon || typeof weapon.id !== 'string') continue;
    if (!proficient.has(weapon.id)) continue;
    if (!weapon.mastery) continue;
    eligible.push(weapon);
  }
  return eligible;
}

/**
 * Decides whether a single weapon is proficient given the granted categories
 * and the specific-id set.
 * @param {{id:string,weapon_type:string}} weapon
 * @param {boolean} simpleGranted
 * @param {boolean} martialGranted
 * @param {Set<string>} specificIds
 * @returns {boolean}
 */
function isProficient(weapon, simpleGranted, martialGranted, specificIds) {
  if (specificIds.has(weapon.id)) return true;
  const type = typeof weapon.weapon_type === 'string' ? weapon.weapon_type : '';
  if (simpleGranted && type.startsWith('simple_')) return true;
  if (martialGranted && type.startsWith('martial_')) return true;
  return false;
}

/**
 * Normalises an array-or-nullish value into an array for iteration.
 * @param {*} value
 * @returns {Array} the value if an array, else []
 */
function toArray(value) {
  if (Array.isArray(value)) return value;
  return [];
}

/**
 * Normalises an Array or Set into a Set for membership lookup.
 * @param {string[]|Set<string>} value
 * @returns {Set<string>}
 */
function toSet(value) {
  if (value instanceof Set) return value;
  if (Array.isArray(value)) return new Set(value);
  return new Set();
}
