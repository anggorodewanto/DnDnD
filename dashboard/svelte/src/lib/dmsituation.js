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
