/**
 * Errors panel API helper.
 *
 * Thin wrapper around `GET /api/errors` so the ErrorsPanel component can
 * stay focused on rendering. The endpoint returns the most recent error
 * log entries from the last 24 hours wrapped as `{ errors: [...] }` so
 * future additions (totals, paging) do not break the wire shape.
 *
 * Each entry has the shape:
 *   { timestamp: string (RFC3339), command: string, user_id: string, summary: string }
 */

export const ERRORS_ENDPOINT = '/api/errors';

/**
 * Fetch the recent errors list. Throws on non-OK responses with the response
 * body as the error message (falls back to "Request failed: <status>").
 *
 * @param {typeof fetch} [fetchImpl] - injected fetch for tests.
 * @returns {Promise<Array<{timestamp: string, command: string, user_id: string, summary: string}>>}
 */
export async function fetchRecentErrors(fetchImpl = fetch) {
  const res = await fetchImpl(ERRORS_ENDPOINT, { credentials: 'same-origin' });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  const body = await res.json();
  if (!body || !Array.isArray(body.errors)) return [];
  return body.errors;
}

/**
 * Format an RFC3339 timestamp for display in the Errors table. Falls back to
 * the raw string when the input is unparseable so the DM still sees the data.
 *
 * @param {string} timestamp - RFC3339 timestamp from the server.
 * @returns {string} formatted local-time string, or the raw timestamp on parse error.
 */
export function formatErrorTimestamp(timestamp) {
  if (!timestamp) return '';
  const d = new Date(timestamp);
  if (Number.isNaN(d.getTime())) return timestamp;
  return d.toLocaleString();
}
