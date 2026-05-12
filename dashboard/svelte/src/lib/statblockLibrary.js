/**
 * Stat Block Library API client (Phase 98).
 *
 * Wraps `/api/statblocks` from `internal/statblocklibrary/handler.go`.
 *
 * The endpoint supports the following optional query params:
 *   - search       — case-insensitive substring on name
 *   - type         — creature type (repeatable or comma-separated)
 *   - size         — creature size (repeatable or comma-separated)
 *   - cr_min, cr_max — float CR range
 *   - source       — '' (any) | 'srd' | 'homebrew'
 *   - campaign_id  — UUID; required to surface homebrew + Open5e rows
 *   - limit, offset — pagination
 */

export const STATBLOCK_LIBRARY_ENDPOINT = '/api/statblocks';

/**
 * Build a URL with query params for the Stat Block Library list endpoint.
 * Pure function so the tests can assert URL shape without a fetch mock.
 * @param {object} filters - { campaignId, search, types, sizes, crMin, crMax, source, limit, offset }
 * @returns {string}
 */
export function buildStatBlockListUrl(filters = {}) {
  const params = new URLSearchParams();
  if (filters.campaignId) params.set('campaign_id', filters.campaignId);
  if (filters.search) params.set('search', filters.search);
  if (filters.types) {
    for (const t of filters.types) {
      if (t) params.append('type', t);
    }
  }
  if (filters.sizes) {
    for (const s of filters.sizes) {
      if (s) params.append('size', s);
    }
  }
  if (typeof filters.crMin === 'number') params.set('cr_min', String(filters.crMin));
  if (typeof filters.crMax === 'number') params.set('cr_max', String(filters.crMax));
  if (filters.source) params.set('source', filters.source);
  if (typeof filters.limit === 'number' && filters.limit > 0) {
    params.set('limit', String(filters.limit));
  }
  if (typeof filters.offset === 'number' && filters.offset > 0) {
    params.set('offset', String(filters.offset));
  }
  const qs = params.toString();
  return qs ? `${STATBLOCK_LIBRARY_ENDPOINT}?${qs}` : STATBLOCK_LIBRARY_ENDPOINT;
}

/**
 * List stat blocks via `/api/statblocks`.
 * @param {object} filters
 * @returns {Promise<object[]>}
 */
export async function listStatBlocks(filters = {}) {
  const url = buildStatBlockListUrl(filters);
  const res = await fetch(url, { credentials: 'same-origin' });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json();
}

/**
 * Get one stat block via `/api/statblocks/{id}`.
 * @param {string} id
 * @param {string} [campaignId]
 * @returns {Promise<object>}
 */
export async function getStatBlock(id, campaignId) {
  const params = new URLSearchParams();
  if (campaignId) params.set('campaign_id', campaignId);
  const qs = params.toString();
  const url = qs
    ? `${STATBLOCK_LIBRARY_ENDPOINT}/${encodeURIComponent(id)}?${qs}`
    : `${STATBLOCK_LIBRARY_ENDPOINT}/${encodeURIComponent(id)}`;
  const res = await fetch(url, { credentials: 'same-origin' });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json();
}
