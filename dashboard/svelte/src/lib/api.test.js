import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  uploadAsset, getEnemyTurnPlan, executeEnemyTurn,
  getCombatWorkspace, updateCombatantHP, updateCombatantConditions,
  updateCombatantPosition, removeCombatant,
  listReactionsPanel, resolveReaction, cancelReaction,
  listActionLog,
  undoLastAction,
  overrideCombatantHP as overrideCombatantHPDM,
  overrideCombatantPosition as overrideCombatantPositionDM,
  overrideCombatantConditions as overrideCombatantConditionsDM,
  overrideCombatantInitiative,
  overrideCharacterSpellSlots,
  updateEncounterDisplayName,
  startCombat,
  importTiledMap,
  listOpen5eSources,
  getCampaignOpen5eSources,
  updateCampaignOpen5eSources,
  getLootPool,
  createLootPool,
  addLootPoolItem,
  removeLootPoolItem,
  setLootGold,
  postLootAnnouncement,
  listEligibleLootEncounters,
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

// Phase 105c: updateEncounterDisplayName
describe('updateEncounterDisplayName', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('sends PATCH with the new display name and returns parsed JSON', async () => {
    const mockResult = { id: 'enc-1', name: 'Goblin Ambush', display_name: 'Forest Chase' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResult),
    });

    const result = await updateEncounterDisplayName('enc-1', 'Forest Chase');
    expect(result).toEqual(mockResult);

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/display-name');
    expect(options.method).toBe('PATCH');
    expect(options.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(options.body)).toEqual({ display_name: 'Forest Chase' });
  });

  it('passes empty string through unchanged so the backend clears the override', async () => {
    const mockResult = { id: 'enc-1', name: 'Goblin Ambush', display_name: null };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResult),
    });

    const result = await updateEncounterDisplayName('enc-1', '');
    expect(result).toEqual(mockResult);

    const [, options] = fetch.mock.calls[0];
    expect(JSON.parse(options.body)).toEqual({ display_name: '' });
  });

  it('throws on non-ok response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: () => Promise.resolve('boom'),
    });

    await expect(updateEncounterDisplayName('enc-1', 'x')).rejects.toThrow('boom');
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

// --- Reactions Panel API ---

describe('listReactionsPanel', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches enriched reactions for the panel', async () => {
    const mockData = [
      { id: 'r-1', combatant_display_name: 'Aragorn', description: 'Shield', status: 'active' },
    ];
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockData),
    });

    const result = await listReactionsPanel('enc-1');
    expect(result).toEqual(mockData);
    expect(fetch).toHaveBeenCalledWith(
      '/api/combat/enc-1/reactions/panel',
      undefined,
    );
  });
});

describe('resolveReaction', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('sends POST to resolve a reaction', async () => {
    const mockResult = { id: 'r-1', status: 'used' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResult),
    });

    const result = await resolveReaction('enc-1', 'r-1');
    expect(result).toEqual(mockResult);

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/reactions/r-1/resolve');
    expect(options.method).toBe('POST');
  });
});

describe('cancelReaction', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('sends POST to cancel a reaction', async () => {
    const mockResult = { id: 'r-1', status: 'cancelled' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResult),
    });

    const result = await cancelReaction('enc-1', 'r-1');
    expect(result).toEqual(mockResult);

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/reactions/r-1/cancel');
    expect(options.method).toBe('POST');
  });
});

// --- Action Log Viewer API ---

describe('listActionLog', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches action log with no filters', async () => {
    const mock = [{ id: 'log-1', action_type: 'attack' }];
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(mock) });

    const result = await listActionLog('enc-1');
    expect(result).toEqual(mock);
    expect(fetch).toHaveBeenCalledWith('/api/combat/enc-1/action-log', undefined);
  });

  it('fetches action log with filters', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) });

    await listActionLog('enc-1', {
      actionTypes: ['attack', 'dm_override'],
      actorId: 'actor-1',
      targetId: 'target-1',
      round: 3,
      turnId: 'turn-1',
      sort: 'asc',
    });

    const [url] = fetch.mock.calls[0];
    expect(url).toContain('/api/combat/enc-1/action-log?');
    expect(url).toContain('action_type=attack%2Cdm_override');
    expect(url).toContain('actor_id=actor-1');
    expect(url).toContain('target_id=target-1');
    expect(url).toContain('round=3');
    expect(url).toContain('turn_id=turn-1');
    expect(url).toContain('sort=asc');
  });

  it('omits empty filter fields', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) });

    await listActionLog('enc-1', { actionTypes: [], actorId: '', round: 0 });

    const [url] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/action-log');
  });

  it('throws on non-ok response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve('bad request'),
    });
    await expect(listActionLog('enc-1')).rejects.toThrow('bad request');
  });
});

// --- Undo & Manual Override (Phase 97b) ---

describe('undoLastAction', () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it('POSTs reason in body', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ status: 'undone' }) });
    const result = await undoLastAction('enc-1', 'missed resistance');
    expect(result).toEqual({ status: 'undone' });
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/undo-last-action');
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body)).toEqual({ reason: 'missed resistance' });
  });

  it('defaults reason to empty string', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({}) });
    await undoLastAction('enc-1');
    const [, options] = fetch.mock.calls[0];
    expect(JSON.parse(options.body)).toEqual({ reason: '' });
  });

  it('throws on non-ok response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: false, status: 404, text: () => Promise.resolve('nothing to undo') });
    await expect(undoLastAction('enc-1')).rejects.toThrow('nothing to undo');
  });
});

describe('overrideCombatantHP (DM dashboard)', () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it('POSTs HP override payload', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ status: 'ok' }) });
    await overrideCombatantHPDM('enc-1', 'comb-1', { hp_current: 7, temp_hp: 0, reason: 'fix' });
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/override/combatant/comb-1/hp');
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body)).toEqual({ hp_current: 7, temp_hp: 0, reason: 'fix' });
  });
});

describe('overrideCombatantPosition (DM dashboard)', () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it('POSTs position override payload with default altitude', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ status: 'ok' }) });
    await overrideCombatantPositionDM('enc-1', 'comb-1', { position_col: 'C', position_row: 4, reason: 'wrong tile' });
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/override/combatant/comb-1/position');
    const body = JSON.parse(options.body);
    expect(body.position_col).toBe('C');
    expect(body.position_row).toBe(4);
    expect(body.altitude_ft).toBe(0);
    expect(body.reason).toBe('wrong tile');
  });
});

describe('overrideCombatantConditions (DM dashboard)', () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it('POSTs conditions override payload', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ status: 'ok' }) });
    await overrideCombatantConditionsDM('enc-1', 'comb-1', { conditions: [{ condition: 'blessed' }], reason: 'forgot bless' });
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/override/combatant/comb-1/conditions');
    expect(JSON.parse(options.body)).toEqual({ conditions: [{ condition: 'blessed' }], reason: 'forgot bless' });
  });
});

describe('overrideCombatantInitiative', () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it('POSTs initiative override payload', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ status: 'ok' }) });
    await overrideCombatantInitiative('enc-1', 'comb-1', { initiative_roll: 18, initiative_order: 1, reason: 'reroll' });
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/override/combatant/comb-1/initiative');
    expect(JSON.parse(options.body)).toEqual({ initiative_roll: 18, initiative_order: 1, reason: 'reroll' });
  });
});

describe('overrideCharacterSpellSlots', () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it('POSTs spell slot override payload', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ status: 'ok' }) });
    await overrideCharacterSpellSlots('enc-1', 'char-1', { spell_slots: { '1': { max: 4, used: 1 } }, reason: 'fix' });
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/override/character/char-1/spell-slots');
    expect(JSON.parse(options.body)).toEqual({ spell_slots: { '1': { max: 4, used: 1 } }, reason: 'fix' });
  });
});

// Phase 114 — startCombat posts to /api/combat/start with the shape the Go
// handler expects (template_id, character_ids, character_positions,
// surprised_combatant_short_ids). The DM flips the surprise toggle per
// combatant in the encounter builder before clicking Start.
describe('startCombat', () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it('POSTs template + surprised short IDs to /api/combat/start', async () => {
    const mockResponse = {
      encounter: { id: 'enc-1', round_number: 1 },
      combatants: [],
      initiative_tracker: '...',
      first_turn: { turn_id: 't-1', combatant_id: 'c-1', round_number: 1 },
    };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });

    const result = await startCombat({
      template_id: 'tmpl-uuid',
      character_ids: ['char-1', 'char-2'],
      character_positions: { 'char-1': { col: 'A', row: 1 }, 'char-2': { col: 'B', row: 2 } },
      surprised_combatant_short_ids: ['GB2', 'OR1'],
    });

    expect(result).toEqual(mockResponse);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/start');
    expect(options.method).toBe('POST');
    expect(options.headers['Content-Type']).toBe('application/json');

    const body = JSON.parse(options.body);
    expect(body.template_id).toBe('tmpl-uuid');
    expect(body.character_ids).toEqual(['char-1', 'char-2']);
    expect(body.character_positions).toEqual({
      'char-1': { col: 'A', row: 1 },
      'char-2': { col: 'B', row: 2 },
    });
    expect(body.surprised_combatant_short_ids).toEqual(['GB2', 'OR1']);
  });

  it('omits surprised_combatant_short_ids when none selected', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({}),
    });
    await startCombat({
      template_id: 'tmpl-1',
      character_ids: [],
      character_positions: {},
    });
    const body = JSON.parse(fetch.mock.calls[0][1].body);
    // Either the field is absent or empty — both are OK per the Go request type
    // (`omitempty`). Assert it's not an accidentally-stringified value.
    if ('surprised_combatant_short_ids' in body) {
      expect(body.surprised_combatant_short_ids).toEqual([]);
    }
  });

  it('throws on non-ok response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve('invalid template_id'),
    });
    await expect(startCombat({ template_id: 'bad' })).rejects.toThrow('invalid template_id');
  });
});

// F-7: Tiled .tmj import — wired by MapEditor.svelte's new "Import Tiled" button.
describe('importTiledMap', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('reads the file as text, parses it as JSON, and POSTs to /api/maps/import', async () => {
    const responseBody = {
      map: { id: 'map-123', name: 'Dungeon', width: 10, height: 8, tiled_json: {} },
      skipped: [],
    };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(responseBody),
    });

    const tmj = { width: 10, height: 8, orientation: 'orthogonal', layers: [] };
    const file = new File([JSON.stringify(tmj)], 'dungeon.tmj', { type: 'application/json' });

    const result = await importTiledMap({
      campaignId: 'campaign-uuid',
      name: 'Dungeon',
      file,
    });

    expect(result).toEqual(responseBody);
    expect(fetch).toHaveBeenCalledTimes(1);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/maps/import');
    expect(options.method).toBe('POST');
    expect(options.headers['Content-Type']).toBe('application/json');
    const body = JSON.parse(options.body);
    expect(body.campaign_id).toBe('campaign-uuid');
    expect(body.name).toBe('Dungeon');
    expect(body.tmj).toEqual(tmj);
    expect(body.background_image_id).toBeUndefined();
  });

  it('includes background_image_id when provided', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ map: {}, skipped: [] }),
    });
    const file = new File(['{"layers":[]}'], 'm.tmj', { type: 'application/json' });
    await importTiledMap({
      campaignId: 'c1',
      name: 'M',
      file,
      backgroundImageId: 'asset-9',
    });
    const body = JSON.parse(fetch.mock.calls[0][1].body);
    expect(body.background_image_id).toBe('asset-9');
  });

  it('rejects when the file is not valid JSON', async () => {
    globalThis.fetch = vi.fn();
    const file = new File(['not json {{'], 'bad.tmj', { type: 'application/json' });
    await expect(importTiledMap({
      campaignId: 'c1',
      name: 'Bad',
      file,
    })).rejects.toThrow(/not valid JSON/);
    expect(fetch).not.toHaveBeenCalled();
  });

  it('surfaces backend error text from a non-ok response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve('infinite maps are not supported'),
    });
    const file = new File(['{"infinite":true}'], 'big.tmj', { type: 'application/json' });
    await expect(importTiledMap({
      campaignId: 'c1',
      name: 'Big',
      file,
    })).rejects.toThrow('infinite maps are not supported');
  });
});

// F-8: per-campaign Open5e source toggle — wired by Open5eSourcesPanel.svelte.
describe('listOpen5eSources', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('GETs the catalog and returns parsed JSON', async () => {
    const mockResponse = {
      sources: [
        { slug: 'wotc-srd', title: '5e SRD' },
        { slug: 'tome-of-beasts', title: 'Tome of Beasts', publisher: 'Kobold Press' },
      ],
    };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });
    const result = await listOpen5eSources();
    expect(result).toEqual(mockResponse);
    expect(fetch).toHaveBeenCalledWith('/api/open5e/sources', undefined);
  });

  it('throws on non-ok response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: () => Promise.resolve('boom'),
    });
    await expect(listOpen5eSources()).rejects.toThrow('boom');
  });
});

describe('getCampaignOpen5eSources', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('GETs per-campaign enabled list', async () => {
    const mockResponse = { campaign_id: 'cid', enabled: ['tome-of-beasts'] };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });
    const result = await getCampaignOpen5eSources('cid');
    expect(result).toEqual(mockResponse);
    expect(fetch).toHaveBeenCalledWith('/api/open5e/campaigns/cid/sources', undefined);
  });
});

describe('updateCampaignOpen5eSources', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('PUTs the new enabled list and returns parsed JSON', async () => {
    const mockResponse = { campaign_id: 'cid', enabled: ['tome-of-beasts', 'deep-magic'] };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });
    const result = await updateCampaignOpen5eSources('cid', ['tome-of-beasts', 'deep-magic']);
    expect(result).toEqual(mockResponse);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/open5e/campaigns/cid/sources');
    expect(options.method).toBe('PUT');
    expect(options.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(options.body)).toEqual({ enabled: ['tome-of-beasts', 'deep-magic'] });
  });

  it('PUTs an empty list to disable everything', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ campaign_id: 'cid', enabled: [] }),
    });
    await updateCampaignOpen5eSources('cid', []);
    const [, options] = fetch.mock.calls[0];
    expect(JSON.parse(options.body)).toEqual({ enabled: [] });
  });

  it('surfaces backend error text on non-ok response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve('unknown open5e source: "bogus"'),
    });
    await expect(updateCampaignOpen5eSources('cid', ['bogus'])).rejects.toThrow('unknown open5e source');
  });
});

// --- F-13: Loot Pool API ---

describe('loot pool API', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('getLootPool GETs the encounter pool', async () => {
    const mockResult = { Pool: { id: 'p1', gold_total: 10 }, Items: [{ id: 'it1', name: 'Sword' }] };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResult),
    });
    const result = await getLootPool('cid', 'eid');
    expect(result).toEqual(mockResult);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/campaigns/cid/encounters/eid/loot');
    // GET should send no body / method override
    expect(options).toBeUndefined();
  });

  it('createLootPool POSTs to the pool root', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ Pool: { id: 'p1' }, Items: [] }),
    });
    await createLootPool('cid', 'eid');
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/campaigns/cid/encounters/eid/loot');
    expect(options.method).toBe('POST');
  });

  it('addLootPoolItem POSTs the item JSON body', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ id: 'it1' }),
    });
    const item = { name: 'Potion of Healing', quantity: 2, type: 'consumable', is_magic: true, rarity: 'common' };
    await addLootPoolItem('cid', 'eid', item);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/campaigns/cid/encounters/eid/loot/items');
    expect(options.method).toBe('POST');
    expect(options.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(options.body)).toEqual(item);
  });

  it('removeLootPoolItem DELETEs the specific item', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ status: 'ok' }),
    });
    await removeLootPoolItem('cid', 'eid', 'it1');
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/campaigns/cid/encounters/eid/loot/items/it1');
    expect(options.method).toBe('DELETE');
  });

  it('setLootGold PUTs the new total', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ id: 'p1', gold_total: 50 }),
    });
    await setLootGold('cid', 'eid', 50);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/campaigns/cid/encounters/eid/loot/gold');
    expect(options.method).toBe('PUT');
    expect(JSON.parse(options.body)).toEqual({ gold: 50 });
  });

  it('postLootAnnouncement POSTs to the post route', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ status: 'ok', message: 'Loot available...' }),
    });
    await postLootAnnouncement('cid', 'eid');
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/campaigns/cid/encounters/eid/loot/post');
    expect(options.method).toBe('POST');
  });

  it('surfaces a 404 (no pool yet) as a rejected promise', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      text: () => Promise.resolve('loot pool not found'),
    });
    await expect(getLootPool('cid', 'eid')).rejects.toThrow('loot pool not found');
  });

  it('listEligibleLootEncounters GETs the campaign-scoped route', async () => {
    const mockResp = {
      encounters: [
        { id: 'e1', name: 'Goblin Ambush', display_name: 'Goblin Ambush!', status: 'completed' },
      ],
    };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResp),
    });
    const result = await listEligibleLootEncounters('cid');
    expect(result).toEqual(mockResp);
    const [url] = fetch.mock.calls[0];
    expect(url).toBe('/api/campaigns/cid/loot/eligible-encounters');
  });
});
