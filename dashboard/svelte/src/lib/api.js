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
 * Import a Tiled `.tmj` file as a new map. Reads the file as text, parses it
 * as JSON, and POSTs the payload to /api/maps/import. The backend validates
 * the .tmj, strips unsupported features, and returns
 * { map: {...}, skipped: [...] }.
 *
 * @param {object} params - { campaignId, name, file, backgroundImageId? }
 * @param {string} params.campaignId - Campaign UUID.
 * @param {string} params.name - Map name (required by the backend).
 * @param {File}   params.file - The .tmj file selected by the user.
 * @param {string} [params.backgroundImageId] - Optional asset UUID.
 * @returns {Promise<{map: object, skipped: object[]}>}
 */
export async function importTiledMap({ campaignId, name, file, backgroundImageId }) {
  const text = await file.text();
  let tmj;
  try {
    tmj = JSON.parse(text);
  } catch (e) {
    throw new Error('Selected file is not valid JSON');
  }
  const body = {
    campaign_id: campaignId,
    name,
    tmj,
  };
  if (backgroundImageId) {
    body.background_image_id = backgroundImageId;
  }
  const res = await apiFetch(`${API_BASE}/import`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
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
 * Start combat for a prepared encounter template. Mirrors the Go
 * `startCombatRequest` shape in internal/combat/handler.go.
 *
 * @param {object} payload
 * @param {string} payload.template_id - Encounter template UUID.
 * @param {string[]} [payload.character_ids] - PC UUIDs joining combat.
 * @param {Object<string,{col:string,row:number}>} [payload.character_positions]
 *   - Optional map of character UUID to starting position.
 * @param {string[]} [payload.surprised_combatant_short_ids] - Short IDs
 *   (e.g. "GB2") of combatants the DM is flagging as surprised for round 1.
 *   Phase 114.
 * @returns {Promise<object>} The start-combat response: encounter, combatants,
 *   initiative_tracker, first_turn.
 */
export async function startCombat(payload) {
  const res = await apiFetch(`${COMBAT_BASE}/start`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return res.json();
}

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
 * Update the player-facing display name of an encounter.
 * Passing an empty string clears the override so the label falls back
 * to the encounter's internal name.
 * @param {string} encounterId - Encounter UUID.
 * @param {string} name - New display name, or '' to clear.
 * @returns {Promise<object>} Updated encounter response.
 */
export async function updateEncounterDisplayName(encounterId, name) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/display-name`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ display_name: name }),
  });
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
 * Update a combatant's position.
 * @param {string} encounterId - Encounter UUID.
 * @param {string} combatantId - Combatant UUID.
 * @param {object} data - { position_col, position_row }
 * @returns {Promise<object>} Updated combatant.
 */
export async function updateCombatantPosition(encounterId, combatantId, data) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/combatants/${combatantId}/position`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  return res.json();
}

/**
 * Remove a combatant from an encounter.
 * @param {string} encounterId - Encounter UUID.
 * @param {string} combatantId - Combatant UUID.
 * @returns {Promise<void>}
 */
export async function removeCombatant(encounterId, combatantId) {
  await apiFetch(`${COMBAT_BASE}/${encounterId}/combatants/${combatantId}`, {
    method: 'DELETE',
  });
}

/**
 * Advance to the next turn in an encounter.
 * @param {string} encounterId - Encounter UUID.
 * @returns {Promise<object>} The new turn info.
 */
export async function advanceTurn(encounterId) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/advance-turn`, {
    method: 'POST',
  });
  return res.json();
}

/**
 * Get pending actions for an encounter.
 * @param {string} encounterId - Encounter UUID.
 * @returns {Promise<object[]>} Array of pending actions.
 */
export async function getPendingActions(encounterId) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/pending-actions`);
  return res.json();
}

/**
 * Resolve a pending action.
 * @param {string} encounterId - Encounter UUID.
 * @param {string} actionId - Action UUID.
 * @param {object} data - { outcome, effects }
 * @returns {Promise<object>} Resolved action.
 */
export async function resolvePendingAction(encounterId, actionId, data) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/pending-actions/${actionId}/resolve`, {
    method: 'POST',
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

// --- Reactions Panel API ---

/**
 * List all reaction declarations for an encounter, enriched with combatant info.
 * @param {string} encounterId - Encounter UUID.
 * @returns {Promise<object[]>} Array of enriched reaction panel entries.
 */
export async function listReactionsPanel(encounterId) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/reactions/panel`);
  return res.json();
}

/**
 * Resolve (use) a reaction declaration.
 * @param {string} encounterId - Encounter UUID.
 * @param {string} reactionId - Reaction UUID.
 * @returns {Promise<object>} The resolved reaction.
 */
export async function resolveReaction(encounterId, reactionId) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/reactions/${reactionId}/resolve`, {
    method: 'POST',
  });
  return res.json();
}

/**
 * Cancel (dismiss) a reaction declaration.
 * @param {string} encounterId - Encounter UUID.
 * @param {string} reactionId - Reaction UUID.
 * @returns {Promise<object>} The cancelled reaction.
 */
export async function cancelReaction(encounterId, reactionId) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/reactions/${reactionId}/cancel`, {
    method: 'POST',
  });
  return res.json();
}

// --- Undo & Manual Override API (Phase 97b) ---

/**
 * Undo the most recent mutation in the current turn.
 * @param {string} encounterId - Encounter UUID.
 * @param {string} [reason] - Optional reason text shown in the Discord correction.
 * @returns {Promise<object>} Status payload.
 */
export async function undoLastAction(encounterId, reason = '') {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/undo-last-action`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ reason }),
  });
  return res.json();
}

/**
 * Manually override a combatant's HP.
 * @param {string} encounterId
 * @param {string} combatantId
 * @param {{hp_current:number, temp_hp:number, reason?:string}} payload
 */
export async function overrideCombatantHP(encounterId, combatantId, payload) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/override/combatant/${combatantId}/hp`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return res.json();
}

/**
 * Manually override a combatant's position.
 * @param {string} encounterId
 * @param {string} combatantId
 * @param {{position_col:string, position_row:number, altitude_ft?:number, reason?:string}} payload
 */
export async function overrideCombatantPosition(encounterId, combatantId, payload) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/override/combatant/${combatantId}/position`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ altitude_ft: 0, ...payload }),
  });
  return res.json();
}

/**
 * Manually override a combatant's conditions.
 * @param {string} encounterId
 * @param {string} combatantId
 * @param {{conditions:object[], reason?:string}} payload
 */
export async function overrideCombatantConditions(encounterId, combatantId, payload) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/override/combatant/${combatantId}/conditions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return res.json();
}

/**
 * Manually override a combatant's initiative.
 * @param {string} encounterId
 * @param {string} combatantId
 * @param {{initiative_roll:number, initiative_order:number, reason?:string}} payload
 */
export async function overrideCombatantInitiative(encounterId, combatantId, payload) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/override/combatant/${combatantId}/initiative`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return res.json();
}

/**
 * Manually override a character's spell slots.
 * @param {string} encounterId
 * @param {string} characterId
 * @param {{spell_slots:object, reason?:string}} payload
 */
export async function overrideCharacterSpellSlots(encounterId, characterId, payload) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/override/character/${characterId}/spell-slots`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return res.json();
}

// --- Narration API (Phase 100a) ---

const NARRATION_BASE = '/api/narration';

/**
 * Render a narration preview on the server.
 * @param {string} body
 */
export async function previewNarration(body) {
  const res = await apiFetch(`${NARRATION_BASE}/preview`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ body }),
  });
  return res.json();
}

/**
 * Post a narration to #the-story.
 * @param {{campaign_id:string, author_user_id:string, body:string, attachment_asset_ids?:string[]}} payload
 */
export async function postNarration(payload) {
  const res = await apiFetch(`${NARRATION_BASE}/post`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return res.json();
}

/**
 * List recent narration posts for a campaign.
 * @param {string} campaignId
 * @param {number} [limit]
 */
export async function listNarrationHistory(campaignId, limit = 20) {
  const res = await apiFetch(`${NARRATION_BASE}/history?campaign_id=${campaignId}&limit=${limit}`);
  return res.json();
}

// --- Narration Templates API (Phase 100b) ---

/**
 * List narration templates for a campaign with optional filters.
 * @param {string} campaignId
 * @param {{category?: string, q?: string}} [filters]
 */
export async function listNarrationTemplates(campaignId, filters = {}) {
  const params = new URLSearchParams({ campaign_id: campaignId });
  if (filters.category) params.set('category', filters.category);
  if (filters.q) params.set('q', filters.q);
  const res = await apiFetch(`${NARRATION_BASE}/templates?${params}`);
  return res.json();
}

/**
 * Create a new narration template.
 * @param {{campaign_id: string, name: string, category?: string, body: string}} payload
 */
export async function createNarrationTemplate(payload) {
  const res = await apiFetch(`${NARRATION_BASE}/templates`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return res.json();
}

/**
 * Update a narration template by id.
 * @param {string} id
 * @param {{name: string, category?: string, body: string}} payload
 */
export async function updateNarrationTemplate(id, payload) {
  const res = await apiFetch(`${NARRATION_BASE}/templates/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return res.json();
}

/**
 * Delete a narration template by id.
 * @param {string} id
 */
export async function deleteNarrationTemplate(id) {
  await apiFetch(`${NARRATION_BASE}/templates/${id}`, { method: 'DELETE' });
}

/**
 * Duplicate a narration template by id.
 * @param {string} id
 */
export async function duplicateNarrationTemplate(id) {
  const res = await apiFetch(`${NARRATION_BASE}/templates/${id}/duplicate`, {
    method: 'POST',
  });
  return res.json();
}

// --- Action Log Viewer API ---

/**
 * List action log entries for an encounter with optional filters.
 * @param {string} encounterId - Encounter UUID.
 * @param {object} [filters] - Optional filters.
 * @param {string[]} [filters.actionTypes] - Action types to include.
 * @param {string} [filters.actorId] - Actor combatant UUID.
 * @param {string} [filters.targetId] - Target combatant UUID.
 * @param {number} [filters.round] - Round number.
 * @param {string} [filters.turnId] - Turn UUID.
 * @param {'asc'|'desc'} [filters.sort] - Sort order (default desc, newest-first).
 * @returns {Promise<object[]>} Array of action log viewer entries.
 */
export async function listActionLog(encounterId, filters = {}) {
  const params = new URLSearchParams();
  if (filters.actionTypes && filters.actionTypes.length > 0) {
    params.set('action_type', filters.actionTypes.join(','));
  }
  if (filters.actorId) params.set('actor_id', filters.actorId);
  if (filters.targetId) params.set('target_id', filters.targetId);
  if (filters.round) params.set('round', String(filters.round));
  if (filters.turnId) params.set('turn_id', filters.turnId);
  if (filters.sort) params.set('sort', filters.sort);

  const qs = params.toString();
  const url = qs
    ? `${COMBAT_BASE}/${encounterId}/action-log?${qs}`
    : `${COMBAT_BASE}/${encounterId}/action-log`;
  const res = await apiFetch(url);
  return res.json();
}

// --- F-8: per-campaign Open5e source toggle ---

/**
 * Fetch the curated catalog of Open5e source documents the DM can toggle
 * per campaign. The catalog itself is global (not campaign-scoped).
 *
 * @returns {Promise<{sources: Array<{slug: string, title: string, publisher?: string, description?: string}>}>}
 */
export async function listOpen5eSources() {
  const res = await apiFetch('/api/open5e/sources');
  return res.json();
}

/**
 * Read the slugs of Open5e documents currently enabled for a campaign.
 * Always returns a `{campaign_id, enabled}` shape with `enabled` an array
 * (possibly empty).
 *
 * @param {string} campaignId
 * @returns {Promise<{campaign_id: string, enabled: string[]}>}
 */
export async function getCampaignOpen5eSources(campaignId) {
  const res = await apiFetch(`/api/open5e/campaigns/${campaignId}/sources`);
  return res.json();
}

/**
 * Replace the campaign's `open5e_sources` JSONB list with the given slugs.
 * Other settings fields (turn_timeout_hours, channel_ids, ...) are
 * preserved server-side. Slugs that are not in the catalog are rejected
 * with a 400.
 *
 * @param {string} campaignId
 * @param {string[]} enabled
 * @returns {Promise<{campaign_id: string, enabled: string[]}>}
 */
export async function updateCampaignOpen5eSources(campaignId, enabled) {
  const res = await apiFetch(`/api/open5e/campaigns/${campaignId}/sources`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
  });
  return res.json();
}
