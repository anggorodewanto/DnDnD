/**
 * Character status editor helpers (DM out-of-combat HP / condition control).
 *
 * The DM dashboard's vitest harness runs under the `node` environment with no
 * DOM, so Svelte components can't be rendered in unit tests. Following the
 * messageplayer.js / inventoryEditor.js pattern, the fetch logic, endpoint
 * construction, and request-body shaping live in this pure module so they can
 * be exercised directly with a mocked `fetch`. CharacterOverview.svelte stays a
 * thin presentation layer that imports these helpers.
 */

export const CHARACTER_OVERVIEW_ENDPOINT = '/api/character-overview';

// The 14 conditions a DM can toggle on a character outside combat. Exhaustion
// is tracked separately as a 0-6 numeric level and is intentionally NOT in
// this list.
export const STATUS_CONDITIONS = [
  'blinded',
  'charmed',
  'deafened',
  'frightened',
  'grappled',
  'incapacitated',
  'invisible',
  'paralyzed',
  'petrified',
  'poisoned',
  'prone',
  'restrained',
  'stunned',
  'unconscious',
];

export const MAX_EXHAUSTION = 6;

/**
 * Build the status-save endpoint for a single character:
 *   POST /api/character-overview/{characterID}/status
 * @param {string} characterId
 * @returns {string}
 */
export function statusEndpoint(characterId) {
  return `${CHARACTER_OVERVIEW_ENDPOINT}/${encodeURIComponent(characterId)}/status`;
}

/**
 * Coerce raw editor fields into the JSON body the backend expects. Numbers are
 * coerced to ints, exhaustion is clamped to 0-6, conditions are filtered down
 * to the known set, and an empty/whitespace-only reason is omitted.
 * @param {{hpMax?:any, hpCurrent?:any, tempHp?:any, exhaustionLevel?:any, conditions?:string[], reason?:string}} fields
 * @returns {{hp_max:number, hp_current:number, temp_hp:number, exhaustion_level:number, conditions:string[], reason?:string}}
 */
export function toStatusPayload(fields = {}) {
  const toInt = (v) => {
    const n = parseInt(v, 10);
    return Number.isFinite(n) ? n : 0;
  };
  const exhaustion = Math.max(0, Math.min(MAX_EXHAUSTION, toInt(fields.exhaustionLevel)));
  const conditions = Array.isArray(fields.conditions)
    ? fields.conditions.filter((c) => STATUS_CONDITIONS.includes(c))
    : [];
  const payload = {
    hp_max: toInt(fields.hpMax),
    hp_current: toInt(fields.hpCurrent),
    temp_hp: toInt(fields.tempHp),
    exhaustion_level: exhaustion,
    conditions,
  };
  const reason = (fields.reason || '').trim();
  if (reason) payload.reason = reason;
  return payload;
}

/**
 * POST a character's new out-of-combat status. Throws Error(serverText) on any
 * non-2xx response so the caller can surface the backend's explanation verbatim
 * (e.g. the 409 "use combat controls" message, or 403/400/404 text). Returns
 * the parsed JSON body on success, tolerating an empty body.
 * @param {string} characterId  the card's `character_id`
 * @param {object} fields        raw editor fields (see toStatusPayload)
 * @returns {Promise<object>}
 */
export async function saveCharacterStatus(characterId, fields) {
  const res = await fetch(statusEndpoint(characterId), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify(toStatusPayload(fields)),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  try {
    return await res.json();
  } catch (_) {
    return {};
  }
}
