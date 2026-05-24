/**
 * Helpers for decorating class catalog data in the character builder.
 *
 * The class API (ClassInfo) returns `skill_choices` as a JSON blob shaped
 * like `{"choose": N, "from": ["acrobatics", ...]}` (verified against
 * internal/refdata/seed_classes.go). It may arrive as a parsed object or a
 * JSON string, so we parse defensively.
 */

import { parseJSONField } from './race-perks.js';

/**
 * Title-cases a kebab-case slug, replacing hyphens with spaces.
 *   'animal-handling' -> 'Animal Handling'
 * @param {string} slug
 * @returns {string} '' for falsy input
 */
export function titleCaseSlug(slug) {
  if (!slug) return '';
  return slug
    .split('-')
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');
}

/**
 * Formats a class skill_choices blob into a human string.
 *   {"choose":2,"from":["acrobatics","arcana"]} -> "Choose 2: Acrobatics, Arcana"
 * @param {*} raw - object or JSON string
 * @returns {string} '' if none / no options
 */
export function formatSkillChoices(raw) {
  const parsed = parseJSONField(raw);
  if (!parsed || typeof parsed !== 'object') return '';

  const from = Array.isArray(parsed.from) ? parsed.from : [];
  if (from.length === 0) return '';

  const count = parsed.choose || from.length;
  const skills = from.map(titleCaseSlug).join(', ');
  return `Choose ${count}: ${skills}`;
}
