/**
 * Pure, framework-free serialize/parse/key logic for the character builder's
 * in-progress draft persistence. The draft lets a page reload recover the
 * unsubmitted form fields.
 *
 * This module performs NO I/O: it never touches `localStorage`. It only turns
 * a plain builder-state object into a versioned JSON string and back, and
 * computes the per-token storage key. The actual storage read/write and the
 * Svelte wiring live elsewhere.
 *
 * All functions are non-mutating and side-effect-free.
 */

/**
 * Schema version stamped into every serialized draft. Bump this whenever the
 * persisted shape changes incompatibly; `parseDraft` discards any draft whose
 * `v` does not match, so stale-schema drafts are never restored.
 * @type {number}
 */
export const DRAFT_VERSION = 1;

/**
 * The builder-state field names that are persisted, in canonical order.
 * Anything outside this allow-list is dropped on the way in and on the way out.
 * Module-private on purpose.
 * @type {string[]}
 */
const DRAFT_FIELDS = [
  'currentStep',
  'name',
  'race',
  'subrace',
  'background',
  'classEntries',
  'scores',
  'abilityMethod',
  'abilityRolls',
  'selectedSkills',
  'selectedSpells',
  'selectedMasteries',
  'packChoices',
  'manualEquipment',
  'wornArmor',
  'equippedWeapon',
];

/**
 * Computes the localStorage key for a draft scoped to `token`. An empty,
 * null or undefined token collapses to the shared `:default` namespace.
 * @param {string} [token] - per-session/user token
 * @returns {string} the storage key, e.g. 'dndnd-builder-draft:abc'
 */
export function draftKey(token) {
  if (!token) return 'dndnd-builder-draft:default';
  return `dndnd-builder-draft:${token}`;
}

/**
 * Serializes `state` to a versioned JSON string. The result always carries
 * `v: DRAFT_VERSION`, plus every DRAFT_FIELDS entry that is present
 * (`!== undefined`) on `state`. Fields that are absent/undefined are omitted,
 * and any property on `state` outside DRAFT_FIELDS is ignored. A null or
 * undefined `state` serializes to just the version envelope.
 * @param {object|null|undefined} state - builder-state object
 * @returns {string} JSON string of the draft
 */
export function serializeDraft(state) {
  const draft = { v: DRAFT_VERSION };
  if (state == null) return JSON.stringify(draft);

  for (const field of DRAFT_FIELDS) {
    if (state[field] !== undefined) draft[field] = state[field];
  }
  return JSON.stringify(draft);
}

/**
 * Parses a serialized draft string back into a plain object containing only
 * the known DRAFT_FIELDS that were present. The `v` envelope key is consumed
 * and stripped, and unknown keys are dropped. Defensive: returns `null` for
 * any unusable input rather than throwing.
 *
 * Returns `null` when:
 *   - `raw` is empty/null/undefined
 *   - `raw` is not valid JSON
 *   - the parsed value is not a non-null, non-array object
 *   - the parsed `v` does not equal DRAFT_VERSION (version mismatch)
 * @param {string|null|undefined} raw - serialized draft
 * @returns {object|null} restored field subset, or null
 */
export function parseDraft(raw) {
  if (!raw) return null;

  let parsed;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }

  if (typeof parsed !== 'object' || parsed === null) return null;
  if (Array.isArray(parsed)) return null;
  if (parsed.v !== DRAFT_VERSION) return null;

  const result = {};
  for (const field of DRAFT_FIELDS) {
    if (parsed[field] !== undefined) result[field] = parsed[field];
  }
  return result;
}
