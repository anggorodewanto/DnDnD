/**
 * API client for map, asset, and encounter operations.
 */

const API_BASE = '/api/maps';
const ENCOUNTERS_BASE = '/api/encounters';

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

/**
 * Create a new map.
 * @param {object} params - { campaign_id, name, width, height, tiled_json? }
 * @returns {Promise<object>} The created map.
 */
export async function createMap(params) {
  const res = await apiFetch(API_BASE, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  });
  return res.json();
}

/**
 * Get a map by ID.
 * @param {string} id - Map UUID.
 * @returns {Promise<object>} The map.
 */
export async function getMap(id) {
  const res = await apiFetch(`${API_BASE}/${id}`);
  return res.json();
}

/**
 * List maps for a campaign.
 * @param {string} campaignId - Campaign UUID.
 * @returns {Promise<object[]>} Array of maps.
 */
export async function listMaps(campaignId) {
  const res = await apiFetch(`${API_BASE}?campaign_id=${campaignId}`);
  return res.json();
}

/**
 * Update a map.
 * @param {string} id - Map UUID.
 * @param {object} params - { name, width, height, tiled_json }
 * @returns {Promise<object>} The updated map.
 */
export async function updateMap(id, params) {
  const res = await apiFetch(`${API_BASE}/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  });
  return res.json();
}

/**
 * Upload an asset file.
 * @param {object} params - { campaignId, type, file }
 * @returns {Promise<object>} The uploaded asset { id, url }.
 */
export async function uploadAsset({ campaignId, type, file }) {
  const formData = new FormData();
  formData.append('campaign_id', campaignId);
  formData.append('type', type);
  formData.append('file', file);

  const res = await apiFetch('/api/assets/upload', {
    method: 'POST',
    body: formData,
  });
  return res.json();
}

/**
 * Delete a map.
 * @param {string} id - Map UUID.
 * @returns {Promise<void>}
 */
export async function deleteMap(id) {
  await apiFetch(`${API_BASE}/${id}`, {
    method: 'DELETE',
  });
}

// --- Encounter API ---

/**
 * Create a new encounter template.
 * @param {object} params - { campaign_id, name, display_name?, map_id?, creatures? }
 * @returns {Promise<object>} The created encounter.
 */
export async function createEncounter(params) {
  const res = await apiFetch(ENCOUNTERS_BASE, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  });
  return res.json();
}

/**
 * Get an encounter template by ID.
 * @param {string} id - Encounter UUID.
 * @returns {Promise<object>} The encounter.
 */
export async function getEncounter(id) {
  const res = await apiFetch(`${ENCOUNTERS_BASE}/${id}`);
  return res.json();
}

/**
 * List encounter templates for a campaign.
 * @param {string} campaignId - Campaign UUID.
 * @returns {Promise<object[]>} Array of encounters.
 */
export async function listEncounters(campaignId) {
  const res = await apiFetch(`${ENCOUNTERS_BASE}?campaign_id=${campaignId}`);
  return res.json();
}

/**
 * Update an encounter template.
 * @param {string} id - Encounter UUID.
 * @param {object} params - { name, display_name?, map_id?, creatures? }
 * @returns {Promise<object>} The updated encounter.
 */
export async function updateEncounter(id, params) {
  const res = await apiFetch(`${ENCOUNTERS_BASE}/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  });
  return res.json();
}

/**
 * Delete an encounter template.
 * @param {string} id - Encounter UUID.
 * @returns {Promise<void>}
 */
export async function deleteEncounter(id) {
  await apiFetch(`${ENCOUNTERS_BASE}/${id}`, {
    method: 'DELETE',
  });
}

/**
 * Duplicate an encounter template.
 * @param {string} id - Encounter UUID.
 * @returns {Promise<object>} The duplicated encounter.
 */
export async function duplicateEncounter(id) {
  const res = await apiFetch(`${ENCOUNTERS_BASE}/${id}/duplicate`, {
    method: 'POST',
  });
  return res.json();
}

/**
 * List creatures (stat blocks).
 * @returns {Promise<object[]>} Array of creatures.
 */
export async function listCreatures() {
  const res = await apiFetch('/api/creatures');
  return res.json();
}

// --- Enemy Turn Builder API ---

const COMBAT_BASE = '/api/combat';

/**
 * Get a suggested turn plan for an NPC combatant.
 * @param {string} encounterId - Encounter UUID.
 * @param {string} combatantId - Combatant UUID.
 * @returns {Promise<object>} The turn plan.
 */
export async function getEnemyTurnPlan(encounterId, combatantId) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/enemy-turn/${combatantId}/plan`);
  return res.json();
}

/**
 * Execute a finalized enemy turn plan.
 * @param {string} encounterId - Encounter UUID.
 * @param {object} plan - { combatant_id, steps }
 * @returns {Promise<object>} The execution result.
 */
export async function executeEnemyTurn(encounterId, plan) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/enemy-turn`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(plan),
  });
  return res.json();
}
