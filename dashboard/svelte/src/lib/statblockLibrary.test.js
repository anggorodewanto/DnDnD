import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  STATBLOCK_LIBRARY_ENDPOINT,
  buildStatBlockListUrl,
  listStatBlocks,
  getStatBlock,
  normalizeStatBlock,
} from './statblockLibrary.js';

describe('STATBLOCK_LIBRARY_ENDPOINT', () => {
  it('matches the backend route', () => {
    expect(STATBLOCK_LIBRARY_ENDPOINT).toBe('/api/statblocks');
  });
});

describe('buildStatBlockListUrl', () => {
  it('returns the bare endpoint when no filters are provided', () => {
    expect(buildStatBlockListUrl()).toBe('/api/statblocks');
    expect(buildStatBlockListUrl({})).toBe('/api/statblocks');
  });

  it('encodes campaign_id and search', () => {
    const url = buildStatBlockListUrl({
      campaignId: 'camp-1',
      search: 'dragon',
    });
    expect(url).toContain('campaign_id=camp-1');
    expect(url).toContain('search=dragon');
  });

  it('appends repeated type and size params', () => {
    const url = buildStatBlockListUrl({ types: ['beast', 'undead'], sizes: ['Small'] });
    expect(url).toContain('type=beast');
    expect(url).toContain('type=undead');
    expect(url).toContain('size=Small');
  });

  it('encodes CR bounds and source', () => {
    const url = buildStatBlockListUrl({ crMin: 0.5, crMax: 5, source: 'homebrew' });
    expect(url).toContain('cr_min=0.5');
    expect(url).toContain('cr_max=5');
    expect(url).toContain('source=homebrew');
  });

  it('skips empty / zero pagination', () => {
    const url = buildStatBlockListUrl({ limit: 0, offset: 0 });
    expect(url).toBe('/api/statblocks');
  });

  it('includes pagination when set', () => {
    const url = buildStatBlockListUrl({ limit: 25, offset: 50 });
    expect(url).toContain('limit=25');
    expect(url).toContain('offset=50');
  });
});

describe('listStatBlocks', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('GETs the constructed URL and returns normalized JSON', async () => {
    const mock = [{ id: 'c1', name: 'Goblin', homebrew: { Bool: false, Valid: true } }];
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mock),
    });
    const result = await listStatBlocks({ campaignId: 'c1', search: 'gob' });
    expect(result[0]).toMatchObject({ id: 'c1', name: 'Goblin', homebrew: false });
    const [url] = fetch.mock.calls[0];
    expect(url).toContain('/api/statblocks?');
    expect(url).toContain('campaign_id=c1');
    expect(url).toContain('search=gob');
  });

  it('returns an empty array when the body is null', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(null),
    });
    expect(await listStatBlocks({})).toEqual([]);
  });

  it('throws on non-ok responses', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: () => Promise.resolve('boom'),
    });
    await expect(listStatBlocks({})).rejects.toThrow(/boom/);
  });
});

describe('getStatBlock', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('GETs the per-id endpoint with optional campaign_id', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ id: 'c1', name: 'Goblin' }),
    });
    await getStatBlock('c1', 'camp-1');
    const [url] = fetch.mock.calls[0];
    expect(url).toBe('/api/statblocks/c1?campaign_id=camp-1');
  });

  it('omits campaign_id when not provided', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({}),
    });
    await getStatBlock('c1');
    const [url] = fetch.mock.calls[0];
    expect(url).toBe('/api/statblocks/c1');
  });
});

describe('normalizeStatBlock', () => {
  it('unwraps sql.NullString into a plain string or null', () => {
    expect(normalizeStatBlock({ alignment: { String: 'neutral evil', Valid: true } }).alignment).toBe(
      'neutral evil',
    );
    expect(normalizeStatBlock({ alignment: { String: '', Valid: false } }).alignment).toBeNull();
    expect(normalizeStatBlock({ source: { String: 'SRD', Valid: true } }).source).toBe('SRD');
  });

  it('unwraps sql.NullBool into a real boolean', () => {
    expect(normalizeStatBlock({ homebrew: { Bool: true, Valid: true } }).homebrew).toBe(true);
    expect(normalizeStatBlock({ homebrew: { Bool: false, Valid: true } }).homebrew).toBe(false);
    expect(normalizeStatBlock({ homebrew: { Bool: false, Valid: false } }).homebrew).toBe(false);
    expect(normalizeStatBlock({}).homebrew).toBe(false);
  });

  it('unwraps pqtype.NullRawMessage into its payload or null', () => {
    const abilities = [{ name: 'Nimble Escape', description: 'x' }];
    expect(
      normalizeStatBlock({ abilities: { RawMessage: abilities, Valid: true } }).abilities,
    ).toEqual(abilities);
    expect(normalizeStatBlock({ skills: { RawMessage: null, Valid: false } }).skills).toBeNull();
    expect(
      normalizeStatBlock({ saving_throws: { RawMessage: { dex: 2 }, Valid: true } }).saving_throws,
    ).toEqual({ dex: 2 });
  });

  it('passes already-clean values through unchanged (idempotent)', () => {
    const clean = {
      id: 'c1',
      name: 'Goblin',
      alignment: 'neutral evil',
      homebrew: false,
      abilities: [{ name: 'A', description: 'd' }],
      speed: { walk: 30 },
      ability_scores: { str: 8 },
      attacks: [{ name: 'Bite' }],
    };
    expect(normalizeStatBlock(normalizeStatBlock(clean))).toMatchObject({
      id: 'c1',
      name: 'Goblin',
      alignment: 'neutral evil',
      homebrew: false,
      speed: { walk: 30 },
    });
  });

  it('preserves plain pass-through fields and returns non-objects unchanged', () => {
    const out = normalizeStatBlock({ id: 'x', ac: 15, cr: '1/4', languages: ['Common'] });
    expect(out).toMatchObject({ id: 'x', ac: 15, cr: '1/4', languages: ['Common'] });
    expect(normalizeStatBlock(null)).toBeNull();
    expect(normalizeStatBlock('nope')).toBe('nope');
  });
});
