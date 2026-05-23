import { describe, it, expect, vi } from 'vitest';
import { ERRORS_ENDPOINT, fetchRecentErrors, formatErrorTimestamp } from './errors.js';

describe('ERRORS_ENDPOINT', () => {
  it('points at the JSON errors endpoint', () => {
    expect(ERRORS_ENDPOINT).toBe('/api/errors');
  });
});

describe('fetchRecentErrors', () => {
  it('GETs the endpoint with same-origin credentials and unwraps the errors array', async () => {
    const body = {
      errors: [
        { timestamp: '2026-04-24T12:00:00Z', command: 'cast', user_id: 'alice', summary: 'boom' },
      ],
    };
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => body,
    });

    const got = await fetchRecentErrors(fetchImpl);

    expect(fetchImpl).toHaveBeenCalledWith('/api/errors', { credentials: 'same-origin' });
    expect(got).toEqual(body.errors);
  });

  it('returns [] when the response body has no errors array', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) });
    expect(await fetchRecentErrors(fetchImpl)).toEqual([]);
  });

  it('returns [] when the response body is null', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({ ok: true, json: async () => null });
    expect(await fetchRecentErrors(fetchImpl)).toEqual([]);
  });

  it('throws with the response body on non-OK', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 403,
      text: async () => 'forbidden: DM only',
    });
    await expect(fetchRecentErrors(fetchImpl)).rejects.toThrow(/forbidden: DM only/);
  });

  it('falls back to a generic error message when the body is empty', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: async () => '',
    });
    await expect(fetchRecentErrors(fetchImpl)).rejects.toThrow(/Request failed: 500/);
  });
});

describe('formatErrorTimestamp', () => {
  it('returns the empty string when the timestamp is missing', () => {
    expect(formatErrorTimestamp('')).toBe('');
    expect(formatErrorTimestamp(undefined)).toBe('');
    expect(formatErrorTimestamp(null)).toBe('');
  });

  it('returns the raw input when the timestamp cannot be parsed', () => {
    expect(formatErrorTimestamp('not-a-date')).toBe('not-a-date');
  });

  it('renders a non-empty localised string for a valid RFC3339 timestamp', () => {
    const out = formatErrorTimestamp('2026-04-24T12:00:00Z');
    expect(typeof out).toBe('string');
    expect(out.length).toBeGreaterThan(0);
    expect(out).not.toBe('2026-04-24T12:00:00Z');
  });
});
