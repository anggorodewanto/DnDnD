import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  STATBLOCK_LIBRARY_ENDPOINT,
  buildStatBlockListUrl,
  listStatBlocks,
  getStatBlock,
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

  it('GETs the constructed URL and returns parsed JSON', async () => {
    const mock = [{ id: 'c1', name: 'Goblin' }];
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mock),
    });
    const result = await listStatBlocks({ campaignId: 'c1', search: 'gob' });
    expect(result).toEqual(mock);
    const [url] = fetch.mock.calls[0];
    expect(url).toContain('/api/statblocks?');
    expect(url).toContain('campaign_id=c1');
    expect(url).toContain('search=gob');
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
