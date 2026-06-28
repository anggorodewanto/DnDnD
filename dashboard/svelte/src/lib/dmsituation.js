/**
 * DM situation client helper.
 *
 * Wraps the aggregated DM console endpoint (`GET /api/dm/situation`), which the
 * server projects from the authenticated DM's active campaign: the unified
 * pending worklist, live encounter state, recent timeline, and a single
 * suggested next step. Keeping URL + fetch shape in a pure JS module lets us
 * unit-test it under the project's node/vitest harness without a DOM. The
 * endpoint is gated by the DM auth middleware on the server, so this client
 * never sends a campaign id — the server resolves it from the session.
 */

export const DM_SITUATION_ENDPOINT = '/api/dm/situation';

/**
 * Fetch the aggregated DM situation for the authenticated DM's active
 * campaign. Throws on non-OK responses with the response body as message.
 *
 * @param {typeof fetch} [fetchImpl] - injected fetch for tests.
 * @returns {Promise<object>} the situation projection (pending, state,
 *   timeline, next_step).
 */
export async function fetchDMSituation(fetchImpl = fetch) {
  const res = await fetchImpl(DM_SITUATION_ENDPOINT, { credentials: 'same-origin' });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json();
}

/**
 * Format one condition for the DM console. The payload sends each condition as
 * an object `{name, duration_rounds, source_spell, expires_on}` (so the DM can
 * see whether it's one-shot or ongoing); a bare string is tolerated for
 * back-compat. Renders e.g. "poisoned (3r)" or "invisible (greater-invisibility)".
 */
export function formatCondition(cond) {
  if (!cond) return '';
  if (typeof cond === 'string') return cond;
  const name = cond.name || '';
  const bits = [];
  if (cond.duration_rounds > 0) bits.push(`${cond.duration_rounds}r`);
  if (cond.source_spell) bits.push(cond.source_spell);
  return bits.length ? `${name} (${bits.join(', ')})` : name;
}

/** Join a combatant's conditions into a display string (never "[object Object]"). */
export function formatConditions(conds) {
  if (!Array.isArray(conds) || conds.length === 0) return '';
  return conds.map(formatCondition).filter(Boolean).join(', ');
}

/** Death-save tally as a compact "✓1 ✗2" string, or "" when not dying. */
export function formatDeathSaves(ds) {
  if (!ds) return '';
  return `✓${ds.successes || 0} ✗${ds.failures || 0}`;
}
