/**
 * Pure formatting helpers for the spell detail sidebar. Turns a portal API
 * `SpellInfo` ({ id, name, level, school, casting_time?, duration?,
 * description?, classes[] }) into the headline, level label and metadata rows
 * the SpellPicker's detail panel renders. All functions are non-mutating.
 */

import { schoolLabel } from './spell-filter.js';
import { formatCastingTime } from './spell-perks.js';

/**
 * English ordinal for a positive integer, e.g. 1 → "1st", 3 → "3rd",
 * 11 → "11th". Accepts numeric strings.
 * @param {number|string} n
 * @returns {string}
 */
export function ordinal(n) {
  const num = Number(n);
  const tens = num % 100;
  if (tens >= 11 && tens <= 13) return `${num}th`;
  switch (num % 10) {
    case 1:
      return `${num}st`;
    case 2:
      return `${num}nd`;
    case 3:
      return `${num}rd`;
    default:
      return `${num}th`;
  }
}

/**
 * Short level label: "Cantrip" for level 0, "Level N" otherwise. Returns ""
 * when the level is missing or non-numeric. Accepts numeric strings.
 * @param {number|string} level
 * @returns {string}
 */
export function levelLabel(level) {
  const n = Number(level);
  if (!Number.isFinite(n) || (typeof level === 'string' && level.trim() === '')) return '';
  if (level == null) return '';
  if (n === 0) return 'Cantrip';
  return `Level ${n}`;
}

/**
 * D&D-style headline combining level and school, e.g. "Evocation cantrip" or
 * "3rd-level abjuration". The school is dropped when missing. Returns "" for a
 * null/undefined spell.
 * @param {object} spell - a SpellInfo object
 * @returns {string}
 */
export function spellHeadline(spell) {
  if (!spell) return '';
  const lvl = Number(spell.level);
  const school = spell.school || '';
  if (lvl === 0) {
    return school ? `${schoolLabel(school)} cantrip` : 'Cantrip';
  }
  const lead = `${ordinal(lvl)}-level`;
  return school ? `${lead} ${school}` : lead;
}

/**
 * Title-cases a list of class slugs, dropping falsy entries. Null/undefined → [].
 * @param {string[]} classes
 * @returns {string[]}
 */
export function classLabels(classes) {
  return (classes || []).filter(Boolean).map((c) => c.charAt(0).toUpperCase() + c.slice(1));
}

/**
 * Ordered `{ label, value }` metadata rows for the detail panel, omitting any
 * field that is empty/missing. Values are trimmed. Null spell → [].
 * @param {object} spell - a SpellInfo object
 * @returns {{label: string, value: string}[]}
 */
export function spellDetailMeta(spell) {
  if (!spell) return [];
  const rows = [];
  const ct = formatCastingTime(spell.casting_time);
  if (ct) rows.push({ label: 'Casting Time', value: ct });
  const range = (spell.range || '').trim();
  if (range) rows.push({ label: 'Range', value: range });
  const components = (spell.components || []).filter(Boolean);
  if (components.length) rows.push({ label: 'Components', value: components.join(', ') });
  const duration = (spell.duration || '').trim();
  if (duration) rows.push({ label: 'Duration', value: duration });
  const classes = classLabels(spell.classes);
  if (classes.length) rows.push({ label: 'Classes', value: classes.join(', ') });
  return rows;
}
