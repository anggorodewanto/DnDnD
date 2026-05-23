/**
 * HomePanel structural tests.
 *
 * The repo's vitest config runs under the node environment with no DOM, so
 * Svelte components cannot be rendered/queried here. We follow the existing
 * pattern (see CampaignsPage tests) and parse the .svelte source as text to
 * assert on the contract the panel must honour: which lib it talks to, which
 * cards/buttons it renders, and the loading / error / empty states.
 */
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const panelSrc = readFileSync(
  fileURLToPath(new URL('./HomePanel.svelte', import.meta.url)),
  'utf8',
);

describe('HomePanel.svelte', () => {
  it('accepts a campaignId prop so the parent App can pass /api/me state in', () => {
    expect(panelSrc).toContain('campaignId');
    expect(panelSrc).toContain('$props()');
  });

  it('fetches the Campaign Home payload on mount via lib/home.js', () => {
    expect(panelSrc).toContain("from './lib/home.js'");
    expect(panelSrc).toContain('getHomeData');
    expect(panelSrc).toContain('$effect');
  });

  it('renders the four required cards', () => {
    expect(panelSrc).toContain('Pending dm-queue Items');
    expect(panelSrc).toContain('Pending Character Approvals');
    expect(panelSrc).toContain('Active Encounters');
    expect(panelSrc).toContain('Saved Encounters');
  });

  it('exposes Pause/Resume + quick-action navigation', () => {
    expect(panelSrc).toContain('toggleCampaignStatus');
    expect(panelSrc).toContain('pauseButtonLabel');
    expect(panelSrc).toContain('#encounter-new');
    expect(panelSrc).toContain('#narrate');
  });

  it('does NOT re-import lib/api.js (parallel agents own that module)', () => {
    expect(panelSrc).not.toContain("from './lib/api.js'");
  });

  it('does NOT POST directly to /api/campaigns — must go through home.js helpers', () => {
    expect(panelSrc).not.toMatch(/fetch\(['"`]\/api\/campaigns\//);
  });
});
