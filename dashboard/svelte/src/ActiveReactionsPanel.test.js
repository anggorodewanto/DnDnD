import { describe, it, expect, vi, beforeEach } from 'vitest';
import { triggerCounterspell, isCounterspellReaction } from './lib/api.js';

describe('isCounterspellReaction', () => {
  it('returns true when description contains "counterspell" (case-insensitive)', () => {
    expect(isCounterspellReaction({ description: 'Counterspell reaction' })).toBe(true);
    expect(isCounterspellReaction({ description: 'ready COUNTERSPELL' })).toBe(true);
    expect(isCounterspellReaction({ description: 'will counterspell if enemy casts' })).toBe(true);
  });

  it('returns false when description does not contain "counterspell"', () => {
    expect(isCounterspellReaction({ description: 'Shield reaction' })).toBe(false);
    expect(isCounterspellReaction({ description: 'Opportunity attack' })).toBe(false);
  });
});

describe('triggerCounterspell', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('POSTs to the correct endpoint with default body', async () => {
    const mockResponse = { declaration_id: 'decl-1', caster_name: 'Wizard' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });

    const result = await triggerCounterspell('enc-123', 'react-456');

    expect(result).toEqual(mockResponse);
    expect(fetch).toHaveBeenCalledTimes(1);

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-123/reactions/react-456/counterspell/trigger');
    expect(options.method).toBe('POST');
    expect(options.headers['Content-Type']).toBe('application/json');

    const body = JSON.parse(options.body);
    expect(body.enemy_spell_name).toBe('Unknown');
    expect(body.enemy_cast_level).toBe(1);
    expect(body.is_subtle).toBe(false);
  });
});
