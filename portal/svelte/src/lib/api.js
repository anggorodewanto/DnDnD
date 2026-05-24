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

/** List campaign-enabled ability score generation methods. */
export async function listAbilityMethods(campaignId) {
  let url = `${API_BASE}/ability-methods`;
  if (campaignId) {
    url += `?campaign_id=${encodeURIComponent(campaignId)}`;
  }
  const res = await apiFetch(url);
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

/**
 * Build a mode-aware API client for the shared character builder.
 *
 * The portal and the DM dashboard expose the same conceptual endpoints under
 * different URL prefixes and with slightly different request shapes. The
 * factory hides those differences behind a uniform method set so the shared
 * CharacterBuilder.svelte does not have to branch on mode at the call site.
 *
 * @param {'player'|'dm'} mode
 * @param {{ campaignId?: string, token?: string }} [opts]
 * @returns {{
 *   listRaces: () => Promise<object[]>,
 *   listClasses: () => Promise<object[]>,
 *   listSpells: (className: string) => Promise<object[]>,
 *   listEquipment: () => Promise<object[]>,
 *   getStartingEquipment: (className: string) => Promise<object[]>,
 *   listAbilityMethods: () => Promise<string[]>,
 *   previewCharacter: (submission: object) => Promise<object>,
 *   submitCharacter: (submission: object) => Promise<object>,
 * }}
 */
export function makeBuilderApi(mode, { campaignId = '', token = '' } = {}) {
  if (mode === 'dm') {
    const REF = '/dashboard/api/characters/ref';
    const CHARS = '/dashboard/api/characters';
    return {
      async listRaces() {
        return (await apiFetch(`${REF}/races`)).json();
      },
      async listClasses() {
        return (await apiFetch(`${REF}/classes`)).json();
      },
      async listSpells(className) {
        const params = new URLSearchParams({ class: className || '' });
        if (campaignId) params.set('campaign_id', campaignId);
        return (await apiFetch(`${REF}/spells?${params}`)).json();
      },
      async listEquipment() {
        const qs = campaignId ? `?campaign_id=${encodeURIComponent(campaignId)}` : '';
        return (await apiFetch(`${REF}/equipment${qs}`)).json();
      },
      async getStartingEquipment(className) {
        return (await apiFetch(`${REF}/starting-equipment?class=${encodeURIComponent(className)}`)).json();
      },
      async listAbilityMethods() {
        const qs = campaignId ? `?campaign_id=${encodeURIComponent(campaignId)}` : '';
        return (await apiFetch(`${CHARS}/ability-methods${qs}`)).json();
      },
      async previewCharacter(submission) {
        const res = await apiFetch(`${CHARS}/preview`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(submission),
        });
        return res.json();
      },
      async submitCharacter(submission) {
        const res = await apiFetch(`${CHARS}/`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ campaign_id: campaignId, ...submission }),
        });
        return res.json();
      },
    };
  }

  // Default: player (portal) mode.
  return {
    async listRaces() {
      return (await apiFetch(`${API_BASE}/races`)).json();
    },
    async listClasses() {
      return (await apiFetch(`${API_BASE}/classes`)).json();
    },
    async listSpells(className) {
      let url = `${API_BASE}/spells?class=${encodeURIComponent(className || '')}`;
      if (campaignId) url += `&campaign_id=${encodeURIComponent(campaignId)}`;
      return (await apiFetch(url)).json();
    },
    async listEquipment() {
      return (await apiFetch(`${API_BASE}/equipment`)).json();
    },
    async getStartingEquipment(className) {
      return (await apiFetch(`${API_BASE}/starting-equipment?class=${encodeURIComponent(className)}`)).json();
    },
    async listAbilityMethods() {
      let url = `${API_BASE}/ability-methods`;
      if (campaignId) url += `?campaign_id=${encodeURIComponent(campaignId)}`;
      return (await apiFetch(url)).json();
    },
    async previewCharacter(submission) {
      const res = await apiFetch(`${API_BASE}/characters/preview`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(submission),
      });
      return res.json();
    },
    async submitCharacter(submission) {
      const res = await apiFetch(`${API_BASE}/characters`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token, campaign_id: campaignId, ...submission }),
      });
      return res.json();
    },
  };
}
