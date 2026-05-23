import { describe, it, expect, vi, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import {
  getHomeData,
  pauseCampaign,
  resumeCampaign,
  toggleCampaignStatus,
  pauseButtonLabel,
} from './home.js';

describe('pauseButtonLabel', () => {
  it('returns Resume Campaign when status is paused', () => {
    expect(pauseButtonLabel('paused')).toBe('Resume Campaign');
  });
  it('returns Pause Campaign when status is active', () => {
    expect(pauseButtonLabel('active')).toBe('Pause Campaign');
  });
  it('returns Pause Campaign for unknown/empty status (safe default)', () => {
    expect(pauseButtonLabel('')).toBe('Pause Campaign');
    expect(pauseButtonLabel(undefined)).toBe('Pause Campaign');
    expect(pauseButtonLabel('archived')).toBe('Pause Campaign');
  });
});

describe('getHomeData', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('GETs /api/dashboard/home and returns the parsed JSON', async () => {
    const payload = {
      campaign_id: 'c-1',
      campaign_status: 'active',
      dm_queue_count: 3,
      pending_approvals: 2,
      active_encounters: ['Battle'],
      saved_encounters: ['Saved'],
    };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(payload),
    });

    const result = await getHomeData();
    expect(result).toEqual(payload);
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(fetch.mock.calls[0][0]).toBe('/api/dashboard/home');
  });

  it('throws when the response is not ok', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: () => Promise.resolve('boom'),
    });
    await expect(getHomeData()).rejects.toThrow(/boom/);
  });
});

describe('pauseCampaign', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('POSTs to /api/campaigns/{id}/pause', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ status: 'paused' }),
    });
    const out = await pauseCampaign('c-1');
    expect(out).toEqual({ status: 'paused' });
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/campaigns/c-1/pause');
    expect(options.method).toBe('POST');
  });

  it('throws on non-ok responses', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      text: () => Promise.resolve('already paused'),
    });
    await expect(pauseCampaign('c-1')).rejects.toThrow(/already paused/);
  });
});

describe('resumeCampaign', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('POSTs to /api/campaigns/{id}/resume', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ status: 'active' }),
    });
    const out = await resumeCampaign('c-1');
    expect(out).toEqual({ status: 'active' });
    expect(fetch.mock.calls[0][0]).toBe('/api/campaigns/c-1/resume');
  });
});

describe('toggleCampaignStatus', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('calls pause when current status is active', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ status: 'paused' }),
    });
    await toggleCampaignStatus('c-1', 'active');
    expect(fetch.mock.calls[0][0]).toBe('/api/campaigns/c-1/pause');
  });

  it('calls resume when current status is paused', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ status: 'active' }),
    });
    await toggleCampaignStatus('c-1', 'paused');
    expect(fetch.mock.calls[0][0]).toBe('/api/campaigns/c-1/resume');
  });
});

const panelSrc = readFileSync(
  fileURLToPath(new URL('../HomePanel.svelte', import.meta.url)),
  'utf8',
);

describe('HomePanel.svelte', () => {
  it('imports its data fetcher from lib/home.js', () => {
    expect(panelSrc).toContain("from './lib/home.js'");
    expect(panelSrc).toContain('getHomeData');
  });

  it('renders the Campaign Home title', () => {
    expect(panelSrc).toContain('Campaign Home');
  });

  it('renders all four cards', () => {
    expect(panelSrc).toContain('Pending dm-queue Items');
    expect(panelSrc).toContain('Pending Character Approvals');
    expect(panelSrc).toContain('Active Encounters');
    expect(panelSrc).toContain('Saved Encounters');
  });

  it('shows empty-state copy when encounter lists are empty', () => {
    expect(panelSrc).toContain('No active encounters');
    expect(panelSrc).toContain('No saved encounters');
  });

  it('renders quick-action buttons / links', () => {
    expect(panelSrc).toContain('New Encounter');
    expect(panelSrc).toContain('Narrate');
    expect(panelSrc).toContain('#encounter-new');
    expect(panelSrc).toContain('#narrate');
  });

  it('uses the pauseButtonLabel helper to label the pause/resume button', () => {
    expect(panelSrc).toContain('pauseButtonLabel');
  });

  it('wires the pause/resume button to toggleCampaignStatus', () => {
    expect(panelSrc).toContain('toggleCampaignStatus');
  });

  it('renders a loading state', () => {
    expect(panelSrc).toMatch(/loading/i);
  });

  it('renders an error state', () => {
    expect(panelSrc).toMatch(/error/i);
  });

  it('handles the empty-campaign state', () => {
    expect(panelSrc).toMatch(/No active campaign|no campaign/i);
  });
});
