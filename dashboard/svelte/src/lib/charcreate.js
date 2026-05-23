/**
 * Character-creation API helpers.
 *
 * Wraps the seven JSON endpoints exposed by
 * internal/dashboard/charcreate_handler.go so the Svelte CharCreatePanel
 * does not need to inline `fetch` calls or repeat error handling.
 *
 * All helpers throw on non-OK responses with the response body as the
 * message, matching the apiFetch-style contract in lib/api.js.
 */

export const CHARCREATE_BASE = '/dashboard/api/characters';

/**
 * Perform a fetch and throw on non-OK responses.
 *
 * @param {string} url
 * @param {RequestInit} [options]
 * @returns {Promise<Response>}
 */
async function apiFetch(url, options) {
  const res = await fetch(url, options);
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res;
}

/**
 * Fetch the list of races available for character creation.
 *
 * @returns {Promise<object[]>}
 */
export async function fetchRaces() {
  const res = await apiFetch(`${CHARCREATE_BASE}/ref/races`);
  return res.json();
}

/**
 * Fetch the list of classes available for character creation.
 *
 * @returns {Promise<object[]>}
 */
export async function fetchClasses() {
  const res = await apiFetch(`${CHARCREATE_BASE}/ref/classes`);
  return res.json();
}

/**
 * Fetch the list of campaign-scoped equipment items.
 *
 * @param {string} [campaignId]
 * @returns {Promise<object[]>}
 */
export async function fetchEquipment(campaignId = '') {
  const qs = campaignId ? `?campaign_id=${encodeURIComponent(campaignId)}` : '';
  const res = await apiFetch(`${CHARCREATE_BASE}/ref/equipment${qs}`);
  return res.json();
}

/**
 * Fetch starting equipment packs for the named class.
 *
 * @param {string} className
 * @returns {Promise<object[]>}
 */
export async function fetchStartingEquipment(className) {
  if (!className) throw new Error('class is required');
  const res = await apiFetch(
    `${CHARCREATE_BASE}/ref/starting-equipment?class=${encodeURIComponent(className)}`,
  );
  return res.json();
}

/**
 * Fetch the spell list for the named class, optionally filtered by max level.
 *
 * @param {string} className
 * @param {{ maxLevel?: number, campaignId?: string }} [opts]
 * @returns {Promise<object[]>}
 */
export async function fetchSpells(className, opts = {}) {
  if (!className) throw new Error('class is required');
  const params = new URLSearchParams({ class: className });
  if (typeof opts.maxLevel === 'number' && opts.maxLevel > 0) {
    params.set('max_level', String(opts.maxLevel));
  }
  if (opts.campaignId) params.set('campaign_id', opts.campaignId);
  const res = await apiFetch(`${CHARCREATE_BASE}/ref/spells?${params}`);
  return res.json();
}

/**
 * Fetch the campaign-enabled ability score methods.
 *
 * @param {string} [campaignId]
 * @returns {Promise<string[]>}
 */
export async function fetchAbilityMethods(campaignId = '') {
  const qs = campaignId ? `?campaign_id=${encodeURIComponent(campaignId)}` : '';
  const res = await apiFetch(`${CHARCREATE_BASE}/ability-methods${qs}`);
  return res.json();
}

/**
 * POST a draft submission to /preview to compute derived stats (HP, AC,
 * spell slots, features, etc.) without persisting.
 *
 * @param {object} submission
 * @returns {Promise<object>}
 */
export async function previewCharacter(submission) {
  const res = await apiFetch(`${CHARCREATE_BASE}/preview`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(submission),
  });
  return res.json();
}

/**
 * Create the character. Returns { character_id, player_character_id }.
 *
 * @param {{ campaign_id?: string } & object} payload
 * @returns {Promise<object>}
 */
export async function createCharacter(payload) {
  const res = await apiFetch(CHARCREATE_BASE, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return res.json();
}

/**
 * Parse a subclass payload, which Go encodes either as a JSON array or
 * a JSON object keyed by subclass id. Returns a flat array of
 * { value, label } pairs for use in a `<select>`.
 *
 * Accepts either a raw value (already-parsed JSON) or a string.
 *
 * @param {unknown} subclasses
 * @returns {{ value: string, label: string }[]}
 */
export function parseSubclasses(subclasses) {
  if (!subclasses) return [];
  let value = subclasses;
  if (typeof value === 'string') {
    try {
      value = JSON.parse(value);
    } catch {
      return [];
    }
  }
  if (Array.isArray(value)) {
    return value.map((s) => {
      const name = (s && typeof s === 'object' ? s.name : s) || '';
      return { value: name, label: name };
    });
  }
  if (typeof value === 'object') {
    return Object.keys(value).map((key) => {
      const entry = value[key] || {};
      const name = entry.name || key;
      return { value: name, label: name };
    });
  }
  return [];
}

/**
 * Roll a single 4d6-drop-lowest score and return both the sum and the
 * four individual dice (so the server-side ability-roll validator can
 * accept the submission). Uses Math.random() unless a custom RNG is
 * provided (mostly for tests).
 *
 * @param {() => number} [rng] - returns a float in [0, 1)
 * @returns {{ score: number, dice: number[] }}
 */
export function roll4d6DropLowest(rng = Math.random) {
  const dice = [];
  for (let i = 0; i < 4; i += 1) {
    dice.push(Math.floor(rng() * 6) + 1);
  }
  const sorted = dice.slice().sort((a, b) => a - b);
  const score = sorted[1] + sorted[2] + sorted[3];
  return { score, dice };
}

/**
 * Standard array preset (the SRD-standard 15/14/13/12/10/8 layout).
 *
 * @returns {{ str: number, dex: number, con: number, int: number, wis: number, cha: number }}
 */
export function standardArrayScores() {
  return { str: 15, dex: 14, con: 13, int: 12, wis: 10, cha: 8 };
}

/**
 * Point buy starting scores (all 8s).
 *
 * @returns {{ str: number, dex: number, con: number, int: number, wis: number, cha: number }}
 */
export function pointBuyStartingScores() {
  return { str: 8, dex: 8, con: 8, int: 8, wis: 8, cha: 8 };
}

/**
 * Human-readable label for an ability-score method id.
 *
 * @param {string} method
 * @returns {string}
 */
export function labelForAbilityMethod(method) {
  if (method === 'point_buy') return 'Point Buy';
  if (method === 'standard_array') return 'Standard Array';
  if (method === 'roll') return 'Roll';
  return method;
}
