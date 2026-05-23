import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  EXPLORATION_API_ENDPOINT,
  EXPLORATION_START_ENDPOINT,
  EXPLORATION_TRANSITION_ENDPOINT,
  fetchExplorationData,
  startExploration,
  transitionToCombat,
  parseOverridesText,
} from './exploration.js';

describe('exploration endpoint constants', () => {
  it('exposes the GET data endpoint', () => {
    expect(EXPLORATION_API_ENDPOINT).toBe('/api/exploration');
  });
  it('exposes the POST start endpoint', () => {
    expect(EXPLORATION_START_ENDPOINT).toBe('/dashboard/exploration/start');
  });
  it('exposes the POST transition endpoint', () => {
    expect(EXPLORATION_TRANSITION_ENDPOINT).toBe('/dashboard/exploration/transition-to-combat');
  });
});

describe('fetchExplorationData', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('GETs /api/exploration with the campaign_id query param', async () => {
    const data = { campaign_id: 'c1', maps: [{ id: 'm1', name: 'Forest' }] };
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(data),
    });

    const got = await fetchExplorationData('c1', fetchImpl);
    expect(got).toEqual(data);
    const [url, opts] = fetchImpl.mock.calls[0];
    expect(url).toBe('/api/exploration?campaign_id=c1');
    expect(opts.credentials).toBe('same-origin');
  });

  it('throws on missing campaign id', async () => {
    await expect(fetchExplorationData('')).rejects.toThrow(/campaign/i);
  });

  it('throws with response text on non-OK', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: () => Promise.resolve('boom'),
    });
    await expect(fetchExplorationData('c1', fetchImpl)).rejects.toThrow(/boom/);
  });

  it('falls back to a generic error when the body is empty', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 503,
      text: () => Promise.resolve(''),
    });
    await expect(fetchExplorationData('c1', fetchImpl)).rejects.toThrow(/503/);
  });
});

describe('startExploration', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('POSTs JSON with campaign_id, map_id, name, character_ids', async () => {
    const reply = { encounter_id: 'enc-1', mode: 'exploration', pcs: {} };
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(reply),
    });

    const got = await startExploration(
      {
        campaignId: 'c1',
        mapId: 'm1',
        name: 'Exploring Forest',
        characterIds: ['ch1', 'ch2'],
      },
      fetchImpl,
    );

    expect(got).toEqual(reply);
    const [url, opts] = fetchImpl.mock.calls[0];
    expect(url).toBe('/dashboard/exploration/start');
    expect(opts.method).toBe('POST');
    expect(opts.headers['Content-Type']).toBe('application/json');
    const payload = JSON.parse(opts.body);
    expect(payload).toEqual({
      campaign_id: 'c1',
      map_id: 'm1',
      name: 'Exploring Forest',
      character_ids: ['ch1', 'ch2'],
    });
  });

  it('omits character_ids when none were provided', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ encounter_id: 'enc-1' }),
    });
    await startExploration({ campaignId: 'c1', mapId: 'm1' }, fetchImpl);
    const payload = JSON.parse(fetchImpl.mock.calls[0][1].body);
    expect(payload).toEqual({ campaign_id: 'c1', map_id: 'm1' });
  });

  it('rejects missing campaign or map id before calling fetch', async () => {
    const fetchImpl = vi.fn();
    await expect(startExploration({ campaignId: '', mapId: 'm1' }, fetchImpl)).rejects.toThrow(
      /campaign/i,
    );
    await expect(startExploration({ campaignId: 'c1', mapId: '' }, fetchImpl)).rejects.toThrow(
      /map/i,
    );
    expect(fetchImpl).not.toHaveBeenCalled();
  });

  it('throws the response text on non-OK', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve('invalid map_id'),
    });
    await expect(
      startExploration({ campaignId: 'c1', mapId: 'm1' }, fetchImpl),
    ).rejects.toThrow(/invalid map_id/);
  });
});

describe('transitionToCombat', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('POSTs JSON with encounter_id and overrides map', async () => {
    const reply = { mode: 'combat', positions: { 'ch1': { col: 'D', row: 5 } } };
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(reply),
    });

    const got = await transitionToCombat(
      { encounterId: 'enc-1', overrides: { ch1: 'D5' } },
      fetchImpl,
    );

    expect(got).toEqual(reply);
    const [url, opts] = fetchImpl.mock.calls[0];
    expect(url).toBe('/dashboard/exploration/transition-to-combat');
    expect(opts.method).toBe('POST');
    expect(opts.headers['Content-Type']).toBe('application/json');
    const payload = JSON.parse(opts.body);
    expect(payload).toEqual({ encounter_id: 'enc-1', overrides: { ch1: 'D5' } });
  });

  it('omits overrides when not supplied', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ mode: 'combat', positions: {} }),
    });
    await transitionToCombat({ encounterId: 'enc-1' }, fetchImpl);
    const payload = JSON.parse(fetchImpl.mock.calls[0][1].body);
    expect(payload).toEqual({ encounter_id: 'enc-1' });
  });

  it('rejects missing encounter_id before calling fetch', async () => {
    const fetchImpl = vi.fn();
    await expect(transitionToCombat({ encounterId: '' }, fetchImpl)).rejects.toThrow(
      /encounter/i,
    );
    expect(fetchImpl).not.toHaveBeenCalled();
  });

  it('throws the response text on non-OK', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve('encounter not exploration'),
    });
    await expect(
      transitionToCombat({ encounterId: 'enc-1' }, fetchImpl),
    ).rejects.toThrow(/encounter not exploration/);
  });
});

describe('parseOverridesText', () => {
  it('parses one override per line in `<char_id>=<coord>` form', () => {
    const text = 'abc=D5\nxyz=A1';
    expect(parseOverridesText(text)).toEqual({ abc: 'D5', xyz: 'A1' });
  });

  it('trims whitespace and ignores blank lines', () => {
    const text = '\n  abc = D5  \n\n  xyz=A1\n';
    expect(parseOverridesText(text)).toEqual({ abc: 'D5', xyz: 'A1' });
  });

  it('returns an empty object for empty or whitespace input', () => {
    expect(parseOverridesText('')).toEqual({});
    expect(parseOverridesText('   \n  ')).toEqual({});
  });

  it('throws when a line lacks the `=` separator', () => {
    expect(() => parseOverridesText('no-equals-here')).toThrow(/format/i);
  });

  it('throws when a line is missing the char id or coord side', () => {
    expect(() => parseOverridesText('=D5')).toThrow();
    expect(() => parseOverridesText('abc=')).toThrow();
  });
});
