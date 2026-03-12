/**
 * API client for map CRUD operations.
 */

const API_BASE = '/api/maps';

/**
 * Create a new map.
 * @param {object} params - { campaign_id, name, width, height, tiled_json? }
 * @returns {Promise<object>} The created map.
 */
export async function createMap(params) {
  const res = await fetch(API_BASE, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Failed to create map: ${res.status}`);
  }
  return res.json();
}

/**
 * Get a map by ID.
 * @param {string} id - Map UUID.
 * @returns {Promise<object>} The map.
 */
export async function getMap(id) {
  const res = await fetch(`${API_BASE}/${id}`);
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Failed to get map: ${res.status}`);
  }
  return res.json();
}

/**
 * List maps for a campaign.
 * @param {string} campaignId - Campaign UUID.
 * @returns {Promise<object[]>} Array of maps.
 */
export async function listMaps(campaignId) {
  const res = await fetch(`${API_BASE}?campaign_id=${campaignId}`);
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Failed to list maps: ${res.status}`);
  }
  return res.json();
}

/**
 * Update a map.
 * @param {string} id - Map UUID.
 * @param {object} params - { name, width, height, tiled_json }
 * @returns {Promise<object>} The updated map.
 */
export async function updateMap(id, params) {
  const res = await fetch(`${API_BASE}/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Failed to update map: ${res.status}`);
  }
  return res.json();
}

/**
 * Delete a map.
 * @param {string} id - Map UUID.
 * @returns {Promise<void>}
 */
export async function deleteMap(id) {
  const res = await fetch(`${API_BASE}/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Failed to delete map: ${res.status}`);
  }
}
