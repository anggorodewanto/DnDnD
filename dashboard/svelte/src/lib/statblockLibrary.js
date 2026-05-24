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

// --- Response normalization -------------------------------------------------
// The backend returns refdata.Creature verbatim, so Go's database/sql and
// pqtype null-wrapper types leak into the JSON: sql.NullString becomes
// {String, Valid}, sql.NullBool becomes {Bool, Valid}, and
// pqtype.NullRawMessage becomes {RawMessage, Valid}. Plain json.RawMessage
// fields (speed, ability_scores, attacks) arrive already unwrapped. We flatten
// the wrappers here so the UI consumes clean values. All unwrap helpers are
// idempotent, so feeding already-clean data through them is safe.

function unwrapNullString(v) {
  if (v && typeof v === 'object' && 'Valid' in v) return v.Valid ? v.String : null;
  return v ?? null;
}

function unwrapNullBool(v) {
  if (v && typeof v === 'object' && 'Valid' in v) return v.Valid ? v.Bool : false;
  return Boolean(v);
}

function unwrapNullRaw(v) {
  if (v && typeof v === 'object' && !Array.isArray(v) && 'Valid' in v && 'RawMessage' in v) {
    return v.Valid ? v.RawMessage : null;
  }
  return v ?? null;
}

/**
 * Flatten the Go null-wrapper types in a raw stat block into plain values.
 * Returns the input unchanged when it is not an object.
 * @param {object} raw
 * @returns {object}
 */
export function normalizeStatBlock(raw) {
  if (!raw || typeof raw !== 'object') return raw;
  return {
    ...raw,
    alignment: unwrapNullString(raw.alignment),
    ac_type: unwrapNullString(raw.ac_type),
    source: unwrapNullString(raw.source),
    homebrew: unwrapNullBool(raw.homebrew),
    saving_throws: unwrapNullRaw(raw.saving_throws),
    skills: unwrapNullRaw(raw.skills),
    senses: unwrapNullRaw(raw.senses),
    abilities: unwrapNullRaw(raw.abilities),
    bonus_actions: unwrapNullRaw(raw.bonus_actions),
  };
}

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
  const data = await res.json();
  return (data || []).map(normalizeStatBlock);
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
  return normalizeStatBlock(await res.json());
}
