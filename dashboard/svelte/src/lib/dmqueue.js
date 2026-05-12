/**
 * DM queue list helpers (F-12 dashboard aggregator).
 *
 * Wraps `GET /dashboard/queue` so the DM dashboard panel can render every
 * pending dm-queue item (whisper, freeform, rest, etc.) in one place
 * instead of forcing the DM to scroll Discord #dm-queue. Keeping the URL
 * builder and kind-icon lookup in a pure JS module lets us unit-test
 * them under the project's node/vitest harness without a DOM.
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
    default:
      return '??';
  }
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
