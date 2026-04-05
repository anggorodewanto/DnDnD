import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  uploadAsset, getEnemyTurnPlan, executeEnemyTurn,
  getCombatWorkspace, updateCombatantHP, updateCombatantConditions,
  updateCombatantPosition, removeCombatant,
} from './api.js';

describe('uploadAsset', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('sends multipart form data and returns response', async () => {
    const mockResponse = { id: 'abc-123', url: '/api/assets/abc-123' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });

    const file = new File(['fake-image-data'], 'map.png', { type: 'image/png' });
    const result = await uploadAsset({
      campaignId: 'campaign-uuid',
      type: 'map_background',
      file,
    });

    expect(result).toEqual(mockResponse);
    expect(fetch).toHaveBeenCalledTimes(1);

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/assets/upload');
    expect(options.method).toBe('POST');
    expect(options.body).toBeInstanceOf(FormData);

    const formData = options.body;
    expect(formData.get('campaign_id')).toBe('campaign-uuid');
    expect(formData.get('type')).toBe('map_background');
    expect(formData.get('file')).toBeInstanceOf(File);
    expect(formData.get('file').name).toBe('map.png');
  });

  it('throws on non-ok response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve('invalid asset type'),
    });

    const file = new File(['data'], 'bad.png', { type: 'image/png' });
    await expect(uploadAsset({
      campaignId: 'campaign-uuid',
      type: 'bogus',
      file,
    })).rejects.toThrow('invalid asset type');
  });
});

describe('getEnemyTurnPlan', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches turn plan for an NPC combatant', async () => {
    const mockPlan = {
      combatant_id: 'npc-uuid',
      display_name: 'Goblin',
      steps: [{ type: 'attack' }],
      reactions: [],
    };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockPlan),
    });

    const result = await getEnemyTurnPlan('encounter-uuid', 'npc-uuid');
    expect(result).toEqual(mockPlan);
    expect(fetch).toHaveBeenCalledWith(
      '/api/combat/encounter-uuid/enemy-turn/npc-uuid/plan',
      undefined,
    );
  });
});

describe('executeEnemyTurn', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('sends finalized plan and returns result', async () => {
    const mockResult = { combat_log: 'Goblin attacks...', success: true };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResult),
    });

    const plan = { combatant_id: 'npc-uuid', steps: [{ type: 'attack' }] };
    const result = await executeEnemyTurn('encounter-uuid', plan);
    expect(result).toEqual(mockResult);

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/encounter-uuid/enemy-turn');
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body)).toEqual(plan);
  });
});

// TDD Cycle 7: getCombatWorkspace
describe('getCombatWorkspace', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches combat workspace for a campaign', async () => {
    const mockData = { encounters: [] };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockData),
    });

    const result = await getCombatWorkspace('campaign-uuid');
    expect(result).toEqual(mockData);
    expect(fetch).toHaveBeenCalledWith(
      '/api/combat/workspace?campaign_id=campaign-uuid',
      undefined,
    );
  });
});

// TDD Cycle 8: updateCombatantHP
describe('updateCombatantHP', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('sends PATCH to update combatant HP', async () => {
    const mockResult = { id: 'comb-1', hp_current: 15, temp_hp: 0 };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResult),
    });

    const result = await updateCombatantHP('enc-1', 'comb-1', { hp_current: 15, temp_hp: 0, is_alive: true });
    expect(result).toEqual(mockResult);

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/combatants/comb-1/hp');
    expect(options.method).toBe('PATCH');
    expect(JSON.parse(options.body)).toEqual({ hp_current: 15, temp_hp: 0, is_alive: true });
  });
});

// TDD Cycle 9: updateCombatantConditions
describe('updateCombatantConditions', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('sends PATCH to update combatant conditions', async () => {
    const mockResult = { id: 'comb-1', conditions: ['Blinded'] };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResult),
    });

    const result = await updateCombatantConditions('enc-1', 'comb-1', ['Blinded']);
    expect(result).toEqual(mockResult);

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/combatants/comb-1/conditions');
    expect(options.method).toBe('PATCH');
    expect(JSON.parse(options.body)).toEqual({ conditions: ['Blinded'] });
  });
});

// TDD Cycle 12: updateCombatantPosition
describe('updateCombatantPosition', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('sends PATCH to update combatant position', async () => {
    const mockResult = { id: 'comb-1', position_col: 'D', position_row: 4 };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResult),
    });

    const result = await updateCombatantPosition('enc-1', 'comb-1', { position_col: 'D', position_row: 4 });
    expect(result).toEqual(mockResult);

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/combatants/comb-1/position');
    expect(options.method).toBe('PATCH');
    expect(JSON.parse(options.body)).toEqual({ position_col: 'D', position_row: 4 });
  });
});

// TDD Cycle 13: removeCombatant
describe('removeCombatant', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('sends DELETE to remove combatant', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(''),
    });

    await removeCombatant('enc-1', 'comb-1');

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/combatants/comb-1');
    expect(options.method).toBe('DELETE');
  });
});
