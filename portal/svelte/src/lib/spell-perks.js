/**
 * Spell display helpers derived from the SpellInfo API shape.
 * Concentration spells encode it in the duration string, e.g.
 * "Concentration, up to 1 minute" (see internal/refdata/seed_spells_*.go).
 */

/**
 * Returns true when the duration string denotes a concentration spell.
 * @param {string} duration - Spell duration string
 * @returns {boolean}
 */
export function isConcentration(duration) {
  if (!duration) return false;
  return duration.toLowerCase().includes('concentration');
}

/**
 * Returns the casting-time string, trimmed; "" when empty/undefined.
 * @param {string} ct - Casting time string
 * @returns {string}
 */
export function formatCastingTime(ct) {
  if (!ct) return '';
  return ct.trim();
}

/**
 * Trims a description to <= maxLen chars, cutting at the last word
 * boundary before maxLen and appending "…" when truncation occurred.
 * @param {string} desc - Full description
 * @param {number} [maxLen=140] - Maximum length before the ellipsis
 * @returns {string}
 */
export function shortDescription(desc, maxLen = 140) {
  if (!desc) return '';
  const trimmed = desc.trim();
  if (trimmed.length <= maxLen) return trimmed;

  const slice = trimmed.slice(0, maxLen);
  const lastSpace = slice.lastIndexOf(' ');
  if (lastSpace <= 0) return `${slice}…`;
  return `${slice.slice(0, lastSpace)}…`;
}
