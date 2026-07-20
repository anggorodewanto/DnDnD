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

export async function getCurrentUser() {
  const res = await apiFetch('/api/me');
  return res.json();
}

export async function listCampaigns() {
  const res = await apiFetch('/api/campaigns');
  return res.json();
}

export async function createCampaign(params) {
  const res = await apiFetch('/api/campaigns', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  });
  return res.json();
}

/**
 * Set the authenticated DM's active campaign (T20 / Finding 12). The dashboard
 * then binds Maps/Encounters/Party to this campaign instead of silently
 * following the most-recently-created one.
 * @param {string} id - Campaign UUID to activate.
 * @returns {Promise<object>} { campaign_id, status }
 */
export async function setActiveCampaign(id) {
  const res = await apiFetch(`/api/campaigns/${id}/set-active`, {
    method: 'POST',
  });
  return res.json();
}

/**
 * List the Discord guilds the bot is currently in, for the campaign-create
 * guild dropdown (T20 / Finding 12).
 * @returns {Promise<{guilds: {id: string, name: string}[]}>}
 */
export async function listGuilds() {
  const res = await apiFetch('/api/guilds');
  return res.json();
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
 * Get a map by ID. The backend scopes map access per campaign, so the
 * campaign UUID is required as a query parameter.
 * @param {string} id - Map UUID.
 * @param {string} campaignId - Campaign UUID.
 * @returns {Promise<object>} The map.
 */
export async function getMap(id, campaignId) {
  const res = await apiFetch(`${API_BASE}/${id}?campaign_id=${campaignId}`);
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
 * Update a map. The backend scopes map access per campaign, so the campaign
 * UUID is required as a query parameter.
 * @param {string} id - Map UUID.
 * @param {string} campaignId - Campaign UUID.
 * @param {object} params - { name, width, height, tiled_json }
 * @returns {Promise<object>} The updated map.
 */
export async function updateMap(id, campaignId, params) {
  const res = await apiFetch(`${API_BASE}/${id}?campaign_id=${campaignId}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  });
  return res.json();
}

/**
 * Import a multi-file Tiled project (.tmj + every tileset / image-layer image
 * the map references) as a new map. POSTs multipart/form-data to
 * /api/maps/import with `campaign_id`, `name`, the `tmj` file, and one
 * `images` entry per image file (matched server-side by filename basename).
 *
 * The browser sets the multipart boundary, so we deliberately omit the
 * Content-Type header. The backend validates the .tmj, uploads the images,
 * rewrites tileset/image-layer paths to asset URLs, strips unsupported
 * features, and returns { map: {...}, skipped: [...] }. When images are
 * missing it returns 400 with a plain-text body listing them, which
 * apiFetch surfaces as the thrown Error's message.
 *
 * @param {object} params
 * @param {string} params.campaignId - Campaign UUID.
 * @param {string} params.name - Map name (required by the backend).
 * @param {File}   params.tmjFile - The .tmj file selected by the user.
 * @param {File[]} [params.imageFiles] - Tileset + image-layer image files.
 * @returns {Promise<{map: object, skipped: object[]}>}
 */
export async function importTiledMap({ campaignId, name, tmjFile, imageFiles = [] }) {
  const formData = new FormData();
  formData.append('campaign_id', campaignId);
  formData.append('name', name);
  formData.append('tmj', tmjFile);
  for (const img of imageFiles) {
    formData.append('images', img);
  }
  const res = await apiFetch(`${API_BASE}/import`, {
    method: 'POST',
    body: formData,
  });
  return res.json();
}

/**
 * Reimport a multi-file Tiled project into an EXISTING map, overwriting its
 * content in place (same map ID, so encounters that reference it stay valid).
 * Same multipart shape as importTiledMap but PUTs to /api/maps/{id}/import. The
 * backend preserves the map's background image, and its name when `name` is
 * empty. Returns { map: {...}, skipped: [...] }.
 *
 * @param {object} params
 * @param {string} params.mapId - Map UUID to overwrite.
 * @param {string} params.campaignId - Campaign UUID.
 * @param {string} params.name - Map name (kept if empty server-side).
 * @param {File}   params.tmjFile - The .tmj file selected by the user.
 * @param {File[]} [params.imageFiles] - Tileset + image-layer image files.
 * @returns {Promise<{map: object, skipped: object[]}>}
 */
export async function reimportTiledMap({ mapId, campaignId, name, tmjFile, imageFiles = [] }) {
  const formData = new FormData();
  formData.append('campaign_id', campaignId);
  formData.append('name', name);
  formData.append('tmj', tmjFile);
  for (const img of imageFiles) {
    formData.append('images', img);
  }
  const res = await apiFetch(`${API_BASE}/${mapId}/import`, {
    method: 'PUT',
    body: formData,
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
 * @param {string} campaignId - Campaign UUID (required by the backend).
 * @returns {Promise<object>} The encounter.
 */
export async function getEncounter(id, campaignId) {
  const res = await apiFetch(`${ENCOUNTERS_BASE}/${id}?campaign_id=${encodeURIComponent(campaignId)}`);
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
 * @param {string} campaignId - Campaign UUID (required by the backend).
 * @returns {Promise<object>} The updated encounter.
 */
export async function updateEncounter(id, params, campaignId) {
  const res = await apiFetch(`${ENCOUNTERS_BASE}/${id}?campaign_id=${encodeURIComponent(campaignId)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  });
  return res.json();
}

/**
 * Delete an encounter template.
 * @param {string} id - Encounter UUID.
 * @param {string} campaignId - Campaign UUID (required by the backend).
 * @returns {Promise<void>}
 */
export async function deleteEncounter(id, campaignId) {
  await apiFetch(`${ENCOUNTERS_BASE}/${id}?campaign_id=${encodeURIComponent(campaignId)}`, {
    method: 'DELETE',
  });
}

/**
 * Duplicate an encounter template.
 * @param {string} id - Encounter UUID.
 * @param {string} campaignId - Campaign UUID (required by the backend).
 * @returns {Promise<object>} The duplicated encounter.
 */
export async function duplicateEncounter(id, campaignId) {
  const res = await apiFetch(`${ENCOUNTERS_BASE}/${id}/duplicate?campaign_id=${encodeURIComponent(campaignId)}`, {
    method: 'POST',
  });
  return res.json();
}

/**
 * List creatures (stat blocks) visible to a campaign: the SRD library plus
 * that campaign's homebrew. Without a campaignId only SRD creatures return.
 * @param {string} [campaignId] - Campaign to scope homebrew to.
 * @returns {Promise<object[]>} Array of creatures.
 */
export async function listCreatures(campaignId) {
  const path = campaignId
    ? `/api/creatures?campaign_id=${encodeURIComponent(campaignId)}`
    : '/api/creatures';
  const res = await apiFetch(path);
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
 * Build the URL of the server-rendered combat map PNG for an encounter.
 * Defaults to the unfogged DM view; pass `{ playerView: true }` to get the
 * fogged, player-perspective render (`?view=player`) so the DM can preview
 * exactly what players see through fog of war and lighting. Backed by
 * handleDMMapPNG in cmd/dndnd/dashboard_apis.go.
 *
 * @param {string} encounterId - Encounter UUID.
 * @param {object} [opts]
 * @param {boolean} [opts.playerView=false] - Render the fogged player view.
 * @returns {string} The map PNG URL.
 */
export function combatMapUrl(encounterId, { playerView = false } = {}) {
  const base = `${COMBAT_BASE}/${encounterId}/map.png`;
  if (!playerView) return base;
  return `${base}?view=player`;
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

/**
 * List encounters in this campaign that are eligible to own a loot pool.
 * Backend currently returns encounters with status == "completed", sorted by
 * most recent.
 *
 * @param {string} campaignId - Campaign UUID.
 * @returns {Promise<object>} { encounters: [{ id, name, display_name, status }] }
 */
export async function listEligibleLootEncounters(campaignId) {
  const res = await apiFetch(`/api/campaigns/${campaignId}/loot/eligible-encounters`);
  return res.json();
}

// --- Loot Pool API (F-13) ---
//
// Mirrors mountLootRoutes in cmd/dndnd/dashboard_apis.go. All routes are nested
// under /api/campaigns/:campaignID/encounters/:encounterID/loot. The backend
// returns a LootPoolResult of the form { Pool: {...}, Items: [...] } (Go's
// default JSON encoding capitalises field names since the struct has no tags).
//
function lootBase(campaignId, encounterId) {
  return `/api/campaigns/${campaignId}/encounters/${encounterId}/loot`;
}

/**
 * Get the loot pool for an encounter. Returns 404 when no pool exists yet.
 * @param {string} campaignId - Campaign UUID.
 * @param {string} encounterId - Encounter UUID.
 * @returns {Promise<object>} { Pool, Items }
 */
export async function getLootPool(campaignId, encounterId) {
  const res = await apiFetch(lootBase(campaignId, encounterId));
  return res.json();
}

/**
 * Create the loot pool for a completed encounter. Auto-populates from defeated
 * NPC inventories. Errors with 400 if encounter is not completed, 409 if a
 * pool already exists.
 * @param {string} campaignId - Campaign UUID.
 * @param {string} encounterId - Encounter UUID.
 * @returns {Promise<object>} { Pool, Items }
 */
export async function createLootPool(campaignId, encounterId) {
  const res = await apiFetch(lootBase(campaignId, encounterId), {
    method: 'POST',
  });
  return res.json();
}

/**
 * Add a custom item to the loot pool.
 * @param {string} campaignId - Campaign UUID.
 * @param {string} encounterId - Encounter UUID.
 * @param {object} item - { item_id?, name, description?, quantity?, type?,
 *   is_magic?, magic_bonus?, magic_properties?, requires_attunement?, rarity? }
 * @returns {Promise<object>} The created loot pool item.
 */
export async function addLootPoolItem(campaignId, encounterId, item) {
  const res = await apiFetch(`${lootBase(campaignId, encounterId)}/items`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(item),
  });
  return res.json();
}

/**
 * Remove an item from the loot pool.
 * @param {string} campaignId - Campaign UUID.
 * @param {string} encounterId - Encounter UUID.
 * @param {string} itemId - Loot pool item UUID.
 * @returns {Promise<object>} { status: "ok" }
 */
export async function removeLootPoolItem(campaignId, encounterId, itemId) {
  const res = await apiFetch(`${lootBase(campaignId, encounterId)}/items/${itemId}`, {
    method: 'DELETE',
  });
  return res.json();
}

/**
 * Set the total gold pool value.
 * @param {string} campaignId - Campaign UUID.
 * @param {string} encounterId - Encounter UUID.
 * @param {number} gold - New gold total.
 * @returns {Promise<object>} The updated loot pool.
 */
export async function setLootGold(campaignId, encounterId, gold) {
  const res = await apiFetch(`${lootBase(campaignId, encounterId)}/gold`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ gold }),
  });
  return res.json();
}

/**
 * Post the loot announcement to the campaign's #combat-log / #the-story channel.
 * @param {string} campaignId - Campaign UUID.
 * @param {string} encounterId - Encounter UUID.
 * @returns {Promise<object>} { status, message }
 */
export async function postLootAnnouncement(campaignId, encounterId) {
  const res = await apiFetch(`${lootBase(campaignId, encounterId)}/post`, {
    method: 'POST',
  });
  return res.json();
}

// --- DM Inventory API ---
//
// DM-only endpoints (guarded server-side by dmAuthMw) for adjusting a player
// character's inventory and gold from the Party page. The mutating calls return
// only { status: "ok" }; reload via getCharacterInventory to refresh the view.

/**
 * Fetch a character's current inventory and gold.
 * @param {string} characterId - Character UUID.
 * @returns {Promise<{character_id: string, name: string, gold: number, items: object[]}>}
 */
export async function getCharacterInventory(characterId) {
  const res = await apiFetch(`/api/inventory?character_id=${encodeURIComponent(characterId)}`);
  return res.json();
}

/**
 * Add an item to a character's inventory (stacks if the item id already exists).
 * @param {string} characterId - Character UUID.
 * @param {object} item - InventoryItem payload ({ item_id, name, quantity, type, ... }).
 * @returns {Promise<object>} { status: "ok" }
 */
export async function addInventoryItem(characterId, item) {
  const res = await apiFetch('/api/inventory/add', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ character_id: characterId, item }),
  });
  return res.json();
}

/**
 * Remove a quantity of an item from a character's inventory.
 * @param {string} characterId - Character UUID.
 * @param {string} itemId - Item id to decrement.
 * @param {number} quantity - How many to remove.
 * @returns {Promise<object>} { status: "ok" }
 */
export async function removeInventoryItem(characterId, itemId, quantity) {
  const res = await apiFetch('/api/inventory/remove', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ character_id: characterId, item_id: itemId, quantity }),
  });
  return res.json();
}

/**
 * Set a character's gold to an absolute value (not additive).
 * @param {string} characterId - Character UUID.
 * @param {number} gold - New gold total.
 * @returns {Promise<object>} { status: "ok" }
 */
export async function setCharacterGold(characterId, gold) {
  const res = await apiFetch('/api/inventory/gold', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ character_id: characterId, gold }),
  });
  return res.json();
}

/**
 * Reveal or hide a magic item's identification for a character.
 * @param {string} characterId - Character UUID.
 * @param {string} itemId - Item id to toggle.
 * @param {boolean} identified - true reveals, false hides.
 * @returns {Promise<object>} { status: "ok" }
 */
export async function setInventoryItemIdentified(characterId, itemId, identified) {
  const res = await apiFetch('/api/inventory/identify', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ character_id: characterId, item_id: itemId, identified }),
  });
  return res.json();
}

/**
 * Transfer a quantity of an item from one character to another.
 * @param {string} fromCharacterId - Source character UUID.
 * @param {string} toCharacterId - Target character UUID.
 * @param {string} itemId - Item id to move.
 * @param {number} quantity - How many to transfer.
 * @returns {Promise<object>} { status: "ok" }
 */
export async function transferInventoryItem(fromCharacterId, toCharacterId, itemId, quantity) {
  const res = await apiFetch('/api/inventory/transfer', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      from_character_id: fromCharacterId,
      to_character_id: toCharacterId,
      item_id: itemId,
      quantity,
    }),
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
 * End the active combat for an encounter (POST /api/combat/{id}/end).
 *
 * Sets the encounter to completed, breaks lingering concentration, pauses
 * timers, and posts the encounter end to Discord. Rejects if the encounter
 * is not active (HTTP 409).
 *
 * @param {string} encounterId - Encounter UUID.
 * @returns {Promise<object>} End-of-combat summary (encounter, casualties, rounds…).
 */
export async function endCombat(encounterId) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/end`, {
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

// --- Pending monster AoE saving throws ---
//
// Monster (NPC) AoE saves that the DM must roll. Surfaced both in the Combat
// workspace and the DM Console. The list endpoint returns only unresolved
// monster saves and works regardless of whose turn it is; resolve rolls the
// save server-side and applies any damage. Backed by
// internal/combat/pending_save_handler.go.

/**
 * List unresolved monster AoE saving throws for an encounter.
 * @param {string} encounterId - Encounter UUID.
 * @returns {Promise<Array<{id:string, combatant_id:string, combatant_name:string,
 *   ability:string, dc:number, source:string, spell_id:string, cover_bonus:number}>>}
 */
export async function getPendingSaves(encounterId) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/pending-saves`);
  return res.json();
}

/**
 * Resolve a single monster pending save by its IDs. Sends an empty JSON body.
 * apiFetch throws Error(serverText) on non-2xx so callers can surface the
 * backend's plain-text message (404 not found, 409 already-resolved /
 * player-save, 400 bad ids).
 * @param {string} encounterId - Encounter UUID.
 * @param {string} saveId - Pending save UUID.
 * @returns {Promise<object>} The rolled result (save_id, natural_roll, total,
 *   success, optional damage…).
 */
export async function resolveMonsterSave(encounterId, saveId) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/pending-saves/${saveId}/resolve`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({}),
  });
  return res.json();
}

/**
 * Resolve a monster pending save via a server-supplied resolve URL (the DM
 * situation `pending[]` items carry the POST path as `resolve_url`). Same body
 * and error semantics as resolveMonsterSave.
 * @param {string} resolveUrl - The POST resolve path from the situation item.
 * @returns {Promise<object>} The rolled result.
 */
export async function resolveMonsterSaveByUrl(resolveUrl) {
  const res = await apiFetch(resolveUrl, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({}),
  });
  return res.json();
}

/**
 * Cancel (void) the AoE cast a pending save belongs to. Forfeits every
 * same-source pending save so the cast lands NO damage — used when a DM grants
 * a player's undo of a misplaced AoE spell. apiFetch throws Error(serverText)
 * on non-2xx (404 not found, 409 already-applied, 400 bad ids).
 * @param {string} encounterId - Encounter UUID.
 * @param {string} saveId - Any pending save UUID in the cast.
 * @returns {Promise<{save_id:string, spell_id:string, canceled:number}>}
 */
export async function cancelMonsterSave(encounterId, saveId) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/pending-saves/${saveId}/cancel`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({}),
  });
  return res.json();
}

/**
 * Restore (give back) the action the active combatant spent this turn — the
 * missing step when a DM grants a player's undo of a misplayed action (e.g. an
 * AoE cast whose pending saves were voided). Clears action_used / the
 * leveled-spell flag and reseeds the turn's attack(s); leaves movement alone.
 * apiFetch throws Error(serverText) on non-2xx (409 not-active-turn /
 * already-available, 400 bad ids).
 * @param {string} encounterId - Encounter UUID.
 * @param {string} combatantId - The active combatant UUID.
 * @returns {Promise<{combatant_id:string, combatant_name:string, restored:boolean}>}
 */
export async function restoreCombatantAction(encounterId, combatantId) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/combatants/${combatantId}/restore-action`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({}),
  });
  return res.json();
}

/**
 * Restore (give back) the bonus action the active combatant spent this turn —
 * the sibling of {@link restoreCombatantAction} for the bonus-action economy.
 * Clears bonus_action_used and reseeds the turn's bonus action; leaves the main
 * action and movement alone.
 * apiFetch throws Error(serverText) on non-2xx (409 not-active-turn /
 * already-available, 400 bad ids).
 * @param {string} encounterId - Encounter UUID.
 * @param {string} combatantId - The active combatant UUID.
 * @returns {Promise<{combatant_id:string, combatant_name:string, restored:boolean}>}
 */
export async function restoreCombatantBonusAction(encounterId, combatantId) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/combatants/${combatantId}/restore-bonus-action`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({}),
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

/**
 * Stamp a source-tagged concentration marker (Hex / Hunter's Mark) on a
 * combatant. Unlike the plain conditions PATCH, this carries the source spell +
 * caster id the on-hit rider matches on, so a DM-placed marker actually fires.
 * @param {string} encounterId - Encounter UUID.
 * @param {string} combatantId - The marked (target) combatant UUID.
 * @param {string} spell - "hex" or "hunters-mark".
 * @param {string} sourceCombatantId - The caster combatant UUID.
 * @returns {Promise<object>} The updated combatant.
 */
export async function applySpellMarker(encounterId, combatantId, spell, sourceCombatantId) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/combatants/${combatantId}/spell-marker`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ spell, source_combatant_id: sourceCombatantId }),
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

/**
 * Returns true if the reaction is a counterspell declaration.
 */
export function isCounterspellReaction(reaction) {
  return /counterspell/i.test(reaction.description || '');
}

/**
 * Trigger counterspell for a reaction declaration.
 * @param {string} encounterId - Encounter UUID.
 * @param {string} reactionId - Reaction UUID.
 * @returns {Promise<object>} The counterspell prompt response.
 */
export async function triggerCounterspell(encounterId, reactionId) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/reactions/${reactionId}/counterspell/trigger`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enemy_spell_name: 'Unknown', enemy_cast_level: 1, is_subtle: false }),
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
 * Manually override the action economy of a combatant's own active turn
 * (action / bonus action / reaction / movement / attacks). Every resource key
 * is optional — omit one to leave it untouched; build the body with
 * toTurnResourcesPayload(). Returns 409 when the target combatant is not the
 * one currently acting, since a completed turn's flags are meaningless.
 * @param {string} encounterId
 * @param {string} combatantId
 * @param {{action_used?:boolean, bonus_action_used?:boolean, reaction_used?:boolean,
 *          movement_remaining_ft?:number, attacks_remaining?:number, reason:string}} payload
 */
export async function overrideCombatantTurnResources(encounterId, combatantId, payload) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/override/combatant/${combatantId}/turn-resources`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return res.json();
}

/**
 * Read a character's current spell + pact-magic slots (DM prefill). Allowed
 * both in and out of combat.
 *   GET /api/character-overview/{characterID}/slots
 * @param {string} characterId
 * @returns {Promise<{spell_slots:object, pact_magic_slots:object|null}>}
 */
export async function getCharacterSlots(characterId) {
  const res = await apiFetch(`/api/character-overview/${characterId}/slots`);
  return res.json();
}

/**
 * Save a character's spell / pact-magic slots out of combat.
 *   POST /api/character-overview/{characterID}/slots
 * Omit a slot field to leave that store untouched. apiFetch throws
 * Error(serverText) on any non-2xx so callers can surface the backend's
 * explanation verbatim (e.g. the 409 "use the in-combat controls" message).
 * @param {string} characterId
 * @param {{spell_slots?:object, pact_magic_slots?:object, reason?:string}} payload
 * @returns {Promise<{spell_slots:object, pact_magic_slots:object|null}>}
 */
export async function saveCharacterSlots(characterId, payload) {
  const res = await apiFetch(`/api/character-overview/${characterId}/slots`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return res.json();
}

/**
 * Manually override a character's spell / pact-magic slots in combat (requires
 * an active turn). Same body shape as saveCharacterSlots; apiFetch throws the
 * server's response text on error.
 *   POST /api/combat/{encounterID}/override/character/{characterID}/slots
 * @param {string} encounterId
 * @param {string} characterId
 * @param {{spell_slots?:object, pact_magic_slots?:object, reason?:string}} payload
 */
export async function overrideCharacterSlots(encounterId, characterId, payload) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/override/character/${characterId}/slots`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return res.json();
}

/**
 * Read a character's limited-use feature resources (DM prefill), e.g. Barbarian
 * rage uses. Allowed both in and out of combat; the returned map may be empty.
 *   GET /api/character-overview/{characterID}/feature-uses
 * @param {string} characterId
 * @returns {Promise<{feature_uses:Object<string,{current:number,max:number,recharge:string}>}>}
 */
export async function getCharacterFeatureUses(characterId) {
  const res = await apiFetch(`/api/character-overview/${characterId}/feature-uses`);
  return res.json();
}

/**
 * Save a character's limited-use feature pools out of combat (e.g. rage / ki).
 *   POST /api/character-overview/{characterID}/feature-uses
 * Body is the FeatureUsesEditor shape: a batch of per-feature Current changes
 * plus an optional reason. apiFetch throws Error(serverText) on any non-2xx so
 * callers can surface the backend's explanation verbatim (e.g. the 409
 * "use the in-combat controls" message during active combat).
 * @param {string} characterId
 * @param {{changes:{feature:string,current:number}[], reason?:string}} payload
 * @returns {Promise<{feature_uses:Object<string,{current:number,max:number,recharge:string}>}>}
 */
export async function saveCharacterFeatureUses(characterId, payload) {
  const res = await apiFetch(`/api/character-overview/${characterId}/feature-uses`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return res.json();
}

/**
 * Permanently delete a character from the campaign (DM-only).
 *   DELETE /api/character-overview/{characterID}
 * Resolves on 204. apiFetch throws Error(serverText) on any non-2xx so callers
 * can surface the backend's explanation verbatim (e.g. the 409 "end combat
 * before deleting" message during active combat).
 * @param {string} characterId
 */
export async function deleteCharacter(characterId) {
  await apiFetch(`/api/character-overview/${characterId}`, { method: 'DELETE' });
}

/**
 * Manually override a single character feature's remaining uses in combat
 * (requires an active turn). One feature per request; apiFetch throws the
 * server's response text on error (e.g. 400 unknown feature / current>max,
 * 404 no active turn).
 *   POST /api/combat/{encounterID}/override/character/{characterID}/feature-uses
 * @param {string} encounterId
 * @param {string} characterId
 * @param {{feature:string, current:number, reason?:string}} payload
 */
export async function overrideCharacterFeatureUses(encounterId, characterId, payload) {
  const res = await apiFetch(`${COMBAT_BASE}/${encounterId}/override/character/${characterId}/feature-uses`, {
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

// --- Level-up API ---
// Mirrors LevelUpRequest / LevelUpResponse in internal/levelup/handler.go.

/**
 * Apply a level-up for a character.
 *
 * @param {object} req
 * @param {string} req.character_id - Character UUID.
 * @param {string} req.class_id - Class identifier (e.g. "fighter").
 * @param {number} req.new_level - New class level (>= 1).
 * @returns {Promise<{
 *   new_level: number,
 *   hp_gained: number,
 *   new_hp_max: number,
 *   new_proficiency_bonus: number,
 *   new_attacks_per_action: number,
 *   grants_asi: boolean,
 *   needs_subclass: boolean,
 * }>}
 */
export async function applyLevelUp(req) {
  const res = await apiFetch('/api/levelup', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });
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

/**
 * Add (or update) an admin-managed custom Open5e source. Extends the global
 * catalog so the slug becomes selectable in every campaign's source toggle.
 * DM-only server-side. The slug must be a canonical Open5e document slug
 * (lowercase, hyphenated) and must not collide with a built-in entry.
 *
 * @param {{slug: string, title: string, publisher?: string, description?: string}} source
 * @returns {Promise<{slug: string, title: string, publisher?: string, description?: string, builtin: boolean}>}
 */
export async function addOpen5eSource(source) {
  const res = await apiFetch('/api/open5e/sources', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(source),
  });
  return res.json();
}

/**
 * Delete an admin-added custom Open5e source from the global catalog. Built-in
 * sources are protected server-side (409). DM-only. Resolves on success (204).
 *
 * @param {string} slug
 * @returns {Promise<void>}
 */
export async function deleteOpen5eSource(slug) {
  await apiFetch(`/api/open5e/sources/${encodeURIComponent(slug)}`, {
    method: 'DELETE',
  });
}
