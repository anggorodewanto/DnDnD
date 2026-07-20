/**
 * Turn action-economy override payload builder.
 *
 * The backend endpoint
 * POST /api/combat/{enc}/override/combatant/{comb}/turn-resources takes every
 * resource field as an optional pointer: an omitted key leaves that resource
 * alone, while an explicit `false` / `0` is honoured. The dashboard has no
 * read-side view of turn resources to prefill from, so the editor uses
 * "unchanged" as the default for each control and this builder drops those
 * keys from the body rather than guessing at the turn's current state.
 */

/** Sentinel a tri-state select uses for "leave this resource as it is". */
export const TURN_RESOURCE_UNCHANGED = '';

/**
 * Convert a tri-state select value into a boolean, or undefined for
 * "unchanged". Accepts real booleans so the caller may bind either way.
 * @param {any} v
 * @returns {boolean|undefined}
 */
function toTriStateBool(v) {
  if (typeof v === 'boolean') return v;
  if (v === 'true') return true;
  if (v === 'false') return false;
  return undefined;
}

/**
 * Convert a numeric input value into an int, or undefined when it is blank or
 * unparseable — sending NaN would serialise to `null` and be rejected.
 * @param {any} v
 * @returns {number|undefined}
 */
function toOptionalInt(v) {
  if (v === '' || v === null || v === undefined) return undefined;
  const n = parseInt(v, 10);
  return Number.isFinite(n) ? n : undefined;
}

/**
 * Build the turn-resources override body from raw editor fields. Only fields
 * the DM actually set are included; `reason` is always sent (trimmed) so the
 * backend's "reason is required" 400 stays the single source of truth.
 * @param {{actionUsed?:any, bonusActionUsed?:any, reactionUsed?:any,
 *          movementRemainingFt?:any, attacksRemaining?:any, reason?:string}} fields
 * @returns {object}
 */
export function toTurnResourcesPayload(fields = {}) {
  /** @type {Record<string, any>} */
  const payload = {};

  const bools = {
    action_used: toTriStateBool(fields.actionUsed),
    bonus_action_used: toTriStateBool(fields.bonusActionUsed),
    reaction_used: toTriStateBool(fields.reactionUsed),
  };
  for (const [key, value] of Object.entries(bools)) {
    if (value !== undefined) payload[key] = value;
  }

  const ints = {
    movement_remaining_ft: toOptionalInt(fields.movementRemainingFt),
    attacks_remaining: toOptionalInt(fields.attacksRemaining),
  };
  for (const [key, value] of Object.entries(ints)) {
    if (value !== undefined) payload[key] = value;
  }

  payload.reason = (fields.reason || '').trim();
  return payload;
}
