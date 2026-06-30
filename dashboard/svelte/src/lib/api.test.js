import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  uploadAsset, getEnemyTurnPlan, executeEnemyTurn,
  getCombatWorkspace, updateCombatantHP, updateCombatantConditions,
  updateCombatantPosition, removeCombatant,
  listReactionsPanel, resolveReaction, cancelReaction,
  getPendingSaves, resolveMonsterSave, resolveMonsterSaveByUrl, cancelMonsterSave,
  listActionLog,
  undoLastAction,
  overrideCombatantHP as overrideCombatantHPDM,
  overrideCombatantPosition as overrideCombatantPositionDM,
  overrideCombatantConditions as overrideCombatantConditionsDM,
  overrideCombatantInitiative,
  overrideCharacterSlots,
  getCharacterSlots,
  getCharacterFeatureUses,
  overrideCharacterFeatureUses,
  saveCharacterFeatureUses,
  saveCharacterSlots,
  updateEncounterDisplayName,
  getEncounter,
  updateEncounter,
  deleteEncounter,
  duplicateEncounter,
  startCombat,
  endCombat,
  combatMapUrl,
  importTiledMap,
  reimportTiledMap,
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
  getCharacterInventory,
  addInventoryItem,
  removeInventoryItem,
  setCharacterGold,
  setInventoryItemIdentified,
  transferInventoryItem,
  applyLevelUp,
  getCurrentUser,
  listCampaigns,
  createCampaign,
  setActiveCampaign,
  listGuilds,
  getMap,
  updateMap,
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

describe('campaign APIs', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches the current dashboard user', async () => {
    const mockResponse = { discord_user_id: 'dm-1', campaign_id: 'camp-1' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });

    await expect(getCurrentUser()).resolves.toEqual(mockResponse);
    expect(fetch).toHaveBeenCalledWith('/api/me', undefined);
  });

  it('lists campaigns for the authenticated DM', async () => {
    const mockResponse = { campaigns: [{ id: 'camp-1', name: 'Local' }] };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });

    await expect(listCampaigns()).resolves.toEqual(mockResponse);
    expect(fetch).toHaveBeenCalledWith('/api/campaigns', undefined);
  });

  it('creates a campaign', async () => {
    const mockResponse = { id: 'camp-1', name: 'Local', guild_id: 'guild-1' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });

    await expect(createCampaign({ name: 'Local', guild_id: 'guild-1' })).resolves.toEqual(mockResponse);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/campaigns');
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body)).toEqual({ name: 'Local', guild_id: 'guild-1' });
  });

  it('sets the active campaign', async () => {
    const mockResponse = { campaign_id: 'camp-1', status: 'active' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });

    await expect(setActiveCampaign('camp-1')).resolves.toEqual(mockResponse);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/campaigns/camp-1/set-active');
    expect(options.method).toBe('POST');
  });

  it('lists guilds the bot is in', async () => {
    const mockResponse = { guilds: [{ id: 'g1', name: 'Tavern' }] };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });

    await expect(listGuilds()).resolves.toEqual(mockResponse);
    expect(fetch).toHaveBeenCalledWith('/api/guilds', undefined);
  });
});

describe('combatMapUrl', () => {
  it('returns the unfogged DM map URL by default', () => {
    expect(combatMapUrl('enc-uuid')).toBe('/api/combat/enc-uuid/map.png');
  });

  it('appends ?view=player for the fogged player preview', () => {
    expect(combatMapUrl('enc-uuid', { playerView: true })).toBe(
      '/api/combat/enc-uuid/map.png?view=player',
    );
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

describe('endCombat', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('POSTs to the end endpoint and returns the summary', async () => {
    const mockResult = {
      encounter: { id: 'enc-1', status: 'completed' },
      combatants: [],
      summary: 'Victory',
      casualties: 1,
      rounds_elapsed: 3,
      initiative_tracker: '',
    };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResult),
    });

    const result = await endCombat('enc-1');
    expect(result).toEqual(mockResult);

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/end');
    expect(options.method).toBe('POST');
  });

  it('throws on a non-ok response (e.g. encounter not active)', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      text: () => Promise.resolve('encounter is not active'),
    });
    await expect(endCombat('enc-1')).rejects.toThrow('encounter is not active');
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

// Encounter CRUD must pass campaign_id as a query param — the backend
// requires it on GET/PUT/DELETE/duplicate (400 "campaign_id query parameter
// required" otherwise).
describe('encounter CRUD passes campaign_id', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('getEncounter GETs with campaign_id', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ id: 'enc-1' }) });
    await getEncounter('enc-1', 'camp-9');
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/encounters/enc-1?campaign_id=camp-9');
    expect(options).toBeUndefined();
  });

  it('updateEncounter PUTs with campaign_id and JSON body', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ id: 'enc-1' }) });
    const payload = { name: 'Cellar', creatures: [] };
    await updateEncounter('enc-1', payload, 'camp-9');
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/encounters/enc-1?campaign_id=camp-9');
    expect(options.method).toBe('PUT');
    expect(JSON.parse(options.body)).toEqual(payload);
  });

  it('deleteEncounter DELETEs with campaign_id', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, text: () => Promise.resolve('') });
    await deleteEncounter('enc-1', 'camp-9');
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/encounters/enc-1?campaign_id=camp-9');
    expect(options.method).toBe('DELETE');
  });

  it('duplicateEncounter POSTs to /duplicate with campaign_id', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ id: 'enc-2' }) });
    await duplicateEncounter('enc-1', 'camp-9');
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/encounters/enc-1/duplicate?campaign_id=camp-9');
    expect(options.method).toBe('POST');
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

// --- Pending Monster Saves API ---

describe('getPendingSaves', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('GETs the encounter pending-saves list', async () => {
    const mock = [
      { id: 's-1', combatant_id: 'c-1', combatant_name: 'Wight', ability: 'con', dc: 13, source: 'spell', spell_id: 'fireball', cover_bonus: 0 },
    ];
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(mock) });

    const result = await getPendingSaves('enc-1');
    expect(result).toEqual(mock);
    expect(fetch).toHaveBeenCalledWith('/api/combat/enc-1/pending-saves', undefined);
  });

  it('throws on non-ok response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve('invalid encounter ID'),
    });
    await expect(getPendingSaves('bad')).rejects.toThrow('invalid encounter ID');
  });
});

describe('resolveMonsterSave', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('POSTs an empty JSON body to the resolve endpoint and returns the rolled result', async () => {
    const mockResult = {
      save_id: 's-1', combatant_id: 'c-1', combatant_name: 'Wight', ability: 'con',
      dc: 13, natural_roll: 4, save_bonus: 0, cover_bonus: 0, total: 4, success: false,
      damage: { total_damage: 16, targets: [{ combatant_id: 'c-1', display_name: 'Wight', save_success: false, damage_dealt: 16 }] },
    };
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(mockResult) });

    const result = await resolveMonsterSave('enc-1', 's-1');
    expect(result).toEqual(mockResult);

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/pending-saves/s-1/resolve');
    expect(options.method).toBe('POST');
    expect(options.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(options.body)).toEqual({});
  });

  it('throws the server text on a 409 (already resolved / player save)', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      text: () => Promise.resolve('pending save already resolved'),
    });
    await expect(resolveMonsterSave('enc-1', 's-1')).rejects.toThrow('pending save already resolved');
  });
});

describe('resolveMonsterSaveByUrl', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('POSTs an empty JSON body to the supplied resolve URL', async () => {
    const mockResult = { save_id: 's-9', success: true };
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(mockResult) });

    const url = '/api/combat/enc-7/pending-saves/s-9/resolve';
    const result = await resolveMonsterSaveByUrl(url);
    expect(result).toEqual(mockResult);

    const [calledUrl, options] = fetch.mock.calls[0];
    expect(calledUrl).toBe(url);
    expect(options.method).toBe('POST');
    expect(options.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(options.body)).toEqual({});
  });

  it('throws the server text on a 404', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      text: () => Promise.resolve('pending save not found'),
    });
    await expect(resolveMonsterSaveByUrl('/api/combat/x/pending-saves/y/resolve')).rejects.toThrow('pending save not found');
  });
});

describe('cancelMonsterSave', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('POSTs an empty JSON body to the cancel endpoint and returns the result', async () => {
    const mockResult = { save_id: 's-1', spell_id: 'shatter', canceled: 2 };
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(mockResult) });

    const result = await cancelMonsterSave('enc-1', 's-1');
    expect(result).toEqual(mockResult);

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/pending-saves/s-1/cancel');
    expect(options.method).toBe('POST');
    expect(options.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(options.body)).toEqual({});
  });

  it('throws the server text on a 409 (cast already applied)', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      text: () => Promise.resolve('pending save is already resolved'),
    });
    await expect(cancelMonsterSave('enc-1', 's-1')).rejects.toThrow('pending save is already resolved');
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

describe('overrideCharacterSlots', () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it('POSTs the slot override payload to the in-combat slots endpoint', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ status: 'ok' }) });
    const payload = {
      spell_slots: { '1': { current: 1, max: 4 } },
      pact_magic_slots: { slot_level: 2, current: 0, max: 2 },
      reason: 'fix',
    };
    const result = await overrideCharacterSlots('enc-1', 'char-1', payload);
    expect(result).toEqual({ status: 'ok' });
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/override/character/char-1/slots');
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body)).toEqual(payload);
  });

  it('throws the server text on a non-2xx (e.g. 404 no active turn)', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      text: () => Promise.resolve('no active turn'),
    });
    await expect(overrideCharacterSlots('enc-1', 'char-1', {})).rejects.toThrow(/no active turn/);
  });
});

describe('getCharacterFeatureUses', () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it('GETs the per-character feature-uses endpoint and returns parsed JSON', async () => {
    const body = {
      feature_uses: { rage: { current: 1, max: 3, recharge: 'long' } },
    };
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(body) });
    const result = await getCharacterFeatureUses('char-1');
    expect(result).toEqual(body);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/character-overview/char-1/feature-uses');
    // A bare GET (no explicit method/body).
    expect(options).toBeUndefined();
  });

  it('throws the server text on a non-2xx', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 403,
      text: () => Promise.resolve('forbidden'),
    });
    await expect(getCharacterFeatureUses('char-1')).rejects.toThrow(/forbidden/);
  });
});

describe('overrideCharacterFeatureUses', () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it('POSTs the single-feature override body to the in-combat feature-uses endpoint', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({ status: 'ok' }) });
    const payload = { feature: 'rage', current: 2, reason: 'spent' };
    const result = await overrideCharacterFeatureUses('enc-1', 'char-1', payload);
    expect(result).toEqual({ status: 'ok' });
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/combat/enc-1/override/character/char-1/feature-uses');
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body)).toEqual(payload);
  });

  it('throws the server text on a non-2xx (e.g. 404 no active turn)', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      text: () => Promise.resolve('no active turn'),
    });
    await expect(
      overrideCharacterFeatureUses('enc-1', 'char-1', { feature: 'rage', current: 2 }),
    ).rejects.toThrow(/no active turn/);
  });
});

describe('saveCharacterFeatureUses', () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it('POSTs the change batch to the out-of-combat feature-uses endpoint', async () => {
    const body = { feature_uses: { rage: { current: 2, max: 3, recharge: 'long' } } };
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(body) });
    const payload = { changes: [{ feature: 'rage', current: 2 }], reason: 'top up' };
    const result = await saveCharacterFeatureUses('char-1', payload);
    expect(result).toEqual(body);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/character-overview/char-1/feature-uses');
    expect(options.method).toBe('POST');
    expect(options.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(options.body)).toEqual(payload);
  });

  it('throws the server text on a 409 (active combat)', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      text: () => Promise.resolve('character is in an active combat; use the in-combat controls'),
    });
    await expect(
      saveCharacterFeatureUses('char-1', { changes: [{ feature: 'rage', current: 2 }] }),
    ).rejects.toThrow(/in-combat controls/);
  });
});

describe('getCharacterSlots', () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it('GETs the per-character slots endpoint and returns parsed JSON', async () => {
    const body = {
      spell_slots: { '1': { current: 2, max: 4 } },
      pact_magic_slots: { slot_level: 2, current: 0, max: 2 },
    };
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(body) });
    const result = await getCharacterSlots('char-1');
    expect(result).toEqual(body);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/character-overview/char-1/slots');
    // A bare GET (no explicit method/body).
    expect(options).toBeUndefined();
  });

  it('throws the server text on a non-2xx', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 403,
      text: () => Promise.resolve('forbidden'),
    });
    await expect(getCharacterSlots('char-1')).rejects.toThrow(/forbidden/);
  });
});

describe('saveCharacterSlots', () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it('POSTs the slot payload to the out-of-combat slots endpoint', async () => {
    const body = { spell_slots: { '1': { current: 1, max: 4 } }, pact_magic_slots: null };
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(body) });
    const payload = { spell_slots: { '1': { current: 1, max: 4 } }, reason: 'rest' };
    const result = await saveCharacterSlots('char-1', payload);
    expect(result).toEqual(body);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/character-overview/char-1/slots');
    expect(options.method).toBe('POST');
    expect(options.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(options.body)).toEqual(payload);
  });

  it('throws the server text on a 409 (active combat)', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      text: () => Promise.resolve('character is in an active combat; use the in-combat controls'),
    });
    await expect(saveCharacterSlots('char-1', {})).rejects.toThrow(/in-combat controls/);
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

// F-7: multi-file Tiled project import (.tmj + tileset/image-layer images).
// MapEditor.svelte's import control now POSTs multipart/form-data to
// /api/maps/import with campaign_id, name, the tmj file, and one `images`
// entry per referenced image file. The browser sets the multipart boundary,
// so importTiledMap must NOT set a Content-Type header.
describe('importTiledMap', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('POSTs multipart form-data with campaign_id, name, the tmj, and every image', async () => {
    const responseBody = {
      map: { id: 'map-123', name: 'Dungeon', width: 10, height: 8, tiled_json: {} },
      skipped: [],
    };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(responseBody),
    });

    const tmjFile = new File(['{"layers":[]}'], 'dungeon.tmj', { type: 'application/json' });
    const img1 = new File(['png-a'], 'tiles.png', { type: 'image/png' });
    const img2 = new File(['png-b'], 'bg.png', { type: 'image/png' });

    const result = await importTiledMap({
      campaignId: 'campaign-uuid',
      name: 'Dungeon',
      tmjFile,
      imageFiles: [img1, img2],
    });

    expect(result).toEqual(responseBody);
    expect(fetch).toHaveBeenCalledTimes(1);

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/maps/import');
    expect(options.method).toBe('POST');
    // The browser must set the multipart boundary — no manual Content-Type.
    expect(options.headers).toBeUndefined();
    expect(options.body).toBeInstanceOf(FormData);

    const fd = options.body;
    expect(fd.get('campaign_id')).toBe('campaign-uuid');
    expect(fd.get('name')).toBe('Dungeon');
    expect(fd.get('tmj')).toBeInstanceOf(File);
    expect(fd.get('tmj').name).toBe('dungeon.tmj');

    const images = fd.getAll('images');
    expect(images).toHaveLength(2);
    expect(images.map((f) => f.name)).toEqual(['tiles.png', 'bg.png']);
  });

  it('handles a tmj with no referenced images (no images entries)', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ map: {}, skipped: [] }),
    });
    const tmjFile = new File(['{"layers":[]}'], 'abstract.tmj', { type: 'application/json' });

    await importTiledMap({ campaignId: 'c1', name: 'M', tmjFile, imageFiles: [] });

    const fd = fetch.mock.calls[0][1].body;
    expect(fd.getAll('images')).toEqual([]);
    expect(fd.get('tmj').name).toBe('abstract.tmj');
  });

  it('defaults imageFiles to an empty list when omitted', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ map: {}, skipped: [] }),
    });
    const tmjFile = new File(['{"layers":[]}'], 'm.tmj', { type: 'application/json' });

    await importTiledMap({ campaignId: 'c1', name: 'M', tmjFile });

    const fd = fetch.mock.calls[0][1].body;
    expect(fd.getAll('images')).toEqual([]);
  });

  it('surfaces the backend 400 missing-image text from a non-ok response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve('missing images: tiles.png, bg.png'),
    });
    const tmjFile = new File(['{"layers":[]}'], 'big.tmj', { type: 'application/json' });
    await expect(
      importTiledMap({ campaignId: 'c1', name: 'Big', tmjFile, imageFiles: [] }),
    ).rejects.toThrow('missing images: tiles.png, bg.png');
  });
});

// reimportTiledMap overwrites an existing map in place: PUT multipart to
// /api/maps/{id}/import. Like importTiledMap, the browser sets the multipart
// boundary, so no manual Content-Type header.
describe('reimportTiledMap', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('PUTs multipart form-data to /api/maps/{id}/import with the tmj and images', async () => {
    const responseBody = {
      map: { id: 'map-123', name: 'Dungeon', width: 12, height: 9, tiled_json: {} },
      skipped: [],
    };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(responseBody),
    });

    const tmjFile = new File(['{"layers":[]}'], 'dungeon-v2.tmj', { type: 'application/json' });
    const img1 = new File(['png-a'], 'tiles.png', { type: 'image/png' });

    const result = await reimportTiledMap({
      mapId: 'map-123',
      campaignId: 'campaign-uuid',
      name: 'Dungeon',
      tmjFile,
      imageFiles: [img1],
    });

    expect(result).toEqual(responseBody);
    expect(fetch).toHaveBeenCalledTimes(1);

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/maps/map-123/import');
    expect(options.method).toBe('PUT');
    expect(options.headers).toBeUndefined();
    expect(options.body).toBeInstanceOf(FormData);

    const fd = options.body;
    expect(fd.get('campaign_id')).toBe('campaign-uuid');
    expect(fd.get('name')).toBe('Dungeon');
    expect(fd.get('tmj').name).toBe('dungeon-v2.tmj');
    expect(fd.getAll('images').map((f) => f.name)).toEqual(['tiles.png']);
  });

  it('defaults imageFiles to an empty list when omitted', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ map: {}, skipped: [] }),
    });
    const tmjFile = new File(['{"layers":[]}'], 'm.tmj', { type: 'application/json' });

    await reimportTiledMap({ mapId: 'm1', campaignId: 'c1', name: 'M', tmjFile });

    const fd = fetch.mock.calls[0][1].body;
    expect(fd.getAll('images')).toEqual([]);
  });

  it('surfaces a backend error body from a non-ok response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      text: () => Promise.resolve('map not found'),
    });
    const tmjFile = new File(['{"layers":[]}'], 'x.tmj', { type: 'application/json' });
    await expect(
      reimportTiledMap({ mapId: 'gone', campaignId: 'c1', name: 'X', tmjFile }),
    ).rejects.toThrow('map not found');
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

// F-16: SPA migration of the Phase 89 level-up widget.
describe('applyLevelUp', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('POSTs the level-up payload to /api/levelup and returns the parsed response', async () => {
    const mockResp = {
      new_level: 5,
      hp_gained: 7,
      new_hp_max: 42,
      new_proficiency_bonus: 3,
      new_attacks_per_action: 2,
      grants_asi: false,
      needs_subclass: false,
    };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResp),
    });

    const result = await applyLevelUp({
      character_id: '11111111-2222-3333-4444-555555555555',
      class_id: 'fighter',
      new_level: 5,
    });

    expect(result).toEqual(mockResp);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/levelup');
    expect(options.method).toBe('POST');
    expect(options.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(options.body)).toEqual({
      character_id: '11111111-2222-3333-4444-555555555555',
      class_id: 'fighter',
      new_level: 5,
    });
  });

  it('throws when the server returns a non-OK status', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve('character_id is required'),
    });

    await expect(
      applyLevelUp({ character_id: '', class_id: '', new_level: 0 }),
    ).rejects.toThrow('character_id is required');
  });
});

describe('getMap', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('scopes the request with the campaign_id query parameter', async () => {
    const mockMap = { id: 'map-uuid', name: 'Cave' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockMap),
    });

    const result = await getMap('map-uuid', 'campaign-uuid');

    expect(result).toEqual(mockMap);
    const [url] = fetch.mock.calls[0];
    expect(url).toBe('/api/maps/map-uuid?campaign_id=campaign-uuid');
  });
});

describe('updateMap', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('scopes the PUT with the campaign_id query parameter and sends the payload', async () => {
    const mockMap = { id: 'map-uuid', name: 'Cave' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockMap),
    });

    const result = await updateMap('map-uuid', 'campaign-uuid', { name: 'Cave' });

    expect(result).toEqual(mockMap);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/maps/map-uuid?campaign_id=campaign-uuid');
    expect(options.method).toBe('PUT');
    expect(JSON.parse(options.body)).toEqual({ name: 'Cave' });
  });
});

describe('DM Inventory API', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('getCharacterInventory fetches by character_id', async () => {
    const mock = { character_id: 'char-1', name: 'Aria', gold: 120, items: [] };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mock),
    });

    const result = await getCharacterInventory('char-1');
    expect(result).toEqual(mock);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/inventory?character_id=char-1');
    expect(options).toBeUndefined();
  });

  it('addInventoryItem posts character_id + item', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ status: 'ok' }),
    });

    const item = { item_id: 'rope', name: 'Rope', quantity: 1, type: 'gear' };
    const result = await addInventoryItem('char-1', item);
    expect(result).toEqual({ status: 'ok' });
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/inventory/add');
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body)).toEqual({ character_id: 'char-1', item });
  });

  it('removeInventoryItem posts character_id, item_id, quantity', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ status: 'ok' }),
    });

    await removeInventoryItem('char-1', 'rope', 2);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/inventory/remove');
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body)).toEqual({ character_id: 'char-1', item_id: 'rope', quantity: 2 });
  });

  it('setCharacterGold posts character_id + gold', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ status: 'ok' }),
    });

    await setCharacterGold('char-1', 50);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/inventory/gold');
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body)).toEqual({ character_id: 'char-1', gold: 50 });
  });

  it('setInventoryItemIdentified posts character_id, item_id, identified', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ status: 'ok' }),
    });

    await setInventoryItemIdentified('char-1', 'cloak', true);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/inventory/identify');
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body)).toEqual({ character_id: 'char-1', item_id: 'cloak', identified: true });
  });

  it('transferInventoryItem posts from/to/item_id/quantity', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ status: 'ok' }),
    });

    await transferInventoryItem('char-1', 'char-2', 'rope', 3);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/inventory/transfer');
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body)).toEqual({
      from_character_id: 'char-1',
      to_character_id: 'char-2',
      item_id: 'rope',
      quantity: 3,
    });
  });
});
