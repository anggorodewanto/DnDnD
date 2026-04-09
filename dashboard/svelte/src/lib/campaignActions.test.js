import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  pauseCampaign,
  resumeCampaign,
  toggleCampaignStatus,
  nextCampaignStatus,
} from './campaignActions.js';

describe('nextCampaignStatus', () => {
  it('returns paused when active', () => {
    expect(nextCampaignStatus('active')).toBe('paused');
  });
  it('returns active when paused', () => {
    expect(nextCampaignStatus('paused')).toBe('active');
  });
  it('returns active as a safe default for unknown statuses', () => {
    expect(nextCampaignStatus('archived')).toBe('active');
    expect(nextCampaignStatus(undefined)).toBe('active');
  });
});

describe('pauseCampaign', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('POSTs to the pause endpoint and returns JSON', async () => {
    const out = { id: 'c1', status: 'paused' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(out),
    });

    const result = await pauseCampaign('c1');

    expect(result).toEqual(out);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/campaigns/c1/pause');
    expect(options.method).toBe('POST');
  });

  it('throws on non-ok', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      text: () => Promise.resolve('already paused'),
    });
    await expect(pauseCampaign('c1')).rejects.toThrow(/already paused/);
  });
});

describe('resumeCampaign', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('POSTs to the resume endpoint and returns JSON', async () => {
    const out = { id: 'c1', status: 'active' };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(out),
    });

    const result = await resumeCampaign('c1');

    expect(result).toEqual(out);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/campaigns/c1/resume');
    expect(options.method).toBe('POST');
  });

  it('throws on non-ok', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: () => Promise.resolve(''),
    });
    await expect(resumeCampaign('c1')).rejects.toThrow(/500/);
  });
});

describe('toggleCampaignStatus', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('calls pause when current status is active', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ id: 'c1', status: 'paused' }),
    });
    const res = await toggleCampaignStatus('c1', 'active');
    expect(res.status).toBe('paused');
    expect(fetch.mock.calls[0][0]).toBe('/api/campaigns/c1/pause');
  });

  it('calls resume when current status is paused', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ id: 'c1', status: 'active' }),
    });
    const res = await toggleCampaignStatus('c1', 'paused');
    expect(res.status).toBe('active');
    expect(fetch.mock.calls[0][0]).toBe('/api/campaigns/c1/resume');
  });
});
