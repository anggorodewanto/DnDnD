/**
 * D&D 5e skill -> governing ability mapping and label helpers.
 *
 * The character builder lists skills by name only; these helpers decorate
 * each skill with the ability score it keys off of (e.g. Stealth -> DEX).
 */

/**
 * Maps each of the 18 D&D 5e skills (kebab-case slug) to its governing
 * ability code (str/dex/con/int/wis/cha).
 * @type {Record<string,string>}
 */
export const SKILL_ABILITY = {
  acrobatics: 'dex',
  'animal-handling': 'wis',
  arcana: 'int',
  athletics: 'str',
  deception: 'cha',
  history: 'int',
  insight: 'wis',
  intimidation: 'cha',
  investigation: 'int',
  medicine: 'wis',
  nature: 'int',
  perception: 'wis',
  performance: 'cha',
  persuasion: 'cha',
  religion: 'int',
  'sleight-of-hand': 'dex',
  stealth: 'dex',
  survival: 'wis',
};

/**
 * Returns the governing ability code for a skill slug.
 * @param {string} skill - skill slug, e.g. 'stealth'
 * @returns {string} ability code ('dex'), or '' if unknown
 */
export function abilityForSkill(skill) {
  if (!skill) return '';
  return SKILL_ABILITY[skill] || '';
}

/**
 * Returns the uppercased ability label for a skill slug.
 * @param {string} skill - skill slug, e.g. 'stealth'
 * @returns {string} ability label ('DEX'), or '' if unknown
 */
export function abilityLabel(skill) {
  const ability = abilityForSkill(skill);
  if (!ability) return '';
  return ability.toUpperCase();
}
