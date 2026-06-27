import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  CHARACTER_OVERVIEW_ENDPOINT,
  STATUS_CONDITIONS,
  MAX_EXHAUSTION,
  statusEndpoint,
  toStatusPayload,
  saveCharacterStatus,
} from './characterStatus.js';

describe('STATUS_CONDITIONS', () => {
  it('lists exactly the 14 toggleable conditions and excludes exhaustion', () => {
    expect(STATUS_CONDITIONS).toEqual([
      'blinded',
      'charmed',
      'deafened',
      'frightened',
      'grappled',
      'incapacitated',
      'invisible',
      'paralyzed',
      'petrified',
      'poisoned',
      'prone',
      'restrained',
      'stunned',
      'unconscious',
    ]);
    expect(STATUS_CONDITIONS).toHaveLength(14);
    expect(STATUS_CONDITIONS).not.toContain('exhaustion');
  });
});

describe('statusEndpoint', () => {
  it('targets POST /api/character-overview/{id}/status', () => {
    expect(statusEndpoint('abc')).toBe('/api/character-overview/abc/status');
    expect(CHARACTER_OVERVIEW_ENDPOINT).toBe('/api/character-overview');
  });

  it('url-encodes the character id', () => {
    expect(statusEndpoint('a/b')).toBe('/api/character-overview/a%2Fb/status');
  });
});

describe('toStatusPayload', () => {
  it('coerces numeric fields to ints and keeps known conditions', () => {
    const payload = toStatusPayload({
      hpMax: '40',
      hpCurrent: '12',
      tempHp: '5',
      exhaustionLevel: '2',
      conditions: ['poisoned', 'prone'],
      reason: 'trap damage',
    });
    expect(payload).toEqual({
      hp_max: 40,
      hp_current: 12,
      temp_hp: 5,
      exhaustion_level: 2,
      conditions: ['poisoned', 'prone'],
      reason: 'trap damage',
    });
  });

  it('clamps exhaustion to 0-6', () => {
    expect(toStatusPayload({ exhaustionLevel: 99 }).exhaustion_level).toBe(MAX_EXHAUSTION);
    expect(toStatusPayload({ exhaustionLevel: -3 }).exhaustion_level).toBe(0);
  });

  it('drops unknown conditions and a blank reason', () => {
    const payload = toStatusPayload({
      conditions: ['poisoned', 'bogus', 'exhaustion'],
      reason: '   ',
    });
    expect(payload.conditions).toEqual(['poisoned']);
    expect(payload).not.toHaveProperty('reason');
  });

  it('defaults missing numbers to 0 and conditions to []', () => {
    expect(toStatusPayload({})).toEqual({
      hp_max: 0,
      hp_current: 0,
      temp_hp: 0,
      exhaustion_level: 0,
      conditions: [],
    });
  });
});

describe('saveCharacterStatus', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('POSTs the contract body to the per-character status endpoint', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ ok: true }),
    });

    const result = await saveCharacterStatus('char-1', {
      hpMax: 30,
      hpCurrent: 18,
      tempHp: 4,
      exhaustionLevel: 1,
      conditions: ['poisoned'],
      reason: 'failed save',
    });

    expect(result).toEqual({ ok: true });
    expect(fetch).toHaveBeenCalledTimes(1);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/character-overview/char-1/status');
    expect(options.method).toBe('POST');
    expect(options.headers['Content-Type']).toBe('application/json');
    const payload = JSON.parse(options.body);
    expect(payload).toEqual({
      hp_max: 30,
      hp_current: 18,
      temp_hp: 4,
      exhaustion_level: 1,
      conditions: ['poisoned'],
      reason: 'failed save',
    });
  });

  it('tolerates a 200 with an empty (non-JSON) body', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.reject(new Error('Unexpected end of JSON input')),
    });
    await expect(saveCharacterStatus('char-1', {})).resolves.toEqual({});
  });

  it('throws the server text on a 409 (active combat)', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      text: () => Promise.resolve('character is in an active combat; use combat controls'),
    });
    await expect(saveCharacterStatus('char-1', {})).rejects.toThrow(/use combat controls/);
  });

  it('throws the server text on a 403 (not the DM)', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 403,
      text: () => Promise.resolve('only the DM may edit status'),
    });
    await expect(saveCharacterStatus('char-1', {})).rejects.toThrow(/only the DM/);
  });

  it('falls back to a status-coded message when the error body is empty', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve(''),
    });
    await expect(saveCharacterStatus('char-1', {})).rejects.toThrow(/400/);
  });
});
