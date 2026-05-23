import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  CHARCREATE_BASE,
  fetchRaces,
  fetchClasses,
  fetchEquipment,
  fetchStartingEquipment,
  fetchSpells,
  fetchAbilityMethods,
  previewCharacter,
  createCharacter,
  parseSubclasses,
  roll4d6DropLowest,
  standardArrayScores,
  pointBuyStartingScores,
  labelForAbilityMethod,
} from './charcreate.js';

describe('CHARCREATE_BASE', () => {
  it('points at the dashboard JSON API', () => {
    expect(CHARCREATE_BASE).toBe('/dashboard/api/characters');
  });
});

describe('fetchRaces', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('GETs /ref/races and returns parsed JSON', async () => {
    const races = [{ id: 'elf', name: 'Elf', speed_ft: 30 }];
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(races),
    });
    const got = await fetchRaces();
    expect(got).toEqual(races);
    expect(fetch.mock.calls[0][0]).toBe('/dashboard/api/characters/ref/races');
  });

  it('throws on non-OK', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: () => Promise.resolve('boom'),
    });
    await expect(fetchRaces()).rejects.toThrow(/boom/);
  });

  it('falls back to generic error when body is empty', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 503,
      text: () => Promise.resolve(''),
    });
    await expect(fetchRaces()).rejects.toThrow(/503/);
  });
});

describe('fetchClasses', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('GETs /ref/classes', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([{ id: 'fighter' }]),
    });
    await fetchClasses();
    expect(fetch.mock.calls[0][0]).toBe('/dashboard/api/characters/ref/classes');
  });
});

describe('fetchEquipment', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('GETs /ref/equipment with no campaign id when none is provided', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([]),
    });
    await fetchEquipment();
    expect(fetch.mock.calls[0][0]).toBe('/dashboard/api/characters/ref/equipment');
  });

  it('passes campaign id through as a query param', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([]),
    });
    await fetchEquipment('camp-1');
    expect(fetch.mock.calls[0][0]).toBe(
      '/dashboard/api/characters/ref/equipment?campaign_id=camp-1',
    );
  });
});

describe('fetchStartingEquipment', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('rejects when no class is supplied', async () => {
    await expect(fetchStartingEquipment('')).rejects.toThrow(/class is required/);
  });

  it('GETs /ref/starting-equipment with the class query param', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([{ id: 'pack-a' }]),
    });
    await fetchStartingEquipment('Fighter');
    expect(fetch.mock.calls[0][0]).toBe(
      '/dashboard/api/characters/ref/starting-equipment?class=Fighter',
    );
  });
});

describe('fetchSpells', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('rejects when no class is supplied', async () => {
    await expect(fetchSpells('')).rejects.toThrow(/class is required/);
  });

  it('GETs /ref/spells with the class query param', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([]),
    });
    await fetchSpells('Wizard');
    expect(fetch.mock.calls[0][0]).toBe(
      '/dashboard/api/characters/ref/spells?class=Wizard',
    );
  });

  it('includes max_level when provided', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([]),
    });
    await fetchSpells('Wizard', { maxLevel: 2 });
    const url = fetch.mock.calls[0][0];
    expect(url).toContain('class=Wizard');
    expect(url).toContain('max_level=2');
  });

  it('includes campaign_id when provided', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([]),
    });
    await fetchSpells('Wizard', { campaignId: 'camp-1' });
    expect(fetch.mock.calls[0][0]).toContain('campaign_id=camp-1');
  });

  it('omits max_level when set to 0', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([]),
    });
    await fetchSpells('Wizard', { maxLevel: 0 });
    expect(fetch.mock.calls[0][0]).not.toContain('max_level');
  });
});

describe('fetchAbilityMethods', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('GETs /ability-methods without query when no campaign id', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(['point_buy', 'standard_array', 'roll']),
    });
    const got = await fetchAbilityMethods();
    expect(fetch.mock.calls[0][0]).toBe('/dashboard/api/characters/ability-methods');
    expect(got).toEqual(['point_buy', 'standard_array', 'roll']);
  });

  it('passes campaign_id through', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([]),
    });
    await fetchAbilityMethods('camp-1');
    expect(fetch.mock.calls[0][0]).toBe(
      '/dashboard/api/characters/ability-methods?campaign_id=camp-1',
    );
  });
});

describe('previewCharacter', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('POSTs a JSON body to /preview', async () => {
    const stats = { hp_max: 10, ac: 11 };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(stats),
    });
    const submission = { name: 'Tess', race: 'Elf' };
    const got = await previewCharacter(submission);
    expect(got).toEqual(stats);
    const [url, opts] = fetch.mock.calls[0];
    expect(url).toBe('/dashboard/api/characters/preview');
    expect(opts.method).toBe('POST');
    expect(opts.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(opts.body)).toEqual(submission);
  });

  it('throws on non-OK with the server text', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve('invalid'),
    });
    await expect(previewCharacter({})).rejects.toThrow(/invalid/);
  });
});

describe('createCharacter', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('POSTs the payload and returns the created ids', async () => {
    const created = { character_id: 'c-1', player_character_id: 'pc-1' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(created),
    });
    const payload = { campaign_id: 'camp-1', name: 'Thorin' };
    const got = await createCharacter(payload);
    expect(got).toEqual(created);
    const [url, opts] = fetch.mock.calls[0];
    expect(url).toBe('/dashboard/api/characters');
    expect(opts.method).toBe('POST');
    expect(JSON.parse(opts.body)).toEqual(payload);
  });

  it('throws on non-OK', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve('validation: name is required'),
    });
    await expect(createCharacter({})).rejects.toThrow(/name is required/);
  });
});

describe('parseSubclasses', () => {
  it('returns [] for nullish input', () => {
    expect(parseSubclasses(null)).toEqual([]);
    expect(parseSubclasses(undefined)).toEqual([]);
    expect(parseSubclasses('')).toEqual([]);
  });

  it('parses a JSON-string array of objects', () => {
    const raw = JSON.stringify([{ name: 'Champion' }, { name: 'Battle Master' }]);
    expect(parseSubclasses(raw)).toEqual([
      { value: 'Champion', label: 'Champion' },
      { value: 'Battle Master', label: 'Battle Master' },
    ]);
  });

  it('parses a plain array of objects', () => {
    expect(parseSubclasses([{ name: 'Champion' }])).toEqual([
      { value: 'Champion', label: 'Champion' },
    ]);
  });

  it('parses an array of bare strings', () => {
    expect(parseSubclasses(['Champion', 'Battle Master'])).toEqual([
      { value: 'Champion', label: 'Champion' },
      { value: 'Battle Master', label: 'Battle Master' },
    ]);
  });

  it('parses an object keyed by subclass id', () => {
    const obj = { champion: { name: 'Champion' }, 'battle-master': { name: 'Battle Master' } };
    const got = parseSubclasses(obj);
    expect(got).toContainEqual({ value: 'Champion', label: 'Champion' });
    expect(got).toContainEqual({ value: 'Battle Master', label: 'Battle Master' });
  });

  it('falls back to the object key when no name is present', () => {
    expect(parseSubclasses({ champion: {} })).toEqual([
      { value: 'champion', label: 'champion' },
    ]);
  });

  it('returns [] when the string is not valid JSON', () => {
    expect(parseSubclasses('not-json')).toEqual([]);
  });
});

describe('roll4d6DropLowest', () => {
  it('returns the sum of the three highest dice', () => {
    // Sequential RNG yields 0, 0.5, 0.99, 0.99
    // -> dice 1, 4, 6, 6 -> drop the lowest (1) -> 4 + 6 + 6
    const values = [0, 0.5, 0.99, 0.99];
    let i = 0;
    const rng = () => values[i++];
    const { score, dice } = roll4d6DropLowest(rng);
    expect(dice).toEqual([1, 4, 6, 6]);
    expect(score).toBe(4 + 6 + 6);
  });

  it('always returns four dice and a sum >= 3 and <= 18', () => {
    for (let i = 0; i < 20; i += 1) {
      const { score, dice } = roll4d6DropLowest();
      expect(dice).toHaveLength(4);
      expect(score).toBeGreaterThanOrEqual(3);
      expect(score).toBeLessThanOrEqual(18);
    }
  });
});

describe('standardArrayScores', () => {
  it('returns the SRD-standard layout', () => {
    expect(standardArrayScores()).toEqual({
      str: 15,
      dex: 14,
      con: 13,
      int: 12,
      wis: 10,
      cha: 8,
    });
  });
});

describe('pointBuyStartingScores', () => {
  it('returns all 8s for point-buy starting scores', () => {
    expect(pointBuyStartingScores()).toEqual({
      str: 8,
      dex: 8,
      con: 8,
      int: 8,
      wis: 8,
      cha: 8,
    });
  });
});

describe('labelForAbilityMethod', () => {
  it('maps known method ids to display strings', () => {
    expect(labelForAbilityMethod('point_buy')).toBe('Point Buy');
    expect(labelForAbilityMethod('standard_array')).toBe('Standard Array');
    expect(labelForAbilityMethod('roll')).toBe('Roll');
  });

  it('passes unknown labels through unchanged', () => {
    expect(labelForAbilityMethod('mystery')).toBe('mystery');
  });
});
