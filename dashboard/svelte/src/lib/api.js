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

/**
 * Get a legendary action plan for an NPC combatant.
 * @param {string} encounterId - Encounter UUID.
 * @param {string} combatantId - Combatant UUID.
 * @param {number} [budgetRemaining] - Optional remaining budget.
 * @returns {Promise<object>} The legendary action plan.
 */
export async function getLegendaryActionPlan(encounterId, combatantId, budgetRemaining) {
  let url = `${COMBAT_BASE}/${encounterId}/legendary/${combatantId}/plan`;
  if (budgetRemaining !== undefined) {
    url += `?budget_remaining=${budgetRemaining}`;
  }
  const res = await apiFetch(url);
  return res.json();
}

/**
 * Execute a legendary action.
 * @param {string} encounterId - Encounter UUID.
 * @param {object} data - { combatant_id, action_name, budget_remaining }
 * @returns {Promise<object>} The execution result.
 */
export async function executeLegendaryAction(encounterId, data) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/legendary`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  return res.json();
}

/**
 * Get a lair action plan for an encounter.
 * @param {string} encounterId - Encounter UUID.
 * @param {string} [lastUsed] - Name of last used lair action.
 * @returns {Promise<object>} The lair action plan.
 */
export async function getLairActionPlan(encounterId, lastUsed) {
  let url = `${COMBAT_BASE}/${encounterId}/lair-action/plan`;
  if (lastUsed) {
    url += `?last_used=${encodeURIComponent(lastUsed)}`;
  }
  const res = await apiFetch(url);
  return res.json();
}

/**
 * Execute a lair action.
 * @param {string} encounterId - Encounter UUID.
 * @param {object} data - { action_name, last_used_action }
 * @returns {Promise<object>} The execution result.
 */
export async function executeLairAction(encounterId, data) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/lair-action`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  return res.json();
}

/**
 * Get the turn queue with legendary/lair action entries.
 * @param {string} encounterId - Encounter UUID.
 * @returns {Promise<object>} The turn queue.
 */
export async function getTurnQueue(encounterId) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/turn-queue`);
  return res.json();
}

// --- Item Picker API ---

/**
 * Search items across weapons, armor, and magic items.
 * @param {string} campaignId - Campaign UUID.
 * @param {object} params - { q?, category? }
 * @returns {Promise<object[]>} Array of search results.
 */
export async function searchItems(campaignId, { q, category } = {}) {
  const params = new URLSearchParams();
  if (q) params.set('q', q);
  if (category) params.set('category', category);
  const res = await apiFetch(`/api/campaigns/${campaignId}/items/search?${params}`);
  return res.json();
}

/**
 * Get creature inventories for a completed encounter (loot context).
 * @param {string} campaignId - Campaign UUID.
 * @param {string} encounterId - Encounter UUID.
 * @returns {Promise<object>} { creatures: [...] }
 */
export async function getCreatureInventories(campaignId, encounterId) {
  const res = await apiFetch(`/api/campaigns/${campaignId}/encounters/${encounterId}/creature-inventories/`);
  return res.json();
}

// --- Shops API ---

/**
 * List shops for a campaign.
 * @param {string} campaignId - Campaign UUID.
 * @returns {Promise<object[]>} Array of shops.
 */
export async function listShops(campaignId) {
  const res = await apiFetch(`/api/campaigns/${campaignId}/shops`);
  return res.json();
}

/**
 * Create a new shop.
 * @param {string} campaignId - Campaign UUID.
 * @param {object} params - { name, description }
 * @returns {Promise<object>} The created shop.
 */
export async function createShop(campaignId, params) {
  const res = await apiFetch(`/api/campaigns/${campaignId}/shops`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  });
  return res.json();
}

/**
 * Get a shop with its items.
 * @param {string} campaignId - Campaign UUID.
 * @param {string} shopId - Shop UUID.
 * @returns {Promise<object>} { shop, items }
 */
export async function getShop(campaignId, shopId) {
  const res = await apiFetch(`/api/campaigns/${campaignId}/shops/${shopId}`);
  return res.json();
}

/**
 * Update a shop.
 * @param {string} campaignId - Campaign UUID.
 * @param {string} shopId - Shop UUID.
 * @param {object} params - { name, description }
 * @returns {Promise<object>} The updated shop.
 */
export async function updateShop(campaignId, shopId, params) {
  const res = await apiFetch(`/api/campaigns/${campaignId}/shops/${shopId}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  });
  return res.json();
}

/**
 * Delete a shop.
 * @param {string} campaignId - Campaign UUID.
 * @param {string} shopId - Shop UUID.
 * @returns {Promise<void>}
 */
export async function deleteShop(campaignId, shopId) {
  await apiFetch(`/api/campaigns/${campaignId}/shops/${shopId}`, {
    method: 'DELETE',
  });
}

/**
 * Add an item to a shop.
 * @param {string} campaignId - Campaign UUID.
 * @param {string} shopId - Shop UUID.
 * @param {object} params - { item_id, name, description, price_gp, quantity, type }
 * @returns {Promise<object>} The created shop item.
 */
export async function addShopItem(campaignId, shopId, params) {
  const res = await apiFetch(`/api/campaigns/${campaignId}/shops/${shopId}/items`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  });
  return res.json();
}

/**
 * Remove an item from a shop.
 * @param {string} campaignId - Campaign UUID.
 * @param {string} shopId - Shop UUID.
 * @param {string} itemId - Item UUID.
 * @returns {Promise<void>}
 */
export async function removeShopItem(campaignId, shopId, itemId) {
  await apiFetch(`/api/campaigns/${campaignId}/shops/${shopId}/items/${itemId}`, {
    method: 'DELETE',
  });
}

/**
 * Post a shop to Discord's #the-story channel.
 * @param {string} campaignId - Campaign UUID.
 * @param {string} shopId - Shop UUID.
 * @returns {Promise<object>} { status, message }
 */
export async function postShopToDiscord(campaignId, shopId) {
  const res = await apiFetch(`/api/campaigns/${campaignId}/shops/${shopId}/post`, {
    method: 'POST',
  });
  return res.json();
}

// --- Combat Workspace API ---

/**
 * Get combat workspace data for a campaign (active encounters, combatants, maps, zones).
 * @param {string} campaignId - Campaign UUID.
 * @returns {Promise<object>} { encounters: [...] }
 */
export async function getCombatWorkspace(campaignId) {
  const res = await apiFetch(`${COMBAT_BASE}/workspace?campaign_id=${campaignId}`);
  return res.json();
}

/**
 * Update a combatant's HP.
 * @param {string} encounterId - Encounter UUID.
 * @param {string} combatantId - Combatant UUID.
 * @param {object} data - { hp_current, temp_hp, is_alive }
 * @returns {Promise<object>} Updated combatant.
 */
export async function updateCombatantHP(encounterId, combatantId, data) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/combatants/${combatantId}/hp`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  return res.json();
}

/**
 * Update a combatant's conditions.
 * @param {string} encounterId - Encounter UUID.
 * @param {string} combatantId - Combatant UUID.
 * @param {string[]} conditions - Array of condition names.
 * @returns {Promise<object>} Updated combatant.
 */
export async function updateCombatantConditions(encounterId, combatantId, conditions) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/combatants/${combatantId}/conditions`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ conditions }),
  });
  return res.json();
}
