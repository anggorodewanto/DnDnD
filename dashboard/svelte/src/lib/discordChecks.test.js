import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fetchDiscordChecks, failingChecks, hasFailures } from './discordChecks.js';

describe('fetchDiscordChecks', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('GETs /api/dashboard/discord-checks and returns the parsed JSON', async () => {
    const payload = {
      results: [{ name: 'token-identity', ok: true, detail: 'bot DnDnD' }],
      ran_at: '2026-05-23T10:00:00Z',
    };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(payload),
    });

    const result = await fetchDiscordChecks();
    expect(result).toEqual(payload);
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(fetch.mock.calls[0][0]).toBe('/api/dashboard/discord-checks');
  });

  it('throws when the response is not ok', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: () => Promise.resolve('boom'),
    });
    await expect(fetchDiscordChecks()).rejects.toThrow(/boom/);
  });
});

describe('failingChecks', () => {
  it('returns only failing entries', () => {
    const report = {
      results: [
        { name: 'token-identity', ok: true },
        { name: 'guild-membership-g-1', ok: false, detail: 'kicked' },
        { name: 'guild-membership-g-2', ok: false, detail: 'never joined' },
      ],
    };
    expect(failingChecks(report)).toEqual([
      { name: 'guild-membership-g-1', ok: false, detail: 'kicked' },
      { name: 'guild-membership-g-2', ok: false, detail: 'never joined' },
    ]);
  });

  it('returns an empty array when disabled is true even with failing entries', () => {
    const report = {
      disabled: true,
      results: [{ name: 'x', ok: false, detail: 'irrelevant' }],
    };
    expect(failingChecks(report)).toEqual([]);
  });

  it('returns an empty array for null / undefined / malformed reports', () => {
    expect(failingChecks(null)).toEqual([]);
    expect(failingChecks(undefined)).toEqual([]);
    expect(failingChecks({})).toEqual([]);
    expect(failingChecks({ results: 'not-an-array' })).toEqual([]);
  });
});

describe('hasFailures', () => {
  it('returns true when at least one check failed', () => {
    expect(
      hasFailures({
        results: [
          { name: 'a', ok: true },
          { name: 'b', ok: false },
        ],
      }),
    ).toBe(true);
  });

  it('returns false when every check passed', () => {
    expect(
      hasFailures({
        results: [
          { name: 'a', ok: true },
          { name: 'b', ok: true },
        ],
      }),
    ).toBe(false);
  });

  it('returns false when disabled', () => {
    expect(hasFailures({ disabled: true, results: [{ ok: false }] })).toBe(false);
  });

  it('returns false for empty / nullish input', () => {
    expect(hasFailures(null)).toBe(false);
    expect(hasFailures({})).toBe(false);
  });
});
