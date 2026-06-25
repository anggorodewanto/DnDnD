import { describe, it, expect, vi } from 'vitest';
import { DM_SITUATION_ENDPOINT, fetchDMSituation } from './dmsituation.js';

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
