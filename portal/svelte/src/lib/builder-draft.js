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
 * Computes the stable draft scope (the string handed to `draftKey`) for a
 * builder session. A player's draft is keyed by *campaign*, not by the
 * single-use portal token: a token is consumed/expired on submit and a
 * reissued `/create-character` link mints a fresh one, so token-keying orphaned
 * the in-progress draft and the new page came up blank (usability T10 /
 * Finding 4·a). Campaign-keying survives that rotation. The token is only a
 * fallback for the (rare) case where no campaign id is known.
 *
 * The `mode` prefix matters: the player portal and the DM dashboard mount this
 * same builder from one localStorage origin, so without it a player's PC draft
 * and a DM's NPC draft for the same campaign would clobber each other.
 *
 * Returns '' when nothing identifies the draft, so `draftKey('')` collapses to
 * the shared `:default` namespace exactly as before.
 * @param {string} mode - 'player' | 'dm'
 * @param {string} [campaignId] - active campaign id (preferred, stable key)
 * @param {string} [token] - portal token (fallback only)
 * @returns {string} scope string for draftKey()
 */
export function draftScope(mode, campaignId, token) {
  const id = campaignId || token || '';
  if (!id) return '';
  return `${mode || 'player'}:${id}`;
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

/**
 * Reports whether a parsed draft object represents a build the player actually
 * started, as opposed to the bare version envelope a fresh form serializes to.
 * The server-hydration logic uses this to decide whether a usable local draft
 * already exists: an empty `{v:1}` draft (which `parseDraft` reduces to `{}`)
 * must read as "no draft" so a returning player still pulls their saved
 * server-side draft instead of being stuck on a blank form.
 *
 * Returns true when any tracked entry point is truthy/non-empty: a typed
 * `name`, a chosen `race`/`subrace`/`background`, having advanced past the
 * first step (`currentStep`), a selected first-row `class`, or any selected
 * skill / spell / manual equipment item.
 * @param {object|null|undefined} d - parsed builder draft
 * @returns {boolean} true iff the draft has player-entered content
 */
export function draftHasContent(d) {
  if (d == null) return false;
  if (d.name) return true;
  if (d.race) return true;
  if (d.subrace) return true;
  if (d.background) return true;
  if (d.currentStep) return true;
  if (d.classEntries?.[0]?.class) return true;
  if (d.selectedSkills?.length > 0) return true;
  if (d.selectedSpells?.length > 0) return true;
  if (d.manualEquipment?.length > 0) return true;
  return false;
}
