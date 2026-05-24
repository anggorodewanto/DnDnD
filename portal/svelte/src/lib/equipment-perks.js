/**
 * Human-readable formatting for equipment (weapon/armor) perks.
 *
 * Data shapes (verified against internal/refdata/seeder.go):
 *   - Weapon `properties` are plain lowercase strings, possibly hyphenated:
 *       "light", "finesse", "thrown", "versatile", "two-handed", "heavy",
 *       "reach", "ammunition", "loading", "special".
 *     (Versatile damage and thrown range live in separate API fields, so the
 *      current API emits plain strings; "key:value" support is defensive for
 *      homebrew / future shapes.)
 *   - Armor `armor_type` values: "light", "medium", "heavy", "shield".
 */

/**
 * Title-cases a single token, including hyphen-separated words.
 * e.g. "two-handed" -> "Two-Handed", "finesse" -> "Finesse".
 * @param {string} word
 * @returns {string}
 */
function titleCase(word) {
  return word
    .split('-')
    .map((part) => {
      if (part.length === 0) return part;
      return part.charAt(0).toUpperCase() + part.slice(1).toLowerCase();
    })
    .join('-');
}

/**
 * Formats a single property string.
 * "finesse" -> "Finesse"; "versatile:1d10" -> "Versatile (1d10)".
 * @param {string} prop
 * @returns {string} '' for blank input
 */
function formatProperty(prop) {
  const trimmed = prop.trim();
  if (trimmed === '') return '';
  const sep = trimmed.indexOf(':');
  if (sep === -1) return titleCase(trimmed);
  const key = trimmed.slice(0, sep);
  const value = trimmed.slice(sep + 1);
  return `${titleCase(key)} (${value})`;
}

/**
 * Formats a weapon property list into a comma-joined human-readable string.
 * @param {string[]} props
 * @returns {string} '' when empty/undefined
 */
export function formatProperties(props) {
  if (!props || props.length === 0) return '';
  return props
    .map(formatProperty)
    .filter((p) => p !== '')
    .join(', ');
}

/**
 * Formats an armor AC formula string.
 * @param {string} armorType - light | medium | heavy | shield (case-insensitive)
 * @param {number} acBase
 * @returns {string} '' when acBase is null/undefined
 */
export function armorACText(armorType, acBase) {
  if (acBase === null || acBase === undefined) return '';
  const type = (armorType || '').toLowerCase();
  if (type === 'light') return `AC ${acBase} + DEX`;
  if (type === 'medium') return `AC ${acBase} + DEX (max 2)`;
  if (type === 'shield') return `+${acBase} AC`;
  // heavy + unknown/empty both fall back to plain AC.
  return `AC ${acBase}`;
}
