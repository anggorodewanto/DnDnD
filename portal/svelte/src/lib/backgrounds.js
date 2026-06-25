import BACKGROUNDS from './backgrounds.json';

/**
 * SINGLE SOURCE OF TRUTH: ./backgrounds.json.
 *
 * The JSON is the canonical PHB background data shared by both sides of the
 * stack. This frontend file derives its maps from it; the Go backend generates
 * its maps from the same JSON (`scripts/gen_backgrounds` -> internal/portal/
 * backgrounds_gen.go, drift-guarded by `make backgrounds-check`). Editing the
 * JSON is the ONLY place background data should change — never hand-edit a
 * derived map here or a generated map in Go (that drift caused the
 * guild-artisan / folk-hero 400; see docs/live-play ISSUE-013).
 *
 * Slugs are lowercase kebab-case and must match CharacterBuilder.svelte
 * BACKGROUNDS.
 */

/**
 * PHB Background skill proficiencies (two fixed skills each). The builder uses
 * this map to auto-add background skills, deduped against class-skill picks.
 */
export const BACKGROUND_SKILLS = Object.fromEntries(
  Object.entries(BACKGROUNDS).map(([id, b]) => [id, b.skills]),
);

/**
 * Structured PHB mechanical grants per background, beyond the two skills.
 *
 *   tools     — readable tool-proficiency labels ([] if none)
 *   languages — count of bonus languages of the player's choice
 *   feature   — { name, description }; descriptions are brief original
 *               paraphrases (NOT copied PHB/SRD prose) capturing the gist.
 */
export const BACKGROUND_DETAILS = Object.fromEntries(
  Object.entries(BACKGROUNDS).map(([id, b]) => [
    id,
    { tools: b.tools, languages: b.languages, feature: b.feature },
  ]),
);

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

/**
 * Returns the full mechanical profile for a background id, merging skills
 * with the structured detail data, or null if the background is unknown.
 * @param {string} bg
 * @returns {{ skills: string[], tools: string[], languages: number, feature: { name: string, description: string } } | null}
 */
export function backgroundDetails(bg) {
  if (!bg) return null;
  const detail = BACKGROUND_DETAILS[bg];
  if (!detail) return null;
  return {
    skills: skillsForBackground(bg),
    tools: detail.tools,
    languages: detail.languages,
    feature: detail.feature,
  };
}

/**
 * Renders a bonus-language count as display text.
 * Returns '' for 0/falsy, the singular form for 1, and a counted plural
 * for higher values.
 * @param {number} n
 * @returns {string}
 */
export function formatLanguages(n) {
  if (!n) return '';
  if (n === 1) return 'One language of your choice';
  return `${n} languages of your choice`;
}
