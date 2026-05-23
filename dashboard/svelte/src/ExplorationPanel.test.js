import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const panelSrc = readFileSync(resolve(__dirname, 'ExplorationPanel.svelte'), 'utf-8');

describe('ExplorationPanel', () => {
  it('imports the exploration JSON helpers (not lib/api.js)', () => {
    expect(panelSrc).toContain("from './lib/exploration.js'");
    expect(panelSrc).toContain('fetchExplorationData');
    expect(panelSrc).toContain('startExploration');
    expect(panelSrc).toContain('transitionToCombat');
    expect(panelSrc).toContain('parseOverridesText');
    expect(panelSrc).not.toContain("from './lib/api.js'");
  });

  it('declares a campaignId prop', () => {
    expect(panelSrc).toMatch(/\$props\s*\(\s*\)/);
    expect(panelSrc).toContain('campaignId');
  });

  it('renders a map selector dropdown bound to a selectedMapId', () => {
    expect(panelSrc).toContain('selectedMapId');
    expect(panelSrc).toMatch(/<select[^>]*>/);
    expect(panelSrc).toMatch(/{#each\s+maps\s+as\s+m}/);
  });

  it('renders a Start Exploration button', () => {
    expect(panelSrc).toMatch(/Start Exploration/);
  });

  it('renders a Transition to Combat button', () => {
    expect(panelSrc).toMatch(/Transition to Combat/);
  });

  it('shows the current exploration encounter state when present', () => {
    // Either references currentEncounter or encounter state object.
    expect(panelSrc).toMatch(/currentEncounter|currentState|currentExploration/);
  });

  it('does not POST URL-encoded form data', () => {
    expect(panelSrc).not.toContain('application/x-www-form-urlencoded');
    expect(panelSrc).not.toContain('URLSearchParams');
  });

  it('does not reference the legacy GET /dashboard/exploration page', () => {
    expect(panelSrc).not.toContain('"/dashboard/exploration"');
    expect(panelSrc).not.toContain("'/dashboard/exploration'");
  });
});
