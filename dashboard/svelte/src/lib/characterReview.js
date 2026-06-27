/**
 * Pure helpers for the DM character review / edit-diff panel.
 *
 * The approval API returns a ReviewCharacter projection (`review`) of the
 * current state and, for an edited character, the last-approved baseline
 * (`review_before`). These helpers turn that pair into display-ready rows:
 * a full-review field list and a "changes since last approval" diff. They build
 * on diffStates so the deep-equality comparison stays in one place.
 */

import { diffStates } from './diff.js';

/** Human labels for the review projection's JSON fields. */
export const REVIEW_FIELD_LABELS = {
  name: 'Name',
  race: 'Race',
  subrace: 'Subrace',
  background: 'Background',
  classes: 'Classes',
  level: 'Level',
  ability_scores: 'Ability Scores',
  hp_max: 'HP Max',
  ac: 'AC',
  speed_ft: 'Speed',
  skills: 'Skills',
  expertise: 'Expertise',
  saves: 'Saves',
  languages: 'Languages',
  equipment: 'Equipment',
  spells: 'Spells',
  weapon_masteries: 'Weapon Masteries',
  features: 'Features',
  appearance: 'Appearance',
  backstory: 'Backstory',
};

/** Order the full review renders fields in. */
export const REVIEW_FIELD_ORDER = [
  'name',
  'race',
  'subrace',
  'background',
  'classes',
  'level',
  'ability_scores',
  'hp_max',
  'ac',
  'speed_ft',
  'saves',
  'skills',
  'expertise',
  'languages',
  'equipment',
  'spells',
  'weapon_masteries',
  'features',
  'appearance',
  'backstory',
];

/** Optional fields hidden from the full review when empty. */
const OPTIONAL_TEXT_FIELDS = new Set(['subrace', 'background', 'appearance', 'backstory']);

const ABILITY_ORDER = ['str', 'dex', 'con', 'int', 'wis', 'cha'];

/**
 * Turn a before/after ReviewCharacter pair into display rows:
 *  - ability_scores expands into per-ability scalar rows (e.g. CHA 8 -> 10)
 *  - array fields become {kind:'list', added, removed}
 *  - scalars become {kind:'scalar', before, after}
 * @param {object|null} before
 * @param {object|null} after
 * @returns {Array<object>}
 */
export function reviewChanges(before, after) {
  const rows = [];
  for (const { field, before: bv, after: av } of diffStates(before, after)) {
    if (field === 'ability_scores') {
      rows.push(...abilityChanges(bv, av));
      continue;
    }
    const label = REVIEW_FIELD_LABELS[field] ?? field;
    if (Array.isArray(bv) || Array.isArray(av)) {
      const { added, removed } = listDiff(bv ?? [], av ?? []);
      rows.push({ field, label, kind: 'list', added, removed });
    } else {
      rows.push({ field, label, kind: 'scalar', before: bv, after: av });
    }
  }
  return rows;
}

function abilityChanges(before, after) {
  const b = before ?? {};
  const a = after ?? {};
  const rows = [];
  for (const ab of ABILITY_ORDER) {
    const bv = b[ab] ?? 0;
    const av = a[ab] ?? 0;
    if (bv !== av) {
      rows.push({ field: ab, label: ab.toUpperCase(), kind: 'scalar', before: b[ab], after: a[ab] });
    }
  }
  return rows;
}

/**
 * Set difference of two arrays: what's only in after (added) / only in before (removed).
 * @param {Array} before
 * @param {Array} after
 * @returns {{added: Array, removed: Array}}
 */
export function listDiff(before, after) {
  const bs = new Set(before);
  const as = new Set(after);
  return {
    added: after.filter((x) => !bs.has(x)),
    removed: before.filter((x) => !as.has(x)),
  };
}

/**
 * Render a single review field value for the full-review display.
 * @param {string} field
 * @param {*} value
 * @returns {string}
 */
export function formatReviewField(field, value) {
  if (field === 'ability_scores') return formatAbilityScores(value);
  if (Array.isArray(value)) return value.length ? value.join(', ') : '—';
  if (value === null || value === undefined || value === '') return '—';
  return String(value);
}

function formatAbilityScores(scores) {
  const s = scores ?? {};
  return ABILITY_ORDER.map((ab) => `${ab.toUpperCase()} ${s[ab] ?? '—'}`).join(' · ');
}

/**
 * Ordered [{field,label,value}] rows for the full current-state review,
 * skipping absent optional text fields.
 * @param {object|null} review
 * @returns {Array<{field:string,label:string,value:string}>}
 */
export function reviewFields(review) {
  const r = review ?? {};
  const out = [];
  for (const field of REVIEW_FIELD_ORDER) {
    const value = r[field];
    if (OPTIONAL_TEXT_FIELDS.has(field) && !value) continue;
    out.push({ field, label: REVIEW_FIELD_LABELS[field], value: formatReviewField(field, value) });
  }
  return out;
}
