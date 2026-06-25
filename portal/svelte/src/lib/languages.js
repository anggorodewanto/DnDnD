/**
 * Concrete language handling for the character builder.
 *
 * The races API returns each race's base languages as Title-Cased strings
 * (e.g. ["Common","Dwarvish"]); backgrounds grant an integer COUNT of bonus
 * languages of the player's choice. This module turns that race base + the
 * player's bonus picks into the concrete list of strings persisted as
 * CharacterSubmission.Languages.
 *
 * All exports are pure (no Svelte/DOM deps) so they unit-test cleanly.
 */

/** PHB standard languages (Title-Case display strings). */
export const STANDARD_LANGUAGES = [
  'Common', 'Dwarvish', 'Elvish', 'Giant', 'Gnomish', 'Goblin', 'Halfling', 'Orc',
];

/** PHB exotic languages (Title-Case display strings). */
export const EXOTIC_LANGUAGES = [
  'Abyssal', 'Celestial', 'Draconic', 'Deep Speech', 'Infernal', 'Primordial', 'Sylvan', 'Undercommon',
];

/** Every selectable language: standard followed by exotic, display order. */
export const ALL_LANGUAGES = [...STANDARD_LANGUAGES, ...EXOTIC_LANGUAGES];

/**
 * Extracts a race's base languages from the race API object, preserving their
 * given casing and dropping empty/whitespace entries. Returns [] when the race
 * data (or its languages array) is missing.
 * @param {{ languages?: string[] } | null | undefined} raceData
 * @returns {string[]}
 */
export function raceBaseLanguages(raceData) {
  const langs = raceData?.languages;
  if (!Array.isArray(langs)) return [];
  return langs.filter(l => typeof l === 'string' && l.trim() !== '');
}

/**
 * Returns ALL_LANGUAGES minus everything already known, compared
 * case-insensitively, preserving ALL_LANGUAGES order. `known` is an array of
 * strings (race base languages + already-chosen bonus picks).
 * @param {string[]} [known]
 * @returns {string[]}
 */
export function availableLanguageChoices(known) {
  const knownLower = new Set(
    (known ?? [])
      .filter(l => typeof l === 'string' && l.trim() !== '')
      .map(l => l.trim().toLowerCase()),
  );
  return ALL_LANGUAGES.filter(l => !knownLower.has(l.toLowerCase()));
}

/**
 * Unions the race base languages then the chosen bonus languages into the
 * final concrete list: de-duplicated case-insensitively (first-seen casing
 * wins), first-seen order preserved, empty/whitespace entries dropped.
 * @param {string[]} [raceLangs]
 * @param {string[]} [chosen]
 * @returns {string[]}
 */
export function assembleLanguages(raceLangs, chosen) {
  const out = [];
  const seen = new Set();
  for (const lang of [...(raceLangs ?? []), ...(chosen ?? [])]) {
    if (typeof lang !== 'string') continue;
    const trimmed = lang.trim();
    if (trimmed === '') continue;
    const key = trimmed.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(trimmed);
  }
  return out;
}

/**
 * Coerces a background detail's `languages` field (an integer count of bonus
 * languages of the player's choice) into a non-negative integer.
 * @param {{ languages?: number } | null | undefined} backgroundDetail
 * @returns {number}
 */
export function bonusLanguageCount(backgroundDetail) {
  return Math.max(0, Number(backgroundDetail?.languages) || 0);
}
