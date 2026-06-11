import { describe, it, expect, vi, beforeEach } from 'vitest';
import { listRaces, listClasses, listSpells, submitCharacter, getPreparation, savePreparation, makeBuilderApi } from './api.js';

describe('API client', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('listRaces fetches from /portal/api/races', async () => {
    const mockRaces = [{ id: 'elf', name: 'Elf' }];
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockRaces),
    });

    const result = await listRaces();
    expect(result).toEqual(mockRaces);
    expect(fetch).toHaveBeenCalledWith('/portal/api/races', undefined);
  });

  it('listClasses fetches from /portal/api/classes', async () => {
    const mockClasses = [{ id: 'fighter', name: 'Fighter' }];
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockClasses),
    });

    const result = await listClasses();
    expect(result).toEqual(mockClasses);
  });

  it('listSpells fetches with class param', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([]),
    });

    await listSpells('wizard');
    expect(fetch).toHaveBeenCalledWith('/portal/api/spells?class=wizard', undefined);
  });

  it('listSpells includes campaign_id when provided', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([]),
    });

    await listSpells('wizard', 'camp-123');
    expect(fetch).toHaveBeenCalledWith('/portal/api/spells?class=wizard&campaign_id=camp-123', undefined);
  });

  it('submitCharacter posts to /portal/api/characters', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ character_id: 'c-1' }),
    });

    const data = { name: 'Test', race: 'elf' };
    const result = await submitCharacter(data);
    expect(result.character_id).toBe('c-1');
    expect(fetch).toHaveBeenCalledWith('/portal/api/characters', expect.objectContaining({
      method: 'POST',
    }));
  });

  it('getPreparation fetches the character preparation endpoint', async () => {
    const mockInfo = { max_prepared: 4, current_prepared: [], spells: [] };
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockInfo),
    });

    const result = await getPreparation('char-1');
    expect(result).toEqual(mockInfo);
    expect(fetch).toHaveBeenCalledWith('/portal/api/characters/char-1/preparation', undefined);
  });

  it('savePreparation posts the chosen spells', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ prepared_count: 2, max_prepared: 4, always_prepared: [] }),
    });

    const result = await savePreparation('char-1', ['bless', 'guidance']);
    expect(result.prepared_count).toBe(2);
    expect(fetch).toHaveBeenCalledWith(
      '/portal/api/characters/char-1/preparation',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ spells: ['bless', 'guidance'] }),
      }),
    );
  });

  it('throws on non-OK response', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve('bad request'),
    });

    await expect(listRaces()).rejects.toThrow('bad request');
  });
});

describe('makeBuilderApi player getCharacterDraft', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('GETs the draft endpoint and returns body.draft', async () => {
    const draft = { v: 1, name: 'Gimli', currentStep: 2 };
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ draft }),
    });

    const api = makeBuilderApi('player', { campaignId: 'camp-1', token: 'tok' });
    const result = await api.getCharacterDraft();

    expect(result).toEqual(draft);
    expect(fetch).toHaveBeenCalledWith('/portal/api/characters/draft?campaign_id=camp-1', undefined);
  });

  it('returns null when the server stored no draft', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ draft: null }),
    });

    const api = makeBuilderApi('player', { campaignId: 'camp-1' });
    expect(await api.getCharacterDraft()).toBe(null);
  });

  it('returns null and does NOT fetch when campaignId is empty', async () => {
    global.fetch = vi.fn();

    const api = makeBuilderApi('player', { campaignId: '' });
    const result = await api.getCharacterDraft();

    expect(result).toBe(null);
    expect(fetch).not.toHaveBeenCalled();
  });

  it('encodes the campaign id in the query string', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ draft: null }),
    });

    const api = makeBuilderApi('player', { campaignId: 'a/b c' });
    await api.getCharacterDraft();

    expect(fetch).toHaveBeenCalledWith('/portal/api/characters/draft?campaign_id=a%2Fb%20c', undefined);
  });

  it('propagates a fetch error to the caller', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      text: () => Promise.resolve('unauthorized'),
    });

    const api = makeBuilderApi('player', { campaignId: 'camp-1' });
    await expect(api.getCharacterDraft()).rejects.toThrow('unauthorized');
  });
});

describe('makeBuilderApi player submitCharacter builder_draft', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('includes builder_draft in the POST body when provided', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ character_id: 'c-1' }),
    });

    const draft = { v: 1, name: 'Gimli' };
    const api = makeBuilderApi('player', { campaignId: 'camp-1', token: 'tok' });
    await api.submitCharacter({ name: 'Gimli' }, draft);

    const body = JSON.parse(fetch.mock.calls[0][1].body);
    expect(body.builder_draft).toEqual(draft);
    expect(body.token).toBe('tok');
    expect(body.campaign_id).toBe('camp-1');
    expect(body.name).toBe('Gimli');
  });

  it('omits builder_draft when no draft is passed', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ character_id: 'c-1' }),
    });

    const api = makeBuilderApi('player', { campaignId: 'camp-1', token: 'tok' });
    await api.submitCharacter({ name: 'Gimli' });

    const body = JSON.parse(fetch.mock.calls[0][1].body);
    expect('builder_draft' in body).toBe(false);
  });
});

describe('makeBuilderApi player ref-data + preview', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  function stubJson(value) {
    global.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(value) });
  }

  it('listRaces / listClasses hit the portal ref endpoints', async () => {
    stubJson([{ id: 'elf' }]);
    const api = makeBuilderApi('player', { campaignId: 'camp-1' });
    expect(await api.listRaces()).toEqual([{ id: 'elf' }]);
    expect(fetch).toHaveBeenCalledWith('/portal/api/races', undefined);
    await api.listClasses();
    expect(fetch).toHaveBeenCalledWith('/portal/api/classes', undefined);
  });

  it('listSpells appends the campaign id', async () => {
    stubJson([]);
    const api = makeBuilderApi('player', { campaignId: 'camp-1' });
    await api.listSpells('wizard');
    expect(fetch).toHaveBeenCalledWith('/portal/api/spells?class=wizard&campaign_id=camp-1', undefined);
  });

  it('listSpells omits the campaign id when none is set', async () => {
    stubJson([]);
    const api = makeBuilderApi('player', {});
    await api.listSpells('wizard');
    expect(fetch).toHaveBeenCalledWith('/portal/api/spells?class=wizard', undefined);
  });

  it('listEquipment / getStartingEquipment hit the portal endpoints', async () => {
    stubJson([]);
    const api = makeBuilderApi('player', {});
    await api.listEquipment();
    expect(fetch).toHaveBeenCalledWith('/portal/api/equipment', undefined);
    await api.getStartingEquipment('cleric');
    expect(fetch).toHaveBeenCalledWith('/portal/api/starting-equipment?class=cleric', undefined);
  });

  it('listAbilityMethods appends the campaign id when set', async () => {
    stubJson(['point_buy']);
    const api = makeBuilderApi('player', { campaignId: 'camp-1' });
    expect(await api.listAbilityMethods()).toEqual(['point_buy']);
    expect(fetch).toHaveBeenCalledWith('/portal/api/ability-methods?campaign_id=camp-1', undefined);
  });

  it('previewCharacter POSTs the submission', async () => {
    stubJson({ ok: true });
    const api = makeBuilderApi('player', {});
    await api.previewCharacter({ name: 'Gimli' });
    expect(fetch).toHaveBeenCalledWith('/portal/api/characters/preview', expect.objectContaining({ method: 'POST' }));
  });
});

describe('makeBuilderApi dm ref-data + preview', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  function stubJson(value) {
    global.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(value) });
  }

  it('listRaces / listClasses hit the dashboard ref endpoints', async () => {
    stubJson([]);
    const api = makeBuilderApi('dm', { campaignId: 'camp-1' });
    await api.listRaces();
    expect(fetch).toHaveBeenCalledWith('/dashboard/api/characters/ref/races', undefined);
    await api.listClasses();
    expect(fetch).toHaveBeenCalledWith('/dashboard/api/characters/ref/classes', undefined);
  });

  it('listSpells sets class + campaign_id query params', async () => {
    stubJson([]);
    const api = makeBuilderApi('dm', { campaignId: 'camp-1' });
    await api.listSpells('wizard');
    const url = fetch.mock.calls[0][0];
    expect(url).toContain('/dashboard/api/characters/ref/spells?');
    expect(url).toContain('class=wizard');
    expect(url).toContain('campaign_id=camp-1');
  });

  it('listEquipment appends campaign_id; getStartingEquipment hits ref', async () => {
    stubJson([]);
    const api = makeBuilderApi('dm', { campaignId: 'camp-1' });
    await api.listEquipment();
    expect(fetch).toHaveBeenCalledWith('/dashboard/api/characters/ref/equipment?campaign_id=camp-1', undefined);
    await api.getStartingEquipment('cleric');
    expect(fetch).toHaveBeenCalledWith('/dashboard/api/characters/ref/starting-equipment?class=cleric', undefined);
  });

  it('listAbilityMethods appends campaign_id', async () => {
    stubJson(['roll']);
    const api = makeBuilderApi('dm', { campaignId: 'camp-1' });
    expect(await api.listAbilityMethods()).toEqual(['roll']);
    expect(fetch).toHaveBeenCalledWith('/dashboard/api/characters/ability-methods?campaign_id=camp-1', undefined);
  });

  it('previewCharacter POSTs to the dashboard preview endpoint', async () => {
    stubJson({ ok: true });
    const api = makeBuilderApi('dm', { campaignId: 'camp-1' });
    await api.previewCharacter({ name: 'Goblin' });
    expect(fetch).toHaveBeenCalledWith('/dashboard/api/characters/preview', expect.objectContaining({ method: 'POST' }));
  });
});

describe('makeBuilderApi dm parity', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('getCharacterDraft resolves to null without fetching', async () => {
    global.fetch = vi.fn();

    const api = makeBuilderApi('dm', { campaignId: 'camp-1' });
    expect(await api.getCharacterDraft()).toBe(null);
    expect(fetch).not.toHaveBeenCalled();
  });

  it('submitCharacter ignores the builder_draft arg (dashboard endpoint)', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ character_id: 'c-1' }),
    });

    const api = makeBuilderApi('dm', { campaignId: 'camp-1' });
    await api.submitCharacter({ name: 'Goblin' }, { v: 1, name: 'Goblin' });

    const body = JSON.parse(fetch.mock.calls[0][1].body);
    expect('builder_draft' in body).toBe(false);
    expect(body.campaign_id).toBe('camp-1');
    expect(body.name).toBe('Goblin');
  });
});
