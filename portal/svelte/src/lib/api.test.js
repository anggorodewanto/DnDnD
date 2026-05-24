import { describe, it, expect, vi, beforeEach } from 'vitest';
import { listRaces, listClasses, listSpells, submitCharacter, getPreparation, savePreparation } from './api.js';

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
