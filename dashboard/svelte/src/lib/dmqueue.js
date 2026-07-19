/**
 * DM queue client helpers.
 *
 * Wraps the dashboard's per-campaign queue list (`GET /dashboard/queue/`),
 * per-item detail (`GET /dashboard/queue/{id}`), and JSON resolve/reply/narrate
 * POSTs so the SPA can drive the whole resolver flow inline. Keeping URL
 * construction, payload shape, and kind-icon lookup in a pure JS module lets
 * us unit-test them under the project's node/vitest harness without a DOM.
 */

export const DM_QUEUE_LIST_ENDPOINT = '/dashboard/queue/';

/**
 * Map a queue item kind to a single-glyph icon. Mirrors the Discord
 * formatter in internal/dmqueue/format.go so the dashboard and #dm-queue
 * surface the same vocabulary at a glance. Unknown kinds fall through to a
 * generic notification icon.
 *
 * @param {string} kind - the EventKind string (e.g. "player_whisper").
 * @returns {string} the icon glyph.
 */
export function iconForKind(kind) {
  switch (kind) {
    case 'freeform_action':
      return 'AC';
    case 'reaction_declaration':
      return 'RX';
    case 'rest_request':
      return 'RS';
    case 'skill_check_narration':
      return 'CK';
    case 'consumable':
      return 'IT';
    case 'enemy_turn_ready':
      return 'EN';
    case 'narrative_teleport':
      return 'SP';
    case 'player_whisper':
      return 'WH';
    case 'undo_request':
      return 'UN';
    case 'retire_request':
      return 'RT';
    case 'initiative_staged':
      return 'IN';
    default:
      return '??';
  }
}

/**
 * Build the per-item detail URL. Same path the POST handlers live under,
 * but issued as GET to fetch the JSON projection used by the Svelte
 * detail view.
 *
 * @param {string} itemID
 * @returns {string}
 */
export function buildItemEndpoint(itemID) {
  return `/dashboard/queue/${encodeURIComponent(itemID)}`;
}

/**
 * Build the JSON resolve endpoint URL for the generic outcome flow.
 * @param {string} itemID
 * @returns {string}
 */
export function buildResolveEndpoint(itemID) {
  return `${buildItemEndpoint(itemID)}/resolve`;
}

/**
 * Build the JSON whisper-reply endpoint URL.
 * @param {string} itemID
 * @returns {string}
 */
export function buildReplyEndpoint(itemID) {
  return `${buildItemEndpoint(itemID)}/reply`;
}

/**
 * Build the JSON skill-check-narration endpoint URL.
 * @param {string} itemID
 * @returns {string}
 */
export function buildNarrateEndpoint(itemID) {
  return `${buildItemEndpoint(itemID)}/narrate`;
}

/**
 * Fetch the pending dm-queue list for the authenticated DM's active
 * campaign. Throws on non-OK responses with the response body as message.
 * The endpoint is gated by dmAuthMw (F-2) on the server, so this client
 * never has to send the campaign id — the server resolves it from the
 * authenticated session.
 *
 * @param {typeof fetch} [fetchImpl] - injected fetch for tests.
 * @returns {Promise<Array<object>>} list of pending dm-queue entries.
 */
export async function fetchDMQueueList(fetchImpl = fetch) {
  const res = await fetchImpl(DM_QUEUE_LIST_ENDPOINT, { credentials: 'same-origin' });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json();
}

/**
 * Fetch a single dm-queue item as JSON. Returns the projection used by the
 * Svelte detail view (status flags, outcome, kind label, etc.).
 *
 * @param {string} itemID
 * @param {typeof fetch} [fetchImpl]
 * @returns {Promise<object>}
 */
export async function fetchDMQueueItem(itemID, fetchImpl = fetch) {
  const res = await fetchImpl(buildItemEndpoint(itemID), { credentials: 'same-origin' });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json();
}

/**
 * Internal: POST a JSON payload to the given endpoint and surface a useful
 * error message on non-OK. Returns the parsed JSON response when the
 * server provides one (handlers may return 204/empty body — caller decides).
 *
 * @param {string} url
 * @param {object} payload
 * @param {typeof fetch} fetchImpl
 */
async function postJSON(url, payload, fetchImpl) {
  const res = await fetchImpl(url, {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res;
}

/**
 * Resolve a generic pending item by submitting an outcome string.
 *
 * @param {string} itemID
 * @param {string} outcome
 * @param {typeof fetch} [fetchImpl]
 */
export async function resolveItem(itemID, outcome, fetchImpl = fetch) {
  return postJSON(buildResolveEndpoint(itemID), { outcome }, fetchImpl);
}

/**
 * Resolve a whisper item by submitting a DM reply for delivery.
 *
 * @param {string} itemID
 * @param {string} reply
 * @param {typeof fetch} [fetchImpl]
 */
export async function replyItem(itemID, reply, fetchImpl = fetch) {
  return postJSON(buildReplyEndpoint(itemID), { reply }, fetchImpl);
}

/**
 * Resolve a skill-check-narration item by submitting the channel narration.
 *
 * @param {string} itemID
 * @param {string} narration
 * @param {typeof fetch} [fetchImpl]
 */
export async function narrateItem(itemID, narration, fetchImpl = fetch) {
  return postJSON(buildNarrateEndpoint(itemID), { narration }, fetchImpl);
}
