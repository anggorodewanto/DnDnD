/**
 * Portal API client for the character builder.
 */

const API_BASE = '/portal/api';

/**
 * Perform a fetch and throw on non-OK responses.
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

/** List all races. */
export async function listRaces() {
  const res = await apiFetch(`${API_BASE}/races`);
  return res.json();
}

/** List all classes. */
export async function listClasses() {
  const res = await apiFetch(`${API_BASE}/classes`);
  return res.json();
}

/**
 * List spells filtered by class. The optional campaignId scopes Open5e
 * extended content to the campaign's enabled sources — omitting it (or
 * passing an empty string) hides all Open5e rows as a safe default.
 * @param {string} className
 * @param {string} [campaignId]
 * @returns {Promise<object[]>}
 */
export async function listSpells(className, campaignId) {
  let url = `${API_BASE}/spells?class=${encodeURIComponent(className)}`;
  if (campaignId) {
    url += `&campaign_id=${encodeURIComponent(campaignId)}`;
  }
  const res = await apiFetch(url);
  return res.json();
}

/** List all equipment (weapons + armor). */
export async function listEquipment() {
  const res = await apiFetch(`${API_BASE}/equipment`);
  return res.json();
}

/**
 * Get starting equipment packs for a class.
 * @param {string} className
 * @returns {Promise<object[]>}
 */
export async function getStartingEquipment(className) {
  const res = await apiFetch(`${API_BASE}/starting-equipment?class=${encodeURIComponent(className)}`);
  return res.json();
}

/**
 * Submit a character for DM approval.
 * @param {object} data - The character submission payload.
 * @returns {Promise<object>} { character_id, player_character_id }
 */
export async function submitCharacter(data) {
  const res = await apiFetch(`${API_BASE}/characters`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  return res.json();
}
