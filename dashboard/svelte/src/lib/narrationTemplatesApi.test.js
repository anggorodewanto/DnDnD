import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  listNarrationTemplates,
  createNarrationTemplate,
  updateNarrationTemplate,
  deleteNarrationTemplate,
  duplicateNarrationTemplate,
} from './api.js';

function mockFetch(response = {}) {
  globalThis.fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: () => Promise.resolve(response),
    text: () => Promise.resolve(''),
  });
}

beforeEach(() => {
  vi.restoreAllMocks();
});

describe('listNarrationTemplates', () => {
  it('encodes filters as query string', async () => {
    mockFetch([{ id: 't1', name: 'Tavern' }]);
    const out = await listNarrationTemplates('camp-1', { category: 'Loc', q: 'tav' });
    expect(out).toEqual([{ id: 't1', name: 'Tavern' }]);
    const [url] = fetch.mock.calls[0];
    expect(url).toContain('campaign_id=camp-1');
    expect(url).toContain('category=Loc');
    expect(url).toContain('q=tav');
  });

  it('omits empty filters', async () => {
    mockFetch([]);
    await listNarrationTemplates('camp-1');
    const [url] = fetch.mock.calls[0];
    expect(url).not.toContain('category=');
    expect(url).not.toContain('q=');
  });
});

describe('createNarrationTemplate', () => {
  it('POSTs JSON body', async () => {
    mockFetch({ id: 'new' });
    const out = await createNarrationTemplate({
      campaign_id: 'c',
      name: 'n',
      category: 'cat',
      body: 'b',
    });
    expect(out).toEqual({ id: 'new' });
    const [url, opts] = fetch.mock.calls[0];
    expect(url).toBe('/api/narration/templates');
    expect(opts.method).toBe('POST');
    expect(JSON.parse(opts.body)).toEqual({
      campaign_id: 'c',
      name: 'n',
      category: 'cat',
      body: 'b',
    });
  });
});

describe('updateNarrationTemplate', () => {
  it('PUTs to the id endpoint', async () => {
    mockFetch({ id: 't1', name: 'updated' });
    const out = await updateNarrationTemplate('t1', { name: 'updated', body: 'b' });
    expect(out.name).toBe('updated');
    const [url, opts] = fetch.mock.calls[0];
    expect(url).toBe('/api/narration/templates/t1');
    expect(opts.method).toBe('PUT');
  });
});

describe('deleteNarrationTemplate', () => {
  it('DELETEs the id endpoint', async () => {
    mockFetch({});
    await deleteNarrationTemplate('t1');
    const [url, opts] = fetch.mock.calls[0];
    expect(url).toBe('/api/narration/templates/t1');
    expect(opts.method).toBe('DELETE');
  });
});

describe('duplicateNarrationTemplate', () => {
  it('POSTs to /duplicate', async () => {
    mockFetch({ id: 't2' });
    const out = await duplicateNarrationTemplate('t1');
    expect(out).toEqual({ id: 't2' });
    const [url, opts] = fetch.mock.calls[0];
    expect(url).toBe('/api/narration/templates/t1/duplicate');
    expect(opts.method).toBe('POST');
  });
});
