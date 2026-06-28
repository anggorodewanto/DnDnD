import { describe, it, expect, vi } from 'vitest';
import {
  DM_SITUATION_ENDPOINT,
  fetchDMSituation,
  formatCondition,
  formatConditions,
  formatDeathSaves,
} from './dmsituation.js';

describe('DM_SITUATION_ENDPOINT', () => {
  it('points at the DM situation route', () => {
    expect(DM_SITUATION_ENDPOINT).toBe('/api/dm/situation');
  });
});

describe('fetchDMSituation', () => {
  it('GETs the endpoint with same-origin credentials and returns the JSON body', async () => {
    const body = {
      pending: [
        {
          id: 'abc',
          source: 'queue',
          kind: 'player_whisper',
          label: 'Player Whisper',
          player: 'Aria',
          summary: 'x',
          resolve_url: '#dm-queue',
          priority: 0,
          created_at: '2026-06-25T12:00:00Z',
        },
      ],
      state: { has_encounter: false, combatants: [] },
      timeline: [],
      next_step: 'Resolve Aria’s whisper.',
    };
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => body,
    });
    const got = await fetchDMSituation(fetchImpl);
    expect(fetchImpl).toHaveBeenCalledWith('/api/dm/situation', { credentials: 'same-origin' });
    expect(got).toEqual(body);
  });

  it('throws with the response text on non-OK', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 403,
      text: async () => 'forbidden: DM only',
    });
    await expect(fetchDMSituation(fetchImpl)).rejects.toThrow(/forbidden: DM only/);
  });

  it('falls back to a generic error when the body is empty', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: async () => '',
    });
    await expect(fetchDMSituation(fetchImpl)).rejects.toThrow(/Request failed: 500/);
  });
});

describe('formatCondition', () => {
  it('renders an object condition with duration + source spell, never [object Object]', () => {
    expect(formatCondition({ name: 'poisoned', duration_rounds: 3, source_spell: 'ray-of-sickness' }))
      .toBe('poisoned (3r, ray-of-sickness)');
    expect(formatCondition({ name: 'prone' })).toBe('prone');
    expect(formatCondition({ name: 'invisible', source_spell: 'greater-invisibility' }))
      .toBe('invisible (greater-invisibility)');
  });

  it('tolerates a bare string for back-compat', () => {
    expect(formatCondition('stunned')).toBe('stunned');
  });

  it('returns empty for nullish', () => {
    expect(formatCondition(null)).toBe('');
    expect(formatCondition(undefined)).toBe('');
  });
});

describe('formatConditions', () => {
  it('joins object conditions and never yields [object Object]', () => {
    const out = formatConditions([{ name: 'prone' }, { name: 'poisoned', duration_rounds: 2 }]);
    expect(out).toBe('prone, poisoned (2r)');
    expect(out).not.toContain('[object Object]');
  });

  it('returns empty for an empty/non-array input', () => {
    expect(formatConditions([])).toBe('');
    expect(formatConditions(undefined)).toBe('');
  });
});

describe('formatDeathSaves', () => {
  it('formats successes/failures', () => {
    expect(formatDeathSaves({ successes: 1, failures: 2 })).toBe('✓1 ✗2');
    expect(formatDeathSaves({})).toBe('✓0 ✗0');
  });

  it('returns empty when not dying', () => {
    expect(formatDeathSaves(null)).toBe('');
  });
});
