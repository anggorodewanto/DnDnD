/**
 * Exploration dashboard client helpers.
 *
 * Wraps the JSON exploration endpoints so the Svelte panel can fetch the
 * current campaign's maps, start an exploration encounter, and flip an
 * encounter to combat. Keeping URL construction, payload shaping, and
 * input validation in a pure JS module lets us exercise them under the
 * project's node/vitest harness without a DOM.
 */

export const EXPLORATION_API_ENDPOINT = '/api/exploration';
export const EXPLORATION_START_ENDPOINT = '/dashboard/exploration/start';
export const EXPLORATION_TRANSITION_ENDPOINT = '/dashboard/exploration/transition-to-combat';

/**
 * Fetch the maps + current state needed to render the exploration panel.
 *
 * @param {string} campaignId
 * @param {typeof fetch} [fetchImpl]
 * @returns {Promise<{campaign_id:string, maps:Array<object>}>}
 */
export async function fetchExplorationData(campaignId, fetchImpl = fetch) {
  if (!campaignId) {
    throw new Error('campaign id is required');
  }
  const url = `${EXPLORATION_API_ENDPOINT}?campaign_id=${encodeURIComponent(campaignId)}`;
  const res = await fetchImpl(url, { credentials: 'same-origin' });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json();
}

/**
 * Start a new exploration encounter.
 *
 * @param {{campaignId:string, mapId:string, name?:string, characterIds?:string[]}} input
 * @param {typeof fetch} [fetchImpl]
 * @returns {Promise<object>}
 */
export async function startExploration(input, fetchImpl = fetch) {
  if (!input || !input.campaignId) {
    throw new Error('campaign id is required');
  }
  if (!input.mapId) {
    throw new Error('map id is required');
  }
  const payload = { campaign_id: input.campaignId, map_id: input.mapId };
  if (input.name && input.name.trim() !== '') {
    payload.name = input.name;
  }
  if (Array.isArray(input.characterIds) && input.characterIds.length > 0) {
    payload.character_ids = input.characterIds;
  }
  const res = await fetchImpl(EXPLORATION_START_ENDPOINT, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json();
}

/**
 * Transition an exploration encounter to combat, optionally overriding PC
 * positions.
 *
 * @param {{encounterId:string, overrides?:Record<string,string>}} input
 * @param {typeof fetch} [fetchImpl]
 * @returns {Promise<{mode:string, positions:Record<string,{col:string,row:number}>}>}
 */
export async function transitionToCombat(input, fetchImpl = fetch) {
  if (!input || !input.encounterId) {
    throw new Error('encounter id is required');
  }
  const payload = { encounter_id: input.encounterId };
  if (input.overrides && Object.keys(input.overrides).length > 0) {
    payload.overrides = input.overrides;
  }
  const res = await fetchImpl(EXPLORATION_TRANSITION_ENDPOINT, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json();
}

/**
 * Parse a textarea of `<character_id>=<coord>` lines into the overrides map
 * accepted by the transition endpoint. Blank lines are ignored; whitespace
 * around either side of `=` is trimmed.
 *
 * @param {string} text
 * @returns {Record<string,string>}
 */
export function parseOverridesText(text) {
  const out = {};
  if (!text || text.trim() === '') {
    return out;
  }
  const lines = text.split(/\r?\n/);
  for (const raw of lines) {
    const line = raw.trim();
    if (line === '') continue;
    const idx = line.indexOf('=');
    if (idx < 0) {
      throw new Error(`override line "${line}" is not in <char_id>=<coord> format`);
    }
    const charId = line.slice(0, idx).trim();
    const coord = line.slice(idx + 1).trim();
    if (charId === '' || coord === '') {
      throw new Error(`override line "${line}" is missing a side`);
    }
    out[charId] = coord;
  }
  return out;
}
