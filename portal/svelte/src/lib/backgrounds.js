/**
 * PHB Background skill proficiencies.
 *
 * Each background grants two fixed skill proficiencies (per the PHB).
 * The builder uses this map to auto-add background skills onto the
 * character's skill list, deduped against any class-skill picks.
 *
 * IDs match the slugs used elsewhere in the portal (lowercase, kebab-case).
 */
export const BACKGROUND_SKILLS = {
  acolyte: ['insight', 'religion'],
  charlatan: ['deception', 'sleight-of-hand'],
  criminal: ['deception', 'stealth'],
  entertainer: ['acrobatics', 'performance'],
  'folk-hero': ['animal-handling', 'survival'],
  'guild-artisan': ['insight', 'persuasion'],
  hermit: ['medicine', 'religion'],
  noble: ['history', 'persuasion'],
  outlander: ['athletics', 'survival'],
  sage: ['arcana', 'history'],
  sailor: ['athletics', 'perception'],
  soldier: ['athletics', 'intimidation'],
  urchin: ['sleight-of-hand', 'stealth'],
};

/**
 * Returns the canonical skill list for a background id, or an empty
 * array if the background is unknown.
 * @param {string} bg
 * @returns {string[]}
 */
export function skillsForBackground(bg) {
  if (!bg) return [];
  return BACKGROUND_SKILLS[bg] || [];
}

/**
 * Merges background skills into the player's skill picks, deduped.
 * Returns a new array (does not mutate the input).
 * @param {string[]} chosenSkills
 * @param {string} background
 * @returns {string[]}
 */
export function mergeBackgroundSkills(chosenSkills, background) {
  const bgSkills = skillsForBackground(background);
  if (bgSkills.length === 0) return [...(chosenSkills || [])];
  const merged = [...(chosenSkills || [])];
  for (const s of bgSkills) {
    if (!merged.includes(s)) merged.push(s);
  }
  return merged;
}
