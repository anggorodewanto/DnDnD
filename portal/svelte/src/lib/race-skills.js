/**
 * Extracts flat skill proficiencies granted by a race's traits.
 *
 * Races grant skill proficiencies via trait `mechanical_effect` codes of the
 * form `proficiency_<skill>` (e.g. Elf "Keen Senses" -> `proficiency_perception`).
 * Codes may be comma-separated, and `mechanical_effect` uses underscores while
 * skill ids use hyphens. We keep only bare `proficiency_<skill>` codes whose
 * skill is one of the 18 canonical skills, excluding weapon/tool codes and
 * `double_proficiency_*` codes.
 */

import { parseTraits } from './race-perks.js';
import { SKILL_ABILITY } from './skills.js';

/**
 * Extracts the deduped, first-seen-order list of skill ids a race grants.
 * @param {*} traitsRaw - trait array or JSON string of {name, description, mechanical_effect}
 * @returns {string[]} skill ids (e.g. ['perception']); [] for none/invalid
 */
export function raceGrantedSkills(traitsRaw) {
  // parseTraits drops mechanical_effect, so re-parse the raw blob ourselves to
  // reach the codes. Reuse parseTraits only to validate the array shape.
  if (parseTraits(traitsRaw).length === 0) return [];

  const parsed = typeof traitsRaw === 'string' ? safeParse(traitsRaw) : traitsRaw;
  if (!Array.isArray(parsed)) return [];

  const skills = [];
  for (const trait of parsed) {
    if (!trait || typeof trait.mechanical_effect !== 'string') continue;
    for (const rawCode of trait.mechanical_effect.split(',')) {
      const code = rawCode.trim();
      const match = code.match(/^proficiency_(.+)$/);
      if (!match) continue;
      const skill = match[1].replace(/_/g, '-');
      if (!(skill in SKILL_ABILITY)) continue;
      if (skills.includes(skill)) continue;
      skills.push(skill);
    }
  }
  return skills;
}

/**
 * Merges race-granted skills into the player's chosen skills, deduped.
 * Returns a new array; does not mutate either input. Mirrors
 * mergeBackgroundSkills in backgrounds.js.
 * @param {string[]} chosen
 * @param {string[]} granted
 * @returns {string[]}
 */
export function mergeGrantedSkills(chosen, granted) {
  const merged = [...(chosen || [])];
  for (const s of granted || []) {
    if (!merged.includes(s)) merged.push(s);
  }
  return merged;
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
