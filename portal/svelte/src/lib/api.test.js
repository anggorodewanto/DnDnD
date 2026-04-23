import { describe, it, expect, vi, beforeEach } from 'vitest';
import { listRaces, listClasses, listSpells, submitCharacter } from './api.js';

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

  it('throws on non-OK response', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      text: () => Promise.resolve('bad request'),
    });

    await expect(listRaces()).rejects.toThrow('bad request');
  });
});
