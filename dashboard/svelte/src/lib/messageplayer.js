/**
 * Message Player client helpers (Phase 102 mobile-lite tab).
 *
 * Wraps the Phase 101 `/api/message-player` handler. Keeping validation and
 * URL construction in a pure JS module allows full unit testing under the
 * project's node/vitest harness without a DOM.
 */

export const MESSAGE_PLAYER_ENDPOINT = '/api/message-player/';
export const MESSAGE_PLAYER_HISTORY_ENDPOINT = '/api/message-player/history';
export const CHARACTER_OVERVIEW_ENDPOINT = '/api/character-overview';

/**
 * Validate the DM's draft message before submission. Returns either
 * `{ ok: true }` or `{ ok: false, error }` with a user-visible reason.
 * @param {{campaignId:string, playerCharacterId:string, authorUserId:string, body:string}} input
 */
export function validateMessagePlayerInput(input) {
  if (!input) return { ok: false, error: 'message body cannot be empty' };
  if (!input.campaignId) return { ok: false, error: 'campaign is required' };
  if (!input.playerCharacterId) return { ok: false, error: 'player must be selected' };
  if (!input.authorUserId) return { ok: false, error: 'author user id missing' };
  if (!input.body || !input.body.trim()) {
    return { ok: false, error: 'message body cannot be empty' };
  }
  return { ok: true };
}

/**
 * Send a DM to a player by POSTing to the Phase 101 handler. Throws on non-ok.
 * @param {{campaignId:string, playerCharacterId:string, authorUserId:string, body:string}} input
 */
export async function sendPlayerMessage(input) {
  const res = await fetch(MESSAGE_PLAYER_ENDPOINT, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      campaign_id: input.campaignId,
      player_character_id: input.playerCharacterId,
      author_user_id: input.authorUserId,
      body: input.body,
    }),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json();
}

/**
 * Build the URL for `/api/message-player/history`.
 * @param {{campaignId:string, playerCharacterId:string, limit?:number, offset?:number}} params
 * @returns {string}
 */
export function buildHistoryUrl({ campaignId, playerCharacterId, limit, offset } = {}) {
  const qs = new URLSearchParams();
  if (campaignId) qs.set('campaign_id', campaignId);
  if (playerCharacterId) qs.set('player_character_id', playerCharacterId);
  if (typeof limit === 'number' && limit > 0) qs.set('limit', String(limit));
  if (typeof offset === 'number' && offset > 0) qs.set('offset', String(offset));
  const s = qs.toString();
  return s ? `${MESSAGE_PLAYER_HISTORY_ENDPOINT}?${s}` : MESSAGE_PLAYER_HISTORY_ENDPOINT;
}

/**
 * Fetch the message history for a given player character.
 * @param {{campaignId:string, playerCharacterId:string, limit?:number, offset?:number}} params
 * @returns {Promise<object[]>}
 */
export async function fetchHistory(params) {
  const res = await fetch(buildHistoryUrl(params), { credentials: 'same-origin' });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json();
}

/**
 * Fetch the campaign's approved characters so the panel can populate a picker.
 * @param {string} campaignId
 * @returns {Promise<{characters: object[]}>}
 */
export async function fetchPartyCharacters(campaignId) {
  if (!campaignId) return { characters: [] };
  const url = `${CHARACTER_OVERVIEW_ENDPOINT}?campaign_id=${encodeURIComponent(campaignId)}`;
  const res = await fetch(url, { credentials: 'same-origin' });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json();
}
